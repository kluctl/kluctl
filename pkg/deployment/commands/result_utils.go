package commands

import (
	"fmt"
	git2 "github.com/go-git/go-git/v5"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"time"
)

func addBaseCommandInfoToResult(targetCtx *kluctl_project.TargetContext, r *result.CommandResult, command string) error {
	r.Target = targetCtx.Target
	r.Command = result.CommandInfo{
		StartTime: metav1.NewTime(targetCtx.KluctlProject.LoadTime),
		EndTime:   metav1.Now(),
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

	err = addClusterInfo(targetCtx.SharedContext.K, r)
	if err != nil {
		return err
	}

	r.TargetKey.TargetName = targetCtx.Target.Name
	r.TargetKey.Discriminator = targetCtx.Target.Discriminator
	r.TargetKey.ClusterId = r.ClusterInfo.ClusterId

	return nil
}

func addDeleteCommandInfoToResult(r *result.CommandResult, k *k8s2.K8sCluster, startTime time.Time, inclusion *utils.Inclusion) error {
	r.Command = result.CommandInfo{
		StartTime: metav1.NewTime(startTime),
		EndTime:   metav1.Now(),
		Command:   "delete",
	}

	r.Command.IncludeTags = inclusion.GetIncludes("tags")
	r.Command.ExcludeTags = inclusion.GetExcludes("tags")
	r.Command.IncludeDeploymentDirs = inclusion.GetIncludes("deploymentItemDir")
	r.Command.ExcludeDeploymentDirs = inclusion.GetExcludes("deploymentItemDir")

	err := addClusterInfo(k, r)
	if err != nil {
		return err
	}

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
		if err == git2.ErrRepositoryNotExists {
			return nil
		}
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

	var repoKey types.GitRepoKey
	if originUrl != nil {
		repoKey = originUrl.RepoKey()
	}

	r.GitInfo = result.GitInfo{
		Url:    originUrl,
		SubDir: subDir,
		Commit: head.Hash().String(),
		Dirty:  !s.IsClean(),
	}
	if head.Name().IsBranch() {
		r.GitInfo.Ref = &types.GitRef{
			Branch: head.Name().Short(),
		}
	} else if head.Name().IsTag() {
		r.GitInfo.Ref = &types.GitRef{
			Tag: head.Name().Short(),
		}
	}
	r.ProjectKey.GitRepoKey = repoKey
	r.ProjectKey.SubDir = subDir
	return nil
}

func addClusterInfo(k *k8s2.K8sCluster, r *result.CommandResult) error {
	kubeSystemNs, _, err := k.GetSingleObject(
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
