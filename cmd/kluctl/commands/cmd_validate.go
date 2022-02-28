package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
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
		for true {
			result := ctx.deploymentCollection.Validate(ctx.k)
			failed := len(result.Errors) != 0 || (cmd.WarningsAsErrors && len(result.Warnings) != 0)

			err := outputValidateResult(cmd.Output, result)
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
