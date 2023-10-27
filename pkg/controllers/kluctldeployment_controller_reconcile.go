package controllers

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type getResultPtrCallback func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string)

func (r *KluctlDeploymentReconciler) startHandleManualRequest(ctx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string,
	requestAnnotation string, getResultPtr getResultPtrCallback,
) (*kluctlv1.RequestResult, error) {
	key := client.ObjectKeyFromObject(obj)

	v, _ := obj.GetAnnotations()[requestAnnotation]
	if v == "" {
		return nil, nil
	}

	resultPtr, legacyValue := getResultPtr(&obj.Status)
	rr := *resultPtr

	if rr == nil && legacyValue == v {
		// legacy value in status is still present, and we never executed the new request status handling
		// ensure that we don't accidentally re-process the request
		t := metav1.Now()
		rr = &kluctlv1.RequestResult{
			RequestValue: v,
			StartTime:    t,
			EndTime:      &t,
			ReconcileId:  "unknown",
		}
		err := r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			resultPtr, _ := getResultPtr(status)
			*resultPtr = rr
			return nil
		})
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	if rr == nil || rr.RequestValue != v {
		rr = &kluctlv1.RequestResult{
			RequestValue: v,
			StartTime:    metav1.Now(),
			ReconcileId:  reconcileId,
		}
		err := r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			resultPtr, _ := getResultPtr(status)
			*resultPtr = rr
			return nil
		})
		if err != nil {
			return nil, err
		}
		return rr, nil
	}
	return nil, nil
}

func (r *KluctlDeploymentReconciler) finishHandleManualRequest(ctx context.Context,
	obj *kluctlv1.KluctlDeployment, rr *kluctlv1.RequestResult, resultId string, commandErr error, getResultPtr getResultPtrCallback) error {
	key := client.ObjectKeyFromObject(obj)

	if rr == nil {
		return nil
	}

	t := metav1.Now()
	rr.EndTime = &t
	rr.ResultId = resultId
	if commandErr != nil {
		rr.CommandError = commandErr.Error()
	}
	return r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
		resultPtr, _ := getResultPtr(status)
		*resultPtr = rr
		return nil
	})
}

func (r *KluctlDeploymentReconciler) prepareProjectAndTarget(ctx context.Context, obj *kluctlv1.KluctlDeployment) (*preparedProject, *preparedTarget, *kluctl_project.TargetContext, error) {
	pp, err := prepareProject(ctx, r, obj, true)
	if err != nil {
		return nil, nil, nil, err
	}

	pt := pp.newTarget()

	lp, err := pp.loadKluctlProject(ctx, pt)
	if err != nil {
		return pp, pt, nil, err
	}

	targetContext, err := pt.loadTarget(ctx, lp)
	if err != nil {
		return pp, pt, nil, err
	}
	return pp, pt, targetContext, err
}

type reconcileCallback func(targetContext *kluctl_project.TargetContext,
	pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error)

func (r *KluctlDeploymentReconciler) reconcileManualRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string,
	commandName string, annotationName string,
	getResultPtr getResultPtrCallback, force bool,
	reconcileRequest reconcileCallback,
) (bool, error) {
	req, err := r.startHandleManualRequest(ctx, obj, reconcileId, annotationName, getResultPtr)
	if err != nil {
		return false, r.patchFailPrepare(ctx, obj, err)
	}
	if req == nil && !force {
		return false, nil
	}

	obj.Status.LastPrepareError = ""

	doError := func(resultId string, err error) (bool, error) {
		err2 := r.patchFailPrepare(ctx, obj, err)
		if err2 != nil {
			err = multierror.Append(err, err2)
		}
		err2 = r.finishHandleManualRequest(ctx, obj, req, resultId, err, getResultPtr)
		if err2 != nil {
			err = multierror.Append(err, err2)
		}
		return true, err
	}

	pp, pt, targetContext, err := r.prepareProjectAndTarget(timeoutCtx, obj)
	if pp != nil {
		obj.Status.ObservedCommit = pp.co.CheckedOutCommit
		defer pp.cleanup()
	}
	if err != nil {
		return doError("", err)
	}

	objectsHash, err := targetContext.DeploymentCollection.CalcObjectsHash()
	if err != nil {
		return doError("", err)
	}

	err = r.patchProgressingCondition(ctx, obj, fmt.Sprintf("Performing %s", commandName), false)
	if err != nil {
		return doError("", err)
	}

	cmdResult, failReason, cmdErr := reconcileRequest(targetContext, pt, reconcileId, objectsHash)

	resultId := ""
	if x, ok := cmdResult.(*result.CommandResult); ok {
		resultId = x.Id
	}
	if x, ok := cmdResult.(*result.ValidateResult); ok {
		resultId = x.Id
	}

	err = r.finishHandleManualRequest(ctx, obj, req, resultId, cmdErr, getResultPtr)
	if err != nil {
		if cmdErr != nil {
			err = multierror.Append(err, cmdErr)
		}
		return true, r.patchFail(ctx, obj, failReason, err)
	}

	return true, nil
}

