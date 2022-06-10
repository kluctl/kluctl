package repoprovider

import (
	"github.com/kluctl/kluctl/v2/pkg/git"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
)

type RepoInfo struct {
	Url        git_url.GitUrl    `yaml:"url"`
	RemoteRefs map[string]string `yaml:"remoteRefs"`
	DefaultRef string            `yaml:"defaultRef"`
}

type RepoProvider interface {
	GetRepoInfo(url git_url.GitUrl) (RepoInfo, error)
	GetClonedDir(url git_url.GitUrl, ref string) (string, git.CheckoutInfo, error)
	UnlockAll()
	Clear()
}
