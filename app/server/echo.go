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

	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
)

type ServerEcho struct {
	streamID    string
	state       echoState
	headersChan chan *pb.TunnelHeaders
	dataChan    chan []byte
	doneChan    chan bool
	failChan    chan int
}

type echoState int

const (
	stateHeaders echoState = iota
	stateData
	stateDone
)

func MakeIncomingEchoer(ctx context.Context, streamID string) *ServerEcho {
	e := &ServerEcho{
		streamID:    streamID,
		state:       stateHeaders,
		headersChan: make(chan *pb.TunnelHeaders),
		dataChan:    make(chan []byte),
		doneChan:    make(chan bool),
		failChan:    make(chan int),
	}
	return e
}

func (e *ServerEcho) Headers(ctx context.Context, h *pb.TunnelHeaders) error {
	if e.state != stateHeaders {
		return fmt.Errorf("programmer error: Headers called when not in correct state (in %d)", e.state)
	}
	e.state = stateData
	e.headersChan <- h
	return nil
}

func (e *ServerEcho) Data(ctx context.Context, data []byte) error {
	if e.state != stateData {
		return fmt.Errorf("programmer error: Data called when not in correct state (in %d)", e.state)
	}
	e.dataChan <- data
	return nil
}

func (e *ServerEcho) closeChannels() {
	close(e.dataChan)
	close(e.doneChan)
	close(e.headersChan)
	close(e.failChan)
}

func (e *ServerEcho) Fail(ctx context.Context, code int, err error) error {
	log.Printf("echo.Fail(%s): code %d, err %v", e.streamID, code, err)
	defer e.closeChannels()
	e.state = stateDone
	e.failChan <- code
	return nil
}

func (e *ServerEcho) Done(ctx context.Context) error {
	defer e.closeChannels()
	if e.state != stateData {
		return fmt.Errorf("programmer error: Done called when not in correct state (in %d)", e.state)
	}
	e.doneChan <- true
	return nil
}
