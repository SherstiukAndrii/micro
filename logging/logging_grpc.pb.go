// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package logging

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

// LoggingServiceClient is the client API for LoggingService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type LoggingServiceClient interface {
	SaveMessage(ctx context.Context, in *SaveMessageRequest, opts ...grpc.CallOption) (*SaveMessageResponse, error)
	GetMessages(ctx context.Context, in *GetMessagesRequest, opts ...grpc.CallOption) (*GetMessagesResponse, error)
}

type loggingServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewLoggingServiceClient(cc grpc.ClientConnInterface) LoggingServiceClient {
	return &loggingServiceClient{cc}
}

func (c *loggingServiceClient) SaveMessage(ctx context.Context, in *SaveMessageRequest, opts ...grpc.CallOption) (*SaveMessageResponse, error) {
	out := new(SaveMessageResponse)
	err := c.cc.Invoke(ctx, "/logging.LoggingService/SaveMessage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *loggingServiceClient) GetMessages(ctx context.Context, in *GetMessagesRequest, opts ...grpc.CallOption) (*GetMessagesResponse, error) {
	out := new(GetMessagesResponse)
	err := c.cc.Invoke(ctx, "/logging.LoggingService/GetMessages", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LoggingServiceServer is the server API for LoggingService service.
// All implementations must embed UnimplementedLoggingServiceServer
// for forward compatibility
type LoggingServiceServer interface {
	SaveMessage(context.Context, *SaveMessageRequest) (*SaveMessageResponse, error)
	GetMessages(context.Context, *GetMessagesRequest) (*GetMessagesResponse, error)
	mustEmbedUnimplementedLoggingServiceServer()
}

// UnimplementedLoggingServiceServer must be embedded to have forward compatible implementations.
type UnimplementedLoggingServiceServer struct {
}

func (UnimplementedLoggingServiceServer) SaveMessage(context.Context, *SaveMessageRequest) (*SaveMessageResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SaveMessage not implemented")
}
func (UnimplementedLoggingServiceServer) GetMessages(context.Context, *GetMessagesRequest) (*GetMessagesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMessages not implemented")
}
func (UnimplementedLoggingServiceServer) mustEmbedUnimplementedLoggingServiceServer() {}

// UnsafeLoggingServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to LoggingServiceServer will
// result in compilation errors.
type UnsafeLoggingServiceServer interface {
	mustEmbedUnimplementedLoggingServiceServer()
}

func RegisterLoggingServiceServer(s grpc.ServiceRegistrar, srv LoggingServiceServer) {
	s.RegisterService(&LoggingService_ServiceDesc, srv)
}

func _LoggingService_SaveMessage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SaveMessageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LoggingServiceServer).SaveMessage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logging.LoggingService/SaveMessage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LoggingServiceServer).SaveMessage(ctx, req.(*SaveMessageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LoggingService_GetMessages_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetMessagesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LoggingServiceServer).GetMessages(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/logging.LoggingService/GetMessages",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LoggingServiceServer).GetMessages(ctx, req.(*GetMessagesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// LoggingService_ServiceDesc is the grpc.ServiceDesc for LoggingService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var LoggingService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "logging.LoggingService",
	HandlerType: (*LoggingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SaveMessage",
			Handler:    _LoggingService_SaveMessage_Handler,
		},
		{
			MethodName: "GetMessages",
			Handler:    _LoggingService_GetMessages_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "logging/logging.proto",
}
