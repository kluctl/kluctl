package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type DeploymentItemConfig struct {
	Path          *string                  `json:"path,omitempty"`
	Include       *string                  `json:"include,omitempty"`
	Git           *GitProject              `json:"git,omitempty"`
	Oci           *OciProject              `json:"oci,omitempty"`
	DeleteObjects []DeleteObjectItemConfig `json:"deleteObjects,omitempty"`

	Tags    []string `json:"tags,omitempty"`
	Barrier bool     `json:"barrier,omitempty"`
	Message *string  `json:"message,omitempty"`

	WaitReadiness        bool                            `json:"waitReadiness,omitempty"`
	WaitReadinessObjects []WaitReadinessObjectItemConfig `json:"waitReadinessObjects,omitempty"`

	Args     *uo.UnstructuredObject `json:"args,omitempty"`
	PassVars bool                   `json:"passVars,omitempty"`
	Vars     []*VarsSource          `json:"vars,omitempty"`

	SkipDeleteIfTags bool   `json:"skipDeleteIfTags,omitempty"`
	OnlyRender       bool   `json:"onlyRender,omitempty"`
	AlwaysDeploy     bool   `json:"alwaysDeploy,omitempty"`
	When             string `json:"when,omitempty"`

	// these are only allowed when writing the command result
	RenderedHelmChartConfig *HelmChartConfig         `json:"renderedHelmChartConfig,omitempty"`
	RenderedObjects         []k8s.ObjectRef          `json:"renderedObjects,omitempty"`
	RenderedInclude         *DeploymentProjectConfig `json:"renderedInclude,omitempty"`
}

func ValidateDeploymentItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeploymentItemConfig)
	cnt := 0
	isInclude := false
	if s.Path != nil {
		cnt += 1
	}
	if s.Include != nil {
		cnt += 1
		isInclude = true
	}
	if s.Git != nil {
		cnt += 1
		isInclude = true
	}
	if s.Oci != nil {
		cnt += 1
		isInclude = true
	}
	if cnt > 1 {
		sl.ReportError(s, "self", "self", "only one of path, include, git and oci can be set at the same time", "")
	}
	if s.Path == nil && s.WaitReadiness {
		sl.ReportError(s, "waitReadiness", "WaitReadiness", "only kustomize deployments are allowed to have waitReadiness set", "")
	}
	if !s.Args.IsZero() && !isInclude {
		sl.ReportError(s, "self", "self", "args are only allowed when another project is included (via include, git or oci)", "")
	}
	if s.PassVars && !isInclude {
		sl.ReportError(s, "self", "self", "passVars is only allowed when another project is included (via include, git or oci)", "")
	}
}

type ObjectRefItem struct {
	Group     *string `json:"group,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Name      string  `json:"name" validate:"required"`
	Namespace string  `json:"namespace,omitempty"`
}

type DeleteObjectItemConfig struct {
	ObjectRefItem
}

func ValidateDeleteObjectItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeleteObjectItemConfig)
	if s.Group == nil && s.Kind == nil {
		sl.ReportError(s, "self", "self", "at least one of group or kind must be set", "")
	}
}

type WaitReadinessObjectItemConfig struct {
	ObjectRefItem
}

func ValidateWaitReadinessObjectItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(WaitReadinessObjectItemConfig)
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
	if err := yaml.ReadYamlBytes(b, &single); err == nil {
		// it's a single project
		*s = []string{single}
		return nil
	}
	// try as array
	var arr []string
	if err := yaml.ReadYamlBytes(b, &arr); err != nil {
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
	yaml.Validator.RegisterStructValidation(ValidateWaitReadinessObjectItemConfig, WaitReadinessObjectItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateIgnoreForDiffItemConfig, IgnoreForDiffItemConfig{})
}
