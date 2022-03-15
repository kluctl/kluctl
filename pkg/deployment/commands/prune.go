package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
)

type PruneCommand struct {
	c *deployment.DeploymentCollection
}

func NewPruneCommand(c *deployment.DeploymentCollection) *PruneCommand {
	return &PruneCommand{
		c: c,
	}
}

func (cmd *PruneCommand) Run(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(dew)
	err := ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), nil)
	if err != nil {
		return nil, err
	}

	return FindOrphanObjects(k, ru, cmd.c)
}

func FindOrphanObjects(k *k8s.K8sCluster, ru *utils2.RemoteObjectUtils, c *deployment.DeploymentCollection) ([]k8s2.ObjectRef, error) {
	return utils2.FindObjectsForDelete(k, ru.GetFilteredRemoteObjects(c.Inclusion), c.Inclusion.HasType("tags"), c.LocalObjectRefs())
}
