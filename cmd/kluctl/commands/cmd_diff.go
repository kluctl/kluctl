package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
)

type diffCmd struct {
	args.ProjectFlags
	args.KubeconfigFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
	args.ImageFlags
	args.GitCredentials
	args.HelmCredentials
	args.RegistryCredentials
	args.ForceApplyFlags
	args.ReplaceOnErrorFlags
	args.IgnoreFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags

	Discriminator string `group:"misc" help:"Override the target discriminator."`
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
		kubeconfigFlags:      cmd.KubeconfigFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		gitCredentials:       cmd.GitCredentials,
		helmCredentials:      cmd.HelmCredentials,
		registryCredentials:  cmd.RegistryCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		discriminator:        cmd.Discriminator,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		cmd2 := commands.NewDiffCommand(cmdCtx.targetCtx)
		cmd2.ForceApply = cmd.ForceApply
		cmd2.ReplaceOnError = cmd.ReplaceOnError
		cmd2.ForceReplaceOnError = cmd.ForceReplaceOnError
		cmd2.IgnoreTags = cmd.IgnoreTags
		cmd2.IgnoreLabels = cmd.IgnoreLabels
		cmd2.IgnoreAnnotations = cmd.IgnoreAnnotations
		cmd2.IgnoreKluctlMetadata = cmd.IgnoreKluctlMetadata
		result := cmd2.Run()
		err := outputCommandResult(ctx, cmdCtx, cmd.OutputFormatFlags, result, false)
		if err != nil {
			return err
		}
		if len(result.Errors) != 0 {
			return fmt.Errorf("command failed")
		}
		return nil
	})
}
