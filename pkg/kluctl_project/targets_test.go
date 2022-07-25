package kluctl_project

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTargetConfigFileFromLocal(t *testing.T) {
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

func TestLoadTargetConfigFileFromGit(t *testing.T) {
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	assert.NoError(t, err)

	wt, err := r.Worktree()
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(dir, "target-config.yml"), []byte("test"), 0600)
	assert.NoError(t, err)

	_, err = wt.Add("target-config.yml")
	assert.NoError(t, err)

	h, err := wt.Commit("test", &git.CommitOptions{})
	assert.NoError(t, err)

	commit, err := object.GetCommit(r.Storer, h)
	assert.NoError(t, err)

	gitTree, err := commit.Tree()
	assert.NoError(t, err)

	c := LoadedKluctlProject{}
	ti := &dynamicTargetInfo{
		baseTarget: &types.Target{
			TargetConfig: &types.ExternalTargetConfig{},
		},
		gitTree: gitTree,
	}

	data, err := c.loadTargetConfigFile(ti)
	assert.NoError(t, err)

	assert.Equal(t, []byte("test"), data)
}
