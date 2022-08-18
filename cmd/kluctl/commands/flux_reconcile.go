package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type fluxReconcileCmd struct {
	args.KluctlDeploymentFlags
}

func (cmd *fluxReconcileCmd) Run() error {
	// var (
	// 	sourceNamespace string
	// 	sourceName      string
	// )
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment
	// source := cmd.KluctlDeploymentFlags.WithSource
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
		Path: "/spec/suspend",
		Value: map[string]string{
			"fluxcd.io/reconcileAt": timestamp,
		},
	}}

	_, _, err = k.PatchObjectWithJsonPatch(ref, patch, k8s.PatchOptions{})
	if err != nil {
		return err
	}

	// client := flux.CreateClient()

	// payload := []patchAnnotation{{
	// 	Op:   "replace",
	// 	Path: "/metadata/annotations",
	// 	Value: map[string]string{
	// 		"fluxcd.io/reconcileAt": timestamp,
	// 	},
	// }}

	// if source {
	// 	objectFields, err := flux.GetObject(client, ns, kd, args.KluctlDeployment)
	// 	if err != nil {
	// 		panic(err.Error())
	// 	}
	// 	sourceName, sourceNamespace = flux.GetSource(objectFields)
	// 	fmt.Printf("► annotating Source %s in %s namespace \n", sourceName, sourceNamespace)
	// 	err = flux.Patch(client, sourceNamespace, sourceName, args.GitRepository, payload)
	// 	if err != nil {
	// 		flux.HandleError(err, kd, ns)
	// 	}
	// }

	// fmt.Printf("► annotating KluctlDeployment %s in %s namespace \n", kd, ns)
	// err := flux.Patch(client, ns, kd, args.KluctlDeploymentGVK, payload)
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Println(("✔ KluctlDeployment annotated"))

	return err
}
