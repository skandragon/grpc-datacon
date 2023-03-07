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
	"sync"
	"time"

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	sync.Mutex
	agentIdleTimeout int64
	pb.UnimplementedTunnelServiceServer
	agents map[agentKey]*agentContext
}

func (s *server) Hello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloResponse, error) {
	agentID, _ := IdentityFromContext(ctx)
	_, logger := loggerFromContext(ctx)
	logger.Infof("Hello")
	session := s.registerAgentSession(agentID, ulid.GlobalContext.Ulid())
	return &pb.HelloResponse{
		InstanceId: session.sessionID,
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

func (s *server) WaitForRequest(ctx context.Context, in *pb.WaitForRequestArgs) (*pb.TunnelRequest, error) {
	session, err := s.findAgentSessionContext(ctx)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Hello must be called first")
	}
	ctx, logger := loggerFromContext(ctx)
	logger.Infof("WaitForRequest")

	select {
	case <-ctx.Done():
		logger.Infow("closed connection")
		s.removeAgentSession(session)
		return nil, status.Error(codes.Canceled, "client closed connection")
	case r := <-session.out:
		logger.Infof("request: %v", r)
		return r, nil
	}
}

func (s *server) SendHeaders(ctx context.Context, in *pb.TunnelHeaders) (*pb.SendHeadersResponse, error) {
	ctx, logger := loggerFromContext(ctx, zap.String("streamID", in.StreamId))
	_, err := s.findAgentSessionContext(ctx)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Hello must be called first")
	}
	logger.Infow("SendHeaders", "contentLength", in.ContentLength, "headersLength", len(in.Headers))
	return &pb.SendHeadersResponse{}, nil
}
