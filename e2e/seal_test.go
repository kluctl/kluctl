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
	"github.com/stretchr/testify/suite"
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

type SealedSecretsTestSuite struct {
	suite.Suite

	k  *test_utils.EnvTestCluster
	k2 *test_utils.EnvTestCluster

	certServer1 *certServer
	certServer2 *certServer
}

type certServer struct {
	server   http.Server
	url      string
	certHash string
}

func TestSealedSecrets(t *testing.T) {
	t.Parallel()
	suite.Run(t, &SealedSecretsTestSuite{})
}

func (s *SealedSecretsTestSuite) SetupSuite() {
	s.k = defaultCluster1
	s.k2 = defaultCluster2

	test_resources.ApplyYaml("sealed-secrets.yaml", s.k)
	test_resources.ApplyYaml("sealed-secrets.yaml", s.k2)

	var err error
	s.certServer1, err = s.startCertServer()
	if err != nil {
		s.T().Fatal(err)
	}
	s.certServer2, err = s.startCertServer()
	if err != nil {
		s.T().Fatal(err)
	}
}

func (s *SealedSecretsTestSuite) TearDownSuite() {
	s.k = nil
	s.k2 = nil

	if s.certServer1 != nil {
		s.certServer1.server.Close()
	}
	if s.certServer2 != nil {
		s.certServer2.server.Close()
	}
	s.certServer1 = nil
	s.certServer2 = nil
}

func (s *SealedSecretsTestSuite) startCertServer() (*certServer, error) {
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

func (s *SealedSecretsTestSuite) addProxyVars(p *testProject) {
	f := func(idx int, k *test_utils.EnvTestCluster, cs *certServer) {
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_API_HOST=%s", idx, k.RESTConfig().Host))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_NAMESPACE=%s", idx, "kube-system"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_NAME=%s", idx, "sealed-secrets-controller"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_SERVICE_PORT=%s", idx, "http"))
		p.extraEnv = append(p.extraEnv, fmt.Sprintf("KLUCTL_K8S_SERVICE_PROXY_%d_LOCAL_URL=%s", idx, cs.url))
	}
	f(0, s.k, s.certServer1)
	f(1, s.k2, s.certServer2)
}

func (s *SealedSecretsTestSuite) prepareSealTest(k *test_utils.EnvTestCluster, namespace string, secrets map[string]string, varsSources []*uo.UnstructuredObject, proxy bool) *testProject {
	p := &testProject{}
	p.init(s.T(), k, fmt.Sprintf("seal-%s", namespace))
	if proxy {
		s.addProxyVars(p)
	}

	createNamespace(s.T(), k, namespace)

	s.addSecretsSet(p, "test", varsSources)
	s.addSecretsSetToTarget(p, "test-target", "test")

	addSecretDeployment(p, "secret-deployment", secrets, true, resourceOpts{name: "secret", namespace: namespace})

	return p
}

func (s *SealedSecretsTestSuite) addSecretsSet(p *testProject, name string, varsSources []*uo.UnstructuredObject) {
	p.updateSecretSet(name, func(secretSet *uo.UnstructuredObject) {
		_ = secretSet.SetNestedField(varsSources, "vars")
	})
}

func (s *SealedSecretsTestSuite) addSecretsSetToTarget(p *testProject, targetName string, secretSetName string) {
	p.updateTarget(targetName, func(target *uo.UnstructuredObject) {
		l, _, _ := target.GetNestedList("sealingConfig", "secretSets")
		l = append(l, secretSetName)
		_ = target.SetNestedField(l, "sealingConfig", "secretSets")
	})
}

func (s *SealedSecretsTestSuite) assertSealedSecret(k *test_utils.EnvTestCluster, namespace string, secretName string, expectedCertHash string, expectedSecrets map[string]string) {

	y := k.MustGet(s.T(), schema.GroupVersionResource{
		Group:    "bitnami.com",
		Version:  "v1alpha1",
		Resource: "sealedsecrets",
	}, namespace, secretName)

	h1 := y.GetK8sAnnotation("kluctl.io/sealedsecret-cert-hash")
	if h1 == nil {
		s.T().Fatal("kluctl.io/sealedsecret-cert-hash annotation not found")
	}
	s.Assertions.Equal(expectedCertHash, *h1)

	hashesStr := y.GetK8sAnnotation("kluctl.io/sealedsecret-hashes")
	if hashesStr == nil {
		s.T().Fatal("kluctl.io/sealedsecret-hashes annotation not found")
	}
	hashes, err := uo.FromString(*hashesStr)
	if err != nil {
		s.T().Fatal(err)
	}

	expectedHashes := map[string]any{}
	for k, v := range expectedSecrets {
		expectedHashes[k] = seal.HashSecret(k, []byte(v), secretName, namespace, "strict")
	}

	s.Assertions.Equal(expectedHashes, hashes.Object)
}

