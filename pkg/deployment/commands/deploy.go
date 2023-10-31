package commands

import (
	"fmt"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"time"
)

type DeployCommand struct {
	targetCtx *target_context.TargetContext

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	AbortOnError        bool
	ReadinessTimeout    time.Duration
	NoWait              bool
	Prune               bool
	WaitPrune           bool
}

func NewDeployCommand(targetCtx *target_context.TargetContext) *DeployCommand {
	return &DeployCommand{
		targetCtx: targetCtx,
	}
}

func (cmd *DeployCommand) Run(diffResultCb func(diffResult *result.CommandResult) error) *result.CommandResult {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	r := newCommandResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, "deploy")
	r.Command.ForceApply = cmd.ForceApply
	r.Command.ReplaceOnError = cmd.ReplaceOnError
	r.Command.ForceReplaceOnError = cmd.ForceReplaceOnError
	r.Command.AbortOnError = cmd.AbortOnError
	r.Command.NoWait = cmd.NoWait

	defer func() {
		finishCommandResult(r, cmd.targetCtx, dew)
	}()

	if cmd.targetCtx.Target.Discriminator == "" {
		status.Warning(cmd.targetCtx.SharedContext.Ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &cmd.targetCtx.Target.Discriminator, cmd.targetCtx.DeploymentCollection.LocalObjectRefs(), false)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	// prepare for a diff
	o := &utils2.ApplyUtilOptions{
		ForceApply:          cmd.ForceApply,
		ReplaceOnError:      cmd.ReplaceOnError,
		ForceReplaceOnError: cmd.ForceReplaceOnError,
		DryRun:              true,
		AbortOnError:        false,
		ReadinessTimeout:    cmd.ReadinessTimeout,
		NoWait:              cmd.NoWait,
	}

	if diffResultCb != nil {
		diffDew := dew.Clone()
		au := utils2.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, diffDew, ru, cmd.targetCtx.SharedContext.K, o)
		au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

		du := utils2.NewDiffUtil(diffDew, ru, au.GetAppliedObjectsMap())
		du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

		orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
		diffResult := &result.CommandResult{
			Objects:    collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, nil),
			Errors:     diffDew.GetErrorsList(),
			Warnings:   diffDew.GetWarningsList(),
			SeenImages: cmd.targetCtx.DeploymentCollection.Images.SeenImages(false),
		}

		err = diffResultCb(diffResult)
		if err != nil {
			dew.AddError(k8s2.ObjectRef{}, err)
			return r
		}
	}

	// modify options to become a deploy
	o.DryRun = cmd.targetCtx.SharedContext.K.DryRun
	o.AbortOnError = cmd.AbortOnError

	au := utils2.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, o)
	au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

	du := utils2.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
	du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

	var orphanObjects []k8s2.ObjectRef
	var deleted []k8s2.ObjectRef

	orphanObjects, err = FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
	}

	if cmd.Prune && cmd.targetCtx.Target.Discriminator == "" {
		dew.AddError(k8s2.ObjectRef{}, fmt.Errorf("pruning without a discriminator is not supported"))
	} else if cmd.Prune {
		deleted = utils2.DeleteObjects(cmd.targetCtx.SharedContext.Ctx, cmd.targetCtx.SharedContext.K, orphanObjects, dew, cmd.WaitPrune)

		// now clean up the list of orphan objects (remove the ones that got deleted)
		orphanObjects = filterDeletedOrphans(orphanObjects, deleted)
	}

	r.Objects = collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, deleted)

	return r
}
