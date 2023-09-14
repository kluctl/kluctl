package controllers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	internal_metrics "github.com/kluctl/kluctl/v2/pkg/controllers/metrics"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	kuberecorder "k8s.io/client-go/tools/record"
	"path"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
	"time"
)

type KluctlDeploymentReconciler struct {
	client.Client
	RestConfig            *rest.Config
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	MetricsRecorder       *metrics.Recorder
	ControllerName        string
	DefaultServiceAccount string
	DryRun                bool

	SshPool *ssh_pool.SshPool

	ResultStore results.ResultStore

	mutex               sync.Mutex
	resourceVersionsMap map[client.ObjectKey]map[k8s.ObjectRef]string
}

// KluctlDeploymentReconcilerOpts contains options for the BaseReconciler.
type KluctlDeploymentReconcilerOpts struct {
	Concurrency int
}

// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments/finalizers,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;secrets;serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *KluctlDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reconcileStart := time.Now()

	// we reuse the reconcileID as command result id
	reconcileID := string(controller.ReconcileIDFromContext(ctx))

	log := ctrl.LoggerFrom(ctx)

	ctx = status.NewContext(ctx, status.NewSimpleStatusHandler(func(level status.Level, message string) {
		log.Info(message)
	}, false))

	obj := &kluctlv1.KluctlDeployment{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, r.calcTimeout(obj))
	defer cancel()

	retryInterval := obj.Spec.GetRetryInterval()
	interval := obj.Spec.Interval.Duration

	defer func() {
		r.recordSuspension(ctx, obj)
		r.recordReadiness(ctx, obj)
		r.recordDuration(ctx, obj, reconcileStart)
	}()

	// Add our finalizer if it does not exist
	if !controllerutil.ContainsFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer) {
		patch := client.MergeFrom(obj.DeepCopy())
		controllerutil.AddFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer)
		if err := r.Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
			log.Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Examine if the object is under deletion
	if !obj.GetDeletionTimestamp().IsZero() {
		return r.finalize(ctx, obj)
	}

	// Return early if the KluctlDeployment is suspended.
	if obj.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	patchObj := obj.DeepCopy()
	patch := client.MergeFrom(patchObj)

	// reconcile kluctlDeployment by applying the latest revision
	ctrlResult, reconcileErr := r.doReconcile(ctx, obj, reconcileID)

	// make sure conditions were not touched in the obj. changing conditions is only allowed via patchCondition,
	// which directly patches the object in-cluster and does not touch the local obj
	if !reflect.DeepEqual(patchObj.GetConditions(), obj.GetConditions()) {
		panic("conditions were modified in doReconcile")
	}

	// now patch all the other changes to the status
	if err := r.Status().Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	if ctrlResult == nil {
		if reconcileErr != nil {
			ctrlResult = &ctrl.Result{RequeueAfter: retryInterval}
		} else {
			ctrlResult = &ctrl.Result{RequeueAfter: interval}
		}
	}

	// broadcast the reconciliation failure and requeue at the specified retry interval
	if reconcileErr != nil {
		log.Info(fmt.Sprintf("Reconciliation failed after %s, next try in %s: %s",
			time.Since(reconcileStart).String(),
			ctrlResult.RequeueAfter.String(), reconcileErr.Error(),
		))
		r.event(ctx, obj, true,
			reconcileErr.Error(), nil)
		return *ctrlResult, nil
	}

	// broadcast the reconciliation result and requeue at the specified interval
	msg := fmt.Sprintf("Reconciliation finished in %s, next run in %s",
		time.Since(reconcileStart).String(),
		ctrlResult.RequeueAfter.String())
	log.Info(msg)
	r.event(ctx, obj, true,
		msg, map[string]string{kluctlv1.GroupVersion.Group + "/commit_status": "update"})
	return *ctrlResult, nil
}

