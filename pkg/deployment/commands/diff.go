package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type DiffCommand struct {
	c             *deployment.DeploymentCollection
	discriminator string

	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	IgnoreTags          bool
	IgnoreLabels        bool
	IgnoreAnnotations   bool
}

func NewDiffCommand(discriminator string, c *deployment.DeploymentCollection) *DiffCommand {
	return &DiffCommand{
		discriminator: discriminator,
		c:             c,
	}
}

func (cmd *DiffCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*result.CommandResult, error) {
	dew := utils.NewDeploymentErrorsAndWarnings()

	if cmd.discriminator == "" {
		status.Warning(ctx, "No discriminator configured. Orphan object detection will not work")
		dew.AddWarning(k8s2.ObjectRef{}, fmt.Errorf("no discriminator configured. Orphan object detection will not work"))
	}

	ru := utils.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &cmd.discriminator, cmd.c.LocalObjectRefs(), false)
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
