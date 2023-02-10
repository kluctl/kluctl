package commands

import (
	"context"
	"fmt"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type DeleteCommand struct {
	discriminator string
	inclusion     *utils.Inclusion
}

func NewDeleteCommand(discriminator string, inclusion *utils.Inclusion) *DeleteCommand {
	return &DeleteCommand{
		discriminator: discriminator,
		inclusion:     inclusion,
	}
}

func (cmd *DeleteCommand) Run(ctx context.Context, k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	if cmd.discriminator == "" {
		return nil, fmt.Errorf("deletion without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()
	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &cmd.discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(cmd.inclusion), cmd.inclusion.HasType("tags"), nil)
}
