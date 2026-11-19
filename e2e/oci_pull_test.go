package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
)

func TestOciPull(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t)
	const contents = "test artifact\n"
	p.UpdateFile("nested/project.txt", func(string) (string, error) {
		return contents, nil
	}, "")

	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Oci, "", nil)
	repo.HttpServer.Username = "user"
	repo.HttpServer.Password = "password"
	repo.Start(t)

	repository := repo.URL.Host + "/org/project"
	registryCreds := repository + "=user:password"
	tag := repository + ":test"
	p.KluctlMust(t, "oci", "push", "--url", "oci://"+tag, "--registry-creds", registryCreds)

	digest, err := crane.Digest(tag, crane.WithContext(t.Context()), crane.WithAuth(&authn.Basic{
		Username: repo.HttpServer.Username,
		Password: repo.HttpServer.Password,
	}))
	if !assert.NoError(t, err) {
		return
	}

	for _, ref := range []string{tag, repository + "@" + digest} {
		outDir := filepath.Join(t.TempDir(), "project")
		pull := func() error {
			_, _, err := test_project.KluctlExecute(t, t.Context(), t.Log, "oci", "pull", "--url", "oci://"+ref,
				"--output-dir", outDir, "--registry-creds", registryCreds)
			return err
		}
		if !assert.NoError(t, pull()) {
			continue
		}
		assertContents := func() {
			b, err := os.ReadFile(filepath.Join(outDir, "nested", "project.txt"))
			if assert.NoError(t, err) {
				assert.Equal(t, contents, string(b))
			}
		}
		assertContents()

		assert.ErrorContains(t, pull(), "is not empty")
		assertContents()
	}
}
