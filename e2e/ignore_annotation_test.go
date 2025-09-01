package e2e

import (
	"testing"

	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

func TestIgnoreAnnotation(t *testing.T) {
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
		annotations: map[string]string{
			"kluctl.io/ignore": "true",
		},
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")

	p.UpdateYaml("cm2/configmap-cm2.yml", func(o *uo.UnstructuredObject) error {
		o.SetK8sAnnotation("kluctl.io/ignore", "false")
		return nil
	}, "")
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
}
