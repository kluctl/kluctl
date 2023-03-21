package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	cp "github.com/otiai10/copy"
	"os"
	"path/filepath"
	"runtime"
)

// PoorMansCloneCommit poor mans clone from a local repo, which does not rely on go-git using git-upload-pack
func PoorMansClone(sourceDir string, targetDir string, coOptions *git.CheckoutOptions) error {
	err := os.MkdirAll(targetDir, 0o700)
	if err != nil {
		return err
	}
	err = os.Mkdir(filepath.Join(targetDir, ".git"), 0o700)
	if err != nil {
		return err
	}
	des, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, de := range des {
		s := filepath.Join(sourceDir, de.Name())
		d := filepath.Join(targetDir, ".git", de.Name())
		if de.Name() == ".cache.lock" {
			continue
		}
		if de.Name() == "objects" {
			err = os.Symlink(s, d)
			if err != nil && runtime.GOOS == "windows" {
				// Windows 10 does not support symlinks as unprivileged users, so we revert to deep copying
				err = cp.Copy(s, d, cp.Options{OnSymlink: func(src string) cp.SymlinkAction {
					return cp.Deep
				}})
			}
		} else {
			err = cp.Copy(s, d)
		}
		if err != nil {
			return err
		}
	}

	gitConfigReader, err := os.Open(filepath.Join(targetDir, ".git", "config"))
	if err != nil {
		return err
	}
	defer gitConfigReader.Close()
	gitConfig, err := config.ReadConfig(gitConfigReader)
	if err != nil {
		return err
	}
	gitConfig.Core.IsBare = false

	b, err := gitConfig.Marshal()
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(targetDir, ".git", "config"), b, 0o600)
	if err != nil {
		return err
	}

	r, err := git.PlainOpen(targetDir)
	if err != nil {
		return err
	}

	wt, err := r.Worktree()
	if err != nil {
		return err
	}

	err = wt.Checkout(coOptions)
	if err != nil {
		return err
	}
	return nil
}
