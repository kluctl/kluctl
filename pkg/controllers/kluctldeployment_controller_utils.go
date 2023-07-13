package controllers

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (r *KluctlDeploymentReconciler) event(ctx context.Context, obj *kluctlv1.KluctlDeployment, warning bool, msg string, metadata map[string]string) {
	if metadata == nil {
		metadata = map[string]string{}
	}

	reason := "info"
	if warning {
		reason = "warning"
	}
	if c := apimeta.FindStatusCondition(obj.GetConditions(), meta.ReadyCondition); c != nil {
		reason = c.Reason
	}

	eventtype := "Normal"
	if warning {
		eventtype = "Warning"
	}

	r.EventRecorder.AnnotatedEventf(obj, metadata, eventtype, reason, msg)
}

func (r *KluctlDeploymentReconciler) recordReadiness(ctx context.Context, obj *kluctlv1.KluctlDeployment) {
	if r.MetricsRecorder == nil {
		return
	}
	log := ctrl.LoggerFrom(ctx)

	// make sure we use the latest conditions
	var latest kluctlv1.KluctlDeployment
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), &latest)
	if err != nil {
		return
	}
	obj = &latest

	objRef, err := reference.GetReference(r.Scheme, obj)
	if err != nil {
		log.Error(err, "unable to record readiness metric")
		return
	}
	if rc := apimeta.FindStatusCondition(obj.GetConditions(), meta.ReadyCondition); rc != nil {
		r.MetricsRecorder.RecordCondition(*objRef, *rc, !obj.GetDeletionTimestamp().IsZero())
	} else {
		r.MetricsRecorder.RecordCondition(*objRef, metav1.Condition{
			Type:   meta.ReadyCondition,
			Status: metav1.ConditionUnknown,
		}, !obj.GetDeletionTimestamp().IsZero())
	}
}

func (r *KluctlDeploymentReconciler) recordSuspension(ctx context.Context, obj *kluctlv1.KluctlDeployment) {
	if r.MetricsRecorder == nil {
		return
	}
	log := ctrl.LoggerFrom(ctx)

	objRef, err := reference.GetReference(r.Scheme, obj)
	if err != nil {
		log.Error(err, "unable to record suspended metric")
		return
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		r.MetricsRecorder.RecordSuspend(*objRef, false)
	} else {
		r.MetricsRecorder.RecordSuspend(*objRef, obj.Spec.Suspend)
	}
}

func (r *KluctlDeploymentReconciler) recordDuration(ctx context.Context, obj *kluctlv1.KluctlDeployment, startTime time.Time) {
	if r.MetricsRecorder == nil {
		return
	}
	log := ctrl.LoggerFrom(ctx)

	objRef, err := reference.GetReference(r.Scheme, obj)
	if err != nil {
		log.Error(err, "unable to record duration metric")
		return
	}

	r.MetricsRecorder.RecordDuration(*objRef, startTime)
}

func (r *KluctlDeploymentReconciler) patchCondition(ctx context.Context, key client.ObjectKey, updateConditions func(c *[]metav1.Condition) error) error {
	backoff := wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Jitter:   1.0,
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		var latest kluctlv1.KluctlDeployment
		err := r.Client.Get(ctx, key, &latest)
		if err != nil {
			return false, err
		}

		conditionsPatch := client.MergeFromWithOptions(latest.DeepCopy(), client.MergeFromWithOptimisticLock{})

		c := latest.GetConditions()
		err = updateConditions(&c)
		if err != nil {
			return false, err
		}
		latest.SetConditions(c)

		err = r.Status().Patch(ctx, &latest, conditionsPatch, client.FieldOwner(r.ControllerName))
		if err != nil {
			if errors.IsConflict(err) {
				// retry
				return false, nil
			}
			return false, err
		}

		return true, nil
	})
}
