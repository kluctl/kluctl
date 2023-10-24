package sourceoverride

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"sync"
)

const (
	maxTarSize   = 32 * 1024 * 1024
	maxUntarSize = 64 * 1024 * 1024
	maxMsgSize   = maxTarSize + 1024
)

type ProxyServerImpl struct {
	UnimplementedProxyServer

	ctx context.Context

	mutex       sync.Mutex
	connections map[string]*ProxyConnection
	grpcServer  *grpc.Server
}

type ProxyConnection struct {
	server *ProxyServerImpl

	id     string
	stream Proxy_ProxyStreamServer

	requestsCh chan *wrappedRequest
	stopCh     chan struct{}
	doneCh     chan struct{}
}

type wrappedRequest struct {
	req    *ResolveOverrideRequest
	respCh chan *ResolveOverrideResponse
	errCh  chan error
}

func NewProxyServerImpl(ctx context.Context) *ProxyServerImpl {
	m := &ProxyServerImpl{
		ctx:         ctx,
		connections: map[string]*ProxyConnection{},
	}
	m.grpcServer = grpc.NewServer(grpc.MaxRecvMsgSize(maxMsgSize))
	RegisterProxyServer(m.grpcServer, m)
	return m
}

func (m *ProxyServerImpl) Start(addr string) error {
	is, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	err = m.grpcServer.Serve(is)
	if err != nil {
		return err
	}

	return nil
}

func (m *ProxyServerImpl) Stop() {
	m.grpcServer.Stop()
}

func (s *ProxyServerImpl) ProxyStream(stream Proxy_ProxyStreamServer) error {
	con, err := s.handshake(stream)
	if err != nil {
		return err
	}

	defer func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		delete(s.connections, con.id)
	}()

	return con.proxyRequests()
}

func (s *ProxyServerImpl) handshake(stream Proxy_ProxyStreamServer) (*ProxyConnection, error) {
	msg, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	if msg.Response != nil || msg.Auth == nil {
		return nil, fmt.Errorf("expected auth")
	}

	doResp := func(err error) error {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		err2 := stream.Send(&ProxyRequest{
			AuthError: &errStr,
			ServerId:  msg.Auth.ServerId,
		})
		if err2 != nil {
			return err2
		}
		return nil
	}

	con := &ProxyConnection{
		server:     s,
		id:         msg.Auth.ServerId,
		stream:     stream,
		requestsCh: make(chan *wrappedRequest),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	s.mutex.Lock()
	if _, ok := s.connections[msg.Auth.ServerId]; ok {
		s.mutex.Unlock()
		err = fmt.Errorf("duplicate server id")
		_ = doResp(err)
		return nil, err
	}
	s.connections[msg.Auth.ServerId] = con
	s.mutex.Unlock()

	return con, doResp(nil)
}

func (s *ProxyServerImpl) ResolveOverride(ctx context.Context, req *ProxyRequest) (*ResolveOverrideResponse, error) {
	if req.Request == nil {
		return nil, fmt.Errorf("missing request")
	}

	s.mutex.Lock()
	con, ok := s.connections[req.ServerId]
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
		ServerId: c.id,
		Request:  req.req,
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
