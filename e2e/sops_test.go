package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"go.mozilla.org/sops/v3/age"
	"testing"
)

func TestSopsVars(t *testing.T) {
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	t.Setenv(age.SopsAgeKeyEnv, string(key))

	k := defaultCluster1

	p := test_utils.NewTestProject(t, k)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", map[string]string{
		"v1": "{{ test1.test2 }}",
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})
	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]map[string]any{
			{
				"file": "encrypted-vars.yaml",
			},
		}, "vars")
		return nil
	})

	p.UpdateFile("encrypted-vars.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
		return string(b), nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")

	cm := assertConfigMapExists(t, k, p.TestSlug(), "cm")
	assertNestedFieldEquals(t, cm, map[string]any{
		"v1": "42",
	}, "data")
}

func TestSopsResources(t *testing.T) {
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	t.Setenv(age.SopsAgeKeyEnv, string(key))

	k := defaultCluster1

	p := test_utils.NewTestProject(t, k)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)
	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(p.TestSlug(), "overrideNamespace")
		return nil
	})

	p.AddKustomizeDeployment("cm", []test_utils.KustomizeResource{
		{Name: "encrypted-cm.yaml"},
	}, nil)

	p.UpdateFile("cm/encrypted-cm.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test-configmap.yaml")
		return string(b), nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")

	cm := assertConfigMapExists(t, k, p.TestSlug(), "encrypted-cm")
	assertNestedFieldEquals(t, cm, map[string]any{
		"a": "b",
	}, "data")
}
