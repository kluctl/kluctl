package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type DeploymentItemConfig struct {
	Path             *string                  `yaml:"path,omitempty"`
	Include          *string                  `yaml:"include,omitempty"`
	Git              *GitProject              `yaml:"git,omitempty"`
	Tags             []string                 `yaml:"tags,omitempty"`
	Barrier          bool                     `yaml:"barrier,omitempty"`
	WaitReadiness    bool                     `yaml:"waitReadiness,omitempty"`
	Vars             []*VarsSource            `yaml:"vars,omitempty"`
	SkipDeleteIfTags bool                     `yaml:"skipDeleteIfTags,omitempty"`
	OnlyRender       bool                     `yaml:"onlyRender,omitempty"`
	AlwaysDeploy     bool                     `yaml:"alwaysDeploy,omitempty"`
	DeleteObjects    []DeleteObjectItemConfig `yaml:"deleteObjects,omitempty"`
	When             string                   `yaml:"when,omitempty"`
}

func ValidateDeploymentItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeploymentItemConfig)
	cnt := 0
	if s.Path != nil {
		cnt += 1
	}
	if s.Include != nil {
		cnt += 1
	}
	if s.Git != nil {
		cnt += 1
	}
	if cnt > 1 {
		sl.ReportError(s, "self", "self", "only one of path, include and git can be set at the same time", "")
	}
	if s.Path == nil && s.WaitReadiness {
		sl.ReportError(s, "waitReadiness", "WaitReadiness", "only kustomize deployments are allowed to have waitReadiness set", "")
	}
}

type DeleteObjectItemConfig struct {
	Group     *string `yaml:"group,omitempty"`
	Kind      *string `yaml:"kind,omitempty"`
	Name      string  `yaml:"name" validate:"required"`
	Namespace string  `yaml:"namespace,omitempty"`
}

func ValidateDeleteObjectItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeleteObjectItemConfig)
	if s.Group == nil && s.Kind == nil {
		sl.ReportError(s, "self", "self", "at least one of group/version/kind must be set", "")
	}
}

type SealedSecretsConfig struct {
	OutputPattern *string `yaml:"outputPattern,omitempty"`
}

type SingleStringOrList []string

func (s *SingleStringOrList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		// it's a single project
		*s = []string{single}
		return nil
	}
	// try as array
	var arr []string
	if err := unmarshal(&arr); err != nil {
		return err
	}
	*s = arr
	return nil
}

type IgnoreForDiffItemConfig struct {
	FieldPath      SingleStringOrList `yaml:"fieldPath" validate:"required"`
	FieldPathRegex SingleStringOrList `yaml:"fieldPathRegex,omitempty"`
	Group          *string            `yaml:"group,omitempty"`
	Kind           *string            `yaml:"kind,omitempty"`
	Name           *string            `yaml:"name,omitempty"`
	Namespace      *string            `yaml:"namespace,omitempty"`
}

func ValidateIgnoreForDiffItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(IgnoreForDiffItemConfig)
	if len(s.FieldPath)+len(s.FieldPathRegex) == 0 {
		sl.ReportError(s, "self", "self", "at least one of fieldPath or fieldPathRegex must be set", "")
	}
}

type DeploymentProjectConfig struct {
	Args          []*DeploymentArg     `yaml:"args,omitempty"`
	Vars          []*VarsSource        `yaml:"vars,omitempty"`
	SealedSecrets *SealedSecretsConfig `yaml:"sealedSecrets,omitempty"`

	When string `yaml:"when,omitempty"`

	Deployments []*DeploymentItemConfig `yaml:"deployments,omitempty"`

	CommonLabels      map[string]string `yaml:"commonLabels,omitempty"`
	OverrideNamespace *string           `yaml:"overrideNamespace,omitempty"`
	Tags              []string          `yaml:"tags,omitempty"`

	IgnoreForDiff []*IgnoreForDiffItemConfig `yaml:"ignoreForDiff,omitempty"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateDeploymentItemConfig, DeploymentItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateDeleteObjectItemConfig, DeleteObjectItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateIgnoreForDiffItemConfig, IgnoreForDiffItemConfig{})
}
