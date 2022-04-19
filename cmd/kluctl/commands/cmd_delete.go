package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	utils2 "github.com/kluctl/kluctl/v2/pkg/utils"
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

	DeleteByLabel []string `group:"misc" short:"l" help:"Override the labels used to find objects for deletion."`
}

func (cmd *deleteCmd) Help() string {
	return `Objects are located based on 'commonLabels'', configured in 'deployment.yml'

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
		cmd2 := commands.NewDeleteCommand(ctx.targetCtx.DeploymentCollection)

		deleteByLabels, err := deployment.ParseArgs(cmd.DeleteByLabel)
		if err != nil {
			return err
		}

		cmd2.OverrideDeleteByLabels = deleteByLabels

		objects, err := cmd2.Run(ctx.targetCtx.K)
		if err != nil {
			return err
		}
		result, err := confirmedDeleteObjects(ctx.targetCtx.K, objects, cmd.DryRun, cmd.Yes)
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

func confirmedDeleteObjects(k *k8s.K8sCluster, refs []k8s2.ObjectRef, dryRun bool, forceYes bool) (*types.CommandResult, error) {
	if len(refs) != 0 {
		_, _ = os.Stderr.WriteString("The following objects will be deleted:\n")
		for _, ref := range refs {
			_, _ = os.Stderr.WriteString(fmt.Sprintf("  %s\n", ref.String()))
		}
		if !forceYes && !dryRun {
			if !utils2.AskForConfirmation(fmt.Sprintf("Do you really want to delete %d objects?", len(refs))) {
				return nil, fmt.Errorf("aborted")
			}
		}
	}

	return utils.DeleteObjects(k, refs, true)
}
