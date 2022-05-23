package types

type HelmChartConfig2 struct {
	Repo          *string `yaml:"repo" validate:"required"`
	CredentialsId *string `yaml:"credentialsId,omitempty"`
	ChartName     *string `yaml:"chartName,omitempty"`
	ChartVersion  *string `yaml:"chartVersion" validate:"required"`
	ReleaseName   string  `yaml:"releaseName" validate:"required"`
	Namespace     *string `yaml:"namespace,omitempty"`
	Output        *string `yaml:"output,omitempty"`
	SkipCRDs      bool    `yaml:"skipCRDs,omitempty"`
	SkipUpdate    bool    `yaml:"skipUpdate,omitempty"`
}

type HelmChartConfig struct {
	HelmChartConfig2 `yaml:"helmChart" validate:"required"`
}
