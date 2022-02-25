package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"os"
)

type deleteCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
}

func (cmd *deleteCmd) Help() string {
	return `Objects are located based on 'deleteByLabels'', configured in 'deployment.yml'

WARNING: This command will also delete objects which are not part of your deployment
project (anymore). It really only decides based on the 'deleteByLabel' labels and does NOT
take the local target/state into account!`
}

func (cmd *deleteCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:   cmd.ProjectFlags,
		targetFlags:    cmd.TargetFlags,
		argsFlags:      cmd.ArgsFlags,
		imageFlags:     cmd.ImageFlags,
		inclusionFlags: cmd.InclusionFlags,
		dryRunArgs:     &cmd.DryRunFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		objects, err := ctx.deploymentCollection.FindDeleteObjects(ctx.k)
		if err != nil {
			return err
		}
		result, err := confirmedDeleteObjects(ctx.k, objects, cmd.DryRun, cmd.Yes)
		if err != nil {
			return err
		}
		err = outputCommandResult(cmd.OutputFormat, result)
		if err != nil {
			return err
		}
		if len(result.Errors) != 0 {
			return fmt.Errorf("command failed")
		}
		return nil
	})
}

func confirmedDeleteObjects(k *k8s.K8sCluster, refs []types.ObjectRef, dryRun bool, forceYes bool) (*types.CommandResult, error) {
	if len(refs) != 0 {
		_, _ = os.Stderr.WriteString("The following objects will be deleted:\n")
		for _, ref := range refs {
			_, _ = os.Stderr.WriteString(fmt.Sprintf("  %s\n", ref.String()))
		}
		if !forceYes && !dryRun {
			if !AskForConfirmation(fmt.Sprintf("Do you really want to delete %d objects?", len(refs))) {
				return nil, fmt.Errorf("aborted")
			}
		}
	}

	return k8s.DeleteObjects(k, refs, true)
}
