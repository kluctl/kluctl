package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/lib/git/types"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
)

type VarsSourceGit struct {
	Url  types.GitUrl  `json:"url" validate:"required"`
	Ref  *types.GitRef `json:"ref,omitempty"`
	Path string        `json:"path" validate:"required"`
}

type VarsSourceGitFiles struct {
	Url types.GitUrl  `json:"url" validate:"required"`
	Ref *types.GitRef `json:"ref,omitempty"`

	Files []GitFile `json:"files,omitempty"`
}

type GitFile struct {
	Glob         string `json:"glob" validate:"required"`
	Render       bool   `json:"render,omitempty"`
	ParseYaml    bool   `json:"parseYaml,omitempty"`
	YamlMultiDoc bool   `json:"yamlMultiDoc,omitempty"`
}

type GitFileMatch struct {
	File    GitFile               `json:"file"`
	Path    string                `json:"path"`
	Size    int32                 `json:"size"`
	Content string                `json:"content"`
	Parsed  *runtime.RawExtension `json:"parsed,omitempty"`
}

type GitFilesRefMatch struct {
	Ref    types.GitRef `json:"ref"`
	RefStr string       `json:"refStr"`

	Files       []GitFileMatch          `json:"files"`
	FilesByPath map[string]GitFileMatch `json:"filesByPath"`
	FilesTree   *uo.UnstructuredObject  `json:"filesTree"`
}

type VarsSourceClusterConfigMapOrSecret struct {
	Name       string            `json:"name,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Namespace  string            `json:"namespace" validate:"required"`
	Key        string            `json:"key" validate:"required"`
	TargetPath string            `json:"targetPath,omitempty"`
}

func ValidateVarsSourceClusterConfigMapOrSecret(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSourceClusterConfigMapOrSecret)

	if s.Name == "" && len(s.Labels) == 0 {
		sl.ReportError(s, "self", "self", "either name or labels must be set", "")
	} else if s.Name != "" && len(s.Labels) != 0 {
		sl.ReportError(s, "self", "self", "only one of name or labels can be set", "")
	}
}

type VarsSourceClusterObject struct {
	Kind       string `json:"kind" validate:"required"`
	ApiVersion string `json:"apiVersion,omitempty"`

	Namespace string `json:"namespace"`

	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	List   bool              `json:"list,omitempty"`

	Path      string `json:"path" validate:"required"`
	Render    bool   `json:"render,omitempty"`
	ParseYaml bool   `json:"parseYaml,omitempty"`
}

func ValidateVarsSourceClusterObject(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSourceClusterObject)

	if s.Name == "" && len(s.Labels) == 0 {
		sl.ReportError(s, "self", "self", "either name or labels must be set", "")
	} else if s.Name != "" && len(s.Labels) != 0 {
		sl.ReportError(s, "self", "self", "only one of name or labels can be set", "")
	}
}

type VarsSourceHttp struct {
	Url      YamlUrl           `json:"url,omitempty" validate:"required"`
	Method   *string           `json:"method,omitempty"`
	Body     *string           `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	JsonPath *string           `json:"jsonPath,omitempty"`
}

type VarsSourceAwsSecretsManager struct {
	// Name or ARN of the secret. In case a name is given, the region must be specified as well
	SecretName string `json:"secretName" validate:"required"`
	// The aws region
	Region *string `json:"region,omitempty"`
	// AWS credentials profile to use. The AWS_PROFILE environemnt variables will take precedence in case it is also set
	Profile *string `json:"profile,omitempty"`
}

type VarSourceAzureKeyVault struct {
	// Name or ARN of the secret. In case a name is given, the region must be specified as well
	VaultUri string `json:"vaultUri" validate:"required"`
	// Name of the secret
	SecretName string `json:"secretName" validate:"required"`
}

type VarsSourceGcpSecretManager struct {
	// Name of the secret. Should be provided in relative resource name format: "projects/my-project/secrets/secret/versions/latest"
	SecretName string `json:"secretName" validate:"required"`
}

type VarsSourceVault struct {
	Address string `json:"address" validate:"required"`
	Path    string `json:"path" validate:"required"`
}

type VarsSource struct {
	IgnoreMissing *bool `json:"ignoreMissing,omitempty"`
	NoOverride    *bool `json:"noOverride,omitempty"`
	Sensitive     *bool `json:"sensitive,omitempty"`

	Values            *uo.UnstructuredObject              `json:"values,omitempty" isVarsSource:"true"`
	File              *string                             `json:"file,omitempty" isVarsSource:"true"`
	Git               *VarsSourceGit                      `json:"git,omitempty" isVarsSource:"true"`
	GitFiles          *VarsSourceGitFiles                 `json:"gitFiles,omitempty" isVarsSource:"true"`
	ClusterConfigMap  *VarsSourceClusterConfigMapOrSecret `json:"clusterConfigMap,omitempty" isVarsSource:"true"`
	ClusterSecret     *VarsSourceClusterConfigMapOrSecret `json:"clusterSecret,omitempty" isVarsSource:"true"`
	ClusterObject     *VarsSourceClusterObject            `json:"clusterObject,omitempty" isVarsSource:"true"`
	SystemEnvVars     *uo.UnstructuredObject              `json:"systemEnvVars,omitempty" isVarsSource:"true"`
	Http              *VarsSourceHttp                     `json:"http,omitempty" isVarsSource:"true" isVarsSource:"true"`
	AwsSecretsManager *VarsSourceAwsSecretsManager        `json:"awsSecretsManager,omitempty" isVarsSource:"true"`
	GcpSecretManager  *VarsSourceGcpSecretManager         `json:"gcpSecretManager,omitempty" isVarsSource:"true"`
	Vault             *VarsSourceVault                    `json:"vault,omitempty" isVarsSource:"true"`
	AzureKeyVault     *VarSourceAzureKeyVault             `json:"azureKeyVault,omitempty" isVarsSource:"true"`

	TargetPath string `json:"targetPath,omitempty"`

	When string `json:"when,omitempty"`

	// these are only allowed when writing the command result
	RenderedSensitive bool                   `json:"renderedSensitive,omitempty"`
	RenderedVars      *uo.UnstructuredObject `json:"renderedVars,omitempty"`
}

func ValidateVarsSource(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSource)

	count := 0
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if f.Tag.Get("isVarsSource") == "true" && !v.Field(i).IsNil() {
			count += 1
		}
	}

	if count == 0 {
		sl.ReportError(s, "self", "self", "unknown vars source type", "")
	} else if count != 1 {
		sl.ReportError(s, "self", "self", "more then one vars source type", "")
	}
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateVarsSourceClusterConfigMapOrSecret, VarsSourceClusterConfigMapOrSecret{})
	yaml.Validator.RegisterStructValidation(ValidateVarsSourceClusterObject, VarsSourceClusterObject{})
	yaml.Validator.RegisterStructValidation(ValidateVarsSource, VarsSource{})
}
