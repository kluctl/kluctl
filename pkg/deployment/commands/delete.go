package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"time"
)

type DeleteCommand struct {
	discriminator string
	targetCtx     *target_context.TargetContext
	inclusion     *utils.Inclusion
	wait          bool
}

func NewDeleteCommand(discriminator string, targetCtx *target_context.TargetContext, inclusion *utils.Inclusion, wait bool) *DeleteCommand {
	return &DeleteCommand{
		discriminator: discriminator,
		targetCtx:     targetCtx,
		inclusion:     inclusion,
		wait:          wait,
	}
}

func (cmd *DeleteCommand) Run(ctx context.Context, k *k8s.K8sCluster, confirmCb func(refs []k8s2.ObjectRef) error) *result.CommandResult {
	startTime := time.Now()

	inclusion := cmd.inclusion
	if inclusion == nil && cmd.targetCtx != nil {
		inclusion = cmd.targetCtx.DeploymentCollection.Inclusion
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()

	var r *result.CommandResult
	if cmd.targetCtx != nil {
		r = newCommandResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, "delete")
	} else {
		r = newDeleteCommandResult(k, startTime, inclusion)
	}

	defer func() {
		finishCommandResult(r, cmd.targetCtx, dew)
	}()

	discriminator := cmd.discriminator
	if discriminator == "" && cmd.targetCtx != nil {
		discriminator = cmd.targetCtx.Target.Discriminator
	}

	if discriminator == "" {
		dew.AddError(k8s2.ObjectRef{}, fmt.Errorf("deletion without a discriminator is not supported"))
		return r
	}

	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &discriminator, nil, false)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	deleteRefs, err := utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(inclusion), inclusion.HasType("tags"), nil)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	if confirmCb != nil {
		err = confirmCb(deleteRefs)
		if err != nil {
			dew.AddError(k8s2.ObjectRef{}, err)
			return r
		}
	}

	deleted := utils2.DeleteObjects(ctx, k, deleteRefs, dew, cmd.wait)

	var c *deployment.DeploymentCollection
	if cmd.targetCtx != nil {
		c = cmd.targetCtx.DeploymentCollection
	}

	r.Objects = collectObjects(c, ru, nil, nil, nil, deleted)

	return r
}
