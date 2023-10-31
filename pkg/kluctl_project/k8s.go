package kluctl_project

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func (p *LoadedKluctlProject) LoadK8sConfig(ctx context.Context, targetName string, contextOverride string, offlineK8s bool) (*rest.Config, string, error) {
	if offlineK8s {
		return nil, "", nil
	}

	var contextName *string
	if targetName != "" {
		t, err := p.FindTarget(targetName)
		if err != nil {
			return nil, "", err
		}
		contextName = t.Context
	}
	if contextOverride != "" {
		contextName = &contextOverride
	}

	var err error
	var clientConfig *rest.Config
	var restConfig *api.Config
	clientConfig, restConfig, err = p.LoadArgs.ClientConfigGetter(contextName)
	if err != nil {
		if contextName == nil && clientcmd.IsEmptyConfig(err) {
			status.Warning(ctx, "No valid KUBECONFIG provided, which means the Kubernetes client is not available. Depending on your deployment project, this might cause follow-up errors.")
			return nil, "", nil
		}
		return nil, "", err
	}
	contextName = &restConfig.CurrentContext
	return clientConfig, *contextName, nil
}
