package commands

import (
	"fmt"
	"github.com/google/uuid"
	utils2 "github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"sync"
)

type PokeImagesCommand struct {
	targetCtx *kluctl_project.TargetContext
}

func NewPokeImagesCommand(targetCtx *kluctl_project.TargetContext) *PokeImagesCommand {
	return &PokeImagesCommand{
		targetCtx: targetCtx,
	}
}

func (cmd *PokeImagesCommand) Run() (*result.CommandResult, error) {
	var wg sync.WaitGroup

	dew := utils2.NewDeploymentErrorsAndWarnings()

	ru := utils2.NewRemoteObjectsUtil(cmd.targetCtx.SharedContext.Ctx, dew)
	err := ru.UpdateRemoteObjects(cmd.targetCtx.SharedContext.K, nil, cmd.targetCtx.DeploymentCollection.LocalObjectRefs(), false)
	if err != nil {
		return nil, err
	}

	allObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range cmd.targetCtx.DeploymentCollection.Deployments {
		for _, o := range d.Objects {
			allObjects[o.GetK8sRef()] = o
		}
	}

	containersAndImages := make(map[k8s2.ObjectRef][]types.FixedImage)
	for _, fi := range cmd.targetCtx.DeploymentCollection.Images.SeenImages(false) {
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

	au := utils2.NewApplyDeploymentsUtil(cmd.targetCtx.SharedContext.Ctx, dew, ru, cmd.targetCtx.SharedContext.K, &utils2.ApplyUtilOptions{})

	for ref, containers := range containersAndImages {
		ref := ref
		containers := containers
		wg.Add(1)
		go func() {
			defer wg.Done()
			au := au.NewApplyUtil(cmd.targetCtx.SharedContext.Ctx, nil)
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

	du := utils2.NewDiffUtil(dew, ru, au.GetAppliedObjectsMap())
	du.DiffDeploymentItems(cmd.targetCtx.DeploymentCollection.Deployments)

	orphanObjects, err := FindOrphanObjects(cmd.targetCtx.SharedContext.K, ru, cmd.targetCtx.DeploymentCollection)
	if err != nil {
		return nil, err
	}

	r := &result.CommandResult{
		Id:         uuid.New().String(),
		Objects:    collectObjects(cmd.targetCtx.DeploymentCollection, ru, au, du, orphanObjects, nil),
		Errors:     dew.GetErrorsList(),
		Warnings:   dew.GetWarningsList(),
		SeenImages: cmd.targetCtx.DeploymentCollection.Images.SeenImages(false),
	}
	err = addBaseCommandInfoToResult(cmd.targetCtx, cmd.targetCtx.KluctlProject.LoadTime, r, "deploy")
	if err != nil {
		return r, err
	}
	return r, nil
}
