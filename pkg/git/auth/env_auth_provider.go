package auth

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"sync"
)

type GitEnvAuthProvider struct {
	MessageCallbacks messages.MessageCallbacks

	Prefix string

	mutext  sync.Mutex
	list    *ListAuthProvider
	listErr error
}

func (a *GitEnvAuthProvider) buildList(ctx context.Context) error {
	a.mutext.Lock()
	defer a.mutext.Unlock()
	if a.listErr != nil {
		return a.listErr
	}
	if a.list != nil {
		return nil
	}
	a.listErr = a.doBuildList(ctx)
	return a.listErr
}

func (a *GitEnvAuthProvider) doBuildList(ctx context.Context) error {
	a.list = &ListAuthProvider{}

	for _, s := range utils.ParseEnvConfigSets(a.Prefix) {
		m := s.Map
		e := AuthEntry{
			Host:     m["HOST"],
			Username: m["USERNAME"],
			Password: m["PASSWORD"],
		}
		if e.Host == "" {
			continue
		}

		path, ok := m["PATH"]
		if !ok {
			path, ok = m["PATH_PREFIX"]
			if ok {
				status.Deprecation(ctx, "git-prefix-path", "The environment variable KLUCTL_GIT_PREFIX_PATH is deprecated and support for it will be removed in a future Kluctl version. Please switch to KLUCTL_GIT_PATH with wildcards instead.")
				path += "**"
			}
		}
		if path != "" {
			e.PathStr = path
			g, err := glob.Compile(e.PathStr, '/')
			if err != nil {
				return err
			}
			e.PathGlob = g
		}

		ssh_key_path, _ := m["SSH_KEY"]

		a.MessageCallbacks.Trace(fmt.Sprintf("GitEnvAuthProvider: adding entry host=%s, path=%s, username=%s, ssh_key=%s", e.Host, e.PathStr, e.Username, ssh_key_path))

		if ssh_key_path != "" {
			ssh_key_path = expandHomeDir(ssh_key_path)
			b, err := os.ReadFile(ssh_key_path)
			if err != nil {
				a.MessageCallbacks.Trace(fmt.Sprintf("GitEnvAuthProvider: failed to read key %s: %v", ssh_key_path, err))
			} else {
				e.SshKey = b
			}
		}
		ca_bundle_path := m["CA_BUNDLE"]
		if ca_bundle_path != "" {
			ca_bundle_path = expandHomeDir(ca_bundle_path)
			b, err := os.ReadFile(ca_bundle_path)
			if err != nil {
				a.MessageCallbacks.Trace(fmt.Sprintf("GitEnvAuthProvider: failed to read ca bundle %s: %v", ca_bundle_path, err))
			} else {
				e.CABundle = b
			}
		}
		a.list.AddEntry(e)
	}

	return nil
}

func (a *GitEnvAuthProvider) BuildAuth(ctx context.Context, gitUrl types.GitUrl) (AuthMethodAndCA, error) {
	err := a.buildList(ctx)
	if err != nil {
		return AuthMethodAndCA{}, err
	}
	return a.list.BuildAuth(ctx, gitUrl)
}
