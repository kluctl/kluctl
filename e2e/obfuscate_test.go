package e2e

import (
	"encoding/base64"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestObfuscateSecrets(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addSecretDeployment(p, "secret", map[string]string{
		"secret": "secret_value_1",
	}, resourceOpts{
		name:      "secret",
		namespace: p.TestSlug(),
	}, false)
	stdout, _ := p.KluctlMust("deploy", "--yes", "-t", "test")
	assertSecretExists(t, k, p.TestSlug(), "secret")
	assert.NotContains(t, stdout, "secret_value")

	p.UpdateYaml("secret/secret-secret.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("secret_value_2", "stringData", "secret")
		return nil
	}, "")
	stdout, _ = p.KluctlMust("deploy", "--yes", "-t", "test")
	assert.NotContains(t, stdout, "secret_value")
	assert.Contains(t, stdout, "***** (obfuscated)")

	p.UpdateYaml("secret/secret-secret.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("secret_value_3", "stringData", "secret")
		return nil
	}, "")
	stdout, _ = p.KluctlMust("deploy", "--yes", "-t", "test", "--no-obfuscate")
	assert.Contains(t, stdout, "-"+base64.StdEncoding.EncodeToString([]byte("secret_value_2")))
	assert.Contains(t, stdout, "+"+base64.StdEncoding.EncodeToString([]byte("secret_value_3")))
	assert.NotContains(t, stdout, "***** (obfuscated)")
}
