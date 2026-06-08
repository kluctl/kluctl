package git

import (
	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/storage/filesystem"
)

func FastSharedClone(sourceDir string, targetDir string, coOptions *git.CheckoutOptions) error {
	fs := osfs.New(targetDir, osfs.WithBoundOS())
	gitFs, err := fs.Chroot(".git")
	if err != nil {
		return err
	}
	s := filesystem.NewStorageWithOptions(gitFs, cache.NewObjectLRUDefault(), filesystem.Options{})
	defer s.Close()

	r, err := git.Clone(s, fs, &git.CloneOptions{
		URL:        sourceDir,
		Shared:     true,
		NoCheckout: true,
	})
	if err != nil {
		return err
	}
	defer r.Close()

	err = r.DeleteRemote("origin")
	if err != nil {
		return err
	}

	sourceRepo, err := git.PlainOpen(sourceDir)
	if err != nil {
		return err
	}
	defer sourceRepo.Close()
	remotes, err := sourceRepo.Remotes()
	if err != nil {
		return err
	}
	for _, remote := range remotes {
		_, err = r.CreateRemote(remote.Config())
		if err != nil {
			return err
		}
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
