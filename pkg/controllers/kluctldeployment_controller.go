package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	ssh_pool "github.com/kluctl/kluctl/v2/lib/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	internal_metrics "github.com/kluctl/kluctl/v2/pkg/controllers/metrics"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	kuberecorder "k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sync"
	"time"
)

type KluctlDeploymentReconciler struct {
	RestConfig            *rest.Config
	Client                client.Client
	ApiReader             client.Reader
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	MetricsRecorder       *metrics.Recorder
	ControllerName        string
	ControllerNamespace   string
	DefaultServiceAccount string
	UseSystemPython       bool
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
	// we must use ApiReader here to ensure that we don't get stale objects from the cache, which can easily happen
	// if the previous reconciliation loop modified the object itself
	if err := r.ApiReader.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

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
		if err := r.Client.Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
			log.Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Examine if the object is under deletion
	if !obj.GetDeletionTimestamp().IsZero() {
		return r.finalize(ctx, obj, reconcileID)
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
	if err := r.Client.Status().Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
		return ctrl.Result{}, err
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

	r.exportDeploymentObjectToProm(obj)

	patchErr := r.patchProgressingCondition(ctx, obj, "Initializing", true)
	if patchErr != nil {
		return nil, patchErr
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, r.calcTimeout(obj))
	defer cancel()

	err := r.patchProjectKey(ctx, obj)
	if err != nil {
		return nil, r.patchFailPrepare(ctx, obj, err)
	}

	if obj.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object, only allowing manual requests to be processed")
	}

	processed, err := r.reconcileManualRequests(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return nil, err
	}
	if processed {
		// immediately cause another reconcile loop to ensure we didn't miss regular reconciliation
		return &ctrl.Result{Requeue: true}, nil
	}

	_, err = r.reconcileFullRequest(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return nil, err
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

		patchErr = r.patchReadyCondition(ctx, obj, metav1.ConditionFalse, reason, finalStatus)
		if patchErr != nil {
			err = multierror.Append(err, patchErr)
			return nil, err
		}
		return &ctrlResult, err
	}

	internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(1.0)
	patchErr = r.patchReadyCondition(ctx, obj, metav1.ConditionTrue, reason, finalStatus)
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

func (r *KluctlDeploymentReconciler) buildErrorFromResult(errors_ []result.DeploymentError, warnings []result.DeploymentError, commandName string) error {
	if len(errors_) == 0 {
		return nil
	}
	return errors.New(r.buildBaseResultMessage(errors_, warnings, commandName))
}

func (r *KluctlDeploymentReconciler) buildBaseResultMessage(errors []result.DeploymentError, warnings []result.DeploymentError, commandName string) string {
	var msg string
	if len(errors) != 0 {
		if len(warnings) != 0 {
			msg = fmt.Sprintf("%s failed with %d errors and %d warnings.", commandName, len(errors), len(warnings))
		} else {
			msg = fmt.Sprintf("%s failed with %d errors.", commandName, len(errors))
		}
	} else if len(warnings) != 0 {
		msg = fmt.Sprintf("%s succeeded with %d warnings.", commandName, len(warnings))
	} else {
		msg = fmt.Sprintf("%s succeeded.", commandName)
	}
	return msg
}

func (r *KluctlDeploymentReconciler) buildResultMessage(summary *result.CommandResultSummary, commandName string) string {
	msg := r.buildBaseResultMessage(summary.Errors, summary.Warnings, commandName)
	if summary.NewObjects != 0 {
		msg += fmt.Sprintf(" %d new objects.", summary.NewObjects)
	}
	if summary.ChangedObjects != 0 {
		msg += fmt.Sprintf(" %d changed objects.", summary.ChangedObjects)
	}
	if summary.AppliedHookObjects != 0 {
		msg += fmt.Sprintf(" %d hooks run.", summary.AppliedHookObjects)
	}
	if summary.DeletedObjects != 0 {
		msg += fmt.Sprintf(" %d deleted objects.", summary.DeletedObjects)
	}
	if summary.OrphanObjects != 0 {
		msg += fmt.Sprintf(" %d orphan objects.", summary.OrphanObjects)
	}
	return msg
}

func (r *KluctlDeploymentReconciler) buildFinalStatus(ctx context.Context, obj *kluctlv1.KluctlDeployment) (string, string) {
	log := ctrl.LoggerFrom(ctx)

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

	if lastDeployResult == nil {
		return "deployment status unknown", kluctlv1.DeployFailedReason
	}

	msg := r.buildResultMessage(lastDeployResult, "deploy")
	if obj.Spec.Validate {
		if lastValidateResult == nil {
			return msg + " Validation status unknown.", kluctlv1.ValidateFailedReason
		}
		msg += " " + r.buildBaseResultMessage(lastValidateResult.Errors, lastDeployResult.Warnings, "validate")
	}

	if len(lastDeployResult.Errors) != 0 {
		return msg, kluctlv1.DeployFailedReason
	}
	if lastValidateResult != nil && len(lastValidateResult.Errors) != 0 {
		return msg, kluctlv1.ValidateFailedReason
	}

	return msg, kluctlv1.ReconciliationSucceededReason
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
	if obj.Spec.Suspend {
		return nil
	}
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
	if obj.Spec.Suspend {
		return nil
	}
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

func (r *KluctlDeploymentReconciler) finalize(ctx context.Context, obj *kluctlv1.KluctlDeployment, reconcileId string) (ctrl.Result, error) {
	r.doFinalize(ctx, obj, reconcileId)

	r.mutex.Lock()
	delete(r.resourceVersionsMap, client.ObjectKeyFromObject(obj))
	r.mutex.Unlock()

	// Remove our finalizer from the list and update it
	patch := client.MergeFrom(obj.DeepCopy())
	controllerutil.RemoveFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer)
	if err := r.Client.Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
		return ctrl.Result{}, err
	}

	// Stop reconciliation as the object is being deleted
	return ctrl.Result{}, nil
}

