package types

import (
	"fmt"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"helm.sh/helm/v3/pkg/registry"
)

type HelmChartConfig2 struct {
	Repo              string        `json:"repo,omitempty"`
	Git               types.GitInfo `json:"git,omitempty" validate:"required"`
	Path              string        `json:"path,omitempty"`
	CredentialsId     *string       `json:"credentialsId,omitempty"`
	ChartName         string        `json:"chartName,omitempty"`
	ChartVersion      string        `json:"chartVersion,omitempty"`
	UpdateConstraints *string       `json:"updateConstraints,omitempty"`
	ReleaseName       string        `json:"releaseName" validate:"required"`
	Namespace         *string       `json:"namespace,omitempty"`
	Output            *string       `json:"output,omitempty"`
	SkipCRDs          bool          `json:"skipCRDs,omitempty"`
	SkipUpdate        bool          `json:"skipUpdate,omitempty"`
	SkipPrePull       bool          `json:"skipPrePull,omitempty"`
}

func ValidateHelmChartConfig2(sl validator.StructLevel) {
	c := sl.Current().Interface().(HelmChartConfig2)
	if c.Repo == "" && c.Path == "" && c.Git == (types.GitInfo{}) {
		sl.ReportError("self", "repo", "repo", "either repo, path or git must be specified", "")
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
	HelmChartConfig2 `json:"helmChart" validate:"required"`
}

func (c *HelmChartConfig2) IsLocalChart() bool {
	return c.Path != ""
}
func (c *HelmChartConfig2) ErrWhenLocalPathInvalid() error {
	if filepath.IsAbs(c.Path) {
		return fmt.Errorf("absolute path is not allowed in helm-chart.yaml")
	}
	return nil
}

func (c *HelmChartConfig2) GetAbsoluteLocalPath(projectRoot string, relDirInProject string) (string, error) {
	localPath := ""
	localPath = filepath.Join(projectRoot, relDirInProject, c.Path)
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}
	err = utils.CheckInDir(projectRoot, localPath)
	if err != nil {
		return "", err
	}
	return localPath, nil
}

func (c *HelmChartConfig2) IsOciRegistryChart() bool {
	return c.Repo != "" && registry.IsOCI(c.Repo)
}

func (c *HelmChartConfig2) IsHelmRegistryChart() bool {
	return c.Repo != "" && !registry.IsOCI(c.Repo)
}

func (c *HelmChartConfig2) IsGitRepositoryChart() bool {
	return c.Git != (types.GitInfo{})
}

func (c *HelmChartConfig2) GetGitRef() (string, string, error) {
	if c.Git.Ref.Branch != "" {
		return c.Git.Ref.Branch, "branch", nil
	}
	if c.Git.Ref.Tag != "" {
		return c.Git.Ref.Tag, "tag", nil
	}

	if c.Git.Ref.Commit != "" {
		return c.Git.Ref.Commit, "commit", nil
	}
	return "", "", fmt.Errorf("neither branch, tag nor commit defined")
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateHelmChartConfig2, HelmChartConfig2{})
}
