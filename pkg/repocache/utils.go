package repocache

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"os"
	"path"
	"strings"
)

func findRepoOverride(repoOverrides []RepoOverride, repoKey types.RepoKey) (string, error) {
	for _, ro := range repoOverrides {
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
			return "", fmt.Errorf("can not override repo %s with %s: %w", repoKey.String(), overridePath, err)
		} else if !st.IsDir() {
			return "", fmt.Errorf("can not override repo %s. %s is not a directory", repoKey.String(), overridePath)
		}

		return overridePath, nil
	}
	return "", nil
}
