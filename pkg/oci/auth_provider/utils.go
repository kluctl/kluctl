package auth_provider

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/pkg/errors"
)

func newTLSConfig(cert []byte, key []byte, ca []byte, insecureSkipTLSverify bool) (*tls.Config, error) {
	config := tls.Config{
		InsecureSkipVerify: insecureSkipTLSverify,
	}

	if cert != nil && key != nil {
		x, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{x}
	}

	if ca != nil {
		cp := x509.NewCertPool()
		if !cp.AppendCertsFromPEM(ca) {
			return nil, errors.Errorf("failed to append certificates")
		}
		config.RootCAs = cp
	}

	return &config, nil
}
