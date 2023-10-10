package auth_provider

import (
	"context"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/hashicorp/go-multierror"
)

type OciAuthInfo struct {
	Authenticator authn.Authenticator

	Insecure bool
}

type OciAuthProvider interface {
	Login(ctx context.Context, ociUrl string) (*OciAuthInfo, error)
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

func (a *OciAuthProviders) Login(ctx context.Context, ociUrl string) (*OciAuthInfo, error) {
	var errs *multierror.Error
	for _, p := range a.authProviders {
		auth, err := p.Login(ctx, ociUrl)
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

func (a *OciAuthInfo) BuildCraneOptions() []crane.Option {
	var ret []crane.Option
	if a == nil {
		return ret
	}
	if a.Insecure {
		ret = append(ret, crane.Insecure)
	}
	if a.Authenticator != nil {
		ret = append(ret, crane.WithAuth(a.Authenticator))
	}
	return ret
}
