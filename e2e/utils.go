package e2e

import (
	"context"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"testing"
)

func createNamespace(t *testing.T, k *test_utils.EnvTestCluster, namespace string) {
	var ns unstructured.Unstructured
	ns.SetName(namespace)

	_, err := k.DynamicClient.Resource(v1.SchemeGroupVersion.WithResource("namespaces")).Create(context.Background(), &ns, metav1.CreateOptions{})

	if err != nil {
		t.Fatal(err)
	}
}

func assertConfigMapExists(t *testing.T, k *test_utils.EnvTestCluster, namespace string, name string) *uo.UnstructuredObject {
	x, err := k.Get(v1.SchemeGroupVersion.WithResource("configmaps"), namespace, name)
	if err != nil {
		t.Fatalf("unexpected error '%v' while getting ConfigMap %s/%s", err, namespace, name)
	}
	return x
}

func assertConfigMapNotExists(t *testing.T, k *test_utils.EnvTestCluster, namespace string, name string) {
	_, err := k.Get(v1.SchemeGroupVersion.WithResource("configmaps"), namespace, name)
	if err == nil {
		t.Fatalf("expected %s/%s to not exist", namespace, name)
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("unexpected error '%v' for %s/%s, expected a NotFound error", err, namespace, name)
	}
}

func assertNestedFieldEquals(t *testing.T, o *uo.UnstructuredObject, expected interface{}, keys ...interface{}) {
	v, ok, err := o.GetNestedField(keys...)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("field %s not found in object", uo.KeyPath(keys).ToJsonPath())
	}
	if !reflect.DeepEqual(v, expected) {
		t.Fatalf("%v != %v", v, expected)
	}
}

func updateObject(t *testing.T, k *test_utils.EnvTestCluster, o *uo.UnstructuredObject) {
	_, err := k.DynamicClient.Resource(schema.GroupVersionResource{
		Version:  "v1",
		Resource: "configmaps",
	}).Namespace(o.GetK8sNamespace()).Update(context.Background(), o.ToUnstructured(), metav1.UpdateOptions{})
	assert.NoError(t, err)
}
