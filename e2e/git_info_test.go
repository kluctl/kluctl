package e2e

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	copy2 "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func copyProject(t *testing.T, p *test_project.TestProject) string {
	tmp := t.TempDir()
	err := copy2.Copy(p.LocalWorkDir(), tmp, copy2.Options{
		Skip: func(srcinfo os.FileInfo, src, dest string) (bool, error) {
			if srcinfo.Name() == ".git" {
				return true, nil
			}
			return false, nil
		},
	})
	assert.NoError(t, err)

	return tmp
}

func prepareGitRepoTest(t *testing.T) (*test_project.TestProject, string) {
	k := defaultCluster1

	p := test_project.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	tmp := copyProject(t, p)

	return p, tmp
}

func TestNGitNoRepo(t *testing.T) {
	t.Parallel()

	p, tmp := prepareGitRepoTest(t)

	// no repo at all
	r, _ := p.KluctlMustCommandResult(t, "deploy", "--project-dir", tmp, "--yes", "-oyaml")
	assert.Equal(t, gittypes.GitInfo{}, r.GitInfo)
}

func TestGitNoCommit(t *testing.T) {
	t.Parallel()

	p, tmp := prepareGitRepoTest(t)

	// empty repo (no commit)
	_, err := git.PlainInit(tmp, false)
	assert.NoError(t, err)
	r, _ := p.KluctlMustCommandResult(t, "deploy", "--project-dir", tmp, "--yes", "-oyaml")
	assert.Equal(t, gittypes.GitInfo{
		Dirty: true,
	}, r.GitInfo)
}

func TestGitSingleCommit(t *testing.T) {
	t.Parallel()

	p, tmp := prepareGitRepoTest(t)

	// empty repo (no commit)
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

	_, err = w.Add(".")
	assert.NoError(t, err)
	c, err := w.Commit("initial", &git.CommitOptions{})
	assert.NoError(t, err)

	r, _ := p.KluctlMustCommandResult(t, "deploy", "--project-dir", tmp, "--yes", "-oyaml")
	assert.Equal(t, gittypes.GitInfo{
		Ref: &gittypes.GitRef{
			Branch: "master",
		},
		Commit: c.String(),
		Dirty:  false,
	}, r.GitInfo)

	_, err = gr.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://example.com"},
	})
	assert.NoError(t, err)

	r, _ = p.KluctlMustCommandResult(t, "deploy", "--project-dir", tmp, "--yes", "-oyaml")
	assert.Equal(t, gittypes.GitInfo{
		Url: gittypes.ParseGitUrlMust("https://example.com"),
		Ref: &gittypes.GitRef{
			Branch: "master",
		},
		Commit: c.String(),
		Dirty:  false,
	}, r.GitInfo)
}
