package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestOciProjectUrl(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	p := test_project.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())
	p.UpdateTarget("test", nil)
	addConfigMapDeployment(p, "cm", map[string]string{"source": "artifact"}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Oci, "", nil)
	repo.HttpServer.Username = "user"
	repo.HttpServer.Password = "password"
	repo.Start(t)

	repository := repo.URL.Host + "/org/project"
	projectURL := "oci://" + repository + ":test"
	registryCreds := repository + "=user:password"
	stdout, _ := p.KluctlMust(t, "oci", "push", "--url", projectURL, "--registry-creds", registryCreds, "--output", "json")
	var artifact struct {
		URL string `json:"url"`
	}
	if !assert.NoError(t, json.Unmarshal([]byte(stdout), &artifact)) || !assert.NotEmpty(t, artifact.URL) {
		return
	}

	tmpDir := t.TempDir()
	cacheDir := t.TempDir()
	ctx := utils.WithTmpBaseDir(t.Context(), tmpDir)
	ctx = utils.WithCacheDir(ctx, cacheDir)
	run := func(args ...string) (string, error) {
		stdout, _, err := test_project.KluctlExecute(t, ctx, t.Log, append(args, "--registry-creds", registryCreds)...)
		return stdout, err
	}
	assertCleanedUp := func() {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(utils.GetTmpBaseDir(ctx), "oci-pulled"))
		assert.NoError(t, err)
		assert.Empty(t, entries)
	}

	_, err := run("deploy", "--project-url", projectURL, "-t", "test", "--yes")
	if assert.NoError(t, err) {
		assertConfigMapExists(t, k, p.TestSlug(), "cm")
	}
	assertCleanedUp()
	entries, err := os.ReadDir(filepath.Join(cacheDir, "oci"))
	assert.NoError(t, err)
	assert.NotEmpty(t, entries)

	_, err = run("diff", "--project-url", artifact.URL, "-t", "test")
	assert.NoError(t, err)
	assertCleanedUp()

	stdout, err = run("list-targets", "--project-url", projectURL, "--only-names")
	assert.NoError(t, err)
	assert.Equal(t, "test\n", stdout)

	_, err = run("diff", "--project-url", projectURL, "-t", "missing")
	assert.ErrorContains(t, err, "missing")
	assertCleanedUp()

	_, err = run("list-targets", "--project-url", "https://example.com/project")
	assert.ErrorContains(t, err, "must use the oci:// scheme")
	_, err = run("list-targets", "--project-url", projectURL, "--project-dir", p.LocalProjectDir())
	assert.ErrorContains(t, err, "--project-url and --project-dir cannot be used together")
}
