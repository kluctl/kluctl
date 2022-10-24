package e2e

import (
	"bufio"
	"bytes"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func createNamespace(t *testing.T, k *test_utils.EnvTestCluster, namespace string) {
	k.KubectlMust(t, "create", "ns", namespace)
	k.KubectlMust(t, "label", "ns", namespace, "kluctl-e2e=true")
}

func assertResourceExists(t *testing.T, k *test_utils.EnvTestCluster, namespace string, resource string) *uo.UnstructuredObject {
	var args []string
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "get", resource)
	return k.KubectlYamlMust(t, args...)
}

func assertResourceNotExists(t *testing.T, k *test_utils.EnvTestCluster, namespace string, resource string) {
	var args []string
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "get", resource)
	_, stderr, err := k.KubectlYaml(args...)
	if err == nil {
		t.Fatalf("'kubectl get' for %s should not have succeeded", resource)
	} else {
		if strings.Index(stderr, "(NotFound)") == -1 {
			t.Fatal(err)
		}
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
