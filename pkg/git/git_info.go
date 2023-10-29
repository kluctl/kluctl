package git

import (
	"context"
	git2 "github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"os"
	"path/filepath"
)

func BuildGitInfo(ctx context.Context, repoRoot string, projectDir string) (result.GitInfo, result.ProjectKey, error) {
	var gitInfo result.GitInfo
	var projectKey result.ProjectKey
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

	head, err := g.Head()
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

	var repoKey types.RepoKey
	if originUrl != nil {
		repoKey = originUrl.RepoKey()
	}

	gitInfo = result.GitInfo{
		Url:    originUrl,
		SubDir: subDir,
		Commit: head.Hash().String(),
		Dirty:  !s.IsClean(),
	}
	if head.Name().IsBranch() {
		gitInfo.Ref = &types.GitRef{
			Branch: head.Name().Short(),
		}
	} else if head.Name().IsTag() {
		gitInfo.Ref = &types.GitRef{
			Tag: head.Name().Short(),
		}
	}
	projectKey.RepoKey = repoKey
	projectKey.SubDir = subDir
	return gitInfo, projectKey, nil
}
