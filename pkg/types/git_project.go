package types

import (
	"encoding/json"
	"fmt"
	yaml2 "github.com/kluctl/kluctl/v2/lib/yaml"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// gitDirPatternNeg defines forbidden characters on git directory path/subDir
var gitDirPatternNeg = regexp.MustCompile(`[\\\/:\*?"<>|[:cntrl:]\0^]`)

type GitProject struct {
	Url    GitUrl  `json:"url" validate:"required"`
	Ref    *GitRef `json:"ref,omitempty"`
	SubDir string  `json:"subDir,omitempty"`
}

func (gp *GitProject) UnmarshalJSON(b []byte) error {
	if err := yaml2.ReadYamlBytes(b, &gp.Url); err == nil {
		// it's a simple string
		return nil
	}
	type raw GitProject
	return yaml2.ReadYamlBytes(b, (*raw)(gp))
}

type GitRef struct {
	// Branch to use.
	// +optional
	Branch string `json:"branch,omitempty"`

	// Tag to use.
	// +optional
	Tag string `json:"tag,omitempty"`

	// Ref is only used to keep compatibility to the old string based ref field in GitProject
	// You are not allowed to use it directly
	Ref string `json:"-"`

	// Commit SHA to use.
	// +optional
	Commit string `json:"commit,omitempty"`
}

func (ref *GitRef) UnmarshalJSON(b []byte) error {
	if err := yaml2.ReadYamlBytes(b, &ref.Ref); err == nil {
		// it's a simple string
		return nil
	}
	ref.Ref = ""
	type raw GitRef
	err := yaml2.ReadYamlBytes(b, (*raw)(ref))
	if err != nil {
		return err
	}

	cnt := 0
	if ref.Tag != "" {
		cnt++
	}
	if ref.Branch != "" {
		cnt++
	}
	if ref.Commit != "" {
		cnt++
	}
	if cnt == 0 {
		return fmt.Errorf("either branch, tag or commit must be set")
	}
	if cnt != 1 {
		return fmt.Errorf("only one of the ref fields can be set")
	}
	return nil
}

func (ref GitRef) MarshalJSON() ([]byte, error) {
	if ref.Ref != "" {
		return json.Marshal(ref.Ref)
	}

	type raw GitRef
	return json.Marshal((*raw)(&ref))
}

func (ref *GitRef) String() string {
	if ref == nil {
		return ""
	}
	if ref.Tag != "" {
		return fmt.Sprintf("refs/tags/%s", ref.Tag)
	} else if ref.Branch != "" {
		return fmt.Sprintf("refs/heads/%s", ref.Branch)
	} else if ref.Ref != "" {
		return ref.Ref
	} else {
		return ""
	}
}

func ParseGitRef(s string) (GitRef, error) {
	if strings.HasPrefix(s, "refs/heads/") {
		return GitRef{Branch: strings.SplitN(s, "/", 3)[2]}, nil
	} else if strings.HasPrefix(s, "refs/tags/") {
		return GitRef{Tag: strings.SplitN(s, "/", 3)[2]}, nil
	} else {
		return GitRef{}, fmt.Errorf("can't parse %s as GitRef", s)
	}
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
	yaml2.Validator.RegisterStructValidation(ValidateGitProject, GitProject{})
}
