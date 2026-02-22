// Copyright 2024 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/provider"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

const (
	// SessionTimeout is the duration after which idle sessions are cleaned up.
	SessionTimeout = 30 * time.Minute

	// MaxSessions is the maximum number of concurrent sessions allowed.
	MaxSessions = 1000

	// MaxMessagesPerSession is the maximum number of messages retained per session.
	// Older messages are trimmed when the limit is exceeded.
	MaxMessagesPerSession = 200
)

// Session represents an AI chat session with conversation history.
type Session struct {
	ID                   string
	UserID               string
	Messages             []provider.Message
	Mode                 tools.ChatMode
	PendingConfirmations map[string]*PendingToolCall
	CreatedAt            time.Time
	LastAccessedAt       time.Time
	mu                   sync.Mutex
}

// PendingToolCall represents a tool call awaiting user confirmation.
type PendingToolCall struct {
	ToolCallID string
	ToolName   string
	Arguments  map[string]interface{}
	ResultCh   chan ToolCallDecision
}

// ToolCallDecision represents the user's decision on a pending tool call.
type ToolCallDecision struct {
	Approved bool
}

// SessionManager manages in-memory chat sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager creates a new session manager.
// The provided context controls the lifetime of the background cleanup goroutine.
func NewSessionManager(ctx context.Context) *SessionManager {
	sm := &SessionManager{
		sessions: make(map[string]*Session),
	}
	go sm.cleanupLoop(ctx)
	return sm
}

// GetOrCreate retrieves an existing session or creates a new one.
// The userID parameter binds the session to the authenticated caller;
// if a session already exists for a different user a new, user-scoped
// session is created instead of reusing the existing one.
func (sm *SessionManager) GetOrCreate(sessionID string, mode tools.ChatMode, userID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if s, ok := sm.sessions[sessionID]; ok {
		s.mu.Lock()
		if s.UserID != "" && userID != "" && s.UserID != userID {
			s.mu.Unlock()
			// Create a new session for this user instead of reusing another user's session
			sessionID = sessionID + "-" + userID
			// Check again
			if s2, ok2 := sm.sessions[sessionID]; ok2 {
				s2.mu.Lock()
				s2.LastAccessedAt = time.Now()
				s2.Mode = mode
				s2.mu.Unlock()
				return s2
			}
			s = &Session{
				ID:                   sessionID,
				UserID:               userID,
				Messages:             nil,
				Mode:                 mode,
				PendingConfirmations: make(map[string]*PendingToolCall),
				CreatedAt:            time.Now(),
				LastAccessedAt:       time.Now(),
			}
			sm.sessions[sessionID] = s
			return s
		}
		s.LastAccessedAt = time.Now()
		s.Mode = mode
		s.mu.Unlock()
		return s
	}

	// Enforce max session count by evicting least-recently-used sessions.
	if len(sm.sessions) >= MaxSessions {
		sm.evictLRU()
	}

	s := &Session{
		ID:                   sessionID,
		UserID:               userID,
		Messages:             nil,
		Mode:                 mode,
		PendingConfirmations: make(map[string]*PendingToolCall),
		CreatedAt:            time.Now(),
		LastAccessedAt:       time.Now(),
	}
	sm.sessions[sessionID] = s
	return s
}

// evictLRU removes the least-recently-used session. Caller must hold sm.mu write lock.
func (sm *SessionManager) evictLRU() {
	var oldestID string
	var oldestTime time.Time

	for id, s := range sm.sessions {
		s.mu.Lock()
		accessed := s.LastAccessedAt
		s.mu.Unlock()
		if oldestID == "" || accessed.Before(oldestTime) {
			oldestID = id
			oldestTime = accessed
		}
	}

	if oldestID != "" {
		s := sm.sessions[oldestID]
		s.mu.Lock()
		for _, p := range s.PendingConfirmations {
			select {
			case p.ResultCh <- ToolCallDecision{Approved: false}:
			default:
			}
		}
		s.mu.Unlock()
		delete(sm.sessions, oldestID)
		glog.Warningf("Evicted LRU session %s to enforce max session limit", oldestID)
	}
}

