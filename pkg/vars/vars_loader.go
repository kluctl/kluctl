package vars

import (
	"context"
	"encoding/base64"
	errors2 "errors"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/aws"
	"github.com/kluctl/kluctl/v2/pkg/vars/vault"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"go.mozilla.org/sops/v3/cmd/sops/formats"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"strings"
)

type usernamePassword struct {
	username string
	password string
}

type VarsLoader struct {
	ctx  context.Context
	k    *k8s.K8sCluster
	sops *decryptor.Decryptor
	rp   *repocache.GitRepoCache
	aws  aws.AwsClientFactory

	credentialsCache map[string]usernamePassword
}

func NewVarsLoader(ctx context.Context, k *k8s.K8sCluster, sops *decryptor.Decryptor, rp *repocache.GitRepoCache, aws aws.AwsClientFactory) *VarsLoader {
	return &VarsLoader{
		ctx:              ctx,
		k:                k,
		sops:             sops,
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
	if sourceIn.RenderedVars != nil && len(sourceIn.RenderedVars.Object) != 0 {
		return fmt.Errorf("renderedVars is not allowed here")
	}

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

	if source.When != "" {
		r, err := kluctl_jinja2.RenderConditional(varsCtx.J2, globals, source.When)
		if err != nil {
			return err
		}
		if !kluctl_jinja2.IsConditionalTrue(r) {
			return nil
		}
	}

	ignoreMissing := false
	if source.IgnoreMissing != nil {
		ignoreMissing = *source.IgnoreMissing
	}

	var newVars *uo.UnstructuredObject
	if source.Values != nil {
		newVars = source.Values
		if rootKey != "" {
			newVars = uo.FromMap(map[string]interface{}{
				rootKey: newVars.Object,
			})
		}
	} else if source.File != nil {
		newVars, err = v.loadFile(varsCtx, *source.File, ignoreMissing, searchDirs)
	} else if source.Git != nil {
		newVars, err = v.loadGit(varsCtx, source.Git, ignoreMissing)
	} else if source.ClusterConfigMap != nil {
		newVars, err = v.loadFromK8sObject(varsCtx, *source.ClusterConfigMap, "ConfigMap", ignoreMissing, false)
	} else if source.ClusterSecret != nil {
		newVars, err = v.loadFromK8sObject(varsCtx, *source.ClusterSecret, "Secret", ignoreMissing, true)
	} else if source.SystemEnvVars != nil {
		newVars, err = v.loadSystemEnvs(varsCtx, &source, ignoreMissing, rootKey)
	} else if source.Http != nil {
		newVars, err = v.loadHttp(varsCtx, &source, ignoreMissing)
	} else if source.AwsSecretsManager != nil {
		newVars, err = v.loadAwsSecretsManager(varsCtx, &source, ignoreMissing)
	} else if source.Vault != nil {
		newVars, err = v.loadVault(varsCtx, &source, ignoreMissing)
	} else {
		return fmt.Errorf("invalid vars source")
	}
	if err != nil {
		return err
	}

	sourceIn.RenderedVars = newVars.Clone()

	if source.NoOverride == nil || !*source.NoOverride {
		varsCtx.Vars.Merge(newVars)
	} else {
		newVars.Merge(varsCtx.Vars)
		varsCtx.Vars = newVars
	}

	return nil
}

func (v *VarsLoader) mergeVars(varsCtx *VarsCtx, newVars *uo.UnstructuredObject, rootKey string) {
	if rootKey == "" {
		varsCtx.Update(newVars)
	} else {
		varsCtx.UpdateChild(rootKey, newVars)
	}
}

func (v *VarsLoader) loadFile(varsCtx *VarsCtx, path string, ignoreMissing bool, searchDirs []string) (*uo.UnstructuredObject, error) {
	rendered, err := varsCtx.RenderFile(path, searchDirs)
	if err != nil {
		// TODO the Jinja2 renderer should be able to better report this error
		if ignoreMissing && err.Error() == fmt.Sprintf("template %s not found", path) {
			return uo.New(), nil
		}
		return nil, fmt.Errorf("failed to render vars file %s: %w", path, err)
	}

	format := formats.FormatForPath(path)
	decrypted, _, err := sops.MaybeDecrypt(v.sops, []byte(rendered), format, format)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vars file %s: %w", path, err)
	}
	rendered = string(decrypted)

	newVars := uo.New()
	err = yaml.ReadYamlString(rendered, newVars)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load vars from %s: %w", path, err)
	}
	return newVars, nil
}

