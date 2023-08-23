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
)

type webuiRunCmd struct {
	Host        string   `group:"misc" help:"Host to bind to. Pass an empty string to bind to all addresses." default:"localhost"`
	Port        int      `group:"misc" help:"Port to bind to." default:"8080"`
	Context     []string `group:"misc" help:"List of kubernetes contexts to use."`
	AllContexts bool     `group:"misc" help:"Use all Kubernetes contexts found in the kubeconfig."`

	InCluster        bool   `group:"misc" help:"This enables in-cluster functionality. This also enforces authentication."`
	InClusterContext string `group:"misc" help:"The context to use fo in-cluster functionality."`

	OnlyApi bool `group:"misc" help:"Only serve API without the actual UI."`

	AuthSecretName string `group:"auth" help:"Specify the secret name for the secret used for internal encryption of tokens and cookies." default:"webui-secret"`
	AuthSecretKey  string `group:"auth" help:"Specify the secret key for the secret used for internal encryption of tokens and cookies." default:"auth-secret"`

	AuthStaticLoginEnabled    bool   `group:"auth" help:"Enable the admin user." default:"true"`
	AuthStaticLoginSecretName string `group:"auth" help:"Specify the secret name for the admin and viewer passwords." default:"webui-secret"`
	AuthStaticAdminSecretKey  string `group:"auth" help:"Specify the secret key for the admin password." default:"admin-password"`
	AuthStaticViewerSecretKey string `group:"auth" help:"Specify the secret key for the viewer password." default:"viewer-password"`

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

	AuthLogoutURL         string `group:"auth" help:"Specify the logout URL, to which the user should be redirected after clearing the Kluctl Webui session."`
	AuthLogoutReturnParam string `group:"auth" help:"Specify the parameter name to pass to the logout redirect url, containing the return URL to redirect back."`
}

func (cmd *webuiRunCmd) Help() string {
	return `This command will run the Kluctl Webui. It can be run in two modes:
1. Locally, simply by invoking 'kluctl webui', which will run the Webui against the current Kubernetes context.
2. On a Kubernetes Cluster. See https://kluctl.io/docs/kluctl/installation/#installing-the-webui for details and documentation.
`
}

func (cmd *webuiRunCmd) buildAuthConfig(ctx context.Context, c client.Client) (webui.AuthConfig, error) {
	var authConfig webui.AuthConfig
	authConfig.AuthEnabled = cmd.InCluster

	authConfig.AuthSecretName = cmd.AuthSecretName
	authConfig.AuthSecretKey = cmd.AuthSecretKey

	authConfig.StaticLoginEnabled = cmd.AuthStaticLoginEnabled
	authConfig.StaticLoginSecretName = cmd.AuthStaticLoginSecretName
	authConfig.StaticAdminSecretKey = cmd.AuthStaticAdminSecretKey
	authConfig.StaticViewerSecretKey = cmd.AuthStaticViewerSecretKey

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
		authConfig.LogoutUrl = cmd.AuthLogoutURL
		authConfig.LogoutReturnParam = cmd.AuthLogoutReturnParam
	}

	return authConfig, nil
}

func (cmd *webuiRunCmd) Run(ctx context.Context) error {
	if !cmd.OnlyApi && !webui.IsWebUiBuildIncluded() {
		return fmt.Errorf("this build of Kluctl does not have the webui embedded")
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

	var inClusterConfig *rest.Config
	var inClusterClient client.Client
	if cmd.InCluster {
		configOverrides := &clientcmd.ConfigOverrides{
			CurrentContext: cmd.InClusterContext,
		}
		var err error
		inClusterConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			configOverrides).ClientConfig()
		if err != nil {
			return err
		}
		inClusterClient, err = client.NewWithWatch(inClusterConfig, client.Options{})
		if err != nil {
			return err
		}

		authConfig, err = cmd.buildAuthConfig(ctx, inClusterClient)
		if err != nil {
			return err
		}
	}

	stores, configs, err := createResultStores(ctx, cmd.Context, cmd.AllContexts, cmd.InCluster)
	if err != nil {
		return err
	}

	collector := results.NewResultsCollector(ctx, stores)
	collector.Start()

	server, err := webui.NewCommandResultsServer(ctx, collector, configs, inClusterConfig, inClusterClient, authConfig, cmd.OnlyApi)
	if err != nil {
		return err
	}
	return server.Run(cmd.Host, cmd.Port, isTerminal)
}