func (r *KluctlDeploymentReconciler) doReconcile(
	ctx context.Context,
	obj *kluctlv1.KluctlDeployment,
	reconcileId string) (*ctrl.Result, error) {

	log := ctrl.LoggerFrom(ctx)
	key := client.ObjectKeyFromObject(obj)

	r.exportDeploymentObjectToProm(obj)

	// keep old status until we're doing real work (deploying, validating, ...)
	oldReadyCondition := apimeta.FindStatusCondition(obj.GetConditions(), meta.ReadyCondition)

	doFail := func(reason string, err error) (*ctrl.Result, error) {
		internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(0.0)
		patchErr := r.patchCondition(ctx, key, func(c *[]metav1.Condition) error {
			setReadinessCondition(c, metav1.ConditionFalse, reason, err.Error(), obj.Generation)
			apimeta.RemoveStatusCondition(c, meta.ReconcilingCondition)
			return nil
		})
		if patchErr != nil {
			err = multierror.Append(err, patchErr)
		}
		return nil, err
	}

	doFailPrepare := func(err error) (*ctrl.Result, error) {
		obj.Status.LastPrepareError = err.Error()
		return doFail(kluctlv1.PrepareFailedReason, err)
	}

	doProgressingCondition := func(message string, keepOldReadyStatus bool) error {
		return r.patchCondition(ctx, key, func(c *[]metav1.Condition) error {
			if keepOldReadyStatus && oldReadyCondition != nil {
				setReadinessCondition(c, oldReadyCondition.Status, oldReadyCondition.Reason, oldReadyCondition.Message, obj.Generation)
			} else {
				setReadinessCondition(c, metav1.ConditionUnknown, meta.ProgressingReason, "Reconciliation in progress", obj.Generation)
			}
			setReconcilingCondition(c, metav1.ConditionTrue, meta.ProgressingReason, message, obj.Generation)
			return nil
		})
	}

	doReadyCondition := func(status metav1.ConditionStatus, reason, message string) error {
		return r.patchCondition(ctx, key, func(c *[]metav1.Condition) error {
			setReadinessCondition(c, status, reason, message, obj.Generation)
			apimeta.RemoveStatusCondition(c, meta.ReconcilingCondition)
			return nil
		})
	}

	// record the value of the reconciliation request, if any
	if v, ok := obj.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		obj.Status.LastHandledReconcileAt = v
	}

	patchErr := doProgressingCondition("Initializing", true)
	if patchErr != nil {
		return nil, patchErr
	}

	oldGeneration := obj.Status.ObservedGeneration
	oldDeployRequest := obj.Status.LastHandledDeployAt
	oldPruneRequest := obj.Status.LastHandledPruneAt
	oldValidateRequest := obj.Status.LastHandledValidateAt
	curDeployRequest, _ := obj.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]
	curPruneRequest, _ := obj.GetAnnotations()[kluctlv1.KluctlRequestPruneAnnotation]
	curValidateRequest, _ := obj.GetAnnotations()[kluctlv1.KluctlRequestValidateAnnotation]
	obj.Status.ObservedGeneration = obj.GetGeneration()
	obj.Status.LastHandledDeployAt = curDeployRequest
	obj.Status.LastHandledPruneAt = curPruneRequest
	obj.Status.LastHandledValidateAt = curValidateRequest

	patchErr = doProgressingCondition("Preparing project", true)
	if patchErr != nil {
		return nil, patchErr
	}

	obj.Status.LastPrepareError = ""

	pp, err := prepareProject(ctx, r, obj, true)
	if err != nil {
		return doFailPrepare(err)
	}
	defer pp.cleanup()

	patchErr = doProgressingCondition("Loading project and target", true)
	if patchErr != nil {
		return nil, patchErr
	}

	obj.Status.ObservedCommit = pp.co.CheckedOutCommit

	newProjectKey := result.ProjectKey{
		GitRepoKey: obj.Spec.Source.URL.RepoKey(),
		SubDir:     path.Clean(obj.Spec.Source.Path),
	}
	if newProjectKey.SubDir == "." {
		newProjectKey.SubDir = ""
	}

	// we patch the projectKey immediately so that the webui knows it asap
	if obj.Status.ProjectKey == nil || *obj.Status.ProjectKey != newProjectKey {
		patchErr = r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			status.ProjectKey = &newProjectKey
			return nil
		})
	}
	obj.Status.ProjectKey = &newProjectKey

	j2, err := kluctl_jinja2.NewKluctlJinja2(true)
	if err != nil {
		return doFailPrepare(err)
	}
	defer j2.Close()

	pt := pp.newTarget()

	lp, err := pp.loadKluctlProject(ctx, pt, j2)
	if err != nil {
		return doFailPrepare(err)
	}

	targetContext, err := pt.loadTarget(ctx, lp)
	if targetContext != nil {
		clusterId, err := targetContext.SharedContext.K.GetClusterId()
		if err != nil {
			return doFailPrepare(err)
		}
		newTargetKey := result.TargetKey{
			TargetName:    targetContext.Target.Name,
			Discriminator: targetContext.Target.Discriminator,
			ClusterId:     clusterId,
		}
		// we patch the targetKey immediately so that the webui knows it asap
		if obj.Status.TargetKey == nil || *obj.Status.TargetKey != newTargetKey {
			patchErr = r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
				status.TargetKey = &newTargetKey
				return nil
			})
		}
		obj.Status.TargetKey = &newTargetKey
	}
	if err != nil {
		return doFailPrepare(err)
	}

	objectsHash, err := targetContext.DeploymentCollection.CalcObjectsHash()
	if err != nil {
		return doFailPrepare(err)
	}

	needDeploy := false
	needDeployByRequest := false
	needPrune := false
	needValidate := false
	needDriftDetection := true

	if obj.Status.LastDeployResult == nil || obj.Status.LastObjectsHash != objectsHash {
		// either never deployed or source code changed
		needDeploy = true
	} else if obj.Spec.Manual && !utils.StrPtrEquals(obj.Status.LastManualObjectsHash, obj.Spec.ManualObjectsHash) {
		// approval hash was changed
		needDeploy = true
	} else if curDeployRequest != "" && oldDeployRequest != curDeployRequest {
		// explicitly requested a deploy
		needDeploy = true
		needDeployByRequest = true
	} else if oldGeneration != obj.GetGeneration() {
		// spec has changed
		needDeploy = true
	} else {
		// was deployed before, let's check if we need to do periodic deployments
		nextDeployTime := r.nextDeployTime(obj)
		if nextDeployTime != nil {
			needDeploy = nextDeployTime.Before(time.Now())
		}
	}

	if needDeploy && obj.Spec.Manual {
		log.Info("checking manual object hash")
		if obj.Spec.ManualObjectsHash == nil || *obj.Spec.ManualObjectsHash != objectsHash {
			log.Info("deployment is not approved", "manualObjectsHash", obj.Spec.ManualObjectsHash, "objectsHash", objectsHash)
			needDeploy = false
		} else {
			log.Info("deployment is approved", "objectsHash", objectsHash)
		}
	}

	if curPruneRequest != "" && oldPruneRequest != curPruneRequest {
		// explicitly requested a prune
		needPrune = true
	}

	if obj.Spec.Validate {
		if obj.Status.LastValidateResult == nil || needDeploy {
			// either never validated before or a deployment requested (which required re-validation)
			needValidate = true
		} else if curValidateRequest != "" && oldValidateRequest != curValidateRequest {
			// explicitly requested a validate
			needValidate = true
		} else {
			nextValidateTime := r.nextValidateTime(obj)
			if nextValidateTime != nil {
				needValidate = nextValidateTime.Before(time.Now())
			}
		}
	} else {
		obj.Status.LastValidateResult = nil
		obj.Status.LastValidateError = ""
	}

	if !needDeploy && obj.Status.LastObjectsHash != objectsHash {
		// force full drift detection as we can't know which objects changed in-between
		r.updateResourceVersions(key, nil, nil)
	}

	obj.Status.LastObjectsHash = objectsHash
	obj.Status.LastManualObjectsHash = obj.Spec.ManualObjectsHash

	var deployResult *result.CommandResult
	if needDeploy {
		patchErr = doProgressingCondition(fmt.Sprintf("Performing kluctl %s", obj.Spec.DeployMode), false)
		if patchErr != nil {
			return nil, patchErr
		}
		deployResult, err = pt.kluctlDeployOrPokeImages(ctx, obj.Spec.DeployMode, targetContext, reconcileId, objectsHash, needDeployByRequest)
		err = obj.Status.SetLastDeployResult(deployResult.BuildSummary(), err)
		if err != nil {
			log.Error(err, "Failed to write deploy result")
		}

		if obj.Spec.DryRun {
			// force full drift detection (otherwise we'd see the dry-run applied changes as non-drifted)
			r.updateResourceVersions(key, nil, nil)
		} else {
			r.updateResourceVersions(key, deployResult.Objects, nil)
		}
	}

	if needPrune {
		patchErr = doProgressingCondition("Performing kluctl prune", false)
		if patchErr != nil {
			return nil, patchErr
		}
		pruneResult, err := pt.kluctlPrune(ctx, targetContext, reconcileId, objectsHash)
		err = obj.Status.SetLastDeployResult(pruneResult.BuildSummary(), err)
		if err != nil {
			log.Error(err, "Failed to write prune result")
		}

		// force full drift detection
		r.updateResourceVersions(key, nil, nil)
	}

	if needValidate {
		patchErr = doProgressingCondition("Performing kluctl validate", false)
		if patchErr != nil {
			return nil, patchErr
		}
		validateResult, err := pt.kluctlValidate(ctx, targetContext, deployResult, reconcileId, objectsHash)
		err = obj.Status.SetLastValidateResult(validateResult, err)
		if err != nil {
			log.Error(err, "Failed to write validate result")
		}
	}

	if needDriftDetection {
		patchErr = doProgressingCondition("Performing drift detection", false)
		if patchErr != nil {
			return nil, patchErr
		}

		resourceVersions := r.getResourceVersions(key)

		diffResult, cmdErr := pt.kluctlDiff(ctx, targetContext, reconcileId, objectsHash, resourceVersions)
		driftDetectionResult := diffResult.BuildDriftDetectionResult()
		err = obj.Status.SetLastDriftDetectionResult(driftDetectionResult, cmdErr)
		if err != nil {
			log.Error(err, "Failed to write drift detection result")
		}

		r.updateResourceVersions(key, diffResult.Objects, driftDetectionResult.Objects)
	}

	var ctrlResult ctrl.Result
	ctrlResult.RequeueAfter = r.nextReconcileTime(obj).Sub(time.Now())
	if ctrlResult.RequeueAfter < 0 {
		ctrlResult.RequeueAfter = 0
		ctrlResult.Requeue = true
	}

	finalStatus, reason := r.buildFinalStatus(ctx, obj)
	if reason != kluctlv1.ReconciliationSucceededReason {
		internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(0.0)
		err = fmt.Errorf(finalStatus)
		patchErr = doReadyCondition(metav1.ConditionFalse, reason, finalStatus)
		if patchErr != nil {
			err = multierror.Append(err, patchErr)
			return nil, err
		}
		return &ctrlResult, err
	}
	internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(1.0)
	patchErr = doReadyCondition(metav1.ConditionTrue, reason, finalStatus)
	if patchErr != nil {
		return nil, patchErr
	}
	return &ctrlResult, nil
}

