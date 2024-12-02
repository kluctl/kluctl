package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type deployCmd struct {
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
	args.ForceApplyFlags
	args.ReplaceOnErrorFlags
	args.AbortOnErrorFlags
	args.HookFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
	args.CommandResultFlags

	DeployExtraFlags

	Discriminator string `group:"misc" help:"Override the target discriminator."`

	internal bool
}

type DeployExtraFlags struct {
	NoWait bool `group:"misc" help:"Don't wait for objects readiness."`
	Prune  bool `group:"misc" help:"Prune orphaned objects directly after deploying. See the help for the 'prune' sub-command for details."`
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
		internalDeploy:       cmd.internal,
		discriminator:        cmd.Discriminator,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		return cmd.runCmdDeploy(ctx, cmdCtx)
	})
}

func (cmd *deployCmd) runCmdDeploy(ctx context.Context, cmdCtx *commandCtx) error {
	status.Trace(ctx, "enter runCmdDeploy")
	defer status.Trace(ctx, "leave runCmdDeploy")

	cmd2 := commands.NewDeployCommand(cmdCtx.targetCtx)
	cmd2.ForceApply = cmd.ForceApply
	cmd2.ReplaceOnError = cmd.ReplaceOnError
	cmd2.ForceReplaceOnError = cmd.ForceReplaceOnError
	cmd2.AbortOnError = cmd.AbortOnError
	cmd2.ReadinessTimeout = cmd.ReadinessTimeout
	cmd2.NoWait = cmd.NoWait
	cmd2.Prune = cmd.Prune
	cmd2.WaitPrune = !cmd.NoWait

	cb := func(diffResult *result.CommandResult) error {
		return cmd.diffResultCb(ctx, cmdCtx, diffResult)
	}
	if cmd.Yes || cmd.DryRun {
		cb = nil
	}

	result := cmd2.Run(cb)
	err := outputCommandResult(ctx, cmdCtx, cmd.OutputFormatFlags, result, !cmd.DryRun || cmd.ForceWriteCommandResult)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}

func (cmd *deployCmd) diffResultCb(ctx context.Context, cmdCtx *commandCtx, diffResult *result.CommandResult) error {
	flags := cmd.OutputFormatFlags
	flags.OutputFormat = nil // use default output format

	err := outputCommandResult(ctx, cmdCtx, flags, diffResult, false)
	if err != nil {
		return err
	}
	if cmd.Yes || cmd.DryRun {
		return nil
	}
	if len(diffResult.Errors) != 0 {
		if !prompts.AskForConfirmation(ctx, "The diff resulted in errors, do you still want to proceed?") {
			return fmt.Errorf("aborted")
		}
	} else {
		if !prompts.AskForConfirmation(ctx, "The diff succeeded, do you want to proceed?") {
			return fmt.Errorf("aborted")
		}
	}
	return nil
}
