package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type gitopsDeployCmd struct {
	args.GitOpsArgs
	args.OutputFormatFlags
	args.GitOpsLogArgs
	args.GitOpsOverridableArgs `groupOverride:"override"`

	DeployExtraFlags
}

func (cmd *gitopsDeployCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced deployment.
It does this by setting the annotation 'kluctl.io/request-deploy' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.`
}

func (cmd *gitopsDeployCmd) Run(ctx context.Context) error {
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
		err := g.patchManualRequest(ctx, client.ObjectKeyFromObject(&kd), v1beta1.KluctlRequestDeployAnnotation, v)
		if err != nil {
			return err
		}

		rr, err := g.waitForRequestToStartAndFinish(ctx, client.ObjectKeyFromObject(&kd), v, func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.RequestResult {
			return status.DeployRequestResult
		})
		if err != nil {
			return err
		}

		if g.resultStore != nil && rr != nil && rr.ResultId != "" {
			cmdResult, err := g.resultStore.GetCommandResult(results.GetCommandResultOptions{Id: rr.ResultId, Reduced: true})
			if err != nil {
				return err
			}
			err = outputCommandResult2(ctx, cmd.OutputFormatFlags, cmdResult)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
