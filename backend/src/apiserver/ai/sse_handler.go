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
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
	"github.com/kubeflow/pipelines/backend/src/apiserver/common"
)

const (
	// maxRequestBodySize limits request body to 1MB to prevent DoS.
	maxRequestBodySize = 1 << 20

	// rateLimitWindow is the time window for rate limiting.
	rateLimitWindow = time.Minute

	// rateLimitMaxRequests is the maximum number of requests per user per window.
	rateLimitMaxRequests = 20
)

// rateLimiter provides per-user rate limiting for AI endpoints.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
	}
}

// allow checks if a request is allowed for the given user key.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateLimitWindow)

	// Remove old entries
	entries := rl.requests[key]
	start := 0
	for start < len(entries) && entries[start].Before(cutoff) {
		start++
	}
	entries = entries[start:]

	if len(entries) >= rateLimitMaxRequests {
		rl.requests[key] = entries
		return false
	}

	rl.requests[key] = append(entries, now)
	return true
}

// globalRateLimiter is shared across all AI handlers.
var globalRateLimiter = newRateLimiter()

// requireAuth checks that the user is authenticated in multi-user mode.
// Returns the userID and true if OK, or writes an HTTP error and returns false.
func requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !common.IsMultiUserMode() {
		return "", true
	}
	userID := extractUserID(r)
	if userID == "" {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return "", false
	}
	return userID, true
}

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
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req ChatRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		userID := extractUserID(r)
		req.UserID = userID

		// Authenticate in multi-user mode
		if common.IsMultiUserMode() && userID == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// Per-user rate limiting
		rateLimitKey := userID
		if rateLimitKey == "" {
			rateLimitKey = r.RemoteAddr
		}
		if !globalRateLimiter.allow(rateLimitKey) {
			http.Error(w, "Rate limit exceeded. Please wait before sending another message.", http.StatusTooManyRequests)
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
			// Send sanitized error event to client
			errEvent := ChatResponseEvent{
				Type: "error",
				Data: map[string]interface{}{
					"message":   "An error occurred processing your request. Please try again.",
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
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req ApproveToolCallRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		userID := extractUserID(r)
		if common.IsMultiUserMode() {
			if userID == "" {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			if err := aiServer.ValidateSessionOwner(req.SessionID, userID); err != nil {
				http.Error(w, fmt.Sprintf("Unauthorized: %v", err), http.StatusForbidden)
				return
			}
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

		// Auth check
		userID, ok := requireAuth(w, r)
		if !ok {
			return
		}
		_ = userID

		// Rate limiting
		rateLimitKey := userID
		if rateLimitKey == "" {
			rateLimitKey = r.RemoteAddr
		}
		if !globalRateLimiter.allow(rateLimitKey) {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var req struct {
			PipelineID        string `json:"pipeline_id"`
			PipelineVersionID string `json:"pipeline_version_id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Fetch pipeline spec to ground the documentation
		pipelineID := req.PipelineID
		if pipelineID == "" {
			http.Error(w, "pipeline_id is required", http.StatusBadRequest)
			return
		}

		specContext := ""
		// Try to get pipeline metadata
		if pipeline, err := aiServer.contextBuilder.GetResourceManager().GetPipeline(pipelineID); err == nil {
			specContext += fmt.Sprintf("Pipeline Name: %s\nDescription: %s\nNamespace: %s\n\n",
				pipeline.Name, pipeline.Description, pipeline.Namespace)
		}

		// Try to get the actual pipeline spec/template
		versionID := req.PipelineVersionID
		if versionID != "" {
			if templateBytes, err := aiServer.contextBuilder.GetResourceManager().GetPipelineVersionTemplate(versionID); err == nil {
				specContext += fmt.Sprintf("Pipeline Spec (version %s):\n```json\n%s\n```\n", versionID, string(templateBytes))
			}
		} else {
			if templateBytes, err := aiServer.contextBuilder.GetResourceManager().GetPipelineLatestTemplate(pipelineID); err == nil {
				specContext += fmt.Sprintf("Pipeline Spec (latest version):\n```json\n%s\n```\n", string(templateBytes))
			}
		}

		prompt := fmt.Sprintf("Generate comprehensive documentation for the following pipeline. Include an overview, description of each component/step, input parameters, output artifacts, and usage examples.\n\n%s", specContext)

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

		// Auth check
		if _, ok := requireAuth(w, r); !ok {
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

		// Auth check
		if _, ok := requireAuth(w, r); !ok {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}

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

// extractUserID extracts the user identity from HTTP request headers.
// It checks the Kubeflow user ID header (configurable, defaults to x-goog-authenticated-user-email).
// Returns empty string if not in multi-user mode or no header present.
func extractUserID(r *http.Request) string {
	if !common.IsMultiUserMode() {
		return ""
	}
	header := common.GetKubeflowUserIDHeader()
	prefix := common.GetKubeflowUserIDPrefix()
	value := r.Header.Get(header)
	if value == "" {
		return ""
	}
	return strings.TrimPrefix(value, prefix)
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return "session-" + hex.EncodeToString(b)
}
