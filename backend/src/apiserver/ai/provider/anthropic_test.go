package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// redirectTransport rewrites every outgoing request so that it targets the
// httptest server instead of the real Anthropic API.
type redirectTransport struct {
	base      http.RoundTripper
	targetURL string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := url.Parse(t.targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target URL: %w", err)
	}
	req.URL.Scheme = u.Scheme
	req.URL.Host = u.Host
	return t.base.RoundTrip(req)
}

// newTestProvider creates an AnthropicProvider whose HTTP client is redirected
// to the supplied httptest server.
func newTestProvider(ts *httptest.Server, apiKey, model string, maxTokens int) *AnthropicProvider {
	p := NewAnthropicProvider(apiKey, model, maxTokens)
	p.client = &http.Client{
		Transport: &redirectTransport{
			base:      http.DefaultTransport,
			targetURL: ts.URL,
		},
	}
	return p
}

// collectEvents drains both channels returned by StreamChat and returns the
// collected events and the first error (if any).
func collectEvents(eventCh <-chan StreamEvent, errCh <-chan error) ([]StreamEvent, error) {
	var events []StreamEvent
	var firstErr error
	for eventCh != nil || errCh != nil {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				eventCh = nil
				continue
			}
			events = append(events, ev)
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return events, firstErr
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestModelName(t *testing.T) {
	p := NewAnthropicProvider("key", "claude-sonnet-4-20250514", 1024)
	if got := p.ModelName(); got != "claude-sonnet-4-20250514" {
		t.Errorf("ModelName() = %q, want %q", got, "claude-sonnet-4-20250514")
	}
}

func TestNewAnthropicProvider_DefaultMaxTokens(t *testing.T) {
	p := NewAnthropicProvider("key", "model", 0)
	if p.maxTokens != 4096 {
		t.Errorf("maxTokens = %d, want %d (defaultMaxTokens)", p.maxTokens, 4096)
	}
}

func TestStreamChat_TextResponse(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","role":"assistant"}}`,
		"",
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		"",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
		"",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
		"",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer ts.Close()

	p := newTestProvider(ts, "test-key", "claude-sonnet-4-20250514", 1024)
	ctx := context.Background()
	eventCh, errCh := p.StreamChat(ctx, []Message{{Role: "user", Content: "Hi"}}, nil, "")

	events, err := collectEvents(eventCh, errCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We expect 7 events in order.
	expectedTypes := []string{
		"message_start",
		"content_block_start",
		"content_block_delta",
		"content_block_delta",
		"content_block_stop",
		"message_delta",
		"message_stop",
	}
	if len(events) != len(expectedTypes) {
		t.Fatalf("got %d events, want %d", len(events), len(expectedTypes))
	}
	for i, want := range expectedTypes {
		if events[i].Type != want {
			t.Errorf("events[%d].Type = %q, want %q", i, events[i].Type, want)
		}
	}

	// Verify message_start carries the message.
	if events[0].Message == nil {
		t.Fatal("message_start event missing Message field")
	}
	if events[0].Message.ID != "msg_1" {
		t.Errorf("message id = %q, want %q", events[0].Message.ID, "msg_1")
	}
	if events[0].Message.Role != "assistant" {
		t.Errorf("message role = %q, want %q", events[0].Message.Role, "assistant")
	}

	// Verify content_block_start.
	if events[1].ContentBlock == nil {
		t.Fatal("content_block_start event missing ContentBlock")
	}
	if events[1].ContentBlock.Type != "text" {
		t.Errorf("content_block type = %q, want %q", events[1].ContentBlock.Type, "text")
	}

	// Verify text deltas.
	if events[2].Delta == nil || events[2].Delta.Text != "Hello" {
		t.Errorf("first text delta = %v, want text='Hello'", events[2].Delta)
	}
	if events[3].Delta == nil || events[3].Delta.Text != " world" {
		t.Errorf("second text delta = %v, want text=' world'", events[3].Delta)
	}

	// Verify message_delta stop_reason.
	if events[5].Delta == nil || events[5].Delta.StopReason != "end_turn" {
		t.Errorf("message_delta stop_reason = %v, want 'end_turn'", events[5].Delta)
	}
}

func TestStreamChat_ToolUse(t *testing.T) {
	ssePayload := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_run"}}`,
		"",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"run"}}`,
		"",
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"_id\":\"abc\"}"}}`,
		"",
		`data: {"type":"content_block_stop","index":0}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		"",
		`data: {"type":"message_stop"}`,
		"",
	}, "\n")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ssePayload)
	}))
	defer ts.Close()

	p := newTestProvider(ts, "test-key", "claude-sonnet-4-20250514", 1024)
	ctx := context.Background()
	eventCh, errCh := p.StreamChat(ctx, []Message{{Role: "user", Content: "run it"}}, nil, "")

	events, err := collectEvents(eventCh, errCh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 6 {
		t.Fatalf("got %d events, want 6", len(events))
	}

	// content_block_start should carry tool_use content block.
	cb := events[0].ContentBlock
	if cb == nil {
		t.Fatal("content_block_start missing ContentBlock")
	}
	if cb.Type != "tool_use" {
		t.Errorf("content_block type = %q, want %q", cb.Type, "tool_use")
	}
	if cb.ID != "toolu_1" {
		t.Errorf("content_block id = %q, want %q", cb.ID, "toolu_1")
	}
	if cb.Name != "get_run" {
		t.Errorf("content_block name = %q, want %q", cb.Name, "get_run")
	}

	// Verify partial_json deltas.
	if events[1].Delta == nil || events[1].Delta.PartialJSON != `{"run` {
		t.Errorf("first partial_json = %q, want %q", events[1].Delta.PartialJSON, `{"run`)
	}
	if events[2].Delta == nil || events[2].Delta.PartialJSON != `_id":"abc"}` {
		t.Errorf("second partial_json = %q, want %q", events[2].Delta.PartialJSON, `_id":"abc"}`)
	}

	// message_delta stop_reason should be tool_use.
	if events[4].Delta == nil || events[4].Delta.StopReason != "tool_use" {
		t.Errorf("message_delta stop_reason = %v, want 'tool_use'", events[4].Delta)
	}
}

