package auth_provider

import (
	"context"
	"encoding/json"
	"github.com/docker/cli/cli/config/configfile"
	types2 "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"helm.sh/helm/v3/pkg/registry"
	"net/http"
	"os"
)

type AuthEntry struct {
	Registry string
	Repo     string

	AuthConfig authn.AuthConfig

	Key  []byte
	Cert []byte
	CA   []byte

	PlainHTTP             bool
	InsecureSkipTlsVerify bool
}

type OciAuthProvider interface {
	FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error)
}

type OciAuthProviders struct {
	authProviders []OciAuthProvider
}

func (a *OciAuthProviders) RegisterAuthProvider(p OciAuthProvider, last bool) {
	if last {
		a.authProviders = append(a.authProviders, p)
	} else {
		a.authProviders = append([]OciAuthProvider{p}, a.authProviders...)
	}
}

func (a *OciAuthProviders) FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error) {
	var errs *multierror.Error
	for _, p := range a.authProviders {
		auth, err := p.FindAuthEntry(ctx, ociUrl)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		if auth == nil {
			continue
		}
		return auth, nil
	}
	return nil, errs.ErrorOrNil()
}

func NewDefaultAuthProviders(envPrefix string) *OciAuthProviders {
	a := &OciAuthProviders{}
	a.RegisterAuthProvider(&OciEnvAuthProvider{Prefix: envPrefix}, true)
	a.RegisterAuthProvider(&OciDockerConfigAuthProvider{}, true)
	return a
}

func (a *AuthEntry) BuildHttpTransport() (*http.Transport, error) {
	tlsConfig, err := newTLSConfig(a.Cert, a.Key, a.CA, a.InsecureSkipTlsVerify)
	if err != nil {
		return nil, err
	}
	t := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	return t, nil
}

func (a *AuthEntry) BuildCraneOptions() ([]crane.Option, error) {
	var ret []crane.Option
	if a == nil {
		return ret, nil
	}
	if a.PlainHTTP {
		ret = append(ret, func(options *crane.Options) {
			options.Name = append(options.Name, name.Insecure)
		})
	}

	t, err := a.BuildHttpTransport()
	if err != nil {
		return nil, err
	}
	ret = append(ret, crane.WithTransport(t))

	if a.AuthConfig != (authn.AuthConfig{}) {
		ret = append(ret, crane.WithAuth(authn.FromConfig(a.AuthConfig)))
	}

	return ret, nil
}

func (a *AuthEntry) BuildDockerConfig(ctx context.Context, host string) (string, error) {
	cf := configfile.ConfigFile{
		AuthConfigs: map[string]types2.AuthConfig{
			host: {
				Username:      a.AuthConfig.Username,
				Password:      a.AuthConfig.Password,
				Auth:          a.AuthConfig.Auth,
				IdentityToken: a.AuthConfig.IdentityToken,
				RegistryToken: a.AuthConfig.RegistryToken,
			},
		},
	}
	cfStr, err := json.Marshal(&cf)
	if err != nil {
		return "", err
	}

	tmpFile, err := os.CreateTemp(utils.GetTmpBaseDir(ctx), "oci-config-*.json")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	_, err = tmpFile.Write(cfStr)
	if err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func (a *AuthEntry) BuildHelmRegistryOptions(ctx context.Context, host string) ([]registry.ClientOption, func(), error) {
	var ret []registry.ClientOption

	cleanup := func() {}

	if a.AuthConfig != (authn.AuthConfig{}) {
		cf, err := a.BuildDockerConfig(ctx, host)
		if err != nil {
			return nil, nil, err
		}
		cleanup = func() {
			_ = os.Remove(cf)
		}
		ret = append(ret, registry.ClientOptCredentialsFile(cf))
	}

	t, err := a.BuildHttpTransport()
	if err != nil {
		cleanup()
		return nil, nil, err
	}

	hc := &http.Client{
		Transport: t,
	}
	ret = append(ret, registry.ClientOptHTTPClient(hc))

	if a.PlainHTTP {
		ret = append(ret, registry.ClientOptPlainHTTP())
	}

	return ret, cleanup, nil
}
