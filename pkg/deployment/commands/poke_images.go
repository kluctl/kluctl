package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
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

func (cmd *PokeImagesCommand) Run(ctx context.Context, k *k8s.K8sCluster) (*result.CommandResult, error) {
	var wg sync.WaitGroup

	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(ctx, dew)
	err := ru.UpdateRemoteObjects(k, nil, cmd.c.LocalObjectRefs(), false)
	if err != nil {
		return nil, err
	}

	allObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range cmd.c.Deployments {
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

	var fieldPathes []uo.KeyPath
	fieldPathes = append(fieldPathes, uo.KeyPath{"spec", "template", "spec", "containers"})
	fieldPathes = append(fieldPathes, uo.KeyPath{"spec", "template", "spec", "initContainers"})

	doPokeImage := func(images []types.FixedImage, o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
		for _, image := range images {
			for _, jsp := range fieldPathes {
				containers, _, _ := o.GetNestedObjectList(jsp...)
				for _, c := range containers {
					containerName, _, _ := c.GetNestedString("name")
					if image.Container != nil && containerName == *image.Container {
						_ = c.SetNestedField(image.ResultImage, "image")
					}
				}
			}
		}
		return o, nil
	}

	ad := utils2.NewApplyDeploymentsUtil(ctx, dew, cmd.c.Deployments, ru, k, &utils2.ApplyUtilOptions{})

	for ref, containers := range containersAndImages {
		ref := ref
		containers := containers
		wg.Add(1)
		go func() {
			defer wg.Done()
			au := ad.NewApplyUtil(ctx, nil)
			remote := ru.GetRemoteObject(ref)
			if remote == nil {
				dew.AddWarning(ref, fmt.Errorf("remote object not found, skipped image replacement"))
				return
			}
			au.ReplaceObject(ref, remote, func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
				return doPokeImage(containers, o)
			})
		}()
	}
	wg.Wait()

	du := utils2.NewDiffUtil(dew, cmd.c.Deployments, ru, ad.GetAppliedObjectsMap())
	du.Diff()

	return &result.CommandResult{
		NewObjects:     nil,
		ChangedObjects: du.ChangedObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     cmd.c.Images.SeenImages(false),
	}, nil
}
