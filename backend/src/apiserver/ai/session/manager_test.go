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
	"testing"
	"time"

	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/provider"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

// newTestManager creates a SessionManager without the background cleanup goroutine,
// so tests can control cleanup explicitly.
func newTestManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func TestGetOrCreate_NewSession(t *testing.T) {
	sm := newTestManager()

	s := sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")
	if s == nil {
		t.Fatalf("GetOrCreate returned nil for new session")
	}
	if s.ID != "sess-1" {
		t.Errorf("expected session ID %q, got %q", "sess-1", s.ID)
	}
	if s.Mode != tools.ChatModeAsk {
		t.Errorf("expected mode %d, got %d", tools.ChatModeAsk, s.Mode)
	}
	if s.PendingConfirmations == nil {
		t.Errorf("expected PendingConfirmations map to be initialized, got nil")
	}
	if s.CreatedAt.IsZero() {
		t.Errorf("expected CreatedAt to be set")
	}
	if s.LastAccessedAt.IsZero() {
		t.Errorf("expected LastAccessedAt to be set")
	}
}

func TestGetOrCreate_ReturnsExistingSession(t *testing.T) {
	sm := newTestManager()

	s1 := sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")
	s2 := sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")

	if s1 != s2 {
		t.Fatalf("GetOrCreate returned a different session pointer for the same ID")
	}
}

func TestGetOrCreate_UpdatesModeOnExistingSession(t *testing.T) {
	sm := newTestManager()

	s := sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")
	if s.Mode != tools.ChatModeAsk {
		t.Fatalf("expected initial mode %d, got %d", tools.ChatModeAsk, s.Mode)
	}

	s2 := sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")
	if s2.Mode != tools.ChatModeAgent {
		t.Errorf("expected mode to be updated to %d, got %d", tools.ChatModeAgent, s2.Mode)
	}
}

func TestGet_ExistingSession(t *testing.T) {
	sm := newTestManager()

	sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")
	s, ok := sm.Get("sess-1")
	if !ok {
		t.Fatalf("Get returned false for existing session")
	}
	if s == nil {
		t.Fatalf("Get returned nil session for existing ID")
	}
	if s.ID != "sess-1" {
		t.Errorf("expected session ID %q, got %q", "sess-1", s.ID)
	}
}

func TestGet_NonExistentSession(t *testing.T) {
	sm := newTestManager()

	s, ok := sm.Get("does-not-exist")
	if ok {
		t.Errorf("Get returned true for non-existent session")
	}
	if s != nil {
		t.Errorf("Get returned non-nil session for non-existent ID")
	}
}

func TestAddMessage_Success(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")

	msg := provider.Message{Role: "user", Content: "hello"}
	if err := sm.AddMessage("sess-1", msg); err != nil {
		t.Fatalf("AddMessage returned unexpected error: %v", err)
	}

	s, _ := sm.Get("sess-1")
	if len(s.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(s.Messages))
	}
	if s.Messages[0].Role != "user" {
		t.Errorf("expected message role %q, got %q", "user", s.Messages[0].Role)
	}
	if s.Messages[0].Content != "hello" {
		t.Errorf("expected message content %q, got %v", "hello", s.Messages[0].Content)
	}
}

func TestAddMessage_MultipleMessages(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAsk, "")

	msgs := []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "user", Content: "how are you"},
	}
	for _, msg := range msgs {
		if err := sm.AddMessage("sess-1", msg); err != nil {
			t.Fatalf("AddMessage returned unexpected error: %v", err)
		}
	}

	s, _ := sm.Get("sess-1")
	if len(s.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(s.Messages))
	}
}

func TestAddMessage_UnknownSession(t *testing.T) {
	sm := newTestManager()

	msg := provider.Message{Role: "user", Content: "hello"}
	err := sm.AddMessage("unknown-session", msg)
	if err == nil {
		t.Fatalf("AddMessage should have returned an error for unknown session")
	}
}

