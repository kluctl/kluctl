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
	Name      string `yaml:"name" validate:"required"`
	Namespace string `yaml:"namespace,omitempty"`
	Key       string `yaml:"key" validate:"required"`
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
	VaultAddr   string `yaml:"vaultAddr" validate:"required"`
	SecretPath  string `yaml:"secretPath" validate:"required"`
	TokenEnvVar string `yaml:"tokenEnvVar,omitempty"`
}

type VarsSource struct {
	Values            *uo.UnstructuredObject              `yaml:"values,omitempty"`
	File              *string                             `yaml:"file,omitempty"`
	Path              *string                             `yaml:"path,omitempty"`
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
	yaml.Validator.RegisterStructValidation(ValidateVarsSource, VarsSource{})
}
