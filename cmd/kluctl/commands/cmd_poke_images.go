package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment/commands"
)

type pokeImagesCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.YesFlags
	args.DryRunFlags
	args.OutputFlags
	args.RenderOutputDirFlags
}

func (cmd *pokeImagesCmd) Help() string {
	return `This command will fully render the target and then only replace images instead of fully
deploying the target. Only images used in combination with 'images.get_image(...)' are
replaced`
}

func (cmd *pokeImagesCmd) Run() error {
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
			if !AskForConfirmation(fmt.Sprintf("Do you really want to poke images to the context/cluster %s?", ctx.k.Context())) {
				return fmt.Errorf("aborted")
			}
		}

		cmd2 := commands.NewPokeImagesCommand(ctx.deploymentCollection)

		result, err := cmd2.Run(ctx.k)
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
	})
}
