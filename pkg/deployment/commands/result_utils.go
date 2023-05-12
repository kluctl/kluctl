package commands

import (
	"fmt"
	git2 "github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"path/filepath"
	"time"
)

func addBaseCommandInfoToResult(targetCtx *kluctl_project.TargetContext, r *result.CommandResult, command string) error {
	r.Target = targetCtx.Target
	r.Command = result.CommandInfo{
		StartTime: types.FromTime(targetCtx.KluctlProject.LoadTime),
		EndTime:   types.FromTime(time.Now()),
		Command:   command,
		Args:      targetCtx.KluctlProject.LoadArgs.ExternalArgs,
	}
	r.Command.TargetNameOverride = targetCtx.Params.TargetNameOverride
	r.Command.ContextOverride = targetCtx.Params.ContextOverride
	r.Command.Images = targetCtx.Params.Images.FixedImages()
	r.Command.IncludeTags = targetCtx.Params.Inclusion.GetIncludes("tags")
	r.Command.ExcludeTags = targetCtx.Params.Inclusion.GetExcludes("tags")
	r.Command.IncludeDeploymentDirs = targetCtx.Params.Inclusion.GetIncludes("deploymentItemDir")
	r.Command.ExcludeDeploymentDirs = targetCtx.Params.Inclusion.GetExcludes("deploymentItemDir")
	r.Command.DryRun = targetCtx.Params.DryRun

	r.Deployment = &targetCtx.DeploymentProject.Config

	err := addGitInfo(targetCtx, r)
	if err != nil {
		return err
	}

	err = addClusterInfo(targetCtx, r)
	if err != nil {
		return err
	}

	r.TargetKey.TargetName = targetCtx.Target.Name
	r.TargetKey.Discriminator = targetCtx.Target.Discriminator
	r.TargetKey.ClusterId = r.ClusterInfo.ClusterId
	return nil
}

func addGitInfo(targetCtx *kluctl_project.TargetContext, r *result.CommandResult) error {
	if targetCtx.KluctlProject.LoadArgs.RepoRoot == "" {
		return nil
	}

	projectDirAbs, err := filepath.Abs(targetCtx.KluctlProject.LoadArgs.ProjectDir)
	if err != nil {
		return err
	}

	subDir, err := filepath.Rel(targetCtx.KluctlProject.LoadArgs.RepoRoot, projectDirAbs)
	if err != nil {
		return err
	}
	if subDir == "." {
		subDir = ""
	}

	g, err := git2.PlainOpen(targetCtx.KluctlProject.LoadArgs.RepoRoot)
	if err != nil {
		return err
	}

	w, err := g.Worktree()
	if err != nil {
		return err
	}

	s, err := w.Status()
	if err != nil {
		return err
	}

	head, err := g.Head()
	if err != nil {
		return err
	}

	remotes, err := g.Remotes()
	if err != nil {
		return err
	}

	var originUrl *types.GitUrl
	for _, r := range remotes {
		if r.Config().Name == "origin" {
			originUrl, err = types.ParseGitUrl(r.Config().URLs[0])
			if err != nil {
				return err
			}
		}
	}

	var normaliedUrl string
	if originUrl != nil {
		normaliedUrl = originUrl.NormalizedRepoKey()
	}

	r.GitInfo = &result.GitInfo{
		Url:    originUrl,
		Ref:    head.Name().String(),
		SubDir: subDir,
		Commit: head.Hash().String(),
		Dirty:  !s.IsClean(),
	}
	r.ProjectKey.NormalizedGitUrl = normaliedUrl
	r.ProjectKey.SubDir = subDir
	return nil
}

func addClusterInfo(targetCtx *kluctl_project.TargetContext, r *result.CommandResult) error {
	kubeSystemNs, _, err := targetCtx.SharedContext.K.GetSingleObject(
		k8s.NewObjectRef("", "v1", "Namespace", "kube-system", ""))
	if err != nil {
		return err
	}
	// we reuse the kube-system namespace uid as global cluster id
	clusterId := kubeSystemNs.GetK8sUid()
	if clusterId == "" {
		return fmt.Errorf("kube-system namespace has no uid")
	}
	r.ClusterInfo = result.ClusterInfo{
		ClusterId: clusterId,
	}
	return nil
}
