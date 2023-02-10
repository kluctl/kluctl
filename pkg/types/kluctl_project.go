package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type DynamicArg struct {
	Name string `yaml:"name" validate:"required"`
}

type SealingConfig struct {
	Args       *uo.UnstructuredObject `yaml:"args,omitempty"`
	SecretSets []string               `yaml:"secretSets,omitempty"`
	CertFile   *string                `yaml:"certFile,omitempty"`
}

type Target struct {
	Name          string                 `yaml:"name" validate:"required"`
	Context       *string                `yaml:"context,omitempty"`
	Args          *uo.UnstructuredObject `yaml:"args,omitempty"`
	DynamicArgs   []DynamicArg           `yaml:"dynamicArgs,omitempty"`
	SealingConfig *SealingConfig         `yaml:"sealingConfig,omitempty"`
	Images        []FixedImage           `yaml:"images,omitempty"`
	Discriminator string                 `yaml:"discriminator,omitempty"`
}

type DeploymentArg struct {
	Name    string      `yaml:"name" validate:"required"`
	Default interface{} `yaml:"default,omitempty"`
}

type SecretSet struct {
	Name string        `yaml:"name" validate:"required"`
	Vars []*VarsSource `yaml:"vars,omitempty"`
}

type GlobalSealedSecretsConfig struct {
	Bootstrap      *bool   `yaml:"bootstrap,omitempty"`
	Namespace      *string `yaml:"namespace,omitempty"`
	ControllerName *string `yaml:"controllerName,omitempty"`
}

type SecretsConfig struct {
	SealedSecrets *GlobalSealedSecretsConfig `yaml:"sealedSecrets,omitempty"`
	SecretSets    []SecretSet                `yaml:"secretSets,omitempty"`
}

type KluctlProject struct {
	Targets       []*Target        `yaml:"targets,omitempty"`
	Args          []*DeploymentArg `yaml:"args,omitempty"`
	SecretsConfig *SecretsConfig   `yaml:"secretsConfig,omitempty"`
	Discriminator string           `yaml:"discriminator,omitempty"`
}
