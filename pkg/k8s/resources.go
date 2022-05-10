package k8s

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type k8sResources struct {
	ctx       context.Context
	k         *K8sCluster
	discovery *disk.CachedDiscoveryClient

	allResources       map[schema.GroupVersionKind]v1.APIResource
	preferredResources map[schema.GroupKind]v1.APIResource
	crds               map[schema.GroupKind]*uo.UnstructuredObject
	mutex              sync.Mutex
}

func newK8sResources(ctx context.Context, config *rest.Config, k2 *K8sCluster) (*k8sResources, error) {
	k := &k8sResources{
		ctx:                ctx,
		k:                  k2,
		allResources:       map[schema.GroupVersionKind]v1.APIResource{},
		preferredResources: map[schema.GroupKind]v1.APIResource{},
		crds:               map[schema.GroupKind]*uo.UnstructuredObject{},
		mutex:              sync.Mutex{},
	}

	apiHost, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	discoveryCacheDir := filepath.Join(utils.GetTmpBaseDir(), "kube-cache/discovery", apiHost.Hostname())
	discovery2, err := disk.NewCachedDiscoveryClientForConfig(dynamic.ConfigFor(config), discoveryCacheDir, "", time.Hour*24)
	if err != nil {
		return nil, err
	}
	k.discovery = discovery2

	return k, nil
}

func (k *k8sResources) updateResources() error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.allResources = map[schema.GroupVersionKind]v1.APIResource{}
	k.preferredResources = map[schema.GroupKind]v1.APIResource{}
	k.crds = map[schema.GroupKind]*uo.UnstructuredObject{}

	// the discovery client doesn't support cancellation, so we need to run it in the background and wait for it
	var arls []*v1.APIResourceList
	var preferredArls []*v1.APIResourceList
	finished := make(chan error)
	go func() {
		var err error
		_, arls, err = k.discovery.ServerGroupsAndResources()
		if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
			finished <- err
			return
		}
		preferredArls, err = k.discovery.ServerPreferredResources()
		if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
			finished <- err
			return
		}
		finished <- nil
	}()

	select {
	case err := <-finished:
		if err != nil {
			return err
		}
	case <-k.ctx.Done():
		return fmt.Errorf("failed listing api resources: %w", k.ctx.Err())
	}

	for _, arl := range arls {
		for _, ar := range arl.APIResources {
			if strings.Index(ar.Name, "/") != -1 {
				// skip subresources
				continue
			}
			gv, err := schema.ParseGroupVersion(arl.GroupVersion)
			if err != nil {
				continue
			}

			ar := ar
			ar.Group = gv.Group
			ar.Version = gv.Version

			gvk := schema.GroupVersionKind{
				Group:   ar.Group,
				Version: ar.Version,
				Kind:    ar.Kind,
			}
			if _, ok := deprecatedResources[gvk.GroupKind()]; ok {
				continue
			}
			if _, ok := k.allResources[gvk]; ok {
				ok = false
			}
			k.allResources[gvk] = ar
		}
	}

	for _, arl := range preferredArls {
		for _, ar := range arl.APIResources {
			if strings.Index(ar.Name, "/") != -1 {
				// skip subresources
				continue
			}
			gv, err := schema.ParseGroupVersion(arl.GroupVersion)
			if err != nil {
				continue
			}

			ar := ar
			ar.Group = gv.Group
			ar.Version = gv.Version

			gk := schema.GroupKind{
				Group: ar.Group,
				Kind:  ar.Kind,
			}
			if _, ok := deprecatedResources[gk]; ok {
				continue
			}
			k.preferredResources[gk] = ar
		}
	}
	return nil
}

var crdGK = schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}

