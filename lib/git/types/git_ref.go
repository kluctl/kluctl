package types

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kluctl/kluctl/lib/yaml"
)

type GitRef struct {
	// Branch to use.
	// +optional
	Branch string `json:"branch,omitempty"`

	// Tag to use.
	// +optional
	Tag string `json:"tag,omitempty"`

	// Commit SHA to use.
	// +optional
	Commit string `json:"commit,omitempty"`
}

func (ref *GitRef) UnmarshalJSON(b []byte) error {
	type raw GitRef
	err := yaml.ReadYamlBytes(b, (*raw)(ref))
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
