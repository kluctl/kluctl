package e2e

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/seal"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	certUtil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

type certServer struct {
	server   http.Server
	url      string
	certHash string
}

var certServer1 *certServer
var certServer2 *certServer

func init() {
	var err error
	certServer1, err = startCertServer()
	if err != nil {
		panic(err)
	}
	certServer2, err = startCertServer()
	if err != nil {
		panic(err)
	}
}

func startCertServer() (*certServer, error) {
	key, cert, err := crypto.GeneratePrivateKeyAndCert(2048, 10*365*24*time.Hour, "tests.kluctl.io")
	if err != nil {
		return nil, err
	}

	certbytes := []byte{}
	certbytes = append(certbytes, pem.EncodeToMemory(&pem.Block{Type: certUtil.CertificateBlockType, Bytes: cert.Raw})...)
	keybytes := pem.EncodeToMemory(&pem.Block{Type: keyutil.RSAPrivateKeyBlockType, Bytes: x509.MarshalPKCS1PrivateKey(key)})
	_ = keybytes
	l, err := net.Listen("tcp", "")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/cert.pem", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(certbytes)
	}))

	certUrl := fmt.Sprintf("http://localhost:%d", l.Addr().(*net.TCPAddr).Port)

	var cs certServer
	cs.url = certUrl
	cs.server.Handler = mux
	cs.certHash, err = seal.HashPublicKey(cert)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = cs.server.Serve(l)
	}()

	return &cs, nil
}

func addProxyVars(p *testProject) {
	f := func(idx int, k *test_utils.EnvTestCluster, cs *certServer) {
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_API_HOST=%s", idx, k.RESTConfig().Host))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_NAMESPACE=%s", idx, "kube-system"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_NAME=%s", idx, "sealed-secrets-controller"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_PORT=%s", idx, "http"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_LOCAL_URL=%s", idx, cs.url))
	}
	f(0, defaultCluster1, certServer1)
	f(1, defaultCluster2, certServer2)
}

func prepareSealTest(t *testing.T, k *test_utils.EnvTestCluster, namespace string, secrets map[string]string, varsSources []*uo.UnstructuredObject, proxy bool) *testProject {
	p := &testProject{}
	p.init(t, k, fmt.Sprintf("seal-%s", namespace))

	if proxy {
		addProxyVars(p)
	}

	createNamespace(t, k, namespace)

	addSecretsSet(p, "test", varsSources)
	addSecretsSetToTarget(p, "test-target", "test")

	addSecretDeployment(p, "secret-deployment", secrets, resourceOpts{name: "secret", namespace: namespace})

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

func assertSealedSecret(t *testing.T, k *test_utils.EnvTestCluster, namespace string, secretName string, expectedCertHash string, expectedSecrets map[string]string) {
	y := k.MustGet(t, schema.GroupVersionResource{
		Group:    "bitnami.com",
		Version:  "v1alpha1",
		Resource: "sealedsecrets",
	}, namespace, secretName)

	h1 := y.GetK8sAnnotation("kluctl.io/sealedsecret-cert-hash")
	if h1 == nil {
		t.Fatal("kluctl.io/sealedsecret-cert-hash annotation not found")
	}
	assert.Equal(t, expectedCertHash, *h1)

	hashesStr := y.GetK8sAnnotation("kluctl.io/sealedsecret-hashes")
	if hashesStr == nil {
		t.Fatal("kluctl.io/sealedsecret-hashes annotation not found")
	}
	hashes, err := uo.FromString(*hashesStr)
	if err != nil {
		t.Fatal(err)
	}

	expectedHashes := map[string]any{}
	for k, v := range expectedSecrets {
		expectedHashes[k] = seal.HashSecret(k, []byte(v), secretName, namespace, "strict")
	}

	assert.Equal(t, expectedHashes, hashes.Object)
}

func TestSeal_WithOperator(t *testing.T) {
	t.Parallel()
	k := defaultCluster1
	namespace := "seal-with-operator"

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
		}, true)

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")
	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_WithBootstrap(t *testing.T) {
	// this test must NOT run in parallel

	k := defaultCluster1
	namespace := "seal-with-bootstrap"

	// deleting the crd causes kluctl to not recognize the operator, so it will do a bootstrap
	_ = k.DynamicClient.Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
		Delete(context.Background(), "sealedsecrets.bitnami.com", metav1.DeleteOptions{})

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
		}, false)

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	test_resources.ApplyYaml("sealed-secrets.yaml", k)

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	pkCm := k.MustGetCoreV1(t, "configmaps", "kube-system", "sealed-secrets-key-kluctl-bootstrap")
	certBytes, ok, _ := pkCm.GetNestedString("data", "tls.crt")
	assert.True(t, ok)

	cert, err := seal.ParseCert([]byte(certBytes))
	assert.NoError(t, err)

	certHash, err := seal.HashPublicKey(cert)
	assert.NoError(t, err)

	assertSealedSecret(t, k, namespace, "secret", certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleVarSources(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-vs"

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
		}, true)

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleSecretSets(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-ss"

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
		}, true)

	addSecretsSet(p, "test2", []*uo.UnstructuredObject{
		uo.FromMap(map[string]interface{}{
			"values": map[string]interface{}{
				"s2": "v2",
			},
		}),
	})
	addSecretsSetToTarget(p, "test-target", "test2")

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_MultipleTargets(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-targets"

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
		}, true)

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

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target2/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")
	p.KluctlMust("deploy", "--yes", "-t", "test-target2")

	assertSealedSecret(t, defaultCluster1, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
	assertSealedSecret(t, defaultCluster2, namespace, "secret", certServer2.certHash, map[string]string{
		"s1": "v3",
		"s2": "v4",
	})
}

func TestSeal_MultipleSecrets(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-multiple-secrets"

	secret1 := map[string]string{
		"s1": "{{ secrets.s1 }}",
	}
	secret2 := map[string]string{
		"s2": "{{ secrets.s2 }}",
	}

	p := prepareSealTest(t, k, namespace,
		secret1,
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"values": map[string]interface{}{
					"s1": "v1",
					"s2": "v2",
				},
			}),
		}, true)
	addSecretDeployment(p, "secret-deployment2", secret2, resourceOpts{name: "secret2", namespace: namespace})

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment2/test-target/secret-secret2.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
	})
	assertSealedSecret(t, k, namespace, "secret2", certServer1.certHash, map[string]string{
		"s2": "v2",
	})
}

func TestSeal_File(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-file"

	p := prepareSealTest(t, k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"file": utils.StrPtr("secret-values.yaml"),
			}),
		}, true)

	p.updateYaml("secret-values.yaml", func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"secrets": map[string]interface{}{
				"s1": "v1",
				"s2": "v2",
			},
		})
		return nil
	}, "")

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func TestSeal_Vault(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	namespace := "seal-vault"

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/secret/data/secret" {
			http.NotFound(writer, request)
			return
		}
		o := uo.FromMap(map[string]interface{}{
			"data": map[string]interface{}{
				"data": map[string]interface{}{
					"secrets": map[string]interface{}{
						"s1": "v1",
						"s2": "v2",
					},
				},
			},
		})
		s, _ := yaml.WriteJsonString(o)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(s))
	}))

	vaultUrl := server.URL

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
		}, true)

	p.extraEnv = append(p.extraEnv, "VAULT_TOKEN=root")
	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(t, filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	assertSealedSecret(t, k, namespace, "secret", certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}
