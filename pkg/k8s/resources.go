package k8s

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"strings"
)

var (
	deprecatedResources = map[schema.GroupKind]bool{
		{Group: "extensions", Kind: "Ingress"}: true,
		{Group: "", Kind: "ComponentStatus"}:   true,
	}
)

func (k *K8sCluster) doGetApiGroupResources() ([]*restmapper.APIGroupResources, error) {
	var ret []*restmapper.APIGroupResources

	// the discovery client doesn't support cancellation, so we need to run it in the background and wait for it
	finished := make(chan error)
	go func() {
		var err error
		ret, err = restmapper.GetAPIGroupResources(k.discovery)
		if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
			finished <- err
			return
		}
		finished <- nil
	}()

	select {
	case err := <-finished:
		if err != nil {
			return nil, err
		}
	case <-k.ctx.Done():
		return nil, fmt.Errorf("failed listing api resources: %w", k.ctx.Err())
	}
	return ret, nil
}

func (k *K8sCluster) doGetFilteredGVKs(ret *[]schema.GroupVersionKind, g v1.APIGroup, v string, vrs []v1.APIResource, filter func(ar *v1.APIResource) bool) {
	for _, vr := range vrs {
		if strings.Index(vr.Name, "/") != -1 {
			// skip sub-resources
			continue
		}
		gvk := schema.GroupVersionKind{
			Group:   g.Name,
			Version: v,
			Kind:    vr.Kind,
		}
		if _, ok := deprecatedResources[gvk.GroupKind()]; ok {
			continue
		}
		if filter != nil && !filter(&vr) {
			continue
		}
		*ret = append(*ret, gvk)
	}
}

func (k *K8sCluster) GetFilteredGVKs(filter func(ar *v1.APIResource) bool) ([]schema.GroupVersionKind, error) {
	agrs, err := k.doGetApiGroupResources()
	if err != nil {
		return nil, err
	}

	var ret []schema.GroupVersionKind
	for _, agr := range agrs {
		for v, vrs := range agr.VersionedResources {
			k.doGetFilteredGVKs(&ret, agr.Group, v, vrs, filter)
		}
	}
	return ret, nil
}

func (k *K8sCluster) GetFilteredPreferredGVKs(filter func(ar *v1.APIResource) bool) ([]schema.GroupVersionKind, error) {
	agrs, err := k.doGetApiGroupResources()
	if err != nil {
		return nil, err
	}

	var ret []schema.GroupVersionKind
	for _, agr := range agrs {
		vrs, ok := agr.VersionedResources[agr.Group.PreferredVersion.Version]
		if !ok {
			continue
		}
		k.doGetFilteredGVKs(&ret, agr.Group, agr.Group.PreferredVersion.Version, vrs, filter)
	}
	return ret, nil
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
