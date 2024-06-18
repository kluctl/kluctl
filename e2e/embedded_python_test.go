package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedPython(t *testing.T) {
	cacheDir := t.TempDir()

	p := test_project.NewTestProject(t, test_project.WithCacheDir(cacheDir))
	p.SetEnv("KLUCTL_TEST_JINJA2_CACHE", "1")

	addConfigMapDeployment(p, "cm", map[string]string{
		"k": `{{ "magic" }}{{ 40 + 2 }}`,
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	stdout, _ := p.KluctlProcessMust(t, "render", "--print-all")
	assert.Contains(t, stdout, "magic42")

	des, err := os.ReadDir(filepath.Join(cacheDir, "go-embed-jinja2"))
	assert.NoError(t, err)

	found := false
	for _, de := range des {
		if strings.HasPrefix(de.Name(), "kluctl-python-") {
			found = true
		}
	}
	assert.Truef(t, found, "extracted python distribution not found")
}

func TestSystemPython(t *testing.T) {
	cacheDir := t.TempDir()

	p := test_project.NewTestProject(t, test_project.WithCacheDir(cacheDir))
	p.SetEnv("KLUCTL_TEST_JINJA2_CACHE", "1")
	p.SetEnv("KLUCTL_USE_SYSTEM_PYTHON", "1")

	addConfigMapDeployment(p, "cm", map[string]string{
		"k": `{{ "magic" }}{{ 40 + 2 }}`,
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	stdout, _ := p.KluctlProcessMust(t, "render", "--print-all")
	assert.Contains(t, stdout, "magic42")

	des, err := os.ReadDir(filepath.Join(cacheDir, "go-embed-jinja2"))
	assert.NoError(t, err)

	found := false
	for _, de := range des {
		if strings.HasPrefix(de.Name(), "kluctl-python-") {
			found = true
		}
	}
	assert.Falsef(t, found, "extracted python distribution found even though we should have used the system distribution")
}
