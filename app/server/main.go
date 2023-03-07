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

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
	"github.com/skandragon/grpc-datacon/internal/ulid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
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

// get a random session
func (s *server) getRandomSession() *agentContext {
	s.Lock()
	defer s.Unlock()
	for _, session := range s.agents {
		return session
	}
	return nil
}

// fake a http request to some random agent
func (s *server) randomRequest(ctx context.Context) {
	session := s.getRandomSession()
	if session == nil {
		return
	}
	session.out <- &pb.TunnelRequest{
		StreamId: ulid.GlobalContext.Ulid(),
		Name:     "bobService",
		Type:     "bobServiceType",
		Method:   "GET",
		URI:      "http://blog.flame.org/",
	}
}

func (s *server) requestOnTimer(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.randomRequest(ctx)
		}
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

	requesterCtx, requesterCancel := context.WithCancel(ctx)
	defer requesterCancel()
	go sconfig.requestOnTimer(requesterCtx)

	log.Printf("Listening for connections on TCP port 50051")
	check(s.Serve(lis))
}
