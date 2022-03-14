package deployment

import (
	"context"
	"fmt"
	utils2 "github.com/codablock/kluctl/pkg/deployment/utils"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/validation"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/api/errors"
	"path/filepath"
	"reflect"
	"sync"
)

type DeploymentCollection struct {
	Project *DeploymentProject
	Images  *Images
	inclusion *utils.Inclusion
	RenderDir string
	forSeal   bool

	Deployments   []*DeploymentItem
	RemoteObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	mutex         sync.Mutex
}

func NewDeploymentCollection(project *DeploymentProject, images *Images, inclusion *utils.Inclusion, renderDir string, forSeal bool) (*DeploymentCollection, error) {
	dc := &DeploymentCollection{
		Project:       project,
		Images:        images,
		inclusion:     inclusion,
		RenderDir:     renderDir,
		forSeal:       forSeal,
		RemoteObjects: map[k8s2.ObjectRef]*uo.UnstructuredObject{},
	}

	indexes := make(map[string]int)
	deployments, err := dc.collectDeployments(project, indexes)
	if err != nil {
		return nil, err
	}
	dc.Deployments = deployments
	return dc, nil
}

func (c *DeploymentCollection) createBarrierDummy(project *DeploymentProject) *DeploymentItem {
	b := true
	tmpDiConfig := &types.DeploymentItemConfig{
		Barrier: &b,
	}
	di, err := NewDeploymentItem(project, c, tmpDiConfig, nil, 0)
	if err != nil {
		log.Fatal(err)
	}
	return di
}

func findDeploymentItemIndex(project *DeploymentProject, pth *string, indexes map[string]int) (int, *string) {
	if pth == nil {
		return 0, nil
	}
	var dir2 *string
	index := 0
	dir := filepath.Join(project.dir, *pth)
	absDir, err := filepath.Abs(dir)
	if err != nil {
		// we pre-checked directories, so this should not happen
		log.Fatal(err)
	}

	if _, ok := indexes[absDir]; !ok {
		indexes[absDir] = 0
	}
	index, _ = indexes[absDir]
	indexes[absDir] = index + 1
	dir2 = &absDir
	return index, dir2
}

func (c *DeploymentCollection) collectDeployments(project *DeploymentProject, indexes map[string]int) ([]*DeploymentItem, error) {
	var ret []*DeploymentItem

	for i, diConfig := range project.Config.Deployments {
		if diConfig.Include != nil {
			includedProject, ok := project.includes[i]
			if !ok {
				log.Fatalf("Did not find find index %d in project.includes", i)
			}
			ret2, err := c.collectDeployments(includedProject, indexes)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ret2...)
			if diConfig.Barrier != nil && *diConfig.Barrier {
				ret = append(ret, c.createBarrierDummy(project))
			}
		} else {
			index, dir2 := findDeploymentItemIndex(project, diConfig.Path, indexes)
			di, err := NewDeploymentItem(project, c, diConfig, dir2, index)
			if err != nil {
				return nil, err
			}
			ret = append(ret, di)
		}
	}

	return ret, nil
}

func (c *DeploymentCollection) RenderDeployments(k *k8s.K8sCluster) error {
	log.Infof("Rendering templates and Helm charts")

	wp := utils.NewDebuggerAwareWorkerPool(16)
	defer wp.StopWait(false)

	for _, d := range c.Deployments {
		err := d.render(k, wp)
		if err != nil {
			return err
		}
	}
	err := wp.StopWait(true)
	if err != nil {
		return err
	}

	for _, d := range c.Deployments {
		err := d.renderHelmCharts(k, wp)
		if err != nil {
			return err
		}
	}
	err = wp.StopWait(false)
	if err != nil {
		return err
	}

	return nil
}

