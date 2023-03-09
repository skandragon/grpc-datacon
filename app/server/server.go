/*
 * Copyright 2023 OpsMx, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License")
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedTunnelServiceServer
	sync.Mutex
	agentIdleTimeout int64
	agents           map[agentKey]*agentContext
}

func (s *server) Hello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloResponse, error) {
	agentID, _ := IdentityFromContext(ctx)
	_, logger := loggerFromContext(ctx)
	logger.Infof("Hello", "endpoints", in.Endpoints, "annotations", in.Annotations)
	session := s.registerAgentSession(agentID, ulid.GlobalContext.Ulid())
	return &pb.HelloResponse{
		InstanceId: session.sessionID,
		AgentId:    agentID,
	}, nil
}

func (s *server) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	_, logger := loggerFromContext(ctx)
	session, err := s.findAgentSessionContext(ctx)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Hello must be called first")
	}
	now := time.Now().UnixNano()
	s.touchSession(session, now)
	logger.Infof("Ping")
	r := &pb.PingResponse{
		Ts:       uint64(now),
		EchoedTs: in.Ts,
	}
	return r, nil
}

func (s *server) WaitForRequest(in *pb.WaitForRequestArgs, stream pb.TunnelService_WaitForRequestServer) error {
	ctx, logger := loggerFromContext(stream.Context())
	logger.Infof("WaitForRequest")
	session, err := s.findAgentSessionContext(stream.Context())
	if err != nil {
		return status.Error(codes.FailedPrecondition, "Hello must be called first")
	}
	defer s.removeAgentSession(session)

	for {
		select {
		case <-ctx.Done():
			logger.Infow("closed connection")
			return status.Error(codes.Canceled, "client closed connection")
		case r := <-session.out:
			logger.Infow("->TunnelRequest",
				"streamID", r.StreamId,
				"method", r.Method,
				"serviceName", r.Name,
				"serviceType", r.Type,
				"uri", r.URI,
				"bodyLength", len(r.Body))
			if err := stream.Send(r); err != nil {
				logger.Errorw("WaitForRequest stream.Send() failed, dropping agent", "error", err)
				return status.Error(codes.Canceled, "send failed")
			}
		}
	}
}

func (s *server) SendHeaders(ctx context.Context, in *pb.TunnelHeaders) (*pb.SendHeadersResponse, error) {
	ctx, logger := loggerFromContext(ctx, zap.String("streamID", in.StreamId))
	_, err := s.findAgentSessionContext(ctx)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Hello must be called first")
	}
	logger.Infow("SendHeaders",
		"contentLength", in.ContentLength,
		"headersLength", len(in.Headers),
		"statusCode", in.Status)
	return &pb.SendHeadersResponse{}, nil
}

func (s *server) SendData(stream pb.TunnelService_SendDataServer) error {
	var logger *zap.SugaredLogger
	for {
		data, err := stream.Recv()
		if err != nil {
			_, logger = loggerFromContext(stream.Context(), zap.String("streamID", "--UNKNOWN--"))
			logger.Warnw("SendData", "error", err)
			return err
		}
		if logger == nil {
			_, logger = loggerFromContext(stream.Context(), zap.String("streamID", data.StreamId))
		}
		if len(data.Data) == 0 {
			logger.Infow("SendData stream ended with length 0 (EOF)")
			return nil
		}
		logger.Infow("SendData", "dataLength", len(data.Data))
	}
}

func runAgentGRPCServer(ctx context.Context, useTLS bool, serverCert *tls.Certificate) {
	ctx, logger := loggerFromContext(ctx, zap.String("component", "grpcServer"))
	logger.Infow("starting agent GRPC server", "port", config.AgentListenPort)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.AgentListenPort))
	if err != nil {
		logger.Fatalw("failed to listen on agent port", "error", err)
	}

	idleTimeout := 60 * time.Second

	sconfig := &server{
		agentIdleTimeout: idleTimeout.Nanoseconds(),
		agents:           map[agentKey]*agentContext{},
	}

	cleanerCtx, cleanerCancel := context.WithCancel(ctx)
	defer cleanerCancel()
	go sconfig.checkSessionTimeouts(cleanerCtx)

	requesterCtx, requesterCancel := context.WithCancel(ctx)
	defer requesterCancel()
	go sconfig.requestOnTimer(requesterCtx)

	certPool, err := authority.MakeCertPool()
	if err != nil {
		logger.Fatalw("authority.MakeCertPool", "error", err)
	}
	creds := credentials.NewTLS(&tls.Config{
		ClientCAs:    certPool,
		ClientAuth:   tls.NoClientCert,
		Certificates: []tls.Certificate{*serverCert},
		MinVersion:   tls.VersionTLS13,
	})
	interceptor := NewJWTInterceptor()
	opts := []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterTunnelServiceServer(grpcServer, sconfig)
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatalw("grpcServer.Serve() failed", "error", err)
	}
}
