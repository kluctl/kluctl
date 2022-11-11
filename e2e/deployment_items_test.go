package e2e

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func TestKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	p.updateTarget("test", nil)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.testSlug(),
	})
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "cm")

	addConfigMapDeployment(p, "cm2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.testSlug(),
	})
	p.KluctlMust("deploy", "--yes", "-t", "test", "--dry-run")
	assertConfigMapNotExists(t, k, p.testSlug(), "cm2")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "cm2")
}

func TestGeneratedKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	p.updateTarget("test", nil)

	p.updateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]any{
			map[string]any{
				"path": "generated-kustomize",
			},
		}, "deployments")
		return nil
	})
	p.updateYaml("generated-kustomize/cm1.yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm1",
			namespace: p.testSlug(),
		})
		return nil
	}, "")
	p.updateYaml("generated-kustomize/cm2.yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm2",
			namespace: p.testSlug(),
		})
		return nil
	}, "")
	p.updateYaml("generated-kustomize/cm3._yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(nil, resourceOpts{
			name:      "cm3",
			namespace: p.testSlug(),
		})
		return nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "cm1")
	assertConfigMapExists(t, k, p.testSlug(), "cm2")
	assertConfigMapNotExists(t, k, p.testSlug(), "cm3")
}
