package controllers

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SetupWithManager sets up the controller with the Manager.
func (r *KluctlDeploymentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, opts KluctlDeploymentReconcilerOpts) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: opts.Concurrency,
		}).
		For(&kluctlv1.KluctlDeployment{}, builder.WithPredicates(
			predicate.Or(predicate.GenerationChangedPredicate{}, ReconcileRequestedPredicate{}),
		)).
		Complete(r)
}
