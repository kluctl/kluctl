package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type DynamicArg struct {
	Name    string  `yaml:"name" validate:"required"`
	Pattern *string `yaml:"pattern,omitempty"`
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
}

type Target struct {
	Name          string                 `yaml:"name" validate:"required"`
	Cluster       *string                `yaml:"cluster,omitempty"`
	Context       *string                `yaml:"context,omitempty"`
	Args          *uo.UnstructuredObject `yaml:"args,omitempty"`
	DynamicArgs   []DynamicArg           `yaml:"dynamicArgs,omitempty"`
	TargetConfig  *ExternalTargetConfig  `yaml:"targetConfig,omitempty"`
	SealingConfig *SealingConfig         `yaml:"sealingConfig,omitempty"`
	Images        []FixedImage           `yaml:"images,omitempty"`
}

type DynamicTarget struct {
	Target         *Target `yaml:"target" validate:"required"`
	BaseTargetName string  `yaml:"baseTargetName"`
}

type SecretSet struct {
	Name    string         `yaml:"name" validate:"required"`
	Sources []SecretSource `yaml:"sources" validate:"required,gt=0"`
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
	Deployment    *ExternalProject `yaml:"deployment,omitempty"`
	SealedSecrets *ExternalProject `yaml:"sealedSecrets,omitempty"`
	Clusters      ExternalProjects `yaml:"clusters,omitempty"`
	Targets       []*Target        `yaml:"targets,omitempty"`
	SecretsConfig *SecretsConfig   `yaml:"secretsConfig,omitempty"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateSecretSource, SecretSource{})
}
