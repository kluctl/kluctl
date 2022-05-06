package kluctl_project

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func (c *KluctlProjectContext) getConfigPath(projectDir string) string {
	configPath := c.loadArgs.ProjectConfig
	if configPath == "" {
		p := yaml.FixPathExt(filepath.Join(projectDir, ".kluctl.yml"))
		if utils.IsFile(p) {
			configPath = p
		}
	}
	return configPath
}

func (c *KluctlProjectContext) load(ctx context.Context, allowGit bool) error {
	kluctlProjectInfo, err := c.cloneKluctlProject(ctx)
	if err != nil {
		return err
	}

	configPath := c.getConfigPath(kluctlProjectInfo.dir)

	if configPath != "" {
		err = yaml.ReadYamlFile(configPath, &c.Config)
		if err != nil {
			return err
		}
	}

	if allowGit {
		err = c.updateGitCaches(ctx)
		if err != nil {
			return err
		}
	}

	doClone := func(ep *types2.ExternalProject, defaultGitSubDir string, localDir string) (gitProjectInfo, error) {
		if localDir != "" {
			return c.localProject(localDir), nil
		}

		if ep == nil {
			// no ExternalProject provided, so we point into the kluctl project + defaultGitSubDir
			p := kluctlProjectInfo.dir
			if defaultGitSubDir != "" {
				p = filepath.Join(p, defaultGitSubDir)
			}
			return c.localProject(p), nil
		}

		if ep.Project != nil {
			// pointing to an actual external project, so let's try to clone it
			if !allowGit {
				return gitProjectInfo{}, fmt.Errorf("tried to load something from git while it was not allowed")
			}
			return c.cloneGitProject(ctx, ep.Project, defaultGitSubDir, true, true)
		}

		// ExternalProject was provided but without an external repo url, so point into the kluctl project.
		// We also allow to leave the kluctl project dir in case RepoRoot is provided via loadArgs, as long as
		// the pointed to directory does not leave the RepoRoot

		repoRoot := c.loadArgs.RepoRoot
		if repoRoot == "" {
			// don't allow to leave the project dir
			repoRoot = kluctlProjectInfo.dir
		}

		p := filepath.Join(kluctlProjectInfo.dir, *ep.Path)
		err = utils.CheckInDir(repoRoot, p)
		if err != nil {
			if c.loadArgs.RepoRoot == "" {
				return gitProjectInfo{}, fmt.Errorf("it looks like your kluctl project is not part of a Git repository, which means you are not allowed to use paths that leave your kluctl project")
			}
			return gitProjectInfo{}, fmt.Errorf("path '%s' is not inside git project root '%s': %w", p, repoRoot, err)
		}
		return c.localProject(p), nil
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

	mergedClustersDir := filepath.Join(c.TmpDir, "merged-clusters")
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

func LoadKluctlProject(ctx context.Context, args LoadKluctlProjectArgs, tmpDir string, j2 *jinja2.Jinja2) (*KluctlProjectContext, error) {
	if args.FromArchive != "" {
		if args.ProjectUrl != nil || args.ProjectRef != "" || args.ProjectConfig != "" || args.LocalClusters != "" || args.LocalDeployment != "" || args.LocalSealedSecrets != "" {
			return nil, fmt.Errorf("--from-archive can not be combined with any other project related option")
		}
		project, err := loadKluctlProjectFromArchive(args, tmpDir, j2)
		if err != nil {
			return nil, err
		}
		err = project.load(ctx, false)
		if err != nil {
			return nil, err
		}
		return project, nil
	} else {
		p := NewKluctlProjectContext(args, tmpDir, j2)
		err := p.load(ctx, true)
		if err != nil {
			return nil, err
		}
		err = p.loadTargets(ctx)
		if err != nil {
			return nil, err
		}
		return p, nil
	}
}

func loadKluctlProjectFromArchive(args LoadKluctlProjectArgs, tmpDir string, j2 *jinja2.Jinja2) (*KluctlProjectContext, error) {
	var dir string
	if utils.IsFile(args.FromArchive) {
		err := utils.ExtractTarGzFile(args.FromArchive, tmpDir)
		if err != nil {
			return nil, fmt.Errorf("failed to extract archive %v: %w", args.FromArchive, err)
		}
		dir = tmpDir
	} else {
		dir = args.FromArchive
	}

	var metdataPath string
	if args.FromArchiveMetadata != "" {
		metdataPath = args.FromArchiveMetadata
	} else {
		metdataPath = yaml.FixPathExt(filepath.Join(dir, "metadata.yml"))
	}

	var metadata types2.ProjectMetadata
	err := yaml.ReadYamlFile(metdataPath, &metadata)
	if err != nil {
		return nil, err
	}

	p := NewKluctlProjectContext(LoadKluctlProjectArgs{
		ProjectConfig:      yaml.FixPathExt(filepath.Join(dir, ".kluctl.yml")),
		LocalClusters:      filepath.Join(dir, "clusters"),
		LocalDeployment:    filepath.Join(dir, "deployment"),
		LocalSealedSecrets: filepath.Join(dir, "sealed-secrets"),
	}, dir, j2)
	p.involvedRepos = metadata.InvolvedRepos
	p.DynamicTargets = metadata.Targets
	return p, nil
}

func (c *KluctlProjectContext) GetMetadata() *types2.ProjectMetadata {
	md := &types2.ProjectMetadata{
		InvolvedRepos: c.involvedRepos,
		Targets:       c.DynamicTargets,
	}
	return md
}

func (c *KluctlProjectContext) CreateTGZArchive(archivePath string, embedMetadata bool) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	filter := func(h *tar.Header, size int64) (*tar.Header, error) {
		if strings.HasSuffix(strings.ToLower(h.Name), ".git") {
			return nil, nil
		}
		h.Uid = 0
		h.Gid = 0
		h.Uname = ""
		h.Gname = ""
		h.ModTime = time.Time{}
		h.ChangeTime = time.Time{}
		h.AccessTime = time.Time{}
		return h, nil
	}

	if embedMetadata {
		md := c.GetMetadata()
		mdStr, err := yaml.WriteYamlBytes(md)
		if err != nil {
			return err
		}

		err = tw.WriteHeader(&tar.Header{
			Name: "metadata.yaml",
			Size: int64(len(mdStr)),
			Mode: 0o666 | tar.TypeReg,
		})
		if err != nil {
			return err
		}
		_, err = tw.Write(mdStr)
		if err != nil {
			return err
		}
	}

	if err = utils.AddToTar(tw, c.getConfigPath(c.ProjectDir), yaml.FixNameExt(c.ProjectDir, ".kluctl.yml"), filter); err != nil {
		return err
	}
	if err = utils.AddToTar(tw, c.ProjectDir, "kluctl-project", filter); err != nil {
		return err
	}
	if err = utils.AddToTar(tw, c.DeploymentDir, "deployment", filter); err != nil {
		return err
	}
	if utils.Exists(c.ClustersDir) {
		if err = utils.AddToTar(tw, c.ClustersDir, "clusters", filter); err != nil {
			return err
		}
	}
	if utils.Exists(c.SealedSecretsDir) {
		if err = utils.AddToTar(tw, c.SealedSecretsDir, "sealed-secrets", filter); err != nil {
			return err
		}
	}

	return nil
}
