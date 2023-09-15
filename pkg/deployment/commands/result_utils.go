package commands

import (
	git2 "github.com/go-git/go-git/v5"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/git"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"time"
)

func newCommandResult(targetCtx *kluctl_project.TargetContext, startTime time.Time, command string) *result.CommandResult {
	r := &result.CommandResult{}

	r.Target = targetCtx.Target
	r.Command = result.CommandInfo{
		StartTime: metav1.NewTime(startTime),
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

	var err error
	r.GitInfo, r.ProjectKey, err = buildGitInfo(targetCtx)
	if err != nil {
		r.Errors = append(r.Errors, result.DeploymentError{
			Message: err.Error(),
		})
	}

	r.ClusterInfo, err = buildClusterInfo(targetCtx.SharedContext.K)
	if err != nil {
		r.Errors = append(r.Errors, result.DeploymentError{
			Message: err.Error(),
		})
	}

	r.TargetKey.TargetName = targetCtx.Target.Name
	r.TargetKey.Discriminator = targetCtx.Target.Discriminator
	r.TargetKey.ClusterId = r.ClusterInfo.ClusterId

	return r
}

func newValidateCommandResult(targetCtx *kluctl_project.TargetContext, startTime time.Time) *result.ValidateResult {
	r := &result.ValidateResult{}

	r.StartTime = metav1.NewTime(startTime)
	r.EndTime = metav1.Now()
	var err error
	_, r.ProjectKey, err = buildGitInfo(targetCtx)
	if err != nil {
		r.Errors = append(r.Errors, result.DeploymentError{
			Message: err.Error(),
		})
	}

	clusterInfo, err := buildClusterInfo(targetCtx.SharedContext.K)
	if err != nil {
		r.Errors = append(r.Errors, result.DeploymentError{
			Message: err.Error(),
		})
	}

	r.TargetKey.TargetName = targetCtx.Target.Name
	r.TargetKey.Discriminator = targetCtx.Target.Discriminator
	r.TargetKey.ClusterId = clusterInfo.ClusterId

	return r
}

func newDeleteCommandResult(k *k8s2.K8sCluster, startTime time.Time, inclusion *utils.Inclusion) *result.CommandResult {
	r := &result.CommandResult{}

	r.Command = result.CommandInfo{
		StartTime: metav1.NewTime(startTime),
		Command:   "delete",
	}

	r.Command.IncludeTags = inclusion.GetIncludes("tags")
	r.Command.ExcludeTags = inclusion.GetExcludes("tags")
	r.Command.IncludeDeploymentDirs = inclusion.GetIncludes("deploymentItemDir")
	r.Command.ExcludeDeploymentDirs = inclusion.GetExcludes("deploymentItemDir")

	var err error
	r.ClusterInfo, err = buildClusterInfo(k)
	if err != nil {
		r.Errors = append(r.Errors, result.DeploymentError{
			Message: err.Error(),
		})
	}

	return r
}

func finishCommandResult(r *result.CommandResult, targetCtx *kluctl_project.TargetContext, dew *utils2.DeploymentErrorsAndWarnings) {
	r.Errors = append(r.Errors, dew.GetErrorsList()...)
	r.Warnings = append(r.Warnings, dew.GetWarningsList()...)
	if targetCtx != nil {
		r.SeenImages = targetCtx.DeploymentCollection.Images.SeenImages(false)
	}
	r.Command.EndTime = metav1.Now()
}

func finishValidateResult(r *result.ValidateResult, targetCtx *kluctl_project.TargetContext, dew *utils2.DeploymentErrorsAndWarnings) {
	r.Errors = append(r.Errors, dew.GetErrorsList()...)
	r.Warnings = append(r.Warnings, dew.GetWarningsList()...)
	r.EndTime = metav1.Now()
}

func buildGitInfo(targetCtx *kluctl_project.TargetContext) (result.GitInfo, result.ProjectKey, error) {
	var gitInfo result.GitInfo
	var projectKey result.ProjectKey
	if targetCtx.KluctlProject.LoadArgs.RepoRoot == "" {
		return gitInfo, projectKey, nil
	}
	if _, err := os.Stat(filepath.Join(targetCtx.KluctlProject.LoadArgs.RepoRoot, ".git")); os.IsNotExist(err) {
		return gitInfo, projectKey, nil
	}

	projectDirAbs, err := filepath.Abs(targetCtx.KluctlProject.LoadArgs.ProjectDir)
	if err != nil {
		return gitInfo, projectKey, err
	}

	subDir, err := filepath.Rel(targetCtx.KluctlProject.LoadArgs.RepoRoot, projectDirAbs)
	if err != nil {
		return gitInfo, projectKey, err
	}
	if subDir == "." {
		subDir = ""
	}

	g, err := git2.PlainOpen(targetCtx.KluctlProject.LoadArgs.RepoRoot)
	if err != nil {
		return gitInfo, projectKey, err
	}

	s, err := git.GetWorktreeStatus(targetCtx.SharedContext.Ctx, targetCtx.KluctlProject.LoadArgs.RepoRoot)
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

	var repoKey types.GitRepoKey
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
	projectKey.GitRepoKey = repoKey
	projectKey.SubDir = subDir
	return gitInfo, projectKey, nil
}

func buildClusterInfo(k *k8s2.K8sCluster) (result.ClusterInfo, error) {
	var clusterInfo result.ClusterInfo
	clusterId, err := k.GetClusterId()
	if err != nil {
		return clusterInfo, err
	}
	clusterInfo = result.ClusterInfo{
		ClusterId: clusterId,
	}
	return clusterInfo, nil
}
