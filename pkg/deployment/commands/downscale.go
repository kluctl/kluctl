package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"sync"
)

type DownscaleCommand struct {
	c *deployment.DeploymentCollection
}

func NewDownscaleCommand(c *deployment.DeploymentCollection) *DownscaleCommand {
	return &DownscaleCommand{
		c: c,
	}
}

func (cmd *DownscaleCommand) Run(k *k8s.K8sCluster) (*types.CommandResult, error) {
	var wg sync.WaitGroup

	dew := utils2.NewDeploymentErrorsAndWarnings()
	au := utils2.NewApplyUtil(dew, cmd.c, k, utils2.ApplyUtilOptions{})

	appliedObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	var deletedObjects []k8s2.ObjectRef

	for _, d := range cmd.c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		for _, o := range d.Objects {
			o := o
			ref := o.GetK8sRef()
			wg.Add(1)
			if utils2.IsDownscaleDelete(o) {
				go func() {
					defer wg.Done()
					au.DeleteObject(ref, false)
				}()
			} else {
				go func() {
					defer wg.Done()
					au.ReplaceObject(ref, cmd.c.GetRemoteObject(ref), func(remote *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
						return utils2.DownscaleObject(remote, o)
					})
				}()
			}
		}
	}
	wg.Wait()

	du := utils2.NewDiffUtil(dew, cmd.c.Deployments, cmd.c.RemoteObjects, appliedObjects)
	du.Diff(k)

	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		DeletedObjects: deletedObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}