package test_utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	port_tool "github.com/kluctl/kluctl/v2/e2e/test-utils/port-tool"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

var nonLoopbackIp = findNonLoopbackIp()
var caBytes, serverCert, clientCert, clientKey, _ = certsetup()

type TestHttpServer struct {
	TLSEnabled             bool
	TLSClientCertEnabled   bool
	NoLoopbackProxyEnabled bool

	Username string
	Password string

	Server     *httptest.Server
	ServerCAs  []byte
	ClientCAs  []byte
	ClientCert []byte
	ClientKey  []byte
}

func (s *TestHttpServer) Start(t *testing.T, h http.Handler) {
	authH := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if s.Username != "" || s.Password != "" {
			u, p, ok := request.BasicAuth()
			if !ok {
				writer.Header().Add("WWW-Authenticate", "Basic")
				http.Error(writer, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
			if u != s.Username || p != s.Password {
				http.Error(writer, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(writer, request)
	})

	s.Server = &httptest.Server{
		Listener: port_tool.NewListenerWithUniquePort("127.0.0.1"),
		Config:   &http.Server{Handler: authH},
	}

	if s.TLSEnabled {
		s.Server.TLS = &tls.Config{
			Certificates: []tls.Certificate{*serverCert},
		}
		s.ServerCAs = caBytes

		if s.TLSClientCertEnabled {
			s.ClientCAs = caBytes
			s.ClientCert = clientCert
			s.ClientKey = clientKey

			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(caBytes)

			s.Server.TLS.ClientAuth = tls.RequireAndVerifyClientCert
			s.Server.TLS.ClientCAs = certPool
		}

		s.Server.StartTLS()
	} else {
		s.Server.Start()
	}

	transport := s.Server.Client().Transport.(*http.Transport)
	if s.TLSClientCertEnabled {
		clientCertX, _ := tls.X509KeyPair(clientCert, clientKey)

		transport.TLSClientConfig.Certificates = append(transport.TLSClientConfig.Certificates, clientCertX)
	}

	if s.NoLoopbackProxyEnabled {
		s.startTLSProxy(t)
	}

	t.Cleanup(s.Server.Close)
}

// the TLS proxy is required because oras is treating local registries as non-tls, so we must use a non-loopback IP
func (s *TestHttpServer) startTLSProxy(t *testing.T) {
	u, _ := url.Parse(s.Server.URL)

	localhostIp := net.ParseIP(u.Hostname())
	port, _ := strconv.ParseInt(u.Port(), 10, 32)

	frontendAddr := &net.TCPAddr{IP: nonLoopbackIp}
	backendAddr := &net.TCPAddr{IP: localhostIp, Port: int(port)}

	l := port_tool.NewListenerWithUniquePort(frontendAddr.IP.String())
	p, err := NewTCPProxy(l, backendAddr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		p.Close()
	})

	u.Host = p.FrontendAddr().String()
	s.Server.URL = u.String()

	go func() {
		p.Run()
	}()
}

func findNonLoopbackIp() net.IP {
	ifs, _ := net.Interfaces()
	var goodAddr *net.IPNet
outer:
	for _, x := range ifs {
		addrs, _ := x.Addrs()
		if len(addrs) == 0 {
			continue
		}
		if (x.Flags&net.FlagRunning) != 0 && (x.Flags&net.FlagLoopback) == 0 {
			for _, a := range addrs {
				if a.Network() == "ip+net" {
					a2 := a.(*net.IPNet)
					if a2.IP.Equal(a2.IP.To4()) {
						goodAddr = a2
						break outer
					}
				}
			}
		}
	}
	if goodAddr == nil {
		panic("no good listen address found")
	}
	return goodAddr.IP
}

func certsetup() (caPEMBytes []byte, serverCert *tls.Certificate, clientCertPEMBytes []byte, clientKeyPEMBytes []byte, err error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	caPEMBytes = caPEM.Bytes()

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	serverCertPEM, serverKeyPEM := generateCert(ca, caPrivKey)

	serverCert2, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	serverCert = &serverCert2

	clientCertPEMBytes, clientKeyPEMBytes = generateCert(ca, caPrivKey)

	return
}

func generateCert(ca *x509.Certificate, caPrivKey *rsa.PrivateKey) ([]byte, []byte) {
	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		IPAddresses:  []net.IP{nonLoopbackIp, net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	return certPEM.Bytes(), certPrivKeyPEM.Bytes()
}
