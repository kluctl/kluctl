package commands

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/types"
)

type listImagesCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.OutputFlags

	Simple bool `group:"misc" help:"Output a simplified version of the images list"`
}

func (cmd *listImagesCmd) Help() string {
	return `The result is a compatible with yaml files expected by --fixed-images-file.

If fixed images ('-f/--fixed-image') are provided, these are also taken into account,
as described in for the deploy command.`
}

func (cmd *listImagesCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:   cmd.ProjectFlags,
		targetFlags:    cmd.TargetFlags,
		argsFlags:      cmd.ArgsFlags,
		imageFlags:     cmd.ImageFlags,
		inclusionFlags: cmd.InclusionFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		result := types.FixedImagesConfig{
			Images: ctx.images.SeenImages(cmd.Simple),
		}
		return outputYamlResult(cmd.Output, result, false)
	})
}
