package controllers

import (
	"context"
	"errors"
	"fmt"
	json_patch "github.com/evanphx/json-patch/v5"
	"github.com/hashicorp/go-multierror"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type getResultPtrCallback func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult

func (r *KluctlDeploymentReconciler) startHandleManualRequest(ctx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string,
	requestAnnotation string, getResultPtr getResultPtrCallback,
) (*kluctlv1.ManualRequestResult, error) {
	key := client.ObjectKeyFromObject(obj)

	log := ctrl.LoggerFrom(ctx)

	v, _ := obj.GetAnnotations()[requestAnnotation]
	if v == "" {
		return nil, nil
	}

	log.Info(fmt.Sprintf("Processing %s: %s", requestAnnotation, v))

	// first remove the annotation so that it doesn't get re-processed
	err := r.patch(ctx, key, false, func(obj *kluctlv1.KluctlDeployment) error {
		a := obj.GetAnnotations()
		if a == nil {
			return nil
		}
		v2 := a[requestAnnotation]
		if v != v2 {
			return nil
		}
		delete(a, requestAnnotation)
		return nil
	})

	var mr kluctlv1.ManualRequest
	err = yaml.ReadYamlString(v, &mr)
	if err != nil {
		return nil, err
	}
	if mr.RequestValue == "" {
		return nil, errors.New("missing requestValue in manual request annotation")
	}

	err = r.applyOverridePatch(ctx, obj, &mr)
	if err != nil {
		return nil, err
	}

	resultPtr := getResultPtr(&obj.Status)
	rr := *resultPtr

	if rr == nil || rr.Request.RequestValue != v {
		rr = &kluctlv1.ManualRequestResult{
			Request:     mr,
			StartTime:   metav1.Now(),
			ReconcileId: reconcileId,
		}
		err := r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			*getResultPtr(status) = rr
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
	obj *kluctlv1.KluctlDeployment, rr *kluctlv1.ManualRequestResult, resultId string, commandErr error, getResultPtr getResultPtrCallback) error {
	key := client.ObjectKeyFromObject(obj)

	if rr == nil {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)

	log.Info("Finishing handling of manual request")

	t := metav1.Now()
	rr.EndTime = &t
	rr.ResultId = resultId
	if commandErr != nil {
		rr.CommandError = commandErr.Error()
	}
	return r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
		*getResultPtr(status) = rr
		return nil
	})
}

func (r *KluctlDeploymentReconciler) applyOverridePatch(ctx context.Context, obj *kluctlv1.KluctlDeployment, mr *kluctlv1.ManualRequest) error {
	if mr.OverridesPatch == nil {
		return nil
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Applying onetime patch from manual request")

	objJson, err := yaml.WriteJsonString(obj)
	if err != nil {
		return err
	}

	objJson2, err := json_patch.MergePatch([]byte(objJson), mr.OverridesPatch.Raw)
	if err != nil {
		return err
	}

	var patchedObj kluctlv1.KluctlDeployment
	err = yaml.ReadYamlBytes(objJson2, &patchedObj)
	if err != nil {
		return err
	}

	testObj1 := *obj
	testObj2 := patchedObj
	testObj1.Spec = kluctlv1.KluctlDeploymentSpec{}
	testObj2.Spec = kluctlv1.KluctlDeploymentSpec{}

	if !reflect.DeepEqual(&testObj1, &testObj2) {
		return fmt.Errorf("override patch tried to modify non-spec fields")
	}

	*obj = patchedObj
	return nil
}

func (r *KluctlDeploymentReconciler) prepareProjectAndTarget(ctx context.Context, obj *kluctlv1.KluctlDeployment) (*preparedProject, *preparedTarget, *target_context.TargetContext, error) {
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

type reconcileCallback func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext,
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

	cmdResult, failReason, cmdErr := reconcileRequest(req, targetContext, pt, reconcileId, objectsHash)

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

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult {
		return &status.DiffRequestResult
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"diff", kluctlv1.KluctlRequestDiffAnnotation,
		getResultPtr, false,
		func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult := pt.kluctlDiff(targetContext, nil)
			err := pt.writeCommandResult(ctx, cmdResult, rr, "diff", reconcileId, objectsHash, true)
			if err != nil {
				log.Error(err, "Failed to write diff result")
			}
			obj.Status.SetLastDiffResult(cmdResult.BuildSummary())
			return cmdResult, kluctlv1.DiffFailedReason, r.buildErrorFromResult(cmdResult.Errors, cmdResult.Warnings, "diff")
		})
}

func (r *KluctlDeploymentReconciler) reconcileDeployRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult {
		return &status.DeployRequestResult
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		obj.Spec.DeployMode, kluctlv1.KluctlRequestDeployAnnotation,
		getResultPtr, false,
		func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult, err := pt.kluctlDeployOrPokeImages(obj.Spec.DeployMode, targetContext)
			if err != nil {
				return nil, kluctlv1.DeployFailedReason, err
			}
			err = pt.writeCommandResult(ctx, cmdResult, rr, "deploy", reconcileId, objectsHash, true)
			if err != nil {
				log.Error(err, "Failed to write deploy result")
			}
			obj.Status.SetLastDeployResult(cmdResult.BuildSummary())
			return cmdResult, kluctlv1.DeployFailedReason, r.buildErrorFromResult(cmdResult.Errors, cmdResult.Warnings, "deploy")
		})
}

