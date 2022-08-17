package commands

import (
	"fmt"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/flux"
)

type fluxSuspendCmd struct {
	args.KluctlDeploymentFlags
}

type patchSuspend struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

func (cmd *fluxSuspendCmd) Run() error {
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment
	client := flux.CreateClient()

	payload := []patchSuspend{{
		Op:    "replace",
		Path:  "/spec/suspend",
		Value: true,
	}}

	fmt.Printf("► Suspending KluctlDeployment %s in %s namespace \n", kd, ns)
	err := flux.Patch(client, ns, kd, args.KluctlDeployment, payload)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(("✔ KluctlDeployment suspended"))

	return err
}
