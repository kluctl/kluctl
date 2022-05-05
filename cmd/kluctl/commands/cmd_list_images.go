package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"sort"
)

type listImagesCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.OutputFlags
	args.RenderOutputDirFlags

	Sort   bool `group:"misc" help:"Sort Output of the image list"`
	Simple bool `group:"misc" help:"Output a simplified version of the images list"`
}

func (cmd *listImagesCmd) Help() string {
	return `The result is a compatible with yaml files expected by --fixed-images-file.

If fixed images ('-f/--fixed-image') are provided, these are also taken into account,
as described in for the deploy command.`
}

func (cmd *listImagesCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		result := types.FixedImagesConfig{
			Images: ctx.images.SeenImages(cmd.Simple),
		}
		if cmd.Sort {
			sort.Slice(result.Images, func(i, j int) bool {
				return result.Images[i].Image < result.Images[j].Image
			})
		}
		return outputYamlResult(cmd.Output, result, false)
	})
}
