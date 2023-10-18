package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GitopsSuspendCmd struct {
	args.GitOpsArgs
	args.OutputFormatFlags
	args.GitOpsLogArgs

	All bool `group:"misc" help:"If enabled, suspend all deployments."`

	// we re-use the same code for "gitops resume" as well :)
	forResume bool
}

type gitopsResumeCmd struct {
	GitopsSuspendCmd
}

func (cmd *GitopsSuspendCmd) Help() string {
	if cmd.forResume {
		return `This command will resume a GitOps deployment by setting spec.suspend to 'false'.`
	} else {
		return `This command will suspend a GitOps deployment by setting spec.suspend to 'true'.`
	}
}

func (cmd *GitopsSuspendCmd) Run(ctx context.Context) error {
	g := gitopsCmdHelper{
		args:     cmd.GitOpsArgs,
		logsArgs: cmd.GitOpsLogArgs,
	}
	na := noArgsForbid
	if cmd.All {
		na = noArgsAllDeployments
	}
	err := g.init(ctx, na)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		err = g.patchDeployment(ctx, client.ObjectKeyFromObject(&kd), func(kd *v1beta1.KluctlDeployment) error {
			if cmd.forResume {
				kd.Spec.Suspend = false
			} else {
				kd.Spec.Suspend = true
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (cmd *gitopsResumeCmd) Run(ctx context.Context) error {
	cmd.forResume = true
	return cmd.GitopsSuspendCmd.Run(ctx)
}
