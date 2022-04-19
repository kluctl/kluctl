package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/cmd/kluctl/args"
	"github.com/kluctl/kluctl/pkg/deployment/commands"
	"github.com/kluctl/kluctl/pkg/types"
	"github.com/kluctl/kluctl/pkg/utils"
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

	NoWait bool `group:"misc" help:"Don't wait for objects readiness'"`
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
	cmd2 := commands.NewDeployCommand(ctx.targetCtx.DeploymentCollection)
	cmd2.ForceApply = cmd.ForceApply
	cmd2.ReplaceOnError = cmd.ReplaceOnError
	cmd2.ForceReplaceOnError = cmd.ForceReplaceOnError
	cmd2.AbortOnError = cmd.AbortOnError
	cmd2.HookTimeout = cmd.HookTimeout
	cmd2.NoWait = cmd.NoWait

	cb := cmd.diffResultCb
	if cmd.Yes || cmd.DryRun {
		cb = nil
	}

	result, err := cmd2.Run(ctx.targetCtx.K, cb)
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

func (cmd *deployCmd) diffResultCb(diffResult *types.CommandResult) error {
	err := outputCommandResult(nil, diffResult)
	if err != nil {
		return err
	}
	if cmd.Yes || cmd.DryRun {
		return nil
	}
	if len(diffResult.Errors) != 0 {
		if !utils.AskForConfirmation("\nThe diff resulted in errors, do you still want to proceed?") {
			return fmt.Errorf("aborted")
		}
	} else {
		if !utils.AskForConfirmation("\nThe diff succeeded, do you want to proceed?") {
			return fmt.Errorf("aborted")
		}
	}
	return nil
}
