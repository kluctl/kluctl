package vars

import (
	"context"
	"encoding/base64"
	errors2 "errors"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/clouds/azure"
	"github.com/kluctl/kluctl/v2/pkg/clouds/gcp"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/vault"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"sort"
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
	gcp  gcp.GcpClientFactory

	credentialsCache map[string]usernamePassword
}

func NewVarsLoader(ctx context.Context, k *k8s.K8sCluster, sops *decryptor.Decryptor, rp *repocache.GitRepoCache, aws aws.AwsClientFactory, gcp gcp.GcpClientFactory) *VarsLoader {
	return &VarsLoader{
		ctx:              ctx,
		k:                k,
		sops:             sops,
		rp:               rp,
		aws:              aws,
		gcp:              gcp,
		credentialsCache: map[string]usernamePassword{},
	}
}

func (v *VarsLoader) LoadVarsList(ctx context.Context, varsCtx *VarsCtx, varsList []types.VarsSource, searchDirs []string, rootKey string) error {
	for i, _ := range varsList {
		source := &varsList[i]
		err := v.LoadVars(ctx, varsCtx, source, searchDirs, rootKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VarsLoader) LoadVars(ctx context.Context, varsCtx *VarsCtx, sourceIn *types.VarsSource, searchDirs []string, rootKey string) error {
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

	whenTrue, err := varsCtx.CheckConditional(source.When)
	if err != nil {
		return err
	}
	if !whenTrue {
		return nil
	}

	ignoreMissing := false
	if source.IgnoreMissing != nil {
		ignoreMissing = *source.IgnoreMissing
	}

	var newValue any
	var sensitive bool
	if source.Values != nil {
		if rootKey != "" {
			newValue = uo.FromMap(map[string]interface{}{
				rootKey: source.Values.Object,
			})
		} else {
			newValue = source.Values
		}
	} else if source.File != nil {
		newValue, sensitive, err = v.loadFile(varsCtx, *source.File, ignoreMissing, searchDirs)
	} else if source.Git != nil {
		newValue, sensitive, err = v.loadGit(ctx, varsCtx, source.Git, ignoreMissing)
	} else if source.GitFiles != nil {
		newValue, sensitive, err = v.loadGitFiles(ctx, varsCtx, source.GitFiles, ignoreMissing)
	} else if source.ClusterConfigMap != nil {
		newValue, err = v.loadFromK8sConfigMapOrSecret(varsCtx, *source.ClusterConfigMap, "ConfigMap", ignoreMissing, false)
	} else if source.ClusterSecret != nil {
		newValue, err = v.loadFromK8sConfigMapOrSecret(varsCtx, *source.ClusterSecret, "Secret", ignoreMissing, true)
		sensitive = true
	} else if source.ClusterObject != nil {
		newValue, err = v.loadFromK8sObject(varsCtx, *source.ClusterObject, ignoreMissing)
		sensitive = true
	} else if source.SystemEnvVars != nil {
		newValue, err = v.loadSystemEnvs(varsCtx, &source, ignoreMissing, rootKey)
		sensitive = true
	} else if source.Http != nil {
		newValue, sensitive, err = v.loadHttp(varsCtx, &source, ignoreMissing)
	} else if source.AwsSecretsManager != nil {
		newValue, err = v.loadAwsSecretsManager(varsCtx, &source, ignoreMissing)
		sensitive = true
	} else if source.GcpSecretManager != nil {
		newValue, err = v.loadGcpSecretManager(varsCtx, &source, ignoreMissing)
		sensitive = true
	} else if source.Vault != nil {
		newValue, err = v.loadVault(varsCtx, &source, ignoreMissing)
		sensitive = true
	} else if source.AzureKeyVault != nil {
		newValue, err = v.loadAzureKeyVault(varsCtx, &source, ignoreMissing)
		sensitive = true
	} else {
		return fmt.Errorf("invalid vars source")
	}
	if err != nil {
		return err
	}

	if sourceIn.Sensitive != nil {
		// override the default
		sensitive = *sourceIn.Sensitive
	}

	var newVars *uo.UnstructuredObject
	if source.TargetPath != "" {
		p, err := uo.NewMyJsonPath(source.TargetPath)
		if err != nil {
			return fmt.Errorf("failed to parse targetPath: %w", err)
		}
		newVars = uo.New()
		err = p.SetOne(newVars, newValue)
		if err != nil {
			return fmt.Errorf("failed to set value to targetPath: %w", err)
		}
	} else {
		var ok bool
		newVars, ok = newValue.(*uo.UnstructuredObject)
		if !ok {
			return fmt.Errorf("'targetPath' is required for this variable source")
		}
	}

	sourceIn.RenderedSensitive = sensitive
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

func (v *VarsLoader) loadFile(varsCtx *VarsCtx, path string, ignoreMissing bool, searchDirs []string) (*uo.UnstructuredObject, bool, error) {
	rendered, err := varsCtx.RenderFile(path, searchDirs)
	if err != nil {
		// TODO the Jinja2 renderer should be able to better report this error
		notFound := err.Error() == fmt.Sprintf("template %s not found", path) || err.Error() == fmt.Sprintf("absolute path of %s could not be resolved", path)
		if ignoreMissing && notFound {
			return uo.New(), false, nil
		}
		return nil, false, fmt.Errorf("failed to render vars file %s: %w", path, err)
	}

	format := formats.FormatForPath(path)
	decrypted, sensitive, err := sops.MaybeDecrypt(v.sops, []byte(rendered), format, format)
	if err != nil {
		return nil, false, fmt.Errorf("failed to decrypt vars file %s: %w", path, err)
	}
	rendered = string(decrypted)

	newVars := uo.New()
	err = yaml.ReadYamlString(rendered, newVars)
	if err != nil {
		return nil, false, err
	}
	if err != nil {
		return nil, false, fmt.Errorf("failed to load vars from %s: %w", path, err)
	}
	return newVars, sensitive, nil
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

func (v *VarsLoader) loadGcpSecretManager(varsCtx *VarsCtx, source *types.VarsSource, ignoreMissing bool) (*uo.UnstructuredObject, error) {
	if v.gcp == nil {
		return uo.New(), fmt.Errorf("no GCP client factory provided")
	}

	secret, err := gcp.GetGoogleSecretsManagerSecret(v.ctx, v.gcp, source.GcpSecretManager.SecretName)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			if ignoreMissing {
				return uo.New(), nil
			}
			return nil, fmt.Errorf("secret not found: %v", err)
		}
		return nil, err
	}
	return v.loadFromString(varsCtx, secret)
}

func (v *VarsLoader) loadAzureKeyVault(varsCtx *VarsCtx, source *types.VarsSource, ignoreMissing bool) (*uo.UnstructuredObject, error) {
	secret, err := azure.GetAzureKeyVaultSecret(v.ctx, source.AzureKeyVault.VaultUri, source.AzureKeyVault.SecretName)
	if err != nil {
		return nil, err
	}
	if secret == "" {
		if ignoreMissing {
			return uo.New(), nil
		}
		return nil, fmt.Errorf("the specified azure-key-vault secret was not found")
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

func (v *VarsLoader) loadGit(ctx context.Context, varsCtx *VarsCtx, gitFile *types.VarsSourceGit, ignoreMissing bool) (*uo.UnstructuredObject, bool, error) {
	ge, err := v.rp.GetEntry(gitFile.Url.String())
	if err != nil {
		return nil, false, err
	}

	if gitFile.Ref != nil && gitFile.Ref.Ref != "" {
		status.Deprecation(ctx, "git-vars-string-ref", "Passing 'ref' as string into git vars source is "+
			"deprecated and support for this will be removed in a future version of Kluctl. Please refer to the "+
			"documentation for details: https://kluctl.io/docs/kluctl/reference/templating/variable-sources/#git")
	}

	clonedDir, _, err := ge.GetClonedDir(gitFile.Ref)
	if err != nil {
		return nil, false, fmt.Errorf("failed to load vars from git repository %s: %w", gitFile.Url.String(), err)
	}

	return v.loadFile(varsCtx, gitFile.Path, ignoreMissing, []string{clonedDir})
}

func (v *VarsLoader) loadFromK8sConfigMapOrSecret(varsCtx *VarsCtx, varsSource types.VarsSourceClusterConfigMapOrSecret, kind string, ignoreMissing bool, base64Decode bool) (*uo.UnstructuredObject, error) {
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
		status.Deprecation(v.ctx, "cm-or-secret-target-path", "'targetPath' in clusterConfigMap and clusterSecret is deprecated, use the common 'targetPath' property from one level above instead.")

		p, err := uo.NewMyJsonPath(varsSource.TargetPath)
		if err != nil {
			return doError(err)
		}
		newVars := uo.New()
		err = p.SetOne(newVars, parsed)
		if err != nil {
			return doError(err)
		}
		return newVars, nil
	}
}

func (v *VarsLoader) loadFromK8sObject(varsCtx *VarsCtx, varsSource types.VarsSourceClusterObject, ignoreMissing bool) (any, error) {
	if v.k == nil {
		return nil, fmt.Errorf("loading vars from cluster is disabled")
	}

	var err error

	var gvk schema.GroupVersionKind
	if varsSource.ApiVersion != "" {
		gv, err := schema.ParseGroupVersion(varsSource.ApiVersion)
		if err != nil {
			return nil, err
		}
		gvk.Kind = varsSource.Kind
		gvk.Group = gv.Group
		gvk.Version = gv.Version
	} else {
		ars, err := v.k.GetFilteredPreferredAPIResources(func(ar *metav1.APIResource) bool {
			return ar.Kind == varsSource.Kind
		})
		if err != nil {
			return nil, err
		}
		if len(ars) == 0 {
			return nil, fmt.Errorf("no matching resource found for kind %s", varsSource.Kind)
		}
		if len(ars) != 1 {
			return nil, fmt.Errorf("more then one matching resources found for kind %s, consider also specifying apiVersion", varsSource.Kind)
		}
		gvk = schema.GroupVersionKind{
			Group:   ars[0].Group,
			Version: ars[0].Version,
			Kind:    ars[0].Kind,
		}
	}

	var objs []*uo.UnstructuredObject
	if varsSource.Name != "" {
		o, _, err := v.k.GetSingleObject(k8s2.NewObjectRef(gvk.Group, gvk.Version, gvk.Kind, varsSource.Name, varsSource.Namespace))
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if o != nil {
			objs = append(objs, o)
		}
	} else {
		objs, _, err = v.k.ListObjects(gvk, varsSource.Namespace, varsSource.Labels)
		if err != nil {
			return nil, err
		}
	}

	// we want stable sorting
	sort.Slice(objs, func(i, j int) bool {
		return objs[i].GetK8sRef().Less(objs[j].GetK8sRef())
	})

	if len(objs) == 0 {
		if ignoreMissing {
			return uo.New(), nil
		}
		return nil, fmt.Errorf("no object found")
	}
	if len(objs) != 1 {
		if !varsSource.List {
			return nil, fmt.Errorf("found more than one object")
		}
	}

	path, err := uo.NewMyJsonPath(varsSource.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid json path %s: %w", varsSource.Path, err)
	}

	var values []any
	for _, o := range objs {
		ref := o.GetK8sRef()

		doError := func(err error) (*uo.UnstructuredObject, error) {
			return nil, fmt.Errorf("failed to load vars from kubernetes object %s and json path %s: %w", ref.String(), varsSource.Path, err)
		}

		valList := path.Get(o)
		if len(valList) == 0 {
			if ignoreMissing {
				return uo.New(), nil
			}
			return doError(fmt.Errorf("json path not found"))
		} else if len(valList) > 1 {
			return doError(fmt.Errorf("json path resulted in multiple matches"))
		}
		val := valList[0]

		if varsSource.Render {
			if s, ok := val.(string); ok {
				s2, err := varsCtx.RenderString(s, nil)
				if err != nil {
					return doError(err)
				}
				val = s2
			} else if m, ok := val.(map[string]any); ok {
				_, err := varsCtx.RenderStruct(m)
				if err != nil {
					return doError(err)
				}
				val = m
			}
		}

		if varsSource.ParseYaml {
			s, ok := val.(string)
			if !ok {
				return doError(fmt.Errorf("value is not a string, but parsing YAML was requested"))
			}
			x, err := uo.FromString(s)
			if err != nil {
				return doError(err)
			}
			val = x
		}

		values = append(values, val)
	}

	if varsSource.List {
		return values, nil
	} else {
		return values[0], nil
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
