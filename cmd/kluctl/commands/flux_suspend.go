package commands

import (
	"context"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type fluxSuspendCmd struct {
	args.KluctlDeploymentFlags
}

func (cmd *fluxSuspendCmd) Run() error {
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment

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
		Op:    "replace",
		Path:  "/spec/suspend",
		Value: true,
	}}
	_, _, err = k.PatchObjectWithJsonPatch(ref, patch, k8s.PatchOptions{})
	if err != nil {
		return err
	}

	// client := flux.CreateClient()

	// payload := []patchSuspend{{
	// 	Op:    "replace",
	// 	Path:  "/spec/suspend",
	// 	Value: true,
	// }}

	// fmt.Printf("► Suspending KluctlDeployment %s in %s namespace \n", kd, ns)
	// err := flux.Patch(client, ns, kd, args.KluctlDeployment, payload)
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Println(("✔ KluctlDeployment suspended"))

	return err
}