func (c *DeploymentCollection) resolveSealedSecrets() error {
	if c.forSeal {
		return nil
	}

	for _, d := range c.Deployments {
		err := d.resolveSealedSecrets()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *DeploymentCollection) buildKustomizeObjects(k *k8s.K8sCluster) error {
	log.Infof("Building kustomize objects")

	var wg sync.WaitGroup
	var errs []error
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(16)

	for _, d_ := range c.Deployments {
		d := d_

		wg.Add(1)
		go func() {
			err := d.buildKustomize()
			if err != nil {
				mutex.Lock()
				errs = append(errs, fmt.Errorf("building kustomize objects for %s failed. %w", *d.dir, err))
				mutex.Unlock()
			} else {
				wg.Add(1)
				go func() {
					_ = sem.Acquire(context.Background(), 1)
					defer sem.Release(1)

					err := d.postprocessAndLoadObjects(k)
					if err != nil {
						mutex.Lock()
						errs = append(errs, fmt.Errorf("postprocessing kustomize objects for %s failed. %w", *d.dir, err))
						mutex.Unlock()
					}
					wg.Done()
				}()
			}

			wg.Done()
		}()
	}
	wg.Wait()

	if len(errs) != 0 {
		return utils.NewErrorList(errs)
	}
	return nil
}

func (c *DeploymentCollection) updateRemoteObjects(k *k8s.K8sCluster, dew *utils2.DeploymentErrorsAndWarnings) error {
	if k == nil {
		return nil
	}

	log.Infof("Getting remote objects by commonLabels")
	allObjects, apiWarnings, err := k.ListAllObjects([]string{"get"}, "", c.Project.getCommonLabels(), false)
	for gvk, aw := range apiWarnings {
		dew.AddApiWarnings(k8s2.ObjectRef{GVK: gvk}, aw)
	}
	if err != nil {
		return err
	}

	for _, o := range allObjects {
		c.addRemoteObject(o)
	}

	notFoundRefsMap := make(map[k8s2.ObjectRef]bool)
	var notFoundRefsList []k8s2.ObjectRef
	for ref := range c.localObjectsByRef() {
		if _, ok := c.RemoteObjects[ref]; !ok {
			if _, ok = notFoundRefsMap[ref]; !ok {
				notFoundRefsMap[ref] = true
				notFoundRefsList = append(notFoundRefsList, ref)
			}
		}
	}

	if len(notFoundRefsList) != 0 {
		log.Infof("Getting %d additional remote objects", len(notFoundRefsList))
		r, apiWarnings, err := k.GetObjectsByRefs(notFoundRefsList)
		for ref, aw := range apiWarnings {
			dew.AddApiWarnings(ref, aw)
		}
		if err != nil {
			return err
		}
		for _, o := range r {
			c.addRemoteObject(o)
		}
	}
	return nil
}

func (c *DeploymentCollection) addRemoteObject(o *uo.UnstructuredObject) {
	c.RemoteObjects[o.GetK8sRef()] = o
}

func (c *DeploymentCollection) GetRemoteObject(ref k8s2.ObjectRef) *uo.UnstructuredObject {
	o, _ := c.RemoteObjects[ref]
	return o
}

func (c *DeploymentCollection) ForgetRemoteObject(ref k8s2.ObjectRef) {
	delete(c.RemoteObjects, ref)
}

func (c *DeploymentCollection) localObjectsByRef() map[k8s2.ObjectRef]bool {
	ret := make(map[k8s2.ObjectRef]bool)
	for _, d := range c.Deployments {
		for _, o := range d.Objects {
			ret[o.GetK8sRef()] = true
		}
	}
	return ret
}

func (c *DeploymentCollection) localObjectRefs() []k8s2.ObjectRef {
	var ret []k8s2.ObjectRef
	for ref := range c.localObjectsByRef() {
		ret = append(ret, ref)
	}
	return ret
}

func (c *DeploymentCollection) Prepare(k *k8s.K8sCluster) error {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	err := c.RenderDeployments(k)
	if err != nil {
		return err
	}
	err = c.resolveSealedSecrets()
	if err != nil {
		return err
	}
	err = c.buildKustomizeObjects(k)
	if err != nil {
		return err
	}
	err = c.updateRemoteObjects(k, dew)
	if err != nil {
		return err
	}
	err = dew.GetMultiError()
	if err != nil {
		return err
	}
	return nil
}

func (c *DeploymentCollection) PokeImages(k *k8s.K8sCluster) (*types.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	allObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		for _, o := range d.Objects {
			allObjects[o.GetK8sRef()] = o
		}
	}

	containersAndImages := make(map[k8s2.ObjectRef][]types.FixedImage)
	for _, fi := range c.Images.seenImages {
		_, ok := allObjects[*fi.Object]
		if !ok {
			dew.AddError(*fi.Object, fmt.Errorf("object not found while trying to associate image with deployed object"))
			continue
		}

		containersAndImages[*fi.Object] = append(containersAndImages[*fi.Object], fi)
	}

	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

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

	appliedObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	var mutex sync.Mutex

	for ref, containers := range containersAndImages {
		ref := ref
		containers := containers
		wp.Submit(func() error {
			newObject, err := c.doReplaceObject(k, dew, ref, func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
				return doPokeImage(containers, o)
			})
			if err != nil {
				dew.AddError(ref, err)
			} else {
				mutex.Lock()
				defer mutex.Unlock()
				appliedObjects[ref] = newObject
			}
			return nil
		})
	}
	err := wp.StopWait(false)
	if err != nil {
		return nil, err
	}

	du := utils2.NewDiffUtil(dew, c.Deployments, c.RemoteObjects, appliedObjects)
	du.Diff(k)

	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     c.Images.seenImages,
	}, nil
}

