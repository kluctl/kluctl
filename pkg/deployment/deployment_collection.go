package deployment

import (
	"context"
	"fmt"
	"github.com/codablock/kluctl/pkg/diff"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/seal"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/validation"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"k8s.io/apimachinery/pkg/api/errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
)

type DeploymentCollection struct {
	project   *DeploymentProject
	images    *Images
	inclusion *utils.Inclusion
	RenderDir string
	forSeal   bool

	deployments   []*deploymentItem
	remoteObjects map[k8s2.ObjectRef]*uo.UnstructuredObject

	errors   map[k8s2.ObjectRef]map[types.DeploymentError]bool
	warnings map[k8s2.ObjectRef]map[types.DeploymentError]bool
	mutex    sync.Mutex
}

func NewDeploymentCollection(project *DeploymentProject, images *Images, inclusion *utils.Inclusion, renderDir string, forSeal bool) (*DeploymentCollection, error) {
	dc := &DeploymentCollection{
		project:       project,
		images:        images,
		inclusion:     inclusion,
		RenderDir:     renderDir,
		forSeal:       forSeal,
		remoteObjects: map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		errors:        map[k8s2.ObjectRef]map[types.DeploymentError]bool{},
		warnings:      map[k8s2.ObjectRef]map[types.DeploymentError]bool{},
	}

	indexes := make(map[string]int)
	deployments, err := dc.collectDeployments(project, indexes)
	if err != nil {
		return nil, err
	}
	dc.deployments = deployments
	return dc, nil
}

