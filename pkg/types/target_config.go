package types

import (
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
)

type FixedImage struct {
	Image         string         `json:"image" validate:"required"`
	ResultImage   string         `json:"resultImage" validate:"required"`
	DeployedImage *string        `json:"deployedImage,omitempty"`
	Namespace     *string        `json:"namespace,omitempty"`
	Object        *k8s.ObjectRef `json:"object,omitempty"`
	Deployment    *string        `json:"deployment,omitempty"`
	Container     *string        `json:"container,omitempty"`
	DeployTags    []string       `json:"deployTags,omitempty"`
	DeploymentDir *string        `json:"deploymentDir,omitempty"`
}

type FixedImagesConfig struct {
	Images []FixedImage `json:"images,omitempty"`
}
