package auth

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
)

type CleanupFunc func()

var cleanupNoop = func() {}

type HelmAuthProvider interface {
	FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error)
}

type HelmAuthProviders struct {
	authProviders []HelmAuthProvider
}

func (a *HelmAuthProviders) RegisterAuthProvider(p HelmAuthProvider, last bool) {
	if last {
		a.authProviders = append(a.authProviders, p)
	} else {
		a.authProviders = append([]HelmAuthProvider{p}, a.authProviders...)
	}
}

func (a *HelmAuthProviders) FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error) {
	var errs *multierror.Error
	for _, p := range a.authProviders {
		auth, cf, err := p.FindAuthEntry(ctx, repoUrl, credentialsId)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		if auth == nil {
			continue
		}
		return auth, cf, nil
	}
	return nil, cleanupNoop, errs.ErrorOrNil()
}

func NewDefaultAuthProviders(envPrefix string) *HelmAuthProviders {
	a := &HelmAuthProviders{}
	a.RegisterAuthProvider(&HelmEnvAuthProvider{Prefix: envPrefix}, true)
	a.RegisterAuthProvider(&HelmConfigAuthProvider{}, true)
	return a
}
