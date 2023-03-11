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

	"github.com/skandragon/grpc-datacon/internal/serviceconfig"
	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
)

type ServerEcho struct {
	streamID string
	state    echoState
}

type echoState int

const (
	stateHeaders echoState = iota
	stateData
	stateDone
)

func MakeEcho(ctx context.Context, streamID string) serviceconfig.HTTPEcho {
	e := &ServerEcho{
		streamID: streamID,
		state:    stateHeaders,
	}
	return e
}

func (e *ServerEcho) Headers(ctx context.Context, h *pb.TunnelHeaders) error {
	log.Printf("echo.Headers(%s): %v", e.streamID, h)
	if e.state != stateHeaders {
		return fmt.Errorf("programmer error: Headers called when not in correct state (in %d)", e.state)
	}
	e.state = stateData
	return nil
}

func (e *ServerEcho) Data(ctx context.Context, data []byte) error {
	log.Printf("echo.Data(%s): %s", e.streamID, string(data))
	if e.state != stateData {
		return fmt.Errorf("programmer error: Data called when not in correct state (in %d)", e.state)
	}
	return nil
}

func (e *ServerEcho) Fail(ctx context.Context, code int, err error) error {
	log.Printf("echo.Fail(%s): code %d, err %v", e.streamID, code, err)
	//defer close(e.dchan)
	e.state = stateDone
	return nil
}

func (e *ServerEcho) Done(ctx context.Context) error {
	log.Printf("echo.Done(%s)", e.streamID)
	//defer close(e.dchan)
	if e.state != stateData {
		return fmt.Errorf("programmer error: Done called when not in correct state (in %d)", e.state)
	}
	return nil
}
