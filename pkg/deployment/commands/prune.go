package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type PruneCommand struct {
	discriminator string
	c             *deployment.DeploymentCollection
}

func NewPruneCommand(discriminator string, c *deployment.DeploymentCollection) *PruneCommand {
	return &PruneCommand{
		discriminator: discriminator,
		c:             c,
	}
}

func (cmd *PruneCommand) Run(ctx context.Context, k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	if cmd.discriminator == "" {
		return nil, fmt.Errorf("pruning without a discriminator is not supported")
	}

	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, &cmd.discriminator, nil, false)
	if err != nil {
		return nil, err
	}

	return FindOrphanObjects(k, ru, cmd.c)
}

func FindOrphanObjects(k *k8s.K8sCluster, ru *utils2.RemoteObjectUtils, c *deployment.DeploymentCollection) ([]k8s2.ObjectRef, error) {
	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(c.Inclusion), c.Inclusion.HasType("tags"), c.LocalObjectRefs())
}
