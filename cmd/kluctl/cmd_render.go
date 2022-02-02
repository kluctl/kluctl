package main

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/ioutil"
)

func runCmdRender(cmd *cobra.Command, args_ []string) error {
	if args.RenderOutputDir == "" {
		p, err := ioutil.TempDir(utils.GetTmpBaseDir(), "rendered-")
		if err != nil {
			return err
		}
		args.RenderOutputDir = p
	}

	return withProjectCommandContext(func(ctx *commandCtx) error {
		log.Infof("Rendered into %s", ctx.deploymentCollection.RenderDir)
		return nil
	})
}

func init() {
	var cmd = &cobra.Command{
		Use:   "render",
		Short: "Renders all resources and configuration files",
		Long: "Renders all resources and configuration files and stores the result in either " +
			"a temporary directory or a specified directory.",
		RunE: runCmdRender,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		RenderOutputDir: true,
	})

	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}

