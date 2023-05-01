package types

import (
	"github.com/go-playground/validator/v10"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"reflect"
)

type VarsSourceGit struct {
	Url  git_url.GitUrl `json:"url" validate:"required"`
	Ref  string         `json:"ref,omitempty"`
	Path string         `json:"path" validate:"required"`
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

type VarsSourceVault struct {
	Address string `json:"address" validate:"required"`
	Path    string `json:"path" validate:"required"`
}

type VarsSource struct {
	IgnoreMissing *bool `json:"ignoreMissing,omitempty"`
	NoOverride    *bool `json:"noOverride,omitempty"`

	Values            *uo.UnstructuredObject              `json:"values,omitempty"`
	File              *string                             `json:"file,omitempty"`
	Git               *VarsSourceGit                      `json:"git,omitempty"`
	ClusterConfigMap  *VarsSourceClusterConfigMapOrSecret `json:"clusterConfigMap,omitempty"`
	ClusterSecret     *VarsSourceClusterConfigMapOrSecret `json:"clusterSecret,omitempty"`
	SystemEnvVars     *uo.UnstructuredObject              `json:"systemEnvVars,omitempty"`
	Http              *VarsSourceHttp                     `json:"http,omitempty"`
	AwsSecretsManager *VarsSourceAwsSecretsManager        `json:"awsSecretsManager,omitempty"`
	Vault             *VarsSourceVault                    `json:"vault,omitempty"`

	When string `json:"when,omitempty"`
}

func ValidateVarsSource(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsSource)

	count := 0
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		switch v.Type().Field(i).Name {
		case "IgnoreMissing", "NoOverride", "When":
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
