package e2e

import (
	"fmt"
	"testing"

	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/stretchr/testify/assert"
)

func TestFluxCommands(t *testing.T) {
	t.Parallel()

	k := defaultKindCluster1

	p := &testProject{}
	p.init(t, k, "simple")

	defer p.cleanup()

	test_resources.ApplyYaml("flux-source-crd.yaml", k)
	test_resources.ApplyYaml("kluctl-crds.yaml", k)
	test_resources.ApplyYaml("kluctl-deployment.yaml", k)

	assertResourceExists(t, k, "default", "kluctldeployment/microservices-demo-test")

	p.KluctlMust("flux", "suspend", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test")
	suspend := getKluctlSuspendField(t, k)
	assert.Equal(t, true, suspend, "Field status.suspend is not false")

	p.KluctlMust("flux", "resume", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test", "--no-wait")
	resume := getKluctlSuspendField(t, k)
	assert.Equal(t, false, resume, "Field status.suspend is not true")

	p.KluctlMust("flux", "reconcile", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test", "--with-source", "--no-wait")
	annotation := getKluctlAnnotations(t, k)
	assert.Len(t, annotation, 1, "Annotation not present")
}

func getKluctlSuspendField(t *testing.T, k *test_utils.EnvTestCluster) interface{} {
	o := k.KubectlYamlMust(t, "-n", "default", "get", "kluctldeployment", "microservices-demo-test")
	result, ok, err := o.GetNestedField("spec", "suspend")
	fmt.Println(result)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("result not found")
	}
	return result
}

func getKluctlAnnotations(t *testing.T, k *test_utils.EnvTestCluster) interface{} {
	o := k.KubectlYamlMust(t, "-n", "default", "get", "kluctldeployment", "microservices-demo-test")
	result, ok, err := o.GetNestedField("metadata", "annotations")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("result not found")
	}
	fmt.Println(result)
	return result
}
