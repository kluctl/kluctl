package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
)

type DeleteCommand struct {
	c *deployment.DeploymentCollection
}

func NewDeleteCommand(c *deployment.DeploymentCollection) *DeleteCommand {
	return &DeleteCommand{
		c: c,
	}
}

func (cmd *DeleteCommand) Run(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(dew)
	err := ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
	if err != nil {
		return nil, err
	}

	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(cmd.c.Inclusion), cmd.c.Inclusion.HasType("tags"), nil)
}
