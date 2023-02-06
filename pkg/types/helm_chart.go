package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"helm.sh/helm/v3/pkg/registry"
)

type HelmChartConfig2 struct {
	Repo              string  `yaml:"repo,omitempty"`
	Path              string  `yaml:"path,omitempty"`
	CredentialsId     *string `yaml:"credentialsId,omitempty"`
	ChartName         string  `yaml:"chartName,omitempty"`
	ChartVersion      string  `yaml:"chartVersion,omitempty"`
	UpdateConstraints *string `yaml:"updateConstraints,omitempty"`
	ReleaseName       string  `yaml:"releaseName" validate:"required"`
	Namespace         *string `yaml:"namespace,omitempty"`
	Output            *string `yaml:"output,omitempty"`
	SkipCRDs          bool    `yaml:"skipCRDs,omitempty"`
	SkipUpdate        bool    `yaml:"skipUpdate,omitempty"`
	SkipPrePull       bool    `yaml:"skipPrePull,omitempty"`
}

func ValidateHelmChartConfig2(sl validator.StructLevel) {
	c := sl.Current().Interface().(HelmChartConfig2)
	if c.Repo == "" && c.Path == "" {
		sl.ReportError("self", "repo", "repo", "either repo or path must be specified", "")
	} else if c.Repo != "" && c.Path != "" {
		sl.ReportError("self", "repo", "repo", "only one of repo and path can be specified", "")
	} else if c.Repo != "" {
		if c.ChartVersion == "" {
			sl.ReportError("self", "chartVersion", "chartVersion", "chartVersion must be specified when repo is specified", "")
		}
		if registry.IsOCI(c.Repo) {
			if c.ChartName != "" {
				sl.ReportError("self", "chartName", "chartName", "chartName can not be specified when repo is a OCI url", "")
			}
		} else {
			if c.ChartName == "" {
				sl.ReportError("self", "chartName", "chartName", "chartName must be specified when repo is normal Helm repo", "")
			}
		}
	} else if c.Path != "" {
		if c.ChartName != "" {
			sl.ReportError("self", "chartName", "chartName", "chartName can not be specified for local Helm charts", "")
		}
		if c.ChartVersion != "" {
			sl.ReportError("self", "chartVersion", "chartVersion", "chartVersion can not be specified for local Helm charts", "")
		}
		if c.UpdateConstraints != nil {
			sl.ReportError("self", "updateConstraints", "updateConstraints", "updateConstraints can not be specified for local Helm charts", "")
		}
	}
}

type HelmChartConfig struct {
	HelmChartConfig2 `yaml:"helmChart" validate:"required"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateHelmChartConfig2, HelmChartConfig2{})
}
