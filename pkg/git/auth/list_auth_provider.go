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
	status.Trace(ctx, "ListAuthProvider: BuildAuth for %s", gitUrl.String())
	status.Trace(ctx, "ListAuthProvider: path=%s, username=%s, scheme=%s", gitUrl.Path, gitUrl.User.Username(), gitUrl.Scheme)
	for _, e := range a.entries {
		status.Trace(ctx, "ListAuthProvider: try host=%s, pathPrefix=%s, username=%s", e.Host, e.PathPrefix, e.Username)

		if e.Host != "*" && e.Host != gitUrl.Hostname() {
			continue
		}
		urlPath := gitUrl.Path
		if strings.HasPrefix(urlPath, "/") {
			urlPath = urlPath[1:]
		}
		if !strings.HasPrefix(urlPath, e.PathPrefix) {
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
				status.Trace(ctx, "ListAuthProvider: empty ssh key is not accepted")
				continue
			}
			status.Trace(ctx, "ListAuthProvider: using username+sshKey")
			a, err := ssh.NewPublicKeys(username, e.SshKey, "")
			if err != nil {
				status.Trace(ctx, "ListAuthProvider: failed to parse private key: %v", err)
			} else {
				a.HostKeyCallback = buildVerifyHostCallback(ctx, e.KnownHosts)
				return AuthMethodAndCA{
					AuthMethod: a,
				}
			}
		} else {
			if e.Password == "" {
				status.Trace(ctx, "ListAuthProvider: empty password is not accepted")
				continue
			}
			status.Trace(ctx, "ListAuthProvider: using username+password")
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
