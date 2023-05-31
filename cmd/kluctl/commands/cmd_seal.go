package commands

import (
	"context"
	"crypto/x509"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment/commands"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/seal"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"os"
)

type sealCmd struct {
	args.ProjectFlags
	args.TargetFlags
	args.HelmCredentials
	args.OfflineKubernetesFlags

	ForceReseal bool   `group:"misc" help:"Lets kluctl ignore secret hashes found in already sealed secrets and thus forces resealing of those."`
	CertFile    string `group:"misc" help:"Use the given certificate for sealing instead of requesting it from the sealed-secrets controller"`
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
		projectFlags:      cmd.ProjectFlags,
		targetFlags:       cmd.TargetFlags,
		helmCredentials:   cmd.HelmCredentials,
		forSeal:           true,
		offlineKubernetes: cmd.OfflineKubernetes,
		kubernetesVersion: cmd.KubernetesVersion,
	}
	ptArgs.targetFlags.Target = targetName

	// pass forSeal=True so that .sealme files are rendered as well
	return withProjectTargetCommandContext(ctx, ptArgs, p, func(cmdCtx *commandCtx) error {
		err := cmdCtx.targetCtx.DeploymentCollection.RenderDeployments()
		if err != nil {
			return doFail(err)
		}

		cert, err := cmd.loadCert(cmdCtx)
		if err != nil {
			return doFail(err)
		}

		sealer, err := seal.NewSealer(cmdCtx.ctx, cert, cmd.ForceReseal)
		if err != nil {
			return doFail(err)
		}

		outputPattern := targetName
		if cmdCtx.targetCtx.DeploymentProject.Config.SealedSecrets != nil && cmdCtx.targetCtx.DeploymentProject.Config.SealedSecrets.OutputPattern != nil {
			// the outputPattern is rendered already at this point, meaning that for example
			// '{{ cluster.name }}/{{ target.name }}' will already be rendered to 'my-cluster/my-target'
			outputPattern = *cmdCtx.targetCtx.DeploymentProject.Config.SealedSecrets.OutputPattern
		}

		cmd2 := commands.NewSealCommand(cmdCtx.targetCtx.DeploymentCollection, outputPattern, cmdCtx.targetCtx.SharedContext.RenderDir, cmdCtx.targetCtx.SharedContext.SealedSecretsDir)
		err = cmd2.Run(sealer)

		if err != nil {
			return doFail(err)
		}
		s.Success()
		return nil
	})
}

func (cmd *sealCmd) loadCert(cmdCtx *commandCtx) (*x509.Certificate, error) {
	sealingConfig := cmdCtx.targetCtx.Target.SealingConfig

	var certFile string

	if sealingConfig != nil && sealingConfig.CertFile != nil {
		path, err := securejoin.SecureJoin(cmdCtx.targetCtx.KluctlProject.LoadArgs.ProjectDir, *sealingConfig.CertFile)
		if err != nil {
			return nil, err
		}
		certFile = path
	}

	if cmd.CertFile != "" {
		if certFile != "" {
			status.Info(cmdCtx.ctx, "Overriding certFile from target with certFile argument")
		}
		certFile = cmd.CertFile
	}

	if certFile != "" {
		d, err := os.ReadFile(certFile)
		if err != nil {
			return nil, err
		}
		cert, err := seal.ParseCert(d)
		if err != nil {
			return nil, err
		}
		return cert, nil
	} else {
		if cmdCtx.targetCtx.SharedContext.K == nil {
			return nil, fmt.Errorf("must specify certFile when sealing in offline mode")
		}

		secretsConfig := cmdCtx.targetCtx.KluctlProject.Config.SecretsConfig
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
			err := seal.BootstrapSealedSecrets(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, sealedSecretsNamespace)
			if err != nil {
				return nil, err
			}
		}

		cert, err := seal.FetchCert(cmdCtx.ctx, cmdCtx.targetCtx.SharedContext.K, sealedSecretsNamespace, sealedSecretsControllerName)
		if err != nil {
			return nil, err
		}
		return cert, nil
	}
}

func (cmd *sealCmd) Run(ctx context.Context) error {
	return withKluctlProjectFromArgs(ctx, cmd.ProjectFlags, nil, false, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		hadError := false

		noTargetMatch := true
		for _, target := range p.Targets {
			if cmd.Target != "" && cmd.Target != target.Name {
				continue
			}
			if target.SealingConfig == nil {
				status.Info(ctx, "Target %s has no sealingConfig", target.Name)
				continue
			}
			noTargetMatch = false

			err := cmd.runCmdSealForTarget(ctx, p, target.Name)
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
