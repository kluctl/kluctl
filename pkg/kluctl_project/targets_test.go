package kluctl_project

import (
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTargetConfigFile(t *testing.T) {
	c := LoadedKluctlProject{}
	ti := &dynamicTargetInfo{
		baseTarget: &types.Target{
			TargetConfig: &types.ExternalTargetConfig{},
		},
		dir: t.TempDir(),
	}
	err := os.WriteFile(filepath.Join(ti.dir, "target-config.yml"), []byte("test"), 0600)
	assert.NoError(t, err)

	data, err := c.loadTargetConfigFile(ti)
	assert.NoError(t, err)

	assert.Equal(t, []byte("test"), data)
}
