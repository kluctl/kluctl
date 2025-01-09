package types

import (
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/lib/yaml"
	"helm.sh/helm/v3/pkg/registry"
)

type HelmChartConfig2 struct {
	Repo              string      `json:"repo,omitempty"`
	TarUrl            string      `json:"tarUrl,omitempty"`
	Git               *GitProject `json:"git,omitempty"`
	Path              string      `json:"path,omitempty"`
	CredentialsId     *string     `json:"credentialsId,omitempty"`
	ChartName         string      `json:"chartName,omitempty"`
	ChartVersion      *string     `json:"chartVersion,omitempty"`
	UpdateConstraints *string     `json:"updateConstraints,omitempty"`
	ReleaseName       string      `json:"releaseName" validate:"required"`
	Namespace         *string     `json:"namespace,omitempty"`
	Output            *string     `json:"output,omitempty"`
	SkipCRDs          bool        `json:"skipCRDs,omitempty"`
	SkipUpdate        bool        `json:"skipUpdate,omitempty"`
	SkipPrePull       bool        `json:"skipPrePull,omitempty"`
}

func ValidateHelmChartConfig2(sl validator.StructLevel) {
	c := sl.Current().Interface().(HelmChartConfig2)
	cnt := 0
	if c.Repo != "" {
		cnt++
	}
	if c.Path != "" {
		cnt++
	}
	if c.Git != nil {
		cnt++
	}
	if c.TarUrl != "" {
		cnt++
	}
	if cnt == 0 {
		sl.ReportError("self", "repo", "repo", "either repo, path, git, or tarUrl must be specified", "")
	} else if cnt > 1 {
		sl.ReportError("self", "repo", "repo", "only one of repo, path, git, or tarUrl can be specified", "")
	} else if c.Repo != "" {
		if c.ChartVersion == nil || *c.ChartVersion == "" {
			sl.ReportError("self", "chartVersion", "chartVersion", "chartVersion must be specified when repo is specified", "")
		}
		if registry.IsOCI(c.Repo) {
			if c.ChartName != "" {
				sl.ReportError("self", "chartName", "chartName", "chartName cannot be specified when repo is an OCI url", "")
			}
		} else {
			if c.ChartName == "" {
				sl.ReportError("self", "chartName", "chartName", "chartName must be specified when repo is a normal Helm repo", "")
			}
		}
	} else if c.Path != "" {
		if c.ChartName != "" {
			sl.ReportError("self", "chartName", "chartName", "chartName cannot be specified for local Helm charts", "")
		}
		if c.ChartVersion != nil {
			sl.ReportError("self", "chartVersion", "chartVersion", "chartVersion cannot be specified for local Helm charts", "")
		}
		if c.UpdateConstraints != nil {
			sl.ReportError("self", "updateConstraints", "updateConstraints", "updateConstraints cannot be specified for local Helm charts", "")
		}
	} else if c.Git != nil {
		if c.ChartName != "" {
			sl.ReportError("self", "chartName", "chartName", "chartName cannot be specified for git Helm charts", "")
		}
		if c.ChartVersion != nil {
			sl.ReportError("self", "chartVersion", "chartVersion", "chartVersion cannot be specified for git Helm charts", "")
		}
	} else if c.TarUrl != "" {
		// Additional validation for URL if needed
		if c.ChartName == "" {
			sl.ReportError("self", "chartName", "chartName", "chartName must be specified when using a tarUrl", "")
		}
	}
}

type HelmChartConfig struct {
	HelmChartConfig2 `json:"helmChart" validate:"required"`
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateHelmChartConfig2, HelmChartConfig2{})
}
