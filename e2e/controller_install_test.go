package e2e

import (
	"context"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func TestControllerInstall(t *testing.T) {
	k := createTestCluster(t, "cluster1")
	kubeconfig := getKubeconfigTmpFile(t, k.Kubeconfig)

	_, _, err := test_project.KluctlExecute(t, context.TODO(), func(args ...any) {
		t.Log(args...)
	}, "controller", "install", "--yes", "--kubeconfig", kubeconfig)
	assert.NoError(t, err)

	x := assertObjectExists(t, k, schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}, "kluctl-system", "kluctl-controller")
	assertNestedFieldEquals(t, x, "kluctl.io-controller", "metadata", "labels", "kluctl.io/discriminator")

	stdout, _, err := test_project.KluctlExecute(t, context.TODO(), func(args ...any) {
		t.Log(args...)
	}, "controller", "install", "--yes", "--kubeconfig", kubeconfig, "--kluctl-version", "v2.99999")
	assert.NoError(t, err)
	assert.Contains(t, stdout, "v2.99999")
}
