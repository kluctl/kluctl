package kluctl_project

import (
	"fmt"
	types2 "github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/yaml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
)

func (c *KluctlProjectContext) mergeClustersDirs(mergedClustersDir string, clustersInfos []gitProjectInfo) error {
	err := os.MkdirAll(mergedClustersDir, 0o700)
	if err != nil {
		return err
	}

	for _, ci := range clustersInfos {
		if !utils.IsDirectory(ci.dir) {
			log.Warningf("Cluster dir '%s' does not exist", ci.dir)
			continue
		}
		files, err := ioutil.ReadDir(ci.dir)
		if err != nil {
			return err
		}
		for _, fi := range files {
			p := path.Join(ci.dir, fi.Name())
			if utils.IsFile(p) {
				err = utils.CopyFile(p, path.Join(mergedClustersDir, fi.Name()))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (c *KluctlProjectContext) load(allowGit bool) error {
	kluctlProjectInfo, err := c.cloneKluctlProject()
	if err != nil {
		return err
	}

	configPath := c.loadArgs.ProjectConfig
	if configPath == "" {
		p := path.Join(kluctlProjectInfo.dir, ".kluctl.yml")
		if utils.IsFile(p) {
			configPath = p
		}
	}

	if configPath != "" {
		err = yaml.ReadYamlFile(configPath, &c.Config)
		if err != nil {
			return err
		}
	}

	if allowGit {
		err = c.updateGitCaches()
		if err != nil {
			return err
		}
	}

	doClone := func(ep *types2.ExternalProject, defaultGitSubDir string, localDir string) (gitProjectInfo, error) {
		if localDir != "" {
			return c.localProject(localDir), nil
		}
		if ep == nil {
			p := kluctlProjectInfo.dir
			if defaultGitSubDir != "" {
				p = path.Join(p, defaultGitSubDir)
			}
			return c.localProject(p), nil
		}
		if !allowGit {
			return gitProjectInfo{}, fmt.Errorf("tried to load something from git while it was not allowed")
		}

		return c.cloneGitProject(*ep, defaultGitSubDir, true, true)
	}

	deploymentInfo, err := doClone(c.Config.Deployment, "", c.loadArgs.LocalDeployment)
	if err != nil {
		return err
	}
	sealedSecretsInfo, err := doClone(c.Config.SealedSecrets, ".sealed-secrets", c.loadArgs.LocalSealedSecrets)
	if err != nil {
		return err
	}
	var clustersInfos []gitProjectInfo
	if c.loadArgs.LocalClusters != "" {
		clustersInfos = append(clustersInfos, c.localProject(c.loadArgs.LocalClusters))
	} else if len(c.Config.Clusters.Projects) != 0 {
		for _, ep := range c.Config.Clusters.Projects {
			info, err := doClone(&ep, "clusters", "")
			if err != nil {
				return err
			}
			clustersInfos = append(clustersInfos, info)
		}
	} else {
		ci, err := doClone(nil, "clusters", "")
		if err != nil {
			return err
		}
		clustersInfos = append(clustersInfos, ci)
	}

	mergedClustersDir := path.Join(c.TmpDir, "merged-clusters")
	err = c.mergeClustersDirs(mergedClustersDir, clustersInfos)
	if err != nil {
		return err
	}

	c.ProjectDir = kluctlProjectInfo.dir
	c.DeploymentDir = deploymentInfo.dir
	c.ClustersDir = mergedClustersDir
	c.SealedSecretsDir = sealedSecretsInfo.dir

	return nil
}

func LoadKluctlProject(args LoadKluctlProjectArgs, cb func(ctx *KluctlProjectContext) error) error {
	tmpDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "project-")
	if err != nil {
		return fmt.Errorf("creating temporary project directory failed: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if args.FromArchive != "" {
		if args.ProjectUrl != nil || args.ProjectRef != "" || args.ProjectConfig != "" || args.LocalClusters != "" || args.LocalDeployment != "" || args.LocalSealedSecrets != "" {
			return fmt.Errorf("--from-archive can not be combined with any other project related option")
		}
		project, err := loadKluctlProjectFromArchive(args.FromArchive, args.FromArchiveMetadata, tmpDir)
		if err != nil {
			return err
		}
		err = project.load(false)
		if err != nil {
			return err
		}
		return cb(project)
	} else {
		p := NewKluctlProjectContext(args, tmpDir)
		err = p.load(true)
		if err != nil {
			return err
		}
		err = p.loadTargets()
		if err != nil {
			return err
		}
		return cb(p)
	}
}

func loadKluctlProjectFromArchive(fromArchive string, fromArchiveMetadata string, tmpDir string) (*KluctlProjectContext, error) {
	var dir string
	if utils.IsFile(fromArchiveMetadata) {
		err := utils.ExtractTarGz(fromArchive, tmpDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract archive %v: %w", fromArchive, err)
		}
		dir = tmpDir
	} else {
		dir = fromArchive
	}

	var metdataPath string
	if fromArchiveMetadata != "" {
		metdataPath = fromArchiveMetadata
	} else {
		metdataPath = path.Join(dir, "metadata.yml")
	}

	var metadata types2.ArchiveMetadata
	err := yaml.ReadYamlFile(metdataPath, &metadata)
	if err != nil {
		return nil, err
	}

	p := NewKluctlProjectContext(
		LoadKluctlProjectArgs{
			ProjectConfig:      path.Join(dir, ".kluctl.yml"),
			LocalClusters:      path.Join(dir, "clusters"),
			LocalDeployment:    path.Join(dir, "deployment"),
			LocalSealedSecrets: path.Join(dir, "sealed-secrets"),
		}, dir)
	p.involvedRepos = metadata.InvolvedRepos
	p.DynamicTargets = metadata.Targets
	return p, nil
}
