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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

const (
	// maxRequestBodySize limits request body to 1MB to prevent DoS.
	maxRequestBodySize = 1 << 20
)

// NewSSEHandler creates an HTTP handler for streaming AI chat via Server-Sent Events.
// Registered at POST /apis/v2beta1/ai/chat/stream
func NewSSEHandler(aiServer *AIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request body with size limit
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()

		var req ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		// Generate session ID if not provided
		if req.SessionID == "" {
			req.SessionID = generateSessionID()
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send events callback
		sendEvent := func(event ChatResponseEvent) {
			data, err := json.Marshal(event)
			if err != nil {
				glog.Warningf("Failed to marshal SSE event: %v", err)
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}

		// Run the chat
		ctx := r.Context()
		if err := aiServer.StreamChat(ctx, &req, sendEvent); err != nil {
			glog.Errorf("Chat error: %v", err)
			// Try to send error event if connection is still open
			errEvent := ChatResponseEvent{
				Type: "error",
				Data: map[string]interface{}{
					"message":   err.Error(),
					"code":      "internal_error",
					"retryable": false,
				},
			}
			data, _ := json.Marshal(errEvent)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}

		// Send done event
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}
}

// NewApproveHandler creates an HTTP handler for approving/denying tool calls.
// Registered at POST /apis/v2beta1/ai/approve
func NewApproveHandler(aiServer *AIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()

		var req ApproveToolCallRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		if err := aiServer.ApproveToolCall(&req); err != nil {
			http.Error(w, fmt.Sprintf("Failed to process approval: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

// NewGenerateDocsHandler creates an HTTP handler for generating pipeline documentation.
// Registered at POST /apis/v2beta1/ai/generate-docs
func NewGenerateDocsHandler(aiServer *AIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()

		var req struct {
			PipelineID        string `json:"pipeline_id"`
			PipelineVersionID string `json:"pipeline_version_id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		// Single-shot LLM call for doc generation
		ctx := r.Context()
		prompt := fmt.Sprintf("Generate comprehensive documentation for pipeline %s.", req.PipelineID)

		chatReq := &ChatRequest{
			Message:   prompt,
			SessionID: generateSessionID(),
			Mode:      int(tools.ChatModeAsk),
		}

		var markdown string
		sendEvent := func(event ChatResponseEvent) {
			if event.Type == "markdown_chunk" {
				if data, ok := event.Data.(map[string]interface{}); ok {
					if content, ok := data["content"].(string); ok {
						markdown += content
					}
				}
			}
		}

		if err := aiServer.StreamChat(ctx, chatReq, sendEvent); err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate docs: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"documentation_markdown": markdown,
		})
	}
}

// NewListRulesHandler creates an HTTP handler for listing AI rules.
// Registered at GET /apis/v2beta1/ai/rules
func NewListRulesHandler(aiServer *AIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rules := aiServer.ListRules()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rules": rules,
		})
	}
}

// NewToggleRuleHandler creates an HTTP handler for toggling AI rules.
// Registered at POST /apis/v2beta1/ai/rules/{rule_id}:toggle
func NewToggleRuleHandler(aiServer *AIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		defer r.Body.Close()

		var req struct {
			RuleID  string `json:"rule_id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		rule, err := aiServer.ToggleRule(req.RuleID, req.Enabled)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to toggle rule: %v", err), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rule": rule,
		})
	}
}

func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("session-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}
