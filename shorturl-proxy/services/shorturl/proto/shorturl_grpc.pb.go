// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.29.1
// source: shorturl/proto/shorturl.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	ShortUrl_GetShortUrl_FullMethodName    = "/shorturl.chenaws.com.ShortUrl/GetShortUrl"
	ShortUrl_GetOriginalUrl_FullMethodName = "/shorturl.chenaws.com.ShortUrl/GetOriginalUrl"
)

// ShortUrlClient is the client API for ShortUrl service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ShortUrlClient interface {
	GetShortUrl(ctx context.Context, in *Url, opts ...grpc.CallOption) (*Url, error)
	GetOriginalUrl(ctx context.Context, in *ShortKey, opts ...grpc.CallOption) (*Url, error)
}

type shortUrlClient struct {
	cc grpc.ClientConnInterface
}

func NewShortUrlClient(cc grpc.ClientConnInterface) ShortUrlClient {
	return &shortUrlClient{cc}
}

func (c *shortUrlClient) GetShortUrl(ctx context.Context, in *Url, opts ...grpc.CallOption) (*Url, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Url)
	err := c.cc.Invoke(ctx, ShortUrl_GetShortUrl_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *shortUrlClient) GetOriginalUrl(ctx context.Context, in *ShortKey, opts ...grpc.CallOption) (*Url, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(Url)
	err := c.cc.Invoke(ctx, ShortUrl_GetOriginalUrl_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ShortUrlServer is the server API for ShortUrl service.
// All implementations must embed UnimplementedShortUrlServer
// for forward compatibility.
type ShortUrlServer interface {
	GetShortUrl(context.Context, *Url) (*Url, error)
	GetOriginalUrl(context.Context, *ShortKey) (*Url, error)
	mustEmbedUnimplementedShortUrlServer()
}

// UnimplementedShortUrlServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedShortUrlServer struct{}

func (UnimplementedShortUrlServer) GetShortUrl(context.Context, *Url) (*Url, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetShortUrl not implemented")
}
func (UnimplementedShortUrlServer) GetOriginalUrl(context.Context, *ShortKey) (*Url, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetOriginalUrl not implemented")
}
func (UnimplementedShortUrlServer) mustEmbedUnimplementedShortUrlServer() {}
func (UnimplementedShortUrlServer) testEmbeddedByValue()                  {}

// UnsafeShortUrlServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ShortUrlServer will
// result in compilation errors.
type UnsafeShortUrlServer interface {
	mustEmbedUnimplementedShortUrlServer()
}

func RegisterShortUrlServer(s grpc.ServiceRegistrar, srv ShortUrlServer) {
	// If the following call pancis, it indicates UnimplementedShortUrlServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ShortUrl_ServiceDesc, srv)
}

func _ShortUrl_GetShortUrl_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Url)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShortUrlServer).GetShortUrl(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ShortUrl_GetShortUrl_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShortUrlServer).GetShortUrl(ctx, req.(*Url))
	}
	return interceptor(ctx, in, info, handler)
}

func _ShortUrl_GetOriginalUrl_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ShortKey)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ShortUrlServer).GetOriginalUrl(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ShortUrl_GetOriginalUrl_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ShortUrlServer).GetOriginalUrl(ctx, req.(*ShortKey))
	}
	return interceptor(ctx, in, info, handler)
}

// ShortUrl_ServiceDesc is the grpc.ServiceDesc for ShortUrl service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ShortUrl_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "shorturl.chenaws.com.ShortUrl",
	HandlerType: (*ShortUrlServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetShortUrl",
			Handler:    _ShortUrl_GetShortUrl_Handler,
		},
		{
			MethodName: "GetOriginalUrl",
			Handler:    _ShortUrl_GetOriginalUrl_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "shorturl/proto/shorturl.proto",
}
