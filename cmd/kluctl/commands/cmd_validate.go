package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"time"
)

type validateCmd struct {
	args.ProjectFlags
	args.KubeconfigFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
	args.GitCredentials
	args.HelmCredentials
	args.RegistryCredentials
	args.OutputFlags
	args.RenderOutputDirFlags

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
		kubeconfigFlags:      cmd.KubeconfigFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		inclusionFlags:       cmd.InclusionFlags,
		gitCredentials:       cmd.GitCredentials,
		helmCredentials:      cmd.HelmCredentials,
		registryCredentials:  cmd.RegistryCredentials,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}

	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		cmd2 := commands.NewValidateCommand("", cmdCtx.targetCtx)
		return cmd.doValidate(ctx, cmdCtx, cmd2)
	})
}

func (cmd *validateCmd) doValidate(ctx context.Context, cmdCtx *commandCtx, cmd2 *commands.ValidateCommand) error {
	startTime := time.Now()
	for true {
		result := cmd2.Run(ctx)
		failed := len(result.Errors) != 0 || (cmd.WarningsAsErrors && len(result.Warnings) != 0)

		err := outputValidateResult(ctx, cmdCtx, cmd.Output, result)
		if err != nil {
			return err
		}

		if !failed {
			status.Info(ctx, "Validation succeeded")
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
