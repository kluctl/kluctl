package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type listImagesCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.HelmCredentials
	args.OutputFlags
	args.RenderOutputDirFlags
	args.OfflineKubernetesFlags

	Simple bool `group:"misc" help:"Output a simplified version of the images list"`
}

func (cmd *listImagesCmd) Help() string {
	return `The result is a compatible with yaml files expected by --fixed-images-file.

If fixed images ('-f/--fixed-image') are provided, these are also taken into account,
as described in the deploy command.`
}

func (cmd *listImagesCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		offlineKubernetes:    cmd.OfflineKubernetes,
		kubernetesVersion:    cmd.KubernetesVersion,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		result := types.FixedImagesConfig{
			Images: cmdCtx.images.SeenImages(cmd.Simple),
		}
		return outputYamlResult(ctx, cmd.Output, result, false)
	})
}
