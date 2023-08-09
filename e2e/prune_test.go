package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"testing"
)

func TestPrune(t *testing.T) {
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

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")

	p.DeleteKustomizeDeployment("cm2")

	p.KluctlMust("prune", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
}

func TestDeployWithPrune(t *testing.T) {
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

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")

	p.DeleteKustomizeDeployment("cm2")
	addConfigMapDeployment(p, "cm3", map[string]string{}, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
	})

	p.KluctlMust("deploy", "--yes", "-t", "test", "--prune")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
}
