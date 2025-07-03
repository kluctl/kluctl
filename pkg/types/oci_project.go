package types

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/kluctl/kluctl/lib/yaml"
	"strings"
)

type OciProject struct {
	Url    string  `json:"url" validate:"required"`
	Ref    *OciRef `json:"ref,omitempty"`
	SubDir string  `json:"subDir,omitempty"`
}

type OciRef struct {
	// Digest is the image digest to pull, takes precedence over SemVer.
	// The value should be in the format 'sha256:<HASH>'.
	// +optional
	Digest string `json:"digest,omitempty"`

	// Tag is the image tag to pull, defaults to latest.
	// +optional
	Tag string `json:"tag,omitempty"`
}

func (ref *OciRef) String() string {
	if ref == nil {
		return "latest"
	}

	if ref.Tag == "" && ref.Digest == "" {
		return "latest"
	}

	if ref.Digest != "" {
		return ref.Tag + "@" + ref.Digest
	}
	return ref.Tag
}

// ImageSuffix returns a string that can be appended to a repository name,
// e.g. :my-tag, :my-tag@sha256:xxx or @sha256:xxx (when no tag is set)
func (ref *OciRef) ImageSuffix() string {
	s := ref.String()
	if !strings.HasPrefix(s, "@") {
		s = ":" + s
	}
	return s
}

func ValidateOciProject(sl validator.StructLevel) {
	p := sl.Current().Interface().(OciProject)
	if !validateGitSubDir(p.SubDir) {
		sl.ReportError(p.SubDir, "subDir", "SubDir", fmt.Sprintf("'%s' is not valid oci subdirectory path", p.SubDir), "")
	}
}

func init() {
	yaml.Validator.RegisterStructValidation(ValidateOciProject, OciProject{})
}
