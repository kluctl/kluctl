package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func TestTargetKubeconfig(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t)
	createNamespace(t, defaultCluster1, p.TestSlug())

	p.UpdateFile("kubeconfig.yaml", func(f string) (string, error) {
		return string(defaultCluster1.Kubeconfig), nil
	}, "")

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField("kubeconfig.yaml", "kubeconfig")
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, defaultCluster1, p.TestSlug(), "cm")
}

