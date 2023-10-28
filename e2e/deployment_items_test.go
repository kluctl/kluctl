package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm")

	addConfigMapDeployment(p, "cm2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})
	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--dry-run")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
}

func TestGeneratedKustomize(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

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

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm3")
}

func TestOnlyRender(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

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

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "-a", "a=v1")
	// it should not appear in the default namespace as that would indicate that the component was treated as a deployment item
	assertConfigMapNotExists(t, k, "default", p.TestSlug()+"-cm")
	s := assertConfigMapExists(t, k, p.TestSlug(), p.TestSlug()+"-cm")
	assert.Equal(t, s.Object["data"], map[string]any{
		"value.txt": "v1",
	})
}

func TestKustomizeBase(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "base", map[string]string{}, resourceOpts{
		name:      "base-cm",
		namespace: p.TestSlug(),
	})
	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(true, "onlyRender")
		return items
	})

	p.AddKustomizeDeployment("k1", []test_project.KustomizeResource{{
		Name: "../base",
	}}, nil)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "base-cm")
}

func TestTemplateIgnore(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{
		"k1": `{{ "a" }}`,
	}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", map[string]string{
		"k1": `{{ "a" }}`,
	}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm3", map[string]string{
		"k1": `{{ "a" }}`,
	}, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
	})

	// .templateignore outside of deployment item
	p.UpdateFile(".templateignore", func(f string) (string, error) {
		return `cm2/configmap-cm2.yml`, nil
	}, "")
	// .templateignore inside of deployment item
	p.UpdateFile("cm3/.templateignore", func(f string) (string, error) {
		return `/configmap-cm3.yml`, nil
	}, "")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	cm2 := assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	cm3 := assertConfigMapExists(t, k, p.TestSlug(), "cm3")

	assert.Equal(t, map[string]any{
		"k1": "a",
	}, cm1.Object["data"])
	assert.Equal(t, map[string]any{
		"k1": `{{ "a" }}`,
	}, cm2.Object["data"])
	assert.Equal(t, map[string]any{
		"k1": `{{ "a" }}`,
	}, cm3.Object["data"])
}

func testLocalIncludes(t *testing.T, projectDir string) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t, test_project.WithBareProject())

	createNamespace(t, k, p.TestSlug())

	p.UpdateDeploymentYaml("base", func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"deployments": []map[string]any{
				{"path": "cm"},
			},
		})
		return nil
	})
	p.UpdateYaml("base/cm/cm.yaml", func(o *uo.UnstructuredObject) error {
		*o = *createConfigMapObject(map[string]string{
			"d1": "v1",
		}, resourceOpts{name: "{{ name }}", namespace: p.TestSlug()})
		return nil
	}, "")

	baseDir, _ := filepath.Rel(filepath.Join(p.LocalProjectDir(), projectDir), filepath.Join(p.LocalRepoDir(), "base"))
	baseDir = filepath.ToSlash(baseDir)

	p.UpdateDeploymentYaml(projectDir, func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"deployments": []map[string]any{
				{
					"include": baseDir,
					"vars": []map[string]any{
						{
							"values": map[string]any{
								"name": "cm-inc1",
							},
						},
					},
				},
				{
					"include": baseDir,
					"vars": []map[string]any{
						{
							"values": map[string]any{
								"name": "cm-inc2",
							},
						},
					},
				},
			},
		})
		return nil
	})

	p.KluctlMust(t, "deploy", "--yes", "--project-dir", filepath.Join(p.LocalProjectDir(), projectDir))
	assertConfigMapExists(t, k, p.TestSlug(), "cm-inc1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm-inc2")
}

func TestIncludeLocalFromRoot(t *testing.T) {
	testLocalIncludes(t, ".")
}

func TestIncludeLocalFromSubdir(t *testing.T) {
	testLocalIncludes(t, "foo")
}
