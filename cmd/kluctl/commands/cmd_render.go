package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/ioutil"
	"os"
)

type renderCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.HelmCredentials
	args.RenderOutputDirFlags
	args.OfflineKubernetesFlags

	PrintAll bool `group:"misc" help:"Write all rendered manifests to stdout"`
}

func (cmd *renderCmd) Help() string {
	return `Renders all resources and configuration files and stores the result in either
a temporary directory or a specified directory.`
}

func (cmd *renderCmd) Run() error {
	isTmp := false
	if cmd.RenderOutputDir == "" {
		p, err := ioutil.TempDir(utils.GetTmpBaseDir(), "rendered-")
		if err != nil {
			return err
		}
		cmd.RenderOutputDir = p
		isTmp = true
	}

	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		helmCredentials:      cmd.HelmCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		offlineKubernetes:    cmd.OfflineKubernetes,
		kubernetesVersion:    cmd.KubernetesVersion,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		if cmd.PrintAll {
			var all []any
			for _, d := range ctx.targetCtx.DeploymentCollection.Deployments {
				for _, o := range d.Objects {
					all = append(all, o)
				}
			}
			if isTmp {
				defer os.RemoveAll(cmd.RenderOutputDir)
			}
			status.Flush(ctx.ctx)
			return yaml.WriteYamlAllStream(os.Stdout, all)
		} else {
			status.Info(ctx.ctx, "Rendered into %s", ctx.targetCtx.SharedContext.RenderDir)
		}
		return nil
	})
}
