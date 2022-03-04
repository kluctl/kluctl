package types

type InvolvedRepo struct {
	RefPattern string            `yaml:"refPattern"`
	Refs       map[string]string `yaml:"refs"`
}

type ArchiveMetadata struct {
	InvolvedRepos map[string][]InvolvedRepo `yaml:"involvedRepos"`
	Targets       []*DynamicTarget          `yaml:"targets"`
}
