package commands

import (
	"context"
	"fmt"
	"os"
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
	noWait := cmd.KluctlDeploymentFlags.NoWait
	timestamp := time.Now().Format(time.RFC3339)

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
		if !cmd.VerifyFlags() {
			fmt.Println("KluctlDeployment name flag not provided..")
			os.Exit(1)
		}
		sourceName, sourceNamespace, _, err = k.GetObjectSource(ref)
		if err != nil {
			return err
		}
		ref2 := k8s2.ObjectRef{GVK: args.GitRepositoryGVK, Name: sourceName, Namespace: sourceNamespace}

		s := status.Start(cliCtx, "Annotating Source %s in %s namespace", sourceName, sourceNamespace)
		defer s.Failed()

		_, _, err = k.PatchObjectWithJsonPatch(ref2, patch, k8s.PatchOptions{})
		if err != nil {
			return err
		}
		s.Success()

		s = status.Start(cliCtx, "Waiting for Source %s to finish reconciliation", sourceName)

		if !noWait {
			ready, err := k.WaitForReady(ref2)
			if !ready {
				s.FailedWithMessage("Failed while waiting for Source %s to get ready..", sourceName)
				return err
			}
		}

		s.Success()
	}

	s := status.Start(cliCtx, "Annotating KluctlDeployment %s in %s namespace", kd, ns)
	defer s.Failed()

	_, _, err = k.PatchObjectWithJsonPatch(ref, patch, k8s.PatchOptions{})
	if err != nil {
		return err
	}
	s.Success()

	s = status.Start(cliCtx, "Waiting for KluctlDeployment %s in %s namespace to finish reconciliation", kd, ns)

	if !noWait {
		ready, err := k.WaitForReady(ref)
		if !ready {
			s.FailedWithMessage("Failed while waiting for KluctlDeployment %s to get ready..", kd)
			return err
		}
	}

	s.Success()

	return err
}
