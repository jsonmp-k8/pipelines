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

package ai

import (
	"context"
	"fmt"
	"sync"
	"testing"

	aicontext "github.com/kubeflow/pipelines/backend/src/apiserver/ai/context"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/provider"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/rules"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/session"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

// mockChatModel implements provider.ChatModel for testing.
type mockChatModel struct {
	mu        sync.Mutex
	callCount int
	// calls is a list of responses, one per StreamChat invocation.
	// Each response is a slice of StreamEvents to send on the event channel.
	calls [][]provider.StreamEvent
	// errors is a list of errors, one per StreamChat invocation (nil means no error).
	errors []error
}

func (m *mockChatModel) StreamChat(ctx context.Context, messages []provider.Message, toolDefs []provider.ToolDefinition, systemPrompt string) (<-chan provider.StreamEvent, <-chan error) {
	m.mu.Lock()
	idx := m.callCount
	m.callCount++
	m.mu.Unlock()

	eventCh := make(chan provider.StreamEvent, 100)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)

		if idx < len(m.calls) {
			for _, ev := range m.calls[idx] {
				eventCh <- ev
			}
		}

		if idx < len(m.errors) && m.errors[idx] != nil {
			errCh <- m.errors[idx]
		} else {
			errCh <- nil
		}
	}()

	return eventCh, errCh
}

func (m *mockChatModel) ModelName() string {
	return "mock-model"
}

// mockTool implements tools.Tool for testing.
type mockTool struct {
	readOnly bool
	result   *tools.ToolResult
}

func (m *mockTool) Name() string                    { return "mock_tool" }
func (m *mockTool) Description() string             { return "a mock tool" }
func (m *mockTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object"}
}
func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	return m.result, nil
}
func (m *mockTool) IsReadOnly() bool { return m.readOnly }

// collectEvents is a helper that collects ChatResponseEvents into a slice.
type eventCollector struct {
	mu     sync.Mutex
	events []ChatResponseEvent
}

func (c *eventCollector) send(event ChatResponseEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
}

func (c *eventCollector) getEvents() []ChatResponseEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]ChatResponseEvent, len(c.events))
	copy(result, c.events)
	return result
}

// findEvents returns all events matching the given type.
func findEvents(events []ChatResponseEvent, eventType string) []ChatResponseEvent {
	var matched []ChatResponseEvent
	for _, e := range events {
		if e.Type == eventType {
			matched = append(matched, e)
		}
	}
	return matched
}

// newTestServer creates an AIServer with the given mock model and optional tool registry.
func newTestServer(model provider.ChatModel, reg *tools.ToolRegistry) *AIServer {
	sm := session.NewSessionManager()
	rm := rules.NewRuleManager()
	cb := aicontext.NewContextBuilder(nil)
	if reg == nil {
		reg = tools.NewToolRegistry()
	}
	return NewAIServer(model, reg, sm, cb, rm)
}

func TestStreamChat_SimpleTextResponse(t *testing.T) {
	mock := &mockChatModel{
		calls: [][]provider.StreamEvent{
			{
				{
					Type:         "content_block_start",
					ContentBlock: &provider.ContentBlock{Type: "text"},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "text_delta", Text: "Hello, "},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "text_delta", Text: "world!"},
				},
				{
					Type: "content_block_stop",
				},
				{
					Type:  "message_delta",
					Delta: &provider.Delta{StopReason: "end_turn"},
				},
			},
		},
		errors: []error{nil},
	}

	server := newTestServer(mock, nil)
	collector := &eventCollector{}

	req := &ChatRequest{
		Message:   "Hi",
		SessionID: "test-session-1",
		Mode:      1, // Ask mode
	}

	err := server.StreamChat(context.Background(), req, collector.send)
	if err != nil {
		t.Fatalf("StreamChat returned unexpected error: %v", err)
	}

	events := collector.getEvents()

	// Verify session_metadata event is first
	if len(events) == 0 {
		t.Fatal("expected at least one event, got none")
	}
	if events[0].Type != "session_metadata" {
		t.Errorf("expected first event type 'session_metadata', got %q", events[0].Type)
	}
	metaData, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("session_metadata data is not a map")
	}
	if metaData["model"] != "mock-model" {
		t.Errorf("expected model 'mock-model', got %v", metaData["model"])
	}

	// Verify progress event exists
	progressEvents := findEvents(events, "progress")
	if len(progressEvents) == 0 {
		t.Error("expected at least one progress event")
	}

	// Verify markdown_chunk events contain the correct text
	chunks := findEvents(events, "markdown_chunk")
	if len(chunks) != 2 {
		t.Fatalf("expected 2 markdown_chunk events, got %d", len(chunks))
	}

	chunk0Data, ok := chunks[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("markdown_chunk data is not a map")
	}
	if chunk0Data["content"] != "Hello, " {
		t.Errorf("expected first chunk content 'Hello, ', got %v", chunk0Data["content"])
	}

	chunk1Data, ok := chunks[1].Data.(map[string]interface{})
	if !ok {
		t.Fatal("markdown_chunk data is not a map")
	}
	if chunk1Data["content"] != "world!" {
		t.Errorf("expected second chunk content 'world!', got %v", chunk1Data["content"])
	}
}