func TestAddPendingConfirmation_Success(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")

	pending := &PendingToolCall{
		ToolCallID: "tc-1",
		ToolName:   "delete_run",
		Arguments:  map[string]interface{}{"run_id": "abc"},
		ResultCh:   make(chan ToolCallDecision, 1),
	}
	if err := sm.AddPendingConfirmation("sess-1", pending); err != nil {
		t.Fatalf("AddPendingConfirmation returned unexpected error: %v", err)
	}

	s, _ := sm.Get("sess-1")
	s.mu.Lock()
	p, ok := s.PendingConfirmations["tc-1"]
	s.mu.Unlock()
	if !ok {
		t.Fatalf("expected pending confirmation for tc-1 to exist")
	}
	if p.ToolName != "delete_run" {
		t.Errorf("expected tool name %q, got %q", "delete_run", p.ToolName)
	}
}

func TestAddPendingConfirmation_UnknownSession(t *testing.T) {
	sm := newTestManager()

	pending := &PendingToolCall{
		ToolCallID: "tc-1",
		ToolName:   "delete_run",
		ResultCh:   make(chan ToolCallDecision, 1),
	}
	err := sm.AddPendingConfirmation("unknown-session", pending)
	if err == nil {
		t.Fatalf("AddPendingConfirmation should have returned an error for unknown session")
	}
}