func TestStreamChat_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"max_tokens: must be positive"}}`)
	}))
	defer ts.Close()

	p := newTestProvider(ts, "test-key", "claude-sonnet-4-20250514", 1024)
	ctx := context.Background()
	eventCh, errCh := p.StreamChat(ctx, []Message{{Role: "user", Content: "Hi"}}, nil, "")

	_, err := collectEvents(eventCh, errCh)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should contain status code 400, got: %v", err)
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("error should contain sanitized status, got: %v", err)
	}
}

func TestStreamChat_ContextCancellation(t *testing.T) {
	// The server writes one event, then blocks until the handler's context
	// is done (which happens when the client disconnects after our cancel).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write one event so the client starts reading.
		fmt.Fprintln(w, `data: {"type":"message_start","message":{"id":"msg_1","role":"assistant"}}`)
		fmt.Fprintln(w, "")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Block until the client disconnects.
		<-r.Context().Done()
	}))
	defer ts.Close()

	p := newTestProvider(ts, "test-key", "claude-sonnet-4-20250514", 1024)
	ctx, cancel := context.WithCancel(context.Background())
	eventCh, errCh := p.StreamChat(ctx, []Message{{Role: "user", Content: "Hi"}}, nil, "")

	// Read first event to confirm streaming started.
	select {
	case ev, ok := <-eventCh:
		if !ok {
			t.Fatal("eventCh closed before receiving first event")
		}
		if ev.Type != "message_start" {
			t.Errorf("first event type = %q, want %q", ev.Type, "message_start")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first event")
	}

	// Cancel context.
	cancel()

	// Both channels should close within a reasonable time.
	timeout := time.After(5 * time.Second)
	eventDrained := false
	errDrained := false
	for !eventDrained || !errDrained {
		select {
		case _, ok := <-eventCh:
			if !ok {
				eventDrained = true
			}
		case _, ok := <-errCh:
			if !ok {
				errDrained = true
			}
		case <-timeout:
			t.Fatal("timed out waiting for channels to close after context cancellation")
		}
	}
}

func TestStreamChat_VerifyRequestHeaders(t *testing.T) {
	var capturedHeaders http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `data: {"type":"message_stop"}`)
	}))
	defer ts.Close()

	p := newTestProvider(ts, "sk-ant-test-key-123", "claude-sonnet-4-20250514", 2048)
	ctx := context.Background()
	eventCh, errCh := p.StreamChat(ctx, []Message{{Role: "user", Content: "Hi"}}, nil, "You are helpful.")

	// Drain channels so the goroutine completes.
	_, _ = collectEvents(eventCh, errCh)

	if capturedHeaders == nil {
		t.Fatal("handler was never called; no headers captured")
	}

	// Content-Type
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}

	// X-API-Key
	if got := capturedHeaders.Get("X-API-Key"); got != "sk-ant-test-key-123" {
		t.Errorf("X-API-Key = %q, want %q", got, "sk-ant-test-key-123")
	}

	// anthropic-version
	if got := capturedHeaders.Get("anthropic-version"); got != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want %q", got, "2023-06-01")
	}
}
