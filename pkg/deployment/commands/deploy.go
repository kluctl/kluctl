package commands

import (
	"fmt"
	"github.com/google/uuid"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"time"
)

type DeployCommand struct {
	targetCtx *kluctl_project.TargetContext

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	AbortOnError        bool
	ReadinessTimeout    time.Duration
	NoWait              bool
	Prune               bool
	WaitPrune           bool
}

func NewDeployCommand(targetCtx *kluctl_project.TargetContext) *DeployCommand {
	return &DeployCommand{
		targetCtx: targetCtx,
	}
}

func (cmd *DeployCommand) Run(diffResultCb func(diffResult *result.CommandResult) error) (*result.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	if cmd.targetCtx.Target.Discriminator == "" {
		status.Warning(cmd.targetCtx.SharedContext.Ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &cmd.targetCtx.Target.Discriminator, cmd.targetCtx.DeploymentCollection.LocalObjectRefs(), false)
	if err != nil {
		return nil, err
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
		au := utils2.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, o)
		au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

		du := utils2.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
		du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

		orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
		diffResult := &result.CommandResult{
			Id:         uuid.New().String(),
			Objects:    collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, nil),
			Errors:     dew.GetErrorsList(),
			Warnings:   dew.GetWarningsList(),
			SeenImages: cmd.targetCtx.DeploymentCollection.Images.SeenImages(false),
		}

		err = diffResultCb(diffResult)
		if err != nil {
			return nil, err
		}
	}

	// clear errors and warnings
	dew.Init()

	// modify options to become a deploy
	o.DryRun = cmd.targetCtx.SharedContext.K.DryRun
	o.AbortOnError = cmd.AbortOnError

	au := utils2.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, o)
	au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

	du := utils2.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
	du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		return nil, err
	}

	var deleted []k8s2.ObjectRef
	if cmd.Prune && cmd.targetCtx.Target.Discriminator == "" {
		dew.AddError(k8s2.ObjectRef{}, fmt.Errorf("pruning without a discriminator is not supported"))
	} else if cmd.Prune {
		deleted = utils2.DeleteObjects(cmd.targetCtx.SharedContext.Ctx, cmd.targetCtx.SharedContext.K, orphanObjects, dew, cmd.WaitPrune)

		// now clean up the list of orphan objects (remove the ones that got deleted)
		ds := map[k8s2.ObjectRef]bool{}
		for _, x := range deleted {
			ds[x] = true
		}
		var tmp []k8s2.ObjectRef
		for _, x := range orphanObjects {
			if _, ok := ds[x]; !ok {
				tmp = append(tmp, x)
			}
		}
		orphanObjects = tmp
	}

	r := &result.CommandResult{
		Id:         uuid.New().String(),
		Objects:    collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, deleted),
		Errors:     dew.GetErrorsList(),
		Warnings:   dew.GetWarningsList(),
		SeenImages: cmd.targetCtx.DeploymentCollection.Images.SeenImages(false),
	}
	r.Command.ForceApply = cmd.ForceApply
	r.Command.ReplaceOnError = cmd.ReplaceOnError
	r.Command.ForceReplaceOnError = cmd.ForceReplaceOnError
	r.Command.AbortOnError = cmd.AbortOnError
	r.Command.NoWait = cmd.NoWait
	err = addBaseCommandInfoToResult(cmd.targetCtx, r, "deploy")
	if err != nil {
		return r, err
	}
	return r, nil
}
