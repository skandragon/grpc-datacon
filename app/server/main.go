/*
 * Copyright 2023 Michael Graff.
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
	"math/rand"
	"net"
	"time"

	"github.com/skandragon/grpc-datacon/internal/tunnel"
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

type server struct {
	tunnel.UnimplementedTunnelServiceServer
}

func (s *server) Ping(ctx context.Context, in *tunnel.PingRequest) (*tunnel.PingResponse, error) {
	log.Printf("starting Ping(%d)", in.Ts)
	time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
	r := &tunnel.PingResponse{
		Ts:       uint64(time.Now().UnixNano()),
		EchoedTs: in.Ts,
	}

	log.Printf("exiting Ping(%d)", in.Ts)
	return r, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	check(err)
	s := grpc.NewServer(grpc.KeepaliveEnforcementPolicy(kaep), grpc.KeepaliveParams(kasp))
	tunnel.RegisterTunnelServiceServer(s, &server{})
	log.Printf("Listening for connections on TCP port 50051")
	check(s.Serve(lis))
}
