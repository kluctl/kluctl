package types

type ProjectMetadata struct {
	Targets []*DynamicTarget `yaml:"targets"`
}

type ArchiveMetadata struct {
	ProjectRootDir string `yaml:"projectRootDir"`
	ProjectSubDir  string `yaml:"projectSubDir"`
}
