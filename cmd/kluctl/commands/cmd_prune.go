package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type pruneCmd struct {
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

	Discriminator string `group:"misc" help:"Override the target discriminator."`
}

func (cmd *pruneCmd) Help() string {
	return `"Searching works by:

  1. Search the cluster for all objects match 'commonLabels', as configured in 'deployment.yaml'
  2. Render the local target and list all objects.
  3. Remove all objects from the list of 1. that are part of the list in 2.`
}

func (cmd *pruneCmd) Run(ctx context.Context) error {
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
		discriminator:        cmd.Discriminator,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		return cmd.runCmdPrune(ctx, cmdCtx)
	})
}

func (cmd *pruneCmd) runCmdPrune(ctx context.Context, cmdCtx *commandCtx) error {
	cmd2 := commands.NewPruneCommand(cmdCtx.targetCtx.Target.Discriminator, cmdCtx.targetCtx, true)
	result := cmd2.Run(func(refs []k8s2.ObjectRef) error {
		return confirmDeletion(ctx, refs, cmd.DryRun, cmd.Yes)
	})
	err := outputCommandResult(ctx, cmdCtx, cmd.OutputFormatFlags, result, !cmd.DryRun || cmd.ForceWriteCommandResult)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}
