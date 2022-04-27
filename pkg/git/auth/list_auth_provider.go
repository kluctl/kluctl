package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	log "github.com/sirupsen/logrus"
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
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func (a *ListAuthProvider) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
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

		if e.Username == "*" {
			// can't use "*" as username
			continue
		}

		if gitUrl.IsSsh() {
			if e.SshKey == nil {
				continue
			}
			a, err := ssh.NewPublicKeys(username, e.SshKey, "")
			if err != nil {
				log.Debugf("Failed to parse private key: %v", err)
			} else {
				a.HostKeyCallback = verifyHost
				return a
			}
		} else {
			if e.Password == "" {
				// empty password is not accepted
				continue
			}
			return &http.BasicAuth{
				Username: username,
				Password: e.Password,
			}
		}
	}
	return nil
}