func (r *KluctlDeploymentReconciler) buildResourceVersionsMap(objects []result.ResultObject) map[k8s.ObjectRef]string {
	m := map[k8s.ObjectRef]string{}
	for _, o := range objects {
		if o.Applied == nil && o.Remote == nil {
			continue
		}
		ro := o.Applied
		if ro == nil {
			ro = o.Remote
		}
		s := ro.GetK8sResourceVersion()
		if s == "" {
			continue
		}
		m[o.Ref] = s
	}
	return m
}

func (r *KluctlDeploymentReconciler) getResourceVersions(key client.ObjectKey) map[k8s.ObjectRef]string {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	m, _ := r.resourceVersionsMap[key]
	return m
}

func (r *KluctlDeploymentReconciler) updateResourceVersions(key client.ObjectKey, diffObjects []result.ResultObject, driftedObjects []result.DriftedObject) {
	newMap := r.buildResourceVersionsMap(diffObjects)

	// ignore versions from already drifted objects so that we re-diff them the next time
	for _, o := range driftedObjects {
		delete(newMap, o.Ref)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.resourceVersionsMap[key] = newMap
}

func (r *KluctlDeploymentReconciler) buildFinalStatus(ctx context.Context, obj *kluctlv1.KluctlDeployment) (finalStatus string, reason string) {
	log := ctrl.LoggerFrom(ctx)

	if obj.Status.LastDeployError != "" {
		finalStatus = obj.Status.LastDeployError
		reason = kluctlv1.DeployFailedReason
		return
	} else if obj.Status.LastValidateError != "" {
		finalStatus = obj.Status.LastValidateError
		reason = kluctlv1.ValidateFailedReason
		return
	}

	var lastDeployResult *result.CommandResultSummary
	var lastValidateResult *result.ValidateResult
	if obj.Status.LastDeployResult != nil {
		err := yaml.ReadYamlBytes(obj.Status.LastDeployResult.Raw, &lastDeployResult)
		if err != nil {
			log.Info(fmt.Sprintf("Failed to parse last deploy result: %s", err.Error()))
		}
	}
	if obj.Status.LastValidateResult != nil {
		err := yaml.ReadYamlBytes(obj.Status.LastValidateResult.Raw, &lastValidateResult)
		if err != nil {
			log.Info(fmt.Sprintf("Failed to parse last validate result: %s", err.Error()))
		}
	}

	deployOk := lastDeployResult != nil && len(lastDeployResult.Errors) == 0
	validateOk := lastValidateResult != nil && len(lastValidateResult.Errors) == 0 && lastValidateResult.Ready

	if !obj.Spec.Validate {
		validateOk = true
	}

	if obj.Status.LastDeployResult != nil {
		finalStatus += "deploy: "
		if deployOk {
			finalStatus += "ok"
		} else {
			finalStatus += "failed"
		}
	}
	if obj.Spec.Validate && obj.Status.LastValidateResult != nil {
		if finalStatus != "" {
			finalStatus += ", "
		}
		finalStatus += "validate: "
		if validateOk {
			finalStatus += "ok"
		} else {
			finalStatus += "failed"
		}
	}

	if deployOk {
		if validateOk {
			reason = kluctlv1.ReconciliationSucceededReason
		} else {
			reason = kluctlv1.ValidateFailedReason
			return
		}
	} else {
		reason = kluctlv1.DeployFailedReason
		return
	}
	return
}

func (r *KluctlDeploymentReconciler) calcTimeout(obj *kluctlv1.KluctlDeployment) time.Duration {
	var d time.Duration
	if obj.Spec.Timeout != nil {
		d = obj.Spec.Timeout.Duration
	} else if obj.Spec.DeployInterval != nil {
		d = obj.Spec.DeployInterval.Duration.Duration
	} else {
		d = obj.Spec.Interval.Duration
	}
	if d < time.Second*30 {
		d = time.Second * 30
	}
	return d
}

func (r *KluctlDeploymentReconciler) nextReconcileTime(obj *kluctlv1.KluctlDeployment) time.Time {
	t1 := time.Now().Add(obj.Spec.Interval.Duration)
	t2 := r.nextDeployTime(obj)
	t3 := r.nextValidateTime(obj)
	if t2 != nil && t2.Before(t1) {
		t1 = *t2
	}
	if obj.Spec.Validate && t3 != nil && t3.Before(t1) {
		t1 = *t3
	}
	return t1
}

func (r *KluctlDeploymentReconciler) nextDeployTime(obj *kluctlv1.KluctlDeployment) *time.Time {
	if obj.Status.LastDeployResult == nil {
		// was never deployed before. Return early.
		return nil
	}
	if obj.Spec.DeployInterval == nil {
		// periodic deployments disabled
		return nil
	}

	var lastDeployResult result.CommandResultSummary
	err := yaml.ReadYamlBytes(obj.Status.LastDeployResult.Raw, &lastDeployResult)
	if err != nil {
		return nil
	}

	t := lastDeployResult.Command.EndTime.Add(obj.Spec.DeployInterval.Duration.Duration)
	return &t
}

func (r *KluctlDeploymentReconciler) nextValidateTime(obj *kluctlv1.KluctlDeployment) *time.Time {
	if obj.Status.LastValidateResult == nil {
		// was never validated before. Return early.
		return nil
	}
	d := obj.Spec.Interval.Duration
	if obj.Spec.ValidateInterval != nil {
		d = obj.Spec.ValidateInterval.Duration.Duration
	}

	var lastValidateResult result.ValidateResult
	err := yaml.ReadYamlBytes(obj.Status.LastValidateResult.Raw, &lastValidateResult)
	if err != nil {
		return nil
	}

	t := lastValidateResult.EndTime.Add(d)
	return &t
}

func (r *KluctlDeploymentReconciler) finalize(ctx context.Context, obj *kluctlv1.KluctlDeployment) (ctrl.Result, error) {
	r.doFinalize(ctx, obj)

	r.mutex.Lock()
	delete(r.resourceVersionsMap, client.ObjectKeyFromObject(obj))
	r.mutex.Unlock()

	// Remove our finalizer from the list and update it
	patch := client.MergeFrom(obj.DeepCopy())
	controllerutil.RemoveFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer)
	if err := r.Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
		return ctrl.Result{}, err
	}

	// Stop reconciliation as the object is being deleted
	return ctrl.Result{}, nil
}

