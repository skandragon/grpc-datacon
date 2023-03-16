//
// Copyright 2021-2023 OpsMx, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License")
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.21.12
// source: internal/tunnel/tunnel.proto

package tunnel

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
	TunnelService_Hello_FullMethodName          = "/tunnel.TunnelService/Hello"
	TunnelService_Ping_FullMethodName           = "/tunnel.TunnelService/Ping"
	TunnelService_StartTunnel_FullMethodName    = "/tunnel.TunnelService/StartTunnel"
	TunnelService_WaitForRequest_FullMethodName = "/tunnel.TunnelService/WaitForRequest"
	TunnelService_SendData_FullMethodName       = "/tunnel.TunnelService/SendData"
	TunnelService_SendHeaders_FullMethodName    = "/tunnel.TunnelService/SendHeaders"
	TunnelService_ReceiveData_FullMethodName    = "/tunnel.TunnelService/ReceiveData"
	TunnelService_CancelStream_FullMethodName   = "/tunnel.TunnelService/CancelStream"
)

// TunnelServiceClient is the client API for TunnelService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TunnelServiceClient interface {
	// Initial handshake, sent from agent to controller on the control connection, before any other messages.
	Hello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*HelloResponse, error)
	// Keep alive, sent from the agent to the controller, on control and data connections.
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	// The agent will call StartRequest() when it wants the controller to perform
	// an HTTP fetch.
	StartTunnel(ctx context.Context, in *TunnelRequest, opts ...grpc.CallOption) (*TunnelHeaders, error)
	// The agent will perform a long-running call to WaitForRequest() and handle
	// any HTTP request found.
	WaitForRequest(ctx context.Context, in *WaitForRequestArgs, opts ...grpc.CallOption) (TunnelService_WaitForRequestClient, error)
	// HTTP data, issued from the agent to controller.
	//
	// The Send* methods are when the agent is performing the HTTP request, and
	// wants to send the headers and data response to the controller, which will
	// send it to the client.
	//
	// The Receive* methods are when the controller is performing the HTTP
	// request, and the agent has the client which wants the data.
	SendData(ctx context.Context, opts ...grpc.CallOption) (TunnelService_SendDataClient, error)
	SendHeaders(ctx context.Context, in *TunnelHeaders, opts ...grpc.CallOption) (*SendHeadersResponse, error)
	ReceiveData(ctx context.Context, in *ReceiveDataRequest, opts ...grpc.CallOption) (TunnelService_ReceiveDataClient, error)
	CancelStream(ctx context.Context, in *CancelStreamRequest, opts ...grpc.CallOption) (*CancelStreamResponse, error)
}

type tunnelServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTunnelServiceClient(cc grpc.ClientConnInterface) TunnelServiceClient {
	return &tunnelServiceClient{cc}
}

