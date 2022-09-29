package e2e

import (
	"encoding/base64"
	"fmt"
	"testing"

	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

const hookScript = `
kubectl get configmap -oyaml > /tmp/result.yml
cat /tmp/result.yml
if ! kubectl get secret {{ .name }}-result; then
    name="{{ .name }}-result"
else
    name="{{ .name }}-result2"
fi
kubectl create secret generic $name --from-file=result=/tmp/result.yml
kubectl delete configmap cm2 || true
`

func addHookDeployment(p *testProject, dir string, opts resourceOpts, isHelm bool, hook string, hookDeletionPolicy string) {
	annotations := make(map[string]string)
	if isHelm {
		annotations["helm.sh/hook"] = hook
		if hookDeletionPolicy != "" {
			annotations["helm.sh/hook-deletion-policy"] = hookDeletionPolicy
		}
	} else {
		annotations["kluctl.io/hook"] = hook
	}
	if hookDeletionPolicy != "" {
		annotations["kluctl.io/hook-deletion-policy"] = hookDeletionPolicy
	}

	script := renderTemplateHelper(hookScript, map[string]interface{}{
		"name":      opts.name,
		"namespace": opts.namespace,
	})

	opts.annotations = uo.CopyMergeStrMap(opts.annotations, annotations)

	addJobDeployment(p, dir, opts, "bitnami/kubectl:1.21", []string{"sh"}, []string{"-c", script})
}

func addConfigMap(p *testProject, dir string, opts resourceOpts) {
	o := uo.New()
	o.SetK8sGVKs("", "v1", "ConfigMap")
	mergeMetadata(o, opts)
	o.SetNestedField(map[string]interface{}{}, "data")
	p.addKustomizeResources(dir, []kustomizeResource{
		{fmt.Sprintf("%s.yml", opts.name), "", o},
	})
}

func getHookResult(t *testing.T, p *testProject, k *test_utils.KindCluster, secretName string) *uo.UnstructuredObject {
	o := k.KubectlYamlMust(t, "-n", p.projectName, "get", "secret", secretName)
	s, ok, err := o.GetNestedString("data", "result")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("result not found")
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	r, err := uo.FromString(string(b))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func getHookResultCMNames(t *testing.T, p *testProject, k *test_utils.KindCluster, second bool) []string {
	secretName := "hook-result"
	if second {
		secretName = "hook-result2"
	}
	o := getHookResult(t, p, k, secretName)
	items, _, _ := o.GetNestedObjectList("items")
	var names []string
	for _, x := range items {
		names = append(names, x.GetK8sName())
	}
	return names
}

func assertHookResultCMName(t *testing.T, p *testProject, k *test_utils.KindCluster, second bool, cmName string) {
	names := getHookResultCMNames(t, p, k, second)
	for _, x := range names {
		if x == cmName {
			return
		}
	}
	t.Fatalf("%s not found in hook result", cmName)
}

func assertHookResultNotCMName(t *testing.T, p *testProject, k *test_utils.KindCluster, second bool, cmName string) {
	names := getHookResultCMNames(t, p, k, second)
	for _, x := range names {
		if x == cmName {
			t.Fatalf("%s found in hook result", cmName)
		}
	}
}

func prepareHookTestProject(t *testing.T, name string, hook string, hookDeletionPolicy string) (*testProject, *test_utils.KindCluster) {
	isDone := false
	namespace := fmt.Sprintf("hook-%s", name)
	k := defaultKindCluster1
	p := &testProject{}
	p.init(t, k, namespace)
	defer func() {
		if !isDone {
			p.cleanup()
		}
	}()

	recreateNamespace(t, k, namespace)

	p.updateTarget("test", nil)

	addHookDeployment(p, "hook", resourceOpts{name: "hook", namespace: namespace}, false, hook, hookDeletionPolicy)
	addConfigMap(p, "hook", resourceOpts{name: "cm1", namespace: namespace})

	isDone = true
	return p, k
}

func ensureHookExecuted(t *testing.T, p *testProject, k *test_utils.KindCluster) {
	_, _ = k.Kubectl("delete", "-n", p.projectName, "secret", "hook-result", "hook-result2")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertResourceExists(t, k, p.projectName, "ConfigMap/cm1")
}

func ensureHookNotExecuted(t *testing.T, p *testProject, k *test_utils.KindCluster) {
	_, _ = k.Kubectl("delete", "-n", p.projectName, "secret", "hook-result", "hook-result2")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertResourceNotExists(t, k, p.projectName, "Secret/hook-result")
}

func TestHooksPreDeployInitial(t *testing.T) {
	t.Parallel()
	p, k := prepareHookTestProject(t, "pre-deploy-initial", "pre-deploy-initial", "")
	defer p.cleanup()
	ensureHookExecuted(t, p, k)
	assertHookResultNotCMName(t, p, k, false, "cm1")
	ensureHookNotExecuted(t, p, k)
}

func TestHooksPostDeployInitial(t *testing.T) {
	t.Parallel()
	p, k := prepareHookTestProject(t, "post-deploy-initial", "post-deploy-initial", "")
	defer p.cleanup()
	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
	ensureHookNotExecuted(t, p, k)
}

func TestHooksPreDeployUpgrade(t *testing.T) {
	t.Parallel()
	p, k := prepareHookTestProject(t, "pre-deploy-upgrade", "pre-deploy-upgrade", "")
	defer p.cleanup()
	addConfigMap(p, "hook", resourceOpts{name: "cm2", namespace: p.projectName})
	ensureHookNotExecuted(t, p, k)
	k.KubectlMust(t, "delete", "-n", p.projectName, "configmap", "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultNotCMName(t, p, k, false, "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
}

func TestHooksPostDeployUpgrade(t *testing.T) {
	t.Parallel()
	p, k := prepareHookTestProject(t, "post-deploy-upgrade", "post-deploy-upgrade", "")
	defer p.cleanup()
	addConfigMap(p, "hook", resourceOpts{name: "cm2", namespace: p.projectName})
	ensureHookNotExecuted(t, p, k)
	k.KubectlMust(t, "delete", "-n", p.projectName, "configmap", "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
}

func doTestHooksPreDeploy(t *testing.T, name string, hooks string) {
	p, k := prepareHookTestProject(t, name, hooks, "")
	defer p.cleanup()
	addConfigMap(p, "hook", resourceOpts{name: "cm2", namespace: p.projectName})
	ensureHookExecuted(t, p, k)
	k.KubectlMust(t, "delete", "-n", p.projectName, "configmap", "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultNotCMName(t, p, k, false, "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
}

func doTestHooksPostDeploy(t *testing.T, name string, hooks string) {
	p, k := prepareHookTestProject(t, name, hooks, "")
	defer p.cleanup()
	addConfigMap(p, "hook", resourceOpts{name: "cm2", namespace: p.projectName})
	ensureHookExecuted(t, p, k)
	k.KubectlMust(t, "delete", "-n", p.projectName, "configmap", "cm1")
	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
}

func doTestHooksPrePostDeploy(t *testing.T, name string, hooks string) {
	p, k := prepareHookTestProject(t, name, hooks, "")
	defer p.cleanup()
	addConfigMap(p, "hook", resourceOpts{name: "cm2", namespace: p.projectName})
	ensureHookExecuted(t, p, k)
	assertHookResultNotCMName(t, p, k, false, "cm1")
	assertHookResultNotCMName(t, p, k, false, "cm2")
	assertHookResultCMName(t, p, k, true, "cm1")
	assertHookResultCMName(t, p, k, true, "cm2")

	ensureHookExecuted(t, p, k)
	assertHookResultCMName(t, p, k, false, "cm1")
	assertHookResultNotCMName(t, p, k, false, "cm2")
	assertHookResultCMName(t, p, k, true, "cm1")
	assertHookResultCMName(t, p, k, true, "cm2")
}

func TestHooksPreDeploy(t *testing.T) {
	t.Parallel()
	doTestHooksPreDeploy(t, "pre-deploy", "pre-deploy")
}

func TestHooksPreDeploy2(t *testing.T) {
	t.Parallel()
	// same as pre-deploy
	doTestHooksPreDeploy(t, "pre-deploy2", "pre-deploy-initial,pre-deploy-upgrade")
}

func TestHooksPostDeploy(t *testing.T) {
	t.Parallel()
	doTestHooksPostDeploy(t, "post-deploy", "post-deploy")
}

func TestHooksPostDeploy2(t *testing.T) {
	t.Parallel()
	// same as post-deploy
	doTestHooksPostDeploy(t, "post-deploy2", "post-deploy-initial,post-deploy-upgrade")
}

func TestHooksPrePostDeploy(t *testing.T) {
	t.Parallel()
	doTestHooksPrePostDeploy(t, "pre-post-deploy", "pre-deploy,post-deploy")
}

func TestHooksPrePostDeploy2(t *testing.T) {
	t.Parallel()
	doTestHooksPrePostDeploy(t, "pre-post-deploy2", "pre-deploy-initial,pre-deploy-upgrade,post-deploy-initial,post-deploy-upgrade")
}