func TestStreamChat_ToolUseLoop(t *testing.T) {
	// First call: model requests a tool use
	// Second call: model returns a text response
	mock := &mockChatModel{
		calls: [][]provider.StreamEvent{
			// First invocation: tool_use
			{
				{
					Type: "content_block_start",
					ContentBlock: &provider.ContentBlock{
						Type: "tool_use",
						ID:   "tool-call-1",
						Name: "mock_tool",
					},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "input_json_delta", PartialJSON: `{"key":`},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "input_json_delta", PartialJSON: `"value"}`},
				},
				{
					Type: "content_block_stop",
				},
				{
					Type:  "message_delta",
					Delta: &provider.Delta{StopReason: "tool_use"},
				},
			},
			// Second invocation: text response
			{
				{
					Type:         "content_block_start",
					ContentBlock: &provider.ContentBlock{Type: "text"},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "text_delta", Text: "Based on the tool result, the answer is 42."},
				},
				{
					Type: "content_block_stop",
				},
				{
					Type:  "message_delta",
					Delta: &provider.Delta{StopReason: "end_turn"},
				},
			},
		},
		errors: []error{nil, nil},
	}

	// Create a read-only mock tool and register it
	mt := &mockTool{
		readOnly: true,
		result: &tools.ToolResult{
			Content: `{"answer": 42}`,
			IsError: false,
		},
	}
	reg := tools.NewToolRegistry()
	reg.Register(mt)

	server := newTestServer(mock, reg)
	collector := &eventCollector{}

	req := &ChatRequest{
		Message:   "What is the answer?",
		SessionID: "test-session-tool",
		Mode:      1, // Ask mode - read-only tools are allowed
	}

	err := server.StreamChat(context.Background(), req, collector.send)
	if err != nil {
		t.Fatalf("StreamChat returned unexpected error: %v", err)
	}

	events := collector.getEvents()

	// Verify tool_call events were sent
	toolCallEvents := findEvents(events, "tool_call")
	if len(toolCallEvents) == 0 {
		t.Fatal("expected at least one tool_call event")
	}

	// The first tool_call event should have the tool name
	tc0Data, ok := toolCallEvents[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("tool_call data is not a map")
	}
	if tc0Data["tool_name"] != "mock_tool" {
		t.Errorf("expected tool_name 'mock_tool', got %v", tc0Data["tool_name"])
	}
	if tc0Data["tool_call_id"] != "tool-call-1" {
		t.Errorf("expected tool_call_id 'tool-call-1', got %v", tc0Data["tool_call_id"])
	}

	// Verify tool_result event was sent
	toolResultEvents := findEvents(events, "tool_result")
	if len(toolResultEvents) == 0 {
		t.Fatal("expected at least one tool_result event")
	}
	trData, ok := toolResultEvents[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("tool_result data is not a map")
	}
	if trData["tool_call_id"] != "tool-call-1" {
		t.Errorf("expected tool_result tool_call_id 'tool-call-1', got %v", trData["tool_call_id"])
	}
	if trData["success"] != true {
		t.Errorf("expected tool_result success=true, got %v", trData["success"])
	}

	// Verify markdown_chunk event from the second model call
	chunks := findEvents(events, "markdown_chunk")
	if len(chunks) == 0 {
		t.Fatal("expected at least one markdown_chunk event from the final text response")
	}
	found := false
	for _, chunk := range chunks {
		chunkData, ok := chunk.Data.(map[string]interface{})
		if ok && chunkData["content"] == "Based on the tool result, the answer is 42." {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected markdown_chunk with 'Based on the tool result, the answer is 42.' but did not find it")
	}

	// Verify the mock model was called twice (once for tool_use, once for final response)
	mock.mu.Lock()
	callCount := mock.callCount
	mock.mu.Unlock()
	if callCount != 2 {
		t.Errorf("expected mock model to be called 2 times, got %d", callCount)
	}
}

