package sourceoverride

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

const (
	maxTarSize   = 32 * 1024 * 1024
	maxUntarSize = 64 * 1024 * 1024
	maxMsgSize   = maxTarSize + 1024

	challengeSize = 32
)

type ProxyServerImpl struct {
	UnimplementedProxyServer

	ctx                 context.Context
	client              client.Reader
	controllerNamespace string

	mutex       sync.Mutex
	stopped     bool
	connections map[string]*ProxyConnection

	listener   net.Listener
	grpcServer *grpc.Server
}

type ProxyConnection struct {
	server *ProxyServerImpl

	pubKeyHash string
	stream     Proxy_ProxyStreamServer

	requestsCh chan *wrappedRequest
	stopCh     chan struct{}
	doneCh     chan struct{}
}

type wrappedRequest struct {
	req    *ResolveOverrideRequest
	respCh chan *ResolveOverrideResponse
	errCh  chan error
}

func NewProxyServerImpl(ctx context.Context, c client.Reader, controllerNamespace string) *ProxyServerImpl {
	m := &ProxyServerImpl{
		ctx:                 ctx,
		client:              c,
		controllerNamespace: controllerNamespace,
		connections:         map[string]*ProxyConnection{},
	}
	return m
}

func (m *ProxyServerImpl) Listen(addr string) (net.Addr, error) {
	is, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	m.listener = is
	return is.Addr(), nil
}

func (m *ProxyServerImpl) Serve() error {
	cert, err := WaitAndLoadTLSCert(m.ctx, m.client, m.controllerNamespace)
	if err != nil {
		return err
	}
	creds := credentials.NewServerTLSFromCert(cert)

	var grpcServer *grpc.Server

	m.mutex.Lock()
	if m.stopped {
		m.mutex.Unlock()
		return fmt.Errorf("stopped while starting")
	}
	grpcServer = grpc.NewServer(
		grpc.Creds(creds),
		grpc.MaxRecvMsgSize(maxMsgSize),
	)
	RegisterProxyServer(grpcServer, m)
	m.grpcServer = grpcServer
	m.mutex.Unlock()

	return grpcServer.Serve(m.listener)
}

func (m *ProxyServerImpl) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.stopped {
		return
	}
	m.stopped = true
	if m.grpcServer != nil {
		m.grpcServer.Stop()
	}
}

func (s *ProxyServerImpl) ProxyStream(stream Proxy_ProxyStreamServer) error {
	con, err := s.handshake(stream)
	if err != nil {
		return err
	}

	defer func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		delete(s.connections, con.pubKeyHash)
	}()

	return con.proxyRequests()
}

func (s *ProxyServerImpl) handshake(stream Proxy_ProxyStreamServer) (*ProxyConnection, error) {
	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	if len(resp.Auth.PubKey) == 0 {
		return nil, fmt.Errorf("expected auth")
	}

	pubKeyAny, err := x509.ParsePKIXPublicKey(resp.Auth.PubKey)
	if err != nil {
		return nil, err
	}
	pubKey, ok := pubKeyAny.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	h := sha256.New()
	h.Write(resp.Auth.PubKey)
	pubKeyHash := h.Sum(nil)

	challenge := make([]byte, challengeSize)
	_, err = rand.Read(challenge)
	if err != nil {
		return nil, err
	}

	msg := &ProxyRequest{
		Auth: &AuthMsg{
			Challenge: challenge,
		},
	}

	err = stream.Send(msg)
	if err != nil {
		return nil, err
	}

	resp, err = stream.Recv()
	if err != nil {
		return nil, err
	}

	if len(resp.Auth.Pop) == 0 {
		return nil, fmt.Errorf("expected pop")
	}

	h = sha256.New()
	h.Write(pubKeyHash)
	h.Write(challenge)

	if !ecdsa.VerifyASN1(pubKey, h.Sum(nil), resp.Auth.Pop) {
		return nil, fmt.Errorf("proof of possession failed")
	}

	msg = &ProxyRequest{
		Auth: &AuthMsg{
			AuthError: utils.StrPtr(""),
		},
	}

	con := &ProxyConnection{
		server:     s,
		pubKeyHash: hex.EncodeToString(pubKeyHash),
		stream:     stream,
		requestsCh: make(chan *wrappedRequest),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	s.mutex.Lock()
	if _, ok := s.connections[con.pubKeyHash]; ok {
		s.mutex.Unlock()
		return nil, fmt.Errorf("duplicate server id")
	}
	s.connections[con.pubKeyHash] = con
	s.mutex.Unlock()

	err = stream.Send(msg)
	if err != nil {
		return nil, err
	}

	return con, nil
}

func (s *ProxyServerImpl) ResolveOverride(ctx context.Context, req *ProxyRequest) (*ResolveOverrideResponse, error) {
	if req.Auth.PubKeyHash == nil {
		return nil, fmt.Errorf("missing pubKeyHash")
	}
	if req.Request == nil {
		return nil, fmt.Errorf("missing request")
	}

	s.mutex.Lock()
	con, ok := s.connections[*req.Auth.PubKeyHash]
	s.mutex.Unlock()
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}

	wreq := &wrappedRequest{
		req:    req.Request,
		respCh: make(chan *ResolveOverrideResponse),
		errCh:  make(chan error),
	}
	con.requestsCh <- wreq

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-wreq.errCh:
		return nil, err
	case resp := <-wreq.respCh:
		return resp, nil
	}
}

func (c *ProxyConnection) Close() {
	close(c.stopCh)
	<-c.doneCh
}

func (c *ProxyConnection) proxyRequests() error {
	defer func() {
		close(c.doneCh)
	}()
	for {
		select {
		case <-c.server.ctx.Done():
			return c.server.ctx.Err()
		case <-c.stopCh:
			return nil
		case req, ok := <-c.requestsCh:
			if !ok {
				return nil
			}
			err := c.proxyRequest(req)
			if err != nil {
				return err
			}
		}
	}
}

func (c *ProxyConnection) proxyRequest(req *wrappedRequest) error {
	err := c.stream.Send(&ProxyRequest{
		Auth: &AuthMsg{
			PubKeyHash: &c.pubKeyHash,
		},
		Request: req.req,
	})
	if err != nil {
		req.errCh <- err
		return err
	}

	msg, err := c.stream.Recv()
	if err != nil {
		req.errCh <- err
		return err
	}
	if msg.Response == nil {
		err = fmt.Errorf("missing response")
		req.errCh <- err
		return err
	}

	req.respCh <- msg.Response

	return nil
}