func (r *KluctlDeploymentReconciler) doFinalize(ctx context.Context, obj *kluctlv1.KluctlDeployment) {
	log := ctrl.LoggerFrom(ctx)

	if !obj.Spec.Delete || obj.Spec.Suspend {
		return
	}

	if obj.Status.ProjectKey == nil || obj.Status.TargetKey == nil {
		log.V(1).Info("No project/target key set, skipping deletion")
		return
	}

	log.V(1).Info("Deleting target")

	pp, err := prepareProject(ctx, r, obj, false)
	if err != nil {
		return
	}
	defer pp.cleanup()

	pt := pp.newTarget()

	commandResultId := uuid.NewString()

	_, _ = pt.kluctlDelete(ctx, obj.Status.TargetKey.Discriminator, commandResultId)
}

func (r *KluctlDeploymentReconciler) exportDeploymentObjectToProm(obj *kluctlv1.KluctlDeployment) {
	pruneEnabled := 0.0
	deleteEnabled := 0.0
	dryRunEnabled := 0.0
	deploymentInterval := 0.0

	if obj.Spec.Prune {
		pruneEnabled = 1.0
	}
	if obj.Spec.Delete {
		deleteEnabled = 1.0
	}
	if obj.Spec.DryRun {
		dryRunEnabled = 1.0
	}
	//If not set, it defaults to interval
	if obj.Spec.DeployInterval == nil {
		deploymentInterval = obj.Spec.Interval.Seconds()
	} else {
		deploymentInterval = obj.Spec.DeployInterval.Duration.Seconds()
	}

	//Export as Prometheus metric
	internal_metrics.NewKluctlPruneEnabled(obj.Namespace, obj.Name).Set(pruneEnabled)
	internal_metrics.NewKluctlDeleteEnabled(obj.Namespace, obj.Name).Set(deleteEnabled)
	internal_metrics.NewKluctlDryRunEnabled(obj.Namespace, obj.Name).Set(dryRunEnabled)
	internal_metrics.NewKluctlDeploymentInterval(obj.Namespace, obj.Name).Set(deploymentInterval)
	internal_metrics.NewKluctlSourceSpec(obj.Namespace, obj.Name, obj.Spec.Source.URL.String(), obj.Spec.Source.Path, obj.Spec.Source.Ref.String()).Set(0.0)
}
