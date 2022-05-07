package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type DiffCommand struct {
	c *deployment.DeploymentCollection

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	IgnoreTags          bool
	IgnoreLabels        bool
	IgnoreAnnotations   bool
}

func NewDiffCommand(c *deployment.DeploymentCollection) *DiffCommand {
	return &DiffCommand{
		c: c,
	}
}

func (cmd *DiffCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*types.CommandResult, error) {
	dew := utils.NewDeploymentErrorsAndWarnings()

	ru := utils.NewRemoteObjectsUtil(dew)
	err := ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
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
	au := utils.NewApplyDeploymentsUtil(ctx, dew, cmd.c.Deployments, ru, k, o)
	au.ApplyDeployments()

	du := utils.NewDiffUtil(dew, cmd.c.Deployments, ru, au.GetAppliedObjectsMap())
	du.IgnoreTags = cmd.IgnoreTags
	du.IgnoreLabels = cmd.IgnoreLabels
	du.IgnoreAnnotations = cmd.IgnoreAnnotations
	du.Diff()

	orphanObjects, err := FindOrphanObjects(k, ru, cmd.c)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		DeletedObjects: au.GetDeletedObjects(),
		HookObjects:    au.GetAppliedHookObjects(),
		OrphanObjects:  orphanObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}
