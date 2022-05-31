package e2e

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/client-go/tools/clientcmd/api"
	"sync"
	"testing"
)

func prepareContextTest(t *testing.T, name string) *testProject {
	p := &testProject{}
	p.init(t, defaultKindCluster1, name)
	p.mergeKubeconfig(defaultKindCluster2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		recreateNamespace(t, defaultKindCluster1, p.projectName)
	}()
	go func() {
		defer wg.Done()
		recreateNamespace(t, defaultKindCluster2, p.projectName)
	}()
	wg.Wait()

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.projectName,
	})

	return p
}

func TestContextCurrent(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-current")
	defer p.cleanup()

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		// no context set, assume the current one is used
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster1, p.projectName, "ConfigMap/cm")
	assertResourceNotExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")

	p.updateMergedKubeconfig(func(config *api.Config) {
		config.CurrentContext = defaultKindCluster2.Context
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")
}

func TestContext1(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-1")
	defer p.cleanup()

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster1.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster1, p.projectName, "ConfigMap/cm")
	assertResourceNotExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")
}

func TestContext2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-2")
	defer p.cleanup()

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")
	assertResourceNotExists(t, defaultKindCluster1, p.projectName, "ConfigMap/cm")
}

func TestContext1And2(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-1-and-2")
	defer p.cleanup()

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster1.Context, "context")
	})
	p.updateTarget("test2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster1, p.projectName, "ConfigMap/cm")
	assertResourceNotExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")

	p.KluctlMust("deploy", "--yes", "-t", "test2")
	assertResourceExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")
}

func TestContextSwitch(t *testing.T) {
	t.Parallel()

	p := prepareContextTest(t, "context-switch")
	defer p.cleanup()

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster1.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster1, p.projectName, "ConfigMap/cm")
	assertResourceNotExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")

	p.updateTarget("test1", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultKindCluster2.Context, "context")
	})

	p.KluctlMust("deploy", "--yes", "-t", "test1")
	assertResourceExists(t, defaultKindCluster2, p.projectName, "ConfigMap/cm")
}
