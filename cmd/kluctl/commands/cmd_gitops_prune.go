package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"time"
)

type gitopsPruneCmd struct {
	args.GitOpsArgs
}

func (cmd *gitopsPruneCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced prune. It does this by setting the annotation 'kluctl.io/request-prune' to the current time.`
}

func (cmd *gitopsPruneCmd) Run(ctx context.Context) error {
	var g gitopsCmdHelper
	err := g.init(ctx, cmd.GitOpsArgs)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		v := time.Now().Format(time.RFC3339Nano)
		err := g.patchAnnotation(ctx, &kd, v1beta1.KluctlRequestPruneAnnotation, v)
		if err != nil {
			return err
		}
	}
	return nil
}
