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

	InCluster        bool   `group:"misc" help:"This enables in-cluster functionality. This also enforces authentication."`
	InClusterContext string `group:"misc" help:"The context to use fo in-cluster functionality."`

	OnlyApi bool `group:"misc" help:"Only serve API without the actual UI."`

	AuthSecretName string `group:"auth" help:"Specify the secret name for the secret used for internal encryption of tokens and cookies." default:"webui-secret"`
	AuthSecretKey  string `group:"auth" help:"Specify the secret key for the secret used for internal encryption of tokens and cookies." default:"auth-secret"`

	AuthAdminEnabled    bool   `group:"auth" help:"Enable the admin user." default:"true"`
	AuthAdminSecretName string `group:"auth" help:"Specify the secret name for the admin password." default:"webui-secret"`
	AuthAdminSecretKey  string `group:"auth" help:"Specify the secret key for the admin password." default:"admin-password"`

	AuthAdminRbacUser  string `group:"auth" help:"Specify the RBAC user to use for admin access." default:"kluctl-webui-admin"`
	AuthViewerRbacUser string `group:"auth" help:"Specify the RBAC user to use for viewer access." default:"kluctl-webui-viewer"`

	AuthOidcIssuerUrl        string   `group:"auth" help:"Specify the OIDC provider's issuer URL."`
	AuthOidcDisplayName      string   `group:"auth" help:"Specify the name of the OIDC provider to be displayed on the login page." default:"OpenID Connect"`
	AuthOidcClientID         string   `group:"auth" help:"Specify the ClientID."`
	AuthOidcClientSecretName string   `group:"auth" help:"Specify the secret name for the ClientSecret." default:"webui-secret"`
	AuthOidcClientSecretKey  string   `group:"auth" help:"Specify the secret name for the ClientSecret." default:"oidc-client-secret"`
	AuthOidcRedirectURL      string   `group:"auth" help:"Specify the redirect URL."`
	AuthOidcScope            []string `group:"auth" help:"Specify the scopes."`
	AuthOidcParam            []string `group:"auth" help:"Specify additional parameters to be passed to the authorize endpoint."`
	AuthOidcUserClaim        string   `group:"auth" help:"Specify claim for the username.'" default:"email"`
	AuthOidcGroupClaim       string   `group:"auth" help:"Specify claim for the groups.'" default:"groups"`
	AuthOidcAdminsGroup      []string `group:"auth" help:"Specify admins group names.'"`
	AuthOidcViewersGroup     []string `group:"auth" help:"Specify viewers group names.'"`
}

func (cmd *webuiCmd) Help() string {
	return `TODO`
}

func (cmd *webuiCmd) buildAuthConfig(ctx context.Context, c client.Client) (webui.AuthConfig, error) {
	var authConfig webui.AuthConfig
	authConfig.AuthEnabled = cmd.InCluster

	authConfig.AuthSecretName = cmd.AuthSecretName
	authConfig.AuthSecretKey = cmd.AuthSecretKey

	authConfig.AdminEnabled = cmd.AuthAdminEnabled
	authConfig.AdminSecretName = cmd.AuthAdminSecretName
	authConfig.AdminSecretKey = cmd.AuthAdminSecretKey

	authConfig.AdminRbacUser = cmd.AuthAdminRbacUser
	authConfig.ViewerRbacUser = cmd.AuthViewerRbacUser

	authConfig.OidcIssuerUrl = cmd.AuthOidcIssuerUrl
	authConfig.OidcDisplayName = cmd.AuthOidcDisplayName
	if cmd.AuthOidcIssuerUrl != "" {
		authConfig.OidcClientId = cmd.AuthOidcClientID
		authConfig.OidcClientSecretName = cmd.AuthOidcClientSecretName
		authConfig.OidcClientSecretKey = cmd.AuthOidcClientSecretKey
		authConfig.OidcRedirectUrl = cmd.AuthOidcRedirectURL
		authConfig.OidcScopes = cmd.AuthOidcScope
		authConfig.OidcParams = cmd.AuthOidcParam
		authConfig.OidcUserClaim = cmd.AuthOidcUserClaim
		authConfig.OidcGroupClaim = cmd.AuthOidcGroupClaim
		authConfig.OidcAdminsGroups = cmd.AuthOidcAdminsGroup
		authConfig.OidcViewersGroups = cmd.AuthOidcViewersGroup
	}

	return authConfig, nil
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

	var authConfig webui.AuthConfig

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

		authConfig, err = cmd.buildAuthConfig(ctx, inClusterClient)
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
		server, err := webui.NewCommandResultsServer(ctx, collector, configs, inClusterClient, authConfig, cmd.OnlyApi)
		if err != nil {
			return err
		}
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

		store, err := results.NewResultStoreSecrets(ctx, config, client, "", 0, 0)
		if err != nil {
			return nil, nil, err
		}
		stores = append(stores, store)
		configs = append(configs, config)
	}

	return stores, configs, nil
}
