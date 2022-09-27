package kluctl_project

import (
	"fmt"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/git/repocache"
	"github.com/kluctl/kluctl/v2/pkg/status"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/ioutil"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
)

type LoadKluctlProjectArgs struct {
	RepoRoot           string
	ProjectDir         string
	ProjectUrl         *git_url.GitUrl
	ProjectRef         string
	ProjectConfig      string
	LocalClusters      string
	LocalDeployment    string
	LocalSealedSecrets string

	RP *repocache.GitRepoCache

	ClientConfigGetter func(context *string) (*rest.Config, *api.Config, error)
}

type gitProjectInfo struct {
	url      git_url.GitUrl
	ref      string
	commit   string
	repoRoot string
	dir      string
}

func (c *LoadedKluctlProject) getConfigPath() string {
	configPath := c.loadArgs.ProjectConfig
	if configPath == "" {
		p := yaml.FixPathExt(filepath.Join(c.ProjectDir, ".kluctl.yml"))
		if utils.IsFile(p) {
			configPath = p
		}
	}
	return configPath
}

func (c *LoadedKluctlProject) localProject(dir string) gitProjectInfo {
	return gitProjectInfo{
		dir: dir,
	}
}

func (c *LoadedKluctlProject) loadGitProject(gitProject *types2.GitProject, defaultSubDir string) (ret gitProjectInfo, err error) {
	ge, err := c.RP.GetEntry(gitProject.Url)
	if err != nil {
		return
	}

	cloneDir, ri, err := ge.GetClonedDir(gitProject.Ref)
	if err != nil {
		return
	}

	ret.url = gitProject.Url
	ret.ref = ri.CheckedOutRef
	ret.commit = ri.CheckedOutCommit
	ret.repoRoot = cloneDir

	subDir := gitProject.SubDir
	if subDir == "" {
		subDir = defaultSubDir
	}
	ret.dir = filepath.Join(ret.repoRoot, subDir)
	err = utils.CheckInDir(ret.repoRoot, ret.dir)
	if err != nil {
		return
	}

	return
}

func (c *LoadedKluctlProject) loadExternalProject(ep *types2.ExternalProject, defaultGitSubDir string, localDir string) (gitProjectInfo, error) {
	if localDir != "" {
		return c.localProject(localDir), nil
	}

	if ep == nil {
		// no ExternalProject provided, so we point into the kluctl project + defaultGitSubDir
		p := filepath.Join(c.ProjectDir, defaultGitSubDir)
		return c.localProject(p), nil
	}

	if ep.Project != nil {
		status.Deprecation(c.ctx, "external-projects", "External projects are deprecated and support for them will be removed in the future. "+
			"Use Git variable sources as replacement for cluster configs and Git includes as replacement for external deployment projects.")

		// pointing to an actual external project, so let's try to clone it
		return c.loadGitProject(ep.Project, defaultGitSubDir)
	}

	// ExternalProject was provided but without an external repo url, so point into the kluctl project.
	// We also allow to leave the kluctl project dir but limit it to the git project

	p := filepath.Join(c.ProjectDir, *ep.Path)
	err := utils.CheckInDir(c.projectRootDir, p)
	if err != nil {
		return gitProjectInfo{}, fmt.Errorf("path '%s' is not inside git project root '%s': %w", p, c.projectRootDir, err)
	}
	return c.localProject(p), nil
}

func (c *LoadedKluctlProject) loadKluctlProject() error {
	var err error

	if c.loadArgs.ProjectUrl == nil {
		c.projectRootDir = c.loadArgs.RepoRoot
		c.ProjectDir = c.loadArgs.ProjectDir
		err = utils.CheckInDir(c.projectRootDir, c.ProjectDir)
		if err != nil {
			return err
		}
	} else {
		gi, err := c.loadGitProject(&types2.GitProject{
			Url: *c.loadArgs.ProjectUrl,
			Ref: c.loadArgs.ProjectRef,
		}, "")
		if err != nil {
			return err
		}
		c.projectRootDir = gi.repoRoot
		c.ProjectDir = gi.dir
	}

	configPath := c.getConfigPath()

	if configPath != "" {
		err = yaml.ReadYamlFile(configPath, &c.Config)
		if err != nil {
			return err
		}
	}

	err = c.updateGitCaches()
	if err != nil {
		return err
	}

	s := status.Start(c.ctx, "Loading kluctl project")
	defer s.Failed()

	if c.loadArgs.LocalClusters != "" {
		status.Deprecation(c.ctx, "--local-clusters", "--local-clusters is deprecated and will be removed in an upcoming version. Use variables loaded from git instead.")
	}
	if c.loadArgs.LocalDeployment != "" {
		status.Deprecation(c.ctx, "--local-deployment", "--local-deployment is deprecated and will be removed in an upcoming version. Use git includes instead.")
	}
	if c.loadArgs.LocalSealedSecrets != "" {
		status.Deprecation(c.ctx, "--local-sealed-secrets", "--local-sealed-secrets is deprecated and will be removed in an upcoming version.")
	}

	deploymentInfo, err := c.loadExternalProject(c.Config.Deployment, "", c.loadArgs.LocalDeployment)
	if err != nil {
		return err
	}
	sealedSecretsInfo, err := c.loadExternalProject(c.Config.SealedSecrets, ".sealed-secrets", c.loadArgs.LocalSealedSecrets)
	if err != nil {
		return err
	}
	var clustersInfos []gitProjectInfo
	if c.loadArgs.LocalClusters != "" {
		clustersInfos = append(clustersInfos, c.localProject(c.loadArgs.LocalClusters))
	} else if len(c.Config.Clusters.Projects) != 0 {
		for _, ep := range c.Config.Clusters.Projects {
			info, err := c.loadExternalProject(&ep, "clusters", "")
			if err != nil {
				return err
			}
			clustersInfos = append(clustersInfos, info)
		}
	} else {
		ci, err := c.loadExternalProject(nil, "clusters", "")
		if err != nil {
			return err
		}
		clustersInfos = append(clustersInfos, ci)
	}

	mergedClustersDir := filepath.Join(c.TmpDir, "merged-clusters")
	err = c.mergeClustersDirs(mergedClustersDir, clustersInfos)
	if err != nil {
		return err
	}

	c.DeploymentDir = deploymentInfo.dir
	c.ClustersDir = mergedClustersDir
	c.sealedSecretsDir = sealedSecretsInfo.dir

	s.Success()

	return nil
}

func (c *LoadedKluctlProject) mergeClustersDirs(mergedClustersDir string, clustersInfos []gitProjectInfo) error {
	err := os.MkdirAll(mergedClustersDir, 0o700)
	if err != nil {
		return err
	}

	for _, ci := range clustersInfos {
		if !utils.IsDirectory(ci.dir) {
			continue
		}
		files, err := ioutil.ReadDir(ci.dir)
		if err != nil {
			return err
		}
		for _, fi := range files {
			p := filepath.Join(ci.dir, fi.Name())
			if utils.IsFile(p) {
				err = utils.CopyFile(p, filepath.Join(mergedClustersDir, fi.Name()))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
