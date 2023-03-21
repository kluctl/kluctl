package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type deleteCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.HelmCredentials
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags

	Discriminator string `group:"misc" help:"Override the discriminator used to find objects for deletion."`
}

func (cmd *deleteCmd) Help() string {
	return `Objects are located based on the target discriminator.

WARNING: This command will also delete objects which are not part of your deployment
project (anymore). It really only decides based on the discriminator and does NOT
take the local target/state into account!`
}

func (cmd *deleteCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		discriminator := cmdCtx.targetCtx.Target.Discriminator
		if cmd.Discriminator != "" {
			discriminator = cmd.Discriminator
		}
		cmd2 := commands.NewDeleteCommand(discriminator, cmdCtx.targetCtx.DeploymentCollection.Inclusion)

		objects, err := cmd2.Run(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K)
		if err != nil {
			return err
		}
		result, err := confirmedDeleteObjects(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, objects, cmd.DryRun, cmd.Yes)
		if err != nil {
			return err
		}
		err = addCommandInfo(result, "delete", cmdCtx, &cmd.TargetFlags, &cmd.ImageFlags, &cmd.InclusionFlags, &cmd.DryRunFlags, nil, nil, nil, false)
		if err != nil {
			return err
		}
		err = outputCommandResult(ctx, cmd.OutputFormatFlags, result)
		if err != nil {
			return err
		}
		if len(result.Errors) != 0 {
			return fmt.Errorf("command failed")
		}
		return nil
	})
}

func confirmedDeleteObjects(ctx context.Context, k *k8s.K8sCluster, refs []k8s2.ObjectRef, dryRun bool, forceYes bool) (*result.CommandResult, error) {
	if len(refs) != 0 {
		_, _ = getStderr(ctx).WriteString("The following objects will be deleted:\n")
		for _, ref := range refs {
			_, _ = getStderr(ctx).WriteString(fmt.Sprintf("  %s\n", ref.String()))
		}
		if !forceYes && !dryRun {
			if !status.AskForConfirmation(ctx, fmt.Sprintf("Do you really want to delete %d objects?", len(refs))) {
				return nil, fmt.Errorf("aborted")
			}
		}
	}

	return utils.DeleteObjects(ctx, k, refs, true)
}
