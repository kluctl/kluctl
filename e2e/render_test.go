package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRenderPrintAll(t *testing.T) {
	t.Parallel()

	p := test_utils.NewTestProject(t)

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	stdout, _ := p.KluctlMust("render", "-t", "test", "--print-all")
	y, err := yaml.ReadYamlAllString(stdout)
	assert.NoError(t, err)
	assert.Equal(t, []any{map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{"kluctl.io/deployment-item-dir": "cm"},
			"labels": map[string]interface{}{
				"kluctl.io/discriminator": p.Discriminator("test"),
				"kluctl.io/tag-0":         "cm",
				"project_name":            p.TestSlug(),
			},
			"name":      "cm",
			"namespace": "test-render-print-all",
		}}}, y)
}

func TestRenderNoKubeconfig(t *testing.T) {
	t.Setenv("KUBECONFIG", "invalid")

	p := test_utils.NewTestProject(t)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	_, stderr := p.KluctlMust("render", "--print-all")
	assert.Contains(t, stderr, "No valid KUBECONFIG provided, which means the Kubernetes client is not available")

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField("context1", "context")
	})
	_, stderr, _ = p.Kluctl("render", "-t", "test", "--print-all")
	assert.Contains(t, stderr, "context \"context1\" does not exist")

}
