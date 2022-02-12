package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmdDeploy(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		return runCmdDeploy2(cmd, ctx)
	})
}

func runCmdDeploy2(cmd *cobra.Command, ctx *commandCtx) error {
	if !args.ForceYes && !args.DryRun {
		if !AskForConfirmation(fmt.Sprintf("Do you really want to deploy to the context/cluster %s", ctx.k.Context())) {
			return fmt.Errorf("aborted")
		}
	}

	result, err := ctx.deploymentCollection.Deploy(ctx.k, args.ForceApply, args.ReplaceOnError, args.ForceReplaceOnError, args.AbortOnError, args.HookTimeout)
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
		Use:   "deploy",
		Short: "Deploys a target to the corresponding cluster",
		Long: "Deploys a target to the corresponding cluster.\n\n" +
			"This command will also output a diff between the initial state and the state after " +
			"deployment. The format of this diff is the same as for the `diff` command. " +
			"It will also output a list of prunable objects (without actually deleting them).",
		RunE: runCmdDeploy,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Yes:             true,
		DryRun:          true,
		ForceApply:      true,
		ReplaceOnError:  true,
		AbortOnError:    true,
		HookTimeout:     true,
		OutputFormat:    true,
		RenderOutputDir: true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
