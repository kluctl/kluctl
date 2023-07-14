package commands

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type DiffCommand struct {
	targetCtx *kluctl_project.TargetContext

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	IgnoreTags          bool
	IgnoreLabels        bool
	IgnoreAnnotations   bool
}

func NewDiffCommand(targetCtx *kluctl_project.TargetContext) *DiffCommand {
	return &DiffCommand{
		targetCtx: targetCtx,
	}
}

func (cmd *DiffCommand) Run() (*result.CommandResult, error) {
	dew := utils.NewDeploymentErrorsAndWarnings()

	if cmd.targetCtx.Target.Discriminator == "" {
		status.Warning(cmd.targetCtx.SharedContext.Ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &cmd.targetCtx.Target.Discriminator, cmd.targetCtx.DeploymentCollection.LocalObjectRefs(), false)
	if err != nil {
		return nil, err
	}

	o := &utils.ApplyUtilOptions{
		ForceApply:          cmd.ForceApply,
		ReplaceOnError:      cmd.ReplaceOnError,
		ForceReplaceOnError: cmd.ForceReplaceOnError,
		DryRun:              true,
		AbortOnError:        false,
		ReadinessTimeout:    0,
	}
	au := utils.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, o)
	au.ApplyDeployments(cmd.targetCtx.DeploymentCollection.Deployments)

	du := utils.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
	du.IgnoreTags = cmd.IgnoreTags
	du.IgnoreLabels = cmd.IgnoreLabels
	du.IgnoreAnnotations = cmd.IgnoreAnnotations
	du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		return nil, err
	}
	r := &result.CommandResult{
		Id:         uuid.New().String(),
		Objects:    collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, nil),
		Errors:     dew.GetErrorsList(),
		Warnings:   dew.GetWarningsList(),
		SeenImages: cmd.targetCtx.DeploymentCollection.Images.SeenImages(false),
	}
	r.Command.ForceApply = cmd.ForceApply
	r.Command.ReplaceOnError = cmd.ReplaceOnError
	r.Command.ForceReplaceOnError = cmd.ForceReplaceOnError
	err = addBaseCommandInfoToResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, r, "diff")
	if err != nil {
		return r, err
	}
	return r, nil
}
