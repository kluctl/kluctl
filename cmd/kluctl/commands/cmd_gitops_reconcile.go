package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type gitopsReconcileCmd struct {
	args.GitOpsArgs
}

func (cmd *gitopsReconcileCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop. It does this by setting the annotation 'kluctl.io/request-reconcile' to the current time.`
}

func (cmd *gitopsReconcileCmd) Run(ctx context.Context) error {
	var g gitopsCmdHelper
	err := g.init(ctx, cmd.GitOpsArgs)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		v := time.Now().Format(time.RFC3339Nano)
		err := g.patchAnnotation(ctx, &kd, v1beta1.KluctlRequestReconcileAnnotation, v)
		if err != nil {
			return err
		}

		_, err = g.waitForRequestToFinish(ctx, client.ObjectKeyFromObject(&kd), v, func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult {
			return status.ReconcileRequestResult
		})
		if err != nil {
			return err
		}
	}
	return nil
}
