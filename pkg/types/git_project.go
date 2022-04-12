package types

import (
	"github.com/go-playground/validator/v10"
	git_url "github.com/kluctl/kluctl/pkg/git/git-url"
	"github.com/kluctl/kluctl/pkg/yaml"
	"strings"
)

type GitProject struct {
	Url    git_url.GitUrl `yaml:"url" validate:"required"`
	Ref    string         `yaml:"ref,omitempty"`
	SubDir string         `yaml:"subDir,omitempty"`
}

func (gp *GitProject) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&gp.Url); err == nil {
		// it's a simple string
		return nil
	}
	type raw GitProject
	return unmarshal((*raw)(gp))
}

func ValidateGitProject(sl validator.StructLevel) {
	gp := sl.Current().Interface().(GitProject)
	if strings.Index(gp.SubDir, "/") != -1 {
		sl.ReportError(gp.SubDir, "subDir", "SubDir", "subdirinvalid", "/ is not allowed in subdir")
	}
}

type ExternalProject struct {
	Project *GitProject `yaml:"project,omitempty"`
	Path *string `yaml:"path,omitempty"`
}

func ValidateExternalProject(sl validator.StructLevel) {
	p := sl.Current().Interface().(ExternalProject)
	if p.Project == nil && p.Path == nil {
		sl.ReportError(p, ".", ".", "empty", "either project or path must be set")
	} else if p.Project != nil && p.Path != nil {
		sl.ReportError(p, ".", ".", "empty", "only one of project or path can be set")
	}
}

type ExternalProjects struct {
	Projects []ExternalProject
}

func (gp *ExternalProjects) UnmarshalYAML(unmarshal func(interface{}) error) error {
	singleProject := ExternalProject{}
	if err := unmarshal(&singleProject); err == nil {
		// it's a single project
		gp.Projects = []ExternalProject{singleProject}
		return nil
	}
	// try as array
	return unmarshal(&gp.Projects)
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateGitProject, GitProject{})
	yaml.Validator.RegisterStructValidation(ValidateExternalProject, ExternalProject{})
}
