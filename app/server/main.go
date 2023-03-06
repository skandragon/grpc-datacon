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
	"fmt"
	"log"
	"net"
	"sync"
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
	agentKey
	in       chan string
	out      chan string
	lastUsed int64
}

type agentKey struct {
	agentID   string
	sessionID string
}

func newAgentContext(agentID string, sessionID string) (*agentContext, agentKey) {
	key := agentKey{agentID: agentID, sessionID: sessionID}
	session := &agentContext{
		agentKey: key,
		in:       make(chan string),
		out:      make(chan string),
		lastUsed: time.Now().UnixNano(),
	}
	return session, key
}

type server struct {
	sync.Mutex
	pb.UnimplementedTunnelServiceServer
	agents map[agentKey]*agentContext
}

func (s *server) findAgentSessionContext(ctx context.Context) (*agentContext, error) {
	s.Lock()
	defer s.Unlock()
	agentID, sessionID := IdentityFromContext(ctx)
	key := agentKey{agentID: agentID, sessionID: sessionID}
	if session, found := s.agents[key]; found {
		return session, nil
	}
	return nil, fmt.Errorf("no such agent session connected: %s/%s", agentID, sessionID)
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

func (s *server) registerAgentSession(agentID string, sessionID string) *agentContext {
	s.Lock()
	defer s.Unlock()
	session, key := newAgentContext(agentID, sessionID)
	s.agents[key] = session
	return session
}

func (s *server) debugPrintSessions(ctx context.Context) {
	_, logger := loggerFromContext(ctx)
	s.Lock()
	defer s.Unlock()
	for key := range s.agents {
		logger.Infof("agentID %s, sessionID %s", key.agentID, key.sessionID)
	}
}

func (s *server) removeAgentSession(session *agentContext) {
	s.Lock()
	defer s.Unlock()
	key := agentKey{agentID: session.agentID, sessionID: session.sessionID}
	delete(s.agents, key)
}

func (s *server) touchSession(session *agentContext, t int64) {
	s.Lock()
	defer s.Unlock()
	s.agents[session.agentKey].lastUsed = t
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

func (s *server) debugLogger(ctx context.Context) {
	ctx, logger := loggerFromContext(ctx)
	t := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			logger.Infof("sessions:")
			s.debugPrintSessions(ctx)
		}
	}
}

func (s *server) checkSessionTimeouts(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now().UnixNano()
			s.expireOldSessions(ctx, now)
		}
	}
}

func (s *server) expireOldSessions(ctx context.Context, now int64) {
	s.Lock()
	defer s.Unlock()

	// TODO: check times
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	check(err)

	sconfig := &server{
		agents: map[agentKey]*agentContext{},
	}

	interceptor := NewJWTInterceptor()
	s := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	)
	pb.RegisterTunnelServiceServer(s, sconfig)

	debugCtx, debugCancel := context.WithCancel(context.Background())
	defer debugCancel()
	go sconfig.debugLogger(debugCtx)

	log.Printf("Listening for connections on TCP port 50051")
	check(s.Serve(lis))
}
