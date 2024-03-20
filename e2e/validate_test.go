package e2e

import (
	"context"
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
	"time"
)

func buildDeployment(name string, namespace string, ready bool, annotations map[string]string) *uo.UnstructuredObject {
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
	if annotations != nil {
		deployment.SetK8sAnnotations(annotations)
	}
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

func prepareValidateTest(t *testing.T, k *test_utils.EnvTestCluster, annotations map[string]string) *test_project.TestProject {
	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	p.AddKustomizeDeployment("d1", []test_project.KustomizeResource{
		{Name: "d1.yml", Content: buildDeployment("d1", p.TestSlug(), false, annotations)},
	}, nil)

	return p
}

func assertValidate(t *testing.T, p *test_project.TestProject, succeed bool) (string, string) {
	args := []string{"validate"}
	args = append(args, "-t", "test")

	stdout, stderr, err := p.Kluctl(t, args...)

	if succeed {
		assert.NoError(t, err)
		assert.NotContains(t, stdout, fmt.Sprintf("%s/Deployment/d1: readyReplicas field not in status or empty", p.TestSlug()))
		assert.NotContains(t, stderr, "Validation failed")
	} else {
		assert.ErrorContains(t, err, "Validation failed")
		assert.Contains(t, stdout, fmt.Sprintf("%s/Deployment/d1: readyReplicas field not in status or empty", p.TestSlug()))
	}

	return stdout, stderr
}

func TestValidate(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k, nil)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertObjectExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, false)

	readyDeployment := buildDeployment("d1", p.TestSlug(), true, nil)

	_, err := k.DynamicClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(p.TestSlug()).
		Patch(context.Background(), "d1", types.ApplyPatchType, []byte(yaml.WriteJsonStringMust(readyDeployment)), metav1.PatchOptions{
			FieldManager: "test",
		}, "status")
	assert.NoError(t, err)

	assertValidate(t, p, true)
}

func TestValidateSkipHooks(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k, map[string]string{
		"kluctl.io/hook":      "post-deploy",
		"kluctl.io/hook-wait": "false",
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertObjectExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, false)

	p.UpdateYaml("d1/d1.yml", func(o *uo.UnstructuredObject) error {
		o.SetK8sAnnotation("kluctl.io/hook-delete-policy", "hook-succeeded")
		return nil
	}, "")

	assertValidate(t, p, true)
}

func TestValidateSkipDeleteHooks(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k, map[string]string{
		"helm.sh/hook":        "post-delete",
		"kluctl.io/hook-wait": "false",
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertObjectNotExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, true)
}

func TestValidateSkipRollbackHooks(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := prepareValidateTest(t, k, map[string]string{
		"helm.sh/hook":        "post-rollback",
		"kluctl.io/hook-wait": "false",
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertObjectNotExists(t, k, appsv1.SchemeGroupVersion.WithResource("deployments"), p.TestSlug(), "d1")

	assertValidate(t, p, true)
}

func TestValidateWithoutPermissions(t *testing.T) {
	t.Parallel()

	// need our own cluster due to CRDs being involved
	k := createTestCluster(t, "cluster1")
	test_resources.ApplyYaml(t, "example-crds.yaml", k)

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	username := p.TestSlug()
	au, err := k.AddUser(envtest.User{Name: username}, nil)
	assert.NoError(t, err)

	rbac := buildSingleNamespaceRbac(username, p.TestSlug(), nil, []schema.GroupResource{
		{Group: "", Resource: "configmaps"},
		{Group: "stable.example.com", Resource: "crontabs"},
		{Group: "stable.example.com", Resource: "crontabstatuses"},
	})
	for _, x := range rbac {
		k.MustApply(t, x)
	}

	kc, err := au.KubeConfig()
	assert.NoError(t, err)

	p.AddExtraArgs("--kubeconfig", getKubeconfigTmpFile(t, kc))

	cronTab := uo.New()
	cronTab.SetK8sGVK(schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTab"})
	cronTab.SetK8sNamespace(p.TestSlug())
	cronTab.SetK8sName("crontab")
	_ = cronTab.SetNestedField(map[string]any{
		"cronSpec": "x",
	}, "spec")
	cronTab.SetK8sAnnotation("kluctl.io/wait-readiness", "true")
	cronTab2 := cronTab.Clone()
	cronTab2.SetK8sGVK(schema.GroupVersionKind{Group: "stable.example.com", Version: "v1", Kind: "CronTabStatus"})

	p.AddKustomizeDeployment("x", []test_project.KustomizeResource{
		{Name: "crontab.yaml", Content: cronTab},
		{Name: "crontab2.yaml", Content: cronTab2},
	}, nil)
	// a configmap should always be considered ready thanks to the scheme based check
	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
		annotations: map[string]string{
			"kluctl.io/wait-readiness": "true",
		},
	})

	// this should succeed but emit a warning about the wait not knowing if a status is expected
	stdout, _, err := p.Kluctl(t, "deploy", "--yes")
	assert.NoError(t, err)
	assert.Contains(t, stdout, "CronTab/crontab: unable to determine if a status is expected")
	assert.Contains(t, stdout, "CronTabStatus/crontab: unable to determine if a status is expected")

	testSuccess := func(globalRbacResources []schema.GroupResource, rbacResources []schema.GroupResource) {
		rbac = buildSingleNamespaceRbac(username, p.TestSlug(), globalRbacResources, rbacResources)
		for _, x := range rbac {
			k.MustApply(t, x)
		}

		// need to reset status
		err = k.Client.Delete(context.TODO(), cronTab2.ToUnstructured())
		assert.NoError(t, err)

		// this should succeed without any warning
		go func() {
			// set the status sub-resource after a few seconds so that the wait finishes
			time.Sleep(3 * time.Second)
			x := cronTab2.Clone()
			_ = x.SetNestedField("test", "status", "x")
			k.MustApplyStatus(t, x)
		}()
		stdout, _, err = p.Kluctl(t, "deploy", "--yes", "--timeout=10s")
		assert.NoError(t, err)
		assert.NotContains(t, stdout, "CronTab/crontab: unable to determine if a status is expected")
		assert.NotContains(t, stdout, "CronTabStatus/crontab: unable to determine if a status is expected")
	}

	// status requirement detection via sub-resource GET
	testSuccess(nil, []schema.GroupResource{
		{Group: "", Resource: "configmaps"},
		{Group: "stable.example.com", Resource: "crontabs"},
		{Group: "stable.example.com", Resource: "crontabs/status"},
		{Group: "stable.example.com", Resource: "crontabstatuses"},
		{Group: "stable.example.com", Resource: "crontabstatuses/status"},
	})
	// status requirement detection via CRDs
	testSuccess([]schema.GroupResource{
		{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
	}, []schema.GroupResource{
		{Group: "", Resource: "configmaps"},
		{Group: "stable.example.com", Resource: "crontabs"},
		{Group: "stable.example.com", Resource: "crontabstatuses"},
	})
	// both
	testSuccess([]schema.GroupResource{
		{Group: "apiextensions.k8s.io", Resource: "customresourcedefinitions"},
	}, []schema.GroupResource{
		{Group: "", Resource: "configmaps"},
		{Group: "stable.example.com", Resource: "crontabs"},
		{Group: "stable.example.com", Resource: "crontabs/status"},
		{Group: "stable.example.com", Resource: "crontabstatuses"},
		{Group: "stable.example.com", Resource: "crontabstatuses/status"},
	})
}
