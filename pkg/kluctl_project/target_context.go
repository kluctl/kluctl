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

func (p *LoadedKluctlProject) NewTargetContext(ctx context.Context, targetName string, offlineK8s bool, dryRun bool, externalArgs *uo.UnstructuredObject, forSeal bool, images *deployment.Images, inclusion *utils.Inclusion, renderOutputDir string) (*TargetContext, error) {
	deploymentDir, err := filepath.Abs(p.ProjectDir)
	if err != nil {
		return nil, err
	}

	var target *types.Target
	if targetName != "" {
		t, err := p.FindDynamicTarget(targetName)
		if err != nil {
			return nil, err
		}
		target = t.Target

		images.PrependFixedImages(target.Images)
	}

	varsCtx, clientConfig, clusterContext, err := p.buildVars(target, offlineK8s, externalArgs, forSeal)
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
		k, err = k8s.NewK8sCluster(ctx, clientFactory, dryRun)
		if err != nil {
			s.Failed()
			return nil, err
		}
		s.Success()
	}

	varsLoader := vars.NewVarsLoader(ctx, k, p.RP, aws.NewClientFactory())

	if forSeal {
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
		RenderDir:                         renderOutputDir,
		SealedSecretsDir:                  p.sealedSecretsDir,
		DefaultSealedSecretsOutputPattern: targetName,
	}

	d, err := deployment.NewDeploymentProject(dctx, varsCtx, deployment.NewSource(deploymentDir), ".", nil)
	if err != nil {
		return nil, err
	}

	c, err := deployment.NewDeploymentCollection(dctx, d, images, inclusion, forSeal)
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

func (p *LoadedKluctlProject) buildVars(target *types.Target, offlineK8s bool, externalArgs *uo.UnstructuredObject, forSeal bool) (*vars.VarsCtx, *rest.Config, string, error) {
	doError := func(err error) (*vars.VarsCtx, *rest.Config, string, error) {
		return nil, nil, "", err
	}

	varsCtx := vars.NewVarsCtx(p.J2)

	contextName := target.Context

	var err error
	var clientConfig *rest.Config
	if !offlineK8s {
		var restConfig *api.Config
		clientConfig, restConfig, err = p.loadArgs.ClientConfigGetter(contextName)
		if err != nil {
			return doError(err)
		}
		if contextName == nil {
			contextName = &restConfig.CurrentContext
		}
	}

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return doError(err)
	}
	varsCtx.UpdateChild("target", targetVars)

	allArgs := uo.New()

	if target != nil {
		for argName, argValue := range externalArgs.Object {
			err = p.CheckDynamicArg(target, argName, argValue)
			if err != nil {
				return doError(err)
			}
		}
	}

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
		return doError(err)
	}

	varsCtx.UpdateChild("args", allArgs)

	var contextName2 string
	if contextName != nil {
		contextName2 = *contextName
	}
	return varsCtx, clientConfig, contextName2, nil
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
		if len(secretEntry.Sources) != 0 {
			status.Deprecation(p.ctx, "secrets-sets-sources", "'sources' in secretSets is deprecated, use 'vars' instead")
			err = varsLoader.LoadVarsList(varsCtx, secretEntry.Sources, searchDirs, "secrets")
		} else {
			err = varsLoader.LoadVarsList(varsCtx, secretEntry.Vars, searchDirs, "secrets")
		}
		if err != nil {
			return err
		}
	}
	return nil
}
