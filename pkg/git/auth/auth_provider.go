package auth

import (
	git_url "github.com/kluctl/kluctl/pkg/git/git-url"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

type GitAuthProvider interface {
	BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod
}

var authProviders []GitAuthProvider

func RegisterAuthProvider(p GitAuthProvider) {
	authProviders = append(authProviders, p)
}

func BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	for _, p := range authProviders {
		auth := p.BuildAuth(gitUrl)
		if auth != nil {
			return auth
		}
	}
	return nil
}

func init() {
	RegisterAuthProvider(&GitEnvAuthProvider{})
	RegisterAuthProvider(&GitCredentialsFileAuthProvider{})
	RegisterAuthProvider(&GitSshAuthProvider{})
}
