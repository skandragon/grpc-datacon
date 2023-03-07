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
	"log"
	"net"
	"sync"
	"time"

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second,
	PermitWithoutStream: true,
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle: 20 * time.Minute,
	Time:              10 * time.Second,
	Timeout:           10 * time.Second,
}

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
	t := time.NewTicker(60 * time.Second)
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
	case <-t.C:
		return &pb.TunnelRequest{
			StreamId: ulid.GlobalContext.Ulid(),
			Name:     "bob",
			Type:     "smith",
			Method:   "GET",
			URI:      "/foobar",
		}, nil
	}
}

func main() {
	ctx := context.Background()

	lis, err := net.Listen("tcp", ":50051")
	check(err)

	idleTimeout := 60 * time.Second

	sconfig := &server{
		agentIdleTimeout: idleTimeout.Nanoseconds(),
		agents:           map[agentKey]*agentContext{},
	}

	interceptor := NewJWTInterceptor()
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	)
	pb.RegisterTunnelServiceServer(s, sconfig)

	//debugCtx, debugCancel := context.WithCancel(ctx)
	//defer debugCancel()
	//go sconfig.debugLogger(debugCtx)

	cleanerCtx, cleanerCancel := context.WithCancel(ctx)
	defer cleanerCancel()
	go sconfig.checkSessionTimeouts(cleanerCtx)

	log.Printf("Listening for connections on TCP port 50051")
	check(s.Serve(lis))
}
