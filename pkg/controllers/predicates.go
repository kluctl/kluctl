package controllers

import (
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ReconcileRequestedPredicate struct {
	predicate.Funcs
}

// Update implements the default UpdateEvent filter for validating flux_utils.ReconcileRequestAnnotation changes.
func (ReconcileRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	return false
}

type DeployRequestedPredicate struct {
	predicate.Funcs
}

func (DeployRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	return false
}
