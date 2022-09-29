package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func doTestProject(t *testing.T, namespace string, p *testProject) {
	k := defaultKindCluster1

	p.init(t, k, fmt.Sprintf("project-%s", namespace))
	defer p.cleanup()

	createNamespace(t, k, namespace)

	p.updateKindCluster(k, uo.FromMap(map[string]interface{}{
		"cluster_var": "cluster_value1",
	}))
	p.updateTargetDeprecated("test", k.Context, uo.FromMap(map[string]interface{}{
		"target_var": "target_value1",
	}))
	addConfigMapDeployment(p, "cm1", map[string]string{}, resourceOpts{name: "cm1", namespace: namespace})

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertResourceExists(t, k, namespace, "ConfigMap/cm1")

	cmData := map[string]string{
		"cluster_var": "{{ cluster.cluster_var }}",
		"target_var":  "{{ args.target_var }}",
	}

	assertResourceNotExists(t, k, namespace, "ConfigMap/cm2")
	addConfigMapDeployment(p, "cm2", cmData, resourceOpts{name: "cm2", namespace: namespace})
	p.KluctlMust("deploy", "--yes", "-t", "test")

	o := assertResourceExists(t, k, namespace, "ConfigMap/cm2")
	assertNestedFieldEquals(t, o, "cluster_value1", "data", "cluster_var")
	assertNestedFieldEquals(t, o, "target_value1", "data", "target_var")

	p.updateKindCluster(k, uo.FromMap(map[string]interface{}{
		"cluster_var": "cluster_value2",
	}))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	o = assertResourceExists(t, k, namespace, "ConfigMap/cm2")
	assertNestedFieldEquals(t, o, "cluster_value2", "data", "cluster_var")
	assertNestedFieldEquals(t, o, "target_value1", "data", "target_var")

	p.updateTargetDeprecated("test", k.Context, uo.FromMap(map[string]interface{}{
		"target_var": "target_value2",
	}))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	o = assertResourceExists(t, k, namespace, "ConfigMap/cm2")
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
