package commands

import (
	"fmt"
	"time"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/flux"
)

type fluxReconcileCmd struct {
	args.KluctlDeploymentFlags
}

type patchAnnotation struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

func (cmd *fluxReconcileCmd) Run() error {
	var (
		sourceNamespace string
		sourceName      string
	)
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment
	source := cmd.KluctlDeploymentFlags.WithSource
	client := flux.CreateClient()

	timestamp := fmt.Sprintf("%d", time.Now().Unix())

	payload := []patchAnnotation{{
		Op:   "replace",
		Path: "/metadata/annotations",
		Value: map[string]string{
			"fluxcd.io/reconcileAt": timestamp,
		},
	}}

	if source {
		objectFields, err := flux.GetObject(client, ns, kd, args.KluctlDeployment)
		if err != nil {
			panic(err.Error())
		}
		sourceName, sourceNamespace = flux.GetSource(objectFields)
		fmt.Printf("► annotating Source %s in %s namespace \n", sourceName, sourceNamespace)
		err = flux.Patch(client, sourceNamespace, sourceName, args.GitRepository, payload)
		if err != nil {
			flux.HandleError(err, kd, ns)
		}
	}

	fmt.Printf("► annotating KluctlDeployment %s in %s namespace \n", kd, ns)
	err := flux.Patch(client, ns, kd, args.KluctlDeployment, payload)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(("✔ KluctlDeployment annotated"))

	return err
}
