package utils

import (
	errors2 "errors"
	"fmt"
	"github.com/codablock/kluctl/pkg/deployment"
	"github.com/codablock/kluctl/pkg/diff"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/validation"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"reflect"
	"sync"
	"time"
)

type ApplyUtilOptions struct {
	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	DryRun              bool
	AbortOnError        bool
	HookTimeout         time.Duration
}

type ApplyUtil struct {
	dew                  *DeploymentErrorsAndWarnings
	deploymentCollection *deployment.DeploymentCollection
	ru                   *RemoteObjectUtils
	k                    *k8s.K8sCluster
	o                    ApplyUtilOptions

	AppliedObjects     map[k8s2.ObjectRef]*uo.UnstructuredObject
	appliedHookObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	deletedObjects     map[k8s2.ObjectRef]bool
	deletedHookObjects map[k8s2.ObjectRef]bool
	abortSignal        bool
	deployedNewCRD     bool
	mutex              sync.Mutex
}

func NewApplyUtil(dew *DeploymentErrorsAndWarnings, deploymentCollection *deployment.DeploymentCollection, ru *RemoteObjectUtils, k *k8s.K8sCluster, o ApplyUtilOptions) *ApplyUtil {
	return &ApplyUtil{
		dew:                  dew,
		deploymentCollection: deploymentCollection,
		ru:                   ru,
		k:                    k,
		o:                    o,
		AppliedObjects:       map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		appliedHookObjects:   map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		deletedObjects:       map[k8s2.ObjectRef]bool{},
		deletedHookObjects:   map[k8s2.ObjectRef]bool{},
		deployedNewCRD:       true, // assume someone deployed CRDs in the meantime
	}
}

func (a *ApplyUtil) handleResult(appliedObject *uo.UnstructuredObject, hook bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	ref := appliedObject.GetK8sRef()
	if hook {
		a.appliedHookObjects[ref] = appliedObject
	} else {
		a.AppliedObjects[ref] = appliedObject
	}
}

func (a *ApplyUtil) handleApiWarnings(ref k8s2.ObjectRef, warnings []k8s.ApiWarning) {
	a.dew.AddApiWarnings(ref, warnings)
}

func (a *ApplyUtil) HandleWarning(ref k8s2.ObjectRef, warning error) {
	a.dew.AddWarning(ref, warning)
}

func (a *ApplyUtil) HandleError(ref k8s2.ObjectRef, err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.o.AbortOnError {
		a.abortSignal = true
	}

	a.dew.AddError(ref, err)
}

func (a *ApplyUtil) HadError(ref k8s2.ObjectRef) bool {
	return a.dew.HadError(ref)
}

