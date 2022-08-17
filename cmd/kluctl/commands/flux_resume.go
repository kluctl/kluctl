package commands

import (
	"fmt"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/flux"
)

type fluxResumeCmd struct {
	args.KluctlDeploymentFlags
}

type patchResume struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value bool   `json:"value"`
}

// TODO add reoncilation after resume
func (cmd *fluxResumeCmd) Run() error {
	ns := cmd.KluctlDeploymentFlags.Namespace
	kd := cmd.KluctlDeploymentFlags.KluctlDeployment
	client := flux.CreateClient()

	payload := []patchSuspend{{
		Op:    "replace",
		Path:  "/spec/suspend",
		Value: false,
	}}

	fmt.Printf("► Resuming KluctlDeployment %s in %s namespace \n", kd, ns)
	err := flux.Patch(client, ns, kd, args.KluctlDeployment, payload)
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(("✔ KluctlDeployment resumed"))
	return err
}
