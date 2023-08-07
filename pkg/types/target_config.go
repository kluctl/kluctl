package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type FixedImage struct {
	Image         *string        `json:"image,omitempty"`
	ImageRegex    *string        `json:"imageRegex,omitempty"`
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

func ValidateFixedImage(sl validator.StructLevel) {
	s := sl.Current().Interface().(FixedImage)
	if s.Image == nil && s.ImageRegex == nil {
		sl.ReportError(s, "image", "image", "one of image or imageRegex must be set", "")
	} else if s.Image != nil && s.ImageRegex != nil {
		sl.ReportError(s, "image", "image", "only one of image or imageRegex can be set", "")
	}
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateFixedImage, FixedImage{})
}
