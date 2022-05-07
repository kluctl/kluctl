package kluctl_project

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (c *LoadedKluctlProject) loadFromArchive() error {
	var dir string
	if utils.IsFile(c.loadArgs.FromArchive) {
		dir = filepath.Join(c.TmpDir, "archive")
		err := utils.ExtractTarGzFile(c.loadArgs.FromArchive, dir)
		if err != nil {
			return fmt.Errorf("failed to extract archive %v: %w", c.loadArgs.FromArchive, err)
		}
	} else {
		dir = c.loadArgs.FromArchive
	}

	var pmdPath string
	if c.loadArgs.FromArchiveMetadata != "" {
		pmdPath = c.loadArgs.FromArchiveMetadata
	} else {
		pmdPath = yaml.FixPathExt(filepath.Join(dir, "project-metadata.yml"))
	}

	var pmd types2.ProjectMetadata
	err := yaml.ReadYamlFile(pmdPath, &pmd)
	if err != nil {
		return err
	}

	var amd types2.ArchiveMetadata
	err = yaml.ReadYamlFile(yaml.FixPathExt(filepath.Join(dir, "archive-metadata.yml")), &amd)
	if err != nil {
		return err
	}

	c.loadArgs.RepoRoot = filepath.Join(dir, amd.ProjectRootDir)
	c.loadArgs.ProjectDir = filepath.Join(dir, amd.ProjectRootDir, amd.ProjectSubDir)

	err = utils.CheckInDir(dir, c.loadArgs.RepoRoot)
	if err != nil {
		return err
	}
	err = utils.CheckInDir(dir, c.loadArgs.ProjectDir)
	if err != nil {
		return err
	}

	c.archiveDir = dir
	c.involvedRepos = pmd.InvolvedRepos
	c.DynamicTargets = pmd.Targets

	err = c.loadKluctlProject()
	if err != nil {
		return err
	}
	return nil
}

func (c *LoadedKluctlProject) WriteArchive(archivePath string, embedProjectMetadata bool) error {
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

	if embedProjectMetadata {
		err = yaml.WriteYamlToTar(tw, c.GetMetadata(), "project-metadata.yaml")
		if err != nil {
			return nil
		}
	}

	amd := types2.ArchiveMetadata{
		ProjectRootDir: "kluctl-project",
	}

	amd.ProjectSubDir, err = filepath.Rel(c.projectRootDir, c.ProjectDir)
	if err != nil {
		return err
	}

	err = yaml.WriteYamlToTar(tw, &amd, "archive-metadata.yaml")
	if err != nil {
		return nil
	}

	if err = utils.AddToTar(tw, c.projectRootDir, "kluctl-project", filter); err != nil {
		return err
	}
	gitReposBaseDir := filepath.Join(c.TmpDir, "git")
	if utils.Exists(gitReposBaseDir) {
		err = utils.AddToTar(tw, gitReposBaseDir, "git", filter)
		if err != nil {
			return err
		}
	}

	return nil
}
