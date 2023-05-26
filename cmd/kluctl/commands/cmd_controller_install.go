package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/go-embed-python/embed_util"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/install/controller"
	"time"
)

type controllerInstallCmd struct {
	args.YesFlags
	args.DryRunFlags
	args.CommandResultFlags

	Context           string `group:"misc" help:"Override the context to use."`
	ControllerVersion string `group:"misc" help:"Specify the controller version to install."`
}

func (cmd *controllerInstallCmd) Help() string {
	return `This command will install the kluctl-controller to the current Kubernetes clusters.`
}

func (cmd *controllerInstallCmd) Run(ctx context.Context) error {
	src, err := embed_util.NewEmbeddedFiles(controller.Project, "kluctl-controller-deployment")
	if err != nil {
		return err
	}

	var deployArgs []string
	if cmd.ControllerVersion != "" {
		deployArgs = append(deployArgs, fmt.Sprintf("controller_version=%s", cmd.ControllerVersion))
	}

	cmd2 := deployCmd{
		ProjectFlags: args.ProjectFlags{
			ProjectDir: args.ProjectDir{
				ProjectDir: args.ExistingDirType(src.GetExtractedPath()),
			},
			Timeout: 10 * time.Minute,
		},
		TargetFlags: args.TargetFlags{
			Context: cmd.Context,
		},
		ArgsFlags: args.ArgsFlags{
			Arg: deployArgs,
		},
		DryRunFlags:        cmd.DryRunFlags,
		CommandResultFlags: cmd.CommandResultFlags,
		internal:           true,
	}
	return cmd2.Run(ctx)
}
