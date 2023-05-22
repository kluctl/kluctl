package e2e

import (
	"context"
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"path/filepath"
	"testing"
)

func buildDeployment(name string, namespace string, ready bool) *uo.UnstructuredObject {
	deployment := uo.FromStringMust(fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`, name, namespace))
	if ready {
		deployment.Merge(uo.FromStringMust(`
status:
  availableReplicas: 1
  conditions:
  - lastTransitionTime: "2023-03-29T19:23:12Z"
    lastUpdateTime: "2023-03-29T19:23:12Z"
    message: Deployment has minimum availability.
    reason: MinimumReplicasAvailable
    status: "True"
    type: Available
  - lastTransitionTime: "2023-03-29T19:22:30Z"
    lastUpdateTime: "2023-03-29T19:23:12Z"
    message: ReplicaSet "argocd-redis-8f7689686" has successfully progressed.
    reason: NewReplicaSetAvailable
    status: "True"
    type: Progressing
  observedGeneration: 1
  readyReplicas: 1
  replicas: 1
`))
	}
	return deployment
}

func prepareValidateTest(t *testing.T, k *test_utils.EnvTestCluster) *test_utils.TestProject {
	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: fmt.Sprintf("configmap-%s.yml", "d1"), Content: buildDeployment("d1", p.TestSlug(), false)},
	}, nil)

	return p
}

func assertValidate(t *testing.T, p *test_utils.TestProject, commandResult *string, succeed bool) (string, string) {
	args := []string{"validate"}
	if commandResult != nil {
		args = append(args, "--command-result", *commandResult)
	} else {
		args = append(args, "-t", "test")
	}
	stdout, stderr, err := p.Kluctl(args...)

	if succeed {
		assert.NoError(t, err)
		assert.NotContains(t, stdout, fmt.Sprintf("%s/Deployment/d1: readyReplicas field not in status or empty", p.TestSlug()))
		assert.NotContains(t, stderr, "Validation failed")
	} else {
		assert.Error(t, err)
		assert.Contains(t, stdout, fmt.Sprintf("%s/Deployment/d1: readyReplicas field not in status or empty", p.TestSlug()))
		assert.Contains(t, stderr, "Validation failed")
	}

	return stdout, stderr
}

func TestValidate(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertObjectExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, nil, false)

	readyDeployment := buildDeployment("d1", p.TestSlug(), true)

	_, err := k.DynamicClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(p.TestSlug()).
		Patch(context.Background(), "d1", types.ApplyPatchType, []byte(yaml.WriteJsonStringMust(readyDeployment)), metav1.PatchOptions{
			FieldManager: "test",
		}, "status")
	assert.NoError(t, err)

	assertValidate(t, p, nil, true)
}

func TestValidateCommandResult(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k)

	resultPath := filepath.Join(t.TempDir(), "result.yaml")

	p.KluctlMust("deploy", "--yes", "-t", "test", "-oyaml="+resultPath)
	assertObjectExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, &resultPath, false)

	readyDeployment := buildDeployment("d1", p.TestSlug(), true)

	_, err := k.DynamicClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(p.TestSlug()).
		Patch(context.Background(), "d1", types.ApplyPatchType, []byte(yaml.WriteJsonStringMust(readyDeployment)), metav1.PatchOptions{
			FieldManager: "test",
		}, "status")
	assert.NoError(t, err)

	assertValidate(t, p, &resultPath, true)
}

func TestValidateDrift(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k)

	resultPath := filepath.Join(t.TempDir(), "result.yaml")

	p.KluctlMust("deploy", "--yes", "-t", "test", "-oyaml="+resultPath)
	assertObjectExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	stdout, _ := assertValidate(t, p, &resultPath, false)
	assert.NotContains(t, stdout, "Drift:")

	changedDeployment := buildDeployment("d1", p.TestSlug(), false)
	changedDeployment.Merge(uo.FromStringMust(`
spec:
  replicas: 2
`))

	b := true
	_, err := k.DynamicClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(p.TestSlug()).
		Patch(context.Background(), "d1", types.ApplyPatchType, []byte(yaml.WriteJsonStringMust(changedDeployment)), metav1.PatchOptions{
			FieldManager: "test",
			Force:        &b,
		})
	assert.NoError(t, err)

	stdout, _ = assertValidate(t, p, &resultPath, false)
	assert.Contains(t, stdout, "Drift:")
	assert.Contains(t, stdout, "spec.replicas")
}
