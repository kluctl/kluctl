package controllers

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type setResultCallback func(status *kluctlv1.KluctlDeploymentStatus, requestResult *kluctlv1.RequestResult)
type getLegacyOldValueCallback func(status *kluctlv1.KluctlDeploymentStatus) string

func (r *KluctlDeploymentReconciler) startHandleManualRequest(ctx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string, rr *kluctlv1.RequestResult,
	requestAnnotation string, setResult setResultCallback, getLegacyOldValue getLegacyOldValueCallback,
) (*kluctlv1.RequestResult, error) {
	key := client.ObjectKeyFromObject(obj)

	v, _ := obj.GetAnnotations()[requestAnnotation]
	if v == "" {
		return nil, nil
	}
	if rr == nil && getLegacyOldValue(&obj.Status) == v {
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
			setResult(status, rr)
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
			setResult(status, rr)
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
	obj *kluctlv1.KluctlDeployment, rr *kluctlv1.RequestResult, resultId string, commandErr error, setResult setResultCallback) error {
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
		setResult(status, rr)
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
	pt *preparedTarget, reconcileID string, objectsHash string) (any, error)

func (r *KluctlDeploymentReconciler) reconcileManualRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string,
	commandName string, annotationName string, failReason string,
	setResult setResultCallback, getLegacyOldValue getLegacyOldValueCallback,
	reconcileRequest reconcileCallback,
) (bool, error) {
	req, err := r.startHandleManualRequest(ctx, obj, reconcileId, obj.Status.DiffRequestResult, annotationName, setResult, getLegacyOldValue)
	if err != nil {
		return false, r.patchFailPrepare(ctx, obj, err)
	}
	if req == nil {
		return false, nil
	}

	doError := func(resultId string, err error) (bool, error) {
		err2 := r.patchFailPrepare(ctx, obj, err)
		if err2 != nil {
			err = multierror.Append(err, err2)
		}
		err2 = r.finishHandleManualRequest(ctx, obj, req, resultId, err, setResult)
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

	cmdResult, cmdErr := reconcileRequest(targetContext, pt, reconcileId, objectsHash)

	resultId := ""
	if x, ok := cmdResult.(*result.CommandResult); ok {
		resultId = x.Id
	}
	if x, ok := cmdResult.(*result.ValidateResult); ok {
		resultId = x.Id
	}

	err = r.finishHandleManualRequest(ctx, obj, req, resultId, cmdErr, setResult)
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

	setRequestResult := func(status *kluctlv1.KluctlDeploymentStatus, requestResult *kluctlv1.RequestResult) {
		status.DiffRequestResult = requestResult
	}
	getLegacyOldValue := func(status *kluctlv1.KluctlDeploymentStatus) string {
		return ""
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"diff", kluctlv1.KluctlRequestDiffAnnotation, kluctlv1.DiffFailedReason,
		setRequestResult, getLegacyOldValue,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, error) {
			cmdResult, cmdErr := pt.kluctlDiff(ctx, targetContext, reconcileId, objectsHash, nil, true)
			err := obj.Status.SetLastDiffResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write diff result")
			}
			return cmdResult, err
		})
}

func (r *KluctlDeploymentReconciler) reconcileDeployRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledDeployAt = ""
	}()

	setRequestResult := func(status *kluctlv1.KluctlDeploymentStatus, requestResult *kluctlv1.RequestResult) {
		status.DeployRequestResult = requestResult
	}
	getLegacyOldValue := func(status *kluctlv1.KluctlDeploymentStatus) string {
		return status.LastHandledDeployAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		obj.Spec.DeployMode, kluctlv1.KluctlRequestDeployAnnotation, kluctlv1.DeployFailedReason,
		setRequestResult, getLegacyOldValue,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, error) {
			cmdResult, cmdErr := pt.kluctlDeployOrPokeImages(ctx, obj.Spec.DeployMode, targetContext, reconcileId, objectsHash, true)
			err := obj.Status.SetLastDeployResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write deploy result")
			}
			return cmdResult, err
		})
}

func (r *KluctlDeploymentReconciler) reconcilePruneRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledPruneAt = ""
	}()

	setRequestResult := func(status *kluctlv1.KluctlDeploymentStatus, requestResult *kluctlv1.RequestResult) {
		status.PruneRequestResult = requestResult
	}
	getLegacyOldValue := func(status *kluctlv1.KluctlDeploymentStatus) string {
		return status.LastHandledPruneAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"prune", kluctlv1.KluctlRequestPruneAnnotation, kluctlv1.PruneFailedReason,
		setRequestResult, getLegacyOldValue,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, error) {
			cmdResult, cmdErr := pt.kluctlPrune(ctx, targetContext, reconcileId, objectsHash)
			err := obj.Status.SetLastDeployResult(cmdResult.BuildSummary(), cmdErr)
			if err != nil {
				log.Error(err, "Failed to write prune result")
			}
			return cmdResult, err
		})
}

func (r *KluctlDeploymentReconciler) reconcileValidateRequest(ctx context.Context, timeoutCtx context.Context,
	obj *kluctlv1.KluctlDeployment, reconcileId string) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	defer func() {
		obj.Status.LastHandledValidateAt = ""
	}()

	setRequestResult := func(status *kluctlv1.KluctlDeploymentStatus, requestResult *kluctlv1.RequestResult) {
		status.ValidateRequestResult = requestResult
	}
	getLegacyOldValue := func(status *kluctlv1.KluctlDeploymentStatus) string {
		return status.LastHandledValidateAt
	}

	return r.reconcileManualRequest(ctx, timeoutCtx, obj, reconcileId,
		"validate", kluctlv1.KluctlRequestValidateAnnotation, kluctlv1.ValidateFailedReason,
		setRequestResult, getLegacyOldValue,
		func(targetContext *kluctl_project.TargetContext, pt *preparedTarget, reconcileID string, objectsHash string) (any, error) {
			cmdResult, cmdErr := pt.kluctlValidate(ctx, targetContext, nil, reconcileId, objectsHash)
			err := obj.Status.SetLastValidateResult(cmdResult, cmdErr)
			if err != nil {
				log.Error(err, "Failed to write validate result")
			}
			return cmdResult, err
		})
}