// Get retrieves an existing session.
func (sm *SessionManager) Get(sessionID string) (*Session, bool) {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if ok {
		s.mu.Lock()
		s.LastAccessedAt = time.Now()
		s.mu.Unlock()
	}
	return s, ok
}

// GetMessages returns a snapshot of the session's messages, safe for concurrent use.
func (sm *SessionManager) GetMessages(sessionID string) ([]provider.Message, error) {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := make([]provider.Message, len(s.Messages))
	copy(snapshot, s.Messages)
	return snapshot, nil
}

// ValidateSessionOwner checks that the given userID matches the session's owner.
// Returns an error if the session exists and belongs to a different user.
func (sm *SessionManager) ValidateSessionOwner(sessionID, userID string) error {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.UserID != "" && userID != "" && s.UserID != userID {
		return fmt.Errorf("session %s does not belong to the requesting user", sessionID)
	}
	return nil
}

// AddMessage appends a message to the session's conversation history.
// If the message count exceeds MaxMessagesPerSession, older messages are trimmed.
func (sm *SessionManager) AddMessage(sessionID string, msg provider.Message) error {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
	s.LastAccessedAt = time.Now()

	// Trim old messages if over limit, keeping the most recent ones.
	if len(s.Messages) > MaxMessagesPerSession {
		excess := len(s.Messages) - MaxMessagesPerSession
		s.Messages = s.Messages[excess:]
	}

	return nil
}

// AddPendingConfirmation adds a tool call awaiting user confirmation.
func (sm *SessionManager) AddPendingConfirmation(sessionID string, pending *PendingToolCall) error {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.PendingConfirmations[pending.ToolCallID] = pending
	return nil
}

// ResolveConfirmation resolves a pending tool call confirmation.
func (sm *SessionManager) ResolveConfirmation(sessionID, toolCallID string, approved bool) error {
	sm.mu.RLock()
	s, ok := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}

	s.mu.Lock()
	pending, ok := s.PendingConfirmations[toolCallID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("no pending confirmation for tool call %s", toolCallID)
	}
	delete(s.PendingConfirmations, toolCallID)
	s.mu.Unlock()

	// Use non-blocking send to avoid panic if the channel was already
	// resolved (e.g., session expired and sent a denial).
	select {
	case pending.ResultCh <- ToolCallDecision{Approved: approved}:
	default:
		// Channel already has a value; this can happen if the session
		// was cleaned up concurrently and a denial was sent.
	}
	return nil
}

// CleanupExpired removes sessions that have been idle for longer than SessionTimeout.
func (sm *SessionManager) CleanupExpired() {
	now := time.Now()

	// Collect expired session IDs under read lock first.
	sm.mu.RLock()
	var expiredIDs []string
	for id, s := range sm.sessions {
		s.mu.Lock()
		expired := now.Sub(s.LastAccessedAt) > SessionTimeout
		s.mu.Unlock()
		if expired {
			expiredIDs = append(expiredIDs, id)
		}
	}
	sm.mu.RUnlock()

	if len(expiredIDs) == 0 {
		return
	}

	// Remove expired sessions under write lock.
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, id := range expiredIDs {
		s, ok := sm.sessions[id]
		if !ok {
			continue
		}
		s.mu.Lock()
		// Re-check expiry under lock to avoid TOCTOU. Use time.Since for accuracy.
		if time.Since(s.LastAccessedAt) > SessionTimeout {
			// Send denial to any pending confirmations instead of closing channels.
			for _, p := range s.PendingConfirmations {
				select {
				case p.ResultCh <- ToolCallDecision{Approved: false}:
				default:
				}
			}
			delete(sm.sessions, id)
		}
		s.mu.Unlock()
	}
}

func (sm *SessionManager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sm.CleanupExpired()
		case <-ctx.Done():
			return
		}
	}
}
