package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmdDownscale(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		if !args.ForceYes && !args.DryRun {
			if !AskForConfirmation(fmt.Sprintf("Do you really want to downscale on context/cluster %s?", ctx.k.Context())) {
				return fmt.Errorf("aborted")
			}
		}
		result, err := ctx.deploymentCollection.Downscale(ctx.k)
		if err != nil {
			return err
		}
		err = outputCommandResult(args.Output, result)
		if err != nil {
			return err
		}
		if len(result.Errors) != 0 {
			return fmt.Errorf("command failed")
		}
		return nil
	})
}

func init() {
	var cmd = &cobra.Command{
		Use:   "downscale",
		Short: "Downscale all deployments",
		Long: "Downscale all deployments.\n\n" +
			"This command will downscale all Deployments, StatefulSets and CronJobs. " +
			"It is also possible to influence the behaviour with the help of annotations, as described in " +
			"the documentation.",
		RunE: runCmdDownscale,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Yes:             true,
		DryRun:          true,
		OutputFormat:    true,
		RenderOutputDir: true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
