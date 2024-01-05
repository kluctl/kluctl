package target_context

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/clouds/gcp"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TargetContext struct {
	Params TargetContextParams

	SharedContext deployment.SharedContext

	KluctlProject        *kluctl_project.LoadedKluctlProject
	Target               types.Target
	ClusterContext       string
	DeploymentProject    *deployment.DeploymentProject
	DeploymentCollection *deployment.DeploymentCollection
}

type TargetContextParams struct {
	TargetName         string
	TargetNameOverride string
	ContextOverride    string
	OfflineK8s         bool
	K8sVersion         string
	DryRun             bool
	ForSeal            bool
	Images             *deployment.Images
	Inclusion          *utils.Inclusion
	HelmAuthProvider   auth.HelmAuthProvider
	OciAuthProvider    auth_provider.OciAuthProvider
	RenderOutputDir    string
}

func NewTargetContext(ctx context.Context, p *kluctl_project.LoadedKluctlProject, contextName string, k *k8s.K8sCluster, params TargetContextParams) (*TargetContext, error) {
	repoRoot, err := filepath.Abs(p.LoadArgs.RepoRoot)
	if err != nil {
		return nil, err
	}
	relProjectDir, err := filepath.Rel(repoRoot, p.LoadArgs.ProjectDir)
	if err != nil {
		return nil, err
	}

	var target *types.Target
	needRender := false
	if params.TargetName != "" {
		t, err := p.FindTarget(params.TargetName)
		if err != nil {
			return nil, err
		}
		target = &*t
	} else {
		if len(p.Targets) != 0 {
			status.Deprecation(ctx, "no-target", "Warning, tried to use Kluctl without explicitly specifying a target, while the Kluctl project contains target definitions. This was allowed in older version of Kluctl, but is forbidden since v2.23.0. If mixing deployments with and without targets was actually intended, please switch to creating and using a dedicated target that serves as a replacement for no-target deployments.")
			return nil, fmt.Errorf("a target must be explicitly selected when targets are defined in the Kluctl project")
		}
		target = &types.Target{
			Discriminator: p.Config.Discriminator,
		}
		needRender = true
	}
	if params.TargetNameOverride != "" {
		target.Name = params.TargetNameOverride
	}

	params.Images.PrependFixedImages(target.Images)

	target.Context = &contextName

	if needRender {
		// we must render the target after handling overrides
		err = p.RenderTarget(target)
		if err != nil {
			return nil, err
		}
	}

	varsCtx, err := p.BuildVars(target, params.ForSeal)
	if err != nil {
		return nil, err
	}

	var client client.Client
	if k != nil {
		client, err = k.ToClient()
		if err != nil {
			return nil, err
		}
	}

	sopsDecryptor, err := buildSopsDecrypter(ctx, p.LoadArgs.ProjectDir, client, target, p.LoadArgs.AddKeyServersFunc)
	if err != nil {
		return nil, err
	}
	varsLoader := vars.NewVarsLoader(ctx, k, sopsDecryptor, p.GitRP, aws.NewClientFactory(client, target.Aws), gcp.NewClientFactory())

	dctx := deployment.SharedContext{
		Ctx:                               ctx,
		K:                                 k,
		K8sVersion:                        params.K8sVersion,
		GitRP:                             p.GitRP,
		OciRP:                             p.OciRP,
		SopsDecrypter:                     sopsDecryptor,
		VarsLoader:                        varsLoader,
		HelmAuthProvider:                  params.HelmAuthProvider,
		OciAuthProvider:                   params.OciAuthProvider,
		Discriminator:                     target.Discriminator,
		RenderDir:                         params.RenderOutputDir,
		SealedSecretsDir:                  p.SealedSecretsDir,
		DefaultSealedSecretsOutputPattern: target.Name,
	}

	targetCtx := &TargetContext{
		Params:         params,
		SharedContext:  dctx,
		KluctlProject:  p,
		Target:         *target,
		ClusterContext: contextName,
	}

	if params.ForSeal {
		err = targetCtx.loadSecrets(ctx, target, varsCtx, varsLoader)
		if err != nil {
			return nil, err
		}
	}

	d, err := deployment.NewDeploymentProject(dctx, varsCtx, deployment.NewSource(repoRoot), relProjectDir, nil)
	if err != nil {
		return targetCtx, err
	}
	targetCtx.DeploymentProject = d

	c, err := deployment.NewDeploymentCollection(dctx, d, params.Images, params.Inclusion, params.ForSeal)
	if err != nil {
		return targetCtx, err
	}
	targetCtx.DeploymentCollection = c

	return targetCtx, nil
}

func (tc *TargetContext) findSecretsEntry(name string) (*types.SecretSet, error) {
	for _, e := range tc.KluctlProject.Config.SecretsConfig.SecretSets {
		if e.Name == name {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("secret Set with name %s was not found", name)
}

func (tc *TargetContext) loadSecrets(ctx context.Context, target *types.Target, varsCtx *vars.VarsCtx, varsLoader *vars.VarsLoader) error {
	searchDirs := []string{tc.KluctlProject.LoadArgs.ProjectDir}

	for _, secretSetName := range target.SealingConfig.SecretSets {
		secretEntry, err := tc.findSecretsEntry(secretSetName)
		if err != nil {
			return err
		}
		err = varsLoader.LoadVarsList(ctx, varsCtx, secretEntry.Vars, searchDirs, "secrets")
		if err != nil {
			return err
		}
	}
	return nil
}
