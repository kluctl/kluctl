package commands

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
)

type renderCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.RenderOutputDirFlags
}

func (cmd *renderCmd) Help() string {
	return `Renders all resources and configuration files and stores the result in either
a temporary directory or a specified directory.`
}

func (cmd *renderCmd) Run() error {
	if cmd.RenderOutputDir == "" {
		p, err := ioutil.TempDir(utils.GetTmpBaseDir(), "rendered-")
		if err != nil {
			return err
		}
		cmd.RenderOutputDir = p
	}

	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		log.Infof("Rendered into %s", ctx.deploymentCollection.RenderDir)
		return nil
	})
}
