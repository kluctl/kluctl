package helm

import (
	"github.com/Masterminds/semver/v3"
	"github.com/kluctl/kluctl/lib/git/types"
)

type ChartVersion struct {
	Version *string
	GitRef  *types.GitRef
}

func (v ChartVersion) String() string {
	if v.Version != nil {
		return *v.Version
	}
	if v.GitRef != nil {
		s := v.GitRef.String()
		if s == "" {
			return "HEAD"
		}
		return s
	}
	return "unknown"
}

func (v ChartVersion) Semver() *semver.Version {
	var s string
	if v.Version != nil {
		s = *v.Version
	} else if v.GitRef != nil {
		if v.GitRef.Tag != "" {
			s = v.GitRef.Tag
		}
	}
	if s == "" {
		return nil
	}
	x, err := semver.NewVersion(s)
	if err != nil {
		return nil
	}
	return x
}
