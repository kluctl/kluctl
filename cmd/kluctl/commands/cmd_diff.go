package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
)

type diffCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
	args.ImageFlags
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

func (cmd *diffCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		result, err := ctx.deploymentCollection.Diff(ctx.k, cmd.ForceApply, cmd.ReplaceOnError, cmd.ForceReplaceOnError, cmd.IgnoreTags, cmd.IgnoreLabels, cmd.IgnoreAnnotations)
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
	})
}
