package types

type HelmChartConfig2 struct {
	Repo         *string `yaml:"repo,omitempty"`
	ChartName    *string `yaml:"chartName,omitempty"`
	ChartVersion *string `yaml:"chartVersion,omitempty"`
	ReleaseName  string  `yaml:"releaseName"`
	Namespace    *string `yaml:"namespace,omitempty"`
	Output       string  `yaml:"output"`
	SkipCRDs     *bool   `yaml:"skipCRDs,omitempty"`
	SkipUpdate   *bool   `yaml:"skipUpdate,omitempty"`
}

type HelmChartConfig struct {
	HelmChartConfig2 `yaml:"helmChart" validate:"required"`
}
