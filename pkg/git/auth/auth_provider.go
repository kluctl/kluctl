package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_url "github.com/kluctl/kluctl/pkg/git/git-url"
)

type GitAuthProvider interface {
	BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod
}

type GitAuthProviders struct {
	authProviders []GitAuthProvider
}

func (a *GitAuthProviders) RegisterAuthProvider(p GitAuthProvider) {
	a.authProviders = append(a.authProviders, p)
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
	a.RegisterAuthProvider(&GitEnvAuthProvider{})
	a.RegisterAuthProvider(&GitCredentialsFileAuthProvider{})
	a.RegisterAuthProvider(&GitSshAuthProvider{})
	return a
}
