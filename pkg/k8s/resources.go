package k8s

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type crdCacheEntry struct {
	crd *uo.UnstructuredObject
	err error
}

type k8sResources struct {
	ctx       context.Context
	discovery *disk.CachedDiscoveryClient

	allResources        map[schema.GroupVersionKind]*v1.APIResource
	preferredResources  map[schema.GroupKind]*v1.APIResource
	namespacedResources map[schema.GroupKind]bool
	crdCache            map[k8s.ObjectRef]crdCacheEntry
	mutex               sync.Mutex
}

func newK8sResources(ctx context.Context, config *rest.Config) (*k8sResources, error) {
	k := &k8sResources{
		ctx:                 ctx,
		allResources:        map[schema.GroupVersionKind]*v1.APIResource{},
		preferredResources:  map[schema.GroupKind]*v1.APIResource{},
		namespacedResources: map[schema.GroupKind]bool{},
		crdCache:            map[k8s.ObjectRef]crdCacheEntry{},
		mutex:               sync.Mutex{},
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

func (k *k8sResources) updateResources(doLock bool) error {
	if doLock {
		k.mutex.Lock()
		defer k.mutex.Unlock()
	}

	k.allResources = map[schema.GroupVersionKind]*v1.APIResource{}
	k.preferredResources = map[schema.GroupKind]*v1.APIResource{}
	k.crdCache = map[k8s.ObjectRef]crdCacheEntry{}

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
			k.allResources[gvk] = &ar
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
			k.preferredResources[gk] = &ar
		}
	}
	return nil
}

func (k *k8sResources) RediscoverResources() error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.discovery.Invalidate()
	return k.updateResources(false)
}

func (k *k8sResources) SetNamespaced(gv schema.GroupKind, namespaced bool) {
	k.mutex.Lock()
	defer k.mutex.Unlock()
	k.namespacedResources[gv] = namespaced
}

func (k *k8sResources) IsNamespaced(gv schema.GroupKind) bool {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	r, ok := k.preferredResources[gv]
	if !ok {
		n, _ := k.namespacedResources[gv]
		return n
	}
	return r.Namespaced
}

func (k *k8sResources) FixNamespace(o *uo.UnstructuredObject, def string) {
	ref := o.GetK8sRef()
	namespaced := k.IsNamespaced(ref.GVK.GroupKind())
	if !namespaced && ref.Namespace != "" {
		o.SetK8sNamespace("")
	} else if namespaced && ref.Namespace == "" {
		o.SetK8sNamespace(def)
	}
}

func (k *k8sResources) FixNamespaceInRef(ref k8s.ObjectRef) k8s.ObjectRef {
	namespaced := k.IsNamespaced(ref.GVK.GroupKind())
	if !namespaced && ref.Namespace != "" {
		ref.Namespace = ""
	} else if namespaced && ref.Namespace == "" {
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

func (k *k8sResources) GetFilteredGVKs(filter func(ar *v1.APIResource) bool) []schema.GroupVersionKind {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	var ret []schema.GroupVersionKind
	for _, ar := range k.allResources {
		if !filter(ar) {
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
		if !filter(ar) {
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

func (k *k8sResources) GetGVRForGVK(gvk schema.GroupVersionKind) (*schema.GroupVersionResource, bool, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	ar, ok := k.allResources[gvk]
	if !ok {
		return nil, false, &meta.NoKindMatchError{
			GroupKind:        gvk.GroupKind(),
			SearchedVersions: []string{gvk.Version},
		}
	}

	return &schema.GroupVersionResource{
		Group:    ar.Group,
		Version:  ar.Version,
		Resource: ar.Name,
	}, ar.Namespaced, nil
}

func (k *k8sResources) GetCRDForGVK(k2 *K8sCluster, gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	gvr, _, err := k.GetGVRForGVK(gvk)
	if err != nil {
		return nil, err
	}

	crdRef := k8s.ObjectRef{
		GVK:  schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"},
		Name: fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group),
	}

	k.mutex.Lock()
	x, ok := k.crdCache[crdRef]
	k.mutex.Unlock()
	if ok {
		return x.crd, x.err
	}

	crd, _, err := k2.GetSingleObject(crdRef)

	k.mutex.Lock()
	x.crd = crd
	x.err = err
	k.crdCache[crdRef] = x
	k.mutex.Unlock()

	return crd, err
}

func (k *k8sResources) GetSchemaForGVK(k2 *K8sCluster, gvk schema.GroupVersionKind) (*uo.UnstructuredObject, error) {
	crd, err := k.GetCRDForGVK(k2, gvk)
	if err != nil {
		return nil, err
	}

	versions, ok, err := crd.GetNestedObjectList("spec", "versions")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("versions not found in CRD")
	}

	for _, version := range versions {
		name, _, _ := version.GetNestedString("name")
		if name != gvk.Version {
			continue
		}
		s, ok, err := version.GetNestedObject("schema", "openAPIV3Schema")
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
