package e2e

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/tools/clientcmd/api"
	"testing"
)

func prepareContextTest(t *testing.T, name string) *testProject {
	p := &testProject{}
	p.init(t, defaultCluster1, name)
	p.mergeKubeconfig(defaultCluster2)

	createNamespace(t, defaultCluster1, p.projectName)
	createNamespace(t, defaultCluster2, p.projectName)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.projectName,
	})

	return p
}

func TestContextCurrent(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-current")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		// no context set, assume the current one is used
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")

	p.updateMergedKubeconfig(func(config *api.Config) {
		config.CurrentContext = defaultCluster2.Context
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
}

func TestContext1(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-1")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")
}

func TestContext2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-2")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster1, p.projectName, "cm")
}

func TestContext1And2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-1-and-2")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})
	p.updateTarget("test2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")

	p.KluctlMust("deploy", "--yes", "-t", "test2")
	assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
}

func TestContextSwitch(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-switch")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
}

func TestContextOverride(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-override")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertConfigMapExists(t, defaultCluster1, p.projectName, "cm")
	assertConfigMapNotExists(t, defaultCluster2, p.projectName, "cm")

	p.KluctlMust("deploy", "--yes", "-t", "test1", "--context", defaultCluster2.Context)
	assertConfigMapExists(t, defaultCluster2, p.projectName, "cm")
}
