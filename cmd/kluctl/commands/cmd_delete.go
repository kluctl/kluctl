package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type deleteCmd struct {
	args.ProjectFlags
	args.KubeconfigFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.GitCredentials
	args.HelmCredentials
	args.RegistryCredentials
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
	args.CommandResultFlags

	Discriminator string `group:"misc" help:"Override the discriminator used to find objects for deletion."`

	NoWait bool `group:"misc" help:"Don't wait for deletion of objects to finish.'"`
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
		kubeconfigFlags:      cmd.KubeconfigFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		gitCredentials:       cmd.GitCredentials,
		helmCredentials:      cmd.HelmCredentials,
		registryCredentials:  cmd.RegistryCredentials,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		commandResultFlags:   &cmd.CommandResultFlags,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		cmd2 := commands.NewDeleteCommand(cmd.Discriminator, cmdCtx.targetCtx, nil, !cmd.NoWait)

		result := cmd2.Run(cmdCtx.targetCtx.SharedContext.Ctx, cmdCtx.targetCtx.SharedContext.K, func(refs []k8s2.ObjectRef) error {
			return confirmDeletion(ctx, refs, cmd.DryRun, cmd.Yes)
		})

		err := outputCommandResult(cmdCtx, cmd.OutputFormatFlags, result, !cmd.DryRun || cmd.ForceWriteCommandResult)
		if err != nil {
			return err
		}
		if len(result.Errors) != 0 {
			return fmt.Errorf("command failed")
		}
		return nil
	})
}

func confirmDeletion(ctx context.Context, refs []k8s2.ObjectRef, dryRun bool, forceYes bool) error {
	if len(refs) != 0 {
		_, _ = getStderr(ctx).WriteString("The following objects will be deleted:\n")
		for _, ref := range refs {
			_, _ = getStderr(ctx).WriteString(fmt.Sprintf("  %s\n", ref.String()))
		}
		if !forceYes && !dryRun {
			if !prompts.AskForConfirmation(ctx, fmt.Sprintf("Do you really want to delete %d objects?", len(refs))) {
				return fmt.Errorf("aborted")
			}
		}
	}
	return nil
}
