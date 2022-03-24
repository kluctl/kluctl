package seal

import (
	"fmt"
	"github.com/kluctl/kluctl/pkg/kluctl_project"
	"github.com/kluctl/kluctl/pkg/types"
	"github.com/kluctl/kluctl/pkg/utils"
	"github.com/kluctl/kluctl/pkg/utils/aws"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	"os"
	"path/filepath"
	"strings"
)

type SecretsLoader struct {
	project    *kluctl_project.KluctlProjectContext
	secretsDir string
}

func NewSecretsLoader(p *kluctl_project.KluctlProjectContext, secretsDir string) *SecretsLoader {
	return &SecretsLoader{
		project:    p,
		secretsDir: secretsDir,
	}
}

func (s *SecretsLoader) LoadSecrets(source *types.SecretSource) (*uo.UnstructuredObject, error) {
	if source.Path != nil {
		return s.loadSecretsFile(source)
	} else if source.SystemEnvVars != nil {
		return s.loadSecretsSystemEnvs(source)
	} else if source.AwsSecretsManager != nil {
		return s.loadSecretsAwsSecretsManager(source)
	} else {
		return nil, fmt.Errorf("invalid secrets entry")
	}
}

func (s *SecretsLoader) loadSecretsFile(source *types.SecretSource) (*uo.UnstructuredObject, error) {
	var p string
	if utils.Exists(filepath.Join(s.project.DeploymentDir, *source.Path)) {
		p = filepath.Join(s.project.DeploymentDir, *source.Path)
	} else if utils.Exists(filepath.Join(s.secretsDir, *source.Path)) {
		p = filepath.Join(s.secretsDir, *source.Path)
	}
	if p == "" || !utils.Exists(p) {
		return nil, fmt.Errorf("secrets file %s does not exist", *source.Path)
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(abs, s.project.DeploymentDir) {
		return nil, fmt.Errorf("secrets file %s is not part of the deployment project", *source.Path)
	}

	secrets, err := uo.FromFile(p)
	if err != nil {
		return nil, err
	}
	secrets, ok, err := secrets.GetNestedObject("secrets")
	if err != nil {
		return nil, err
	}
	if !ok {
		return uo.New(), nil
	}
	return secrets, nil
}

func (s *SecretsLoader) loadSecretsSystemEnvs(source *types.SecretSource) (*uo.UnstructuredObject, error) {
	secrets := uo.New()
	err := source.SystemEnvVars.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		envName, ok := it.Value().(string)
		if !ok {
			return fmt.Errorf("value at %s is not a string", it.JsonPath())
		}
		envValue, ok := os.LookupEnv(envName)
		if !ok {
			return fmt.Errorf("environment variable %s not found for secret %s", envName, it.JsonPath())
		}
		err := secrets.SetNestedField(envValue, it.KeyPath()...)
		if err != nil {
			return fmt.Errorf("failed to set secret %s: %w", it.JsonPath(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

func (s *SecretsLoader) loadSecretsAwsSecretsManager(source *types.SecretSource) (*uo.UnstructuredObject, error) {
	secret, err := aws.GetAwsSecretsManagerSecret(source.AwsSecretsManager.Profile, source.AwsSecretsManager.Region, source.AwsSecretsManager.SecretName)
	if err != nil {
		return nil, err
	}

	secrets, err := uo.FromString(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to parse yaml from AWS Secrets Manager (secretName=%s): %w", source.AwsSecretsManager.SecretName, err)
	}
	secrets, ok, err := secrets.GetNestedObject("secrets")
	if err != nil {
		return nil, err
	}
	if !ok {
		return uo.New(), nil
	}
	return secrets, nil
}
