package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type webuiCmd struct {
	Run_ webuiRunCmd `cmd:"run" help:"Run the Kluctl Webui"`
}

func (cmd *webuiRunCmd) createResultStores(ctx context.Context) ([]results.ResultStore, []*rest.Config, error) {
	r := clientcmd.NewDefaultClientConfigLoadingRules()

	kcfg, err := r.Load()
	if err != nil {
		return nil, nil, err
	}

	var stores []results.ResultStore
	var configs []*rest.Config

	var contexts []string
	if cmd.AllContexts {
		for name, _ := range kcfg.Contexts {
			contexts = append(contexts, name)
		}
	} else if cmd.InCluster {
		// placeholder for current context
		contexts = append(contexts, "")
	} else {
		if len(cmd.Context) == 0 {
			// placeholder for current context
			contexts = append(contexts, "")
		}
		for _, c := range cmd.Context {
			found := false
			for name, _ := range kcfg.Contexts {
				if c == name {
					contexts = append(contexts, name)
					found = true
					break
				}
			}
			if !found {
				return nil, nil, fmt.Errorf("context '%s' not found in kubeconfig", c)
			}
		}
	}

	for _, c := range contexts {
		configOverrides := &clientcmd.ConfigOverrides{
			CurrentContext: c,
		}
		config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			r,
			configOverrides).ClientConfig()
		if err != nil {
			return nil, nil, err
		}

		client, err := client.NewWithWatch(config, client.Options{})
		if err != nil {
			return nil, nil, err
		}

		store, err := results.NewResultStoreSecrets(ctx, config, client, "", 0, 0)
		if err != nil {
			return nil, nil, err
		}
		stores = append(stores, store)
		configs = append(configs, config)
	}

	return stores, configs, nil
}
