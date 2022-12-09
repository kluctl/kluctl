package commands

import (
	"context"
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
	args.InclusionFlags
	args.HelmCredentials
	args.RenderOutputDirFlags
	args.OfflineKubernetesFlags

	PrintAll bool `group:"misc" help:"Write all rendered manifests to stdout"`
}

func (cmd *renderCmd) Help() string {
	return `Renders all resources and configuration files and stores the result in either
a temporary directory or a specified directory.`
}

func (cmd *renderCmd) Run(ctx context.Context) error {
	isTmp := false
	if cmd.RenderOutputDir == "" {
		p, err := ioutil.TempDir(utils.GetTmpBaseDir(ctx), "rendered-")
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
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		offlineKubernetes:    cmd.OfflineKubernetes,
		kubernetesVersion:    cmd.KubernetesVersion,
	}
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		if cmd.PrintAll {
			var all []any
			for _, d := range cmdCtx.targetCtx.DeploymentCollection.Deployments {
				for _, o := range d.Objects {
					all = append(all, o)
				}
			}
			if isTmp {
				defer os.RemoveAll(cmd.RenderOutputDir)
			}
			status.Flush(cmdCtx.ctx)
			return yaml.WriteYamlAllStream(getStdout(ctx), all)
		} else {
			status.Info(cmdCtx.ctx, "Rendered into %s", cmdCtx.targetCtx.SharedContext.RenderDir)
		}
		return nil
	})
}
