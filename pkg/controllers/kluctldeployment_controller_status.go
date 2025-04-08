package controllers

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/yaml"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	internal_metrics "github.com/kluctl/kluctl/v2/pkg/controllers/metrics"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project/target-context"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"path"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (r *KluctlDeploymentReconciler) patchReadyCondition(ctx context.Context, obj *kluctlv1.KluctlDeployment, status metav1.ConditionStatus, reason, message string) error {
	log := ctrl.LoggerFrom(ctx)
	key := client.ObjectKeyFromObject(obj)

	log.Info(fmt.Sprintf("updating readiness condition: status=%s, reason=%s, message=%s", status, reason, message))
	return r.patchCondition(ctx, key, func(c *[]metav1.Condition) error {
		setReadinessCondition(c, status, reason, message, obj.Generation)
		apimeta.RemoveStatusCondition(c, meta.ReconcilingCondition)
		return nil
	})
}

// patchFail returns the original error + patchErr if required
func (r *KluctlDeploymentReconciler) patchFail(ctx context.Context, obj *kluctlv1.KluctlDeployment, reason string, err error) error {
	var lastDeployResult *result.CommandResultSummary
	if obj.Status.LastDeployResult != nil {
		parseErr := yaml.ReadYamlBytes(obj.Status.LastDeployResult.Raw, &lastDeployResult)
		if parseErr != nil {
			log.Info(fmt.Sprintf("Failed to parse last deploy result: %s", parseErr.Error()))
		}
	}

	if lastDeployResult != nil {
		internal_metrics.NewKluctlLastDeployStartTime(obj.Namespace, obj.Name).Set(float64(lastDeployResult.Command.StartTime.Unix()))
	}

	internal_metrics.NewKluctlLastObjectStatus(obj.Namespace, obj.Name).Set(0.0)
	patchErr := r.patchReadyCondition(ctx, obj, metav1.ConditionFalse, reason, err.Error())
	if patchErr != nil {
		err = multierror.Append(err, patchErr)
	}
	return err
}

func (r *KluctlDeploymentReconciler) patchFailPrepare(ctx context.Context, obj *kluctlv1.KluctlDeployment, err error) error {
	obj.Status.LastPrepareError = err.Error()
	var err2 *multierror.Error
	if errors2.As(err, &err2) {
		// prepare errors tend to be extremely long, which makes tools like k9s unusable
		err = fmt.Errorf("prepare failed with %d errors. Check status.lastPrepareError for details", len(err2.Errors))
	} else {
		err = fmt.Errorf("prepare failed. Check status.lastPrepareError for details")
	}
	return r.patchFail(ctx, obj, kluctlv1.PrepareFailedReason, err)
}

func (r *KluctlDeploymentReconciler) patchProgressingCondition(ctx context.Context, obj *kluctlv1.KluctlDeployment, message string, keepOldReadyStatus bool) error {
	log := ctrl.LoggerFrom(ctx)
	key := client.ObjectKeyFromObject(obj)

	// keep old status until we're doing real work (deploying, validating, ...)
	oldReadyCondition := apimeta.FindStatusCondition(obj.GetConditions(), meta.ReadyCondition)

	log.Info("progressing: " + message)
	return r.patchCondition(ctx, key, func(c *[]metav1.Condition) error {
		if keepOldReadyStatus && oldReadyCondition != nil {
			setReadinessCondition(c, oldReadyCondition.Status, oldReadyCondition.Reason, oldReadyCondition.Message, obj.Generation)
		} else {
			setReadinessCondition(c, metav1.ConditionUnknown, meta.ProgressingReason, "Reconciliation in progress", obj.Generation)
		}
		setReconcilingCondition(c, metav1.ConditionTrue, meta.ProgressingReason, message, obj.Generation)
		return nil
	})
}

