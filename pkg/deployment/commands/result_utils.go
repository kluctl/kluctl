package commands

import (
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/git"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

func newCommandResult(targetCtx *target_context.TargetContext, startTime time.Time, command string) *result.CommandResult {
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
	r.GitInfo, r.ProjectKey, err = git.BuildGitInfo(targetCtx.SharedContext.Ctx,
		targetCtx.KluctlProject.LoadArgs.RepoRoot, targetCtx.KluctlProject.LoadArgs.ProjectDir)
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

func newValidateCommandResult(targetCtx *target_context.TargetContext, startTime time.Time) *result.ValidateResult {
	r := &result.ValidateResult{}

	r.StartTime = metav1.NewTime(startTime)
	r.EndTime = metav1.Now()
	var err error
	_, r.ProjectKey, err = git.BuildGitInfo(targetCtx.SharedContext.Ctx,
		targetCtx.KluctlProject.LoadArgs.RepoRoot, targetCtx.KluctlProject.LoadArgs.ProjectDir)
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

func finishCommandResult(r *result.CommandResult, targetCtx *target_context.TargetContext, dew *utils2.DeploymentErrorsAndWarnings) {
	r.Errors = append(r.Errors, dew.GetErrorsList()...)
	r.Warnings = append(r.Warnings, dew.GetWarningsList()...)
	if targetCtx != nil {
		r.SeenImages = targetCtx.DeploymentCollection.Images.SeenImages(false)
	}
	r.Command.EndTime = metav1.Now()
}

func finishValidateResult(r *result.ValidateResult, targetCtx *target_context.TargetContext, dew *utils2.DeploymentErrorsAndWarnings) {
	r.Errors = append(r.Errors, dew.GetErrorsList()...)
	r.Warnings = append(r.Warnings, dew.GetWarningsList()...)
	r.EndTime = metav1.Now()
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