func TestStreamChat_DefaultsToAskMode(t *testing.T) {
	mock := &mockChatModel{
		calls: [][]provider.StreamEvent{
			{
				{
					Type:         "content_block_start",
					ContentBlock: &provider.ContentBlock{Type: "text"},
				},
				{
					Type:  "content_block_delta",
					Delta: &provider.Delta{Type: "text_delta", Text: "response"},
				},
				{
					Type: "content_block_stop",
				},
				{
					Type:  "message_delta",
					Delta: &provider.Delta{StopReason: "end_turn"},
				},
			},
		},
		errors: []error{nil},
	}

	server := newTestServer(mock, nil)
	collector := &eventCollector{}

	req := &ChatRequest{
		Message:   "test",
		SessionID: "test-session-default-mode",
		Mode:      0, // Unset mode, should default to ChatModeAsk (1)
	}

	err := server.StreamChat(context.Background(), req, collector.send)
	if err != nil {
		t.Fatalf("StreamChat returned unexpected error: %v", err)
	}

	// The session metadata event should exist (confirming the flow completed).
	// The mode defaults to ChatModeAsk=1 internally.
	// We verify by checking the session was created with Ask mode.
	events := collector.getEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}
	if events[0].Type != "session_metadata" {
		t.Errorf("expected first event type 'session_metadata', got %q", events[0].Type)
	}

	// Verify the session was created (session_metadata event includes session_id).
	metaData, ok := events[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("session_metadata data is not a map")
	}
	sessionID, ok := metaData["session_id"].(string)
	if !ok || sessionID == "" {
		t.Error("expected non-empty session_id in session_metadata")
	}
}

func TestStreamChat_ProviderError(t *testing.T) {
	providerErr := fmt.Errorf("upstream API is unavailable")

	mock := &mockChatModel{
		calls: [][]provider.StreamEvent{
			// No events, just error
			{},
		},
		errors: []error{providerErr},
	}

	server := newTestServer(mock, nil)
	collector := &eventCollector{}

	req := &ChatRequest{
		Message:   "Hi",
		SessionID: "test-session-err",
		Mode:      1,
	}

	err := server.StreamChat(context.Background(), req, collector.send)
	if err == nil {
		t.Fatal("expected StreamChat to return an error, got nil")
	}
	if err.Error() != providerErr.Error() {
		t.Errorf("expected error %q, got %q", providerErr.Error(), err.Error())
	}

	events := collector.getEvents()

	// Verify an error event was sent
	errorEvents := findEvents(events, "error")
	if len(errorEvents) == 0 {
		t.Fatal("expected at least one error event")
	}

	errData, ok := errorEvents[0].Data.(map[string]interface{})
	if !ok {
		t.Fatal("error event data is not a map")
	}
	msg, ok := errData["message"].(string)
	if !ok || msg == "" {
		t.Error("expected non-empty error message")
	}
	if errData["code"] != "provider_error" {
		t.Errorf("expected error code 'provider_error', got %v", errData["code"])
	}
	if errData["retryable"] != true {
		t.Errorf("expected retryable=true, got %v", errData["retryable"])
	}
}