func (k *k8sResources) updateResourcesFromCRDs(clients *k8sClients) error {
	ar := k.GetPreferredResource(crdGK)
	if ar == nil {
		return fmt.Errorf("api resource for CRDs not found")
	}
	gvr, err := k.GetGVRForGVK(schema.GroupVersionKind{
		Group:   ar.Group,
		Version: ar.Version,
		Kind:    ar.Kind,
	})
	if err != nil {
		return err
	}

	var crds *unstructured.UnstructuredList
	_, err = clients.withClientFromPool(func(p *parallelClientEntry) error {
		var err error
		crds, err = p.dynamicClient.Resource(*gvr).List(k.ctx, v1.ListOptions{})
		return err
	})
	if err != nil {
		return err
	}

	for _, x := range crds.Items {
		x2 := x
		crd := uo.FromUnstructured(&x2)
		err = k.UpdateResourcesFromCRD(crd)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *k8sResources) UpdateResourcesFromCRD(crd *uo.UnstructuredObject) error {
	var err error
	var ar v1.APIResource
	ar.Group, _, err = crd.GetNestedString("spec", "group")
	if err != nil {
		return err
	}
	ar.Name, _, err = crd.GetNestedString("spec", "names", "plural")
	if err != nil {
		return err
	}
	ar.Kind, _, err = crd.GetNestedString("spec", "names", "kind")
	if err != nil {
		return err
	}
	ar.SingularName, _, err = crd.GetNestedString("spec", "names", "singular")
	if err != nil {
		return err
	}
	scope, _, err := crd.GetNestedString("spec", "scope")
	if err != nil {
		return err
	}
	ar.Namespaced = strings.ToLower(scope) == "namespaced"
	ar.ShortNames, _, err = crd.GetNestedStringList("spec", "names", "shortNames")
	if err != nil {
		return err
	}
	ar.Categories, _, err = crd.GetNestedStringList("spec", "names", "categories")
	if err != nil {
		return err
	}
	versions, _, err := crd.GetNestedObjectList("spec", "versions")
	if err != nil {
		return err
	}

	ar.Verbs = []string{"delete", "deletecollection", "get", "list", "patch", "create", "update", "watch"}

	gk := schema.GroupKind{
		Group: ar.Group,
		Kind:  ar.Kind,
	}

	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.crds[gk] = crd

	var versionStrs []string
	for _, v := range versions {
		name, _, err := v.GetNestedString("name")
		if err != nil {
			return err
		}
		versionStrs = append(versionStrs, name)
	}

	// Sort the same way as api discovery does it. The first entry is then the preferred version
	sort.Slice(versionStrs, func(i, j int) bool {
		return version.CompareKubeAwareVersionStrings(versionStrs[i], versionStrs[j]) > 0
	})

	for i, v := range versionStrs {
		ar2 := ar
		ar2.Version = v
		gvk := schema.GroupVersionKind{
			Group:   gk.Group,
			Version: ar2.Version,
			Kind:    gk.Kind,
		}
		k.allResources[gvk] = ar2

		if i == 0 {
			k.preferredResources[gk] = ar2
		}
	}

	return nil
}

func (k *k8sResources) IsNamespaced(gv schema.GroupKind) *bool {
	ar := k.GetPreferredResource(gv)
	if ar == nil {
		return nil
	}
	return &ar.Namespaced
}

func (k *k8sResources) FixNamespace(o *uo.UnstructuredObject, def string) {
	ref := o.GetK8sRef()
	namespaced := k.IsNamespaced(ref.GVK.GroupKind())
	if namespaced == nil {
		return
	}
	if !*namespaced && ref.Namespace != "" {
		o.SetK8sNamespace("")
	} else if *namespaced && ref.Namespace == "" {
		o.SetK8sNamespace(def)
	}
}

func (k *k8sResources) FixNamespaceInRef(ref k8s.ObjectRef) k8s.ObjectRef {
	namespaced := k.IsNamespaced(ref.GVK.GroupKind())
	if namespaced == nil {
		return ref
	}
	if !*namespaced && ref.Namespace != "" {
		ref.Namespace = ""
	} else if *namespaced && ref.Namespace == "" {
		ref.Namespace = "default"
	}
	return ref
}

func (k *k8sResources) GetAllGroupVersions() ([]schema.GroupVersion, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	m := make(map[schema.GroupVersion]bool)
	var l []schema.GroupVersion

	for gvk, _ := range k.allResources {
		gv := gvk.GroupVersion()
		if _, ok := m[gv]; !ok {
			m[gv] = true
			l = append(l, gv)
		}
	}
	return l, nil
}

func (k *k8sResources) GetPreferredResource(gk schema.GroupKind) *v1.APIResource {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, ok := k.preferredResources[gk]
	if !ok {
		return nil
	}
	return &ar
}

func (k *k8sResources) GetFilteredGVKs(filter func(ar *v1.APIResource) bool) []schema.GroupVersionKind {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var ret []schema.GroupVersionKind
	for _, ar := range k.allResources {
		if !filter(&ar) {
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   ar.Group,
			Version: ar.Version,
			Kind:    ar.Kind,
		}
		ret = append(ret, gvk)
	}
	return ret
}

func (k *k8sResources) GetFilteredPreferredGVKs(filter func(ar *v1.APIResource) bool) []schema.GroupVersionKind {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var ret []schema.GroupVersionKind
	for _, ar := range k.preferredResources {
		if !filter(&ar) {
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   ar.Group,
			Version: ar.Version,
			Kind:    ar.Kind,
		}
		ret = append(ret, gvk)
	}
	return ret
}

func BuildGVKFilter(group *string, version *string, kind *string) func(ar *v1.APIResource) bool {
	return func(ar *v1.APIResource) bool {
		if group != nil && *group != ar.Group {
			return false
		}
		if version != nil && *version != ar.Version {
			return false
		}
		if kind != nil && *kind != ar.Kind {
			return false
		}
		return true
	}
}

func (k *k8sResources) GetGVRForGVK(gvk schema.GroupVersionKind) (*schema.GroupVersionResource, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, ok := k.allResources[gvk]
	if !ok {
		return nil, &meta.NoKindMatchError{
			GroupKind:        gvk.GroupKind(),
			SearchedVersions: []string{gvk.Version},
		}
	}

	return &schema.GroupVersionResource{
		Group:    ar.Group,
		Version:  ar.Version,
		Resource: ar.Name,
	}, nil
}

func (k *k8sResources) GetCRDForGK(gk schema.GroupKind) *uo.UnstructuredObject {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	crd, _ := k.crds[gk]

	return crd
}

func (k *k8sResources) GetSchemaForGVK(gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	crd := k.GetCRDForGK(gvk.GroupKind())
	if crd == nil {
		return nil, nil
	}

	versions, ok, err := crd.GetNestedObjectList("spec", "versions")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("versions not found in CRD")
	}

	for _, v := range versions {
		name, _, _ := v.GetNestedString("name")
		if name != gvk.Version {
			continue
		}
		s, ok, err := v.GetNestedObject("schema", "openAPIV3Schema")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("version %s has no schema", name)
		}
		return s, nil
	}
	return nil, fmt.Errorf("schema for %s not found", gvk.String())
}
