package commands

import (
	"fmt"
	"github.com/google/uuid"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type DeleteCommand struct {
	discriminator string
	targetCtx     *kluctl_project.TargetContext
}

func NewDeleteCommand(discriminator string, targetCtx *kluctl_project.TargetContext) *DeleteCommand {
	return &DeleteCommand{
		discriminator: discriminator,
		targetCtx:     targetCtx,
	}
}

func (cmd *DeleteCommand) Run(confirmCb func(refs []k8s2.ObjectRef) error) (*result.CommandResult, error) {
	discriminator := cmd.targetCtx.Target.Discriminator
	if cmd.discriminator != "" {
		discriminator = cmd.discriminator
	}

	if discriminator == "" {
		return nil, fmt.Errorf("deletion without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()
	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, &discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	deleteRefs, err := utils2.FindObjectsForDelete(cmd.targetCtx.SharedContext.K, ru.GetFilteredRemoteObjects(cmd.targetCtx.DeploymentCollection.Inclusion), cmd.targetCtx.DeploymentCollection.Inclusion.HasType("tags"), nil)
	if err != nil {
		return nil, err
	}

	if confirmCb != nil {
		err = confirmCb(deleteRefs)
		if err != nil {
			return nil, err
		}
	}

	deleted, err := utils2.DeleteObjects(cmd.targetCtx.SharedContext.Ctx, cmd.targetCtx.SharedContext.K, deleteRefs, dew, true)
	if err != nil {
		return nil, err
	}

	r := &result.CommandResult{
		Id:       uuid.New().String(),
		Objects:  collectObjects(cmd.targetCtx.DeploymentCollection, ru, nil, nil, nil, deleted),
		Errors:   dew.GetErrorsList(),
		Warnings: dew.GetWarningsList(),
	}
	err = addBaseCommandInfoToResult(cmd.targetCtx, r, "delete")
	if err != nil {
		return r, err
	}
	return r, nil
}
