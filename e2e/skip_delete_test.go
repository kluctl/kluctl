package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestSkipDelete(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", map[string]string{}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm3", map[string]string{}, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"kluctl.io/skip-delete": "true",
		},
	})
	addConfigMapDeployment(p, "cm4", map[string]string{}, resourceOpts{
		name:      "cm4",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"helm.sh/resource-policy": "keep",
		},
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	cm2 := assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
	assertConfigMapExists(t, k, p.TestSlug(), "cm4")

	cm2.SetK8sAnnotation("kluctl.io/skip-delete", "true")
	updateObject(t, k, cm2)

	p.KluctlMust(t, "delete", "--yes", "-t", "test")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm1")
	cm2 = assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
	assertConfigMapExists(t, k, p.TestSlug(), "cm4")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	cm1.SetK8sAnnotation("kluctl.io/skip-delete", "true")
	cm2.SetK8sAnnotation("kluctl.io/skip-delete", "false")
	updateObject(t, k, cm1)
	updateObject(t, k, cm2)
	p.DeleteKustomizeDeployment("cm1")
	p.DeleteKustomizeDeployment("cm2")
	p.DeleteKustomizeDeployment("cm3")
	p.KluctlMust(t, "prune", "--yes", "-t", "test")
	cm1 = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
	assertConfigMapExists(t, k, p.TestSlug(), "cm4")
}

func TestForceReplaceSkipDelete(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{
		"k1": "v1",
	}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"kluctl.io/skip-delete": "true",
		},
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assert.Equal(t, map[string]any{
		"k1": "v1",
	}, cm1.Object["data"])

	p.UpdateYaml("cm1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v2", "data", "k1")
		return nil
	}, "")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assert.Equal(t, map[string]any{
		"k1": "v2",
	}, cm1.Object["data"])

	invalidLabel := "invalid_label_" + strings.Repeat("x", 63)
	p.UpdateYaml("cm1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		o.SetK8sLabel(invalidLabel, "invalid_label")
		return nil
	}, "")
	stdout, _, err := p.Kluctl(t, "deploy", "--yes", "-t", "test")
	assert.Error(t, err)
	assert.Contains(t, stdout, "invalid: metadata.labels")
	// make sure it did not try to replace it
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")

	stdout, stderr, err := p.Kluctl(t, "deploy", "--yes", "-t", "test", "--force-replace-on-error")
	assert.Error(t, err)
	assert.Contains(t, stdout, "retrying with replace instead of patch")
	assert.Contains(t, stderr, "skipped forced replace")
	assert.Contains(t, stdout, "invalid: metadata.labels")
	// make sure it did not try to replace it
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
}
