package k8s

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"runtime"
	"sort"
	"strings"
	"sync"
)

var (
	deprecatedResources = map[schema.GroupKind]bool{
		{Group: "extensions", Kind: "Ingress"}: true,
		{Group: "", Kind: "ComponentStatus"}:   true,
	}
)

type k8sResources struct {
	ctx       context.Context
	discovery discovery.DiscoveryInterface

	allResources       map[schema.GroupVersionKind]v1.APIResource
	preferredResources map[schema.GroupKind]v1.APIResource
	crds               map[schema.GroupKind]*uo.UnstructuredObject
	mutex              sync.Mutex
}

func newK8sResources(ctx context.Context, clientFactory ClientFactory) (*k8sResources, error) {
	k := &k8sResources{
		ctx:                ctx,
		allResources:       map[schema.GroupVersionKind]v1.APIResource{},
		preferredResources: map[schema.GroupKind]v1.APIResource{},
		crds:               map[schema.GroupKind]*uo.UnstructuredObject{},
		mutex:              sync.Mutex{},
	}

	var err error
	k.discovery, err = clientFactory.DiscoveryClient()
	if err != nil {
		return nil, err
	}

	return k, nil
}

func (k *k8sResources) updateResources() error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.allResources = map[schema.GroupVersionKind]v1.APIResource{}
	k.preferredResources = map[schema.GroupKind]v1.APIResource{}
	k.crds = map[schema.GroupKind]*uo.UnstructuredObject{}

	// the discovery client doesn't support cancellation, so we need to run it in the background and wait for it
	var ags []*v1.APIGroup
	var arls []*v1.APIResourceList
	finished := make(chan error)
	go func() {
		var err error
		ags, arls, err = k.discovery.ServerGroupsAndResources()
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
		var ag *v1.APIGroup
		for _, x := range ags {
			if x.Name == arl.GroupVersionKind().Group {
				ag = x
				break
			}
		}
		if ag == nil {
			continue
		}

		for _, ar := range arl.APIResources {
			if ar.Version == "__internal" {
				continue
			}
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

			k.allResources[gvk] = ar

			if gvk.Version == ag.PreferredVersion.Version {
				k.preferredResources[gvk.GroupKind()] = ar
			}
		}
	}
	if len(k.preferredResources) == 0 {
		runtime.Breakpoint()
	}

	return nil
}

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

func (k *k8sResources) updateResourcesFromCRDs(clients *k8sClients) error {
	var crdList *unstructured.UnstructuredList
	_, err := clients.withDynamicClientForGVR(&crdGVR, "", func(r dynamic.ResourceInterface) error {
		var err error
		crdList, err = r.List(k.ctx, v1.ListOptions{})
		return err
	})
	if err != nil {
		return err
	}

	for _, x := range crdList.Items {
		x := x
		crd := uo.FromUnstructured(&x)
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
	namespaced := k.IsNamespaced(ref.GroupKind())
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
	namespaced := k.IsNamespaced(ref.GroupKind())
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
		if filter != nil && !filter(&ar) {
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
