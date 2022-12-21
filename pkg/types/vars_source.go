package types

import (
	"github.com/go-playground/validator/v10"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"reflect"
)

type VarsSourceGit struct {
	Url  git_url.GitUrl `yaml:"url" validate:"required"`
	Ref  string         `yaml:"ref,omitempty"`
	Path string         `yaml:"path" validate:"required"`
}

type VarsSourceClusterConfigMapOrSecret struct {
	Name      string            `yaml:"name,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Namespace string            `yaml:"namespace" validate:"required"`
	Key       string            `yaml:"key" validate:"required"`
}

func ValidateVarsSourceClusterConfigMapOrSecret(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSourceClusterConfigMapOrSecret)

	if s.Name == "" && len(s.Labels) == 0 {
		sl.ReportError(s, "self", "self", "either name or labels must be set", "")
	} else if s.Name != "" && len(s.Labels) != 0 {
		sl.ReportError(s, "self", "self", "only one of name or labels can be set", "")
	}
}

type VarsSourceHttp struct {
	Url      YamlUrl           `yaml:"url,omitempty" validate:"required"`
	Method   *string           `yaml:"method,omitempty"`
	Body     *string           `yaml:"body,omitempty"`
	Headers  map[string]string `yaml:"headers,omitempty"`
	JsonPath *string           `yaml:"jsonPath,omitempty"`
}

type VarsSourceAwsSecretsManager struct {
	// Name or ARN of the secret. In case a name is given, the region must be specified as well
	SecretName string `yaml:"secretName" validate:"required"`
	// The aws region
	Region *string `yaml:"region,omitempty"`
	// AWS credentials profile to use. The AWS_PROFILE environemnt variables will take precedence in case it is also set
	Profile *string `yaml:"profile,omitempty"`
}

type VarsSourceVault struct {
	Address string `yaml:"address" validate:"required"`
	Path    string `yaml:"path" validate:"required"`
}

type VarsSource struct {
	IgnoreMissing *bool `yaml:"ignoreMissing,omitempty"`
	NoOverride    *bool `yaml:"noOverride,omitempty"`

	Values            *uo.UnstructuredObject              `yaml:"values,omitempty"`
	File              *string                             `yaml:"file,omitempty"`
	Git               *VarsSourceGit                      `yaml:"git,omitempty"`
	ClusterConfigMap  *VarsSourceClusterConfigMapOrSecret `yaml:"clusterConfigMap,omitempty"`
	ClusterSecret     *VarsSourceClusterConfigMapOrSecret `yaml:"clusterSecret,omitempty"`
	SystemEnvVars     *uo.UnstructuredObject              `yaml:"systemEnvVars,omitempty"`
	Http              *VarsSourceHttp                     `yaml:"http,omitempty"`
	AwsSecretsManager *VarsSourceAwsSecretsManager        `yaml:"awsSecretsManager,omitempty"`
	Vault             *VarsSourceVault                    `yaml:"vault,omitempty"`
}

func ValidateVarsSource(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSource)

	count := 0
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "IgnoreMissing" || v.Type().Field(i).Name == "NoOverride" {
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
	yaml.Validator.RegisterStructValidation(ValidateVarsSource, VarsSource{})
}
