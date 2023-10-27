package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type gitopsValidateCmd struct {
	args.GitOpsArgs
	args.GitOpsLogArgs
	args.GitOpsOverridableArgs
	args.OutputFlags

	WarningsAsErrors bool `group:"misc" help:"Consider warnings as failures"`
}

func (cmd *gitopsValidateCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced validation.
It does this by setting the annotation 'kluctl.io/request-validate' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.`
}

func (cmd *gitopsValidateCmd) Run(ctx context.Context) error {
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
		err := g.patchManualRequest(ctx, client.ObjectKeyFromObject(&kd), v1beta1.KluctlRequestValidateAnnotation, v)
		if err != nil {
			return err
		}

		rr, err := g.waitForRequestToStartAndFinish(ctx, client.ObjectKeyFromObject(&kd), v, func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.ManualRequestResult {
			return status.ValidateRequestResult
		})
		if err != nil {
			return err
		}
		if g.resultStore != nil && rr != nil && rr.ResultId != "" {
			cmdResult, err := g.resultStore.GetValidateResult(results.GetValidateResultOptions{Id: rr.ResultId})
			if err != nil {
				return err
			}
			err = outputValidateResult2(ctx, cmd.Output, cmdResult)
			if err != nil {
				return err
			}
			failed := len(cmdResult.Errors) != 0 || (cmd.WarningsAsErrors && len(cmdResult.Warnings) != 0)
			if failed {
				return fmt.Errorf("Validation failed")
			} else {
				status.Info(ctx, "Validation succeeded")
			}
		} else {
			status.Info(ctx, "No validation result was returned.")
		}
		if rr.CommandError != "" {
			return fmt.Errorf("%s", rr.CommandError)
		}
	}
	return nil
}
