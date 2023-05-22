package controllers

import (
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	meta "github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func setReadiness(obj *kluctlv1.KluctlDeployment, status metav1.ConditionStatus, reason, message string) {
	newCondition := metav1.Condition{
		Type:               meta.ReadyCondition,
		Status:             status,
		Reason:             reason,
		Message:            trimString(message, kluctlv1.MaxConditionMessageLength),
		ObservedGeneration: obj.Generation,
	}

	c := obj.GetConditions()
	apimeta.SetStatusCondition(&c, newCondition)
	obj.SetConditions(c)
}

func trimString(str string, limit int) string {
	if len(str) <= limit {
		return str
	}

	return str[0:limit] + "..."
}
