package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type SealingConfig struct {
	Args       *uo.UnstructuredObject `json:"args,omitempty"`
	SecretSets []string               `json:"secretSets,omitempty"`
	CertFile   *string                `json:"certFile,omitempty"`
}

type ServiceAccountRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type AwsConfig struct {
	Profile        *string            `json:"profile,omitempty"`
	ServiceAccount *ServiceAccountRef `json:"serviceAccount,omitempty"`
}

type Target struct {
	Name          string                 `json:"name"`
	Context       *string                `json:"context,omitempty"`
	Args          *uo.UnstructuredObject `json:"args,omitempty"`
	SealingConfig *SealingConfig         `json:"sealingConfig,omitempty"`
	Aws           *AwsConfig             `json:"aws,omitempty"`
	Images        []FixedImage           `json:"images,omitempty"`
	Discriminator string                 `json:"discriminator,omitempty"`
}

type DeploymentArg struct {
	Name    string                `json:"name" validate:"required"`
	Default *apiextensionsv1.JSON `json:"default,omitempty"`
}

type SecretSet struct {
	Name string       `json:"name" validate:"required"`
	Vars []VarsSource `json:"vars,omitempty"`
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
	Targets       []Target        `json:"targets,omitempty"`
	Args          []DeploymentArg `json:"args,omitempty"`
	SecretsConfig *SecretsConfig  `json:"secretsConfig,omitempty"`
	Discriminator string          `json:"discriminator,omitempty"`
	Aws           *AwsConfig      `json:"aws,omitempty"`
}

type KluctlLibraryProject struct {
	Args []DeploymentArg `json:"args,omitempty"`
}
