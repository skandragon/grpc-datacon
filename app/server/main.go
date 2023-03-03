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
	"time"

	"github.com/skandragon/grpc-datacon/internal/logging"
	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"go.uber.org/zap"
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

type agentContext struct {
	in  chan string
	out chan string
}

func newAgentContext() agentContext {
	return agentContext{
		in:  make(chan string),
		out: make(chan string),
	}
}

type server struct {
	pb.UnimplementedTunnelServiceServer
	agents map[string]agentContext
}

func loggerFromContext(ctx context.Context) (context.Context, *zap.SugaredLogger) {
	fields := []zap.Field{}
	agentID, sessionID := IdentityFromContext(ctx)
	if agentID != "" {
		fields = append(fields, zap.String("agentID", agentID))
	}
	if sessionID != "" {
		fields = append(fields, zap.String("sessionID", sessionID))
	}
	ctx = logging.NewContext(ctx, fields...)
	return ctx, logging.WithContext(ctx).Sugar()
}

func (s *server) Hello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloResponse, error) {
	//sessionID, agentID := identityFromContext(ctx)
	_, logger := loggerFromContext(ctx)
	logger.Infof("Hello")
	return &pb.HelloResponse{
		InstanceId: ulid.GlobalContext.Ulid(),
	}, nil
}

func (s *server) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PingResponse, error) {
	//sessionID, agentID := identityFromContext(ctx)
	_, logger := loggerFromContext(ctx)
	logger.Infof("Ping")
	r := &pb.PingResponse{
		Ts:       uint64(time.Now().UnixNano()),
		EchoedTs: in.Ts,
	}
	return r, nil
}

func (s *server) WaitForRequest(ctx context.Context, in *pb.WaitForRequestArgs) (*pb.TunnelRequest, error) {
	t := time.NewTicker(60 * time.Second)
	//sessionID, agentID := identityFromContext(ctx)
	_, logger := loggerFromContext(ctx)
	logger.Infof("Ping")

	select {
	case <-ctx.Done():
		logger.Infow("closed connection")
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
	lis, err := net.Listen("tcp", ":50051")
	check(err)

	interceptor := NewJWTInterceptor()
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	)
	pb.RegisterTunnelServiceServer(s, &server{})
	log.Printf("Listening for connections on TCP port 50051")
	check(s.Serve(lis))
}
