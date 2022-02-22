package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/diff"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/seal"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/validation"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	remoteObjects map[types.ObjectRef]*unstructured.Unstructured

	errors   map[types.ObjectRef]map[types.DeploymentError]bool
	warnings map[types.ObjectRef]map[types.DeploymentError]bool
	mutex    sync.Mutex
}

func NewDeploymentCollection(project *DeploymentProject, images *Images, inclusion *utils.Inclusion, renderDir string, forSeal bool) (*DeploymentCollection, error) {
	dc := &DeploymentCollection{
		project:       project,
		images:        images,
		inclusion:     inclusion,
		RenderDir:     renderDir,
		forSeal:       forSeal,
		remoteObjects: map[types.ObjectRef]*unstructured.Unstructured{},
		errors:        map[types.ObjectRef]map[types.DeploymentError]bool{},
		warnings:      map[types.ObjectRef]map[types.DeploymentError]bool{},
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

	wp := utils.NewDebuggerAwareWorkerPool(8)
	defer wp.StopWait(false)

	for _, d_ := range c.deployments {
		d := d_
		wp.Submit(func() error {
			err := d.buildKustomize()
			if err != nil {
				return fmt.Errorf("building kustomize objects for %s failed. %w", *d.dir, err)
			}
			err = d.postprocessAndLoadObjects(k)
			if err != nil {
				return fmt.Errorf("postprocessing kustomize objects for %s failed. %w", *d.dir, err)
			}
			return nil
		})
	}
	err := wp.StopWait(false)
	if err != nil {
		return err
	}

	return nil
}

func (c *DeploymentCollection) updateRemoteObjects(k *k8s.K8sCluster) error {
	if k == nil {
		return nil
	}

	notFoundRefsMap := make(map[types.ObjectRef]bool)
	var notFoundRefsList []types.ObjectRef
	for ref := range c.localObjectsByRef() {
		if _, ok := c.remoteObjects[ref]; !ok {
			if _, ok = notFoundRefsMap[ref]; !ok {
				notFoundRefsMap[ref] = true
				notFoundRefsList = append(notFoundRefsList, ref)
			}
		}
	}

	log.Infof("Updating %d remote objects", len(notFoundRefsList))

	r, apiWarnings, err := k.GetObjectsByRefs(notFoundRefsList)
	for ref, aw := range apiWarnings {
		c.addApiWarnings(ref, aw)
	}
	if err != nil {
		return err
	}
	for _, o := range r {
		c.remoteObjects[types.RefFromObject(o)] = o
	}
	return nil
}

func (c *DeploymentCollection) ForgetRemoteObject(ref types.ObjectRef) {
	delete(c.remoteObjects, ref)
}

func (c *DeploymentCollection) localObjectsByRef() map[types.ObjectRef]bool {
	ret := make(map[types.ObjectRef]bool)
	for _, d := range c.deployments {
		for _, o := range d.objects {
			ret[types.RefFromObject(o)] = true
		}
	}
	return ret
}

func (c *DeploymentCollection) localObjectRefs() []types.ObjectRef {
	var ret []types.ObjectRef
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
	var appliedHookObjectsList []*unstructured.Unstructured
	for _, o := range appliedHookObjects {
		appliedHookObjectsList = append(appliedHookObjectsList, o)
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
	var appliedHookObjectsList []*unstructured.Unstructured
	for _, o := range appliedHookObjects {
		appliedHookObjectsList = append(appliedHookObjectsList, o)
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
	}, nil
}

func (c *DeploymentCollection) Downscale(k *k8s.K8sCluster) (*types.CommandResult, error) {
	wp := utils.NewWorkerPoolWithErrors(8)
	defer wp.StopWait(false)

	appliedObjects := make(map[types.ObjectRef]*unstructured.Unstructured)
	var deletedObjects []types.ObjectRef
	var mutex sync.Mutex

	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}
		for _, o := range d.objects {
			o := o
			ref := types.RefFromObject(o)
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
					o2, err := c.doReplaceObject(k, ref, func(remote *unstructured.Unstructured) (*unstructured.Unstructured, error) {
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

			ref := types.RefFromObject(o)

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

func (c *DeploymentCollection) FindDeleteObjects(k *k8s.K8sCluster) ([]types.ObjectRef, error) {
	labels := c.project.getDeleteByLabels()
	return k8s.FindObjectsForDelete(k, labels, c.inclusion, nil)
}

func (c *DeploymentCollection) FindOrphanObjects(k *k8s.K8sCluster) ([]types.ObjectRef, error) {
	log.Infof("Searching for orphan objects")
	labels := c.project.getDeleteByLabels()
	return k8s.FindObjectsForDelete(k, labels, c.inclusion, c.localObjectRefs())
}

func (c *DeploymentCollection) doApply(k *k8s.K8sCluster, o applyUtilOptions) (map[types.ObjectRef]*unstructured.Unstructured, map[types.ObjectRef]*unstructured.Unstructured, []types.ObjectRef, error) {
	au := newApplyUtil(c, k, o)
	au.applyDeployments()
	var deletedObjects []types.ObjectRef
	for ref := range au.deletedObjects {
		deletedObjects = append(deletedObjects, ref)
	}
	return au.appliedObjects, au.appliedHookObjects, deletedObjects, nil
}

func (c *DeploymentCollection) doDiff(k *k8s.K8sCluster, appliedObjects map[types.ObjectRef]*unstructured.Unstructured, ignoreTags bool, ignoreLabels bool, ignoreAnnotations bool) ([]*unstructured.Unstructured, []*types.ChangedObject, error) {
	diffObjects := make(map[types.ObjectRef]*unstructured.Unstructured)
	normalizedDiffObjects := make(map[types.ObjectRef]*unstructured.Unstructured)
	normalizedRemoteObjects := make(map[types.ObjectRef]*unstructured.Unstructured)

	for _, d := range c.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}

		ignoreForDiffs := d.project.getIgnoreForDiffs(ignoreTags, ignoreLabels, ignoreAnnotations)
		for _, o := range d.objects {
			ref := types.RefFromObject(o)
			if _, ok := appliedObjects[ref]; !ok {
				continue
			}
			diffObjects[ref] = appliedObjects[ref]
			normalizedDiffObjects[ref] = diff.NormalizeObject(k, diffObjects[ref], ignoreForDiffs, diffObjects[ref])
			if r, ok := c.remoteObjects[ref]; ok {
				normalizedRemoteObjects[ref] = diff.NormalizeObject(k, r, ignoreForDiffs, diffObjects[ref])
			}
		}
	}

	wp := utils.NewDebuggerAwareWorkerPool(8)
	defer wp.StopWait(false)

	var newObjects []*unstructured.Unstructured
	var changedObjects []*types.ChangedObject
	var mutex sync.Mutex

	for ref, o_ := range diffObjects {
		o := o_
		normalizedRemoteObject, ok := normalizedRemoteObjects[ref]
		if !ok {
			newObjects = append(newObjects, o)
			continue
		}
		normalizedDiffObject, _ := normalizedDiffObjects[ref]
		remoteObject, _ := c.remoteObjects[ref]
		wp.Submit(func() error {
			changes, err := diff.Diff(normalizedRemoteObject, normalizedDiffObject)
			if err != nil {
				return err
			}
			if len(changes) == 0 {
				return nil
			}
			mutex.Lock()
			defer mutex.Unlock()
			changedObjects = append(changedObjects, &types.ChangedObject{
				NewObject: o,
				OldObject: remoteObject,
				Changes:   changes,
			})
			return nil
		})
	}

	err := wp.StopWait(false)
	if err != nil {
		return nil, nil, err
	}

	return newObjects, changedObjects, nil
}

func (c *DeploymentCollection) doReplaceObject(k *k8s.K8sCluster, ref types.ObjectRef, callback func(o *unstructured.Unstructured) (*unstructured.Unstructured, error)) (*unstructured.Unstructured, error) {
	firstCall := true
	for true {
		var remote *unstructured.Unstructured
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

		modified, err := callback(remote)
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

func (c *DeploymentCollection) addWarning(ref types.ObjectRef, warning error) {
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

func (c *DeploymentCollection) addError(ref types.ObjectRef, err error) {
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

func (c *DeploymentCollection) addApiWarnings(ref types.ObjectRef, warnings []k8s.ApiWarning) {
	for _, w := range warnings {
		c.addWarning(ref, fmt.Errorf(w.Text))
	}
}

func (c *DeploymentCollection) hadError(ref types.ObjectRef) bool {
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
	c.warnings = map[types.ObjectRef]map[types.DeploymentError]bool{}
	c.errors = map[types.ObjectRef]map[types.DeploymentError]bool{}
}

func (c *DeploymentCollection) getRemoteObject(ref types.ObjectRef) *unstructured.Unstructured {
	o, _ := c.remoteObjects[ref]
	return o
}

func (c *DeploymentCollection) FindRenderedImages() map[types.ObjectRef][]string {
	ret := make(map[types.ObjectRef][]string)
	for _, d := range c.deployments {
		for _, o := range d.objects {
			ref := types.RefFromObject(o)
			u := uo.FromUnstructured(o)
			l, ok, _ := u.GetNestedObjectList("spec", "template", "spec", "containers")
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
