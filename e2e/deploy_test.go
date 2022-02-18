package e2e

import "testing"

func TestCommandDeploySimple(t *testing.T) {
    t.Parallel()
    p := &testProject{}
    p.init(t, "simple")
    defer p.cleanup()

    k := defaultKindCluster
    recreateNamespace(t, k, p.projectName)

    p.updateKindCluster(k, nil)
    p.updateTarget("test", k.Name, nil)

    addConfigMapDeployment(p, "cm", nil, resourceOpts{
        name:      "cm",
        namespace: p.projectName,
    })
    p.KluctlMust("deploy", "--yes", "-t", "test")
    assertResourceExists(t, k, p.projectName, "ConfigMap/cm")

    addConfigMapDeployment(p, "cm2", nil, resourceOpts{
        name:      "cm2",
        namespace: p.projectName,
    })
    p.KluctlMust("deploy", "--yes", "-t", "test", "--dry-run")
    assertResourceNotExists(t, k, p.projectName, "ConfigMap/cm2")
    p.KluctlMust("deploy", "--yes", "-t", "test")
    assertResourceExists(t, k, p.projectName, "ConfigMap/cm2")
}
