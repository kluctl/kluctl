package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/git/repocache"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type LoadedKluctlProject struct {
	ctx context.Context

	loadArgs LoadKluctlProjectArgs

	TmpDir string

	projectRootDir string
	ProjectDir     string

	DeploymentDir    string
	ClustersDir      string
	sealedSecretsDir string

	Config         types2.KluctlProject
	DynamicTargets []*types2.DynamicTarget

	J2 *jinja2.Jinja2
	RP *repocache.GitRepoCache

	warnOnce utils.OnceByKey
}

func (c *LoadedKluctlProject) GetMetadata() *types2.ProjectMetadata {
	md := &types2.ProjectMetadata{
		Targets: c.DynamicTargets,
	}
	return md
}

func (c *LoadedKluctlProject) FindBaseTarget(name string) (*types2.Target, error) {
	for _, target := range c.Config.Targets {
		if target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}

func (c *LoadedKluctlProject) FindDynamicTarget(name string) (*types2.DynamicTarget, error) {
	for _, target := range c.DynamicTargets {
		if target.Target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}

func (c *LoadedKluctlProject) CheckDynamicArg(target *types2.Target, argName string, argValue string) error {
	var dynArg *types2.DynamicArg
	for _, x := range target.DynamicArgs {
		if x.Name == argName {
			dynArg = &x
			break
		}
	}
	if dynArg == nil {
		return fmt.Errorf("dynamic argument %s is not allowed for target", argName)
	}

	return nil
}
