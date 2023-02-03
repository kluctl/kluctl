package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
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

func TestOnlyRender(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"path":       "only-render",
		"onlyRender": true,
	}))
	p.UpdateFile("only-render/value.txt", func(f string) (string, error) {
		return "{{ args.a }}\n", nil
	}, "")
	p.UpdateFile("only-render/kustomization.yaml", func(f string) (string, error) {
		return fmt.Sprintf(`
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component

generatorOptions:
  disableNameSuffixHash: true

configMapGenerator:
- name: %s-cm
  files:
  - value.txt
`, p.TestSlug()), nil
	}, "")

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"path": "d",
	}))
	p.UpdateFile("d/kustomization.yaml", func(f string) (string, error) {
		return fmt.Sprintf(`
components:
- ../only-render
namespace: %s
`, p.TestSlug()), nil
	}, "")

	p.KluctlMust("deploy", "--yes", "-t", "test", "-a", "a=v1")
	// it should not appear in the default namespace as that would indicate that the component was treated as a deployment item
	assertConfigMapNotExists(t, k, "default", p.TestSlug()+"-cm")
	s := assertConfigMapExists(t, k, p.TestSlug(), p.TestSlug()+"-cm")
	assert.Equal(t, s.Object["data"], map[string]any{
		"value.txt": "v1",
	})
}
