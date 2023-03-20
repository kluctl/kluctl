package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"time"
)

type DeployCommand struct {
	c             *deployment.DeploymentCollection
	discriminator string

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	AbortOnError        bool
	ReadinessTimeout    time.Duration
	NoWait              bool
}

func NewDeployCommand(discriminator string, c *deployment.DeploymentCollection) *DeployCommand {
	return &DeployCommand{
		discriminator: discriminator,
		c:             c,
	}
}

func (cmd *DeployCommand) Run(ctx context.Context, k *k8s.K8sCluster, diffResultCb func(diffResult *result.CommandResult) error) (*result.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	if cmd.discriminator == "" {
		status.Warning(ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &cmd.discriminator, cmd.c.LocalObjectRefs(), false)
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
		diffResult := &result.CommandResult{
			NewObjects:     au.GetNewObjects(),
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
	return &result.CommandResult{
		NewObjects:     au.GetNewObjects(),
		ChangedObjects: du.ChangedObjects,
		DeletedObjects: au.GetDeletedObjects(),
		HookObjects:    au.GetAppliedHookObjects(),
		OrphanObjects:  orphanObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}
