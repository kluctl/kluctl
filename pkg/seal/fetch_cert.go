package seal

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	v12 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/cert"
)

func fetchCert(k *k8s.K8sCluster, namespace string, controllerName string) (*rsa.PublicKey, error) {
	var certData []byte

	err := k.WithCoreV1(func(client *v1.CoreV1Client) error {
		s, err := openCertFromController(client, namespace, controllerName)
		if err != nil {
			if controllerName == "sealed-secrets-controller" {
				s2, err2 := openCertFromController(client, namespace, "sealed-secrets")
				if err2 == nil {
					log.Warningf("Looks like you have sealed-secrets controller installed with name 'sealed-secrets', which comes from a legacy kluctl version that deployed it with a non-default name. Please consider re-deploying sealed-secrets operator manually.")
					err = nil
					s = s2
				}
			}

			if err != nil {
				log.Warningf("Failed to retrieve public certificate from sealed-secrets-controller, re-trying with bootstrap secret")
				s, err = openCertFromBootstrap(client, namespace)
				if err != nil {
					return fmt.Errorf("failed to retrieve sealed secrets public key: %w", err)
				}
			}
		}
		certData = s
		return nil
	})
	if err != nil {
		return nil, err
	}

	cert, err := parseKey(certData)
	return cert, err
}

func openCertFromBootstrap(c *v1.CoreV1Client, namespace string) ([]byte, error) {
	cm, err := c.ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	v, ok := cm.Data[v12.TLSCertKey]
	if !ok {
		return nil, fmt.Errorf("%s key not found in ConfigMap %s", v12.TLSCertKey, configMapName)
	}

	return []byte(v), nil
}

func openCertFromController(c v1.CoreV1Interface, namespace, name string) ([]byte, error) {
	portName, err := getServicePortName(c, namespace, name)
	if err != nil {
		return nil, err
	}
	r, err := c.Services(namespace).ProxyGet("http", name, portName, "/v1/cert.pem", nil).Stream(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cannot fetch certificate: %v", err)
	}
	defer r.Close()

	cert, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return cert, nil
}

func getServicePortName(client v1.CoreV1Interface, namespace, serviceName string) (string, error) {
	service, err := client.Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get sealed secret service: %v", err)
	}
	return service.Spec.Ports[0].Name, nil
}

func parseKey(data []byte) (*rsa.PublicKey, error) {
	certs, err := cert.ParseCertsPEM(data)
	if err != nil {
		return nil, err
	}

	// ParseCertsPem returns error if len(certs) == 0, but best to be sure...
	if len(certs) == 0 {
		return nil, errors.New("failed to read any certificates")
	}

	cert, ok := certs[0].PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("fxpected RSA public key but found %v", certs[0].PublicKey)
	}

	return cert, nil
}