func (c *tunnelServiceClient) Hello(ctx context.Context, in *HelloRequest, opts ...grpc.CallOption) (*HelloResponse, error) {
	out := new(HelloResponse)
	err := c.cc.Invoke(ctx, TunnelService_Hello_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tunnelServiceClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := c.cc.Invoke(ctx, TunnelService_Ping_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tunnelServiceClient) StartTunnel(ctx context.Context, in *TunnelRequest, opts ...grpc.CallOption) (*TunnelHeaders, error) {
	out := new(TunnelHeaders)
	err := c.cc.Invoke(ctx, TunnelService_StartTunnel_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tunnelServiceClient) WaitForRequest(ctx context.Context, in *WaitForRequestArgs, opts ...grpc.CallOption) (TunnelService_WaitForRequestClient, error) {
	stream, err := c.cc.NewStream(ctx, &TunnelService_ServiceDesc.Streams[0], TunnelService_WaitForRequest_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &tunnelServiceWaitForRequestClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TunnelService_WaitForRequestClient interface {
	Recv() (*TunnelRequest, error)
	grpc.ClientStream
}

type tunnelServiceWaitForRequestClient struct {
	grpc.ClientStream
}

func (x *tunnelServiceWaitForRequestClient) Recv() (*TunnelRequest, error) {
	m := new(TunnelRequest)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *tunnelServiceClient) SendData(ctx context.Context, opts ...grpc.CallOption) (TunnelService_SendDataClient, error) {
	stream, err := c.cc.NewStream(ctx, &TunnelService_ServiceDesc.Streams[1], TunnelService_SendData_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &tunnelServiceSendDataClient{stream}
	return x, nil
}

type TunnelService_SendDataClient interface {
	Send(*Data) error
	CloseAndRecv() (*SendDataResponse, error)
	grpc.ClientStream
}

type tunnelServiceSendDataClient struct {
	grpc.ClientStream
}

func (x *tunnelServiceSendDataClient) Send(m *Data) error {
	return x.ClientStream.SendMsg(m)
}

func (x *tunnelServiceSendDataClient) CloseAndRecv() (*SendDataResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(SendDataResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *tunnelServiceClient) SendHeaders(ctx context.Context, in *TunnelHeaders, opts ...grpc.CallOption) (*SendHeadersResponse, error) {
	out := new(SendHeadersResponse)
	err := c.cc.Invoke(ctx, TunnelService_SendHeaders_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tunnelServiceClient) ReceiveData(ctx context.Context, in *ReceiveDataRequest, opts ...grpc.CallOption) (TunnelService_ReceiveDataClient, error) {
	stream, err := c.cc.NewStream(ctx, &TunnelService_ServiceDesc.Streams[2], TunnelService_ReceiveData_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &tunnelServiceReceiveDataClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type TunnelService_ReceiveDataClient interface {
	Recv() (*Data, error)
	grpc.ClientStream
}

type tunnelServiceReceiveDataClient struct {
	grpc.ClientStream
}

func (x *tunnelServiceReceiveDataClient) Recv() (*Data, error) {
	m := new(Data)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *tunnelServiceClient) CancelStream(ctx context.Context, in *CancelStreamRequest, opts ...grpc.CallOption) (*CancelStreamResponse, error) {
	out := new(CancelStreamResponse)
	err := c.cc.Invoke(ctx, TunnelService_CancelStream_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TunnelServiceServer is the server API for TunnelService service.
// All implementations must embed UnimplementedTunnelServiceServer
// for forward compatibility
type TunnelServiceServer interface {
	// Initial handshake, sent from agent to controller on the control connection, before any other messages.
	Hello(context.Context, *HelloRequest) (*HelloResponse, error)
	// Keep alive, sent from the agent to the controller, on control and data connections.
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	// The agent will call StartRequest() when it wants the controller to perform
	// an HTTP fetch.
	StartTunnel(context.Context, *TunnelRequest) (*TunnelHeaders, error)
	// The agent will perform a long-running call to WaitForRequest() and handle
	// any HTTP request found.
	WaitForRequest(*WaitForRequestArgs, TunnelService_WaitForRequestServer) error
	// HTTP data, issued from the agent to controller.
	//
	// The Send* methods are when the agent is performing the HTTP request, and
	// wants to send the headers and data response to the controller, which will
	// send it to the client.
	//
	// The Receive* methods are when the controller is performing the HTTP
	// request, and the agent has the client which wants the data.
	SendData(TunnelService_SendDataServer) error
	SendHeaders(context.Context, *TunnelHeaders) (*SendHeadersResponse, error)
	ReceiveData(*ReceiveDataRequest, TunnelService_ReceiveDataServer) error
	CancelStream(context.Context, *CancelStreamRequest) (*CancelStreamResponse, error)
	mustEmbedUnimplementedTunnelServiceServer()
}

// UnimplementedTunnelServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTunnelServiceServer struct {
}

func (UnimplementedTunnelServiceServer) Hello(context.Context, *HelloRequest) (*HelloResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Hello not implemented")
}
func (UnimplementedTunnelServiceServer) Ping(context.Context, *PingRequest) (*PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
func (UnimplementedTunnelServiceServer) StartTunnel(context.Context, *TunnelRequest) (*TunnelHeaders, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StartTunnel not implemented")
}
func (UnimplementedTunnelServiceServer) WaitForRequest(*WaitForRequestArgs, TunnelService_WaitForRequestServer) error {
	return status.Errorf(codes.Unimplemented, "method WaitForRequest not implemented")
}
func (UnimplementedTunnelServiceServer) SendData(TunnelService_SendDataServer) error {
	return status.Errorf(codes.Unimplemented, "method SendData not implemented")
}
func (UnimplementedTunnelServiceServer) SendHeaders(context.Context, *TunnelHeaders) (*SendHeadersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendHeaders not implemented")
}
func (UnimplementedTunnelServiceServer) ReceiveData(*ReceiveDataRequest, TunnelService_ReceiveDataServer) error {
	return status.Errorf(codes.Unimplemented, "method ReceiveData not implemented")
}
func (UnimplementedTunnelServiceServer) CancelStream(context.Context, *CancelStreamRequest) (*CancelStreamResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CancelStream not implemented")
}
func (UnimplementedTunnelServiceServer) mustEmbedUnimplementedTunnelServiceServer() {}

// UnsafeTunnelServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TunnelServiceServer will
// result in compilation errors.
type UnsafeTunnelServiceServer interface {
	mustEmbedUnimplementedTunnelServiceServer()
}

func RegisterTunnelServiceServer(s grpc.ServiceRegistrar, srv TunnelServiceServer) {
	s.RegisterService(&TunnelService_ServiceDesc, srv)
}

func _TunnelService_Hello_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelloRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelServiceServer).Hello(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelService_Hello_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelServiceServer).Hello(ctx, req.(*HelloRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TunnelService_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelServiceServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelService_Ping_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelServiceServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TunnelService_StartTunnel_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TunnelRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelServiceServer).StartTunnel(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelService_StartTunnel_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelServiceServer).StartTunnel(ctx, req.(*TunnelRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TunnelService_WaitForRequest_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(WaitForRequestArgs)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TunnelServiceServer).WaitForRequest(m, &tunnelServiceWaitForRequestServer{stream})
}

type TunnelService_WaitForRequestServer interface {
	Send(*TunnelRequest) error
	grpc.ServerStream
}

type tunnelServiceWaitForRequestServer struct {
	grpc.ServerStream
}

func (x *tunnelServiceWaitForRequestServer) Send(m *TunnelRequest) error {
	return x.ServerStream.SendMsg(m)
}

func _TunnelService_SendData_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(TunnelServiceServer).SendData(&tunnelServiceSendDataServer{stream})
}

type TunnelService_SendDataServer interface {
	SendAndClose(*SendDataResponse) error
	Recv() (*Data, error)
	grpc.ServerStream
}

type tunnelServiceSendDataServer struct {
	grpc.ServerStream
}

func (x *tunnelServiceSendDataServer) SendAndClose(m *SendDataResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *tunnelServiceSendDataServer) Recv() (*Data, error) {
	m := new(Data)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _TunnelService_SendHeaders_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TunnelHeaders)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelServiceServer).SendHeaders(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelService_SendHeaders_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelServiceServer).SendHeaders(ctx, req.(*TunnelHeaders))
	}
	return interceptor(ctx, in, info, handler)
}

func _TunnelService_ReceiveData_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ReceiveDataRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(TunnelServiceServer).ReceiveData(m, &tunnelServiceReceiveDataServer{stream})
}

type TunnelService_ReceiveDataServer interface {
	Send(*Data) error
	grpc.ServerStream
}

type tunnelServiceReceiveDataServer struct {
	grpc.ServerStream
}

func (x *tunnelServiceReceiveDataServer) Send(m *Data) error {
	return x.ServerStream.SendMsg(m)
}

func _TunnelService_CancelStream_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CancelStreamRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TunnelServiceServer).CancelStream(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: TunnelService_CancelStream_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TunnelServiceServer).CancelStream(ctx, req.(*CancelStreamRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TunnelService_ServiceDesc is the grpc.ServiceDesc for TunnelService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TunnelService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "tunnel.TunnelService",
	HandlerType: (*TunnelServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Hello",
			Handler:    _TunnelService_Hello_Handler,
		},
		{
			MethodName: "Ping",
			Handler:    _TunnelService_Ping_Handler,
		},
		{
			MethodName: "StartTunnel",
			Handler:    _TunnelService_StartTunnel_Handler,
		},
		{
			MethodName: "SendHeaders",
			Handler:    _TunnelService_SendHeaders_Handler,
		},
		{
			MethodName: "CancelStream",
			Handler:    _TunnelService_CancelStream_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "WaitForRequest",
			Handler:       _TunnelService_WaitForRequest_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SendData",
			Handler:       _TunnelService_SendData_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "ReceiveData",
			Handler:       _TunnelService_ReceiveData_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/tunnel/tunnel.proto",
}
