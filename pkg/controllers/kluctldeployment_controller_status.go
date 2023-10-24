package controllers

import (
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *KluctlDeploymentReconciler) patchProjectKey(ctx context.Context, obj *kluctlv1.KluctlDeployment) error {
	key := client.ObjectKeyFromObject(obj)

	var newProjectKey result.ProjectKey
	if obj.Spec.Source.Git != nil {
		newProjectKey = result.ProjectKey{
			GitRepoKey: obj.Spec.Source.Git.URL.RepoKey(),
			SubDir:     path.Clean(obj.Spec.Source.Git.Path),
		}
	} else if obj.Spec.Source.Oci != nil {
		repoKey, err := types.NewRepoKeyFromUrl(obj.Spec.Source.Oci.URL)
		if err != nil {
			return err
		}
		newProjectKey = result.ProjectKey{
			OciRepoKey: repoKey,
			SubDir:     path.Clean(obj.Spec.Source.Oci.Path),
		}
	} else if obj.Spec.Source.URL != nil {
		newProjectKey = result.ProjectKey{
			GitRepoKey: obj.Spec.Source.URL.RepoKey(),
			SubDir:     path.Clean(obj.Spec.Source.Path),
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