func (s *SealedSecretsTestSuite) TestSeal_WithOperator() {
	k := s.k
	namespace := "seal-with-operator"

	p := s.prepareSealTest(k, namespace,
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
	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")
	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_WithBootstrap() {
	k := s.k
	namespace := "seal-with-bootstrap"

	// deleting the crd causes kluctl to not recognize the operator, so it will do a bootstrap
	_ = k.DynamicClient.Resource(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions")).
		Delete(context.Background(), "sealedsecrets.bitnami.com", metav1.DeleteOptions{})

	p := s.prepareSealTest(k, namespace,
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

	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	test_resources.ApplyYaml("sealed-secrets.yaml", k)

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	pkCm := k.MustGetCoreV1(s.T(), "configmaps", "kube-system", "sealed-secrets-key-kluctl-bootstrap")
	certBytes, ok, _ := pkCm.GetNestedString("data", "tls.crt")
	s.Assertions.True(ok)

	cert, err := seal.ParseCert([]byte(certBytes))
	s.Assertions.NoError(err)

	certHash, err := seal.HashPublicKey(cert)
	s.Assertions.NoError(err)

	s.assertSealedSecret(k, namespace, "secret", certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_MultipleVarSources() {
	k := s.k
	namespace := "seal-multiple-vs"

	p := s.prepareSealTest(k, namespace,
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
	defer p.cleanup()

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_MultipleSecretSets() {
	k := s.k
	namespace := "seal-multiple-ss"

	p := s.prepareSealTest(k, namespace,
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

	defer p.cleanup()

	s.addSecretsSet(p, "test2", []*uo.UnstructuredObject{
		uo.FromMap(map[string]interface{}{
			"values": map[string]interface{}{
				"s2": "v2",
			},
		}),
	})
	s.addSecretsSetToTarget(p, "test-target", "test2")

	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_MultipleTargets() {
	k := s.k
	namespace := "seal-multiple-targets"

	p := s.prepareSealTest(k, namespace,
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
	defer p.cleanup()

	s.addSecretsSet(p, "test2", []*uo.UnstructuredObject{
		uo.FromMap(map[string]interface{}{
			"values": map[string]interface{}{
				"s1": "v3",
				"s2": "v4",
			},
		}),
	})
	s.addSecretsSetToTarget(p, "test-target2", "test2")

	p.mergeKubeconfig(s.k2)
	createNamespace(s.T(), s.k2, namespace)
	p.updateTarget("test-target", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(s.k.Context, "context")
	})
	p.updateTarget("test-target2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField(s.k2.Context, "context")
	})

	p.KluctlMust("seal", "-t", "test-target")
	p.KluctlMust("seal", "-t", "test-target2")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target2/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")
	p.KluctlMust("deploy", "--yes", "-t", "test-target2")

	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
	s.assertSealedSecret(s.k2, namespace, "secret", s.certServer2.certHash, map[string]string{
		"s1": "v3",
		"s2": "v4",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_File() {
	k := s.k
	namespace := "seal-file"

	p := s.prepareSealTest(k, namespace,
		map[string]string{
			"s1": "{{ secrets.s1 }}",
			"s2": "{{ secrets.s2 }}",
		},
		[]*uo.UnstructuredObject{
			uo.FromMap(map[string]interface{}{
				"file": utils.StrPtr("secret-values.yaml"),
			}),
		}, true)
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

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}

func (s *SealedSecretsTestSuite) TestSeal_Vault() {
	k := s.k
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
	defer server.Close()

	vaultUrl := server.URL

	p := s.prepareSealTest(k, namespace,
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
	defer p.cleanup()

	p.extraEnv = append(p.extraEnv, "VAULT_TOKEN=root")
	p.KluctlMust("seal", "-t", "test-target")

	sealedSecretsDir := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())
	assert.FileExists(s.T(), filepath.Join(sealedSecretsDir, ".sealed-secrets/secret-deployment/test-target/secret-secret.yml"))

	p.KluctlMust("deploy", "--yes", "-t", "test-target")

	s.assertSealedSecret(k, namespace, "secret", s.certServer1.certHash, map[string]string{
		"s1": "v1",
		"s2": "v2",
	})
}
