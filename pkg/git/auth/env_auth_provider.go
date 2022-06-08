package auth

import (
	"context"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"io/ioutil"
)

type GitEnvAuthProvider struct {
}

func (a *GitEnvAuthProvider) BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA {
	var la ListAuthProvider

	for _, m := range utils.ParseEnvConfigSets("KLUCTL_GIT") {
		e := AuthEntry{
			Host:       m["HOST"],
			PathPrefix: m["PATH_PREFIX"],
			Username:   m["USERNAME"],
			Password:   m["PASSWORD"],
		}
		if e.Host == "" {
			continue
		}

		ssh_key_path, _ := m["SSH_KEY"]

		status.Trace(ctx, "GitEnvAuthProvider: adding entry host=%s, pathPrefix=%s, username=%s, ssh_key=%s", e.Host, e.PathPrefix, e.Username, ssh_key_path)

		if ssh_key_path != "" {
			ssh_key_path = utils.ExpandPath(ssh_key_path)
			b, err := ioutil.ReadFile(ssh_key_path)
			if err != nil {
				status.Trace(ctx, "GitEnvAuthProvider: failed to read key %s: %v", ssh_key_path, err)
			} else {
				e.SshKey = b
			}
		}
		ca_bundle_path := m["CA_BUNDLE"]
		if ca_bundle_path != "" {
			ca_bundle_path = utils.ExpandPath(ca_bundle_path)
			b, err := ioutil.ReadFile(ca_bundle_path)
			if err != nil {
				status.Trace(ctx, "GitEnvAuthProvider: failed to read ca bundle %s: %v", ca_bundle_path, err)
			} else {
				e.CABundle = b
			}
		}
		la.AddEntry(e)
	}
	return la.BuildAuth(ctx, gitUrl)
}
