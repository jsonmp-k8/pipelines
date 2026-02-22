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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aicontext "github.com/kubeflow/pipelines/backend/src/apiserver/ai/context"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/rules"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/session"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

// newMinimalAIServer creates an AIServer with no chat model for handler tests
// that do not invoke StreamChat.
func newMinimalAIServer() *AIServer {
	sm := session.NewSessionManager(context.Background())
	rm := rules.NewRuleManager()
	cb := aicontext.NewContextBuilder(nil)
	reg := tools.NewToolRegistry()
	return NewAIServer(nil, reg, sm, cb, rm)
}

func TestSSEHandler_MethodNotAllowed(t *testing.T) {
	server := newMinimalAIServer()
	handler := NewSSEHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/apis/v2beta1/ai/chat/stream", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestSSEHandler_EmptyMessage(t *testing.T) {
	server := newMinimalAIServer()
	handler := NewSSEHandler(server)

	body := `{"message": "", "session_id": "s1", "mode": 1}`
	req := httptest.NewRequest(http.MethodPost, "/apis/v2beta1/ai/chat/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	respBody := strings.TrimSpace(w.Body.String())
	if !strings.Contains(respBody, "message is required") {
		t.Errorf("expected error body to contain 'message is required', got %q", respBody)
	}
}

func TestSSEHandler_RequestTooLarge(t *testing.T) {
	server := newMinimalAIServer()
	handler := NewSSEHandler(server)

	// Create a body larger than 1MB (maxRequestBodySize = 1 << 20 = 1048576)
	largeBody := strings.Repeat("x", 1<<20+1)
	req := httptest.NewRequest(http.MethodPost, "/apis/v2beta1/ai/chat/stream", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, w.Code)
	}
}

func TestApproveHandler_MethodNotAllowed(t *testing.T) {
	server := newMinimalAIServer()
	handler := NewApproveHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/apis/v2beta1/ai/approve", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestApproveHandler_InvalidJSON(t *testing.T) {
	server := newMinimalAIServer()
	handler := NewApproveHandler(server)

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/apis/v2beta1/ai/approve", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if id1 == id2 {
		t.Errorf("expected two different session IDs, but both were %q", id1)
	}

	if !strings.HasPrefix(id1, "session-") {
		t.Errorf("expected session ID to start with 'session-', got %q", id1)
	}
	if !strings.HasPrefix(id2, "session-") {
		t.Errorf("expected session ID to start with 'session-', got %q", id2)
	}
}

func TestListRulesHandler(t *testing.T) {
	sm := session.NewSessionManager(context.Background())
	rm := rules.NewRuleManager()
	cb := aicontext.NewContextBuilder(nil)
	reg := tools.NewToolRegistry()

	// We cannot call rm.LoadRules with a real directory easily in tests,
	// but NewRuleManager starts with an empty rule set, so ListRules returns [].
	server := NewAIServer(nil, reg, sm, cb, rm)
	handler := NewListRulesHandler(server)

	req := httptest.NewRequest(http.MethodGet, "/apis/v2beta1/ai/rules", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", contentType)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	rulesVal, ok := result["rules"]
	if !ok {
		t.Fatal("expected 'rules' key in response JSON")
	}

	// Rules should be an array (possibly empty)
	rulesArr, ok := rulesVal.([]interface{})
	if !ok {
		t.Fatalf("expected 'rules' to be an array, got %T", rulesVal)
	}

	// With no rules loaded, the array should be empty
	if len(rulesArr) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rulesArr))
	}
}
