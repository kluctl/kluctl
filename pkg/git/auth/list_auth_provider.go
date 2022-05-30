package auth

import (
	"context"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"strings"
)

type ListAuthProvider struct {
	entries []AuthEntry
}

type AuthEntry struct {
	Host       string
	PathPrefix string
	Username   string
	Password   string

	SshKey     []byte
	KnownHosts []byte

	CABundle []byte
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func (a *ListAuthProvider) BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA {
	for _, e := range a.entries {
		if e.Host != "*" && e.Host != gitUrl.Hostname() {
			continue
		}
		if !strings.HasPrefix(gitUrl.Path, e.PathPrefix) {
			continue
		}
		if e.Username == "" {
			continue
		}

		username := ""
		if gitUrl.User != nil {
			username = gitUrl.User.Username()
		}

		if username != "" && e.Username != "*" && username != e.Username {
			continue
		}

		if username == "" {
			username = e.Username
		}

		if username == "*" {
			// can't use "*" as username
			continue
		}

		if gitUrl.IsSsh() {
			if e.SshKey == nil {
				continue
			}
			a, err := ssh.NewPublicKeys(username, e.SshKey, "")
			if err != nil {
				status.Trace(ctx, "Failed to parse private key: %v", err)
			} else {
				a.HostKeyCallback = buildVerifyHostCallback(ctx, e.KnownHosts)
				return AuthMethodAndCA{
					AuthMethod: a,
				}
			}
		} else {
			if e.Password == "" {
				// empty password is not accepted
				continue
			}
			return AuthMethodAndCA{
				AuthMethod: &http.BasicAuth{
					Username: username,
					Password: e.Password,
				},
				CABundle: e.CABundle,
			}
		}
	}
	return AuthMethodAndCA{}
}
