package sourceoverride

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"os"
	"path"
	"strings"
)

type Client interface {
	ResolveOverride(ctx context.Context, repoKey types.RepoKey) (*RepoOverride, string, error)
}

type RepoOverride struct {
	RepoKey  types.RepoKey `json:"repoKey"`
	Override string        `json:"override"`
	IsGroup  bool          `json:"isGroup"`
}

type Manager struct {
	repoOverrides []RepoOverride
}

func NewManager(overrides []RepoOverride) *Manager {
	return &Manager{
		repoOverrides: overrides,
	}
}

func (m *Manager) ResolveOverride(ctx context.Context, repoKey types.RepoKey) (*RepoOverride, string, error) {
	for _, ro := range m.repoOverrides {
		if ro.RepoKey.Type != repoKey.Type {
			continue
		}
		if ro.RepoKey.Host != repoKey.Host {
			continue
		}

		var overridePath string
		if ro.IsGroup {
			prefix := ro.RepoKey.Path + "/"
			if !strings.HasPrefix(repoKey.Path, prefix) {
				continue
			}
			relPath := strings.TrimPrefix(repoKey.Path, prefix)
			overridePath = path.Join(ro.Override, relPath)
		} else {
			if ro.RepoKey.Path != repoKey.Path {
				continue
			}
			overridePath = ro.Override
		}

		if st, err := os.Stat(overridePath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", fmt.Errorf("can not override repo %s with %s: %w", repoKey.String(), overridePath, err)
		} else if !st.IsDir() {
			return nil, "", fmt.Errorf("can not override repo %s. %s is not a directory", repoKey.String(), overridePath)
		}

		return &ro, overridePath, nil
	}
	return nil, "", nil
}
