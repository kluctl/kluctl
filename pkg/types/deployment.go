package types

import (
	"encoding/json"
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type DeploymentItemConfig struct {
	Path             *string                  `json:"path,omitempty"`
	Include          *string                  `json:"include,omitempty"`
	Git              *GitProject              `json:"git,omitempty"`
	Tags             []string                 `json:"tags,omitempty"`
	Barrier          bool                     `json:"barrier,omitempty"`
	Message          *string                  `json:"message,omitempty"`
	WaitReadiness    bool                     `json:"waitReadiness,omitempty"`
	Vars             []*VarsSource            `json:"vars,omitempty"`
	SkipDeleteIfTags bool                     `json:"skipDeleteIfTags,omitempty"`
	OnlyRender       bool                     `json:"onlyRender,omitempty"`
	AlwaysDeploy     bool                     `json:"alwaysDeploy,omitempty"`
	DeleteObjects    []DeleteObjectItemConfig `json:"deleteObjects,omitempty"`
	When             string                   `json:"when,omitempty"`

	// these are only allowed when writing the command result
	RenderedHelmChartConfig *HelmChartConfig         `json:"renderedHelmChartConfig,omitempty"`
	RenderedObjects         []k8s.ObjectRef          `json:"renderedObjects,omitempty"`
	RenderedInclude         *DeploymentProjectConfig `json:"renderedInclude,omitempty"`
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
	Group     *string `json:"group,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Name      string  `json:"name" validate:"required"`
	Namespace string  `json:"namespace,omitempty"`
}

func ValidateDeleteObjectItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeleteObjectItemConfig)
	if s.Group == nil && s.Kind == nil {
		sl.ReportError(s, "self", "self", "at least one of group or kind must be set", "")
	}
}

type SealedSecretsConfig struct {
	OutputPattern *string `json:"outputPattern,omitempty"`
}

type SingleStringOrList []string

func (s *SingleStringOrList) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		// it's a single project
		*s = []string{single}
		return nil
	}
	// try as array
	var arr []string
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	*s = arr
	return nil
}

type IgnoreForDiffItemConfig struct {
	FieldPath      SingleStringOrList `json:"fieldPath,omitempty"`
	FieldPathRegex SingleStringOrList `json:"fieldPathRegex,omitempty"`
	Group          *string            `json:"group,omitempty"`
	Kind           *string            `json:"kind,omitempty"`
	Name           *string            `json:"name,omitempty"`
	Namespace      *string            `json:"namespace,omitempty"`
}

func ValidateIgnoreForDiffItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(IgnoreForDiffItemConfig)
	if len(s.FieldPath)+len(s.FieldPathRegex) == 0 {
		sl.ReportError(s, "self", "self", "at least one of fieldPath or fieldPathRegex must be set", "")
	}
}

type DeploymentProjectConfig struct {
	Vars          []*VarsSource        `json:"vars,omitempty"`
	SealedSecrets *SealedSecretsConfig `json:"sealedSecrets,omitempty"`

	When string `json:"when,omitempty"`

	Deployments []*DeploymentItemConfig `json:"deployments,omitempty"`

	CommonLabels      map[string]string `json:"commonLabels,omitempty"`
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty"`
	OverrideNamespace *string           `json:"overrideNamespace,omitempty"`
	Tags              []string          `json:"tags,omitempty"`

	IgnoreForDiff []*IgnoreForDiffItemConfig `json:"ignoreForDiff,omitempty"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateDeploymentItemConfig, DeploymentItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateDeleteObjectItemConfig, DeleteObjectItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateIgnoreForDiffItemConfig, IgnoreForDiffItemConfig{})
}
