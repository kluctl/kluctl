package kluctl_project

import (
	"context"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
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

	GitRP *repocache.GitRepoCache
	OciRP *repocache.OciRepoCache

	OciAuthProvider  auth_provider.OciAuthProvider
	HelmAuthProvider helm_auth.HelmAuthProvider

	AddKeyServersFunc  func(ctx context.Context, d *decryptor.Decryptor) error
	ClientConfigGetter func(context *string) (*rest.Config, *api.Config, error)
}

func (c *LoadedKluctlProject) getConfigPath() string {
	configPath := c.LoadArgs.ProjectConfig
	if configPath == "" {
		p := yaml.FixPathExt(filepath.Join(c.LoadArgs.ProjectDir, ".kluctl.yml"))
		if utils.IsFile(p) {
			configPath = p
		}
	}
	return configPath
}

func (c *LoadedKluctlProject) loadKluctlProject(ctx context.Context) error {
	var err error

	if c.LoadArgs.RepoRoot != "" {
		err = utils.CheckInDir(c.LoadArgs.RepoRoot, c.LoadArgs.ProjectDir)
		if err != nil {
			return err
		}
	}

	configPath := c.getConfigPath()

	if configPath != "" {
		err = yaml.ReadYamlFile(configPath, &c.Config)
		if err != nil {
			return err
		}
	}

	return nil
}
