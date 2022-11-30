package auth

import (
	"context"
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	"golang.org/x/crypto/ssh"
)

type AuthMethodAndCA struct {
	AuthMethod transport.AuthMethod
	CABundle   []byte

	Hash         func() ([]byte, error)
	ClientConfig func() (*ssh.ClientConfig, error)
}

type GitAuthProvider interface {
	BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA
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

func (a *GitAuthProviders) BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA {
	for _, p := range a.authProviders {
		auth := p.BuildAuth(ctx, gitUrl)
		if auth.AuthMethod != nil {
			return auth
		}
	}
	return AuthMethodAndCA{}
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
