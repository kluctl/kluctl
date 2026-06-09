package auth

import (
	"context"
	"strings"

	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/lib/git/messages"
	"github.com/kluctl/kluctl/lib/git/types"
	ssh2 "golang.org/x/crypto/ssh"
)

type ListAuthProvider struct {
	MessageCallbacks messages.MessageCallbacks

	entries []AuthEntry
}

type AuthEntry struct {
	AllowWildcardHostForHttp bool

	Host     string
	PathGlob glob.Glob
	PathStr  string

	Username string
	Password string

	SshKey           []byte
	KnownHosts       []byte
	IgnoreKnownHosts bool

	CABundle []byte
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func (a *ListAuthProvider) BuildAuth(ctx context.Context, gitUrlIn types.GitUrl) (*AuthMethodAndCA, error) {
	gitUrl := gitUrlIn.Normalize()

	a.MessageCallbacks.Trace("ListAuthProvider: BuildAuth for %s", gitUrl.String())
	a.MessageCallbacks.Trace("ListAuthProvider: path=%s, username=%s, scheme=%s", gitUrl.Path, gitUrl.User.Username(), gitUrl.Scheme)

	for _, e := range a.entries {
		a.MessageCallbacks.Trace("ListAuthProvider: try host=%s, path=%s, username=%s", e.Host, e.PathStr, e.Username)

		if !e.AllowWildcardHostForHttp && e.Host == "*" && !gitUrl.IsSsh() {
			a.MessageCallbacks.Trace("ListAuthProvider: wildcard hosts are not allowed for http urls")
			continue
		}

		if e.Host != "*" && e.Host != gitUrl.Host {
			continue
		}
		if e.PathGlob != nil {
			urlPath := strings.TrimPrefix(gitUrl.Path, "/")
			if !e.PathGlob.Match(urlPath) {
				continue
			}
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
				a.MessageCallbacks.Trace("ListAuthProvider: empty ssh key is not accepted")
				continue
			}
			a.MessageCallbacks.Trace("ListAuthProvider: using username+sshKey")
			pk, err := ssh.NewPublicKeys(username, e.SshKey, "")
			if err != nil {
				a.MessageCallbacks.Trace("ListAuthProvider: failed to parse private key: %v", err)
			} else {
				pk.HostKeyCallback = buildVerifyHostCallback(a.MessageCallbacks, e.KnownHosts, e.IgnoreKnownHosts)
				return &AuthMethodAndCA{
					ClientOptions: []client.Option{
						client.WithSSHAuth(pk),
					},
					BuildSSHClientConfig: func(ctx context.Context) (*ssh2.ClientConfig, error) {
						return pk.ClientConfig(ctx, nil)
					},
					Hash: func() ([]byte, error) {
						return buildHash(pk.Signer)
					},
				}, nil
			}
		} else {
			if e.Password == "" {
				a.MessageCallbacks.Trace("ListAuthProvider: empty password is not accepted")
				continue
			}
			a.MessageCallbacks.Trace("ListAuthProvider: using username+password")
			auth := &AuthMethodAndCA{
				ClientOptions: []client.Option{
					client.WithHTTPAuth(&http.BasicAuth{
						Username: username,
						Password: e.Password,
					}),
				},
			}
			if len(e.CABundle) != 0 {
				auth.ClientOptions = append(auth.ClientOptions, client.WithCABundle(e.CABundle))
			}
			return auth, nil
		}
	}
	return nil, nil
}
