package commands

import (
	"context"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
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
		args:        cmd.GitOpsArgs,
		logsArgs:    cmd.GitOpsLogArgs,
		noArgsReact: noArgsAutoDetectProjectAsk,
	}
	if cmd.All {
		g.noArgsReact = noArgsAllDeployments
	}
	err := g.init(ctx)
	if err != nil {
		return err
	}
	for _, kd := range g.kds {
		patchedKd, err := g.patchDeployment(ctx, client.ObjectKeyFromObject(&kd), func(kd *v1beta1.KluctlDeployment) error {
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
		err = func() error {
			// modifying the spec causes a reconciliation loop and we should really wait for it to finish before we consider
			// suspension to be done (otherwise you'd be surprised for some last-second deployments...)
			st := status.Startf(ctx, "Waiting for final reconciliation to finish")
			defer st.Failed()

			tick := time.NewTicker(time.Second)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-tick.C:
					var kd2 v1beta1.KluctlDeployment
					err := g.client.Get(ctx, client.ObjectKeyFromObject(&kd), &kd2)
					if err != nil {
						return err
					}
					if kd2.Status.ObservedGeneration >= patchedKd.Generation {
						st.Success()
						return nil
					}
				}
			}
		}()
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
