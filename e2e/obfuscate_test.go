package e2e

import (
	"encoding/base64"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
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
	addSecretDeployment(p, "secret2", nil, resourceOpts{
		name:      "secret2",
		namespace: p.TestSlug(),
	}, false)
	addSecretDeployment(p, "secret3", nil, resourceOpts{
		name:      "secret3",
		namespace: p.TestSlug(),
	}, false)

	stdout, _ := p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertSecretExists(t, k, p.TestSlug(), "secret")
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value")))

	p.UpdateYaml("secret/secret-secret.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("secret_value_2", "stringData", "secret")
		return nil
	}, "")
	stdout, _ = p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value")))
	assert.Contains(t, stdout, "***** (obfuscated)")

	p.UpdateYaml("secret/secret-secret.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("secret_value_3", "stringData", "secret")
		return nil
	}, "")
	stdout, _ = p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--no-obfuscate")
	assert.Contains(t, stdout, "-"+base64.StdEncoding.EncodeToString([]byte("secret_value_2")))
	assert.Contains(t, stdout, "+"+base64.StdEncoding.EncodeToString([]byte("secret_value_3")))
	assert.NotContains(t, stdout, "***** (obfuscated)")

	// also test changing from empty data to filled data
	p.UpdateYaml("secret2/secret-secret2.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(map[string]any{
			"secret2": "secret_value_2",
		}, "stringData")
		return nil
	}, "")

	stdout, _ = p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value_2")))
	assert.Contains(t, stdout, "+secret2: '***** (obfuscated)'")

	p.UpdateYaml("secret3/secret-secret3.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(map[string]any{
			"secret3": "secret_value_3",
			"secret4": "secret_value_4",
		}, "stringData")
		return nil
	}, "")
	stdout, _ = p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value_3")))
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value_4")))
	assert.Contains(t, stdout, "+secret3: '***** (obfuscated)'")
	assert.Contains(t, stdout, "+secret4: '***** (obfuscated)'")

	p.UpdateYaml("secret3/secret-secret3.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(map[string]any{
			"secret.dot1": "secret_value_5",
			"secret.dot2": "secret_value_6",
		}, "stringData")
		return nil
	}, "")
	stdout, _ = p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value_5")))
	assert.NotContains(t, stdout, base64.StdEncoding.EncodeToString([]byte("secret_value_6")))
	assert.Contains(t, stdout, "data[\"secret.dot1\"] | +***** (obfuscated)")
	assert.Contains(t, stdout, "data[\"secret.dot2\"] | +***** (obfuscated)")
}
