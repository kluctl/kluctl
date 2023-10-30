package sourceoverride

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"math/big"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const certSecretName = "kluctl-source-override-cert"
const caConfigMapName = "kluctl-source-override-ca"

func WaitAndLoadTLSCert(ctx context.Context, c client.Reader, controllerNamespace string) (*tls.Certificate, error) {
	var certSecret corev1.Secret

	for {
		err := c.Get(ctx, client.ObjectKey{Name: certSecretName, Namespace: controllerNamespace}, &certSecret)
		if err == nil {
			break
		} else if errors.IsNotFound(err) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				continue
			}
		} else {
			return nil, err
		}
	}

	certBytes, ok := certSecret.Data["tls.crt"]
	if !ok {
		return nil, fmt.Errorf("missing tls.crt in %s", certSecretName)
	}
	keyBytes, ok := certSecret.Data["tls.key"]
	if !ok {
		return nil, fmt.Errorf("missing tls.key in %s", certSecretName)
	}

	cert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func LoadTLSCA(ctx context.Context, c client.Reader, controllerNamespace string) (*x509.CertPool, error) {
	var caCM corev1.ConfigMap

	err := c.Get(ctx, client.ObjectKey{Name: caConfigMapName, Namespace: controllerNamespace}, &caCM)
	if err != nil {
		return nil, err
	}

	caPEM, ok := caCM.Data["ca.pem"]
	if !ok {
		return nil, fmt.Errorf("missing ca.pem in %s", caConfigMapName)
	}

	cp := x509.NewCertPool()
	ok = cp.AppendCertsFromPEM([]byte(caPEM))
	if !ok {
		return nil, fmt.Errorf("failed to add CA to pool")
	}

	return cp, nil
}

func InitTLS(ctx context.Context, c client.Client, controllerNamespace string) error {
	var certSecret corev1.Secret

	err := c.Get(ctx, client.ObjectKey{Name: certSecretName, Namespace: controllerNamespace}, &certSecret)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if err == nil {
		return initTLSCA(ctx, c, &certSecret, false)
	}

	caPem, cert, key, err := certsetup()
	if err != nil {
		return err
	}

	certSecret.Data = map[string][]byte{
		"ca.pem":  caPem,
		"tls.crt": cert,
		"tls.key": key,
	}

	certSecret.APIVersion = "v1"
	certSecret.Kind = "Secret"
	certSecret.Name = certSecretName
	certSecret.Namespace = controllerNamespace
	err = c.Patch(ctx, &certSecret, client.Apply, client.FieldOwner("kluctl-controller"))
	if err != nil {
		return err
	}
	return initTLSCA(ctx, c, &certSecret, true)
}

func initTLSCA(ctx context.Context, c client.Client, certSecret *corev1.Secret, force bool) error {
	var caCM corev1.ConfigMap

	if !force {
		err := c.Get(ctx, client.ObjectKey{Name: caConfigMapName, Namespace: certSecret.Namespace}, &caCM)
		if err != nil && !errors.IsNotFound(err) {
			return err
		} else if err == nil {
			if _, ok := caCM.Data["ca.pem"]; !ok {
				return fmt.Errorf("invalid CA configmap, missing ca.pem")
			}
			return nil
		}
	}

	caPem, ok := certSecret.Data["ca.pem"]
	if !ok {
		return fmt.Errorf("invalid cert secret, missing ca.pem")
	}

	caCM.Data = map[string]string{
		"ca.pem": string(caPem),
	}

	caCM.APIVersion = "v1"
	caCM.Kind = "ConfigMap"
	caCM.Name = caConfigMapName
	caCM.Namespace = certSecret.Namespace
	err := c.Patch(ctx, &caCM, client.Apply, client.FieldOwner("kluctl-controller"))
	if err != nil {
		return err
	}
	return nil
}

func certsetup() ([]byte, []byte, []byte, error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Kluctl"},
			Country:       []string{"Germany"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	err = pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err != nil {
		return nil, nil, nil, err
	}

	serverCertPEM, serverKeyPEM, err := generateCert(ca, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	return caPEM.Bytes(), serverCertPEM, serverKeyPEM, nil
}

func generateCert(ca *x509.Certificate, caPrivKey *rsa.PrivateKey) ([]byte, []byte, error) {
	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Kluctl"},
			Country:       []string{"Germany"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		// we connect via a local port-forward, so 127.0.0.1 is what we need
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost", "source-override"},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		//SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	err = pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err != nil {
		return nil, nil, err
	}

	certPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
	if err != nil {
		return nil, nil, err
	}

	return certPEM.Bytes(), certPrivKeyPEM.Bytes(), nil
}
