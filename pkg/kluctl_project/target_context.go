package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/clouds/gcp"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TargetContext struct {
	Params TargetContextParams

	SharedContext deployment.SharedContext

	KluctlProject        *LoadedKluctlProject
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
	HelmAuthProvider   helm_auth.HelmAuthProvider
	OciAuthProvider    auth_provider.OciAuthProvider
	RenderOutputDir    string
}

func (p *LoadedKluctlProject) NewTargetContext(ctx context.Context, contextName string, k *k8s.K8sCluster, params TargetContextParams) (*TargetContext, error) {
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
		err = p.renderTarget(target)
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

	sopsDecryptor, err := p.buildSopsDecrypter(ctx, client, target)
	if err != nil {
		return nil, err
	}
	varsLoader := vars.NewVarsLoader(ctx, k, sopsDecryptor, p.GitRP, aws.NewClientFactory(client, target.Aws), gcp.NewClientFactory())

	if params.ForSeal {
		err = p.loadSecrets(ctx, target, varsCtx, varsLoader)
		if err != nil {
			return nil, err
		}
	}

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
		SealedSecretsDir:                  p.sealedSecretsDir,
		DefaultSealedSecretsOutputPattern: target.Name,
	}

	targetCtx := &TargetContext{
		Params:         params,
		SharedContext:  dctx,
		KluctlProject:  p,
		Target:         *target,
		ClusterContext: contextName,
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

func (p *LoadedKluctlProject) LoadK8sConfig(ctx context.Context, params TargetContextParams) (*rest.Config, string, error) {
	if params.OfflineK8s {
		return nil, "", nil
	}

	var contextName *string
	if params.TargetName != "" {
		t, err := p.FindTarget(params.TargetName)
		if err != nil {
			return nil, "", err
		}
		contextName = t.Context
	}
	if params.ContextOverride != "" {
		contextName = &params.ContextOverride
	}

	var err error
	var clientConfig *rest.Config
	var restConfig *api.Config
	clientConfig, restConfig, err = p.LoadArgs.ClientConfigGetter(contextName)
	if err != nil {
		if contextName == nil && clientcmd.IsEmptyConfig(err) {
			status.Warning(ctx, "No valid KUBECONFIG provided, which means the Kubernetes client is not available. Depending on your deployment project, this might cause follow-up errors.")
			return nil, "", nil
		}
		return nil, "", err
	}
	contextName = &restConfig.CurrentContext
	return clientConfig, *contextName, nil
}

func (p *LoadedKluctlProject) BuildVars(target *types.Target, forSeal bool) (*vars.VarsCtx, error) {
	varsCtx := vars.NewVarsCtx(p.J2)

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return nil, err
	}
	varsCtx.UpdateChild("target", targetVars)

	allArgs := uo.New()

	if target != nil {
		if target.Args != nil {
			allArgs.Merge(target.Args)
		}
		if forSeal {
			if target.SealingConfig.Args != nil {
				allArgs.Merge(target.SealingConfig.Args)
			}
		}
	}
	if p.LoadArgs.ExternalArgs != nil {
		allArgs.Merge(p.LoadArgs.ExternalArgs)
	}

	err = deployment.LoadDefaultArgs(p.Config.Args, allArgs)
	if err != nil {
		return nil, err
	}

	varsCtx.UpdateChild("args", allArgs)

	return varsCtx, nil
}

func (p *LoadedKluctlProject) findSecretsEntry(name string) (*types.SecretSet, error) {
	for _, e := range p.Config.SecretsConfig.SecretSets {
		if e.Name == name {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("secret Set with name %s was not found", name)
}

func (p *LoadedKluctlProject) loadSecrets(ctx context.Context, target *types.Target, varsCtx *vars.VarsCtx, varsLoader *vars.VarsLoader) error {
	searchDirs := []string{p.LoadArgs.ProjectDir}

	for _, secretSetName := range target.SealingConfig.SecretSets {
		secretEntry, err := p.findSecretsEntry(secretSetName)
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
