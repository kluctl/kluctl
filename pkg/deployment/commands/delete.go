package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"time"
)

type DeleteCommand struct {
	discriminator string
	targetCtx     *kluctl_project.TargetContext
	inclusion     *utils.Inclusion
	wait          bool
}

func NewDeleteCommand(discriminator string, targetCtx *kluctl_project.TargetContext, inclusion *utils.Inclusion, wait bool) *DeleteCommand {
	return &DeleteCommand{
		discriminator: discriminator,
		targetCtx:     targetCtx,
		inclusion:     inclusion,
		wait:          wait,
	}
}

func (cmd *DeleteCommand) Run(ctx context.Context, k *k8s.K8sCluster, confirmCb func(refs []k8s2.ObjectRef) error) (*result.CommandResult, error) {
	startTime := time.Now()

	inclusion := cmd.inclusion
	if inclusion == nil && cmd.targetCtx != nil {
		inclusion = cmd.targetCtx.DeploymentCollection.Inclusion
	}

	discriminator := cmd.discriminator
	if discriminator == "" && cmd.targetCtx != nil {
		discriminator = cmd.targetCtx.Target.Discriminator
	}

	if discriminator == "" {
		return nil, fmt.Errorf("deletion without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()
	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	deleteRefs, err := utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(inclusion), inclusion.HasType("tags"), nil)
	if err != nil {
		return nil, err
	}

	if confirmCb != nil {
		err = confirmCb(deleteRefs)
		if err != nil {
			return nil, err
		}
	}

	deleted := utils2.DeleteObjects(ctx, k, deleteRefs, dew, cmd.wait)

	var c *deployment.DeploymentCollection
	if cmd.targetCtx != nil {
		c = cmd.targetCtx.DeploymentCollection
	}

	r := &result.CommandResult{
		Objects:  collectObjects(c, ru, nil, nil, nil, deleted),
		Errors:   dew.GetErrorsList(),
		Warnings: dew.GetWarningsList(),
	}
	if cmd.targetCtx != nil {
		err = addBaseCommandInfoToResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, r, "delete")
		if err != nil {
			return r, err
		}
	} else {
		err = addDeleteCommandInfoToResult(r, k, startTime, inclusion)
		if err != nil {
			return r, err
		}
	}
	return r, nil
}
