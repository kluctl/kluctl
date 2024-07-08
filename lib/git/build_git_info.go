package git

import (
	"context"
	"errors"
	git2 "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/kluctl/kluctl/lib/git/types"
	"os"
	"path/filepath"
)

func BuildGitInfo(ctx context.Context, repoRoot string, projectDir string) (types.GitInfo, types.ProjectKey, error) {
	var gitInfo types.GitInfo
	var projectKey types.ProjectKey
	if repoRoot == "" {
		return gitInfo, projectKey, nil
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); os.IsNotExist(err) {
		return gitInfo, projectKey, nil
	}

	projectDirAbs, err := filepath.Abs(projectDir)
	if err != nil {
		return gitInfo, projectKey, err
	}

	subDir, err := filepath.Rel(repoRoot, projectDirAbs)
	if err != nil {
		return gitInfo, projectKey, err
	}
	if subDir == "." {
		subDir = ""
	}

	g, err := git2.PlainOpen(repoRoot)
	if err != nil {
		return gitInfo, projectKey, err
	}

	s, err := GetWorktreeStatus(ctx, repoRoot)
	if err != nil {
		return gitInfo, projectKey, err
	}

	remotes, err := g.Remotes()
	if err != nil {
		return gitInfo, projectKey, err
	}

	var originUrl *types.GitUrl
	for _, r := range remotes {
		if r.Config().Name == "origin" {
			originUrl, err = types.ParseGitUrl(r.Config().URLs[0])
			if err != nil {
				return gitInfo, projectKey, err
			}
		}
	}

	gitInfo = types.GitInfo{
		Url:    originUrl,
		SubDir: subDir,
		Dirty:  !s.IsClean(),
	}

	head, err := g.Head()
	if err == nil {
		gitInfo.Commit = head.Hash().String()
		if head.Name().IsBranch() {
			gitInfo.Ref = &types.GitRef{
				Branch: head.Name().Short(),
			}
		} else if head.Name().IsTag() {
			gitInfo.Ref = &types.GitRef{
				Tag: head.Name().Short(),
			}
		}
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return gitInfo, projectKey, err
	}

	if originUrl != nil {
		projectKey.RepoKey = originUrl.RepoKey()
	}
	projectKey.SubDir = subDir

	return gitInfo, projectKey, nil
}