func (r *KluctlDeploymentReconciler) patchProjectKey(ctx context.Context, obj *kluctlv1.KluctlDeployment) error {
	key := client.ObjectKeyFromObject(obj)

	var newProjectKey gittypes.ProjectKey
	if obj.Spec.Source.Git != nil {
		repoKey, err := gittypes.NewRepoKeyFromGitUrl(obj.Spec.Source.Git.URL)
		if err != nil {
			return err
		}
		newProjectKey = gittypes.ProjectKey{
			RepoKey: repoKey,
			SubDir:  path.Clean(obj.Spec.Source.Git.Path),
		}
	} else if obj.Spec.Source.Oci != nil {
		repoKey, err := gittypes.NewRepoKeyFromUrl(obj.Spec.Source.Oci.URL)
		if err != nil {
			return err
		}
		newProjectKey = gittypes.ProjectKey{
			RepoKey: repoKey,
			SubDir:  path.Clean(obj.Spec.Source.Oci.Path),
		}
	} else if obj.Spec.Source.URL != nil {
		repoKey, err := gittypes.NewRepoKeyFromGitUrl(*obj.Spec.Source.URL)
		if err != nil {
			return err
		}
		newProjectKey = gittypes.ProjectKey{
			RepoKey: repoKey,
			SubDir:  path.Clean(obj.Spec.Source.Path),
		}
	} else {
		return fmt.Errorf("missing source spec")
	}
	if newProjectKey.SubDir == "." {
		newProjectKey.SubDir = ""
	}

	// we patch the projectKey immediately so that the webui knows it asap
	if obj.Status.ProjectKey == nil || *obj.Status.ProjectKey != newProjectKey {
		patchErr := r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			status.ProjectKey = &newProjectKey
			return nil
		})
		if patchErr != nil {
			return patchErr
		}
	}
	obj.Status.ProjectKey = &newProjectKey
	return nil
}

func (r *KluctlDeploymentReconciler) patchTargetKey(ctx context.Context, obj *kluctlv1.KluctlDeployment, targetContext *target_context.TargetContext) error {
	key := client.ObjectKeyFromObject(obj)

	clusterId, err := targetContext.SharedContext.K.GetClusterId()
	if err != nil {
		return err
	}
	newTargetKey := result.TargetKey{
		TargetName:    targetContext.Target.Name,
		Discriminator: targetContext.Target.Discriminator,
		ClusterId:     clusterId,
	}
	// we patch the targetKey immediately so that the webui knows it asap
	if obj.Status.TargetKey == nil || *obj.Status.TargetKey != newTargetKey {
		patchErr := r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
			status.TargetKey = &newTargetKey
			return nil
		})
		if patchErr != nil {
			return patchErr
		}
	}
	obj.Status.TargetKey = &newTargetKey
	return nil
}

func (r *KluctlDeploymentReconciler) patch(ctx context.Context, key client.ObjectKey, patchStatus bool, update func(obj *kluctlv1.KluctlDeployment) error) error {
	backoff := wait.Backoff{
		Steps:    5,
		Duration: 100 * time.Millisecond,
		Jitter:   1.0,
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		var latest kluctlv1.KluctlDeployment
		// we're not using r.ApiReader here because the patch+optimistic-lock already prevents us from updating an outdated object
		err := r.Client.Get(ctx, key, &latest)
		if err != nil {
			return false, err
		}

		patch := client.MergeFromWithOptions(latest.DeepCopy(), client.MergeFromWithOptimisticLock{})

		err = update(&latest)
		if err != nil {
			return false, err
		}

		if patchStatus {
			err = r.Client.Status().Patch(ctx, &latest, patch, client.FieldOwner(r.ControllerName))
		} else {
			err = r.Client.Patch(ctx, &latest, patch, client.FieldOwner(r.ControllerName))
		}
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

func (r *KluctlDeploymentReconciler) patchStatus(ctx context.Context, key client.ObjectKey, updateStatus func(status *kluctlv1.KluctlDeploymentStatus) error) error {
	return r.patch(ctx, key, true, func(obj *kluctlv1.KluctlDeployment) error {
		return updateStatus(&obj.Status)
	})
}

func (r *KluctlDeploymentReconciler) patchCondition(ctx context.Context, key client.ObjectKey, updateConditions func(c *[]metav1.Condition) error) error {
	return r.patchStatus(ctx, key, func(status *kluctlv1.KluctlDeploymentStatus) error {
		return updateConditions(&status.Conditions)
	})
}