func TestResolveConfirmation_Approved(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")

	resultCh := make(chan ToolCallDecision, 1)
	pending := &PendingToolCall{
		ToolCallID: "tc-1",
		ToolName:   "delete_run",
		Arguments:  map[string]interface{}{"run_id": "abc"},
		ResultCh:   resultCh,
	}
	if err := sm.AddPendingConfirmation("sess-1", pending); err != nil {
		t.Fatalf("AddPendingConfirmation error: %v", err)
	}

	if err := sm.ResolveConfirmation("sess-1", "tc-1", true); err != nil {
		t.Fatalf("ResolveConfirmation returned unexpected error: %v", err)
	}

	select {
	case decision := <-resultCh:
		if !decision.Approved {
			t.Errorf("expected decision to be approved, got denied")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for decision on result channel")
	}

	// Verify the pending confirmation was removed.
	s, _ := sm.Get("sess-1")
	s.mu.Lock()
	_, exists := s.PendingConfirmations["tc-1"]
	s.mu.Unlock()
	if exists {
		t.Errorf("expected pending confirmation to be removed after resolution")
	}
}

func TestResolveConfirmation_Denied(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")

	resultCh := make(chan ToolCallDecision, 1)
	pending := &PendingToolCall{
		ToolCallID: "tc-2",
		ToolName:   "stop_run",
		Arguments:  map[string]interface{}{"run_id": "xyz"},
		ResultCh:   resultCh,
	}
	if err := sm.AddPendingConfirmation("sess-1", pending); err != nil {
		t.Fatalf("AddPendingConfirmation error: %v", err)
	}

	if err := sm.ResolveConfirmation("sess-1", "tc-2", false); err != nil {
		t.Fatalf("ResolveConfirmation returned unexpected error: %v", err)
	}

	select {
	case decision := <-resultCh:
		if decision.Approved {
			t.Errorf("expected decision to be denied, got approved")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for decision on result channel")
	}
}

func TestResolveConfirmation_UnknownSession(t *testing.T) {
	sm := newTestManager()

	err := sm.ResolveConfirmation("unknown-session", "tc-1", true)
	if err == nil {
		t.Fatalf("ResolveConfirmation should have returned an error for unknown session")
	}
}

func TestResolveConfirmation_UnknownToolCall(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")

	err := sm.ResolveConfirmation("sess-1", "nonexistent-tc", true)
	if err == nil {
		t.Fatalf("ResolveConfirmation should have returned an error for unknown tool call")
	}
}

func TestResolveConfirmation_NonBlockingSend(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("sess-1", tools.ChatModeAgent, "")

	// Use a buffered channel of size 1 and pre-fill it to simulate the
	// case where a decision was already sent (e.g., by cleanup).
	resultCh := make(chan ToolCallDecision, 1)
	resultCh <- ToolCallDecision{Approved: false} // pre-fill

	pending := &PendingToolCall{
		ToolCallID: "tc-prefilled",
		ToolName:   "delete_run",
		ResultCh:   resultCh,
	}
	if err := sm.AddPendingConfirmation("sess-1", pending); err != nil {
		t.Fatalf("AddPendingConfirmation error: %v", err)
	}

	// ResolveConfirmation should not block or panic even though the channel is full.
	err := sm.ResolveConfirmation("sess-1", "tc-prefilled", true)
	if err != nil {
		t.Fatalf("ResolveConfirmation returned unexpected error: %v", err)
	}

	// The original denial should still be in the channel.
	select {
	case decision := <-resultCh:
		if decision.Approved {
			t.Errorf("expected the pre-filled denial to remain, got approved")
		}
	default:
		t.Fatalf("expected a decision in the channel")
	}
}

func TestCleanupExpired_RemovesOldSessions(t *testing.T) {
	sm := newTestManager()

	// Create a session and manually set its LastAccessedAt to the past.
	s := sm.GetOrCreate("old-session", tools.ChatModeAsk, "")
	s.mu.Lock()
	s.LastAccessedAt = time.Now().Add(-SessionTimeout - time.Minute)
	s.mu.Unlock()

	// Create a recent session that should survive.
	sm.GetOrCreate("new-session", tools.ChatModeAsk, "")

	sm.CleanupExpired()

	if _, ok := sm.Get("old-session"); ok {
		t.Errorf("expected old-session to be cleaned up")
	}
	if _, ok := sm.Get("new-session"); !ok {
		t.Errorf("expected new-session to survive cleanup")
	}
}

func TestCleanupExpired_SendsDenialToPending(t *testing.T) {
	sm := newTestManager()

	s := sm.GetOrCreate("expiring-session", tools.ChatModeAgent, "")

	resultCh := make(chan ToolCallDecision, 1)
	pending := &PendingToolCall{
		ToolCallID: "tc-pending",
		ToolName:   "create_run",
		ResultCh:   resultCh,
	}
	if err := sm.AddPendingConfirmation("expiring-session", pending); err != nil {
		t.Fatalf("AddPendingConfirmation error: %v", err)
	}

	// Make the session expired.
	s.mu.Lock()
	s.LastAccessedAt = time.Now().Add(-SessionTimeout - time.Minute)
	s.mu.Unlock()

	sm.CleanupExpired()

	// The pending confirmation should have received a denial.
	select {
	case decision := <-resultCh:
		if decision.Approved {
			t.Errorf("expected denial from cleanup, got approved")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for denial from cleanup")
	}

	// Session should be removed.
	if _, ok := sm.Get("expiring-session"); ok {
		t.Errorf("expected expiring-session to be removed after cleanup")
	}
}

func TestCleanupExpired_NoExpiredSessions(t *testing.T) {
	sm := newTestManager()
	sm.GetOrCreate("active-1", tools.ChatModeAsk, "")
	sm.GetOrCreate("active-2", tools.ChatModeAgent, "")

	sm.CleanupExpired()

	if _, ok := sm.Get("active-1"); !ok {
		t.Errorf("expected active-1 to survive cleanup")
	}
	if _, ok := sm.Get("active-2"); !ok {
		t.Errorf("expected active-2 to survive cleanup")
	}
}

func TestCleanupExpired_MultiplePendingConfirmations(t *testing.T) {
	sm := newTestManager()
	s := sm.GetOrCreate("multi-pending", tools.ChatModeAgent, "")

	channels := make([]chan ToolCallDecision, 3)
	for i := 0; i < 3; i++ {
		ch := make(chan ToolCallDecision, 1)
		channels[i] = ch
		pending := &PendingToolCall{
			ToolCallID: fmt.Sprintf("tc-%d", i),
			ToolName:   "some_tool",
			ResultCh:   ch,
		}
		if err := sm.AddPendingConfirmation("multi-pending", pending); err != nil {
			t.Fatalf("AddPendingConfirmation error: %v", err)
		}
	}

	s.mu.Lock()
	s.LastAccessedAt = time.Now().Add(-SessionTimeout - time.Minute)
	s.mu.Unlock()

	sm.CleanupExpired()

	for i, ch := range channels {
		select {
		case decision := <-ch:
			if decision.Approved {
				t.Errorf("pending confirmation %d: expected denial, got approved", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("pending confirmation %d: timed out waiting for denial", i)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	sm := newTestManager()

	const numGoroutines = 50
	const sessionID = "concurrent-session"

	sm.GetOrCreate(sessionID, tools.ChatModeAsk, "")

	var wg sync.WaitGroup

	// Concurrent GetOrCreate calls.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mode := tools.ChatModeAsk
			if idx%2 == 0 {
				mode = tools.ChatModeAgent
			}
			s := sm.GetOrCreate(fmt.Sprintf("session-%d", idx), mode, "")
			if s == nil {
				t.Errorf("GetOrCreate returned nil for session-%d", idx)
			}
		}(i)
	}

	// Concurrent Get calls.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.Get(sessionID)
		}()
	}

	// Concurrent AddMessage calls.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := provider.Message{Role: "user", Content: fmt.Sprintf("msg-%d", idx)}
			sm.AddMessage(sessionID, msg)
		}(i)
	}

	// Concurrent AddPendingConfirmation and ResolveConfirmation calls.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tcID := fmt.Sprintf("tc-concurrent-%d", idx)
			pending := &PendingToolCall{
				ToolCallID: tcID,
				ToolName:   "test_tool",
				ResultCh:   make(chan ToolCallDecision, 1),
			}
			if err := sm.AddPendingConfirmation(sessionID, pending); err != nil {
				return
			}
			sm.ResolveConfirmation(sessionID, tcID, idx%2 == 0)
		}(i)
	}

	// Concurrent CleanupExpired calls (should be safe even with active sessions).
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.CleanupExpired()
		}()
	}

	wg.Wait()

	// Verify the main session still exists and has the expected messages.
	s, ok := sm.Get(sessionID)
	if !ok {
		t.Fatalf("expected %q to still exist after concurrent access", sessionID)
	}
	s.mu.Lock()
	msgCount := len(s.Messages)
	s.mu.Unlock()
	if msgCount != numGoroutines {
		t.Errorf("expected %d messages, got %d", numGoroutines, msgCount)
	}
}

