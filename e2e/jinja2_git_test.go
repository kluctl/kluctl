package e2e

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/lib/go-jinja2"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"os"
	"testing"
)

func TestJinjaLoadLatestGitSha(t *testing.T) {
	t.Parallel()

	// empty repo (no commit)
	tmp := t.TempDir()
	gr, err := git.PlainInit(tmp, false)
	assert.NoError(t, err)

	cfg, err := gr.Config()
	assert.NoError(t, err)
	cfg.User.Name = "Test User"
	cfg.User.Email = "no@mail.com"
	cfg.Author = cfg.User
	cfg.Committer = cfg.User
	err = gr.SetConfig(cfg)
	assert.NoError(t, err)

	w, err := gr.Worktree()
	assert.NoError(t, err)

	// create file and commit
	filename := "README.md"
	err = os.WriteFile(fmt.Sprintf("%s/%s", tmp, filename), []byte("Hello World!"), 0644)
	_, err = w.Add(filename)
	assert.NoError(t, err)
	c, err := w.Commit("initial", &git.CommitOptions{})
	assert.NoError(t, err)

	// create jinja2 instance
	name := fmt.Sprintf("jinja2-%d", rand.Uint32())
	j2, err := jinja2.NewJinja2(name, 1, jinja2.WithSearchDir(tmp), jinja2.WithExtension("go_jinja2.ext.kluctl"))
	if err != nil {
		t.Fatalf("failed to create jinja2: %v", err)
	}
	t.Cleanup(j2.Close)
	t.Cleanup(j2.Cleanup)

	s, err := j2.RenderString("{{ load_latest_git_sha('./README.md') }}")
	assert.NoError(t, err)
	assert.Equal(t, 40, len(s)) // 40 is sha length (from `man git-rev-parse`: "The full SHA-1 object name (40-byte hexadecimal string)").
	assert.Equal(t, c.String(), s)
}
