package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func TestRemoteObjectUtils_PermissionErrors(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	username := p.TestSlug()
	au, err := k.AddUser(envtest.User{Name: username}, nil)
	assert.NoError(t, err)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addSecretDeployment(p, "secret", nil, resourceOpts{
		name:      "secret",
		namespace: p.TestSlug(),
	}, false)
	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})

	rbac := fmt.Sprintf(`
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
subjects:
  - kind: User
    name: %s
roleRef:
  kind: ClusterRole
  name: "system:aggregate-to-view"
  apiGroup: rbac.authorization.k8s.io
`, username, username)
	p.AddKustomizeDeployment("rbac", []test_project.KustomizeResource{
		{Name: "rbac.yaml", Content: rbac},
	}, nil)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm")
	assertSecretExists(t, k, p.TestSlug(), "secret")

	kc, err := au.KubeConfig()
	assert.NoError(t, err)

	p.AddExtraArgs("--kubeconfig", getKubeconfigTmpFile(t, kc))

	stdout, _, err := p.Kluctl(t, "deploy", "--yes", "-t", "test", "--write-command-result=false")
	assert.Error(t, err)
	assert.Contains(t, stdout, "at least one permission error was encountered while gathering objects by discriminator labels")
}

func TestNoGetKubeSystemPermissions(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)

	username := p.TestSlug()
	au, err := k.AddUser(envtest.User{Name: username}, nil)
	assert.NoError(t, err)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})

	rbac := fmt.Sprintf(`
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
rules:
  - apiGroups: [""]
    resources: ["namespaces"]
    # only list allowed, get is forbidden. it should still be able to get the cluster ID
    verbs: ["list"]
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create", "update", "patch", "get", "list", "watch"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["*"]
    verbs: ["create", "update",  "patch", "get", "list", "watch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
subjects:
  - kind: User
    name: %s
roleRef:
  kind: ClusterRole
  name: %s
  apiGroup: rbac.authorization.k8s.io
`, username, username, username, username)
	p.AddKustomizeDeployment("rbac", []test_project.KustomizeResource{
		{Name: "rbac.yaml", Content: rbac},
	}, nil)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")

	kc, err := au.KubeConfig()
	assert.NoError(t, err)

	p.AddExtraArgs("--kubeconfig", getKubeconfigTmpFile(t, kc))

	addConfigMapDeployment(p, "cm2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--write-command-result=false")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
}
