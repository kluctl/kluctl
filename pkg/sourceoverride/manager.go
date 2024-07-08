package sourceoverride

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/lib/git/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Resolver interface {
	ResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error)
}

type RepoOverride struct {
	RepoKey  types.RepoKey `json:"repoKey"`
	Override string        `json:"override"`
	IsGroup  bool          `json:"isGroup"`
}

func (ro *RepoOverride) Match(repoKey types.RepoKey) (string, bool) {
	if ro.RepoKey.Type != repoKey.Type {
		return "", false
	}
	if ro.RepoKey.Host != repoKey.Host {
		return "", false
	}

	var overridePath string
	if ro.IsGroup {
		prefix := ro.RepoKey.Path + "/"
		if !strings.HasPrefix(repoKey.Path, prefix) {
			return "", false
		}
		relPath := strings.TrimPrefix(repoKey.Path, prefix)
		overridePath = path.Join(ro.Override, relPath)
	} else {
		if ro.RepoKey.Path != repoKey.Path {
			return "", false
		}
		overridePath = ro.Override
	}

	return overridePath, true
}

type Manager struct {
	Overrides []RepoOverride
}

func NewManager(overrides []RepoOverride) *Manager {
	return &Manager{
		Overrides: overrides,
	}
}

func (m *Manager) ResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	for _, ro := range m.Overrides {
		overridePath, ok := ro.Match(repoKey)
		if !ok {
			continue
		}

		overridePath = filepath.FromSlash(overridePath)

		if st, err := os.Stat(overridePath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("can not override repo %s with %s: %w", repoKey.String(), overridePath, err)
		} else if !st.IsDir() {
			return "", fmt.Errorf("can not override repo %s. %s is not a directory", repoKey.String(), overridePath)
		}

		err := utils.CheckInDir(ro.Override, overridePath)
		if err != nil {
			return "", fmt.Errorf("can not override repo %s with %s: %w", repoKey.String(), overridePath, err)
		}

		return overridePath, nil
	}
	return "", nil
}
