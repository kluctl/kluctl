package types

type InvolvedRepo struct {
	RefPattern string            `yaml:"refPattern"`
	Refs       map[string]string `yaml:"refs"`
}

type ProjectMetadata struct {
	InvolvedRepos map[string][]InvolvedRepo `yaml:"involvedRepos"`
	Targets       []*DynamicTarget          `yaml:"targets"`
}

type GitRepoMetadata struct {
	Ref    string `yaml:"ref"`
	Commit string `yaml:"commit"`
}

type ArchiveMetadata struct {
	ProjectRootDir string `yaml:"projectRootDir"`
	ProjectSubDir  string `yaml:"projectSubDir"`
}
