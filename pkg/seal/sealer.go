package seal

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/scrypt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path"
	"reflect"
	"strconv"
)

const hashAnnotation = "kluctl.io/sealedsecret-hashes"
const clusterIdAnnotation = "kluctl.io/sealedsecret-cluster-id"

type Sealer struct {
	clusterConfig *types.ClusterConfig2
	forceReseal   bool
	cert          *rsa.PublicKey
	clusterId string
}

func NewSealer(k *k8s.K8sCluster, sealedSecretsNamespace string, sealedSecretsControllerName string, clusterConfig *types.ClusterConfig2, forceReseal bool) (*Sealer, error) {
	s := &Sealer{
		clusterConfig: clusterConfig,
		forceReseal:   forceReseal,
	}
	cert, err := fetchCert(k, sealedSecretsNamespace, sealedSecretsControllerName)
	if err != nil {
		return nil, err
	}
	s.cert = cert

	clusterId, err := getClusterId(k)
	if err != nil {
		return nil, err
	}

	s.clusterId = clusterId

	return s, nil
}

// We treat the hashed kube-root-ca.crt as cluster id for now. We also accept that it might change when keys
// get rotated.
func getClusterId(k *k8s.K8sCluster) (string, error) {
	o, _, err := k.GetSingleObject(types.ObjectRef{
		GVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		Name: "kube-root-ca.crt",
		Namespace: "kube-system",
	})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve kube-root-ca.crt: %w", err)
	}
	kubeRootCA, ok, err := uo.FromUnstructured(o).GetNestedString("data", "ca.crt")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve kube-root-ca.crt: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("failed to retrieve kube-root-ca.crt: ca.crt key is missing")
	}
	return utils.Sha256String(kubeRootCA), nil
}

func (s *Sealer) doHash(key string, secret []byte, secretName string, secretNamespace string, scope string) string {
	if secretNamespace == "" {
		secretNamespace = "*"
	}
	salt := fmt.Sprintf("%s-%s-%s-%s", s.clusterConfig.Name, secretName, secretNamespace, key)
	if scope != "strict" {
		salt += "-" + scope
	}
	h, err := scrypt.Key(secret, []byte(salt), 1<<14, 8, 1, 64)
	if err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(h)
}

func encryptionLabel(namespace string, name string, scope string) []byte {
	var l string
	switch scope {
	case "cluster-wide":
		l = ""
	case "namespace-wide":
		l = namespace
	case "strict":
		fallthrough
	default:
		l = fmt.Sprintf("%s/%s", namespace, name)
	}
	return []byte(l)
}

