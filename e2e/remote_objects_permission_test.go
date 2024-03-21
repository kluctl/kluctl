package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func buildSingleNamespaceRbac(username string, namespace string, globalResources []schema.GroupResource, resources []schema.GroupResource) []*uo.UnstructuredObject {
	var ret []*uo.UnstructuredObject

	var clusterRole v1.ClusterRole
	clusterRole.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("ClusterRole"))
	clusterRole.Name = username
	clusterRole.Rules = append(clusterRole.Rules, v1.PolicyRule{
		APIGroups:     []string{""},
		Resources:     []string{"namespaces"},
		ResourceNames: []string{namespace},
		Verbs:         []string{"create", "update", "patch", "get", "list", "watch"},
	})
	for _, r := range globalResources {
		clusterRole.Rules = append(clusterRole.Rules, v1.PolicyRule{
			APIGroups: []string{r.Group},
			Resources: []string{r.Resource},
			Verbs:     []string{"create", "update", "patch", "get", "list", "watch"},
		})
	}
	ret = append(ret, uo.FromStructMust(clusterRole))

	var role v1.Role
	role.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Role"))
	role.Name = username
	role.Namespace = namespace
	for _, r := range resources {
		role.Rules = append(role.Rules, v1.PolicyRule{
			APIGroups: []string{r.Group},
			Resources: []string{r.Resource},
			Verbs:     []string{"create", "update", "patch", "get", "list", "watch"},
		})
	}
	ret = append(ret, uo.FromStructMust(role))

	var clusterRoleBinding v1.ClusterRoleBinding
	clusterRoleBinding.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))
	clusterRoleBinding.Name = username
	clusterRoleBinding.Subjects = append(clusterRoleBinding.Subjects, v1.Subject{
		Kind: "User",
		Name: username,
	})
	clusterRoleBinding.RoleRef = v1.RoleRef{
		Kind:     "ClusterRole",
		Name:     username,
		APIGroup: "rbac.authorization.k8s.io",
	}
	ret = append(ret, uo.FromStructMust(clusterRoleBinding))

	var roleBinding v1.ClusterRoleBinding
	roleBinding.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("RoleBinding"))
	roleBinding.Name = username
	roleBinding.Namespace = namespace
	roleBinding.Subjects = append(roleBinding.Subjects, v1.Subject{
		Kind: "User",
		Name: username,
	})
	roleBinding.RoleRef = v1.RoleRef{
		Kind:     "Role",
		Name:     username,
		APIGroup: "rbac.authorization.k8s.io",
	}
	ret = append(ret, uo.FromStructMust(roleBinding))

	return ret
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

	rbac := buildSingleNamespaceRbac(username, p.TestSlug(), nil, []schema.GroupResource{{Group: "", Resource: "configmaps"}})
	for _, x := range rbac {
		k.MustApply(t, x)
	}

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})

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
