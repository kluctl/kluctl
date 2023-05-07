package kluctl_project

import (
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"path/filepath"
)

type LoadKluctlProjectArgs struct {
	RepoRoot      string
	ProjectDir    string
	ProjectConfig string
	ExternalArgs  *uo.UnstructuredObject

	SopsDecrypter *decryptor.Decryptor
	RP            *repocache.GitRepoCache

	ClientConfigGetter func(context *string) (*rest.Config, *api.Config, error)
}

func (c *LoadedKluctlProject) getConfigPath() string {
	configPath := c.LoadArgs.ProjectConfig
	if configPath == "" {
		p := yaml.FixPathExt(filepath.Join(c.ProjectDir, ".kluctl.yml"))
		if utils.IsFile(p) {
			configPath = p
		}
	}
	return configPath
}

func (c *LoadedKluctlProject) loadKluctlProject() error {
	var err error

	c.projectRootDir = c.LoadArgs.RepoRoot
	c.ProjectDir = c.LoadArgs.ProjectDir
	err = utils.CheckInDir(c.projectRootDir, c.ProjectDir)
	if err != nil {
		return err
	}

	configPath := c.getConfigPath()

	if configPath != "" {
		err = yaml.ReadYamlFile(configPath, &c.Config)
		if err != nil {
			return err
		}
	}

	s := status.Start(c.ctx, "Loading kluctl project")
	defer s.Failed()

	c.sealedSecretsDir = filepath.Join(c.ProjectDir, ".sealed-secrets")

	sealedSecretsUsed := false
	if c.Config.SecretsConfig != nil {
		sealedSecretsUsed = true
	}
	for _, t := range c.Config.Targets {
		if t.SealingConfig != nil {
			sealedSecretsUsed = true
		}
	}
	if sealedSecretsUsed {
		status.Deprecation(c.ctx, "sealed-secrets", "The SealedSecrets integration is deprecated and will be completely removed in an upcoming version. Please switch to using the SOPS integration instead.")
	}

	s.Success()

	return nil
}
