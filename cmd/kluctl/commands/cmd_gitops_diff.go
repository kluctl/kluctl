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

type gitopsDiffCmd struct {
	args.GitOpsArgs
	args.OutputFormatFlags
	args.GitOpsLogArgs
	args.GitOpsOverridableArgs `groupOverride:"override"`
}

func (cmd *gitopsDiffCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced diff.
It does this by setting the annotation 'kluctl.io/request-diff' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.`
}

func (cmd *gitopsDiffCmd) Run(ctx context.Context) error {
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
		err := g.patchManualRequest(ctx, client.ObjectKeyFromObject(&kd), v1beta1.KluctlRequestDiffAnnotation, v)
		if err != nil {
			return err
		}

		rr, err := g.waitForRequestToStartAndFinish(ctx, client.ObjectKeyFromObject(&kd), v, func(status *v1beta1.KluctlDeploymentStatus) *v1beta1.ManualRequestResult {
			return status.DiffRequestResult
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

			err = cmd.cleanupDiffResult(ctx, &g, &kd, rr)
			if err != nil {
				status.Warningf(ctx, "Failed to cleanup diff result: %s", err.Error())
			}
		}
		if rr.CommandError != "" {
			return fmt.Errorf("%s", rr.CommandError)
		}
	}
	return nil
}

func (cmd *gitopsDiffCmd) cleanupDiffResult(ctx context.Context, g *gitopsCmdHelper, kd *v1beta1.KluctlDeployment, rr *v1beta1.ManualRequestResult) error {
	flags := args.CommandResultFlags{
		CommandResultReadOnlyFlags: cmd.CommandResultReadOnlyFlags,
		CommandResultWriteFlags: args.CommandResultWriteFlags{
			WriteCommandResult: true,
		},
	}
	rwRS, err := buildResultStoreRW(ctx, g.restConfig, g.restMapper, &flags, false)
	if err != nil {
		return err
	}

	err = rwRS.DeleteCommandResult(rr.ResultId)
	if err != nil {
		return err
	}

	err = g.updateDeploymentStatus(ctx, client.ObjectKeyFromObject(kd), func(kd *v1beta1.KluctlDeployment) error {
		if kd.Status.DiffRequestResult == nil || kd.Status.DiffRequestResult.Request.RequestValue != rr.Request.RequestValue {
			return nil
		}
		kd.Status.LastDiffResult = nil
		return nil
	}, 5)
	if err != nil {
		return err
	}
	return nil
}
