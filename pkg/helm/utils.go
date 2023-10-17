package helm

import (
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/registry"
)

func buildHelmConfig(k *k8s.K8sCluster, registryClient *registry.Client) (*action.Configuration, error) {
	return &action.Configuration{
		RESTClientGetter: k,
		RegistryClient:   registryClient,
	}, nil
}
