package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type webuiCmd struct {
	Run_  webuiRunCmd   `cmd:"run" help:"Run the Kluctl Webui"`
	Build webuiBuildCmd `cmd:"build" help:"Build the static Kluctl Webui"`
}

func createResultStores(ctx context.Context, k8sContexts []string, allContexts bool, inCluster bool) ([]results.ResultStore, []*rest.Config, error) {
	r := clientcmd.NewDefaultClientConfigLoadingRules()

	kcfg, err := r.Load()
	if err != nil {
		return nil, nil, err
	}

	var contexts []string
	if allContexts {
		for name, _ := range kcfg.Contexts {
			contexts = append(contexts, name)
		}
	} else if inCluster {
		// placeholder for current context
		contexts = append(contexts, "")
	} else {
		if len(k8sContexts) == 0 {
			// placeholder for current context
			contexts = append(contexts, "")
		}
		for _, c := range k8sContexts {
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

	gh := utils.NewGoHelper(ctx, 4)
	stores := make([]results.ResultStore, len(contexts))
	configs := make([]*rest.Config, len(contexts))
	for i, c := range contexts {
		i := i
		c := c
		gh.RunE(func() error {
			configOverrides := &clientcmd.ConfigOverrides{
				CurrentContext: c,
			}
			config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(r, configOverrides).ClientConfig()
			if err != nil {
				return err
			}

			_, mapper, err := k8s.CreateDiscoveryAndMapper(ctx, config)
			if err != nil {
				return err
			}

			store, err := buildResultStoreRO(ctx, config, mapper, &args.CommandResultReadOnlyFlags{})
			if err != nil {
				return err
			}
			stores[i] = store
			configs[i] = config
			return nil
		})
	}
	gh.Wait()
	if gh.ErrorOrNil() != nil {
		return nil, nil, gh.ErrorOrNil()
	}

	return stores, configs, nil
}
