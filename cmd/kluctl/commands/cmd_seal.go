package commands

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/kluctl_project"
	"github.com/codablock/kluctl/pkg/seal"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils/uo"
	log "github.com/sirupsen/logrus"
)

type sealCmd struct {
	args.ProjectFlags
	args.TargetFlags

	SecretsDir  string `help:"Specifies where to find unencrypted secret files. The given directory is NOT meant to be part of your source repository! The given path only matters for secrets of type 'path'. Defaults to the current working directory."`
	ForceReseal bool   `help:"Lets kluctl ignore secret hashes found in already sealed secrets and thus forces resealing of those."`
}

func (cmd *sealCmd) Help() string {
	return `Loads all secrets from the specified secrets sets from the target's sealingConfig and
then renders the target, including all files with the '.sealme' extension. Then runs
kubeseal on each '.sealme' file and stores secrets in the directory specified by
'--local-sealed-secrets', using the outputPattern from your deployment project.

If no '--target' is specified, sealing is performed for all targets.`
}

func findSecretsEntry(ctx *commandCtx, name string) (*types.SecretSet, error) {
	for _, e := range ctx.kluctlProject.Config.SecretsConfig.SecretSets {
		if e.Name == name {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("secret Set with name %s was not found", name)
}

func loadSecrets(ctx *commandCtx, target *types.Target, secretsLoader *seal.SecretsLoader) error {
	secrets := uo.New()
	for _, secretSetName := range target.SealingConfig.SecretSets {
		secretEntry, err := findSecretsEntry(ctx, secretSetName)
		if err != nil {
			return err
		}
		for _, source := range secretEntry.Sources {
			var renderedSource types.SecretSource
			err = ctx.kluctlProject.J2.RenderStruct(&renderedSource, &source, ctx.deploymentProject.VarsCtx.Vars)
			if err != nil {
				return err
			}
			s, err := secretsLoader.LoadSecrets(&renderedSource)
			if err != nil {
				return err
			}
			secrets.Merge(s)
		}
	}
	ctx.deploymentProject.MergeSecretsIntoAllChildren(secrets)
	return nil
}

func (cmd *sealCmd) runCmdSealForTarget(p *kluctl_project.KluctlProjectContext, target *types.Target, secretsLoader *seal.SecretsLoader) error {
	log.Infof("Sealing for target %s", target.Name)

	ptArgs := projectTargetCommandArgs{
		projectFlags: cmd.ProjectFlags,
		targetFlags:  cmd.TargetFlags,
	}

	// pass forSeal=True so that .sealme files are rendered as well
	return withProjectTargetCommandContext(ptArgs, p, target, true, func(ctx *commandCtx) error {
		err := loadSecrets(ctx, target, secretsLoader)
		if err != nil {
			return err
		}
		err = ctx.deploymentCollection.RenderDeployments(ctx.k)
		if err != nil {
			return err
		}

		sealedSecretsNamespace := "kube-system"
		sealedSecretsControllerName := "sealed-secrets-controller"
		if p.Config.SecretsConfig != nil && p.Config.SecretsConfig.SealedSecrets != nil {
			if p.Config.SecretsConfig.SealedSecrets.Namespace != nil {
				sealedSecretsNamespace = *p.Config.SecretsConfig.SealedSecrets.Namespace
			}
			if p.Config.SecretsConfig.SealedSecrets.ControllerName != nil {
				sealedSecretsControllerName = *p.Config.SecretsConfig.SealedSecrets.ControllerName
			}
		}
		if p.Config.SecretsConfig == nil || p.Config.SecretsConfig.SealedSecrets == nil || p.Config.SecretsConfig.SealedSecrets.Bootstrap == nil || *p.Config.SecretsConfig.SealedSecrets.Bootstrap {
			err = seal.BootstrapSealedSecrets(ctx.k, sealedSecretsNamespace)
			if err != nil {
				return err
			}
		}

		clusterConfig, err := p.LoadClusterConfig(target.Cluster)
		if err != nil {
			return err
		}
		sealer, err := seal.NewSealer(ctx.k, sealedSecretsNamespace, sealedSecretsControllerName, clusterConfig.Cluster, cmd.ForceReseal)
		if err != nil {
			return err
		}

		err = ctx.deploymentCollection.Seal(sealer)
		if err != nil {
			return err
		}
		return err
	})
}

func (cmd *sealCmd) Run() error {
	return withKluctlProjectFromArgs(cmd.ProjectFlags, func(p *kluctl_project.KluctlProjectContext) error {
		hadError := false

		secretsLoader := seal.NewSecretsLoader(p, cmd.SecretsDir)

		baseTargets := make(map[string]bool)
		noTargetMatch := true
		for _, target := range p.DynamicTargets {
			if cmd.Target != "" && cmd.Target != target.Target.Name {
				continue
			}
			if cmd.Cluster != "" && cmd.Cluster != target.Target.Cluster {
				continue
			}
			if target.Target.SealingConfig == nil {
				log.Infof("Target %s has no sealingConfig", target.Target.Name)
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

			err := cmd.runCmdSealForTarget(p, sealTarget, secretsLoader)
			if err != nil {
				log.Warningf("Sealing for target %s failed: %v", sealTarget.Name, err)
				hadError = true
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
