package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/webui"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type webuiCmd struct {
	Port        int      `group:"misc" help:"Port to serve the api and webui." default:"8080"`
	Context     []string `group:"misc" help:"List of kubernetes contexts to use."`
	AllContexts bool     `group:"misc" help:"Use all Kubernetes contexts found in the kubeconfig."`
	StaticPath  string   `group:"misc" help:"Build static webui."`
}

func (cmd *webuiCmd) Help() string {
	return `TODO`
}

func (cmd *webuiCmd) Run(ctx context.Context) error {

	stores, configs, err := cmd.createResultStores(ctx)
	if err != nil {
		return err
	}

	collector := results.NewResultsCollector(ctx, stores)
	collector.Start()

	if cmd.StaticPath != "" {
		collector.WaitForInitialSync()
		sbw := webui.NewStaticWebuiBuilder(collector)
		return sbw.Build(cmd.StaticPath)
	} else {
		server := webui.NewCommandResultsServer(ctx, collector, configs)
		return server.Run(cmd.Port)
	}
}

func (cmd *webuiCmd) createResultStores(ctx context.Context) ([]results.ResultStore, []*rest.Config, error) {
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

		client, err := client.New(config, client.Options{})
		if err != nil {
			return nil, nil, err
		}

		cache, err := cache.New(config, cache.Options{})
		if err != nil {
			return nil, nil, err
		}
		go cache.Start(ctx)

		store, err := results.NewResultStoreSecrets(ctx, client, cache, "", 0)
		if err != nil {
			return nil, nil, err
		}
		stores = append(stores, store)
		configs = append(configs, config)
	}

	return stores, configs, nil
}
