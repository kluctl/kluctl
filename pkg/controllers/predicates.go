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

	checkManualRequest := func(aname string) bool {
		v1, ok1 := e.ObjectNew.GetAnnotations()[aname]
		v2, ok2 := e.ObjectOld.GetAnnotations()[aname]
		if !ok2 {
			// if it gets removed, no need for reconciliation
			// the annotation actually gets removed at the beginning of the reconciliation, which would cause an
			// unnecessary reconciliation anyway.
			return false
		}
		return ok1 != ok2 || v1 != v2
	}

	return checkManualRequest(kluctlv1.KluctlRequestReconcileAnnotation) ||
		checkManualRequest(kluctlv1.KluctlRequestDiffAnnotation) ||
		checkManualRequest(kluctlv1.KluctlRequestDeployAnnotation) ||
		checkManualRequest(kluctlv1.KluctlRequestPruneAnnotation) ||
		checkManualRequest(kluctlv1.KluctlRequestValidateAnnotation)
}
