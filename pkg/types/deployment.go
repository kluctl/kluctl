package types

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type DeploymentItemConfig struct {
	Path             *string         `yaml:"path,omitempty"`
	Tags             []string        `yaml:"tags,omitempty"`
	Barrier          *bool           `yaml:"barrier,omitempty"`
	Vars             []*VarsListItem `yaml:"vars,omitempty"`
	SkipDeleteIfTags *bool           `yaml:"skipDeleteIfTags,omitempty"`
	OnlyRender       *bool           `yaml:"onlyRender,omitempty"`
	AlwaysDeploy     *bool           `yaml:"alwaysDeploy,omitempty"`
}

type DeploymentArg struct {
	Name    string      `yaml:"name" validate:"required"`
	Default interface{} `yaml:"default,omitempty"`
}

type VarsListItemClusterConfigMapOrSecret struct {
	Name      string `yaml:"name" validate:"required"`
	Namespace string `yaml:"namespace,omitempty"`
	Key       string `yaml:"key" validate:"required"`
}

type VarsListItem struct {
	Values           *map[string]interface{}               `yaml:"values,omitempty"`
	File             *string                               `yaml:"file,omitempty"`
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

func (s *SingleStringOrList) UnmarshalYAML(value *yaml.Node) error {
	var single string
	if err := value.Decode(&single); err == nil {
		// it's a single project
		*s = []string{single}
		return nil
	}
	// try as array
	var arr []string
	if err := value.Decode(&arr); err != nil {
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

	// Obsolete
	KustomizeDirs []*DeploymentItemConfig `yaml:"kustomizeDirs,omitempty"`
	Includes      []*DeploymentItemConfig `yaml:"includes,omitempty"`

	CommonLabels      map[string]string `yaml:"commonLabels,omitempty"`
	DeleteByLabels    map[string]string `yaml:"deleteByLabels,omitempty"`
	OverrideNamespace *string           `yaml:"overrideNamespace,omitempty"`
	Tags              []string          `yaml:"tags,omitempty"`

	IgnoreForDiff    []*IgnoreForDiffItemConfig `yaml:"ignoreForDiff,omitempty"`
	TemplateExcludes []string                   `yaml:"TemplateExcludes,omitempty"`
}

func LoadDeploymentProjectConfig(p string, o *DeploymentProjectConfig) error {
	err := utils.ReadYamlFile(p, o)
	if err != nil {
		return err
	}
	err = validate.Struct(o)
	if err != nil {
		return fmt.Errorf("validation for %v failed: %w", p, err)
	}
	return nil
}

func init() {
	validate.RegisterStructValidation(ValidateVarsListItem, VarsListItem{})
}
