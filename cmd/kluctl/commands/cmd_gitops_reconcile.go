package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type gitopsReconcileCmd struct {
	args.GitOpsArgs
	args.GitOpsLogArgs
	args.GitOpsOverridableArgs
}

func (cmd *gitopsReconcileCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop.
It does this by setting the annotation 'kluctl.io/request-reconcile' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.`
}

func (cmd *gitopsReconcileCmd) Run(ctx context.Context) error {
	g := gitopsCmdHelper{
		args:            cmd.GitOpsArgs,
		logsArgs:        cmd.GitOpsLogArgs,
		overridableArgs: cmd.GitOpsOverridableArgs,
	}
	err := g.init(ctx)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		v := time.Now().Format(time.RFC3339Nano)
		err := g.patchManualRequest(ctx, client.ObjectKeyFromObject(&kd), v1beta1.KluctlRequestReconcileAnnotation, v)
		if err != nil {
			return err
		}

		rr, err := g.waitForRequestToStartAndFinish(ctx, client.ObjectKeyFromObject(&kd), v, func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult {
			return status.ReconcileRequestResult
		})
		if err != nil {
			return err
		}
		if rr.CommandError != "" {
			return fmt.Errorf("%s", rr.CommandError)
		}
	}
	return nil
}