func (c *DeploymentCollection) createBarrierDummy(project *DeploymentProject) *deploymentItem {
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

func (c *DeploymentCollection) collectDeployments(project *DeploymentProject, indexes map[string]int) ([]*deploymentItem, error) {
	var ret []*deploymentItem

	for i, diConfig := range project.config.Deployments {
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

	for _, d := range c.deployments {
		err := d.render(k, wp)
		if err != nil {
			return err
		}
	}
	err := wp.StopWait(true)
	if err != nil {
		return err
	}

	for _, d := range c.deployments {
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

func (c *DeploymentCollection) Seal(sealer *seal.Sealer) error {
	if c.project.config.SealedSecrets.OutputPattern == nil {
		return fmt.Errorf("sealedSecrets.outputPattern is not defined")
	}

	err := filepath.WalkDir(c.RenderDir, func(p string, d fs.DirEntry, err error) error {
		if !strings.HasSuffix(p, sealmeExt) {
			return nil
		}

		relPath, err := filepath.Rel(c.RenderDir, p)
		if err != nil {
			return err
		}
		targetDir := filepath.Join(c.project.sealedSecretsDir, filepath.Dir(relPath))
		targetFile := filepath.Join(targetDir, *c.project.config.SealedSecrets.OutputPattern, filepath.Base(p))
		targetFile = targetFile[:len(targetFile)-len(sealmeExt)]
		err = sealer.SealFile(p, targetFile)
		if err != nil {
			return fmt.Errorf("failed sealing %s: %w", filepath.Base(p), err)
		}
		return nil
	})
	return err
}

func (c *DeploymentCollection) resolveSealedSecrets() error {
	if c.forSeal {
		return nil
	}

	for _, d := range c.deployments {
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

	for _, d_ := range c.deployments {
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

func (c *DeploymentCollection) updateRemoteObjects(k *k8s.K8sCluster) error {
	if k == nil {
		return nil
	}

	log.Infof("Getting remote objects by commonLabels")
	allObjects, apiWarnings, err := k.ListAllObjects([]string{"get"}, "", c.project.getCommonLabels(), false)
	for gvk, aw := range apiWarnings {
		c.addApiWarnings(k8s2.ObjectRef{GVK: gvk}, aw)
	}
	if err != nil {
		return err
	}

	for _, o := range allObjects {
		c.remoteObjects[o.GetK8sRef()] = o
	}

	notFoundRefsMap := make(map[k8s2.ObjectRef]bool)
	var notFoundRefsList []k8s2.ObjectRef
	for ref := range c.localObjectsByRef() {
		if _, ok := c.remoteObjects[ref]; !ok {
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
			c.addApiWarnings(ref, aw)
		}
		if err != nil {
			return err
		}
		for _, o := range r {
			c.remoteObjects[o.GetK8sRef()] = o
		}
	}
	return nil
}

func (c *DeploymentCollection) ForgetRemoteObject(ref k8s2.ObjectRef) {
	delete(c.remoteObjects, ref)
}

func (c *DeploymentCollection) localObjectsByRef() map[k8s2.ObjectRef]bool {
	ret := make(map[k8s2.ObjectRef]bool)
	for _, d := range c.deployments {
		for _, o := range d.objects {
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
	err = c.updateRemoteObjects(k)
	if err != nil {
		return err
	}
	return nil
}

func (c *DeploymentCollection) Deploy(k *k8s.K8sCluster, forceApply bool, replaceOnError bool, forceReplaceOnError, abortOnError bool, hookTimeout time.Duration) (*types.CommandResult, error) {
	c.clearErrorsAndWarnings()
	o := applyUtilOptions{
		forceApply:          forceApply,
		replaceOnError:      replaceOnError,
		forceReplaceOnError: forceReplaceOnError,
		dryRun:              k.DryRun,
		abortOnError:        abortOnError,
		hookTimeout:         hookTimeout,
	}
	appliedObjects, appliedHookObjects, deletedObjects, err := c.doApply(k, o)
	if err != nil {
		return nil, err
	}
	var appliedHookObjectsList []*types.RefAndObject
	for _, o := range appliedHookObjects {
		appliedHookObjectsList = append(appliedHookObjectsList, &types.RefAndObject{
			Ref:    o.GetK8sRef(),
			Object: o,
		})
	}
	newObjects, changedObjects, err := c.doDiff(k, appliedObjects, false, false, false)
	if err != nil {
		return nil, err
	}
	orphanObjects, err := c.FindOrphanObjects(k)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     newObjects,
		ChangedObjects: changedObjects,
		DeletedObjects: deletedObjects,
		HookObjects:    appliedHookObjectsList,
		OrphanObjects:  orphanObjects,
		Errors:         c.errorsList(),
		Warnings:       c.warningsList(),
		SeenImages:     c.images.seenImages,
	}, nil
}

func (c *DeploymentCollection) Diff(k *k8s.K8sCluster, forceApply bool, replaceOnError bool, forceReplaceOnError bool, ignoreTags bool, ignoreLabels bool, ignoreAnnotations bool) (*types.CommandResult, error) {
	c.clearErrorsAndWarnings()
	o := applyUtilOptions{
		forceApply:          forceApply,
		replaceOnError:      replaceOnError,
		forceReplaceOnError: forceReplaceOnError,
		dryRun:              true,
		abortOnError:        false,
		hookTimeout:         0,
	}
	appliedObjects, appliedHookObjects, deletedObjects, err := c.doApply(k, o)
	if err != nil {
		return nil, err
	}
	var appliedHookObjectsList []*types.RefAndObject
	for _, o := range appliedHookObjects {
		appliedHookObjectsList = append(appliedHookObjectsList, &types.RefAndObject{
			Ref:    o.GetK8sRef(),
			Object: o,
		})
	}
	newObjects, changedObjects, err := c.doDiff(k, appliedObjects, ignoreTags, ignoreLabels, ignoreAnnotations)
	if err != nil {
		return nil, err
	}
	orphanObjects, err := c.FindOrphanObjects(k)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     newObjects,
		ChangedObjects: changedObjects,
		DeletedObjects: deletedObjects,
		HookObjects:    appliedHookObjectsList,
		OrphanObjects:  orphanObjects,
		Errors:         c.errorsList(),
		Warnings:       c.warningsList(),
		SeenImages:     c.images.seenImages,
	}, nil
}

func (c *DeploymentCollection) PokeImages(k *k8s.K8sCluster) (*types.CommandResult, error) {
	allObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}
		for _, o := range d.objects {
			allObjects[o.GetK8sRef()] = o
		}
	}

	containersAndImages := make(map[k8s2.ObjectRef][]types.FixedImage)
	for _, fi := range c.images.seenImages {
		_, ok := allObjects[*fi.Object]
		if !ok {
			c.addError(*fi.Object, fmt.Errorf("object not found while trying to associate image with deployed object"))
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
			newObject, err := c.doReplaceObject(k, ref, func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
				return doPokeImage(containers, o)
			})
			if err != nil {
				c.addError(ref, err)
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

	newObjects, changedObjects, err := c.doDiff(k, appliedObjects, false, false, false)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     newObjects,
		ChangedObjects: changedObjects,
		Errors:         c.errorsList(),
		Warnings:       c.warningsList(),
		SeenImages:     c.images.seenImages,
	}, nil
}

func (c *DeploymentCollection) Downscale(k *k8s.K8sCluster) (*types.CommandResult, error) {
	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

	appliedObjects := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	var deletedObjects []k8s2.ObjectRef
	var mutex sync.Mutex

	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}
		for _, o := range d.objects {
			o := o
			ref := o.GetK8sRef()
			if isDownscaleDelete(o) {
				wp.Submit(func() error {
					apiWarnings, err := k.DeleteSingleObject(ref, k8s.DeleteOptions{IgnoreNotFoundError: true})
					c.addApiWarnings(ref, apiWarnings)
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
					o2, err := c.doReplaceObject(k, ref, func(remote *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
						return downscaleObject(remote, o)
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

	newObjects, changedObjects, err := c.doDiff(k, appliedObjects, false, false, false)
	if err != nil {
		return nil, err
	}
	return &types.CommandResult{
		NewObjects:     newObjects,
		ChangedObjects: changedObjects,
		DeletedObjects: deletedObjects,
		Errors:         c.errorsList(),
		Warnings:       c.warningsList(),
		SeenImages:     c.images.seenImages,
	}, nil
}

func (c *DeploymentCollection) Validate(k *k8s.K8sCluster) *types.ValidateResult {
	var result types.ValidateResult

	for _, m := range c.warnings {
		for e := range m {
			result.Warnings = append(result.Warnings, e)
		}
	}
	for _, m := range c.errors {
		for e := range m {
			result.Errors = append(result.Errors, e)
		}
	}

	a := newApplyUtil(c, k, applyUtilOptions{})
	h := hooksUtil{a: a}
	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}
		for _, o := range d.objects {
			hook := h.getHook(o)
			if hook != nil && !hook.isPersistent() {
				continue
			}

			ref := o.GetK8sRef()

			remoteObject := c.getRemoteObject(ref)
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

	return &result
}

func (c *DeploymentCollection) FindDeleteObjects(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	labels := c.project.getCommonLabels()
	return FindObjectsForDelete(k, labels, c.inclusion, nil)
}

func (c *DeploymentCollection) FindOrphanObjects(k *k8s.K8sCluster) ([]k8s2.ObjectRef, error) {
	log.Infof("Searching for orphan objects")
	labels := c.project.getCommonLabels()
	return FindObjectsForDelete(k, labels, c.inclusion, c.localObjectRefs())
}

func (c *DeploymentCollection) doApply(k *k8s.K8sCluster, o applyUtilOptions) (map[k8s2.ObjectRef]*uo.UnstructuredObject, map[k8s2.ObjectRef]*uo.UnstructuredObject, []k8s2.ObjectRef, error) {
	au := newApplyUtil(c, k, o)
	au.applyDeployments()
	var deletedObjects []k8s2.ObjectRef
	for ref := range au.deletedObjects {
		deletedObjects = append(deletedObjects, ref)
	}
	return au.appliedObjects, au.appliedHookObjects, deletedObjects, nil
}

func (c *DeploymentCollection) doDiff(k *k8s.K8sCluster, appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject, ignoreTags bool, ignoreLabels bool, ignoreAnnotations bool) ([]*types.RefAndObject, []*types.ChangedObject, error) {
	var newObjects []*types.RefAndObject
	var changedObjects []*types.ChangedObject
	var mutex sync.Mutex

	wp := utils.NewDebuggerAwareWorkerPool(8)
	defer wp.StopWait(false)

	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}

		ignoreForDiffs := d.project.getIgnoreForDiffs(ignoreTags, ignoreLabels, ignoreAnnotations)
		for _, o := range d.objects {
			o := o
			ref := o.GetK8sRef()
			ao, ok := appliedObjects[ref]
			if !ok {
				// if we can't even find it in appliedObjects, it probably ran into an error
				continue
			}
			ro := c.getRemoteObject(ref)

			if ao != nil && ro == nil {
				newObjects = append(newObjects, &types.RefAndObject{
					Ref:    ao.GetK8sRef(),
					Object: ao,
				})
			} else if ao == nil && ro != nil {
				// deleted?
				continue
			} else if ao == nil && ro == nil {
				// did not apply? (e.g. in downscale command)
				continue
			} else {
				wp.Submit(func() error {
					nao := diff.NormalizeObject(k, ao, ignoreForDiffs, o)
					nro := diff.NormalizeObject(k, ro, ignoreForDiffs, o)
					changes, err := diff.Diff(nro, nao)
					if err != nil {
						return err
					}
					if len(changes) == 0 {
						return nil
					}
					mutex.Lock()
					defer mutex.Unlock()
					changedObjects = append(changedObjects, &types.ChangedObject{
						Ref:       ref,
						NewObject: ao,
						OldObject: ro,
						Changes:   changes,
					})
					return nil
				})
			}
		}
	}

	err := wp.StopWait(false)
	if err != nil {
		return nil, nil, err
	}

	return newObjects, changedObjects, nil
}

func (c *DeploymentCollection) doReplaceObject(k *k8s.K8sCluster, ref k8s2.ObjectRef, callback func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error)) (*uo.UnstructuredObject, error) {
	firstCall := true
	for true {
		var remote *uo.UnstructuredObject
		if firstCall {
			remote = c.getRemoteObject(ref)
		} else {
			o2, apiWarnings, err := k.GetSingleObject(ref)
			c.addApiWarnings(ref, apiWarnings)
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
		c.addApiWarnings(ref, apiWarnings)
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

func (c *DeploymentCollection) addWarning(ref k8s2.ObjectRef, warning error) {
	de := types.DeploymentError{
		Ref:   ref,
		Error: warning.Error(),
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	m, ok := c.warnings[ref]
	if !ok {
		m = make(map[types.DeploymentError]bool)
		c.warnings[ref] = m
	}
	m[de] = true
}

func (c *DeploymentCollection) addError(ref k8s2.ObjectRef, err error) {
	de := types.DeploymentError{
		Ref:   ref,
		Error: err.Error(),
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	m, ok := c.errors[ref]
	if !ok {
		m = make(map[types.DeploymentError]bool)
		c.errors[ref] = m
	}
	m[de] = true
}

func (c *DeploymentCollection) addApiWarnings(ref k8s2.ObjectRef, warnings []k8s.ApiWarning) {
	for _, w := range warnings {
		c.addWarning(ref, fmt.Errorf(w.Text))
	}
}

func (c *DeploymentCollection) hadError(ref k8s2.ObjectRef) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.errors[ref]
	return ok
}

func (c *DeploymentCollection) errorsList() []types.DeploymentError {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var l []types.DeploymentError
	for _, m := range c.errors {
		for de := range m {
			l = append(l, de)
		}
	}
	return l
}

func (c *DeploymentCollection) warningsList() []types.DeploymentError {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var l []types.DeploymentError
	for _, m := range c.warnings {
		for de := range m {
			l = append(l, de)
		}
	}
	return l
}

func (c *DeploymentCollection) clearErrorsAndWarnings() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.warnings = map[k8s2.ObjectRef]map[types.DeploymentError]bool{}
	c.errors = map[k8s2.ObjectRef]map[types.DeploymentError]bool{}
}

func (c *DeploymentCollection) getRemoteObject(ref k8s2.ObjectRef) *uo.UnstructuredObject {
	o, _ := c.remoteObjects[ref]
	return o
}

func (c *DeploymentCollection) FindRenderedImages() map[k8s2.ObjectRef][]string {
	ret := make(map[k8s2.ObjectRef][]string)
	for _, d := range c.deployments {
		for _, o := range d.objects {
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
