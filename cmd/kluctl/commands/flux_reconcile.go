package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type fluxReconcileCmd struct {
	args.KluctlDeploymentFlags
}

func (cmd *fluxReconcileCmd) Run() error {
	var (
		sourceNamespace string
		sourceName      string
	)
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment
	source := cmd.KluctlDeploymentFlags.WithSource
	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	cf, err := k8s.NewClientFactoryFromDefaultConfig(nil)
	if err != nil {
		return err
	}
	k, err := k8s.NewK8sCluster(context.TODO(), cf, false)
	if err != nil {
		return err
	}

	ref := k8s2.ObjectRef{GVK: args.KluctlDeploymentGVK, Name: kd, Namespace: ns}
	patch := []k8s.JsonPatch{{
		Op:   "replace",
		Path: "/metadata/annotations",
		Value: map[string]string{
			"fluxcd.io/reconcileAt": timestamp,
		},
	}}

	if source {
		sourceName, sourceNamespace, _, err = k.GetObjectSource(ref)
		if err != nil {
			return err
		}
		ref2 := k8s2.ObjectRef{GVK: args.GitRepositoryGVK, Name: sourceName, Namespace: sourceNamespace}

		s := status.Start(cliCtx, "► annotating Source %s in %s namespace", sourceName, sourceNamespace)
		defer s.Failed()

		_, _, err = k.PatchObjectWithJsonPatch(ref2, patch, k8s.PatchOptions{})
		if err != nil {
			return err
		}
		ready, err := k.WaitForReady(ref2)
		if !ready {
			s.FailedWithMessage("Failed while waiting for Source %s to get ready..", sourceName)
			return err
		}

		s.Success()
	}

	s := status.Start(cliCtx, "► annotating KluctlDeployment %s in %s namespace", kd, ns)
	defer s.Failed()

	_, _, err = k.PatchObjectWithJsonPatch(ref, patch, k8s.PatchOptions{})
	if err != nil {
		return err
	}

	ready, err := k.WaitForReady(ref)
	if !ready {
		s.FailedWithMessage("Failed while waiting for KluctlDeployment %s to get ready..", kd)
		return err
	}

	s.Success()

	return err
}
