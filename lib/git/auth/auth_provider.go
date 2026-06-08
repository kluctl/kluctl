package auth

import (
	"context"

	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/lib/git/messages"
	"github.com/kluctl/kluctl/lib/git/types"
	"golang.org/x/crypto/ssh"
)

type AuthMethodAndCA struct {
	ClientOptions []client.Option

	BuildSSHClientConfig func(ctx context.Context) (*ssh.ClientConfig, error)

	Hash func() ([]byte, error)
}

type GitAuthProvider interface {
	BuildAuth(ctx context.Context, gitUrl types.GitUrl) (*AuthMethodAndCA, error)
}

type GitAuthProviders struct {
	authProviders []GitAuthProvider
}

func (a *GitAuthProviders) RegisterAuthProvider(p GitAuthProvider, last bool) {
	if last {
		a.authProviders = append(a.authProviders, p)
	} else {
		a.authProviders = append([]GitAuthProvider{p}, a.authProviders...)
	}
}

func (a *GitAuthProviders) BuildAuth(ctx context.Context, gitUrl types.GitUrl) (*AuthMethodAndCA, error) {
	var errs *multierror.Error
	for _, p := range a.authProviders {
		auth, err := p.BuildAuth(ctx, gitUrl)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		if auth != nil {
			return auth, nil
		}
	}
	return nil, errs.ErrorOrNil()
}

func NewDefaultAuthProviders(envPrefix string, messageCallbacks *messages.MessageCallbacks) *GitAuthProviders {
	if messageCallbacks == nil {
		messageCallbacks = &messages.MessageCallbacks{}
	}

	a := &GitAuthProviders{}
	a.RegisterAuthProvider(&GitEnvAuthProvider{MessageCallbacks: *messageCallbacks, Prefix: envPrefix}, true)
	a.RegisterAuthProvider(&GitCredentialsFileAuthProvider{MessageCallbacks: *messageCallbacks}, true)
	a.RegisterAuthProvider(&GitSshAuthProvider{MessageCallbacks: *messageCallbacks}, true)
	return a
}
