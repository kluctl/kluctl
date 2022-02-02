package types

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
)

type HelmChartConfig2 struct {
	Repo         *string `yaml:"repo,omitempty"`
	ChartName    *string `yaml:"chartName,omitempty"`
	ChartVersion *string `yaml:"chartVersion,omitempty"`
	ReleaseName  string  `yaml:"releaseName"`
	Namespace    *string `yaml:"namespace,omitempty"`
	Output       string  `yaml:"output"`
	SkipCRDs     *bool   `yaml:"skipCRDs,omitempty"`
}

type HelmChartConfig struct {
	HelmChartConfig2 `yaml:"helmChart" validate:"required"`
}

func LoadHelmChartConfig(p string) (*HelmChartConfig, error) {
	var o HelmChartConfig
	err := utils.ReadYamlFile(p, &o)
	if err != nil {
		return nil, err
	}
	err = validate.Struct(o)
	if err != nil {
		return nil, fmt.Errorf("validation for %v failed: %w", p, err)
	}
	return &o, nil
}
