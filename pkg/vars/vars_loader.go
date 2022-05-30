package vars

import (
	"context"
	"encoding/base64"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/git/repoprovider"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/aws"
	"github.com/kluctl/kluctl/v2/pkg/vars/vault"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/ioutil"
	"os"
)

type usernamePassword struct {
	username string
	password string
}

type VarsLoader struct {
	ctx context.Context
	k   *k8s.K8sCluster
	rp  repoprovider.RepoProvider
	aws aws.AwsClientFactory

	credentialsCache map[string]usernamePassword
}

func NewVarsLoader(ctx context.Context, k *k8s.K8sCluster, rp repoprovider.RepoProvider, aws aws.AwsClientFactory) *VarsLoader {
	return &VarsLoader{
		ctx:              ctx,
		k:                k,
		rp:               rp,
		aws:              aws,
		credentialsCache: map[string]usernamePassword{},
	}
}

func (v *VarsLoader) LoadVarsList(varsCtx *VarsCtx, varsList []*types.VarsSource, searchDirs []string, rootKey string) error {
	for _, source := range varsList {
		err := v.LoadVars(varsCtx, source, searchDirs, rootKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VarsLoader) LoadVars(varsCtx *VarsCtx, sourceIn *types.VarsSource, searchDirs []string, rootKey string) error {
	var source types.VarsSource

	err := varsCtx.J2.RenderStruct(&source, sourceIn, varsCtx.Vars)
	if err != nil {
		return err
	}

	if source.Values != nil {
		v.mergeVars(varsCtx, source.Values, rootKey)
		return nil
	} else if source.Path != nil {
		status.Warning(v.ctx, "'path' is deprecated as vars source, use 'file' instead")
		return v.loadFile(varsCtx, *source.Path, searchDirs, rootKey)
	} else if source.File != nil {
		return v.loadFile(varsCtx, *source.File, searchDirs, rootKey)
	} else if source.Git != nil {
		return v.loadGit(varsCtx, source.Git, rootKey)
	} else if source.ClusterConfigMap != nil {
		ref := k8s2.NewObjectRef("", "v1", "ConfigMap", source.ClusterConfigMap.Name, source.ClusterConfigMap.Namespace)
		return v.loadFromK8sObject(varsCtx, ref, source.ClusterConfigMap.Key, rootKey, false)
	} else if source.ClusterSecret != nil {
		ref := k8s2.NewObjectRef("", "v1", "Secret", source.ClusterSecret.Name, source.ClusterSecret.Namespace)
		return v.loadFromK8sObject(varsCtx, ref, source.ClusterSecret.Key, rootKey, true)
	} else if source.SystemEnvVars != nil {
		return v.loadSystemEnvs(varsCtx, &source, rootKey)
	} else if source.Http != nil {
		return v.loadHttp(varsCtx, &source, rootKey)
	} else if source.AwsSecretsManager != nil {
		return v.loadAwsSecretsManager(varsCtx, &source, rootKey)
	} else if source.Vault != nil {
		return v.loadVault(varsCtx, &source, rootKey)
	}
	return fmt.Errorf("invalid vars source")
}

func (v *VarsLoader) mergeVars(varsCtx *VarsCtx, newVars *uo.UnstructuredObject, rootKey string) {
	if rootKey == "" {
		varsCtx.Update(newVars)
	} else {
		varsCtx.UpdateChild(rootKey, newVars)
	}
}

func (v *VarsLoader) loadFile(varsCtx *VarsCtx, path string, searchDirs []string, rootKey string) error {
	var newVars uo.UnstructuredObject
	err := varsCtx.RenderYamlFile(path, searchDirs, &newVars)
	if err != nil {
		return fmt.Errorf("failed to load vars from %s: %w", path, err)
	}
	v.mergeVars(varsCtx, &newVars, rootKey)
	return nil
}

func (v *VarsLoader) loadSystemEnvs(varsCtx *VarsCtx, source *types.VarsSource, rootKey string) error {
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
	v.mergeVars(varsCtx, newVars, rootKey)
	return nil
}

func (v *VarsLoader) loadAwsSecretsManager(varsCtx *VarsCtx, source *types.VarsSource, rootKey string) error {
	if v.aws == nil {
		return fmt.Errorf("no AWS client factory provided")
	}

	secret, err := aws.GetAwsSecretsManagerSecret(v.aws, source.AwsSecretsManager.Profile, source.AwsSecretsManager.Region, source.AwsSecretsManager.SecretName)
	if err != nil {
		return err
	}
	return v.loadFromString(varsCtx, secret, rootKey)
}

func (v *VarsLoader) loadVault(varsCtx *VarsCtx, source *types.VarsSource, rootKey string) error {
	secret, err := vault.GetSecret(source.Vault.Server, source.Vault.Key)
	if err != nil {
		return err
	}
	return v.loadFromString(varsCtx, secret, rootKey)
}

func (v *VarsLoader) loadGit(varsCtx *VarsCtx, gitFile *types.VarsSourceGit, rootKey string) error {
	clonedDir, _, err := v.rp.GetClonedDir(gitFile.Url, gitFile.Ref)
	if err != nil {
		return fmt.Errorf("failed to load vars from git repository %s: %w", gitFile.Url.String(), err)
	}

	path, err := securejoin.SecureJoin(clonedDir, gitFile.Path)
	if err != nil {
		return err
	}

	f, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	return v.loadFromString(varsCtx, string(f), rootKey)
}

func (v *VarsLoader) loadFromK8sObject(varsCtx *VarsCtx, ref k8s2.ObjectRef, key string, rootKey string, base64Decode bool) error {
	if v.k == nil {
		return nil
	}

	o, _, err := v.k.GetSingleObject(ref)
	if err != nil {
		return err
	}

	f, found, err := o.GetNestedField("data", key)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("key %s not found in %s on cluster", key, ref.String())
	}

	var value string
	if b, ok := f.([]byte); ok {
		value = string(b)
	} else if s, ok := f.(string); ok {
		if base64Decode {
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return err
			}
			value = string(b)
		} else {
			value = s
		}
	}

	err = v.loadFromString(varsCtx, value, rootKey)
	if err != nil {
		return fmt.Errorf("failed to load vars from kubernetes object %s and key %s: %w", ref.String(), key, err)
	}
	return nil
}

func (v *VarsLoader) loadFromString(varsCtx *VarsCtx, s string, rootKey string) error {
	newVars := uo.New()
	err := v.renderYamlString(varsCtx, s, newVars)
	if err != nil {
		return err
	}

	if rootKey != "" {
		newVars, _, err = newVars.GetNestedObject(rootKey)
	}

	v.mergeVars(varsCtx, newVars, rootKey)
	return nil
}

func (v *VarsLoader) renderYamlString(varsCtx *VarsCtx, s string, out interface{}) error {
	ret, err := varsCtx.J2.RenderString(s, nil, varsCtx.Vars)
	if err != nil {
		return err
	}

	err = yaml.ReadYamlString(ret, out)
	if err != nil {
		return err
	}

	return nil
}
