package k8s

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"strings"
)

var (
	deprecatedResources = map[schema.GroupResource]bool{
		{Group: "extensions", Resource: "ingresses"}: true,
		{Group: "", Resource: "componentstatuses"}:   true,
	}
)

func (k *K8sCluster) doGetApiGroupResources() ([]*restmapper.APIGroupResources, error) {
	var ret []*restmapper.APIGroupResources

	// the discovery client doesn't support cancellation, so we need to run it in the background and wait for it
	finished := make(chan error)
	go func() {
		// there is a race deep inside the cached discovery client
		// see https://github.com/kubernetes/apimachinery/issues/156
		k.discoveryMutex.Lock()
		defer k.discoveryMutex.Unlock()

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

func (k *K8sCluster) filterResource(ar *v1.APIResource) bool {
	if strings.Index(ar.Name, "/") != -1 {
		// skip sub-resources
		return false
	}

	gr := schema.GroupResource{
		Group:    ar.Group,
		Resource: ar.Name,
	}
	if _, ok := deprecatedResources[gr]; ok {
		return false
	}

	return true
}

func (k *K8sCluster) GetAllAPIResources() ([]v1.APIResource, error) {
	agrs, err := k.doGetApiGroupResources()
	if err != nil {
		return nil, err
	}

	var ret []v1.APIResource
	for _, agr := range agrs {
		for v, ars := range agr.VersionedResources {
			for _, ar := range ars {
				ar := ar
				ar.Group = agr.Group.Name
				ar.Version = v

				if !k.filterResource(&ar) {
					continue
				}
				ret = append(ret, ar)
			}
		}
	}
	return ret, nil
}

func (k *K8sCluster) GetAllPreferredAPIResources() ([]v1.APIResource, error) {
	agrs, err := k.doGetApiGroupResources()
	if err != nil {
		return nil, err
	}

	preferredVersions := map[string]string{}
	grs := map[schema.GroupResource][]v1.APIResource{}
	for _, agr := range agrs {
		preferredVersions[agr.Group.Name] = agr.Group.PreferredVersion.Version
		for v, ars := range agr.VersionedResources {
			for _, ar := range ars {
				ar := ar
				ar.Group = agr.Group.Name
				ar.Version = v

				if !k.filterResource(&ar) {
					continue
				}

				gr := schema.GroupResource{
					Group:    agr.Group.Name,
					Resource: ar.Name,
				}

				l := grs[gr]
				l = append(l, ar)
				grs[gr] = l
			}
		}
	}

	var ret []v1.APIResource
	for gkr, ars := range grs {
		var preferredAR *v1.APIResource
		if len(ars) > 1 {
			err = nil
		}
		for _, ar := range ars {
			ar := ar
			if preferredVersions[gkr.Group] == ar.Version {
				preferredAR = &ar
				break
			}
			if preferredAR == nil || version.CompareKubeAwareVersionStrings(preferredAR.Version, ar.Version) < 0 {
				preferredAR = &ar
			}
		}
		ret = append(ret, *preferredAR)
	}
	return ret, nil
}

func (k *K8sCluster) GetFilteredPreferredAPIResources(filter func(ar *v1.APIResource) bool) ([]v1.APIResource, error) {
	ars, err := k.GetAllPreferredAPIResources()
	if err != nil {
		return nil, err
	}

	var ret []v1.APIResource
	for _, ar := range ars {
		if filter == nil || filter(&ar) {
			ret = append(ret, ar)
		}
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
