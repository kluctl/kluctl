package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func testWaitReadiness(t *testing.T, fn func(p *test_project.TestProject)) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"kluctl.io/is-ready": "false",
		},
	})
	p.AddDeploymentItem(".", uo.FromMap(map[string]interface{}{
		"barrier": true,
	}))
	addConfigMapDeployment(p, "cm3", nil, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
	})

	fn(p)

	_, stderr, err := p.Kluctl(t, "deploy", "--yes", "-t", "test", "--timeout", (3 * time.Second).String())
	assert.Error(t, err)
	assert.Contains(t, stderr, fmt.Sprintf("context cancelled while waiting for readiness of %s/ConfigMap/cm2", p.TestSlug()))

	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm3")

	// we run a goroutine in the background that will wait for a few seconds and then annotate kluctl.io/is-ready=true,
	// which will then cause the hook to get ready
	go func() {
		time.Sleep(3 * time.Second)
		patchConfigMap(t, k, p.TestSlug(), "cm2", func(o *uo.UnstructuredObject) {
			o.SetK8sAnnotation("kluctl.io/is-ready", "true")
		})
	}()

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
}

func TestWaitReadinessViaDeployment(t *testing.T) {
	testWaitReadiness(t, func(p *test_project.TestProject) {
		p.UpdateDeploymentItems(".", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
			items[1].SetNestedField(true, "waitReadiness")
			return items
		})
	})
}

func TestWaitReadinessViaDeployment2(t *testing.T) {
	testWaitReadiness(t, func(p *test_project.TestProject) {
		p.UpdateDeploymentItems(".", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
			items[2].SetNestedField([]map[string]any{
				{
					"kind":      "ConfigMap",
					"namespace": p.TestSlug(),
					"name":      "cm2",
				},
			}, "waitReadinessObjects")
			return items
		})
	})
}

func TestWaitReadinessViaAnnotation(t *testing.T) {
	testWaitReadiness(t, func(p *test_project.TestProject) {
		p.UpdateYaml("cm2/configmap-cm2.yml", func(o *uo.UnstructuredObject) error {
			o.SetK8sAnnotation("kluctl.io/wait-readiness", "true")
			return nil
		}, "")
	})
}

func TestWaitReadinessViaKustomization(t *testing.T) {
	testWaitReadiness(t, func(p *test_project.TestProject) {
		p.UpdateYaml("cm2/kustomization.yml", func(o *uo.UnstructuredObject) error {
			o.SetK8sAnnotation("kluctl.io/wait-readiness", "true")
			return nil
		}, "")
	})
}

func TestWaitReadinessViaKustomizationWithHook(t *testing.T) {
	testWaitReadiness(t, func(p *test_project.TestProject) {
		// the kustomization.yaml wait-readiness should actually be ignored in this case...
		p.UpdateYaml("cm2/kustomization.yml", func(o *uo.UnstructuredObject) error {
			o.SetK8sAnnotation("kluctl.io/wait-readiness", "true")
			return nil
		}, "")
		// because the hook object is what is waited for instead
		p.UpdateYaml("cm2/configmap-cm2.yml", func(o *uo.UnstructuredObject) error {
			o.SetK8sAnnotation("kluctl.io/hook", "post-deploy")
			return nil
		}, "")
	})
}
