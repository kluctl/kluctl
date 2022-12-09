package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
)

type pruneCmd struct {
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
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		return cmd.runCmdPrune(cmdCtx)
	})
}

func (cmd *pruneCmd) runCmdPrune(cmdCtx *commandCtx) error {
	cmd2 := commands.NewPruneCommand(cmdCtx.targetCtx.DeploymentCollection)
	objects, err := cmd2.Run(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K)
	if err != nil {
		return err
	}
	result, err := confirmedDeleteObjects(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, objects, cmd.DryRun, cmd.Yes)
	if err != nil {
		return err
	}
	err = outputCommandResult(cmdCtx.ctx, cmd.OutputFormat, result)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}
