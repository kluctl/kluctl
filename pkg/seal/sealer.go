package seal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/bitnami-labs/sealed-secrets/pkg/crypto"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"golang.org/x/crypto/scrypt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path/filepath"
	"strconv"
)

const hashAnnotation = "kluctl.io/sealedsecret-hashes"
const certHashAnnotation = "kluctl.io/sealedsecret-cert-hash"

type Sealer struct {
	ctx         context.Context
	forceReseal bool
	cert        *x509.Certificate
	pubKey      *rsa.PublicKey
	certHash    string
}

func NewSealer(ctx context.Context, cert *x509.Certificate, forceReseal bool) (*Sealer, error) {
	s := &Sealer{
		ctx:         ctx,
		forceReseal: forceReseal,
		cert:        cert,
	}

	pk, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected RSA public key but found %v", cert.PublicKey)
	}
	s.pubKey = pk

	var err error
	s.certHash, err = HashPublicKey(cert)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func HashSecret(key string, secret []byte, secretName string, secretNamespace string, scope string) string {
	if secretNamespace == "" {
		secretNamespace = "*"
	}
	salt := fmt.Sprintf("%s-%s-%s", secretName, secretNamespace, key)
	if scope != "strict" {
		salt += "-" + scope
	}
	h, err := scrypt.Key(secret, []byte(salt), 1<<14, 8, 1, 64)
	if err != nil {
		panic(err)
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
	// todo
	b, err := crypto.HybridEncrypt(rand.Reader, s.pubKey, secret, encryptionLabel(secretNamespace, secretName, scope))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

type sealedSecret struct {
	content  *uo.UnstructuredObject
	hashes   *uo.UnstructuredObject
	certHash string
}

func buildSecretRef(o *uo.UnstructuredObject) string {
	return fmt.Sprintf("%s/%s", o.GetK8sNamespace(), o.GetK8sName())
}

func (s *Sealer) loadExistingSealedSecrets(p string) (map[string]*sealedSecret, error) {
	ret := map[string]*sealedSecret{}

	if !utils.Exists(p) {
		return ret, nil
	}

	list, err := uo.FromFileMulti(p)
	if err != nil {
		return nil, err
	}
	if len(list) != 1 {
		err = nil
	}

	for _, x := range list {
		var ss sealedSecret

		ss.content = x

		a := x.GetK8sAnnotation(hashAnnotation)
		if a != nil {
			ss.hashes, _ = uo.FromString(*a)
		}
		a = x.GetK8sAnnotation(certHashAnnotation)
		if a != nil {
			ss.certHash = *a
		}
		if ss.hashes == nil {
			ss.hashes = uo.New()
		}

		ret[buildSecretRef(x)] = &ss
	}
	return ret, nil
}

func (s *Sealer) SealFile(p string, targetFile string) error {
	err := os.MkdirAll(filepath.Dir(targetFile), 0o700)
	if err != nil {
		return err
	}

	existingSealedSecrets, err := s.loadExistingSealedSecrets(targetFile)
	if err != nil {
		return err
	}

	secrets, err := uo.FromFileMulti(p)
	if err != nil {
		return err
	}

	var result []any

	for _, o := range secrets {
		existing, _ := existingSealedSecrets[buildSecretRef(o)]
		newSealedSecret, err := s.sealSecret(o, existing)
		if err != nil {
			return err
		}
		result = append(result, newSealedSecret.content)
	}

	err = yaml.WriteYamlAllFile(targetFile, result)
	if err != nil {
		return err
	}
	return nil
}

func (s *Sealer) sealSecret(o *uo.UnstructuredObject, existing *sealedSecret) (*sealedSecret, error) {
	secretName := o.ToUnstructured().GetName()
	secretNamespace := o.ToUnstructured().GetNamespace()
	if secretNamespace == "" {
		secretNamespace = "default"
	}
	secretType, ok, err := o.GetNestedString("type")
	if err != nil {
		return nil, err
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

	secrets := make(map[string][]byte)

	data, ok, err := o.GetNestedObject("data")
	if err != nil {
		return nil, err
	}
	if ok {
		for k, v := range data.Object {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s is not a string", k)
			}
			secrets[k], err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 string for secret %s and key %s", secretName, k)
			}
		}
	}

	stringData, ok, err := o.GetNestedObject("stringData")
	if err != nil {
		return nil, err
	}
	if ok {
		for k, v := range stringData.Object {
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("%s is not a string", k)
			}
			secrets[k] = []byte(s)
		}
	}

	resultSecretHashes := make(map[string]string)

	var result sealedSecret
	result.content = uo.New()
	result.content.SetK8sGVK(schema.GroupVersionKind{Group: "bitnami.com", Version: "v1alpha1", Kind: "SealedSecret"})
	result.content.SetK8sName(secretName)
	result.content.SetK8sNamespace(secretNamespace)
	result.content.SetK8sAnnotation("sealedsecrets.bitnami.com/scope", *scope)
	if *scope == "namespace-wide" {
		result.content.SetK8sAnnotation("sealedsecrets.bitnami.com/namespace-wide", "true")
	}
	if *scope == "cluster-wide" {
		result.content.SetK8sAnnotation("sealedsecrets.bitnami.com/cluster-wide", "true")
	}
	_ = result.content.SetNestedField(secretType, "spec", "template", "type")
	metadata, ok, _ := o.GetNestedObject("metadata")
	if ok {
		result.content.SetNestedField(metadata.Object, "spec", "template", "metadata")
	}

	resealAll := false
	if s.forceReseal {
		resealAll = true
		status.Infof(s.ctx, "Forcing reseal of secrets in %s", secretName)
	} else if existing == nil || existing.certHash != s.certHash {
		resealAll = true
		status.Infof(s.ctx, "Cert for secret %s has changed, forcing reseal", secretName)
	}

	for k, v := range secrets {
		hash := HashSecret(k, v, secretName, secretNamespace, *scope)

		var existingHash string
		if existing != nil {
			existingHash, _, _ = existing.hashes.GetNestedString(k)
		}

		doEncrypt := existing == nil || resealAll
		if !doEncrypt && hash != existingHash {
			status.Infof(s.ctx, "Secret %s and key %s has changed, resealing", secretName, k)
			doEncrypt = true
		}

		if !doEncrypt {
			e, ok, _ := existing.content.GetNestedString("spec", "encryptedData", k)
			if ok {
				status.Tracef(s.ctx, "Secret %s and key %s is unchanged", secretName, k)
				result.content.SetNestedField(e, "spec", "encryptedData", k)
				resultSecretHashes[k] = hash
				continue
			} else {
				status.Infof(s.ctx, "Old encrypted secret %s and key %s not found", secretName, k)
				doEncrypt = true
			}
		}

		e, err := s.encryptSecret(v, secretName, secretNamespace, *scope)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret %s with key %s", secretName, k)
		}
		result.content.SetNestedField(e, "spec", "encryptedData", k)
		resultSecretHashes[k] = hash
	}

	resultSecretHashesStr, err := yaml.WriteYamlString(resultSecretHashes)
	if err != nil {
		return nil, err
	}
	result.content.SetK8sAnnotation(hashAnnotation, resultSecretHashesStr)
	result.content.SetK8sAnnotation(certHashAnnotation, s.certHash)

	return &result, nil
}