func (r *KluctlDeploymentReconciler) reconcilePruneRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult {
		return &status.PruneRequestResult
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"prune", kluctlv1.KluctlRequestPruneAnnotation,
		getResultPtr, false,
		func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult := pt.kluctlPrune(targetContext)
			err := pt.writeCommandResult(ctx, cmdResult, rr, "prune", reconcileId, objectsHash, true)
			if err != nil {
				log.Error(err, "Failed to write prune result")
			}
			obj.Status.SetLastDeployResult(cmdResult.BuildSummary())
			return cmdResult, kluctlv1.PruneFailedReason, r.buildErrorFromResult(cmdResult.Errors, cmdResult.Warnings, "prune")
		})
}

func (r *KluctlDeploymentReconciler) reconcileValidateRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult {
		return &status.ValidateRequestResult
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"validate", kluctlv1.KluctlRequestValidateAnnotation,
		getResultPtr, false,
		func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			cmdResult := pt.kluctlValidate(targetContext)
			err := pt.writeValidateResult(ctx, cmdResult, rr, reconcileId, objectsHash)
			if err != nil {
				log.Error(err, "Failed to write validate result")
			}
			obj.Status.SetLastValidateResult(cmdResult)
			return cmdResult, kluctlv1.ValidateFailedReason, r.buildErrorFromResult(cmdResult.Errors, cmdResult.Warnings, "validate")
		})
}

func (r *KluctlDeploymentReconciler) reconcileFullRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {

	defer func() {
		obj.Status.ObservedGeneration = obj.GetGeneration()
	}()

	getResultPtr := func(status *kluctlv1.KluctlDeploymentStatus) **kluctlv1.ManualRequestResult {
		return &status.ReconcileRequestResult
	}

	forceReconcile := true
	if obj.Spec.Suspend {
		// suspend disables forces reconciliation on every request. It still allows reconciliation via manual requests.
		forceReconcile = false
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"reconcile", kluctlv1.KluctlRequestReconcileAnnotation,
		getResultPtr, forceReconcile,
		func(rr *kluctlv1.ManualRequestResult, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
			return r.reconcileFullRequest2(rr, ctx, timeoutCtx, obj, reconcileId, targetContext, pt, reconcileId, objectsHash)
		})
}

func (r *KluctlDeploymentReconciler) reconcileFullRequest2(rr *kluctlv1.ManualRequestResult, ctx context.Context, timeoutCtx context.Context, obj *kluctlv1.KluctlDeployment, reconcileId string, targetContext *target_context.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, string, error) {
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
	}

	if !needDeploy && obj.Status.LastObjectsHash != objectsHash {
		// force full drift detection as we can't know which objects changed in-between
		r.updateResourceVersions(key, nil, nil)
	}

	obj.Status.LastObjectsHash = objectsHash
	obj.Status.LastManualObjectsHash = obj.Spec.ManualObjectsHash

	var cmdErrors error

	var deployResult *result.CommandResult
	if needDeploy {
		err := r.patchProgressingCondition(ctx, obj, fmt.Sprintf("Performing kluctl %s", obj.Spec.DeployMode), false)
		if err != nil {
			return nil, kluctlv1.DeployFailedReason, err
		}

		deployResult, err = pt.kluctlDeployOrPokeImages(obj.Spec.DeployMode, targetContext)
		if err != nil {
			return nil, kluctlv1.DeployFailedReason, err
		}
		err = pt.writeCommandResult(ctx, deployResult, rr, obj.Spec.DeployMode, reconcileId, objectsHash, false)
		if err != nil {
			log.Error(err, "Failed to write deploy result")
		}
		obj.Status.SetLastDeployResult(deployResult.BuildSummary())

		cmdErrors = r.buildErrorFromResult(deployResult.Errors, deployResult.Warnings, "deploy")

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
		validateResult := pt.kluctlValidate(targetContext)
		err = pt.writeValidateResult(ctx, validateResult, rr, reconcileId, objectsHash)
		if err != nil {
			log.Error(err, "Failed to write deploy result")
		}
		obj.Status.SetLastValidateResult(validateResult)

		err = r.buildErrorFromResult(validateResult.Errors, validateResult.Warnings, "validate")
		if err != nil {
			if cmdErrors == nil {
				cmdErrors = err
			} else {
				cmdErrors = multierror.Append(cmdErrors, err)
			}
		}
	}

	if needDriftDetection {
		err := r.patchProgressingCondition(ctx, obj, "Performing drift detection", false)
		if err != nil {
			return nil, kluctlv1.DiffFailedReason, err
		}

		resourceVersions := r.getResourceVersions(key)

		diffResult := pt.kluctlDiff(targetContext, resourceVersions)
		err = pt.addCommandResultInfo(ctx, diffResult, rr, reconcileId, objectsHash)
		if err != nil {
			log.Error(err, "addCommandResultInfo failed")
		}
		driftDetectionResult := diffResult.BuildDriftDetectionResult()
		obj.Status.SetLastDriftDetectionResult(driftDetectionResult)

		err = r.buildErrorFromResult(diffResult.Errors, diffResult.Warnings, "diff")
		if err != nil {
			if cmdErrors == nil {
				cmdErrors = err
			} else {
				cmdErrors = multierror.Append(cmdErrors, err)
			}
		}

		r.updateResourceVersions(key, diffResult.Objects, driftDetectionResult.Objects)
	}

	return nil, "", cmdErrors
}
