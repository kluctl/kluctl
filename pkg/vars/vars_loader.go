package vars

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/aws"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"os"
)

type usernamePassword struct {
	username string
	password string
}

type VarsLoader struct {
	k          *k8s.K8sCluster
	varsCtx    *VarsCtx
	searchDirs []string
	rootKey    string

	credentialsCache map[string]usernamePassword
}

func NewVarsLoader(k *k8s.K8sCluster, varsCtx *VarsCtx, searchDirs []string, rootKey string) *VarsLoader {
	return &VarsLoader{
		k:                k,
		varsCtx:          varsCtx,
		searchDirs:       searchDirs,
		rootKey:          rootKey,
		credentialsCache: map[string]usernamePassword{},
	}
}

func (s *VarsLoader) LoadVarsList(varsList []*types.VarsSource) error {
	for _, source := range varsList {
		err := s.LoadVars(source)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *VarsLoader) LoadVars(sourceIn *types.VarsSource) error {
	var source types.VarsSource

	err := s.varsCtx.J2.RenderStruct(&source, sourceIn, s.varsCtx.Vars)
	if err != nil {
		return err
	}

	if source.Path != nil {
		return s.loadFile(&source)
	} else if source.SystemEnvVars != nil {
		return s.loadSystemEnvs(&source)
	} else if source.Http != nil {
		return s.loadHttp(&source)
	} else if source.AwsSecretsManager != nil {
		return s.loadAwsSecretsManager(&source)
	} else {
		return fmt.Errorf("invalid vars source")
	}
}

func (s *VarsLoader) mergeVars(newVars *uo.UnstructuredObject) {
	if s.rootKey == "" {
		s.varsCtx.Update(newVars)
	} else {
		s.varsCtx.UpdateChild(s.rootKey, newVars)
	}
}

func (s *VarsLoader) loadFile(source *types.VarsSource) error {
	var newVars uo.UnstructuredObject
	err := s.varsCtx.RenderYamlFile(*source.Path, s.searchDirs, &newVars)
	if err != nil {
		return fmt.Errorf("failed to load vars from %s: %w", *source.Path, err)
	}
	s.mergeVars(&newVars)
	return nil
}

func (s *VarsLoader) loadSystemEnvs(source *types.VarsSource) error {
	newVars := uo.New()
	err := source.SystemEnvVars.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		envName, ok := it.Value().(string)
		if !ok {
			return fmt.Errorf("value at %s is not a string", it.KeyPath().ToJsonPath())
		}
		envValue, ok := os.LookupEnv(envName)
		if !ok {
			return fmt.Errorf("environment variable %s not found for %s", envName, it.KeyPath().ToJsonPath())
		}
		err := newVars.SetNestedField(envValue, it.KeyPath()...)
		if err != nil {
			return fmt.Errorf("failed to set value for %s: %w", it.KeyPath().ToJsonPath(), err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.mergeVars(newVars)
	return nil
}

func (s *VarsLoader) loadAwsSecretsManager(source *types.VarsSource) error {
	secret, err := aws.GetAwsSecretsManagerSecret(source.AwsSecretsManager.Profile, source.AwsSecretsManager.Region, source.AwsSecretsManager.SecretName)
	if err != nil {
		return err
	}

	newVars, err := uo.FromString(secret)
	if err != nil {
		return fmt.Errorf("failed to parse yaml from AWS Secrets Manager (secretName=%s): %w", source.AwsSecretsManager.SecretName, err)
	}
	if s.rootKey != "" {
		newVars, _, err = newVars.GetNestedObject(s.rootKey)
	}
	if newVars != nil {
		s.mergeVars(newVars)
	}
	return nil
}
