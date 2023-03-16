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
	streamID      string
	c             pb.TunnelServiceClient
	state         echoState
	dchan         chan *pb.Data
	doneChan      chan bool
	localDoneChan chan bool
}

type echoState int

const (
	stateHeaders echoState = iota
	stateData
	stateDone
	stateCanceled
)

func MakeEcho(ctx context.Context, c pb.TunnelServiceClient, streamID string, doneChan chan bool) serviceconfig.HTTPEcho {
	e := &AgentEcho{
		streamID:      streamID,
		c:             c,
		state:         stateHeaders,
		dchan:         make(chan *pb.Data),
		doneChan:      doneChan,
		localDoneChan: make(chan bool),
	}
	go e.RunDataSender(ctx)
	return e
}

// TODO: return any errors to "caller"
func (e *AgentEcho) RunDataSender(ctx context.Context) {
	ctx, logger := loggerFromContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	stream, err := e.c.SendData(ctx)

	defer func() {
		if err := stream.CloseSend(); err != nil {
			logger.Warn(err)
		}
		cancel()
		close(e.dchan)
	}()

	if err != nil {
		logger.Errorf("e.c.SendData(): %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			e.trySendDone(ctx)
			logger.Infof("Run() context done")
			return
		case <-e.localDoneChan:
			e.trySendDone(ctx)
			return
		case d := <-e.dchan:
			err := stream.Send(d)
			if err != nil {
				logger.Errorf("stream.Send(): %v", err)
				e.trySendDone(ctx)
				e.setState(ctx, stateCanceled)
				return
			}
		}
	}
}

func (e *AgentEcho) setState(ctx context.Context, state echoState) {
	//logger := logging.WithContext(ctx).Sugar()
	//logger.Errorf("Setting state: %d", state)
	e.state = state
}

func (e *AgentEcho) Headers(ctx context.Context, h *pb.TunnelHeaders) error {
	if e.state != stateHeaders {
		return fmt.Errorf("programmer error: Headers called when not in correct state (in %d)", e.state)
	}
	h.StreamId = e.streamID
	e.setState(ctx, stateData)
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
	ctx, logger := loggerFromContext(ctx)
	defer e.trySendLocalDone(ctx)

	if e.state == stateCanceled || e.state == stateDone {
		return nil
	}

	// headers not sent, so we can return a better error
	if e.state == stateHeaders {
		h := &pb.TunnelHeaders{
			StreamId:   e.streamID,
			StatusCode: int32(code),
		}
		if _, err := e.c.SendHeaders(ctx, h); err != nil {
			logger.Errorf("SendHeaders failed (ignoring): %v", err)
		}
	}
	e.setState(ctx, stateDone)

	// Send EOF data
	d := &pb.Data{
		StreamId: e.streamID,
		Data:     []byte{},
	}
	e.dchan <- d

	return nil
}

func (e *AgentEcho) Done(ctx context.Context) error {
	defer e.trySendLocalDone(ctx)
	if e.state != stateData {
		return fmt.Errorf("programmer error: Done called when not in correct state (in %d)", e.state)
	}
	d := &pb.Data{
		StreamId: e.streamID,
		Data:     []byte{},
	}
	e.dchan <- d
	e.setState(ctx, stateDone)
	return nil
}

func (e *AgentEcho) trySendDone(ctx context.Context) {
	select {
	case e.doneChan <- true:
	default:
	}
}

func (e *AgentEcho) trySendLocalDone(ctx context.Context) {
	select {
	case e.localDoneChan <- true:
	default:
	}
}

func (e *AgentEcho) Cancel(ctx context.Context) error {
	if e.state == stateDone || e.state == stateCanceled {
		return nil
	}
	defer e.trySendLocalDone(ctx)
	ctx, logger := loggerFromContext(ctx)
	e.setState(ctx, stateCanceled)

	c := &pb.CancelStreamRequest{StreamId: e.streamID}
	if _, err := e.c.CancelStream(ctx, c); err != nil {
		logger.Errorf("CancelStream failed (ignoring): %v", err)
	}
	return nil
}
