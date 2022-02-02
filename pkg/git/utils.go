package git

import (
	"github.com/go-git/go-git/v5"
)

type GitRepoInfo struct {
	CheckedOutRef    string
	CheckedOutCommit string
}

func GetGitRepoInfo(path string) (ri GitRepoInfo, err error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return
	}
	head, err := r.Head()
	if err != nil {
		return
	}
	ri.CheckedOutRef = head.Name().String()
	ri.CheckedOutCommit = head.Hash().String()
	return
}
