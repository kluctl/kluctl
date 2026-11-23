package types

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

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
	Kubeconfig    *string                `json:"kubeconfig,omitempty"`
	Args          *uo.UnstructuredObject `json:"args,omitempty"`
	Aws           *AwsConfig             `json:"aws,omitempty"`
	Images        []FixedImage           `json:"images,omitempty"`
	Discriminator string                 `json:"discriminator,omitempty"`
}

type DeploymentArg struct {
	Name    string                `json:"name" validate:"required"`
	Default *apiextensionsv1.JSON `json:"default,omitempty"`
}

type KluctlProject struct {
	Targets       []Target        `json:"targets,omitempty"`
	Args          []DeploymentArg `json:"args,omitempty"`
	Discriminator string          `json:"discriminator,omitempty"`
	Aws           *AwsConfig      `json:"aws,omitempty"`
}

type KluctlLibraryProject struct {
	Args []DeploymentArg `json:"args,omitempty"`
}