func (r *KluctlDeploymentReconciler) reconcileManualRequests(ctx context.Context, timeoutCtx context.Context, obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	processed, err := r.reconcileDiffRequest(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return false, r.patchFailPrepare(ctx, obj, err)
	}
	if processed {
		return true, nil
	}

	processed, err = r.reconcileDeployRequest(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return true, r.patchFailPrepare(ctx, obj, err)
	}
	if processed {
		return true, nil
	}

	processed, err = r.reconcilePruneRequest(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return true, r.patchFailPrepare(ctx, obj, err)
	}
	if processed {
		return true, nil
	}

	processed, err = r.reconcileValidateRequest(ctx, timeoutCtx, obj, reconcileId)
	if err != nil {
		return true, r.patchFailPrepare(ctx, obj, err)
	}
	if processed {
		return true, nil
	}

	return false, nil
}

func (r *KluctlDeploymentReconciler) reconcileDiffRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string) {
		return &status.DiffRequestResult, ""
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"diff", kluctlv1.KluctlRequestDiffAnnotation,
		getResultPtr, false,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult, cmdErr := pt.kluctlDiff(ctx, targetContext, reconcileId, objectsHash, nil, true)
			err := obj.Status.SetLastDiffResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write diff result")
			}
			return cmdResult, kluctlv1.DiffFailedReason, err
		})
}

func (r *KluctlDeploymentReconciler) reconcileDeployRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledDeployAt = ""
	}()

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string) {
		return &status.DeployRequestResult, status.LastHandledDeployAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		obj.Spec.DeployMode, kluctlv1.KluctlRequestDeployAnnotation,
		getResultPtr, false,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult, cmdErr := pt.kluctlDeployOrPokeImages(ctx, obj.Spec.DeployMode, targetContext, reconcileId, objectsHash, true)
			err := obj.Status.SetLastDeployResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write deploy result")
			}
			return cmdResult, kluctlv1.DeployFailedReason, err
		})
}

func (r *KluctlDeploymentReconciler) reconcilePruneRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledPruneAt = ""
	}()

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string) {
		return &status.PruneRequestResult, status.LastHandledPruneAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"prune", kluctlv1.KluctlRequestPruneAnnotation,
		getResultPtr, false,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult, cmdErr := pt.kluctlPrune(ctx, targetContext, reconcileId, objectsHash, true)
			err := obj.Status.SetLastDeployResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write prune result")
			}
			return cmdResult, kluctlv1.PruneFailedReason, err
		})
}

func (r *KluctlDeploymentReconciler) reconcileValidateRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledValidateAt = ""
	}()

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string) {
		return &status.ValidateRequestResult, status.LastHandledValidateAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"validate", kluctlv1.KluctlRequestValidateAnnotation,
		getResultPtr, false,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult, cmdErr := pt.kluctlValidate(ctx, targetContext, nil, reconcileId, objectsHash)
			err := obj.Status.SetLastValidateResult(cmdResult, cmdErr)
			if err != nil {
				log.Error(err, "Failed to write validate result")
			}
			return cmdResult, kluctlv1.ValidateFailedReason, err
		})
}

