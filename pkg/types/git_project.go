package types

import (
	"fmt"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/yaml"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// gitDirPatternNeg defines forbidden characters on git directory path/subDir
var gitDirPatternNeg = regexp.MustCompile(`[\\\/:\*?"<>|[:cntrl:]\0^]`)

type GitProject struct {
	Url    types.GitUrl  `json:"url" validate:"required"`
	Ref    *types.GitRef `json:"ref,omitempty"`
	SubDir string        `json:"subDir,omitempty"`
}

func (gp *GitProject) UnmarshalJSON(b []byte) error {
	if err := yaml.ReadYamlBytes(b, &gp.Url); err == nil {
		// it's a simple string
		return nil
	}
	type raw GitProject
	return yaml.ReadYamlBytes(b, (*raw)(gp))
}

// invalidDirName evaluate directory name against forbidden characters
func invalidDirName(dirName string) bool {
	return gitDirPatternNeg.MatchString(dirName)
}

// validateGitSubDir evaluate syntax for subdirectory path
func validateGitSubDir(path string) bool {
	for _, dirName := range strings.Split(path, "/") {
		if invalidDirName(dirName) {
			return false
		}
	}
	return true
}

func ValidateGitProject(sl validator.StructLevel) {
	gp := sl.Current().Interface().(GitProject)
	if !validateGitSubDir(gp.SubDir) {
		sl.ReportError(gp.SubDir, "subDir", "SubDir", fmt.Sprintf("'%s' is not valid git subdirectory path", gp.SubDir), "")
	}
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateGitProject, GitProject{})
}
