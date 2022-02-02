package types

import (
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
	"strings"
)

type GitProject struct {
	Url    git_url.GitUrl `yaml:"url" validate:"required"`
	Ref    string         `yaml:"ref"`
	SubDir string         `yaml:"subDir"`
}

func (gp *GitProject) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&gp.Url); err == nil {
		// it's a simple string
		return nil
	}
	type raw GitProject
	return value.Decode((*raw)(gp))
}

func ValidateGitProject(sl validator.StructLevel) {
	gp := sl.Current().Interface().(GitProject)
	if strings.Index(gp.SubDir, "/") != -1 {
		sl.ReportError(gp.SubDir, "subDir", "SubDir", "subdirinvalid", "/ is not allowed in subdir")
	}
}

type ExternalProject struct {
	Project GitProject `yaml:"project"`
}

type ExternalProjects struct {
	Projects []ExternalProject
}

func (gp *ExternalProjects) UnmarshalYAML(value *yaml.Node) error {
	singleProject := ExternalProject{}
	if err := value.Decode(&singleProject); err == nil {
		// it's a single project
		gp.Projects = []ExternalProject{singleProject}
		return nil
	}
	// try as array
	return value.Decode(gp.Projects)
}

func init() {
	validate.RegisterStructValidation(ValidateGitProject, GitProject{})
}
