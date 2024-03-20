package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"strings"
	"testing"
)

func buildSingleNamespaceRbac(username string, namespace string) string {
	rbac := strings.NewReplacer(
		"USERNAME", username,
		"NAMESPACE", namespace,
	).Replace(`
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: USERNAME
rules:
  - apiGroups: [""]
    resources: ["namespaces"]
    resourceNames: ["NAMESPACE"]
    verbs: ["create", "update", "patch", "get", "list", "watch"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["*"]
    verbs: ["create", "update",  "patch", "get", "list", "watch"]
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: USERNAME
  namespace: NAMESPACE
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["create", "update", "patch", "get", "list", "watch"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: USERNAME
subjects:
  - kind: User
    name: USERNAME
roleRef:
  kind: ClusterRole
  name: USERNAME
  apiGroup: rbac.authorization.k8s.io
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: USERNAME
  namespace: NAMESPACE
subjects:
  - kind: User
    name: USERNAME
roleRef:
  kind: Role
  name: USERNAME
  apiGroup: rbac.authorization.k8s.io
`)

	return rbac
}

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

func TestOnlyOneNamespacePermissions(t *testing.T) {
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

	rbac := buildSingleNamespaceRbac(username, p.TestSlug())
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

	_, stderr, err := p.Kluctl(t, "deploy", "--yes", "-t", "test")
	assert.NoError(t, err)
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assert.Contains(t, stderr, "Not enough permissions to write to the result store.")
}