func (v *VarsLoader) loadSystemEnvs(varsCtx *VarsCtx, source *types.VarsSource, ignoreMissing bool, rootKey string) (*uo.UnstructuredObject, error) {
	newVars := uo.New()
	err := source.SystemEnvVars.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		envName, ok := it.Value().(string)
		if !ok {
			return fmt.Errorf("value at %s is not a string", it.KeyPath().ToJsonPath())
		}
		var defaultValue string
		hasDefaultValue := false
		if strings.IndexRune(envName, ':') != -1 {
			s := strings.SplitN(envName, ":", 2)
			envName = s[0]
			defaultValue = s[1]
			hasDefaultValue = true
		}
		envValueStr := ""
		if v, ok := os.LookupEnv(envName); ok {
			envValueStr = v
		} else if hasDefaultValue {
			envValueStr = defaultValue
			if envValueStr == "" {
				// treat empty default string as literal empty string instead of treating it as nil
				envValueStr = `""`
			}
		} else {
			if ignoreMissing {
				return nil
			}
			return fmt.Errorf("environment variable %s not found for %s", envName, it.KeyPath().ToJsonPath())
		}

		var envValue any
		err := yaml.ReadYamlString(envValueStr, &envValue)
		if err != nil {
			return fmt.Errorf("failed to parse env value '%s': %w", envValueStr, err)
		}

		err = newVars.SetNestedField(envValue, it.KeyPath()...)
		if err != nil {
			return fmt.Errorf("failed to set value for %s: %w", it.KeyPath().ToJsonPath(), err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if rootKey != "" {
		newVars = uo.FromMap(map[string]interface{}{
			rootKey: newVars.Object,
		})
	}
	return newVars, nil
}

func (v *VarsLoader) loadAwsSecretsManager(varsCtx *VarsCtx, source *types.VarsSource, ignoreMissing bool) (*uo.UnstructuredObject, error) {
	if v.aws == nil {
		return uo.New(), fmt.Errorf("no AWS client factory provided")
	}

	secret, err := aws.GetAwsSecretsManagerSecret(v.ctx, v.aws, source.AwsSecretsManager.Profile, source.AwsSecretsManager.Region, source.AwsSecretsManager.SecretName)
	if err != nil {
		var aerr *types2.ResourceNotFoundException
		if errors2.As(err, &aerr) {
			if ignoreMissing {
				return uo.New(), nil
			}
		}
		return nil, err
	}
	return v.loadFromString(varsCtx, secret)
}

func (v *VarsLoader) loadVault(varsCtx *VarsCtx, source *types.VarsSource, ignoreMissing bool) (*uo.UnstructuredObject, error) {
	secret, err := vault.GetSecret(source.Vault.Address, source.Vault.Path)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		if ignoreMissing {
			return uo.New(), nil
		}
		return nil, fmt.Errorf("the specified vault secret was not found")
	}
	return v.loadFromString(varsCtx, *secret)
}

func (v *VarsLoader) loadGit(varsCtx *VarsCtx, gitFile *types.VarsSourceGit, ignoreMissing bool) (*uo.UnstructuredObject, error) {
	ge, err := v.rp.GetEntry(gitFile.Url)
	if err != nil {
		return nil, err
	}

	clonedDir, _, err := ge.GetClonedDir(gitFile.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to load vars from git repository %s: %w", gitFile.Url.String(), err)
	}

	return v.loadFile(varsCtx, gitFile.Path, ignoreMissing, []string{clonedDir})
}

func (v *VarsLoader) loadFromK8sObject(varsCtx *VarsCtx, varsSource types.VarsSourceClusterConfigMapOrSecret, kind string, ignoreMissing bool, base64Decode bool) (*uo.UnstructuredObject, error) {
	if v.k == nil {
		return nil, fmt.Errorf("loading vars from cluster is disabled")
	}

	var err error
	var o *uo.UnstructuredObject

	if varsSource.Name != "" {
		o, _, err = v.k.GetSingleObject(k8s2.NewObjectRef("", "v1", kind, varsSource.Name, varsSource.Namespace))
		if err != nil {
			if ignoreMissing && errors.IsNotFound(err) {
				return uo.New(), nil
			}
			return nil, err
		}
	} else {
		objs, _, err := v.k.ListObjects(schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    kind,
		}, varsSource.Namespace, varsSource.Labels)
		if err != nil {
			return nil, err
		}
		if len(objs) == 0 {
			if ignoreMissing {
				return uo.New(), nil
			}
			return nil, fmt.Errorf("no object found with labels %v", varsSource.Labels)
		}
		if len(objs) > 1 {
			return nil, fmt.Errorf("found more than one objects with labels %v", varsSource.Labels)
		}
		o = objs[0]
	}

	ref := o.GetK8sRef()

	f, found, err := o.GetNestedField("data", varsSource.Key)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("key %s not found in %s on cluster", varsSource.Key, ref.String())
	}

	var value string
	if b, ok := f.([]byte); ok {
		value = string(b)
	} else if s, ok := f.(string); ok {
		if base64Decode {
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, err
			}
			value = string(b)
		} else {
			value = s
		}
	}

	doError := func(err error) (*uo.UnstructuredObject, error) {
		return nil, fmt.Errorf("failed to load vars from kubernetes object %s and key %s: %w", ref.String(), varsSource.Key, err)
	}

	var parsed any
	err = v.renderYamlString(varsCtx, value, &parsed)
	if err != nil {
		return doError(err)
	}

	if varsSource.TargetPath == "" {
		m, ok := parsed.(map[string]any)
		if !ok {
			return doError(fmt.Errorf("value is not a YAML dictionary"))
		}
		return uo.FromMap(m), nil
	} else {
		p, err := uo.NewMyJsonPath(varsSource.TargetPath)
		if err != nil {
			return doError(err)
		}
		newVars := uo.New()
		err = p.Set(newVars, parsed)
		if err != nil {
			return doError(err)
		}
		return newVars, nil
	}
}

func (v *VarsLoader) loadFromString(varsCtx *VarsCtx, s string) (*uo.UnstructuredObject, error) {
	newVars := uo.New()
	err := v.renderYamlString(varsCtx, s, newVars)
	if err != nil {
		return nil, err
	}
	return newVars, nil
}

func (v *VarsLoader) renderYamlString(varsCtx *VarsCtx, s string, out interface{}) error {
	ret, err := varsCtx.RenderString(s, nil)
	if err != nil {
		return err
	}

	err = yaml.ReadYamlString(ret, out)
	if err != nil {
		return err
	}

	return nil
}
