package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"os"
	"time"
)

type validateCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.ArgsFlags
	args.InclusionFlags
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

func (cmd *validateCmd) Run() error {
	ptArgs := projectTargetCommandArgs{
		projectFlags:         cmd.ProjectFlags,
		targetFlags:          cmd.TargetFlags,
		argsFlags:            cmd.ArgsFlags,
		inclusionFlags:       cmd.InclusionFlags,
		renderOutputDirFlags: cmd.RenderOutputDirFlags,
	}
	return withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		startTime := time.Now()
		cmd2 := commands.NewValidateCommand(ctx.targetCtx.DeploymentCollection)
		for true {
			result, err := cmd2.Run(ctx.targetCtx.K)
			if err != nil {
				return err
			}
			failed := len(result.Errors) != 0 || (cmd.WarningsAsErrors && len(result.Warnings) != 0)

			err = outputValidateResult(cmd.Output, result)
			if err != nil {
				return err
			}

			if !failed {
				_, _ = os.Stderr.WriteString("Validation succeeded\n")
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
