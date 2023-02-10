package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type DynamicArg struct {
	Name string `yaml:"name" validate:"required"`
}

type ExternalTargetConfig struct {
	Project *GitProject `yaml:"project,omitempty"`
	// Ref Branch/Tag to be used. Can't be combined with 'refPattern'. If 'branch' and 'branchPattern' are not used, 'branch' defaults to the default branch of targetConfig.project
	Ref *string `yaml:"ref,omitempty"`
	// RefPattern If set, multiple dynamic targets are created, each with 'ref' being set to the ref that matched the given pattern.
	RefPattern *string `yaml:"refPattern,omitempty"`
	// File defaults to 'target-config.yml'
	File *string `yaml:"file,omitempty"`
}

type SealingConfig struct {
	// DynamicSealing Set this to false if you want to disable sealing for every dynamic target
	DynamicSealing *bool                  `yaml:"dynamicSealing,omitempty"`
	Args           *uo.UnstructuredObject `yaml:"args,omitempty"`
	SecretSets     []string               `yaml:"secretSets,omitempty"`
	CertFile       *string                `yaml:"certFile,omitempty"`
}

type Target struct {
	Name          string                 `yaml:"name" validate:"required"`
	Context       *string                `yaml:"context,omitempty"`
	Args          *uo.UnstructuredObject `yaml:"args,omitempty"`
	DynamicArgs   []DynamicArg           `yaml:"dynamicArgs,omitempty"`
	TargetConfig  *ExternalTargetConfig  `yaml:"targetConfig,omitempty"`
	SealingConfig *SealingConfig         `yaml:"sealingConfig,omitempty"`
	Images        []FixedImage           `yaml:"images,omitempty"`
	Discriminator string                 `yaml:"discriminator,omitempty"`
}

type DynamicTarget struct {
	Target         *Target `yaml:"target" validate:"required"`
	BaseTargetName string  `yaml:"baseTargetName"`
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
