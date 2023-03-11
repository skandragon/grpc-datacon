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

	"github.com/skandragon/grpc-datacon/internal/serviceconfig"
	pb "github.com/skandragon/grpc-datacon/internal/tunnel"
)

type AgentEcho struct {
	streamID string
	c        pb.TunnelServiceClient
	state    echoState
	dchan    chan *pb.Data
}

type echoState int

const (
	stateHeaders echoState = iota
	stateData
	stateDone
)

func MakeEcho(ctx context.Context, c pb.TunnelServiceClient, streamID string) serviceconfig.HTTPEcho {
	e := &AgentEcho{
		streamID: streamID,
		c:        c,
		state:    stateHeaders,
		dchan:    make(chan *pb.Data),
	}
	go e.RunDataSender(ctx)
	return e
}

// TODO: return any errors to "caller"
func (e *AgentEcho) RunDataSender(ctx context.Context) {
	ctx, logger := loggerFromContext(ctx)
	stream, err := e.c.SendData(ctx)
	defer stream.CloseSend()
	if err != nil {
		logger.Errorf("e.c.SendData(): %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			logger.Infof("Run() context done")
			return
		case d, more := <-e.dchan:
			if !more {
				logger.Infof("RunDataSender() exiting")
				return
			}
			err := stream.Send(d)
			if err != nil {
				logger.Errorf("stream.Send(): %v", err)
			}
		}
	}
}

func (e *AgentEcho) Headers(ctx context.Context, h *pb.TunnelHeaders) error {
	if e.state != stateHeaders {
		return fmt.Errorf("programmer error: Headers called when not in correct state (in %d)", e.state)
	}
	h.StreamId = e.streamID
	e.state = stateData
	_, err := e.c.SendHeaders(ctx, h)
	return err
}

func (e *AgentEcho) Data(ctx context.Context, data []byte) error {
	if e.state != stateData {
		return fmt.Errorf("programmer error: Data called when not in correct state (in %d)", e.state)
	}
	d := &pb.Data{
		StreamId: e.streamID,
		Data:     data,
	}
	e.dchan <- d
	return nil
}

func (e *AgentEcho) Fail(ctx context.Context, code int, err error) error {
	_, logger := loggerFromContext(ctx)
	defer close(e.dchan)

	// headers not sent, so we can return a better error
	if e.state == stateHeaders {
		h := &pb.TunnelHeaders{
			StreamId: e.streamID,
			Status:   int32(code),
		}
		_, err = e.c.SendHeaders(ctx, h)
		if err != nil {
			logger.Errorf("SendHeaders failed (ignoring): %v", err)
		}
	}
	e.state = stateDone

	// Send EOF data
	d := &pb.Data{
		StreamId: e.streamID,
		Data:     []byte{},
	}
	e.dchan <- d
	return nil
}

func (e *AgentEcho) Done(ctx context.Context) error {
	defer close(e.dchan)
	if e.state != stateData {
		return fmt.Errorf("programmer error: Done called when not in correct state (in %d)", e.state)
	}
	d := &pb.Data{
		StreamId: e.streamID,
		Data:     []byte{},
	}
	e.dchan <- d
	return nil
}
