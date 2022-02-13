package types

type FixedImage struct {
	Image         string   `yaml:"image" validate:"required"`
	ResultImage   string   `yaml:"resultImage" validate:"required"`
	DeployedImage *string  `yaml:"deployedImage,omitempty"`
	RegistryImage *string  `yaml:"registryImage,omitempty"`
	Namespace     *string  `yaml:"namespace,omitempty"`
	Deployment    *string  `yaml:"deployment,omitempty"`
	Container     *string  `yaml:"container,omitempty"`
	VersionFilter *string  `yaml:"versionFilter,omitempty"`
	DeployTags    []string `yaml:"deployTags,omitempty"`
	DeploymentDir *string  `yaml:"deploymentDir,omitempty"`
}

type FixedImagesConfig struct {
	Images []FixedImage `yaml:"images,omitempty"`
}

type TargetConfig struct {
	FixedImagesConfig `yaml:"fixed_images_config,inline"`
	Args              map[string]interface{} `yaml:"args,omitempty"`
}
