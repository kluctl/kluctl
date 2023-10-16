package types

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
