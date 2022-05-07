package kluctl_project

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"regexp"
)

type LoadedKluctlProject struct {
	ctx context.Context

	loadArgs LoadKluctlProjectArgs

	TmpDir     string
	archiveDir string

	projectRootDir string
	ProjectDir     string

	DeploymentDir    string
	ClustersDir      string
	sealedSecretsDir string

	involvedRepos map[string][]types2.InvolvedRepo
	mirroredRepos map[string]*git.MirroredGitRepo

	Config         types2.KluctlProject
	DynamicTargets []*types2.DynamicTarget

	J2 *jinja2.Jinja2
}

func (c *LoadedKluctlProject) GetMetadata() *types2.ProjectMetadata {
	md := &types2.ProjectMetadata{
		InvolvedRepos: c.involvedRepos,
		Targets:       c.DynamicTargets,
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

func (c *LoadedKluctlProject) LoadClusterConfig(clusterName string) (*types2.ClusterConfig, error) {
	return types2.LoadClusterConfig(c.ClustersDir, clusterName)
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

	argPattern := ".*"
	if dynArg.Pattern != nil {
		argPattern = *dynArg.Pattern
	}
	argPattern = fmt.Sprintf("^%s$", argPattern)

	m, err := regexp.MatchString(argPattern, argValue)
	if err != nil {
		return err
	}
	if !m {
		return fmt.Errorf("dynamic argument %s does not match required pattern '%s", argName, argPattern)
	}
	return nil
}
