package utils

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/diff"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
	"golang.org/x/sync/semaphore"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
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

	SkipResourceVersions map[k8s2.ObjectRef]string
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
	allCRDs       *sync.Map

	crdCache *k8s.CrdCache

	ru   *RemoteObjectUtils
	k    *k8s.K8sCluster
	o    *ApplyUtilOptions
	sctx *status.StatusContext
}

type ApplyDeploymentsUtil struct {
	ctx context.Context

	dew *DeploymentErrorsAndWarnings
	ru  *RemoteObjectUtils
	k   *k8s.K8sCluster
	o   *ApplyUtilOptions

	abortSignal atomic.Value

	// Used to track all created namespaces and CRDs
	// All ApplyUtil instances write to this in parallel and we ignore that order might be unstable
	// This is only used to simulate dryRun apply
	allNamespaces sync.Map
	allCRDs       sync.Map

	crdCache k8s.CrdCache

	resultsMutex sync.Mutex
	results      []*ApplyUtil
}

func NewApplyDeploymentsUtil(ctx context.Context, dew *DeploymentErrorsAndWarnings, ru *RemoteObjectUtils, k *k8s.K8sCluster, o *ApplyUtilOptions) *ApplyDeploymentsUtil {
	ret := &ApplyDeploymentsUtil{
		ctx: ctx,
		dew: dew,
		ru:  ru,
		k:   k,
		o:   o,
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
		allCRDs:            &ad.allCRDs,
		crdCache:           &ad.crdCache,
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

	if err == nil {
		a.mutex.Lock()
		defer a.mutex.Unlock()
		if hook {
			a.deletedHookObjects[ref] = true
		} else {
			a.deletedObjects[ref] = true
		}
		return true
	}
	if !errors.IsNotFound(err) {
		a.HandleError(ref, err)
		return false
	}
	if !a.o.DryRun {
		// just ignore 404 errors
		return false
	}

	// now simulate deletion of objects that got applied in the same run

	wasApplied := false
	if hook {
		if _, ok := a.appliedObjects[ref]; ok {
			wasApplied = true
		}
	} else {
		if _, ok := a.appliedHookObjects[ref]; ok {
			wasApplied = true
		}
	}
	if !wasApplied {
		// did not get applied, so just ignore the 404
		return false
	}

	// it got applied, so we need to pretend it actually got deleted

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
		status.Warningf(a.ctx, "skipped forced replace of %s", ref.String())
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
		r, apiWarnings, err := a.k.ApplyObject(x, o)
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

func (a *ApplyUtil) retryApplyWithConflicts(d *deployment.DeploymentItem, x *uo.UnstructuredObject, hook bool, remoteObject *uo.UnstructuredObject, applyError error) {
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

		cr := diff.ConflictResolver{
			Configs: d.Project.GetConflictResolutionConfigs(),
		}
		x3, lostOwnership, err := cr.ResolveConflicts(x, remoteObject, statusError.ErrStatus)
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
	r, apiWarnings, err := a.k.ApplyObject(x2, options)
	a.handleApiWarnings(ref, apiWarnings)
	if err == nil {
		a.handleResult(r, hook)
	} else {
		a.retryApplyForceReplace(x, hook, remoteObject, err)
	}
}

func (a *ApplyUtil) ApplyObject(d *deployment.DeploymentItem, x *uo.UnstructuredObject, replaced bool, hook bool) {
	ref := x.GetK8sRef()

	x = a.k.FixObjectForPatch(x)
	remoteObject := a.ru.GetRemoteObject(ref)

	if a.o.SkipResourceVersions != nil && remoteObject != nil {
		remoteResourceVersion := remoteObject.GetK8sResourceVersion()
		skipVersion, ok := a.o.SkipResourceVersions[ref]
		if ok && skipVersion == remoteResourceVersion {
			a.handleResult(remoteObject, hook)
			return
		}
	}

	var remoteNamespace *uo.UnstructuredObject
	if ref.Namespace != "" {
		var err error
		remoteNamespace, err = a.ru.GetRemoteNamespace(a.k, ref.Namespace)
		if err != nil {
			a.HandleError(ref, err)
			return
		}
	}

	usesDummyName := false
	if a.o.DryRun && replaced && remoteObject != nil {
		// The object got deleted before, which was however only simulated when in dry-run mode. This means, that
		// trying to patch it will either fail or give different results then when actually re-creating it. To simulate
		// re-creation, we use a temporary name for the dry-run patch and then undo the rename after getting the patch
		// result
		usesDummyName = true
		x = x.Clone()
		x.SetK8sName(utils.RandomizeSuffix(ref.Name, 8, 63))
	} else if a.o.DryRun && remoteNamespace == nil && ref.Namespace != "" {
		if _, ok := a.allNamespaces.Load(ref.Namespace); ok {
			// The namespace does not really exist, but would have been created if dryRun would be false.
			// So let's pretend we deploy it to the default namespace with a dummy name
			usesDummyName = true
			x = x.Clone()
			x.SetK8sName(utils.RandomizeSuffix(ref.Name, 8, 63))
			x.SetK8sNamespace("default")
		}
	}

	undoDummyName := func(x *uo.UnstructuredObject) {
		if !usesDummyName || x == nil {
			return
		}
		tmpName := x.GetK8sName()
		_ = x.ReplaceKeys(tmpName, ref.Name)
		_ = x.ReplaceValues(tmpName, ref.Name)
		x.SetK8sNamespace(ref.Namespace)
	}

	options := k8s.PatchOptions{
		ForceDryRun: a.o.DryRun,
	}
	r, apiWarnings, err := a.k.ApplyObject(x, options)

	retryWhenCRDExists := meta.IsNoMatchError(err)
	if errors.IsUnexpectedServerError(err) {
		reason := errors.ReasonForError(err)
		if reason == metav1.StatusReasonNotFound {
			// this happens when a CRD was present in the past, causing the on-disk discovery cache to persist the info
			// about it. When the CRD is then deleted, the cache gets out-of-date, causing the k8s client to perform
			// requests against non-existing resources. In that case, we do not get no-match errors but instead unexpected
			// 404 errors. We simply retry with invalidated caches in that case.
			retryWhenCRDExists = true
		}
	}

	undoDummyName(r)
	undoDummyName(x)

	if r == nil && retryWhenCRDExists {
		if a.o.DryRun {
			if _, ok := a.allCRDs.Load(x.GetK8sGVK()); ok {
				// simulate that the apply "succeeded"
				a.handleResult(x, hook)
				a.HandleWarning(ref, fmt.Errorf("the underyling custom resource definition for %s has not been applied yet as Kluctl is running in dry-run mode. It is not guaranteed that the object will actually sucessfully apply", x.GetK8sRef().String()))
				return
			}
		} else {
			c, tmpErr := a.k.ToClient()
			if tmpErr != nil {
				status.Errorf(a.ctx, "Unexpectadly failed to create k8s client: %s", tmpErr.Error())
				a.HandleError(ref, err)
				return
			}
			tmpErr = a.crdCache.UpdateForGroup(a.ctx, c, ref.Group)
			if tmpErr != nil {
				status.Tracef(a.ctx, "failed figure out if CRD appeared, so we can't retry with invalidated discovery: %s", tmpErr.Error())
			} else {
				if crd := a.crdCache.GetCRDByGK(ref.GroupKind()); crd != nil {
					status.Tracef(a.ctx, "resource unknown, and CRD %s is available now, retrying with invalidated caches", crd.Name)
					// retry with invalidated discovery
					a.k.ResetMapper()
					r, apiWarnings, err = a.k.ApplyObject(x, options)
				}
			}
		}
	}
	if r != nil && ref.GroupKind().String() == "Namespace" {
		a.allNamespaces.Store(ref.Name, r)
	}
	if r != nil && ref.GroupKind().String() == "CustomResourceDefinition.apiextensions.k8s.io" {
		a.handleObservedCRD(r)
	}
	a.handleApiWarnings(ref, apiWarnings)
	if err == nil {
		a.handleResult(r, hook)
	} else if meta.IsNoMatchError(err) {
		a.HandleError(ref, err)
	} else if errors.IsConflict(err) {
		a.retryApplyWithConflicts(d, x, hook, remoteObject, err)
	} else {
		a.retryApplyWithReplace(x, hook, remoteObject, err)
	}
}

func (a *ApplyUtil) handleObservedCRD(r *uo.UnstructuredObject) {
	status.Tracef(a.ctx, "observed CRD %s", r.GetK8sName())

	y, err := yaml.WriteYamlBytes(r)
	if err == nil {
		var crd *apiextensionsv1.CustomResourceDefinition
		err = yaml.ReadYamlBytes(y, &crd)
		if err == nil {
			for _, v := range crd.Spec.Versions {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Version: v.Name,
					Kind:    crd.Spec.Names.Kind,
				}
				a.allCRDs.Store(gvk, &crd)
			}
		}
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

	status.Tracef(a.ctx, "Waiting for %s to get ready", ref.String())

	lastLogTime := time.Now()
	didLog := false
	seen := false
	startTime := time.Now()
	for true {
		elapsed := int(time.Now().Sub(startTime).Seconds())

		o, apiWarnings, err := a.k.GetSingleObject(ref)
		a.handleApiWarnings(ref, apiWarnings)
		if err != nil {
			if !errors.IsNotFound(err) {
				a.HandleError(ref, err)
				return false
			}
		}

		if o == nil {
			if seen {
				if didLog {
					status.Warningf(a.ctx, "Cancelled waiting for %s as it disappeared while waiting for it (%ds elapsed)", ref.String(), elapsed)
				}
				a.HandleError(ref, fmt.Errorf("%s disappeared while waiting for it to become ready", ref.String()))
				return false
			}
			a.sctx.Update(fmt.Sprintf("Waiting for %s to appear...", ref.String()))
		} else {
			seen = true

			v := validation.ValidateObject(a.ctx, a.k, o, false, false)
			if v.Ready {
				if didLog {
					a.sctx.InfoFallbackf("Finished waiting for %s (%ds elapsed)", ref.String(), elapsed)
				}
				for _, e := range v.Errors {
					a.HandleError(ref, errors2.New(e.Message))
				}
				for _, e := range v.Warnings {
					a.HandleWarning(ref, errors2.New(e.Message))
				}
				return true
			}
			if len(v.Errors) != 0 {
				if didLog {
					status.Warningf(a.ctx, "Cancelled waiting for %s due to errors (%ds elapsed)", ref.String(), elapsed)
				}
				for _, e := range v.Errors {
					a.HandleError(ref, errors2.New(e.Message))
				}
				for _, e := range v.Warnings {
					a.HandleWarning(ref, errors2.New(e.Message))
				}
				return false
			}
			a.sctx.Update(fmt.Sprintf("Waiting for %s to get ready...", ref.String()))
		}

		reportStillWaitingTime := 10 * time.Second
		if testing.Testing() {
			reportStillWaitingTime = 3 * time.Second
		}

		if !didLog {
			a.sctx.InfoFallbackf("Waiting for %s to get ready... (%ds elapsed)", ref.String(), elapsed)
			didLog = true
			lastLogTime = time.Now()
		} else if didLog && time.Now().Sub(lastLogTime) >= reportStillWaitingTime {
			a.sctx.InfoFallbackf("Still waiting for %s to get ready... (%ds elapsed)", ref.String(), elapsed)
			lastLogTime = time.Now()
		}

		select {
		case <-time.After(500 * time.Millisecond):
			continue
		case <-timeoutTimer.C:
			err := fmt.Errorf("timed out while waiting for readiness of %s", ref.String())
			status.Warningf(a.ctx, "%s (%ds elapsed)", err.Error(), elapsed)
			if status.IsTraceEnabled(a.ctx) {
				y, err := yaml.WriteYamlString(o)
				if err == nil {
					status.Trace(a.ctx, "yaml:\n"+y)
				}
			}
			a.HandleError(ref, err)
			return false
		case <-a.ctx.Done():
			err := fmt.Errorf("context cancelled while waiting for readiness of %s", ref.String())
			status.Warningf(a.ctx, "%s (%ds elapsed)", err.Error(), elapsed)
			a.HandleError(ref, err)
			return false
		}
	}
	return false
}

func (a *ApplyUtil) convertObjectRef(x types2.ObjectRefItem, refs map[k8s2.ObjectRef]bool) {
	ars, err := a.k.GetFilteredPreferredAPIResources(k8s.BuildGVKFilter(x.Group, nil, x.Kind))
	if err != nil {
		a.HandleError(k8s2.ObjectRef{}, err)
		return
	}
	if len(ars) == 0 {
		nameAndNs := x.Name
		if x.Namespace != "" {
			nameAndNs = x.Namespace + "/" + x.Name
		}
		var gk schema.GroupKind
		if x.Group != nil {
			gk.Group = *x.Group
		}
		if x.Kind != nil {
			gk.Kind = *x.Kind
		}
		a.HandleError(k8s2.ObjectRef{}, fmt.Errorf("failed to wait for readiness of %s. resource with group/kind %s not found", nameAndNs, gk.String()))
		return
	}
	for _, ar := range ars {
		ref := k8s2.ObjectRef{
			Group:     ar.Group,
			Version:   ar.Version,
			Kind:      ar.Kind,
			Name:      x.Name,
			Namespace: x.Namespace,
		}
		refs[ref] = true
	}
}

func (a *ApplyUtil) applyDeploymentItem(d *deployment.DeploymentItem) {
	h := HooksUtil{a: a}

	toDelete := map[k8s2.ObjectRef]bool{}
	toWaitReadiness := map[k8s2.ObjectRef]bool{}
	for _, x := range d.Config.DeleteObjects {
		a.convertObjectRef(x.ObjectRefItem, toDelete)
	}
	for _, x := range d.Config.WaitReadinessObjects {
		a.convertObjectRef(x.ObjectRefItem, toWaitReadiness)
	}
	for _, x := range d.Objects {
		if x.GetK8sAnnotationBoolNoError("kluctl.io/delete", false) {
			toDelete[x.GetK8sRef()] = true
		}

		// hooks have their own waitReadiness logic, so we must skip them here. Otherwise we'd wait for an object
		// didn't even get deployed yet (e.g. post-deploy hooks).
		if h.GetHook(d, x) == nil {
			waitReadiness := d.Config.WaitReadiness || d.WaitReadiness || x.GetK8sAnnotationBoolNoError("kluctl.io/wait-readiness", false)
			if waitReadiness {
				toWaitReadiness[x.GetK8sRef()] = true
			}
		}
	}

	initialDeploy := true
	for _, o := range d.Objects {
		if a.ru.GetRemoteObject(o.GetK8sRef()) != nil {
			initialDeploy = false
		}
	}

	var applyObjects []*uo.UnstructuredObject
	for _, o := range d.Objects {
		if h.GetHook(d, o) != nil {
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
		a.sctx.InfoFallbackf("Deleting %d objects", len(toDelete))
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
		a.sctx.InfoFallbackf("Applying %d objects", len(applyObjects))
	}
	startTime := time.Now()
	didLog := false
	for i, o := range applyObjects {
		if a.abortSignal.Load().(bool) {
			break
		}

		ref := o.GetK8sRef()
		a.sctx.Updatef("Applying object %s (%d of %d)", ref.String(), i+1, len(applyObjects))
		a.ApplyObject(d, o, false, false)
		a.sctx.Increment()
		if time.Now().Sub(startTime) >= 10*time.Second || (didLog && i == len(applyObjects)-1) {
			a.sctx.InfoFallbackf("...applied %d of %d objects", i+1, len(applyObjects))
			startTime = time.Now()
			didLog = true
		}
	}
	// Wait for readiness if needed after we have applied all objects
	for ref, _ := range toWaitReadiness {
		if a.abortSignal.Load().(bool) {
			break
		}

		if !a.o.NoWait {
			a.WaitReadiness(ref, 0)
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
	if len(d.Config.WaitReadinessObjects) != 0 {
		s := "<wait>"
		return &s
	}
	return nil
}

func (a *ApplyDeploymentsUtil) ApplyDeployments(deployments []*deployment.DeploymentItem) {
	if a.k == nil {
		a.dew.AddError(k8s2.ObjectRef{}, fmt.Errorf("can not apply objects without a Kubernetes API client"))
		return
	}

	var wg sync.WaitGroup
	sem := semaphore.NewWeighted(8)

	maxNameLen := 0
	for _, d := range deployments {
		name := a.buildProgressName(d)
		if name != nil {
			if len(*name) > maxNameLen {
				maxNameLen = len(*name)
			}
		}
	}

	for _, d_ := range deployments {
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
			barrierMessage := "Waiting on barrier..."
			if d.Config.Message != nil {
				barrierMessage = fmt.Sprintf("Waiting on barrier: %s", *d.Config.Message)
			}
			sctx := status.StartWithOptions(a.ctx, status.WithStatus(barrierMessage), status.WithTotal(1))
			wg.Wait()
			sctx.UpdateAndInfoFallback(fmt.Sprintf("Finished waiting"))
			sctx.Success()
		}
	}
	wg.Wait()
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
				status.Tracef(a.ctx, "Conflict while patching %s. Retrying...", ref.String())
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

func (ad *ApplyDeploymentsUtil) collectObjects(f func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject) []*uo.UnstructuredObject {
	ad.resultsMutex.Lock()
	defer ad.resultsMutex.Unlock()

	var ret []*uo.UnstructuredObject
	for _, a := range ad.results {
		for _, o := range f(a) {
			ret = append(ret, o)
		}
	}
	return ret
}

func (ad *ApplyDeploymentsUtil) collectObjectRefs(f func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject) []k8s2.ObjectRef {
	ad.resultsMutex.Lock()
	defer ad.resultsMutex.Unlock()

	var ret []k8s2.ObjectRef
	for _, a := range ad.results {
		for _, o := range f(a) {
			ret = append(ret, o.GetK8sRef())
		}
	}
	return ret
}

func (ad *ApplyDeploymentsUtil) GetNewObjectRefs() []k8s2.ObjectRef {
	return ad.collectObjectRefs(func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject {
		return au.newObjects
	})
}

func (ad *ApplyDeploymentsUtil) GetAppliedObjects() []*uo.UnstructuredObject {
	return ad.collectObjects(func(au *ApplyUtil) map[k8s2.ObjectRef]*uo.UnstructuredObject {
		return au.appliedObjects
	})
}

func (ad *ApplyDeploymentsUtil) GetAppliedObjectsMap() map[k8s2.ObjectRef]*uo.UnstructuredObject {
	ret := make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, o := range ad.GetAppliedObjects() {
		ret[o.GetK8sRef()] = o
	}
	return ret
}

func (ad *ApplyDeploymentsUtil) GetAppliedHookObjects() []*uo.UnstructuredObject {
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
