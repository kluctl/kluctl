package helm

import (
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/registry"
)

func buildHelmConfig(k *k8s.K8sCluster) (*action.Configuration, error) {
	rc, err := registry.NewClient()
	if err != nil {
		return nil, err
	}

	return &action.Configuration{
		RESTClientGetter: k,
		RegistryClient:   rc,
	}, nil
}
