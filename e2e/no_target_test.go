package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func prepareNoTargetTest(t *testing.T, withDeploymentYaml bool) *test_project.TestProject {
	p := test_project.NewTestProject(t)

	cm := createConfigMapObject(map[string]string{
		"targetName":    `{{ target.name }}`,
		"targetContext": `{{ target.context }}`,
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	if withDeploymentYaml {
		p.AddKustomizeDeployment("cm", []test_project.KustomizeResource{{Name: "cm.yaml", Content: cm}}, nil)
	} else {
		p.AddKustomizeResources("", []test_project.KustomizeResource{{Name: "cm.yaml", Content: cm}})
		err := os.Remove(filepath.Join(p.LocalProjectDir(), "deployment.yml"))
		assert.NoError(t, err)
	}

	return p
}

func testNoTarget(t *testing.T, withDeploymentYaml bool) {
	t.Parallel()

	p := prepareNoTargetTest(t, withDeploymentYaml)
	createNamespace(t, defaultCluster1, p.TestSlug())
	createNamespace(t, defaultCluster2, p.TestSlug())

	p.KluctlMust(t, "deploy", "--yes")
	cm := assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "",
		"targetContext": defaultCluster1.Context,
	}, cm.Object["data"])

	p.KluctlMust(t, "deploy", "--yes", "-T", "override-name")
	cm = assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "override-name",
		"targetContext": defaultCluster1.Context,
	}, cm.Object["data"])

	p.KluctlMust(t, "deploy", "--yes", "-T", "override-name", "--context", defaultCluster2.Context)
	cm = assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "override-name",
		"targetContext": defaultCluster2.Context,
	}, cm.Object["data"])
}

func TestNoTarget(t *testing.T) {
	testNoTarget(t, true)
}

func TestNoTargetNoDeployment(t *testing.T) {
	testNoTarget(t, false)
}

func TestNoTargetWithTargetsInProject(t *testing.T) {
	t.Parallel()

	p := prepareNoTargetTest(t, true)
	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	_, _, err := p.Kluctl(t, "deploy", "--yes")
	assert.ErrorContains(t, err, "a target must be explicitly selected when targets are defined in the Kluctl project")
}
