package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
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
	return withProjectCommandContext(ctx, ptArgs, func(cmdCtx *commandCtx) error {
		startTime := time.Now()
		cmd2 := commands.NewValidateCommand(cmdCtx.ctx, cmdCtx.targetCtx.DeploymentCollection)
		for true {
			result, err := cmd2.Run(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K)
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
	})
}
