package git

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"os"
	"path/filepath"
)

type CheckoutInfo struct {
	CheckedOutRef    string `json:"checkedOutRef"`
	CheckedOutCommit string `json:"checkedOutCommit"`
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
		st, err := os.Stat(filepath.Join(path, ".git"))
		if err == nil && st.IsDir() {
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
