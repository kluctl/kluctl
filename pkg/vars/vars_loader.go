package vars

import (
	"context"
	"encoding/base64"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/git/repocache"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/aws"
	"github.com/kluctl/kluctl/v2/pkg/vars/vault"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
)

type usernamePassword struct {
	username string
	password string
}

type VarsLoader struct {
	ctx context.Context
	k   *k8s.K8sCluster
	rp  *repocache.GitRepoCache
	aws aws.AwsClientFactory

	credentialsCache map[string]usernamePassword
}

func NewVarsLoader(ctx context.Context, k *k8s.K8sCluster, rp *repocache.GitRepoCache, aws aws.AwsClientFactory) *VarsLoader {
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
	err := utils.DeepCopy(&source, sourceIn)
	if err != nil {
		return err
	}

	globals, err := varsCtx.Vars.ToMap()
	if err != nil {
		return err
	}

	_, err = varsCtx.J2.RenderStruct(&source, jinja2.WithGlobals(globals))
	if err != nil {
		return err
	}

	if source.Values != nil {
		v.mergeVars(varsCtx, source.Values, rootKey)
		return nil
	} else if source.File != nil {
		return v.loadFile(varsCtx, *source.File, searchDirs, rootKey)
	} else if source.Git != nil {
		return v.loadGit(varsCtx, source.Git, rootKey)
	} else if source.ClusterConfigMap != nil {
		return v.loadFromK8sObject(varsCtx, *source.ClusterConfigMap, "ConfigMap", source.ClusterConfigMap.Key, rootKey, false)
	} else if source.ClusterSecret != nil {
		return v.loadFromK8sObject(varsCtx, *source.ClusterSecret, "Secret", source.ClusterSecret.Key, rootKey, true)
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
	newVars := uo.New()
	err := varsCtx.RenderYamlFile(path, searchDirs, newVars)
	if err != nil {
		return fmt.Errorf("failed to load vars from %s: %w", path, err)
	}
	if rootKey != "" {
		newVars, _, err = newVars.GetNestedObject(rootKey)
		if err != nil {
			return err
		}
		if newVars == nil {
			return fmt.Errorf("vars from %s have no '%s' root", path, rootKey)
		}
	}
	v.mergeVars(varsCtx, newVars, rootKey)
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
	return v.loadFromString(varsCtx, secret, "awsSecretsManager", rootKey)
}

func (v *VarsLoader) loadVault(varsCtx *VarsCtx, source *types.VarsSource, rootKey string) error {
	secret, err := vault.GetSecret(source.Vault.Address, source.Vault.Path)
	if err != nil {
		return err
	}
	return v.loadFromString(varsCtx, secret, "vault", rootKey)
}

func (v *VarsLoader) loadGit(varsCtx *VarsCtx, gitFile *types.VarsSourceGit, rootKey string) error {
	ge, err := v.rp.GetEntry(gitFile.Url)
	if err != nil {
		return err
	}

	clonedDir, _, err := ge.GetClonedDir(gitFile.Ref)
	if err != nil {
		return fmt.Errorf("failed to load vars from git repository %s: %w", gitFile.Url.String(), err)
	}

	path, err := securejoin.SecureJoin(clonedDir, gitFile.Path)
	if err != nil {
		return err
	}

	f, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return v.loadFromString(varsCtx, string(f), "git", rootKey)
}

func (v *VarsLoader) loadFromK8sObject(varsCtx *VarsCtx, varsSource types.VarsSourceClusterConfigMapOrSecret, kind string, key string, rootKey string, base64Decode bool) error {
	if v.k == nil {
		return fmt.Errorf("loading vars from cluster is disabled")
	}

	var err error
	var o *uo.UnstructuredObject

	if varsSource.Name != "" {
		o, _, err = v.k.GetSingleObject(k8s2.NewObjectRef("", "v1", kind, varsSource.Name, varsSource.Namespace))
		if err != nil {
			return err
		}
	} else {
		objs, _, err := v.k.ListObjects(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    kind,
		}, varsSource.Namespace, varsSource.Labels)
		if err != nil {
			return err
		}
		if len(objs) == 0 {
			return fmt.Errorf("no object found with labels %v", varsSource.Labels)
		}
		if len(objs) > 1 {
			return fmt.Errorf("found more than one objects with labels %v", varsSource.Labels)
		}
		o = objs[0]
	}

	ref := o.GetK8sRef()

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

	err = v.loadFromString(varsCtx, value, "k8s", rootKey)
	if err != nil {
		return fmt.Errorf("failed to load vars from kubernetes object %s and key %s: %w", ref.String(), key, err)
	}
	return nil
}

func (v *VarsLoader) loadFromString(varsCtx *VarsCtx, s string, secretType string, rootKey string) error {
	newVars := uo.New()
	err := v.renderYamlString(varsCtx, s, newVars)
	if err != nil {
		return err
	}

	if rootKey != "" {
		newVars, _, err = newVars.GetNestedObject(rootKey)
		if err != nil {
			return err
		}
		if newVars == nil {
			return fmt.Errorf("%s secret has no '%s' root", secretType, rootKey)
		}
	}

	v.mergeVars(varsCtx, newVars, rootKey)
	return nil
}

func (v *VarsLoader) renderYamlString(varsCtx *VarsCtx, s string, out interface{}) error {
	ret, err := varsCtx.RenderString(s)
	if err != nil {
		return err
	}

	err = yaml.ReadYamlString(ret, out)
	if err != nil {
		return err
	}

	return nil
}
