package kluctl_project

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/git"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/codablock/kluctl/pkg/jinja2_server"
	"github.com/codablock/kluctl/pkg/types"
	"regexp"
)

type LoadKluctlProjectArgs struct {
	ProjectUrl          *git_url.GitUrl
	ProjectRef          string
	ProjectConfig       string
	LocalClusters       string
	LocalDeployment     string
	LocalSealedSecrets  string
	FromArchive         string
	FromArchiveMetadata string

	JS *jinja2_server.Jinja2Server
}

type KluctlProjectContext struct {
	loadArgs LoadKluctlProjectArgs

	TmpDir string
	Config types.KluctlProject

	ProjectDir       string
	DeploymentDir    string
	ClustersDir      string
	SealedSecretsDir string

	involvedRepos  map[string][]types.InvolvedRepo
	DynamicTargets []*types.DynamicTarget

	mirroredRepos map[string]*git.MirroredGitRepo

	JS *jinja2_server.Jinja2Server
}

func NewKluctlProjectContext(loadArgs LoadKluctlProjectArgs, tmpDir string) *KluctlProjectContext {
	o := &KluctlProjectContext{
		loadArgs:      loadArgs,
		TmpDir:        tmpDir,
		involvedRepos: make(map[string][]types.InvolvedRepo),
		mirroredRepos: make(map[string]*git.MirroredGitRepo),
		JS:            loadArgs.JS,
	}
	return o
}

func (c *KluctlProjectContext) FindBaseTarget(name string) (*types.Target, error) {
	for _, target := range c.Config.Targets {
		if target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}

func (c *KluctlProjectContext) FindDynamicTarget(name string) (*types.DynamicTarget, error) {
	for _, target := range c.DynamicTargets {
		if target.Target.Name == name {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target %s not existent in kluctl project config", name)
}

func (c *KluctlProjectContext) LoadClusterConfig(clusterName string) (*types.ClusterConfig, error) {
	return types.LoadClusterConfig(c.ClustersDir, clusterName)
}

func (c *KluctlProjectContext) CheckDynamicArg(target *types.Target, argName string, argValue string) error {
	var dynArg *types.DynamicArg
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
