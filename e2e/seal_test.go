package e2e

import (
	"encoding/base64"
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func installSealedSecretsOperator(k *test_utils.EnvTestCluster) {
	test_resources.ApplyYaml("sealed-secrets.yaml", k)
}

func waitForSealedSecretsOperator(t *testing.T, k *test_utils.EnvTestCluster) {
	waitForReadiness(t, k, "kube-system", "deployment/sealed-secrets-controller", 5*time.Minute)
}

func deleteSealedSecretsOperator(k *test_utils.EnvTestCluster) {
	test_resources.DeleteYaml("sealed-secrets.yaml", k)
	_, _, _ = k.Kubectl("-n", "kube-system", "delete", "secret", "-l", "sealedsecrets.bitnami.com/sealed-secrets-key", "--wait")
	_, _, _ = k.Kubectl("-n", "kube-system", "delete", "configmap", "sealed-secrets-key-kluctl-bootstrap", "--wait")
}

func installVault(k *test_utils.EnvTestCluster) {
	_, _, _ = k.Kubectl("create", "ns", "vault")
	test_resources.ApplyYaml("vault.yaml", k)
}

func waitForVault(t *testing.T, k *test_utils.EnvTestCluster) {
	waitForReadiness(t, k, "vault", "statefulset/vault", 5*time.Minute)
}

func init() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		installSealedSecretsOperator(defaultCluster1)
		installVault(defaultCluster1)
	}()
	go func() {
		defer wg.Done()
		installSealedSecretsOperator(defaultCluster2)
	}()
	wg.Wait()
}

func prepareSealTest(t *testing.T, k *test_utils.EnvTestCluster, namespace string, secrets map[string]string, varsSources []*uo.UnstructuredObject) *testProject {
	p := &testProject{}
	p.init(t, k, fmt.Sprintf("seal-%s", namespace))

	createNamespace(t, k, namespace)

	addSecretsSet(p, "test", varsSources)
	addSecretsSetToTarget(p, "test-target", "test")

	addSecretDeployment(p, "secret-deployment", secrets, true, resourceOpts{name: "secret", namespace: namespace})

	return p
}

func addSecretsSet(p *testProject, name string, varsSources []*uo.UnstructuredObject) {
	p.updateSecretSet(name, func(secretSet *uo.UnstructuredObject) {
		_ = secretSet.SetNestedField(varsSources, "vars")
	})
}

func addSecretsSetToTarget(p *testProject, targetName string, secretSetName string) {
	p.updateTarget(targetName, func(target *uo.UnstructuredObject) {
		l, _, _ := target.GetNestedList("sealingConfig", "secretSets")
		l = append(l, secretSetName)
		_ = target.SetNestedField(l, "sealingConfig", "secretSets")
	})
}

func assertDecryptedSecrets(t *testing.T, k *test_utils.EnvTestCluster, namespace string, secretName string, expectedSecrets map[string]string) {
	s := k.KubectlYamlMust(t, "-n", namespace, "get", "secret", secretName)

	for key, value := range expectedSecrets {
		x, _, _ := s.GetNestedString("data", key)
		decoded, _ := base64.StdEncoding.DecodeString(x)
		assert.Equal(t, value, string(decoded))
	}
}

func TestSeal_WithOperator(t *testing.T) {
	k := defaultCluster1
	namespace := "seal-with-operator"

	waitForSealedSecretsOperator(t, k)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
					"s2": "v2",
				},
			}),
		},
	)
	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_WithBootstrap(t *testing.T) {
	k := defaultCluster2
	namespace := "seal-with-bootstrap"

	// we still wait for it to be ready before we then delete it
	// this way it's pre-pulled and pre-warmed when we later start it
	waitForSealedSecretsOperator(t, k)
	deleteSealedSecretsOperator(k)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
					"s2": "v2",
				},
			}),
		},
	)
	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	installSealedSecretsOperator(k)
	waitForSealedSecretsOperator(t, k)

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleVarSources(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-vs"

	waitForSealedSecretsOperator(t, k)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
				},
			}),
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s2": "v2",
				},
			}),
		},
	)
	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleSecretSets(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-ss"

	waitForSealedSecretsOperator(t, k)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
				},
			}),
		},
	)
	defer p.cleanup()

	addSecretsSet(p, "test2", []*uo.UnstructuredObject{
		uo.FromMap(map[string]interface{}{
			"values": map[string]interface{}{
				"s2": "v2",
			},
		}),
	})
	addSecretsSetToTarget(p, "test-target", "test2")

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleTargets(t *testing.T) {
	k := defaultCluster1
	namespace := "seal-multiple-targets"

	waitForSealedSecretsOperator(t, k)
	waitForSealedSecretsOperator(t, defaultCluster2)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
					"s2": "v2",
				},
			}),
		},
	)
	defer p.cleanup()

	addSecretsSet(p, "test2", []*uo.UnstructuredObject{
		uo.FromMap(map[string]interface{}{
			"values": map[string]interface{}{
				"s1": "v3",
				"s2": "v4",
			},
		}),
	})
	addSecretsSetToTarget(p, "test-target2", "test2")

	p.mergeKubeconfig(defaultCluster2)
	createNamespace(t, defaultCluster2, namespace)
	p.updateTarget("test-target", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster1.Context, "context")
	})
	p.updateTarget("test-target2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(defaultCluster2.Context, "context")
	})

	p.KluctlMust("seal", "-t", "test-target")
	p.KluctlMust("seal", "-t", "test-target2")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target2/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")
	p.KluctlMust("deploy", "--yes", "-t", "test-target2")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
	waitForReadiness(t, defaultCluster2, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, defaultCluster2, namespace, "secret", map[string]string{
		"s1": "v3",
		"s2": "v4",
	})
}

func TestSeal_File(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-file"

	waitForSealedSecretsOperator(t, k)

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"file": utils.StrPtr("secret-values.yaml"),
			}),
		},
	)
	defer p.cleanup()

	p.gitServer.UpdateYaml(p.getKluctlProjectRepo(), "secret-values.yaml", func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"secrets": map[string]interface{}{
				"s1": "v1",
				"s2": "v2",
			},
		})
		return nil
	}, "")

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_Vault(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-vault"

	waitForSealedSecretsOperator(t, k)
	waitForVault(t, k)

	u, err := url.Parse(defaultCluster1.RESTConfig().Host)
	if err != nil {
		t.Fatal(err)
	}

	vaultUrl := fmt.Sprintf("http://%s:%d", u.Hostname(), defaultKindCluster1VaultPort)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/secret/data/secret", vaultUrl), strings.NewReader(`{"data": {"secrets":{"s1":"v1","s2":"v2"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vault-Token", "root")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("vault response status %d, body=%s", resp.StatusCode, string(body))
	}

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"vault": map[string]interface{}{
					"address": vaultUrl,
					"path":    "secret/data/secret",
				},
			}),
		},
	)
	defer p.cleanup()

	p.extraEnv = append(p.extraEnv, "VAULT_TOKEN=root")
	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getSealedSecretsRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	waitForReadiness(t, k, namespace, "secret/secret", 1*time.Minute)
	assertDecryptedSecrets(t, k, namespace, "secret", map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}
