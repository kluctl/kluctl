package types

type ProjectMetadata struct {
	Targets []*DynamicTarget `yaml:"targets"`
}
