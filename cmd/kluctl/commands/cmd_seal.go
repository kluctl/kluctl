package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/seal"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type sealCmd struct {
	args.ProjectFlags
	args.TargetFlags

	ForceReseal bool `group:"misc" help:"Lets kluctl ignore secret hashes found in already sealed secrets and thus forces resealing of those."`
}

func (cmd *sealCmd) Help() string {
	return `Loads all secrets from the specified secrets sets from the target's sealingConfig and
then renders the target, including all files with the '.sealme' extension. Then runs
kubeseal on each '.sealme' file and stores secrets in the directory specified by
'--local-sealed-secrets', using the outputPattern from your deployment project.

If no '--target' is specified, sealing is performed for all targets.`
}

func (cmd *sealCmd) runCmdSealForTarget(ctx context.Context, p *kluctl_project.LoadedKluctlProject, targetName string) error {
	s := status.Start(ctx, "%s: Sealing for target", targetName)
	defer s.FailedWithMessage("%s: Sealing failed", targetName)

	doFail := func(err error) error {
		s.FailedWithMessage(fmt.Sprintf("Sealing failed: %v", err))
		return err
	}

	ptArgs := projectTargetCommandArgs{
		projectFlags: cmd.ProjectFlags,
		targetFlags:  cmd.TargetFlags,
		forSeal:      true,
	}
	ptArgs.targetFlags.Target = targetName

	// pass forSeal=True so that .sealme files are rendered as well
	return withProjectTargetCommandContext(ctx, ptArgs, p, func(ctx *commandCtx) error {
		err := ctx.targetCtx.DeploymentCollection.RenderDeployments()
		if err != nil {
			return doFail(err)
		}

		secretsConfig := p.Config.SecretsConfig
		var sealedSecretsConfig *types.GlobalSealedSecretsConfig
		if secretsConfig != nil {
			sealedSecretsConfig = secretsConfig.SealedSecrets
		}

		sealedSecretsNamespace := "kube-system"
		sealedSecretsControllerName := "sealed-secrets-controller"
		if sealedSecretsConfig != nil {
			if sealedSecretsConfig.Namespace != nil {
				sealedSecretsNamespace = *sealedSecretsConfig.Namespace
			}
			if sealedSecretsConfig.ControllerName != nil {
				sealedSecretsControllerName = *sealedSecretsConfig.ControllerName
			}
		}

		if sealedSecretsConfig == nil || sealedSecretsConfig.Bootstrap == nil || *sealedSecretsConfig.Bootstrap {
			err = seal.BootstrapSealedSecrets(ctx.ctx, ctx.targetCtx.SharedContext.K, sealedSecretsNamespace)
			if err != nil {
				return doFail(err)
			}
		}

		cert, err := seal.FetchCert(ctx.ctx, ctx.targetCtx.SharedContext.K, sealedSecretsNamespace, sealedSecretsControllerName)
		if err != nil {
			return doFail(err)
		}

		sealer, err := seal.NewSealer(ctx.ctx, cert, cmd.ForceReseal)
		if err != nil {
			return doFail(err)
		}

		outputPattern := targetName
		if ctx.targetCtx.DeploymentProject.Config.SealedSecrets != nil && ctx.targetCtx.DeploymentProject.Config.SealedSecrets.OutputPattern != nil {
			// the outputPattern is rendered already at this point, meaning that for example
			// '{{ cluster.name }}/{{ target.name }}' will already be rendered to 'my-cluster/my-target'
			outputPattern = *ctx.targetCtx.DeploymentProject.Config.SealedSecrets.OutputPattern
		}

		cmd2 := commands.NewSealCommand(ctx.targetCtx.DeploymentCollection, outputPattern, ctx.targetCtx.SharedContext.RenderDir, ctx.targetCtx.SharedContext.SealedSecretsDir)
		err = cmd2.Run(sealer)

		if err != nil {
			return doFail(err)
		}
		s.Success()
		return nil
	})
}

func (cmd *sealCmd) Run() error {
	return withKluctlProjectFromArgs(cmd.ProjectFlags, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		hadError := false

		baseTargets := make(map[string]bool)
		noTargetMatch := true
		for _, target := range p.DynamicTargets {
			if cmd.Target != "" && cmd.Target != target.Target.Name {
				continue
			}
			if cmd.Cluster != "" && target.Target.Cluster != nil && cmd.Cluster != *target.Target.Cluster {
				continue
			}
			if target.Target.SealingConfig == nil {
				status.Info(ctx, "Target %s has no sealingConfig", target.Target.Name)
				continue
			}
			noTargetMatch = false

			sealTarget := target.Target
			dynamicSealing := target.Target.SealingConfig.DynamicSealing == nil || *target.Target.SealingConfig.DynamicSealing
			isDynamicTarget := target.BaseTargetName != target.Target.Name
			if !dynamicSealing && isDynamicTarget {
				baseTarget, err := p.FindBaseTarget(target.BaseTargetName)
				if err != nil {
					return err
				}
				if baseTargets[target.BaseTargetName] {
					// Skip this target as it was already sealed
					continue
				}
				baseTargets[target.BaseTargetName] = true
				sealTarget = baseTarget
			}

			err := cmd.runCmdSealForTarget(ctx, p, sealTarget.Name)
			if err != nil {
				hadError = true
				status.Error(ctx, err.Error())
			}
		}
		if hadError {
			return fmt.Errorf("sealing for at least one target failed")
		}
		if noTargetMatch {
			return fmt.Errorf("no target matched the given target and/or cluster name")
		}
		return nil
	})
}
