package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
)

type deployCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.YesFlags
	args.DryRunFlags
	args.ForceApplyFlags
	args.ReplaceOnErrorFlags
	args.AbortOnErrorFlags
	args.HookFlags
	args.OutputFlags
	args.RenderOutputDirFlags
}

func (cmd *deployCmd) Help() string {
	return `This command will also output a diff between the initial state and the state after
deployment. The format of this diff is the same as for the 'diff' command.
It will also output a list of prunable objects (without actually deleting them).
`
}

func (cmd *deployCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		return cmd.runCmdDeploy(ctx)
	})
}

func (cmd *deployCmd) runCmdDeploy(ctx *commandCtx) error {
	if !cmd.Yes && !cmd.DryRun {
		if !AskForConfirmation(fmt.Sprintf("Do you really want to deploy to the context/cluster %s?", ctx.k.Context())) {
			return fmt.Errorf("aborted")
		}
	}

	result, err := ctx.deploymentCollection.Deploy(ctx.k, cmd.ForceApply, cmd.ReplaceOnError, cmd.ForceReplaceOnError, cmd.AbortOnError, cmd.HookTimeout)
	if err != nil {
		return err
	}
	err = outputCommandResult(cmd.Output, result)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}
