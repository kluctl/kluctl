package commands

import (
	"context"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type fluxSuspendCmd struct {
	args.KluctlDeploymentFlags
}

func (cmd *fluxSuspendCmd) Run(ctx context.Context) error {
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment

	cf, err := k8s.NewClientFactoryFromDefaultConfig(ctx, nil)
	if err != nil {
		return err
	}
	k, err := k8s.NewK8sCluster(context.TODO(), cf, false)
	if err != nil {
		return err
	}

	ref := k8s2.ObjectRef{GVK: args.KluctlDeploymentGVK, Name: kd, Namespace: ns}
	patch := []k8s.JsonPatch{{
		Op:    "replace",
		Path:  "/spec/suspend",
		Value: true,
	}}

	s := status.Start(ctx, "Suspending KluctlDeployment %s in %s namespace", kd, ns)
	defer s.Failed()

	_, _, err = k.PatchObjectWithJsonPatch(ref, patch, k8s.PatchOptions{})
	if err != nil {
		return err
	}

	s.Success()

	return err
}
