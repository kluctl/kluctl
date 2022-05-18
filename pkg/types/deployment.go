package types

import (
	"github.com/go-playground/validator/v10"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
)

type DeploymentItemConfig struct {
	Path             *string                  `yaml:"path,omitempty"`
	Include          *string                  `yaml:"include,omitempty"`
	Tags             []string                 `yaml:"tags,omitempty"`
	Barrier          *bool                    `yaml:"barrier,omitempty"`
	WaitReadiness    *bool                    `yaml:"waitReadiness,omitempty"`
	Vars             []*VarsListItem          `yaml:"vars,omitempty"`
	SkipDeleteIfTags *bool                    `yaml:"skipDeleteIfTags,omitempty"`
	OnlyRender       *bool                    `yaml:"onlyRender,omitempty"`
	AlwaysDeploy     *bool                    `yaml:"alwaysDeploy,omitempty"`
	DeleteObjects    []DeleteObjectItemConfig `yaml:"deleteObjects,omitempty"`
}

func ValidateDeploymentItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeploymentItemConfig)
	if s.Path != nil && s.Include != nil {
		sl.ReportError(s, "path", "Path", "pathinclude", "path and include can not be set at the same time")
	}
	if s.Path == nil && s.WaitReadiness != nil {
		sl.ReportError(s, "waitReadiness", "WaitReadiness", "waitreadiness", "only kustomize deployments are allowed to have waitReadiness set")
	}
}

type DeleteObjectItemConfig struct {
	Group     *string `yaml:"group,omitempty"`
	Version   *string `yaml:"version,omitempty"`
	Kind      *string `yaml:"kind,omitempty"`
	Name      string  `yaml:"name" validate:"required"`
	Namespace string  `yaml:"namespace,omitempty"`
}

func ValidateDeleteObjectItemConfig(sl validator.StructLevel) {
	s := sl.Current().Interface().(DeleteObjectItemConfig)
	if s.Group == nil && s.Version == nil && s.Kind == nil {
		sl.ReportError(s, "self", "self", "missingfield", "at least one of group/version/kind must be set")
	}
}

type DeploymentArg struct {
	Name    string      `yaml:"name" validate:"required"`
	Default interface{} `yaml:"default,omitempty"`
}

type VarsListItemGit struct {
	Url  git_url.GitUrl `yaml:"url" validate:"required"`
	Ref  string         `yaml:"ref,omitempty"`
	Path string         `yaml:"path" validate:"required"`
}

type VarsListItemClusterConfigMapOrSecret struct {
	Name      string `yaml:"name" validate:"required"`
	Namespace string `yaml:"namespace,omitempty"`
	Key       string `yaml:"key" validate:"required"`
}

type VarsListItem struct {
	Values           *uo.UnstructuredObject                `yaml:"values,omitempty"`
	File             *string                               `yaml:"file,omitempty"`
	Git              *VarsListItemGit                      `yaml:"git,omitempty"`
	ClusterConfigMap *VarsListItemClusterConfigMapOrSecret `yaml:"clusterConfigMap,omitempty"`
	ClusterSecret    *VarsListItemClusterConfigMapOrSecret `yaml:"clusterSecret,omitempty"`
}

func ValidateVarsListItem(sl validator.StructLevel) {
	s := sl.Current().Interface().(VarsListItem)
	count := 0
	if s.Values != nil {
		count += 1
	}
	if s.File != nil {
		count += 1
	}
	if s.Git != nil {
		count += 1
	}
	if s.ClusterConfigMap != nil {
		count += 1
	}
	if s.ClusterSecret != nil {
		count += 1
	}
	if count == 0 {
		sl.ReportError(s, "self", "self", "invalidvars", "unknown vars type")
	} else if count != 1 {
		sl.ReportError(s, "self", "self", "invalidvars", "more then one vars type")
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
	FieldPath SingleStringOrList `yaml:"fieldPath" validate:"required"`
	Group     *string            `yaml:"group,omitempty"`
	Kind      *string            `yaml:"kind,omitempty"`
	Name      *string            `yaml:"name,omitempty"`
	Namespace *string            `yaml:"namespace,omitempty"`
}

type DeploymentProjectConfig struct {
	Args          []*DeploymentArg     `yaml:"args,omitempty"`
	Vars          []*VarsListItem      `yaml:"vars,omitempty"`
	SealedSecrets *SealedSecretsConfig `yaml:"sealedSecrets,omitempty"`

	Deployments []*DeploymentItemConfig `yaml:"deployments,omitempty"`

	CommonLabels      map[string]string `yaml:"commonLabels,omitempty"`
	OverrideNamespace *string           `yaml:"overrideNamespace,omitempty"`
	Tags              []string          `yaml:"tags,omitempty"`

	IgnoreForDiff    []*IgnoreForDiffItemConfig `yaml:"ignoreForDiff,omitempty"`
	TemplateExcludes []string                   `yaml:"templateExcludes,omitempty"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateVarsListItem, VarsListItem{})
	yaml.Validator.RegisterStructValidation(ValidateDeploymentItemConfig, DeploymentItemConfig{})
	yaml.Validator.RegisterStructValidation(ValidateDeleteObjectItemConfig, DeleteObjectItemConfig{})
}
