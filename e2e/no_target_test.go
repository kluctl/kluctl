package e2e

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func prepareNoTargetTest(t *testing.T, name string) *testProject {
	p := &testProject{}
	p.init(t, defaultCluster1, name)
	p.mergeKubeconfig(defaultCluster2)

	createNamespace(t, defaultCluster1, p.projectName)
	createNamespace(t, defaultCluster2, p.projectName)

	addConfigMapDeployment(p, "cm", map[string]string{
		"targetName":    `{{ target.name }}`,
		"targetContext": `{{ target.context }}`,
	}, resourceOpts{
		name:      "cm",
		namespace: p.projectName,
	})

	return p
}

func TestNoTarget(t *testing.T) {
	t.Parallel()

	p := prepareNoTargetTest(t, "no-target")

	p.KluctlMust("deploy", "--yes")
	cm := assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "",
		"targetContext": defaultCluster1.Context,
	}, cm.Object["data"])

	p.KluctlMust("deploy", "--yes", "-T", "override-name")
	cm = assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "override-name",
		"targetContext": defaultCluster1.Context,
	}, cm.Object["data"])

	p.KluctlMust("deploy", "--yes", "-T", "override-name", "--context", defaultCluster2.Context)
	cm = assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
	assert.Equal(t, map[string]any{
		"targetName":    "override-name",
		"targetContext": defaultCluster2.Context,
	}, cm.Object["data"])
}
