package kluctl_project

import (
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/aws"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"os"
	"path/filepath"
)

type usernamePassword struct {
	username string
	password string
}

type SecretsLoader struct {
	project *LoadedKluctlProject

	credentialsCache map[string]usernamePassword
}

func NewSecretsLoader(p *LoadedKluctlProject) *SecretsLoader {
	return &SecretsLoader{
		project:          p,
		credentialsCache: map[string]usernamePassword{},
	}
}

func (s *SecretsLoader) LoadSecrets(source *types.VarsSource) (*uo.UnstructuredObject, error) {
	if source.Path != nil {
		return s.loadSecretsFile(source)
	} else if source.SystemEnvVars != nil {
		return s.loadSecretsSystemEnvs(source)
	} else if source.Http != nil {
		return s.loadSecretsHttp(source)
	} else if source.AwsSecretsManager != nil {
		return s.loadSecretsAwsSecretsManager(source)
	} else {
		return nil, fmt.Errorf("invalid secrets entry")
	}
}

func (s *SecretsLoader) loadSecretsFile(source *types.VarsSource) (*uo.UnstructuredObject, error) {
	var p string
	var err error
	if utils.Exists(filepath.Join(s.project.DeploymentDir, *source.Path)) {
		p, err = securejoin.SecureJoin(s.project.DeploymentDir, *source.Path)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("secrets file %s does not exist", *source.Path)
	}

	err = utils.CheckInDir(s.project.DeploymentDir, p)
	if err != nil {
		return nil, fmt.Errorf("secrets file %s is not part of the deployment project: %w", *source.Path, err)
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

func (s *SecretsLoader) loadSecretsSystemEnvs(source *types.VarsSource) (*uo.UnstructuredObject, error) {
	secrets := uo.New()
	err := source.SystemEnvVars.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		envName, ok := it.Value().(string)
		if !ok {
			return fmt.Errorf("value at %s is not a string", it.KeyPath().ToJsonPath())
		}
		envValue, ok := os.LookupEnv(envName)
		if !ok {
			return fmt.Errorf("environment variable %s not found for secret %s", envName, it.KeyPath().ToJsonPath())
		}
		err := secrets.SetNestedField(envValue, it.KeyPath()...)
		if err != nil {
			return fmt.Errorf("failed to set secret %s: %w", it.KeyPath().ToJsonPath(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return secrets, nil
}

func (s *SecretsLoader) loadSecretsAwsSecretsManager(source *types.VarsSource) (*uo.UnstructuredObject, error) {
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
