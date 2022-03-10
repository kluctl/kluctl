package auth

import (
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	"strings"
)

type GitEnvAuthProvider struct {
}

func (a *GitEnvAuthProvider) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	for _, m := range utils.ParseEnvConfigSets("KLUCTL_GIT") {
		host, _ := m["HOST"]
		pathPrefix, _ := m["PATH_PREFIX"]
		username, _ := m["USERNAME"]
		password, _ := m["PASSWORD"]
		ssh_key, _ := m["SSH_KEY"]

		if host != gitUrl.Hostname() {
			continue
		}
		if !strings.HasPrefix(gitUrl.Path, pathPrefix) {
			continue
		}
		if username == "" {
			continue
		}
		if gitUrl.User != nil && gitUrl.User.Username() != "" && gitUrl.User.Username() != username {
			continue
		}

		if !gitUrl.IsSsh() && password != "" {
			return &http.BasicAuth{
				Username: username,
				Password: password,
			}
		} else if gitUrl.IsSsh() && ssh_key != "" {
			a, err := ssh.NewPublicKeysFromFile(username, ssh_key, "")
			if err != nil {
				log.Debugf("Failed to parse private key %s: %v", ssh_key, err)
			} else {
				return a
			}
		}
	}
	return nil
}