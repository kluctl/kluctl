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

	check := func(aname string) bool {
		v1, ok1 := e.ObjectNew.GetAnnotations()[aname]
		v2, ok2 := e.ObjectOld.GetAnnotations()[aname]
		return ok1 != ok2 || v1 != v2
	}

	return check(kluctlv1.KluctlRequestReconcileAnnotation) ||
		check(kluctlv1.KluctlRequestDiffAnnotation) ||
		check(kluctlv1.KluctlRequestDeployAnnotation) ||
		check(kluctlv1.KluctlRequestPruneAnnotation) ||
		check(kluctlv1.KluctlRequestValidateAnnotation)
}
