package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"reflect"
)

type VarsSourceGit struct {
	Url  GitUrl  `json:"url" validate:"required"`
	Ref  *GitRef `json:"ref,omitempty"`
	Path string  `json:"path" validate:"required"`
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

	Namespace string `json:"namespace" validate:"required"`

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

	Values            *uo.UnstructuredObject              `json:"values,omitempty"`
	File              *string                             `json:"file,omitempty"`
	Git               *VarsSourceGit                      `json:"git,omitempty"`
	ClusterConfigMap  *VarsSourceClusterConfigMapOrSecret `json:"clusterConfigMap,omitempty"`
	ClusterSecret     *VarsSourceClusterConfigMapOrSecret `json:"clusterSecret,omitempty"`
	ClusterObject     *VarsSourceClusterObject            `json:"clusterObject,omitempty"`
	SystemEnvVars     *uo.UnstructuredObject              `json:"systemEnvVars,omitempty"`
	Http              *VarsSourceHttp                     `json:"http,omitempty"`
	AwsSecretsManager *VarsSourceAwsSecretsManager        `json:"awsSecretsManager,omitempty"`
	GcpSecretManager  *VarsSourceGcpSecretManager         `json:"gcpSecretManager,omitempty"`
	Vault             *VarsSourceVault                    `json:"vault,omitempty"`
	AzureKeyVault     *VarSourceAzureKeyVault             `json:"azureKeyVault,omitempty"`

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
		switch v.Type().Field(i).Name {
		case "IgnoreMissing", "NoOverride", "When", "RenderedSensitive", "RenderedVars":
			continue
		}
		if !v.Field(i).IsNil() {
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
