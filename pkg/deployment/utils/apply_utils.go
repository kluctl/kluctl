package utils

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/diff"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type ApplyUtilOptions struct {
	ForceApply          bool
	ReplaceOnError      bool
	ForceReplaceOnError bool
	DryRun              bool
	AbortOnError        bool
	ReadinessTimeout    time.Duration
	NoWait              bool
}

type ApplyUtil struct {
	ctx context.Context

	dew                *DeploymentErrorsAndWarnings
	errorCount         int
	warningCount       int
	newObjects         map[k8s2.ObjectRef]*uo.UnstructuredObject
	appliedObjects     map[k8s2.ObjectRef]*uo.UnstructuredObject
	appliedHookObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	deletedObjects     map[k8s2.ObjectRef]bool
	deletedHookObjects map[k8s2.ObjectRef]bool
	mutex              sync.Mutex

	abortSignal   *atomic.Value
	allNamespaces *sync.Map

	ru   *RemoteObjectUtils
	k    *k8s.K8sCluster
	o    *ApplyUtilOptions
	sctx *status.StatusContext
}

type ApplyDeploymentsUtil struct {
	ctx context.Context

	dew         *DeploymentErrorsAndWarnings
	deployments []*deployment.DeploymentItem
	ru          *RemoteObjectUtils
	k           *k8s.K8sCluster
	o           *ApplyUtilOptions

	abortSignal atomic.Value

	// Used to track all created namespaces
	// All ApplyUtil instances write to this in parallel and we ignore that order might be unstable
	// This is only used to simulate dryRun apply into new namespaces
	allNamespaces sync.Map

	resultsMutex sync.Mutex
	results      []*ApplyUtil
}

func NewApplyDeploymentsUtil(ctx context.Context, dew *DeploymentErrorsAndWarnings, deployments []*deployment.DeploymentItem, ru *RemoteObjectUtils, k *k8s.K8sCluster, o *ApplyUtilOptions) *ApplyDeploymentsUtil {
	ret := &ApplyDeploymentsUtil{
		ctx:         ctx,
		dew:         dew,
		deployments: deployments,
		ru:          ru,
		k:           k,
		o:           o,
	}
	ret.abortSignal.Store(false)
	return ret
}

func (ad *ApplyDeploymentsUtil) NewApplyUtil(ctx context.Context, statusCtx *status.StatusContext) *ApplyUtil {
	ad.resultsMutex.Lock()
	defer ad.resultsMutex.Unlock()

	ret := &ApplyUtil{
		ctx:                ctx,
		dew:                ad.dew,
		newObjects:         map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		appliedObjects:     map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		appliedHookObjects: map[k8s2.ObjectRef]*uo.UnstructuredObject{},
		deletedObjects:     map[k8s2.ObjectRef]bool{},
		deletedHookObjects: map[k8s2.ObjectRef]bool{},
		abortSignal:        &ad.abortSignal,
		allNamespaces:      &ad.allNamespaces,
		ru:                 ad.ru,
		k:                  ad.k,
		o:                  ad.o,
		sctx:               statusCtx,
	}
	ad.results = append(ad.results, ret)
	return ret
}

func (a *ApplyUtil) handleResult(appliedObject *uo.UnstructuredObject, hook bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	ref := appliedObject.GetK8sRef()
	if hook {
		a.appliedHookObjects[ref] = appliedObject
	}
	a.appliedObjects[ref] = appliedObject

	if !hook && a.ru.GetRemoteObject(ref) == nil {
		a.newObjects[ref] = appliedObject
	}
}

func (a *ApplyUtil) handleApiWarnings(ref k8s2.ObjectRef, warnings []k8s.ApiWarning) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.dew.AddApiWarnings(ref, warnings)
	a.warningCount += len(warnings)
}

func (a *ApplyUtil) HandleWarning(ref k8s2.ObjectRef, warning error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.dew.AddWarning(ref, warning)
	a.warningCount++
}

