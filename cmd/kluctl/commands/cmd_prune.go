package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
)

type pruneCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
}

func (cmd *pruneCmd) Help() string {
	return `"Searching works by:

  1. Search the cluster for all objects match 'deleteByLabels', as configured in 'deployment.yml''
  2. Render the local target and list all objects.
  3. Remove all objects from the list of 1. that are part of the list in 2.`
}

func (cmd *pruneCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:   cmd.ProjectFlags,
		targetFlags:    cmd.TargetFlags,
		argsFlags:      cmd.ArgsFlags,
		imageFlags:     cmd.ImageFlags,
		inclusionFlags: cmd.InclusionFlags,
		dryRunArgs:     &cmd.DryRunFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		return cmd.runCmdPrune(ctx)
	})
}

func (cmd *pruneCmd) runCmdPrune(ctx *commandCtx) error {
	objects, err := ctx.deploymentCollection.FindOrphanObjects(ctx.k)
	if err != nil {
		return err
	}
	result, err := confirmedDeleteObjects(ctx.k, objects, cmd.DryRun, cmd.Yes)
	if err != nil {
		return err
	}
	err = outputCommandResult(cmd.OutputFormat, result)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}
