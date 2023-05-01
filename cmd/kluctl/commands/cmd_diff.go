package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
)

type diffCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
	args.ImageFlags
	args.HelmCredentials
	args.ForceApplyFlags
	args.ReplaceOnErrorFlags
	args.IgnoreFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
}

func (cmd *diffCmd) Help() string {
	return `The output is by default in human readable form (a table combined with unified diffs).
The output can also be changed to output a yaml file. Please note however that the format
is currently not documented and prone to changes.
After the diff is performed, the command will also search for prunable objects and list them.`
}

func (cmd *diffCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		cmd2 := commands.NewDiffCommand(cmdCtx.targetCtx.Target.Discriminator, cmdCtx.targetCtx.DeploymentCollection)
		cmd2.ForceApply = cmd.ForceApply
		cmd2.ReplaceOnError = cmd.ReplaceOnError
		cmd2.ForceReplaceOnError = cmd.ForceReplaceOnError
		cmd2.IgnoreTags = cmd.IgnoreTags
		cmd2.IgnoreLabels = cmd.IgnoreLabels
		cmd2.IgnoreAnnotations = cmd.IgnoreAnnotations
		result, err := cmd2.Run(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K)
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
