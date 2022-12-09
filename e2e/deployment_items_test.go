package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func TestKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm")

	addConfigMapDeployment(p, "cm2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})
	p.KluctlMust("deploy", "--yes", "-t", "test", "--dry-run")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
}

func TestGeneratedKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]any{
			map[string]any{
				"path": "generated-kustomize",
			},
		}, "deployments")
		return nil
	})
	p.UpdateYaml("generated-kustomize/cm1.yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm1",
			namespace: p.TestSlug(),
		})
		return nil
	}, "")
	p.UpdateYaml("generated-kustomize/cm2.yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm2",
			namespace: p.TestSlug(),
		})
		return nil
	}, "")
	p.UpdateYaml("generated-kustomize/cm3._yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm3",
			namespace: p.TestSlug(),
		})
		return nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm3")
}
