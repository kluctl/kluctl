package commands

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/deployment"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"sync"
)

type PokeImagesCommand struct {
	c *deployment.DeploymentCollection
}

func NewPokeImagesCommand(c *deployment.DeploymentCollection) *PokeImagesCommand {
	return &PokeImagesCommand{
		c: c,
	}
}

func (cmd *PokeImagesCommand) Run(k *k8s.K8sCluster) (*types.CommandResult, error) {
	var wg sync.WaitGroup

	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(dew)
	err := ru.UpdateRemoteObjects(k, cmd.c.Project.GetCommonLabels(), cmd.c.LocalObjectRefs())
	if err != nil {
		return nil, err
	}

	allObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range cmd.c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		for _, o := range d.Objects {
			allObjects[o.GetK8sRef()] = o
		}
	}

	containersAndImages := make(map[k8s2.ObjectRef][]types.FixedImage)
	for _, fi := range cmd.c.Images.SeenImages(false) {
		_, ok := allObjects[*fi.Object]
		if !ok {
			dew.AddError(*fi.Object, fmt.Errorf("object not found while trying to associate image with deployed object"))
			continue
		}

		containersAndImages[*fi.Object] = append(containersAndImages[*fi.Object], fi)
	}

	doPokeImage := func(images []types.FixedImage, o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
		containers, _, _ := o.GetNestedObjectList("spec", "template", "spec", "containers")

		for _, image := range images {
			for _, c := range containers {
				containerName, _, _ := c.GetNestedString("name")
				if image.Container != nil && containerName == *image.Container {
					c.SetNestedField(image.ResultImage, "image")
				}
			}
		}
		return o, nil
	}

	au := utils2.NewApplyUtil(dew, cmd.c.Deployments, ru, k, utils2.ApplyUtilOptions{})

	for ref, containers := range containersAndImages {
		ref := ref
		containers := containers
		wg.Add(1)
		go func() {
			defer wg.Done()
			au.ReplaceObject(ref, ru.GetRemoteObject(ref), func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
				return doPokeImage(containers, o)
			})
		}()
	}
	wg.Wait()

	du := utils2.NewDiffUtil(dew, cmd.c.Deployments, ru, au.AppliedObjects)
	du.Diff()

	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}