func (r *KluctlDeploymentReconciler) doFinalize(ctx context.Context, obj *kluctlv1.KluctlDeployment, reconcileId string) {
	log := ctrl.LoggerFrom(ctx)

	log.Info("Finalizing")

	if !obj.Spec.Delete || obj.Spec.Suspend {
		return
	}

	if obj.Status.ProjectKey == nil || obj.Status.TargetKey == nil {
		log.V(1).Info("No project/target key set, skipping deletion")
		return
	}

	log.Info(fmt.Sprintf("Deleting objects with discriminator '%s'", obj.Status.TargetKey.Discriminator))

	pp, err := prepareProject(ctx, r, obj, false)
	if err != nil {
		return
	}
	defer pp.cleanup()

	pt := pp.newTarget()

	cmdResult, err := pt.kluctlDelete(ctx, obj.Status.TargetKey.Discriminator)
	if err != nil {
		log.Error(err, "delete failed with an error")
	}
	if cmdResult != nil {
		err = pt.writeCommandResult(ctx, cmdResult, nil, "delete", reconcileId, "", false)
		if err != nil {
			log.Error(err, "write delete command result failed")
		}
	}
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
	if obj.Spec.Source.Git != nil {
		internal_metrics.NewKluctlSourceSpec(obj.Namespace, obj.Name,
			obj.Spec.Source.Git.URL, obj.Spec.Source.Git.Path, obj.Spec.Source.Git.Ref.String()).Set(0.0)
		internal_metrics.NewKluctlGitSourceSpec(obj.Namespace, obj.Name,
			obj.Spec.Source.Git.URL, obj.Spec.Source.Git.Path, obj.Spec.Source.Git.Ref.String()).Set(0.0)
	} else if obj.Spec.Source.Oci != nil {
		internal_metrics.NewKluctlSourceSpec(obj.Namespace, obj.Name,
			obj.Spec.Source.Oci.URL, obj.Spec.Source.Oci.Path, obj.Spec.Source.Oci.Ref.String()).Set(0.0)
		internal_metrics.NewKluctlOciSourceSpec(obj.Namespace, obj.Name,
			obj.Spec.Source.Oci.URL, obj.Spec.Source.Oci.Path, obj.Spec.Source.Oci.Ref.String()).Set(0.0)
	} else if obj.Spec.Source.URL != nil {
		internal_metrics.NewKluctlSourceSpec(obj.Namespace, obj.Name,
			*obj.Spec.Source.URL, obj.Spec.Source.Path, obj.Spec.Source.Ref.String()).Set(0.0)
	}
}
