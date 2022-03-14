package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"time"
)

type DeployCommand struct {
	c *deployment.DeploymentCollection

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	AbortOnError        bool
	HookTimeout         time.Duration
}

func NewDeployCommand(c *deployment.DeploymentCollection) *DeployCommand {
	return &DeployCommand{
		c: c,
	}
}

func (cmd *DeployCommand) Run(k *k8s.K8sCluster) (*types.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	o := utils2.ApplyUtilOptions{
		ForceApply:          cmd.ForceApply,
		ReplaceOnError:      cmd.ReplaceOnError,
		ForceReplaceOnError: cmd.ForceReplaceOnError,
		DryRun:              k.DryRun,
		AbortOnError:        cmd.AbortOnError,
		HookTimeout:         cmd.HookTimeout,
	}
	au := utils2.NewApplyUtil(dew, cmd.c, k, o)
	au.ApplyDeployments()

	du := utils2.NewDiffUtil(dew, cmd.c.Deployments, cmd.c.RemoteObjects, au.AppliedObjects)
	du.Diff(k)

	orphanObjects, err := cmd.c.FindOrphanObjects(k)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		DeletedObjects: au.GetDeletedObjectsList(),
		HookObjects:    au.GetAppliedHookObjects(),
		OrphanObjects:  orphanObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}
