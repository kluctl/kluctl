package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/validation"
	log "github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func recreateNamespace(t *testing.T, k *KindCluster, namespace string) {
	_, _ = k.Kubectl("delete", "ns", namespace)
	k.KubectlMust(t, "create", "ns", namespace)
	k.KubectlMust(t, "label", "ns", namespace, "kluctl-e2e=true")
}

func deleteTestNamespaces(k *KindCluster) {
	_, _ = k.Kubectl("delete", "ns", "-l", "kubectl-e2e=true")
}

func waitForReadiness(t *testing.T, k *KindCluster, namespace string, resource string, timeout time.Duration) bool {
	t.Logf("Waiting for readiness: %s/%s", namespace, resource)

	startTime := time.Now()
	for time.Now().Sub(startTime) < timeout {
		y, err := k.KubectlYaml("-n", namespace, "get", resource)
		if err != nil {
			t.Fatal(err)
		}

		v := validation.ValidateObject(nil, y, true)
		if v.Ready {
			return true
		}

		if log.IsLevelEnabled(log.DebugLevel) {
			errTxt := ""
			for _, e := range v.Errors {
				if errTxt != "" {
					errTxt += "\n"
				}
				errTxt += fmt.Sprintf("%s: %s", e.Ref.String(), e.Error)
			}
			log.Debugf("validation failed for %s/%s. errors:\n%s", namespace, resource, errTxt)
		}
		time.Sleep(1 * time.Second)
	}
	return false
}

func assertReadiness(t *testing.T, k *KindCluster, namespace string, resource string, timeout time.Duration) {
	if !waitForReadiness(t, k, namespace, resource, timeout) {
		t.Errorf("%s/%s did not get ready in time", namespace, resource)
	}
}

func assertResourceExists(t *testing.T, k *KindCluster, namespace string, resource string) *uo.UnstructuredObject {
	var args []string
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "get", resource)
	return k.KubectlYamlMust(t, args...)
}

func assertResourceNotExists(t *testing.T, k *KindCluster, namespace string, resource string) {
	var args []string
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "get", resource)
	_, err := k.KubectlYaml(args...)
	if err == nil {
		t.Fatalf("'kubectl get' for %s should not have succeeded", resource)
	} else {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		if strings.Index(string(ee.Stderr), "(NotFound)") == -1 {
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
		t.Fatalf("field %s not found in object", uo.KeyListToJsonPath(keys))
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

func init() {
	deleteTestNamespaces(defaultKindCluster)
}
