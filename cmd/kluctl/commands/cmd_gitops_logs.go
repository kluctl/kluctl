package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type gitopsLogsCmd struct {
	args.GitOpsArgs
	args.GitOpsLogArgs

	ReconcileId string `group:"misc" help:"If specified, logs are filtered for the given reconcile ID."`
	Follow      bool   `group:"misc" short:"f" help:"Follow logs after printing old logs."`
}

func (cmd *gitopsLogsCmd) Help() string {
	return `Print and watch logs of specified KluctlDeployments from the kluctl-controller.`
}

func (cmd *gitopsLogsCmd) Run(ctx context.Context) error {
	g := gitopsCmdHelper{
		args:        cmd.GitOpsArgs,
		logsArgs:    cmd.GitOpsLogArgs,
		noArgsReact: noArgsNoDeployments,
	}
	err := g.init(ctx)
	if err != nil {
		return err
	}

	stopCh := make(chan struct{})

	gh := utils.NewGoHelper(ctx, 0)
	if !cmd.GitOpsArgs.AnyObjectArgSet() {
		gh.RunE(func() error {
			return g.watchLogs(ctx, stopCh, client.ObjectKey{}, cmd.Follow, cmd.ReconcileId)
		})
	} else {
		for _, kd := range g.kds {
			key := client.ObjectKeyFromObject(&kd)
			gh.RunE(func() error {
				return g.watchLogs(ctx, stopCh, key, cmd.Follow, cmd.ReconcileId)
			})
		}
	}
	gh.Wait()

	return gh.ErrorOrNil()
}
