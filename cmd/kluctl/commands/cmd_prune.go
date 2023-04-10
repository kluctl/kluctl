package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"time"
)

type pruneCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.ImageFlags
	args.InclusionFlags
	args.HelmCredentials
	args.YesFlags
	args.DryRunFlags
	args.OutputFormatFlags
	args.RenderOutputDirFlags
}

func (cmd *pruneCmd) Help() string {
	return `"Searching works by:

  1. Search the cluster for all objects match 'commonLabels', as configured in 'deployment.yaml'
  2. Render the local target and list all objects.
  3. Remove all objects from the list of 1. that are part of the list in 2.`
}

func (cmd *pruneCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		imageFlags:           cmd.ImageFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		dryRunArgs:           &cmd.DryRunFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
		needsResultStore:     true,
	}
	startTime := time.Now()
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		return cmd.runCmdPrune(cmdCtx, startTime)
	})
}

func (cmd *pruneCmd) runCmdPrune(cmdCtx *commandCtx, startTime time.Time) error {
	cmd2 := commands.NewPruneCommand(cmdCtx.targetCtx.Target.Discriminator, cmdCtx.targetCtx.DeploymentCollection)
	result, err := cmd2.Run(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, func(refs []k8s2.ObjectRef) error {
		return confirmDeletion(cmdCtx.ctx, refs, cmd.DryRun, cmd.Yes)
	})
	if err != nil {
		return err
	}
	err = addCommandInfo(result, startTime, "prune", cmdCtx, &cmd.TargetFlags, &cmd.ImageFlags, &cmd.InclusionFlags, &cmd.DryRunFlags, nil, nil, nil, false)
	if err != nil {
		return err
	}
	err = outputCommandResult(cmdCtx, cmd.OutputFormatFlags, result, true)
	if err != nil {
		return err
	}
	if len(result.Errors) != 0 {
		return fmt.Errorf("command failed")
	}
	return nil
}
