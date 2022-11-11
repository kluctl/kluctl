package e2e

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"go.mozilla.org/sops/v3/age"
	"testing"
)

func TestSopsVars(t *testing.T) {
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	t.Setenv(age.SopsAgeKeyEnv, string(key))

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k, "sops-vars")

	createNamespace(t, k, p.projectName)

	p.updateTarget("test", nil)

	addConfigMapDeployment(p, "cm", map[string]string{
		"v1": "{{ test1.test2 }}",
	}, resourceOpts{
		name:      "cm",
		namespace: p.projectName,
	})
	p.updateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]map[string]any{
			{
				"file": "encrypted-vars.yaml",
			},
		}, "vars")
		return nil
	})

	p.updateFile("encrypted-vars.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
		return string(b), nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")

	cm := assertConfigMapExists(t, k, p.projectName, "cm")
	assertNestedFieldEquals(t, cm, map[string]any{
		"v1": "42",
	}, "data")
}

func TestSopsResources(t *testing.T) {
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	t.Setenv(age.SopsAgeKeyEnv, string(key))

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k, "sops-resources")

	createNamespace(t, k, p.projectName)

	p.updateTarget("test", nil)
	p.updateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(p.projectName, "overrideNamespace")
		return nil
	})

	p.addKustomizeDeployment("cm", []kustomizeResource{
		{name: "encrypted-cm.yaml"},
	}, nil)

	p.updateFile("cm/encrypted-cm.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test-configmap.yaml")
		return string(b), nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")

	cm := assertConfigMapExists(t, k, p.projectName, "encrypted-cm")
	assertNestedFieldEquals(t, cm, map[string]any{
		"a": "b",
	}, "data")
}
