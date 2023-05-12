package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"time"
)

type validateCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
	args.HelmCredentials
	args.OutputFlags
	args.RenderOutputDirFlags

	CommandResult args.ExistingFileType `group:"misc" help:"Specify a command result to use instead of loading a project. This will also perform drift detection."`

	Wait             time.Duration `group:"misc" help:"Wait for the given amount of time until the deployment validates"`
	Sleep            time.Duration `group:"misc" help:"Sleep duration between validation attempts" default:"5s"`
	WarningsAsErrors bool          `group:"misc" help:"Consider warnings as failures"`
}

func (cmd *validateCmd) Help() string {
	return `This means that all objects are retrieved from the cluster and checked for readiness.

TODO: This needs to be better documented!`
}

func (cmd *validateCmd) Run(ctx context.Context) error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		inclusionFlags:       cmd.InclusionFlags,
		helmCredentials:      cmd.HelmCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}

	if cmd.CommandResult.String() != "" {
		var ccr result.CompactedCommandResult
		err := yaml.ReadYamlFile(cmd.CommandResult.String(), &ccr)
		if err != nil {
			return err
		}
		commandResult := ccr.ToNonCompacted()

		var k8sContext *string
		if cmd.Context != "" {
			k8sContext = &cmd.Context
		} else if commandResult.Target.Context != nil {
			k8sContext = commandResult.Target.Context
		}
		clientFactory, err := k8s2.NewClientFactoryFromDefaultConfig(ctx, k8sContext)
		if err != nil {
			return err
		}
		k, err := k8s2.NewK8sCluster(ctx, clientFactory, true)
		if err != nil {
			return err
		}

		cmd2 := commands.NewValidateCommand(ctx, "", nil, commandResult)
		return cmd.doValidate(ctx, k, cmd2)

	} else {
		return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
			cmd2 := commands.NewValidateCommand(cmdCtx.ctx, cmdCtx.targetCtx.Target.Discriminator, cmdCtx.targetCtx.DeploymentCollection, nil)
			return cmd.doValidate(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, cmd2)
		})
	}
}

func (cmd *validateCmd) doValidate(ctx context.Context, k *k8s2.K8sCluster, cmd2 *commands.ValidateCommand) error {
	startTime := time.Now()
	for true {
		result, err := cmd2.Run(ctx, k)
		if err != nil {
			return err
		}
		failed := len(result.Errors) != 0 || (cmd.WarningsAsErrors && len(result.Warnings) != 0)

		err = outputValidateResult(ctx, cmd.Output, result)
		if err != nil {
			return err
		}

		if !failed {
			_, _ = getStderr(ctx).WriteString("Validation succeeded\n")
			return nil
		}

		if cmd.Wait <= 0 || time.Now().Sub(startTime) > cmd.Wait {
			return fmt.Errorf("Validation failed")
		}

		time.Sleep(cmd.Sleep)

		// Need to force re-requesting these objects
		for _, e := range result.Results {
			cmd2.ForgetRemoteObject(e.Ref)
		}
		for _, e := range result.Warnings {
			cmd2.ForgetRemoteObject(e.Ref)
		}
		for _, e := range result.Errors {
			cmd2.ForgetRemoteObject(e.Ref)
		}
	}
	return nil
}
