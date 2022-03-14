package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	"github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
)

type DiffCommand struct {
	c *deployment.DeploymentCollection

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	IgnoreTags        bool
	IgnoreLabels      bool
	IgnoreAnnotations bool
}

func NewDiffCommand(c *deployment.DeploymentCollection) *DiffCommand {
	return &DiffCommand{
		c: c,
	}
}

func (cmd *DiffCommand) Run(k *k8s.K8sCluster) (*types.CommandResult, error) {
	dew := utils.NewDeploymentErrorsAndWarnings()

	o := utils.ApplyUtilOptions{
		ForceApply:          cmd.ForceApply,
		ReplaceOnError:      cmd.ReplaceOnError,
		ForceReplaceOnError: cmd.ForceReplaceOnError,
		DryRun:              true,
		AbortOnError:        false,
		HookTimeout:         0,
	}
	au := utils.NewApplyUtil(dew, cmd.c, k, o)
	au.ApplyDeployments()

	du := utils.NewDiffUtil(dew, cmd.c.Deployments, cmd.c.RemoteObjects, au.AppliedObjects)
	du.IgnoreTags = cmd.IgnoreTags
	du.IgnoreLabels = cmd.IgnoreLabels
	du.IgnoreAnnotations = cmd.IgnoreAnnotations
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
