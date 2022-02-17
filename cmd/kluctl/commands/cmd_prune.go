package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmdPrune(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		return runCmdPrune2(cmd, ctx)
	})
}

func runCmdPrune2(cmd *cobra.Command, ctx *commandCtx) error {
	objects, err := ctx.deploymentCollection.FindOrphanObjects(ctx.k)
	if err != nil {
		return err
	}
	result, err := confirmedDeleteObjects(ctx.k, objects)
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
}

func init() {
	var cmd = &cobra.Command{
		Use:   "prune",
		Short: "Searches the target cluster for prunable objects and deletes them",
		Long: "Searches the target cluster for prunable objects and deletes them.\n\n" +
			"Searching works by:\n\n" +
			"\b\n" +
			"  1. Search the cluster for all objects match `deleteByLabels`, as configured in `deployment.yml`\n" +
			"  2. Render the local target and list all objects.\n" +
			"  3. Remove all objects from the list of 1. that are part of the list in 2.\n",
		RunE: runCmdPrune,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Yes:          true,
		DryRun:       true,
		OutputFormat: true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
