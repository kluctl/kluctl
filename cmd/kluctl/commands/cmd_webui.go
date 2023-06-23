package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/webui"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/netip"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type webuiCmd struct {
	Host        string   `group:"misc" help:"Host to bind to. Pass an empty string to bind to all addresses. Defaults to localhost."`
	Port        int      `group:"misc" help:"Port to bind to." default:"8080"`
	Context     []string `group:"misc" help:"List of kubernetes contexts to use."`
	AllContexts bool     `group:"misc" help:"Use all Kubernetes contexts found in the kubeconfig."`
	StaticPath  string   `group:"misc" help:"Build static webui."`

	InCluster        bool   `group:"misc" help:"This enables in-cluster functionality."`
	InClusterContext string `group:"misc" help:"The context to use fo in-cluster functionality."`

	OnlyApi bool `group:"misc" help:"Only serve API without the actual UI."`
}

func (cmd *webuiCmd) Help() string {
	return `TODO`
}

func (cmd *webuiCmd) Run(ctx context.Context) error {
	if !cmd.OnlyApi && !webui.IsWebUiBuildIncluded() {
		return fmt.Errorf("this build of Kluctl does not have the webui embedded")
	}
	if cmd.OnlyApi && cmd.StaticPath != "" {
		return fmt.Errorf("--static-path can not be combined with --only-api")
	}

	if !cmd.InCluster { // no authentication?
		if cmd.Host == "" {
			// we only use "localhost" as default if not running inside a cluster
			cmd.Host = "localhost"
		}
		if cmd.Host != "localhost" {
			isNonLocal := cmd.Host == ""
			if a, err := netip.ParseAddr(cmd.Host); err == nil { // on error, we assume it's a hostname
				if !a.IsLoopback() {
					isNonLocal = true
				}
			}
			if isNonLocal {
				status.Warning(ctx, "When running the webui without authentication enabled, it is extremely dangerous to expose it to non-localhost addresses, as the webui is running with admin privileges.")
			}
		}
	}

	var inClusterClient client.Client
	if cmd.InCluster {
		configOverrides := &clientcmd.ConfigOverrides{
			CurrentContext: cmd.InClusterContext,
		}
		config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			configOverrides).ClientConfig()
		if err != nil {
			return err
		}
		inClusterClient, err = client.NewWithWatch(config, client.Options{})
		if err != nil {
			return err
		}
	}

	stores, configs, err := cmd.createResultStores(ctx)
	if err != nil {
		return err
	}

	collector := results.NewResultsCollector(ctx, stores)
	collector.Start()

	if cmd.StaticPath != "" {
		st := status.Start(ctx, "Collecting results")
		defer st.Failed()
		err = collector.WaitForResults(time.Second, time.Second*30)
		if err != nil {
			return err
		}
		st.Success()
		sbw := webui.NewStaticWebuiBuilder(collector)
		return sbw.Build(cmd.StaticPath)
	} else {
		server := webui.NewCommandResultsServer(ctx, collector, configs, inClusterClient, inClusterClient != nil, cmd.OnlyApi)
		return server.Run(cmd.Host, cmd.Port)
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

		store, err := results.NewResultStoreSecrets(ctx, client, "", 0)
		if err != nil {
			return nil, nil, err
		}
		stores = append(stores, store)
		configs = append(configs, config)
	}

	return stores, configs, nil
}
