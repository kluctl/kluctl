package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type DiffCommand struct {
	targetCtx *target_context.TargetContext

	ForceApply           bool
	ReplaceOnError       bool
	ForceReplaceOnError  bool
	IgnoreTags           bool
	IgnoreLabels         bool
	IgnoreAnnotations    bool
	IgnoreKluctlMetadata bool

	SkipResourceVersions map[k8s2.ObjectRef]string
}

func NewDiffCommand(targetCtx *target_context.TargetContext) *DiffCommand {
	return &DiffCommand{
		targetCtx: targetCtx,
	}
}

func (cmd *DiffCommand) Run() *result.CommandResult {
	dew := utils.NewDeploymentErrorsAndWarnings()

	r := newCommandResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, "diff")
	r.Command.ForceApply = cmd.ForceApply
	r.Command.ReplaceOnError = cmd.ReplaceOnError
	r.Command.ForceReplaceOnError = cmd.ForceReplaceOnError

	defer func() {
		finishCommandResult(r, cmd.targetCtx, dew)
	}()

	if cmd.targetCtx.Target.Discriminator == "" {
		status.Warning(cmd.targetCtx.SharedContext.Ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &cmd.targetCtx.Target.Discriminator, cmd.targetCtx.DeploymentCollection.LocalObjectRefs(), false)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	o := &utils.ApplyUtilOptions{
		ForceApply:           cmd.ForceApply,
		ReplaceOnError:       cmd.ReplaceOnError,
		ForceReplaceOnError:  cmd.ForceReplaceOnError,
		DryRun:               true,
		AbortOnError:         false,
		ReadinessTimeout:     0,
		SkipResourceVersions: cmd.SkipResourceVersions,
	}
	au := utils.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, o)
	au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

	du := utils.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
	du.IgnoreTags = cmd.IgnoreTags
	du.IgnoreLabels = cmd.IgnoreLabels
	du.IgnoreAnnotations = cmd.IgnoreAnnotations
	du.IgnoreKluctlMetadata = cmd.IgnoreKluctlMetadata
	du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}
	r.Objects = collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, nil)

	return r
}