func (r *KluctlDeploymentReconciler) reconcileFullRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {

	defer func() {
		obj.Status.LastHandledReconcileAt = ""
		obj.Status.ObservedGeneration = obj.GetGeneration()
	}()

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) (**kluctlv1.RequestResult, string) {
		return &status.ReconcileRequestResult, status.LastHandledReconcileAt
	}

	forceReconcile := true
	if obj.Spec.Suspend {
		// suspend disables forces reconciliation on every request. It still allows reconciliation via manual requests.
		forceReconcile = false
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"reconcile", kluctlv1.KluctlRequestReconcileAnnotation,
		getResultPtr, forceReconcile,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			return r.reconcileFullRequest2(ctx, timeoutCtx, obj, reconcileId, targetContext, pt, reconcileId, objectsHash)
		})
}

func (r *KluctlDeploymentReconciler) reconcileFullRequest2(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string, targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
	key := client.ObjectKeyFromObject(obj)
	log := ctrl.LoggerFrom(ctx)

	err := r.patchTargetKey(ctx, obj, targetContext)
	if err != nil {
		return nil, kluctlv1.PrepareFailedReason, err
	}

	needDeploy := false
	needValidate := false
	needDriftDetection := true

	if obj.Status.LastDeployResult == nil || obj.Status.LastObjectsHash != objectsHash {
		// either never deployed or source code changed
		needDeploy = true
	} else if obj.Spec.Manual && !utils.StrPtrEquals(obj.Status.LastManualObjectsHash, obj.Spec.ManualObjectsHash) {
		// approval hash was changed
		needDeploy = true
	} else if obj.Status.ObservedGeneration != obj.GetGeneration() {
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

	if !needDeploy && obj.Status.LastObjectsHash != objectsHash {
		// force full drift detection as we can't know which objects changed in-between
		r.updateResourceVersions(key, nil, nil)
	}

	obj.Status.LastObjectsHash = objectsHash
	obj.Status.LastManualObjectsHash = obj.Spec.ManualObjectsHash

	var deployResult *result.CommandResult
	if needDeploy {
		err := r.patchProgressingCondition(ctx, obj, fmt.Sprintf("Performing kluctl %s", obj.Spec.DeployMode), false)
		if err != nil {
			return nil, kluctlv1.DeployFailedReason, err
		}

		var cmdErr error
		deployResult, cmdErr = pt.kluctlDeployOrPokeImages(timeoutCtx, obj.Spec.DeployMode, targetContext, reconcileId, objectsHash, false)
		err = obj.Status.SetLastDeployResult(deployResult.BuildSummary(), cmdErr)
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

	if needValidate {
		err := r.patchProgressingCondition(ctx, obj, "Performing kluctl validate", false)
		if err != nil {
			return nil, kluctlv1.ValidateFailedReason, err
		}
		validateResult, cmdErr := pt.kluctlValidate(timeoutCtx, targetContext, deployResult, reconcileId, objectsHash)
		err = obj.Status.SetLastValidateResult(validateResult, cmdErr)
		if err != nil {
			log.Error(err, "Failed to write validate result")
		}
	}

	if needDriftDetection {
		err := r.patchProgressingCondition(ctx, obj, "Performing drift detection", false)
		if err != nil {
			return nil, kluctlv1.DiffFailedReason, err
		}

		resourceVersions := r.getResourceVersions(key)

		diffResult, cmdErr := pt.kluctlDiff(timeoutCtx, targetContext, reconcileId, objectsHash, resourceVersions, false)
		driftDetectionResult := diffResult.BuildDriftDetectionResult()
		err = obj.Status.SetLastDriftDetectionResult(driftDetectionResult, cmdErr)
		if err != nil {
			log.Error(err, "Failed to write drift detection result")
		}

		r.updateResourceVersions(key, diffResult.Objects, driftDetectionResult.Objects)
	}

	return nil, "", nil
}
