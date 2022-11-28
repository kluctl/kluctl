package e2e

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"
	"testing"

	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"github.com/stretchr/testify/assert"
)

var kluctlDeploymentGVR = schema.GroupVersionResource{
	Group:    "flux.kluctl.io",
	Version:  "v1alpha1",
	Resource: "kluctldeployments",
}

func getKluctlDeploymentObject(t *testing.T, k *test_utils.EnvTestCluster) *uo.UnstructuredObject {
	kd, err := k.DynamicClient.Resource(kluctlDeploymentGVR).Namespace("flux-test").Get(context.Background(), "microservices-demo-test", v1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if kd == nil {
		t.Fatal("kluctldeployment not found")
	}
	return uo.FromUnstructured(kd)
}

func TestFluxCommands(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := NewTestProject(t, k)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		test_resources.ApplyYaml("flux-source-crd.yaml", k)
		wg.Done()
	}()
	go func() {
		test_resources.ApplyYaml("kluctl-crds.yaml", k)
		wg.Done()
	}()
	wg.Wait()
	test_resources.ApplyYaml("kluctl-deployment.yaml", k)

	// assert that it was created
	_ = getKluctlDeploymentObject(t, k)

	p.KluctlMust("flux", "suspend", "--namespace", "flux-test", "--kluctl-deployment", "microservices-demo-test")
	suspend := getKluctlSuspendField(t, k)
	assert.Equal(t, true, suspend, "Field status.suspend is not false")

	p.KluctlMust("flux", "resume", "--namespace", "flux-test", "--kluctl-deployment", "microservices-demo-test", "--no-wait")
	resume := getKluctlSuspendField(t, k)
	assert.Equal(t, false, resume, "Field status.suspend is not true")

	p.KluctlMust("flux", "reconcile", "--namespace", "flux-test", "--kluctl-deployment", "microservices-demo-test", "--with-source", "--no-wait")
	annotation := getKluctlAnnotations(t, k)
	assert.Len(t, annotation, 1, "Annotation not present")
}

func getKluctlSuspendField(t *testing.T, k *test_utils.EnvTestCluster) interface{} {
	o := getKluctlDeploymentObject(t, k)
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
	o := getKluctlDeploymentObject(t, k)
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
