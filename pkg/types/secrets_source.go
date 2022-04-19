package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type SecretSourceAwsSecretsManager struct {
	// Name or ARN of the secret. In case a name is given, the region must be specified as well
	SecretName string `yaml:"secretName" validate:"required"`
	// The aws region
	Region *string `yaml:"region,omitempty"`
	// AWS credentials profile to use. The AWS_PROFILE environemnt variables will take precedence in case it is also set
	Profile *string `yaml:"profile,omitempty"`
}

type SecretSource struct {
	Path              *string                        `yaml:"path,omitempty"`
	SystemEnvVars     *uo.UnstructuredObject         `yaml:"systemEnvVars,omitempty"`
	AwsSecretsManager *SecretSourceAwsSecretsManager `yaml:"awsSecretsManager,omitempty"`
}

func ValidateSecretSource(sl validator.StructLevel) {
	s := sl.Current().Interface().(SecretSource)
	count := 0
	if s.Path != nil {
		count += 1
	}
	if s.SystemEnvVars != nil {
		count += 1
	}
	if s.AwsSecretsManager != nil {
		count += 1
	}
	if count == 0 {
		sl.ReportError(s, "self", "self", "invalidsource", "unknown secret source type")
	} else if count != 1 {
		sl.ReportError(s, "self", "self", "invalidsource", "more then one secret source type")
	}
}
