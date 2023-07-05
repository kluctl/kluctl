package controllers

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func (r *KluctlDeploymentReconciler) getClusterId(ctx context.Context) (string, error) {
	var kubeSystemNs corev1.Namespace
	err := r.Get(ctx, client.ObjectKey{Name: "kube-system"}, &kubeSystemNs)
	if err != nil {
		return "", err
	}
	return string(kubeSystemNs.UID), nil
}
