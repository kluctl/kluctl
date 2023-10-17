package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"time"
)

type gitopsDeployCmd struct {
	args.GitOpsArgs
}

func (cmd *gitopsDeployCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced deployment. It does this by setting the annotation 'kluctl.io/request-deploy' to the current time.`
}

func (cmd *gitopsDeployCmd) Run(ctx context.Context) error {
	var g gitopsCmdHelper
	err := g.init(ctx, cmd.GitOpsArgs)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		v := time.Now().Format(time.RFC3339Nano)
		err := g.patchAnnotation(ctx, &kd, v1beta1.KluctlRequestDeployAnnotation, v)
		if err != nil {
			return err
		}
	}
	return nil
}
