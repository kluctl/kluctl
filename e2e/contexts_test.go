package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func prepareContextTest(t *testing.T) *test_project.TestProject {
	p := test_project.NewTestProject(t)

	createNamespace(t, defaultCluster1, p.TestSlug())
	createNamespace(t, defaultCluster2, p.TestSlug())

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	return p
}

func TestContextCurrent(t *testing.T) {
	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		// no context set, assume the current one is used
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")

	setMergedKubeconfigContext(t, defaultCluster2.Context)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
}

func TestContext1(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")
}

func TestContext2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster1, p.TestSlug(), "cm")
}

func TestContext1And2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})
	p.UpdateTarget("test2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test2")
	assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
}

func TestContextSwitch(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
}

func TestContextOverride(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t)

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.TestSlug(), "cm")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1", "--context", defaultCluster2.Context)
	assertConfigMapExists(t, defaultCluster2, p.TestSlug(), "cm")
}
