package helm

import (
	"github.com/Masterminds/semver/v3"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/registry"
)

func buildHelmConfig(k *k8s.K8sCluster, registryClient *registry.Client, k8sVersion string, apiVersions common.VersionSet) (*action.Configuration, error) {
	var kubeVersion *common.KubeVersion
	var err error
	if k != nil {
		kubeVersion, err = common.ParseKubeVersion(k.ServerVersion.String())
		if err != nil {
			return nil, err
		}
	}
	if k8sVersion != "" {
		kubeVersion, err = common.ParseKubeVersion(k8sVersion)
		if err != nil {
			return nil, err
		}
	}

	c := common.DefaultCapabilities.Copy()
	if kubeVersion != nil {
		c.KubeVersion = *kubeVersion
	}
	c.APIVersions = apiVersions

	kc := kube.New(k)
	// this prevents calling ToRawKubeConfigLoader, which is not implemented for now
	kc.Namespace = "default"

	return &action.Configuration{
		RESTClientGetter: k,
		KubeClient:       kc,
		RegistryClient:   registryClient,
		Capabilities:     c,
	}, nil
}

func IsSemantic(version string) bool {
	_, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	return true
}
