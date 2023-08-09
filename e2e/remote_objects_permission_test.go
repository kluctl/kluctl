package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

func TestRemoteObjectUtils_PermissionErrors(t *testing.T) {
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

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm")
	assertSecretExists(t, k, p.TestSlug(), "secret")

	kc, err := au.KubeConfig()
	assert.NoError(t, err)

	setKubeconfigString(t, kc)

	stdout, _, err := p.Kluctl("deploy", "--yes", "-t", "test", "--write-command-result=false")
	assert.Error(t, err)
	assert.Contains(t, stdout, "at least one permission error was encountered while gathering objects by discriminator labels")
}