func (c *DeploymentCollection) Downscale(k *k8s.K8sCluster) (*types.CommandResult, error) {
	dew := utils2.NewDeploymentErrorsAndWarnings()

	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

	appliedObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	var deletedObjects []k8s2.ObjectRef
	var mutex sync.Mutex

	for _, d := range c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		for _, o := range d.Objects {
			o := o
			ref := o.GetK8sRef()
			if utils2.IsDownscaleDelete(o) {
				wp.Submit(func() error {
					apiWarnings, err := k.DeleteSingleObject(ref, k8s.DeleteOptions{IgnoreNotFoundError: true})
					dew.AddApiWarnings(ref, apiWarnings)
					if err != nil {
						return err
					}
					mutex.Lock()
					defer mutex.Unlock()
					deletedObjects = append(deletedObjects, ref)
					return nil
				})
			} else {
				wp.Submit(func() error {
					o2, err := c.doReplaceObject(k, dew, ref, func(remote *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
						return utils2.DownscaleObject(remote, o)
					})
					if err != nil {
						return err
					}
					mutex.Lock()
					defer mutex.Unlock()
					appliedObjects[ref] = o2
					return nil
				})
			}
		}
	}

	err := wp.StopWait(false)
	if err != nil {
		return nil, err
	}

	du := utils2.NewDiffUtil(dew, c.Deployments, c.RemoteObjects, appliedObjects)
	du.Diff(k)

	return &types.CommandResult{
		NewObjects:     du.NewObjects,
		ChangedObjects: du.ChangedObjects,
		DeletedObjects: deletedObjects,
		Errors:         dew.GetErrorsList(),
		Warnings:       dew.GetWarningsList(),
		SeenImages:     c.Images.seenImages,
	}, nil
}

