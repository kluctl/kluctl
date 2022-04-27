package auth

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
)

type GitEnvAuthProvider struct {
}

func (a *GitEnvAuthProvider) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	var la ListAuthProvider

	for _, m := range utils.ParseEnvConfigSets("KLUCTL_GIT") {
		e := AuthEntry{
			Host:       m["HOST"],
			PathPrefix: m["PATH_PREFIX"],
			Username:   m["USERNAME"],
			Password:   m["PASSWORD"],
		}

		ssh_key_path, _ := m["SSH_KEY"]
		if ssh_key_path != "" {
			ssh_key_path = utils.ExpandPath(ssh_key_path)
			b, err := ioutil.ReadFile(ssh_key_path)
			if err != nil {
				log.Debugf("Failed to read key %s: %v", ssh_key_path, err)
			} else {
				e.SshKey = b
			}
		}
		la.AddEntry(e)
	}
	return la.BuildAuth(gitUrl)
}
