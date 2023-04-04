package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type DeleteCommand struct {
	c             *deployment.DeploymentCollection
	discriminator string
	inclusion     *utils.Inclusion
}

func NewDeleteCommand(discriminator string, c *deployment.DeploymentCollection, inclusion *utils.Inclusion) *DeleteCommand {
	return &DeleteCommand{
		discriminator: discriminator,
		c:             c,
		inclusion:     inclusion,
	}
}

func (cmd *DeleteCommand) Run(ctx context.Context, k *k8s.K8sCluster, confirmCb func(refs []k8s2.ObjectRef) error) (*result.CommandResult, error) {
	if cmd.discriminator == "" {
		return nil, fmt.Errorf("deletion without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()
	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &cmd.discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	deleteRefs, err := utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(cmd.inclusion), cmd.inclusion.HasType("tags"), nil)
	if err != nil {
		return nil, err
	}

	if confirmCb != nil {
		err = confirmCb(deleteRefs)
		if err != nil {
			return nil, err
		}
	}

	deleted, err := utils2.DeleteObjects(ctx, k, deleteRefs, dew, true)
	if err != nil {
		return nil, err
	}

	return &result.CommandResult{
		RenderedObjects: cmd.c.LocalObjects(),
		RemoteObjects:   ru.GetFilteredRemoteObjects(nil),
		DeletedObjects:  deleted,
		Errors:          dew.GetErrorsList(),
		Warnings:        dew.GetWarningsList(),
	}, nil
}
