package commands

import (
	"fmt"
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

func (cmd *PruneCommand) Run(confirmCb func(refs []k8s2.ObjectRef) error) *result.CommandResult {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	r := &result.CommandResult{}

	defer func() {
		r.Errors = append(r.Errors, dew.GetErrorsList()...)
		r.Warnings = append(r.Warnings, dew.GetWarningsList()...)
		r.SeenImages = cmd.targetCtx.DeploymentCollection.Images.SeenImages(false)
		addBaseCommandInfoToResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, r, "prune")
	}()

	discriminator := cmd.discriminator
	if discriminator == "" && cmd.targetCtx != nil {
		discriminator = cmd.targetCtx.Target.Discriminator
	}
	if discriminator == "" {
		dew.AddError(k8s2.ObjectRef{}, fmt.Errorf("pruning without a discriminator is not supported"))
		return r
	}

	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &discriminator, nil, false)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		dew.AddError(k8s2.ObjectRef{}, err)
		return r
	}

	if confirmCb != nil {
		err = confirmCb(orphanObjects)
		if err != nil {
			dew.AddError(k8s2.ObjectRef{}, err)
			return r
		}
	}

	deleted := utils2.DeleteObjects(cmd.targetCtx.SharedContext.Ctx, cmd.targetCtx.SharedContext.K, orphanObjects, dew, cmd.wait)
	orphanObjects = filterDeletedOrphans(orphanObjects, deleted)

	r.Objects = collectObjects(cmd.targetCtx.DeploymentCollection, ru, nil, nil, orphanObjects, deleted)

	return r
}

func FindOrphanObjects(k *k8s.K8sCluster, ru *utils2.RemoteObjectUtils, c *deployment.DeploymentCollection) ([]k8s2.ObjectRef, error) {
	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(c.Inclusion), c.Inclusion.HasType("tags"), c.LocalObjectRefs())
}
