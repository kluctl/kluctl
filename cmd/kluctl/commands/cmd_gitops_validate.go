package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"time"
)

type gitopsValidateCmd struct {
	args.GitOpsArgs
}

func (cmd *gitopsValidateCmd) Help() string {
	return `This command will trigger an existing KluctlDeployment to perform a reconciliation loop. It does this by setting the annotation 'kluctl.io/request-validate' to the current time.`
}

func (cmd *gitopsValidateCmd) Run(ctx context.Context) error {
	var g gitopsCmdHelper
	err := g.init(ctx, cmd.GitOpsArgs)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		v := time.Now().Format(time.RFC3339Nano)
		err := g.patchAnnotation(ctx, &kd, v1beta1.KluctlRequestValidateAnnotation, v)
		if err != nil {
			return err
		}
	}
	return nil
}
