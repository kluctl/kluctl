package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

func runCmdDelete(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		objects, err := ctx.deploymentCollection.FindDeleteObjects(ctx.k)
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
	})
}

func confirmedDeleteObjects(k *k8s.K8sCluster, refs []types.ObjectRef) (*types.CommandResult, error) {
	if len(refs) != 0 {
		_, _ = os.Stderr.WriteString("The following objects will be deleted:\n")
		for _, ref := range refs {
			_, _ = os.Stderr.WriteString(fmt.Sprintf("  %s\n", ref.String()))
		}
		if !args.ForceYes && !args.DryRun {
			if !AskForConfirmation(fmt.Sprintf("Do you really want to delete %d objects?", len(refs))) {
				return nil, fmt.Errorf("aborted")
			}
		}
	}

	return k8s.DeleteObjects(k, refs, false)
}

func init() {
	var cmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a target (or parts of it) from the corresponding cluster",
		Long: "Delete a target (or parts of it) from the corresponding cluster.\n\n" +
			"Objects are located based on `deleteByLabels`, configured in `deployment.yml`\n\n" +
			"WARNING: This command will also delete objects which are not part of your deployment " +
			"project (anymore). It really only decides based on the `deleteByLabel` labels and does NOT " +
			"take the local target/state into account!",
		RunE: runCmdDelete,
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
