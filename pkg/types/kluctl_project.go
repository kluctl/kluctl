package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type SealingConfig struct {
	Args       *uo.UnstructuredObject `json:"args,omitempty"`
	SecretSets []string               `json:"secretSets,omitempty"`
	CertFile   *string                `json:"certFile,omitempty"`
}

type Target struct {
	Name          string                 `json:"name" validate:"required"`
	Context       *string                `json:"context,omitempty"`
	Args          *uo.UnstructuredObject `json:"args,omitempty"`
	SealingConfig *SealingConfig         `json:"sealingConfig,omitempty"`
	Images        []FixedImage           `json:"images,omitempty"`
	Discriminator string                 `json:"discriminator,omitempty"`
}

type DeploymentArg struct {
	Name    string      `json:"name" validate:"required"`
	Default interface{} `json:"default,omitempty"`
}

type SecretSet struct {
	Name string        `json:"name" validate:"required"`
	Vars []*VarsSource `json:"vars,omitempty"`
}

type GlobalSealedSecretsConfig struct {
	Bootstrap      *bool   `json:"bootstrap,omitempty"`
	Namespace      *string `json:"namespace,omitempty"`
	ControllerName *string `json:"controllerName,omitempty"`
}

type SecretsConfig struct {
	SealedSecrets *GlobalSealedSecretsConfig `json:"sealedSecrets,omitempty"`
	SecretSets    []SecretSet                `json:"secretSets,omitempty"`
}

type KluctlProject struct {
	Targets       []*Target        `json:"targets,omitempty"`
	Args          []*DeploymentArg `json:"args,omitempty"`
	SecretsConfig *SecretsConfig   `json:"secretsConfig,omitempty"`
	Discriminator string           `json:"discriminator,omitempty"`
}
