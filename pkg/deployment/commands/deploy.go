package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"time"
)

type DeployCommand struct {
	c *deployment.DeploymentCollection

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	AbortOnError        bool
	ReadinessTimeout    time.Duration
	NoWait              bool
}

func NewDeployCommand(c *deployment.DeploymentCollection) *DeployCommand {
	return &DeployCommand{
		c: c,
	}
}

func (cmd *DeployCommand) Run(ctx context.Context, k *k8s.K8sCluster, diffResultCb func(diffResult *types.CommandResult) error) (*types.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
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
		au := utils2.NewApplyDeploymentsUtil(ctx, dew, cmd.c.Deployments, ru, k, o)
		au.ApplyDeployments()

		du := utils2.NewDiffUtil(dew, cmd.c.Deployments, ru, au.GetAppliedObjectsMap())
		du.Diff()

		orphanObjects, err := FindOrphanObjects(k, ru, cmd.c)
		diffResult := &types.CommandResult{
			NewObjects:     du.NewObjects,
			ChangedObjects: du.ChangedObjects,
			DeletedObjects: au.GetDeletedObjects(),
			HookObjects:    au.GetAppliedHookObjects(),
			OrphanObjects:  orphanObjects,
			Errors:         dew.GetErrorsList(),
			Warnings:       dew.GetWarningsList(),
			SeenImages:     cmd.c.Images.SeenImages(false),
		}

		err = diffResultCb(diffResult)
		if err != nil {
			return nil, err
		}
	}

	// clear errors and warnings
	dew.Init()

	// modify options to become a deploy
	o.DryRun = k.DryRun
	o.AbortOnError = cmd.AbortOnError

	au := utils2.NewApplyDeploymentsUtil(ctx, dew, cmd.c.Deployments, ru, k, o)
	au.ApplyDeployments()

	du := utils2.NewDiffUtil(dew, cmd.c.Deployments, ru, au.GetAppliedObjectsMap())
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
