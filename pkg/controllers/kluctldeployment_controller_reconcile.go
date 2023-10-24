package controllers

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
