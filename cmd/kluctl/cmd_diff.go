package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmdDiff(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		result, err := ctx.deploymentCollection.Diff(ctx.k, args.ForceApply, args.ReplaceOnError, args.ForceReplaceOnError, args.IgnoreTags, args.IgnoreLabels, args.IgnoreAnnotations)
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
		Use:   "diff",
		Short: "Perform a diff between the locally rendered target and the already deployed target",
		Long: `Perform a diff between the locally rendered target and the already deployed target

The output is by default in human readable form (a table combined with unified diffs).
The output can also be changed to output yaml file. Please note however that the format
is currently not documented and prone to changes.
After the diff is performed, the command will also search for prunable objects and list them.`,
		RunE: runCmdDiff,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		ForceApply:      true,
		ReplaceOnError:  true,
		IgnoreLabels:    true,
		OutputFormat:    true,
		RenderOutputDir: true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
