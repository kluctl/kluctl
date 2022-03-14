package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment/commands"
)

type downscaleCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
}

func (cmd *downscaleCmd) Help() string {
	return `This command will downscale all Deployments, StatefulSets and CronJobs.
It is also possible to influence the behaviour with the help of annotations, as described in
the documentation.`
}

func (cmd *downscaleCmd) Run() error {
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
		if !cmd.Yes && !cmd.DryRun {
			if !AskForConfirmation(fmt.Sprintf("Do you really want to downscale on context/cluster %s?", ctx.k.Context())) {
				return fmt.Errorf("aborted")
			}
		}

		cmd2 := commands.NewDownscaleCommand(ctx.deploymentCollection)

		result, err := cmd2.Run(ctx.k)
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
