package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type deployCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.HelmCredentials
	args.YesFlags
	args.DryRunFlags
	args.ForceApplyFlags
	args.ReplaceOnErrorFlags
	args.AbortOnErrorFlags
	args.HookFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
	args.CommandResultFlags

	NoWait bool `group:"misc" help:"Don't wait for objects readiness'"`

	internal bool
}

func (cmd *deployCmd) Help() string {
	return `This command will also output a diff between the initial state and the state after
deployment. The format of this diff is the same as for the 'diff' command.
It will also output a list of prunable objects (without actually deleting them).
`
}

func (cmd *deployCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		commandResultFlags:   &cmd.CommandResultFlags,
		internalDeploy:       cmd.internal,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		return cmd.runCmdDeploy(cmdCtx)
	})
}

func (cmd *deployCmd) runCmdDeploy(cmdCtx *commandCtx) error {
	status.Trace(cmdCtx.ctx, "enter runCmdDeploy")
	defer status.Trace(cmdCtx.ctx, "leave runCmdDeploy")

	cmd2 := commands.NewDeployCommand(cmdCtx.targetCtx)
	cmd2.ForceApply = cmd.ForceApply
	cmd2.ReplaceOnError = cmd.ReplaceOnError
	cmd2.ForceReplaceOnError = cmd.ForceReplaceOnError
	cmd2.AbortOnError = cmd.AbortOnError
	cmd2.ReadinessTimeout = cmd.ReadinessTimeout
	cmd2.NoWait = cmd.NoWait

	cb := func(diffResult *result.CommandResult) error {
		return cmd.diffResultCb(cmdCtx, diffResult)
	}
	if cmd.Yes || cmd.DryRun {
		cb = nil
	}

	result, err := cmd2.Run(cb)
	if err != nil {
		return err
	}
	err = outputCommandResult(cmdCtx, cmd.OutputFormatFlags, result, !cmd.DryRun || cmd.ForceWriteCommandResult)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}

func (cmd *deployCmd) diffResultCb(ctx *commandCtx, diffResult *result.CommandResult) error {
	flags := cmd.OutputFormatFlags
	flags.OutputFormat = nil // use default output format

	err := outputCommandResult(ctx, flags, diffResult, false)
	if err != nil {
		return err
	}
	if cmd.Yes || cmd.DryRun {
		return nil
	}
	if len(diffResult.Errors) != 0 {
		if !status.AskForConfirmation(ctx.ctx, "The diff resulted in errors, do you still want to proceed?") {
			return fmt.Errorf("aborted")
		}
	} else {
		if !status.AskForConfirmation(ctx.ctx, "The diff succeeded, do you want to proceed?") {
			return fmt.Errorf("aborted")
		}
	}
	return nil
}
