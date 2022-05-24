package git

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"path/filepath"
)

type CheckoutInfo struct {
	CheckedOutRef    string `yaml:"checkedOutRef"`
	CheckedOutCommit string `yaml:"checkedOutCommit"`
}

func GetCheckoutInfo(path string) (ri CheckoutInfo, err error) {
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

func DetectGitRepositoryRoot(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for true {
		if utils.Exists(filepath.Join(path, ".git")) {
			break
		}
		old := path
		path = filepath.Dir(path)
		if old == path {
			return "", fmt.Errorf("could not detect git repository root")
		}
	}
	return path, nil
}
