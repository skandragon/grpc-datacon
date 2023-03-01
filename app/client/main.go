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
	"sync"
	"time"

	"github.com/skandragon/grpc-datacon/internal/tunnel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	kparams := keepalive.ClientParameters{
		Time:                10 * time.Second,
		Timeout:             5 * time.Second,
		PermitWithoutStream: true,
	}
	gopts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(kparams),
	}
	conn, err := grpc.Dial("localhost:50051", gopts...)
	check(err)
	defer conn.Close()
	c := tunnel.NewTunnelServiceClient(conn)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Intn(2)) * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			r, err := c.Ping(ctx, &tunnel.PingRequest{Ts: uint64(i)})
			if err != nil {
				log.Printf("Got error: %v", err)
				return
			}
			log.Printf("Got ping repsonse: servertime=%d, mytime=%d", r.Ts, r.EchoedTs)
		}(i)
	}

	log.Printf("waiting...")
	wg.Wait()
	select {}
}
