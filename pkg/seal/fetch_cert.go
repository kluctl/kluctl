package seal

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"io/ioutil"
	v12 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/cert"
)

func fetchCert(ctx context.Context, k *k8s.K8sCluster, namespace string, controllerName string) (*rsa.PublicKey, error) {
	certData, err := openCertFromController(k, namespace, controllerName)
	if err != nil {
		if controllerName == "sealed-secrets-controller" {
			s2, err2 := openCertFromController(k, namespace, "sealed-secrets")
			if err2 == nil {
				status.Warning(ctx, "Looks like you have sealed-secrets controller installed with name 'sealed-secrets', which comes from a legacy kluctl version that deployed it with a non-default name. Please consider re-deploying sealed-secrets operator manually.")
				err = nil
				certData = s2
			}
		}

		if err != nil {
			status.Warning(ctx, "Failed to retrieve public certificate from sealed-secrets-controller, re-trying with bootstrap secret")
			certData, err = openCertFromBootstrap(k, namespace)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve sealed secrets public key: %w", err)
			}
		}
	}

	return parseKey(certData)
}

func openCertFromBootstrap(k *k8s.K8sCluster, namespace string) ([]byte, error) {
	ref := k8s2.NewObjectRef("", "v1", "ConfigMap", configMapName, namespace)
	cm, _, err := k.GetSingleObject(ref)
	if err != nil {
		return nil, err
	}

	v, ok, err := cm.GetNestedString("data", v12.TLSCertKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s key not found in ConfigMap %s", v12.TLSCertKey, configMapName)
	}

	return []byte(v), nil
}

func openCertFromController(k *k8s.K8sCluster, namespace, name string) ([]byte, error) {
	portName, err := getServicePortName(k, namespace, name)
	if err != nil {
		return nil, err
	}
	r, err := k.ProxyGet("http", namespace, name, portName, "/v1/cert.pem", nil)
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

func getServicePortName(k *k8s.K8sCluster, namespace, serviceName string) (string, error) {
	ref := k8s2.NewObjectRef("", "v1", "Service", serviceName, namespace)
	service, _, err := k.GetSingleObject(ref)
	if err != nil {
		return "", fmt.Errorf("cannot get sealed secret service: %v", err)
	}

	n, ok, err := service.GetNestedString("spec", "ports", 0, "name")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("spec.ports[0].name not in service object %s", serviceName)
	}

	return n, nil
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
