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
	"time"

	"github.com/skandragon/grpc-datacon/internal/logging"
	"go.uber.org/zap"
)

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

func (s *server) removeAgentSession(session *agentContext) {
	s.Lock()
	defer s.Unlock()
	s.removeAgentSessionUnlocked(session)
}

func (s *server) removeAgentSessionUnlocked(session *agentContext) {
	key := agentKey{agentID: session.agentID, sessionID: session.sessionID}
	delete(s.agents, key)
	close(session.in)
	close(session.out)
}

func (s *server) touchSession(session *agentContext, t int64) {
	s.Lock()
	defer s.Unlock()
	s.agents[session.agentKey].lastUsed = t
}

func (s *server) debugPrintSessions(ctx context.Context) {
	_, logger := loggerFromContext(ctx)
	now := time.Now().UnixNano()
	s.Lock()
	defer s.Unlock()
	for _, session := range s.agents {
		logger.Infof("agentID %s, sessionID %s, idleTime %d", session.agentID, session.sessionID, (now-session.lastUsed)/1000000)
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
	_, logging := loggerFromContext(ctx)
	s.Lock()
	defer s.Unlock()

	for key, session := range s.agents {
		if session.lastUsed+s.agentIdleTimeout < now {
			logging.Infow("disconnecting idle agent", "lastUsed", session.lastUsed, "now", now, "agentID", key.agentID, "sessionID", key.sessionID)
			s.removeAgentSessionUnlocked(session)
		}
	}
}
