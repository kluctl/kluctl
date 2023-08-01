package commands

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type PruneCommand struct {
	discriminator string
	targetCtx     *kluctl_project.TargetContext
	wait          bool
}

func NewPruneCommand(discriminator string, targetCtx *kluctl_project.TargetContext, wait bool) *PruneCommand {
	return &PruneCommand{
		discriminator: discriminator,
		targetCtx:     targetCtx,
		wait:          wait,
	}
}

func (cmd *PruneCommand) Run(confirmCb func(refs []k8s2.ObjectRef) error) (*result.CommandResult, error) {
	discriminator := cmd.discriminator
	if discriminator == "" && cmd.targetCtx != nil {
		discriminator = cmd.targetCtx.Target.Discriminator
	}
	if discriminator == "" {
		return nil, fmt.Errorf("pruning without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		return nil, err
	}

	if confirmCb != nil {
		err = confirmCb(orphanObjects)
		if err != nil {
			return nil, err
		}
	}

	deleted := utils2.DeleteObjects(cmd.targetCtx.SharedContext.Ctx, cmd.targetCtx.SharedContext.K, orphanObjects, dew, cmd.wait)
	orphanObjects = filterDeletedOrphans(orphanObjects, deleted)

	r := &result.CommandResult{
		Id:       uuid.New().String(),
		Objects:  collectObjects(cmd.targetCtx.DeploymentCollection, ru, nil, nil, orphanObjects, deleted),
		Warnings: dew.GetWarningsList(),
	}
	err = addBaseCommandInfoToResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, r, "prune")
	if err != nil {
		return r, err
	}
	return r, nil
}

func FindOrphanObjects(k *k8s.K8sCluster, ru *utils2.RemoteObjectUtils, c *deployment.DeploymentCollection) ([]k8s2.ObjectRef, error) {
	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(c.Inclusion), c.Inclusion.HasType("tags"), c.LocalObjectRefs())
}
