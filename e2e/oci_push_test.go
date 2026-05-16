package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/oci"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
)

func TestOciPushWithGitignore(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t,
		test_project.WithRepoName("repos/r1"),
	)
	addConfigMapDeployment(p, "cm", map[string]string{"a": "v"}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm", map[string]string{"a": "v"}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	repo := test_utils.NewHelmTestRepo(test_utils.TestHelmRepo_Oci, "", nil)
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"

	opts := oci.DefaultOptions()
	opts = append(opts, crane.WithAuth(&authn.Basic{
		Username: repo.HttpServer.Username,
		Password: repo.HttpServer.Password,
	}))
	c := oci.NewClient(opts)

	testPush := func(gitignore string, expectMissing []string, expectExisting []string) {
		p.UpdateFile(".gitignore", func(f string) (string, error) {
			return gitignore, nil
		}, "")
		p.KluctlMust(t, "oci", "push", "--url", repo1)

		outPath := t.TempDir()
		_, err := c.Pull(t.Context(), strings.TrimLeft(repo1, "oci://"), outPath)
		assert.NoError(t, err)

		for _, f := range expectMissing {
			assert.NoFileExists(t, filepath.Join(outPath, f))
		}
		for _, f := range expectExisting {
			assert.FileExists(t, filepath.Join(outPath, f))
		}
	}

	testPush("cm/configmap-cm1.yml", []string{"cm/configmap-cm1.yml"}, []string{"cm/configmap-cm2.yml"})
	testPush("cm/configmap-cm2.yml", []string{"cm/configmap-cm2.yml"}, []string{"cm/configmap-cm1.yml"})
	testPush("cm/configmap-*", []string{"cm/configmap-cm1.yml", "cm/configmap-cm2.yml"}, nil)
	testPush("cm/configmap-*\n!cm/configmap-cm1.yml", []string{"cm/configmap-cm2.yml"}, []string{"cm/configmap-cm1.yml"})
	testPush("cm\n!cm/configmap-cm1.yml", []string{"cm/configmap-cm2.yml"}, []string{"cm/configmap-cm1.yml"})
}
