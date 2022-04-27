package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
)

type GitAuthProvider interface {
	BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod
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

func (a *GitAuthProviders) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	for _, p := range a.authProviders {
		auth := p.BuildAuth(gitUrl)
		if auth != nil {
			return auth
		}
	}
	return nil
}

func NewDefaultAuthProviders() *GitAuthProviders {
	a := &GitAuthProviders{}
	a.RegisterAuthProvider(&GitEnvAuthProvider{}, true)
	a.RegisterAuthProvider(&GitCredentialsFileAuthProvider{}, true)
	a.RegisterAuthProvider(&GitSshAuthProvider{}, true)
	return a
}
