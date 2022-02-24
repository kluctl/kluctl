package e2e

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"testing"
	"time"
)

func doTestProject(t *testing.T, namespace string, p *testProject) {
	k := defaultKindCluster

	p.init(t, fmt.Sprintf("project-%s", namespace))
	defer p.cleanup()

	recreateNamespace(t, k, namespace)

	p.updateKindCluster(k, uo.FromMap(map[string]interface{}{
		"cluster_var": "cluster_value1",
	}))
	p.updateTarget("test", k.Name, uo.FromMap(map[string]interface{}{
		"target_var": "target_value1",
	}))
	addBusyboxDeployment(p, "busybox", resourceOpts{name: "busybox", namespace: namespace})

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertReadiness(t, k, namespace, "Deployment/busybox", time.Minute*5)

	cmData := map[string]string{
		"cluster_var": "{{ cluster.cluster_var }}",
		"target_var":  "{{ args.target_var }}",
	}

	assertResourceNotExists(t, k, namespace, "ConfigMap/cm")
	addConfigMapDeployment(p, "cm", cmData, resourceOpts{name: "cm", namespace: namespace})
	p.KluctlMust("deploy", "--yes", "-t", "test")

	o := assertResourceExists(t, k, namespace, "ConfigMap/cm")
	assertNestedFieldEquals(t, o, "cluster_value1", "data", "cluster_var")
	assertNestedFieldEquals(t, o, "target_value1", "data", "target_var")

	p.updateKindCluster(k, uo.FromMap(map[string]interface{}{
		"cluster_var": "cluster_value2",
	}))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	o = assertResourceExists(t, k, namespace, "ConfigMap/cm")
	assertNestedFieldEquals(t, o, "cluster_value2", "data", "cluster_var")
	assertNestedFieldEquals(t, o, "target_value1", "data", "target_var")

	p.updateTarget("test", k.Name, uo.FromMap(map[string]interface{}{
		"target_var": "target_value2",
	}))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	o = assertResourceExists(t, k, namespace, "ConfigMap/cm")
	assertNestedFieldEquals(t, o, "cluster_value2", "data", "cluster_var")
	assertNestedFieldEquals(t, o, "target_value2", "data", "target_var")
}

func TestExternalProjects(t *testing.T) {
	testCases := []struct {
		name string
		p    testProject
	}{
		{name: "external-kluctl-project", p: testProject{kluctlProjectExternal: true}},
		{name: "external-clusters-project", p: testProject{clustersExternal: true}},
		{name: "external-deployment-project", p: testProject{deploymentExternal: true}},
		{name: "external-sealed-secrets-project", p: testProject{sealedSecretsExternal: true}},
		{name: "external-all-projects", p: testProject{kluctlProjectExternal: true, clustersExternal: true, deploymentExternal: true, sealedSecretsExternal: true}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doTestProject(t, tc.name, &tc.p)
		})
	}
}
