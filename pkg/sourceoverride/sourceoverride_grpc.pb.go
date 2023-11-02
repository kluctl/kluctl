// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.24.4
// source: sourceoverride.proto

package sourceoverride

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Proxy_ProxyStream_FullMethodName     = "/sourceoverride.Proxy/ProxyStream"
	Proxy_ResolveOverride_FullMethodName = "/sourceoverride.Proxy/ResolveOverride"
)

// ProxyClient is the client API for Proxy service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProxyClient interface {
	ProxyStream(ctx context.Context, opts ...grpc.CallOption) (Proxy_ProxyStreamClient, error)
	ResolveOverride(ctx context.Context, in *ProxyRequest, opts ...grpc.CallOption) (*ResolveOverrideResponse, error)
}

type proxyClient struct {
	cc grpc.ClientConnInterface
}

func NewProxyClient(cc grpc.ClientConnInterface) ProxyClient {
	return &proxyClient{cc}
}

func (c *proxyClient) ProxyStream(ctx context.Context, opts ...grpc.CallOption) (Proxy_ProxyStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &Proxy_ServiceDesc.Streams[0], Proxy_ProxyStream_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &proxyProxyStreamClient{stream}
	return x, nil
}

type Proxy_ProxyStreamClient interface {
	Send(*ProxyResponse) error
	Recv() (*ProxyRequest, error)
	grpc.ClientStream
}

type proxyProxyStreamClient struct {
	grpc.ClientStream
}

func (x *proxyProxyStreamClient) Send(m *ProxyResponse) error {
	return x.ClientStream.SendMsg(m)
}

func (x *proxyProxyStreamClient) Recv() (*ProxyRequest, error) {
	m := new(ProxyRequest)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *proxyClient) ResolveOverride(ctx context.Context, in *ProxyRequest, opts ...grpc.CallOption) (*ResolveOverrideResponse, error) {
	out := new(ResolveOverrideResponse)
	err := c.cc.Invoke(ctx, Proxy_ResolveOverride_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProxyServer is the server API for Proxy service.
// All implementations must embed UnimplementedProxyServer
// for forward compatibility
type ProxyServer interface {
	ProxyStream(Proxy_ProxyStreamServer) error
	ResolveOverride(context.Context, *ProxyRequest) (*ResolveOverrideResponse, error)
	mustEmbedUnimplementedProxyServer()
}

// UnimplementedProxyServer must be embedded to have forward compatible implementations.
type UnimplementedProxyServer struct {
}

func (UnimplementedProxyServer) ProxyStream(Proxy_ProxyStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method ProxyStream not implemented")
}
func (UnimplementedProxyServer) ResolveOverride(context.Context, *ProxyRequest) (*ResolveOverrideResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ResolveOverride not implemented")
}
func (UnimplementedProxyServer) mustEmbedUnimplementedProxyServer() {}

// UnsafeProxyServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProxyServer will
// result in compilation errors.
type UnsafeProxyServer interface {
	mustEmbedUnimplementedProxyServer()
}

func RegisterProxyServer(s grpc.ServiceRegistrar, srv ProxyServer) {
	s.RegisterService(&Proxy_ServiceDesc, srv)
}

func _Proxy_ProxyStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ProxyServer).ProxyStream(&proxyProxyStreamServer{stream})
}

type Proxy_ProxyStreamServer interface {
	Send(*ProxyRequest) error
	Recv() (*ProxyResponse, error)
	grpc.ServerStream
}

type proxyProxyStreamServer struct {
	grpc.ServerStream
}

func (x *proxyProxyStreamServer) Send(m *ProxyRequest) error {
	return x.ServerStream.SendMsg(m)
}

func (x *proxyProxyStreamServer) Recv() (*ProxyResponse, error) {
	m := new(ProxyResponse)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Proxy_ResolveOverride_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProxyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProxyServer).ResolveOverride(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Proxy_ResolveOverride_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProxyServer).ResolveOverride(ctx, req.(*ProxyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Proxy_ServiceDesc is the grpc.ServiceDesc for Proxy service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Proxy_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "sourceoverride.Proxy",
	HandlerType: (*ProxyServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ResolveOverride",
			Handler:    _Proxy_ResolveOverride_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ProxyStream",
			Handler:       _Proxy_ProxyStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "sourceoverride.proto",
}