func (c *DeploymentCollection) Validate(k *k8s.K8sCluster) *types.ValidateResult {
	var result types.ValidateResult

	dew := utils2.NewDeploymentErrorsAndWarnings()

	a := utils2.NewApplyUtil(dew, c, k, utils2.ApplyUtilOptions{})
	h := utils2.NewHooksUtil(a)
	for _, d := range c.Deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}
		for _, o := range d.Objects {
			hook := h.GetHook(o)
			if hook != nil && !hook.IsPersistent() {
				continue
			}

			ref := o.GetK8sRef()

			remoteObject := c.GetRemoteObject(ref)
			if remoteObject == nil {
				result.Errors = append(result.Errors, types.DeploymentError{Ref: ref, Error: "object not found"})
				continue
			}
			r := validation.ValidateObject(remoteObject, true)
			result.Errors = append(result.Errors, r.Errors...)
			result.Warnings = append(result.Warnings, r.Warnings...)
			result.Results = append(result.Results, r.Results...)
		}
	}

	result.Warnings = append(result.Warnings, dew.GetWarningsList()...)
	result.Errors = append(result.Errors, dew.GetErrorsList()...)

	return &result
}

func (c *DeploymentCollection) FindDeleteObjects(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	return utils2.FindObjectsForDelete(k, c.getFilteredRemoteObjects(c.inclusion), c.inclusion.HasType("tags"), nil)
}

func (c *DeploymentCollection) FindOrphanObjects(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	log.Infof("Searching for orphan objects")
	return utils2.FindObjectsForDelete(k, c.getFilteredRemoteObjects(c.inclusion), c.inclusion.HasType("tags"), c.localObjectRefs())
}

func (c *DeploymentCollection) doReplaceObject(k *k8s.K8sCluster, dew *utils2.DeploymentErrorsAndWarnings, ref k8s2.ObjectRef, callback func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error)) (*uo.UnstructuredObject, error) {
	firstCall := true
	for true {
		var remote *uo.UnstructuredObject
		if firstCall {
			remote = c.GetRemoteObject(ref)
		} else {
			o2, apiWarnings, err := k.GetSingleObject(ref)
			dew.AddApiWarnings(ref, apiWarnings)
			if err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
			remote = o2
		}
		if remote == nil {
			return remote, nil
		}
		firstCall = false

		remoteCopy := remote.Clone()
		modified, err := callback(remoteCopy)
		if err != nil {
			return nil, err
		}
		if reflect.DeepEqual(remote.Object, modified.Object) {
			return remote, nil
		}

		result, apiWarnings, err := k.UpdateObject(modified, k8s.UpdateOptions{})
		dew.AddApiWarnings(ref, apiWarnings)
		if err != nil {
			if errors.IsConflict(err) {
				log.Warningf("Conflict while patching %s. Retrying...", ref.String())
				continue
			} else {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("unexpected end of loop")
}

func (c *DeploymentCollection) getFilteredRemoteObjects(inclusion *utils.Inclusion) []*uo.UnstructuredObject {
	var ret []*uo.UnstructuredObject

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, o := range c.RemoteObjects {
		iv := c.getInclusionEntries(o)
		if inclusion.CheckIncluded(iv, false) {
			ret = append(ret, o)
		}
	}

	return ret
}

func (c *DeploymentCollection) getInclusionEntries(o *uo.UnstructuredObject) []utils.InclusionEntry {
	var iv []utils.InclusionEntry
	for _, v := range o.GetK8sLabelsWithRegex("^kluctl.io/tag-\\d+$") {
		iv = append(iv, utils.InclusionEntry{
			Type:  "tag",
			Value: v,
		})
	}

	if itemDir := o.GetK8sAnnotation("kluctl.io/kustomize_dir"); itemDir != nil {
		iv = append(iv, utils.InclusionEntry{
			Type:  "deploymentItemDir",
			Value: *itemDir,
		})
	}
	return iv
}

func (c *DeploymentCollection) FindRenderedImages() map[k8s2.ObjectRef][]string {
	ret := make(map[k8s2.ObjectRef][]string)
	for _, d := range c.Deployments {
		for _, o := range d.Objects {
			ref := o.GetK8sRef()
			l, ok, _ := o.GetNestedObjectList("spec", "template", "spec", "containers")
			if !ok {
				continue
			}
			for _, c := range l {
				image, ok, _ := c.GetNestedString("image")
				if !ok {
					continue
				}
				ret[ref] = append(ret[ref], image)
			}
		}
	}
	return ret
}
