package controllers

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	internal_metrics "github.com/kluctl/kluctl/v2/pkg/controllers/metrics"
	ssh_pool "github.com/kluctl/kluctl/v2/pkg/git/ssh-pool"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	"k8s.io/apimachinery/pkg/api/errors"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	"path"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

type KluctlDeploymentReconciler struct {
	client.Client
	RestConfig            *rest.Config
	httpClient            *retryablehttp.Client
	requeueDependency     time.Duration
	Scheme                *runtime.Scheme
	EventRecorder         kuberecorder.EventRecorder
	MetricsRecorder       *metrics.Recorder
	ControllerName        string
	DefaultServiceAccount string
	DryRun                bool

	SshPool *ssh_pool.SshPool

	ResultStore results.ResultStore
}

// KluctlDeploymentReconcilerOpts contains options for the BaseReconciler.
type KluctlDeploymentReconcilerOpts struct {
	HTTPRetry int
}

// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gitops.kluctl.io,resources=kluctldeployments/finalizers,verbs=get;create;update;patch;delete
// +kubebuilder:rbac:groups=flux.kluctl.io,resources=kluctldeployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=flux.kluctl.io,resources=kluctldeployments/status,verbs=get
// +kubebuilder:rbac:groups="",resources=configmaps;secrets;serviceaccounts,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *KluctlDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	reconcileStart := time.Now()

	ctx = status.NewContext(ctx, status.NewSimpleStatusHandler(func(message string) {
		log.Info(message)
	}, false, false))

	obj := &kluctlv1.KluctlDeployment{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, r.calcTimeout(obj))
	defer cancel()

	retryInterval := obj.Spec.GetRetryInterval()
	interval := obj.Spec.Interval.Duration

	// Record suspended status metric
	defer r.recordSuspension(ctx, obj)

	// Add our finalizer if it does not exist
	if !controllerutil.ContainsFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer) {
		patch := client.MergeFrom(obj.DeepCopy())
		controllerutil.AddFinalizer(obj, kluctlv1.KluctlDeploymentFinalizer)
		if err := r.Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
			log.Error(err, "unable to register finalizer")
			return ctrl.Result{}, err
		}
	}

	// Examine if the object is under deletion
	if !obj.GetDeletionTimestamp().IsZero() {
		return r.finalize(ctx, obj)
	}

	if r.checkLegacyKluctlDeployment(ctx, obj) {
		return ctrl.Result{}, nil
	}

	// Return early if the KluctlDeployment is suspended.
	if obj.Spec.Suspend {
		log.Info("Reconciliation is suspended for this object")
		return ctrl.Result{}, nil
	}

	// record reconciliation duration
	if r.MetricsRecorder != nil {
		objRef, err := reference.GetReference(r.Scheme, obj)
		if err != nil {
			return ctrl.Result{}, err
		}
		defer r.MetricsRecorder.RecordDuration(*objRef, reconcileStart)
	}

	// record the value of the reconciliation request, if any
	if v, ok := obj.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		patch := client.MergeFrom(obj.DeepCopy())
		obj.Status.LastHandledReconcileAt = v
		if err := r.Status().Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
			return ctrl.Result{}, err
		}
	}

	// set the reconciliation status to progressing
	if obj.Status.ObservedGeneration == 0 {
		patch := client.MergeFrom(obj.DeepCopy())
		setReadiness(obj, metav1.ConditionUnknown, meta.ProgressingReason, "reconciliation in progress")
		if err := r.Status().Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		r.recordReadiness(ctx, obj)
	}

	patch := client.MergeFrom(obj.DeepCopy())
	// reconcile kluctlDeployment by applying the latest revision
	ctrlResult, reconcileErr := r.doReconcile(ctx, obj)
	if err := r.Status().Patch(ctx, obj, patch, client.FieldOwner(r.ControllerName)); err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	r.recordReadiness(ctx, obj)

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
	obj *kluctlv1.KluctlDeployment) (*ctrl.Result, error) {

	r.exportDeploymentObjectToProm(obj)

	doFail := func(reason string, err error) (*ctrl.Result, error) {
		setReadiness(obj, metav1.ConditionFalse, reason, err.Error())
		internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(0.0)
		return nil, err
	}

	doFailPrepare := func(err error) (*ctrl.Result, error) {
		return doFail(kluctlv1.PrepareFailedReason, err)
	}

	oldGeneration := obj.Status.ObservedGeneration
	obj.Status.ObservedGeneration = obj.GetGeneration()
	if v, ok := obj.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]; ok {
		obj.Status.LastHandledDeployAt = v
	}

	pp, err := prepareProject(ctx, r, obj, true)
	if err != nil {
		return doFailPrepare(err)
	}
	defer pp.cleanup()

	obj.Status.ObservedCommit = pp.commit

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
	if err != nil {
		return doFailPrepare(err)
	}

	obj.Status.ProjectKey = &result.ProjectKey{
		GitRepoKey: obj.Spec.Source.URL.RepoKey(),
		SubDir:     path.Clean(obj.Spec.Source.Path),
	}
	obj.Status.TargetKey = &result.TargetKey{
		TargetName:    targetContext.Target.Name,
		Discriminator: targetContext.Target.Discriminator,
	}

	if obj.Status.ProjectKey.SubDir == "." {
		obj.Status.ProjectKey.SubDir = ""
	}

	objectsHash, err := targetContext.DeploymentCollection.CalcObjectsHash()
	if err != nil {
		return doFailPrepare(err)
	}

	needDeploy := false
	needPrune := false
	needValidate := false

	if obj.Status.LastDeployResult == nil || obj.Status.LastObjectsHash != objectsHash {
		// either never deployed or source code changed
		needDeploy = true
	} else if r.checkRequestedDeploy(obj) {
		// explicitly requested a deploy
		needDeploy = true
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

	if obj.Spec.Validate {
		if obj.Status.LastValidateResult == nil || needDeploy {
			// either never validated before or a deployment requested (which required re-validation)
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

	if obj.Spec.Prune {
		needPrune = needDeploy
	} else {
		obj.Status.LastPruneResult = nil
		obj.Status.LastPruneError = ""
	}

	obj.Status.LastObjectsHash = objectsHash

	var deployResult *result.CommandResult
	if needDeploy {
		// deploy the kluctl project
		if obj.Spec.DeployMode == kluctlv1.KluctlDeployModeFull {
			deployResult, err = pt.kluctlDeploy(ctx, targetContext)
		} else if obj.Spec.DeployMode == kluctlv1.KluctlDeployPokeImages {
			deployResult, err = pt.kluctlPokeImages(ctx, targetContext)
		} else {
			err = fmt.Errorf("deployMode '%s' not supported", obj.Spec.DeployMode)
		}
		obj.Status.LastDeployResult = deployResult.BuildSummary()
		obj.Status.LastDeployError = ""
		if err != nil {
			obj.Status.LastDeployError = err.Error()
		}
	}

	if needPrune {
		// run garbage collection for stale objects that do not have pruning disabled
		pruneResult, err := pt.kluctlPrune(ctx, targetContext)
		obj.Status.LastPruneResult = pruneResult.BuildSummary()
		obj.Status.LastPruneError = ""
		if err != nil {
			obj.Status.LastPruneError = err.Error()
		}
	}

	if needValidate {
		validateResult, err := pt.kluctlValidate(ctx, targetContext, deployResult)
		obj.Status.LastValidateResult = validateResult
		obj.Status.LastValidateError = ""
		if err != nil {
			obj.Status.LastValidateError = err.Error()
		}
	}

	var ctrlResult ctrl.Result
	ctrlResult.RequeueAfter = r.nextReconcileTime(obj).Sub(time.Now())
	if ctrlResult.RequeueAfter < 0 {
		ctrlResult.RequeueAfter = 0
		ctrlResult.Requeue = true
	}

	finalStatus, reason := r.buildFinalStatus(obj)
	if reason != kluctlv1.ReconciliationSucceededReason {
		setReadiness(obj, metav1.ConditionFalse, reason, finalStatus)
		internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(0.0)
		return &ctrlResult, fmt.Errorf(finalStatus)
	}
	setReadiness(obj, metav1.ConditionTrue, reason, finalStatus)
	internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(1.0)
	return &ctrlResult, nil
}

func (r *KluctlDeploymentReconciler) buildFinalStatus(obj *kluctlv1.KluctlDeployment) (finalStatus string, reason string) {
	if obj.Status.LastDeployError != "" {
		finalStatus = obj.Status.LastDeployError
		reason = kluctlv1.DeployFailedReason
		return
	} else if obj.Status.LastPruneError != "" {
		finalStatus = obj.Status.LastPruneError
		reason = kluctlv1.PruneFailedReason
		return
	} else if obj.Status.LastValidateError != "" {
		finalStatus = obj.Status.LastValidateError
		reason = kluctlv1.ValidateFailedReason
		return
	}

	deployOk := obj.Status.LastDeployResult != nil && obj.Status.LastDeployResult.Errors == 0
	pruneOk := obj.Status.LastPruneResult != nil && obj.Status.LastPruneResult.Errors == 0
	validateOk := obj.Status.LastValidateResult != nil && len(obj.Status.LastValidateResult.Errors) == 0 && obj.Status.LastValidateResult.Ready

	if !obj.Spec.Prune {
		pruneOk = true
	}
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
	if obj.Spec.Prune && obj.Status.LastPruneResult != nil {
		if finalStatus != "" {
			finalStatus += ", "
		}
		finalStatus += "prune: "
		if pruneOk {
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

	if deployOk && pruneOk {
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

	t := obj.Status.LastDeployResult.Command.EndTime.Add(obj.Spec.DeployInterval.Duration.Duration)
	return &t
}

func (r *KluctlDeploymentReconciler) checkRequestedDeploy(obj *kluctlv1.KluctlDeployment) bool {
	v, ok := obj.Annotations[kluctlv1.KluctlRequestDeployAnnotation]
	if !ok {
		return false
	}
	if v != obj.Status.LastHandledDeployAt {
		return true
	}
	return false
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

	t := obj.Status.LastValidateResult.EndTime.Add(d)
	return &t
}

func (r *KluctlDeploymentReconciler) finalize(ctx context.Context, obj *kluctlv1.KluctlDeployment) (ctrl.Result, error) {
	r.doFinalize(ctx, obj)

	// Record deleted status
	r.recordReadiness(ctx, obj)

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

	_, _ = pt.kluctlDelete(ctx, obj.Status.TargetKey.Discriminator)
}

// checkLegacyKluctlDeployment checks if a legacy KluctlDeployment from the old flux.kluctl.io group is present. If yes
// we must ensure that this object is served by a recent legacy controller version which understands that it should stop
// reconciliation in case the new gitops.kluctl.io object is present
func (r *KluctlDeploymentReconciler) checkLegacyKluctlDeployment(ctx context.Context, obj *kluctlv1.KluctlDeployment) bool {
	log := ctrl.LoggerFrom(ctx)

	var obj2 unstructured.Unstructured
	obj2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "flux.kluctl.io",
		Version: "v1alpha1",
		Kind:    "KluctlDeployment",
	})
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), &obj2)
	if err != nil {
		if errors2.Unwrap(err) != nil {
			err = errors2.Unwrap(err)
		}
		if meta2.IsNoMatchError(err) || errors.IsNotFound(err) || discovery.IsGroupDiscoveryFailedError(err) {
			// legacy object not present, we're safe to continue
			return false
		}
		log.Error(err, "Failed to retrieve legacy KluctlDeployment. Skipping reconciliation.")
		// some unexpected error...we should be on the safe side and bail out reconciliation
		return true
	}

	readyForMigration, _, err := unstructured.NestedBool(obj2.Object, "status", "readyForMigration")
	if err != nil {
		// some unexpected error...we should be on the safe side and bail out reconciliation
		log.Error(err, "Failed to retrieve readyForMigration value. Skipping reconciliation.")
		return true
	}
	if !readyForMigration {
		log.V(1).Info("legacy KluctlDeployment does not have the readyForMigration status set. Skipping reconciliation. " +
			"Please ensure that you have upgraded to the latest version of the legacy flux-kluctl-controller and that is is still running.")
		return true
	}

	return false
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
