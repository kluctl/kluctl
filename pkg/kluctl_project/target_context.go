package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"github.com/kluctl/kluctl/v2/pkg/vars/aws"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"path/filepath"
)

type TargetContext struct {
	SharedContext deployment.SharedContext

	KluctlProject        *LoadedKluctlProject
	Target               *types.Target
	ClusterContext       string
	DeploymentProject    *deployment.DeploymentProject
	DeploymentCollection *deployment.DeploymentCollection
}

type TargetContextParams struct {
	TargetName         string
	TargetNameOverride string
	ContextOverride    string
	OfflineK8s         bool
	DryRun             bool
	ExternalArgs       *uo.UnstructuredObject
	ForSeal            bool
	Images             *deployment.Images
	Inclusion          *utils.Inclusion
	RenderOutputDir    string
}

func (p *LoadedKluctlProject) NewTargetContext(ctx context.Context, params TargetContextParams) (*TargetContext, error) {
	deploymentDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, err
	}

	var target *types.Target
	if params.TargetName != "" {
		t, err := p.FindDynamicTarget(params.TargetName)
		if err != nil {
			return nil, err
		}
		target = &*t.Target
	} else {
		target = &types.Target{}
	}
	if params.TargetNameOverride != "" {
		target.Name = params.TargetNameOverride
	}
	if params.ContextOverride != "" {
		target.Context = &params.ContextOverride
	}

	params.Images.PrependFixedImages(target.Images)

	clientConfig, clusterContext, err := p.loadK8sConfig(target, params.OfflineK8s)
	if err != nil {
		return nil, err
	}
	target.Context = &clusterContext

	varsCtx, err := p.buildVars(target, params.ExternalArgs, params.ForSeal)
	if err != nil {
		return nil, err
	}

	var k *k8s.K8sCluster
	if clientConfig != nil {
		s := status.Start(ctx, fmt.Sprintf("Initializing k8s client"))
		clientFactory, err := k8s.NewClientFactory(clientConfig)
		if err != nil {
			return nil, err
		}
		k, err = k8s.NewK8sCluster(ctx, clientFactory, params.DryRun)
		if err != nil {
			s.Failed()
			return nil, err
		}
		s.Success()
	}

	varsLoader := vars.NewVarsLoader(ctx, k, p.RP, aws.NewClientFactory())

	if params.ForSeal {
		err = p.loadSecrets(target, varsCtx, varsLoader)
		if err != nil {
			return nil, err
		}
	}

	dctx := deployment.SharedContext{
		Ctx:                               ctx,
		K:                                 k,
		RP:                                p.RP,
		VarsLoader:                        varsLoader,
		RenderDir:                         params.RenderOutputDir,
		SealedSecretsDir:                  p.sealedSecretsDir,
		DefaultSealedSecretsOutputPattern: target.Name,
	}

	d, err := deployment.NewDeploymentProject(dctx, varsCtx, deployment.NewSource(deploymentDir), ".", nil)
	if err != nil {
		return nil, err
	}

	c, err := deployment.NewDeploymentCollection(dctx, d, params.Images, params.Inclusion, params.ForSeal)
	if err != nil {
		return nil, err
	}

	targetCtx := &TargetContext{
		SharedContext:        dctx,
		KluctlProject:        p,
		Target:               target,
		ClusterContext:       clusterContext,
		DeploymentProject:    d,
		DeploymentCollection: c,
	}

	return targetCtx, nil
}

func (p *LoadedKluctlProject) loadK8sConfig(target *types.Target, offlineK8s bool) (*rest.Config, string, error) {
	if offlineK8s {
		return nil, "", nil
	}

	contextName := target.Context

	var err error
	var clientConfig *rest.Config
	var restConfig *api.Config
	clientConfig, restConfig, err = p.loadArgs.ClientConfigGetter(contextName)
	if err != nil {
		return nil, "", err
	}
	if contextName == nil {
		contextName = &restConfig.CurrentContext
	}
	if contextName != nil {
		return clientConfig, *contextName, nil
	}
	return clientConfig, "", nil
}

func (p *LoadedKluctlProject) buildVars(target *types.Target, externalArgs *uo.UnstructuredObject, forSeal bool) (*vars.VarsCtx, error) {
	varsCtx := vars.NewVarsCtx(p.J2)

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return nil, err
	}
	varsCtx.UpdateChild("target", targetVars)

	allArgs := uo.New()

	allArgs.Merge(externalArgs)
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

	err = deployment.LoadDeploymentArgs(p.ProjectDir, varsCtx, allArgs)
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

func (p *LoadedKluctlProject) loadSecrets(target *types.Target, varsCtx *vars.VarsCtx, varsLoader *vars.VarsLoader) error {
	searchDirs := []string{p.ProjectDir}

	for _, secretSetName := range target.SealingConfig.SecretSets {
		secretEntry, err := p.findSecretsEntry(secretSetName)
		if err != nil {
			return err
		}
		err = varsLoader.LoadVarsList(varsCtx, secretEntry.Vars, searchDirs, "secrets")
		if err != nil {
			return err
		}
	}
	return nil
}
