package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/rest"
	"path/filepath"
)

type TargetContext struct {
	KluctlProject        *LoadedKluctlProject
	Target               *types.Target
	ClusterConfig        *types.ClusterConfig
	K                    *k8s.K8sCluster
	DeploymentProject    *deployment.DeploymentProject
	DeploymentCollection *deployment.DeploymentCollection
}

func (p *LoadedKluctlProject) NewTargetContext(ctx context.Context, clientConfigGetter func(context string) (*rest.Config, error), targetName string, clusterName string, dryRun bool, args map[string]string, forSeal bool, images *deployment.Images, inclusion *utils.Inclusion, renderOutputDir string) (*TargetContext, error) {
	deploymentDir, err := filepath.Abs(p.DeploymentDir)
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

		for _, fi := range target.Images {
			images.AddFixedImage(fi)
		}
	}

	if clusterName == "" {
		if target == nil {
			return nil, fmt.Errorf("you must specify an existing --cluster when not providing a --target")
		}
		clusterName = target.Cluster
	}

	clusterConfig, err := p.LoadClusterConfig(clusterName)
	if err != nil {
		return nil, err
	}

	clientConfig, err := clientConfigGetter(clusterConfig.Cluster.Context)
	if err != nil {
		return nil, err
	}

	var k *k8s.K8sCluster
	if clientConfig != nil {
		s := status.Start(ctx, "Initializing k8s client")
		k, err = k8s.NewK8sCluster(ctx, clientConfig, dryRun)
		if err != nil {
			s.Failed()
			return nil, err
		}
		s.Success()
	}

	varsCtx := jinja2.NewVarsCtx(p.J2)
	err = varsCtx.UpdateChildFromStruct("cluster", clusterConfig.Cluster)
	if err != nil {
		return nil, err
	}

	allArgs := uo.New()

	if target != nil {
		for argName, argValue := range args {
			err = p.CheckDynamicArg(target, argName, argValue)
			if err != nil {
				return nil, err
			}
		}
	}
	allArgs.Merge(deployment.ConvertArgsToVars(args))
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

	err = deployment.CheckRequiredDeployArgs(deploymentDir, varsCtx, allArgs)
	if err != nil {
		return nil, err
	}

	varsCtx.UpdateChild("args", allArgs)

	targetVars, err := uo.FromStruct(target)
	if err != nil {
		return nil, err
	}
	varsCtx.UpdateChild("target", targetVars)

	d, err := deployment.NewDeploymentProject(ctx, k, varsCtx, deploymentDir, p.sealedSecretsDir, nil)
	if err != nil {
		return nil, err
	}
	c, err := deployment.NewDeploymentCollection(ctx, d, images, inclusion, renderOutputDir, forSeal)
	if err != nil {
		return nil, err
	}

	targetCtx := &TargetContext{
		KluctlProject:        p,
		Target:               target,
		ClusterConfig:        clusterConfig,
		K:                    k,
		DeploymentProject:    d,
		DeploymentCollection: c,
	}

	return targetCtx, nil
}
