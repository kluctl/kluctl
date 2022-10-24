package e2e

import (
	"bufio"
	"bytes"
	"context"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os/exec"
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

func runHelper(t *testing.T, cmd *exec.Cmd) (string, string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		_ = stdoutPipe.Close()
		return "", "", err
	}

	stdReader := func(testLogPrefix string, buf io.StringWriter, pipe io.Reader) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			l := scanner.Text()
			t.Log(testLogPrefix + l)
			_, _ = buf.WriteString(l + "\n")
		}
	}

	stdoutBuf := bytes.NewBuffer(nil)
	stderrBuf := bytes.NewBuffer(nil)

	go stdReader("stdout: ", stdoutBuf, stdoutPipe)
	go stdReader("stderr: ", stderrBuf, stderrPipe)

	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