func (a *ApplyUtil) HandleError(ref k8s2.ObjectRef, err error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if errors2.Is(err, context.DeadlineExceeded) || errors2.Is(err, context.Canceled) {
		a.abortSignal.Store(true)
	}

	if a.o.AbortOnError && a.abortSignal != nil {
		a.abortSignal.Store(true)
	}

	a.dew.AddError(ref, err)
	a.errorCount++
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

func (a *ApplyUtil) retryApplyForceReplace(x *uo.UnstructuredObject, hook bool, remoteObject *uo.UnstructuredObject, applyError error) {
	ref := x.GetK8sRef()

	if !a.o.ForceReplaceOnError {
		a.HandleError(ref, applyError)
		return
	}

	skipDelete := isSkipDelete(x)
	if remoteObject != nil {
		skipDelete = skipDelete || isSkipDelete(remoteObject)
	}
	if skipDelete {
		status.Warning(a.ctx, "skipped forced replace of %s", ref.String())
		a.HandleError(ref, applyError)
		return
	}

	warn := fmt.Errorf("patching %s failed, retrying by deleting and re-applying", ref.String())
	a.HandleWarning(ref, warn)
	status.Warning(a.ctx, warn.Error())

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

	if (!a.o.ReplaceOnError && !a.o.ForceReplaceOnError) || remoteObject == nil {
		a.HandleError(ref, applyError)
		return
	}

	warn := fmt.Errorf("patching %s failed, retrying with replace instead of patch", ref.String())
	a.HandleWarning(ref, warn)
	status.Warning(a.ctx, warn.Error())

	rv := remoteObject.GetK8sResourceVersion()
	x2 := x.Clone()
	x2.SetK8sResourceVersion(rv)

	o := k8s.UpdateOptions{
		ForceDryRun: a.o.DryRun,
	}

	r, apiWarnings, err := a.k.UpdateObject(x, o)
	a.handleApiWarnings(ref, apiWarnings)
	if err != nil {
		a.retryApplyForceReplace(x, hook, remoteObject, err)
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

	x = a.k.FixObjectForPatch(x)
	remoteObject := a.ru.GetRemoteObject(ref)
	var remoteNamespace *uo.UnstructuredObject
	if ref.Namespace != "" {
		remoteNamespace = a.ru.GetRemoteNamespace(ref.Namespace)
	}

	usesDummyName := false
	if a.o.DryRun && replaced && remoteObject != nil {
		// The object got deleted before, which was however only simulated when in dry-run mode. This means, that
		// trying to patch it will either fail or give different results then when actually re-creating it. To simulate
		// re-creation, we use a temporary name for the dry-run patch and then undo the rename after getting the patch
		// result
		usesDummyName = true
		x = x.Clone()
		x.SetK8sName(fmt.Sprintf("%s-%s", ref.Name, utils.RandomString(8)))
	} else if a.o.DryRun && remoteNamespace == nil && ref.Namespace != "" {
		if _, ok := a.allNamespaces.Load(ref.Namespace); ok {
			// The namespace does not really exist, but would have been created if dryRun would be false.
			// So let's pretend we deploy it to the default namespace with a dummy name
			usesDummyName = true
			x = x.Clone()
			x.SetK8sName(fmt.Sprintf("%s-%s", ref.Name, utils.RandomString(8)))
			x.SetK8sNamespace("default")
		}
	}

	options := k8s.PatchOptions{
		ForceDryRun: a.o.DryRun,
	}
	r, apiWarnings, err := a.k.PatchObject(x, options)
	if r != nil && usesDummyName {
		tmpName := r.GetK8sName()
		_ = r.ReplaceKeys(tmpName, ref.Name)
		_ = r.ReplaceValues(tmpName, ref.Name)
		r.SetK8sNamespace(ref.Namespace)
	}
	if r != nil && ref.GVK.GroupKind().String() == "Namespace" {
		a.allNamespaces.Store(ref.Name, r)
	}
	a.handleApiWarnings(ref, apiWarnings)
	if err == nil {
		a.handleResult(r, hook)
	} else if meta.IsNoMatchError(err) {
		a.HandleError(ref, err)
	} else if errors.IsConflict(err) {
		a.retryApplyWithConflicts(x, hook, remoteObject, err)
	} else {
		a.retryApplyWithReplace(x, hook, remoteObject, err)
	}
}

func (a *ApplyUtil) WaitReadiness(ref k8s2.ObjectRef, timeout time.Duration) bool {
	if a.o.DryRun {
		return true
	}

	if timeout == 0 {
		timeout = a.o.ReadinessTimeout
	}
	timeoutTimer := time.NewTimer(timeout)

	status.Trace(a.ctx, "Waiting for %s to get ready", ref.String())

	lastLogTime := time.Now()
	didLog := false
	startTime := time.Now()
	for true {
		elapsed := int(time.Now().Sub(startTime).Seconds())

		o, apiWarnings, err := a.k.GetSingleObject(ref)
		a.handleApiWarnings(ref, apiWarnings)
		if err != nil {
			if errors.IsNotFound(err) {
				if didLog {
					status.Warning(a.ctx, "Cancelled waiting for %s as it disappeared while waiting for it (%ds elapsed)", ref.String(), elapsed)
				}
				a.HandleError(ref, fmt.Errorf("%s disappeared while waiting for it to become ready", ref.String()))
				return false
			}
			a.HandleError(ref, err)
			return false
		}
		v := validation.ValidateObject(a.k, o, false, false)
		if v.Ready {
			if didLog {
				a.sctx.InfoFallback("Finished waiting for %s (%ds elapsed)", ref.String(), elapsed)
			}
			return true
		}
		if len(v.Errors) != 0 {
			if didLog {
				status.Warning(a.ctx, "Cancelled waiting for %s due to errors (%ds elapsed)", ref.String(), elapsed)
			}
			for _, e := range v.Errors {
				a.HandleError(ref, fmt.Errorf(e.Error))
			}
			return false
		}

		a.sctx.Update(fmt.Sprintf("Waiting for %s to get ready...", ref.String()))

		if !didLog {
			a.sctx.InfoFallback("Waiting for %s to get ready... (%ds elapsed)", ref.String(), elapsed)
			didLog = true
			lastLogTime = time.Now()
		} else if didLog && time.Now().Sub(lastLogTime) >= 10*time.Second {
			a.sctx.InfoFallback("Still waiting for %s to get ready... (%ds elapsed)", ref.String(), elapsed)
			lastLogTime = time.Now()
		}

		select {
		case <-time.After(500 * time.Millisecond):
			continue
		case <-timeoutTimer.C:
			err := fmt.Errorf("timed out while waiting for readiness of %s", ref.String())
			status.Warning(a.ctx, "%s (%ds elapsed)", err.Error(), elapsed)
			if status.IsTraceEnabled(a.ctx) {
				y, err := yaml.WriteYamlString(o)
				if err == nil {
					status.Trace(a.ctx, "yaml:\n"+y)
				}
			}
			a.HandleError(ref, err)
			return false
		case <-a.ctx.Done():
			err := fmt.Errorf("failed waiting for readiness of %s: %w", ref.String(), err)
			status.Warning(a.ctx, "%s (%ds elapsed)", err.Error(), elapsed)
			a.HandleError(ref, err)
			return false
		}
	}
	return false
}

func (a *ApplyUtil) applyDeploymentItem(d *deployment.DeploymentItem) {
	toDelete := map[k8s2.ObjectRef]bool{}
	for _, x := range d.Config.DeleteObjects {
		for _, gvk := range a.k.Resources.GetFilteredGVKs(k8s.BuildGVKFilter(x.Group, nil, x.Kind)) {
			ref := k8s2.ObjectRef{
				GVK:       gvk,
				Name:      x.Name,
				Namespace: x.Namespace,
			}
			toDelete[ref] = true
		}
	}
	for _, x := range d.Objects {
		if utils.ParseBoolOrFalse(x.GetK8sAnnotation("kluctl.io/delete")) {
			toDelete[x.GetK8sRef()] = true
		}
	}

	h := HooksUtil{a: a}

	initialDeploy := true
	for _, o := range d.Objects {
		if a.ru.GetRemoteObject(o.GetK8sRef()) != nil {
			initialDeploy = false
		}
	}

	var applyObjects []*uo.UnstructuredObject
	for _, o := range d.Objects {
		if h.GetHook(o) != nil {
			continue
		}
		if _, ok := toDelete[o.GetK8sRef()]; ok {
			continue
		}
		applyObjects = append(applyObjects, o)
	}

	var preHooks []*hook
	var postHooks []*hook
	if initialDeploy {
		preHooks = h.DetermineHooks(d, []string{"pre-deploy-initial", "pre-deploy"})
		postHooks = h.DetermineHooks(d, []string{"post-deploy-initial", "post-deploy"})
	} else {
		preHooks = h.DetermineHooks(d, []string{"pre-deploy-upgrade", "pre-deploy"})
		postHooks = h.DetermineHooks(d, []string{"post-deploy-upgrade", "post-deploy"})
	}

	// +1 to ensure that we don't prematurely complete the bar (which would happen as we don't count for waiting)
	total := len(applyObjects) + len(preHooks) + len(postHooks) + 1
	a.sctx.SetTotal(total)

	if len(toDelete) != 0 {
		a.sctx.InfoFallback("Deleting %d objects", len(toDelete))
		i := 0
		for ref := range toDelete {
			a.sctx.Update(fmt.Sprintf("Deleting object %s (%d of %d)", ref.String(), i+1, len(toDelete)))
			a.DeleteObject(ref, false)
			a.sctx.Increment()
			i++
		}
	}

	h.RunHooks(preHooks)

	if len(applyObjects) != 0 {
		a.sctx.InfoFallback("Applying %d objects", len(applyObjects))
	}
	startTime := time.Now()
	didLog := false
	for i, o := range applyObjects {
		if a.abortSignal.Load().(bool) {
			break
		}

		ref := o.GetK8sRef()
		a.sctx.Update(fmt.Sprintf("Applying object %s (%d of %d)", ref.String(), i+1, len(applyObjects)))
		a.ApplyObject(o, false, false)
		a.sctx.Increment()
		if time.Now().Sub(startTime) >= 10*time.Second || (didLog && i == len(applyObjects)-1) {
			a.sctx.InfoFallback("...applied %d of %d objects", i+1, len(applyObjects))
			startTime = time.Now()
			didLog = true
		}

		waitReadiness := d.Config.WaitReadiness || d.WaitReadiness || utils.ParseBoolOrFalse(o.GetK8sAnnotation("kluctl.io/wait-readiness"))
		if !a.o.NoWait && waitReadiness {
			a.WaitReadiness(o.GetK8sRef(), 0)
		}
	}
	if a.abortSignal.Load().(bool) {
		return
	}

	h.RunHooks(postHooks)

	finalStatus := ""
	if len(a.appliedObjects) != 0 {
		finalStatus += fmt.Sprintf(" Applied %d objects.", len(a.appliedObjects))
	}
	if len(a.appliedHookObjects) != 0 {
		finalStatus += fmt.Sprintf(" Applied %d hooks.", len(a.appliedHookObjects))
	}
	if len(a.deletedObjects) != 0 {
		finalStatus += fmt.Sprintf(" Deleted %d objects.", len(a.deletedObjects))
	}
	if len(a.deletedHookObjects) != 0 {
		finalStatus += fmt.Sprintf(" Deleted %d hooks.", len(a.deletedHookObjects))
	}
	if a.errorCount != 0 {
		finalStatus += fmt.Sprintf(" Encountered %d errors.", a.errorCount)
	}
	if a.warningCount != 0 {
		finalStatus += fmt.Sprintf(" Encountered %d warnings.", a.warningCount)
	}
	if finalStatus == "" {
		finalStatus = "Nothing to apply."
	}

	a.sctx.Update(strings.TrimSpace(finalStatus))

	if a.errorCount == 0 {
		a.sctx.Success()
	} else {
		a.sctx.Failed()
	}
}

func (a *ApplyDeploymentsUtil) buildProgressName(d *deployment.DeploymentItem) *string {
	if d.RelToProjectItemDir != "" {
		return &d.RelToProjectItemDir
	}
	if len(d.Config.DeleteObjects) != 0 {
		s := "<delete>"
		return &s
	}
	return nil
}

func (a *ApplyDeploymentsUtil) ApplyDeployments() {
	s := status.Start(a.ctx, "Running server-side apply for all objects")
	defer s.Failed()

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(8)

	maxNameLen := 0
	for _, d := range a.deployments {
		name := a.buildProgressName(d)
		if name != nil {
			if len(*name) > maxNameLen {
				maxNameLen = len(*name)
			}
		}
	}

	for _, d_ := range a.deployments {
		d := d_
		if a.abortSignal.Load().(bool) {
			break
		}

		_ = sem.Acquire(context.Background(), 1)

		progressName := a.buildProgressName(d)
		var sctx *status.StatusContext
		if progressName != nil {
			sctx = status.StartWithOptions(a.ctx,
				status.WithTotal(-1),
				status.WithPrefix(*progressName),
				status.WithStatus("Initializing"),
			)
		}
		a2 := a.NewApplyUtil(a.ctx, sctx)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer sem.Release(1)

			a2.applyDeploymentItem(d)

			// if success was not signalled, get into failed status
			sctx.Failed()
		}()

		barrier := d.Config.Barrier || d.Barrier
		if barrier {
			sctx := status.StartWithOptions(a.ctx, status.WithStatus("Waiting on barrier..."), status.WithTotal(1))
			wg.Wait()
			sctx.UpdateAndInfoFallback(fmt.Sprintf("Finished waiting"))
			sctx.Success()
		}
	}
	wg.Wait()
	s.Success()
}

func (a *ApplyUtil) ReplaceObject(ref k8s2.ObjectRef, firstVersion *uo.UnstructuredObject, callback func(o *uo.UnstructuredObject) (*uo.UnstructuredObject, error)) {
	firstCall := true
	for true {
		if a.ctx.Err() != nil {
			a.HandleError(ref, fmt.Errorf("failed replacing %s: %w", ref.String(), a.ctx.Err()))
			return
		}

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
				status.Trace(a.ctx, "Conflict while patching %s. Retrying...", ref.String())
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

func (ad *ApplyDeploymentsUtil) collectObjects(f func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject) []*types.RefAndObject {
	ad.resultsMutex.Lock()
	defer ad.resultsMutex.Unlock()

	var ret []*types.RefAndObject
	for _, a := range ad.results {
		for _, o := range f(a) {
			ret = append(ret, &types.RefAndObject{
				Ref:    o.GetK8sRef(),
				Object: o,
			})
		}
	}
	return ret
}

func (ad *ApplyDeploymentsUtil) GetNewObjects() []*types.RefAndObject {
	return ad.collectObjects(func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject {
		return au.newObjects
	})
}

func (ad *ApplyDeploymentsUtil) GetAppliedObjects() []*types.RefAndObject {
	return ad.collectObjects(func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject {
		return au.appliedObjects
	})
}

func (ad *ApplyDeploymentsUtil) GetAppliedObjectsMap() map[k8s2.ObjectRef]*uo.UnstructuredObject {
	ret := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, ro := range ad.GetAppliedObjects() {
		ret[ro.Ref] = ro.Object
	}
	return ret
}

func (ad *ApplyDeploymentsUtil) GetAppliedHookObjects() []*types.RefAndObject {
	return ad.collectObjects(func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject {
		return au.appliedHookObjects
	})
}

func (ad *ApplyDeploymentsUtil) GetDeletedObjects() []k8s2.ObjectRef {
	ad.resultsMutex.Lock()
	defer ad.resultsMutex.Unlock()

	var ret []k8s2.ObjectRef
	m := make(map[k8s2.ObjectRef]bool)
	for _, a := range ad.results {
		for ref := range a.deletedObjects {
			if _, ok := m[ref]; !ok {
				ret = append(ret, ref)
				m[ref] = true
			}
		}
	}
	return ret
}