func TestNewSessionManager(t *testing.T) {
	// Verify that NewSessionManager returns a valid manager with an initialized map.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sm := NewSessionManager(ctx)
	if sm == nil {
		t.Fatalf("NewSessionManager returned nil")
	}
	if sm.sessions == nil {
		t.Fatalf("NewSessionManager sessions map is nil")
	}

	// Verify it works by creating a session.
	s := sm.GetOrCreate("test-new", tools.ChatModeAsk, "")
	if s == nil {
		t.Errorf("GetOrCreate returned nil on manager from NewSessionManager")
	}
}

func TestGetOrCreate_UpdatesLastAccessedAt(t *testing.T) {
	sm := newTestManager()

	s := sm.GetOrCreate("sess-time", tools.ChatModeAsk, "")
	s.mu.Lock()
	firstAccess := s.LastAccessedAt
	s.mu.Unlock()

	// Small sleep to ensure time advances.
	time.Sleep(5 * time.Millisecond)

	sm.GetOrCreate("sess-time", tools.ChatModeAsk, "")
	s.mu.Lock()
	secondAccess := s.LastAccessedAt
	s.mu.Unlock()

	if !secondAccess.After(firstAccess) {
		t.Errorf("expected LastAccessedAt to be updated on second GetOrCreate, first=%v second=%v", firstAccess, secondAccess)
	}
}

func TestGet_UpdatesLastAccessedAt(t *testing.T) {
	sm := newTestManager()

	s := sm.GetOrCreate("sess-get-time", tools.ChatModeAsk, "")
	s.mu.Lock()
	firstAccess := s.LastAccessedAt
	s.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	sm.Get("sess-get-time")
	s.mu.Lock()
	secondAccess := s.LastAccessedAt
	s.mu.Unlock()

	if !secondAccess.After(firstAccess) {
		t.Errorf("expected LastAccessedAt to be updated on Get, first=%v second=%v", firstAccess, secondAccess)
	}
}