func (s *Sealer) encryptSecret(secret []byte, secretName string, secretNamespace string, scope string) (string, error) {
	b, err := crypto.HybridEncrypt(rand.Reader, s.cert, secret, encryptionLabel(secretNamespace, secretName, scope))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func (s *Sealer) SealFile(p string, targetFile string) error {
	baseName := path.Base(targetFile)
	err := os.MkdirAll(path.Dir(targetFile), 0o777)
	if err != nil {
		return err
	}

	o, err := uo.FromFile(p)
	if err != nil {
		return err
	}

	secretName := o.ToUnstructured().GetName()
	secretNamespace := o.ToUnstructured().GetNamespace()
	if secretNamespace == "" {
		secretNamespace = "default"
	}
	secretType, ok, err := o.GetNestedString("type")
	if err != nil {
		return err
	}
	if !ok {
		secretType = "Opaque"
	}

	var scope *string
	x, _, _ := o.GetNestedString("metadata", "annotations", "sealedsecrets.bitnami.com/namespace-wide")
	if b, _ := strconv.ParseBool(x); b {
		tmp := "namespace-wide"
		scope = &tmp
	}
	x, _, _ = o.GetNestedString("metadata", "annotations", "sealedsecrets.bitnami.com/cluster-wide")
	if b, _ := strconv.ParseBool(x); b {
		tmp := "cluster-wide"
		scope = &tmp
	}
	if scope == nil {
		x, _, _ = o.GetNestedString("metadata", "annotations", "sealedsecrets.bitnami.com/scope")
		if x == "" {
			x = "strict"
		}
		scope = &x
	}

	var existingContent *uo.UnstructuredObject
	var existingHashes *uo.UnstructuredObject
	var existingClusterId string

	if utils.Exists(targetFile) {
		existingContent, err = uo.FromFile(targetFile)
		a := existingContent.GetK8sAnnotation(hashAnnotation)
		if a != nil {
			existingHashes, _ = uo.FromString(*a)
		}
		a = existingContent.GetK8sAnnotation(clusterIdAnnotation)
		if a != nil {
			existingClusterId = *a
		}
	}
	if existingHashes == nil {
		existingHashes = uo.New()
	}

	secrets := make(map[string][]byte)

	data, ok, err := o.GetNestedObject("data")
	if err != nil {
		return err
	}
	if ok {
		for k, v := range data.Object {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("%s is not a string", k)
			}
			secrets[k], err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("failed to decode base64 string for secret %s and key %s", secretName, k)
			}
		}
	}

	stringData, ok, err := o.GetNestedObject("stringData")
	if err != nil {
		return err
	}
	for k, v := range stringData.Object {
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s is not a string", k)
		}
		secrets[k] = []byte(s)
	}

	resultSecretHashes := make(map[string]string)

	result := uo.New()
	result.SetK8sGVK(schema.GroupVersionKind{Group: "bitnami.com", Version: "v1alpha1", Kind: "SealedSecret"})
	result.SetK8sName(secretName)
	result.SetK8sNamespace(secretNamespace)
	result.SetK8sAnnotation("sealedsecrets.bitnami.com/scope", *scope)
	if *scope == "namespace-wide" {
		result.SetK8sAnnotation("sealedsecrets.bitnami.com/namespace-wide", "true")
	}
	if *scope == "cluster-wide" {
		result.SetK8sAnnotation("sealedsecrets.bitnami.com/cluster-wide", "true")
	}
	_ = result.SetNestedField(secretType, "spec", "template", "type")
	metadata, ok, _ := o.GetNestedObject("metadata")
	if ok {
		result.SetNestedField(metadata.Object, "spec", "template", "metadata")
	}

	resealAll := true
	if s.forceReseal {
		resealAll = true
		log.Infof("Forcing reseal of secrets in %s", secretName)
	} else if existingClusterId != s.clusterId {
		resealAll = true
		log.Infof("Target cluster for secret %s has changed, forcing reseal", secretName)
	}

	for k, v := range secrets {
		hash := s.doHash(k, v, secretName, secretNamespace, *scope)
		existingHash, _, _ := existingHashes.GetNestedString(k)

		doEncrypt := resealAll
		if !doEncrypt && hash != existingHash {
			log.Infof("Secret %s and key %s has changed, resealing", secretName, k)
			doEncrypt = true
		}

		if !doEncrypt {
			e, ok, _ := existingContent.GetNestedString("spec", "encryptedData", k)
			if ok {
				log.Debugf("Secret %s and key %s is unchanged", secretName, k)
				result.SetNestedField(e, "spec", "encryptedData", k)
				resultSecretHashes[k] = hash
				continue
			} else {
				log.Infof("Old encrypted secret %s and key %s not found", secretName, k)
				doEncrypt = true
			}
		}

		e, err := s.encryptSecret(v, secretName, secretNamespace, *scope)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret %s with key %s", secretName, k)
		}
		result.SetNestedField(e, "spec", "encryptedData", k)
		resultSecretHashes[k] = hash
	}

	resultSecretHashesStr, err := yaml.WriteYamlString(resultSecretHashes)
	if err != nil {
		return err
	}
	result.SetK8sAnnotation(hashAnnotation, resultSecretHashesStr)
	result.SetK8sAnnotation(clusterIdAnnotation, s.clusterId)

	if reflect.DeepEqual(existingContent, result) {
		log.Infof("Skipped %s as it did not change", baseName)
		return nil
	}

	err = yaml.WriteYamlFile(targetFile, result)
	if err != nil {
		return err
	}
	return nil
}
