package types

import (
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type FixedImage struct {
	Image         string         `yaml:"image" validate:"required"`
	ResultImage   string         `yaml:"resultImage" validate:"required"`
	DeployedImage *string        `yaml:"deployedImage,omitempty"`
	Namespace     *string        `yaml:"namespace,omitempty"`
	Object        *k8s.ObjectRef `yaml:"object,omitempty"`
	Deployment    *string        `yaml:"deployment,omitempty"`
	Container     *string        `yaml:"container,omitempty"`
	DeployTags    []string       `yaml:"deployTags,omitempty"`
	DeploymentDir *string        `yaml:"deploymentDir,omitempty"`
}

type FixedImagesConfig struct {
	Images []FixedImage `yaml:"images,omitempty"`
}
