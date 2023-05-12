package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"time"
)

type pokeImagesCmd struct {
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
	args.CommandResultFlags
}

func (cmd *pokeImagesCmd) Help() string {
	return `This command will fully render the target and then only replace images instead of fully
deploying the target. Only images used in combination with 'images.get_image(...)' are
replaced`
}

func (cmd *pokeImagesCmd) Run(ctx context.Context) error {
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
	}
	startTime := time.Now()
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		if !cmd.Yes && !cmd.DryRun {
			if !status.AskForConfirmation(ctx, fmt.Sprintf("Do you really want to poke images to the context/cluster %s?", cmdCtx.targetCtx.ClusterContext)) {
				return fmt.Errorf("aborted")
			}
		}

		cmd2 := commands.NewPokeImagesCommand(cmdCtx.targetCtx)

		result, err := cmd2.Run()
		if err != nil {
			return err
		}
		err = addCommandInfo(result, startTime, "poke-images", cmdCtx, &cmd.TargetFlags, &cmd.ImageFlags, &cmd.InclusionFlags, &cmd.DryRunFlags, nil, nil, nil, false)
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
	})
}