func (a *ApplyUtil) DeleteObject(ref k8s2.ObjectRef, hook bool) bool {
	o := k8s.DeleteOptions{
		ForceDryRun: a.o.DryRun,
	}
	apiWarnings, err := a.k.DeleteSingleObject(ref, o)
	a.handleApiWarnings(ref, apiWarnings)
	if err != nil {
		if !errors.IsNotFound(err) {
			a.HandleError(ref, err)
		}
		return false
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()
	if hook {
		a.deletedHookObjects[ref] = true
	} else {
		a.deletedObjects[ref] = true
	}
	return true
}

func (a *ApplyUtil) retryApplyForceReplace(x *uo.UnstructuredObject, hook bool, applyError error) {
	ref := x.GetK8sRef()
	log2 := log.WithField("ref", ref)

	if !a.o.ForceReplaceOnError {
		a.HandleError(ref, applyError)
		return
	}

	log2.Warningf("Patching failed, retrying by deleting and re-applying")

	if !a.DeleteObject(ref, hook) {
		return
	}

	if !a.o.DryRun {
		o := k8s.PatchOptions{
			ForceDryRun: a.o.DryRun,
		}
		r, apiWarnings, err := a.k.PatchObject(x, o)
		a.handleApiWarnings(ref, apiWarnings)
		if err != nil {
			a.HandleError(ref, err)
			return
		}
		a.handleResult(r, hook)
	} else {
		a.handleResult(x, hook)
	}
}

func (a *ApplyUtil) retryApplyWithReplace(x *uo.UnstructuredObject, hook bool, remoteObject *uo.UnstructuredObject, applyError error) {
	ref := x.GetK8sRef()
	log2 := log.WithField("ref", ref)

	if !a.o.ReplaceOnError || remoteObject == nil {
		a.HandleError(ref, applyError)
		return
	}

	log2.Warningf("Patching failed, retrying with replace instead of patch")

	rv := remoteObject.GetK8sResourceVersion()
	x2 := x.Clone()
	x2.SetK8sResourceVersion(rv)

	o := k8s.UpdateOptions{
		ForceDryRun: a.o.DryRun,
	}

	r, apiWarnings, err := a.k.UpdateObject(x, o)
	a.handleApiWarnings(ref, apiWarnings)
	if err != nil {
		a.retryApplyForceReplace(x, hook, err)
		return
	}
	a.handleResult(r, hook)
}

func (a *ApplyUtil) retryApplyWithConflicts(x *uo.UnstructuredObject, hook bool, remoteObject *uo.UnstructuredObject, applyError error) {
	ref := x.GetK8sRef()

	if remoteObject == nil {
		a.HandleError(ref, applyError)
		return
	}

	var x2 *uo.UnstructuredObject
	if !a.o.ForceApply {
		var statusError *errors.StatusError
		if !errors2.As(applyError, &statusError) {
			a.HandleError(ref, applyError)
			return
		}

		x3, lostOwnership, err := diff.ResolveFieldManagerConflicts(x, remoteObject, statusError.ErrStatus)
		if err != nil {
			a.HandleError(ref, err)
			return
		}
		for _, lo := range lostOwnership {
			a.dew.AddWarning(ref, fmt.Errorf("%s. Not updating field '%s' as we lost field ownership", lo.Message, lo.Field))
		}
		x2 = x3
	} else {
		x2 = x
	}

	options := k8s.PatchOptions{
		ForceDryRun: a.o.DryRun,
		ForceApply:  true,
	}
	r, apiWarnings, err := a.k.PatchObject(x2, options)
	a.handleApiWarnings(ref, apiWarnings)
	if err != nil {
		// We didn't manage to solve it, better to abort (and not retry with replace!)
		a.HandleError(ref, err)
		return
	}
	a.handleResult(r, hook)
}

func (a *ApplyUtil) ApplyObject(x *uo.UnstructuredObject, replaced bool, hook bool) {
	ref := x.GetK8sRef()
	log2 := log.WithField("ref", ref)
	log2.Debugf("applying object")

	x = a.k.FixObjectForPatch(x)
	remoteObject := a.ru.GetRemoteObject(ref)

	if a.o.DryRun && replaced && remoteObject != nil {
		// Let's simulate that this object was deleted in dry-run mode. If we'd actually try a dry-run apply with
		// this object, it might fail as it is expected to not exist.
		a.handleResult(x, hook)
		return
	}

	options := k8s.PatchOptions{
		ForceDryRun: a.o.DryRun,
	}
	r, apiWarnings, err := a.k.PatchObject(x, options)
	retry, err := a.handleNewCRDs(r, err)
	if retry {
		r, apiWarnings, err = a.k.PatchObject(x, options)
	}
	a.handleApiWarnings(ref, apiWarnings)
	if err == nil {
		a.handleResult(r, hook)
	} else if meta.IsNoMatchError(err) {
		a.HandleError(ref, err)
	} else if errors.IsConflict(err) {
		a.retryApplyWithConflicts(x, hook, remoteObject, err)
	} else if errors.IsInternalError(err) {
		a.HandleError(ref, err)
	} else {
		a.retryApplyWithReplace(x, hook, remoteObject, err)
	}
}

func (a *ApplyUtil) handleNewCRDs(x *uo.UnstructuredObject, err error) (bool, error) {
	if err != nil && meta.IsNoMatchError(err) {
		// maybe this was a resource for which the CRD was only deployed recently, so we should do rediscovery and then
		// retry the patch
		a.mutex.Lock()
		defer a.mutex.Unlock()
		if a.deployedNewCRD {
			a.deployedNewCRD = false
			err = a.k.RediscoverResources()
			if err != nil {
				return false, err
			}
			return true, nil
		}
	} else if err == nil {
		ref := x.GetK8sRef()
		if ref.GVK.Group == "apiextensions.k8s.io" && ref.GVK.Kind == "CustomResourceDefinition" {
			// this is a freshly deployed CRD, so we must perform rediscovery in case an api resource can't be found
			a.mutex.Lock()
			defer a.mutex.Unlock()
			a.deployedNewCRD = true
			return true, nil
		}
		return false, nil
	}
	return false, err
}

func (a *ApplyUtil) WaitHook(ref k8s2.ObjectRef) bool {
	if a.o.DryRun {
		return true
	}

	log2 := log.WithField("ref", ref)
	log2.Debugf("Waiting for hook to get ready")

	lastLogTime := time.Now()
	didLog := false
	startTime := time.Now()
	for true {
		o, apiWarnings, err := a.k.GetSingleObject(ref)
		a.handleApiWarnings(ref, apiWarnings)
		if err != nil {
			if errors.IsNotFound(err) {
				if didLog {
					log2.Warningf("Cancelled waiting for hook as it disappeared while waiting for it")
				}
				a.HandleError(ref, fmt.Errorf("object disappeared while waiting for it to become ready"))
				return false
			}
			a.HandleError(ref, err)
			return false
		}
		v := validation.ValidateObject(o, false)
		if v.Ready {
			if didLog {
				log2.Infof("Finished waiting for hook")
			}
			return true
		}
		if len(v.Errors) != 0 {
			if didLog {
				log2.Warningf("Cancelled waiting for hook due to errors")
			}
			for _, e := range v.Errors {
				a.HandleError(ref, fmt.Errorf(e.Error))
			}
			return false
		}

		if a.o.HookTimeout != 0 && time.Now().Sub(startTime) >= a.o.HookTimeout {
			err := fmt.Errorf("timed out while waiting for hook")
			log2.Warningf(err.Error())
			a.HandleError(ref, err)
			return false
		}

		if !didLog {
			log2.Infof("Waiting for hook to get ready...")
			didLog = true
			lastLogTime = time.Now()
		} else if didLog && time.Now().Sub(lastLogTime) >= 10*time.Second {
			log2.Infof("Still waiting for hook to get ready (%s)...", time.Now().Sub(startTime).String())
			lastLogTime = time.Now()
		}

		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func (a *ApplyUtil) applyDeploymentItem(d *deployment.DeploymentItem) {
	if !d.CheckInclusionForDeploy() {
		a.DoLog(d, log.InfoLevel, "Skipping")
		return
	}

	var toDelete []k8s2.ObjectRef
	for _, x := range d.Config.DeleteObjects {
		for _, gvk := range a.k.GetGVKs(x.Group, x.Version, x.Kind) {
			ref := k8s2.ObjectRef{
				GVK:       gvk,
				Name:      x.Name,
				Namespace: x.Namespace,
			}
			toDelete = append(toDelete, ref)
		}
	}
	if len(toDelete) != 0 {
		log.Infof("Deleting %d objects", len(toDelete))
		for _, ref := range toDelete {
			a.DeleteObject(ref, false)
		}
	}

	initialDeploy := true
	for _, o := range d.Objects {
		if a.ru.GetRemoteObject(o.GetK8sRef()) != nil {
			initialDeploy = false
		}
	}

	h := HooksUtil{a: a}

	if initialDeploy {
		h.RunHooks(d, []string{"pre-deploy-initial", "pre-deploy"})
	} else {
		h.RunHooks(d, []string{"pre-deploy-upgrade", "pre-deploy"})
	}

	var applyObjects []*uo.UnstructuredObject
	for _, o := range d.Objects {
		if h.GetHook(o) != nil {
			continue
		}
		applyObjects = append(applyObjects, o)
	}

	if len(applyObjects) != 0 {
		a.DoLog(d, log.InfoLevel, "Applying %d objects", len(applyObjects))
	}
	startTime := time.Now()
	didLog := false
	for i, o := range applyObjects {
		a.ApplyObject(o, false, false)
		if time.Now().Sub(startTime) >= 10*time.Second || (didLog && i == len(applyObjects)-1) {
			a.DoLog(d, log.InfoLevel, "...applied %d of %d objects", i+1, len(applyObjects))
			startTime = time.Now()
			didLog = true
		}
	}

	if initialDeploy {
		h.RunHooks(d, []string{"post-deploy-initial", "post-deploy"})
	} else {
		h.RunHooks(d, []string{"post-deploy-upgrade", "post-deploy"})
	}
}

func (a *ApplyUtil) ApplyDeployments() {
	log.Infof("Running server-side apply for all objects")

	wp := utils.NewDebuggerAwareWorkerPool(16)
	defer wp.StopWait(false)

	previousWasBarrier := false
	for _, d_ := range a.deploymentCollection.Deployments {
		d := d_
		if a.abortSignal {
			break
		}
		if previousWasBarrier {
			log.Infof("Waiting on barrier...")
			_ = wp.StopWait(true)
		}

		previousWasBarrier = d.Config.Barrier != nil && *d.Config.Barrier

		wp.Submit(func() error {
			a.applyDeploymentItem(d)
			return nil
		})
	}
	_ = wp.StopWait(false)
}

func (a *ApplyUtil) ReplaceObject(ref k8s2.ObjectRef, firstVersion *uo.UnstructuredObject, callback func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error)) {
	firstCall := true
	for true {
		var remote *uo.UnstructuredObject
		if firstCall && firstVersion != nil {
			remote = firstVersion
		} else {
			o2, apiWarnings, err := a.k.GetSingleObject(ref)
			a.dew.AddApiWarnings(ref, apiWarnings)
			if err != nil && !errors.IsNotFound(err) {
				a.HandleError(ref, err)
				return
			}
			remote = o2
		}
		if remote == nil {
			a.handleResult(remote, false)
			return
		}
		firstCall = false

		remoteCopy := remote.Clone()
		modified, err := callback(remoteCopy)
		if err != nil {
			a.HandleError(ref, err)
			return
		}
		if reflect.DeepEqual(remote.Object, modified.Object) {
			a.handleResult(remote, false)
			return
		}

		result, apiWarnings, err := a.k.UpdateObject(modified, k8s.UpdateOptions{})
		a.dew.AddApiWarnings(ref, apiWarnings)
		if err != nil {
			if errors.IsConflict(err) {
				log.Warningf("Conflict while patching %s. Retrying...", ref.String())
				continue
			} else {
				a.HandleError(ref, err)
				return
			}
		}
		a.handleResult(result, false)
		return
	}
	a.HandleError(ref, fmt.Errorf("unexpected end of loop"))
}

func (a *ApplyUtil) DoLog(d *deployment.DeploymentItem, level log.Level, s string, f ...interface{}) {
	s = fmt.Sprintf("%s: %s", d.RelToProjectItemDir, fmt.Sprintf(s, f...))
	log.StandardLogger().Logf(level, s)
}

func (a *ApplyUtil) GetDeletedObjectsList() []k8s2.ObjectRef {
	var ret []k8s2.ObjectRef
	for ref := range a.deletedObjects {
		ret = append(ret, ref)
	}
	return ret
}

func (a *ApplyUtil) GetAppliedHookObjects() []*types.RefAndObject {
	var ret []*types.RefAndObject
	for _, o := range a.appliedHookObjects {
		ret = append(ret, &types.RefAndObject{
			Ref:    o.GetK8sRef(),
			Object: o,
		})
	}
	return ret
}
