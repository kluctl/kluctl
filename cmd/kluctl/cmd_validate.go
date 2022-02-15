package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"time"
)

func runCmdValidate(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		startTime := time.Now()
		for true {
			result := ctx.deploymentCollection.Validate(ctx.k)
			failed := len(result.Errors) != 0 || (args.WarningsAsErrors && len(result.Warnings) != 0)

			err := outputValidateResult(args.Output, result)
			if err != nil {
				return err
			}

			if !failed {
				_, _ = os.Stderr.WriteString("Validation succeeded\n")
				return nil
			}

			if args.Wait <= 0 || time.Now().Sub(startTime) > args.Wait {
				return fmt.Errorf("Validation failed")
			}

			time.Sleep(args.Sleep)

			// Need to force re-requesting these objects
			for _, e := range result.Results {
				ctx.deploymentCollection.ForgetRemoteObject(e.Ref)
			}
			for _, e := range result.Warnings {
				ctx.deploymentCollection.ForgetRemoteObject(e.Ref)
			}
			for _, e := range result.Errors {
				ctx.deploymentCollection.ForgetRemoteObject(e.Ref)
			}
		}
		return nil
	})
}

func init() {
	var cmd = &cobra.Command{
		Use:   "validate",
		Short: "Validates the already deployed deployment",
		Long: "Validates the already deployed deployment.\n\n" +
			"This means that all objects are retrieved from the cluster and checked for readiness.\n\n" +
			"TODO: This needs to be better documented!",
		RunE: runCmdValidate,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Output:          true,
		RenderOutputDir: true,
	})
	cmd.Flags().DurationVar(&args.Wait, "wait", 0, "Wait for the given amount of time until the deployment validates")
	cmd.Flags().DurationVar(&args.Sleep, "sleep", time.Second*5, "Sleep duration between validation attempts")
	cmd.Flags().BoolVar(&args.WarningsAsErrors, "warnings-as-errors", false, "Consider warnings as failures")
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
