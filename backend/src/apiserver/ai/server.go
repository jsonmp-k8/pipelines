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
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	aicontext "github.com/kubeflow/pipelines/backend/src/apiserver/ai/context"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/provider"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/rules"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/session"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

const (
	maxAgenticLoopIterations = 20

	// confirmationTimeout is the maximum time to wait for a user to approve/deny
	// a mutating tool call before timing out. This prevents goroutine leaks.
	confirmationTimeout = 10 * time.Minute
)

// ChatRequest represents an incoming chat request.
type ChatRequest struct {
	Message     string                `json:"message"`
	SessionID   string                `json:"session_id"`
	Mode        int                   `json:"mode"` // 1=Ask, 2=Agent
	PageContext *aicontext.PageContext `json:"page_context,omitempty"`
	UserID      string                `json:"-"` // Set by server, not from JSON
}

// ChatResponseEvent represents a streaming response event.
type ChatResponseEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ApproveToolCallRequest represents a tool call approval request.
type ApproveToolCallRequest struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	Approved   bool   `json:"approved"`
}

// AIServer implements the AI service logic.
type AIServer struct {
	chatModel      provider.ChatModel
	toolRegistry   *tools.ToolRegistry
	sessionManager *session.SessionManager
	contextBuilder *aicontext.ContextBuilder
	ruleManager    *rules.RuleManager
}

// NewAIServer creates a new AI server.
func NewAIServer(
	chatModel provider.ChatModel,
	toolRegistry *tools.ToolRegistry,
	sessionManager *session.SessionManager,
	contextBuilder *aicontext.ContextBuilder,
	ruleManager *rules.RuleManager,
) *AIServer {
	return &AIServer{
		chatModel:      chatModel,
		toolRegistry:   toolRegistry,
		sessionManager: sessionManager,
		contextBuilder: contextBuilder,
		ruleManager:    ruleManager,
	}
}

// StreamChat runs the agentic chat loop, streaming events to the provided callback.
// It collects tool calls inline during streaming, executes them, and loops until
// the LLM produces a final text response.
func (s *AIServer) StreamChat(ctx context.Context, req *ChatRequest, sendEvent func(ChatResponseEvent)) error {
	mode := tools.ChatMode(req.Mode)
	// Validate mode: only Ask and Agent are valid. Default unknown values to Ask (safe).
	if mode != tools.ChatModeAsk && mode != tools.ChatModeAgent {
		mode = tools.ChatModeAsk
	}

	sess := s.sessionManager.GetOrCreate(req.SessionID, mode, req.UserID)

	sendEvent(ChatResponseEvent{
		Type: "session_metadata",
		Data: map[string]interface{}{
			"session_id":      sess.ID,
			"model":           s.chatModel.ModelName(),
			"available_tools": s.toolRegistry.ListToolNames(),
		},
	})

	rulesContent := s.ruleManager.GetActiveRulesContent()
	systemPrompt := s.contextBuilder.BuildSystemPrompt(ctx, req.PageContext, rulesContent)

	if err := s.sessionManager.AddMessage(sess.ID, provider.Message{
		Role:    "user",
		Content: req.Message,
	}); err != nil {
		glog.Warningf("Failed to add user message to session %s: %v", sess.ID, err)
	}

	toolDefs := s.toolRegistry.ListForMode(mode)

	for iteration := 0; iteration < maxAgenticLoopIterations; iteration++ {
		sendEvent(ChatResponseEvent{
			Type: "progress",
			Data: map[string]interface{}{
				"message":    "Thinking...",
				"percentage": -1,
			},
		})

		// Take a snapshot of messages to avoid data races with concurrent appends.
		messages, err := s.sessionManager.GetMessages(sess.ID)
		if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		}

		eventCh, errCh := s.chatModel.StreamChat(ctx, messages, toolDefs, systemPrompt)

		var textContent strings.Builder
		var currentToolCall *provider.ContentBlock
		var toolCalls []provider.ContentBlock
		var toolCallJSON strings.Builder
		var stopReason string

		for event := range eventCh {
			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil {
					switch event.ContentBlock.Type {
					case "text":
						// Text block starting
					case "tool_use":
						currentToolCall = &provider.ContentBlock{
							Type: "tool_use",
							ID:   event.ContentBlock.ID,
							Name: event.ContentBlock.Name,
						}
						toolCallJSON.Reset()
						sendEvent(ChatResponseEvent{
							Type: "tool_call",
							Data: map[string]interface{}{
								"tool_call_id": event.ContentBlock.ID,
								"tool_name":    event.ContentBlock.Name,
								"read_only":    s.isToolReadOnly(event.ContentBlock.Name),
							},
						})
					}
				}

			case "content_block_delta":
				if event.Delta != nil {
					switch event.Delta.Type {
					case "text_delta":
						if event.Delta.Text != "" {
							textContent.WriteString(event.Delta.Text)
							sendEvent(ChatResponseEvent{
								Type: "markdown_chunk",
								Data: map[string]interface{}{
									"content": event.Delta.Text,
								},
							})
						}
					case "input_json_delta":
						if event.Delta.PartialJSON != "" {
							toolCallJSON.WriteString(event.Delta.PartialJSON)
						}
					}
				}

			case "content_block_stop":
				if currentToolCall != nil {
					// Parse accumulated JSON input
					var input interface{}
					if toolCallJSON.Len() > 0 {
						if err := json.Unmarshal([]byte(toolCallJSON.String()), &input); err != nil {
							glog.Warningf("Failed to parse tool call input JSON: %v", err)
							input = map[string]interface{}{}
						}
					} else {
						input = map[string]interface{}{}
					}
					currentToolCall.Input = input

					argsJSON, err := json.Marshal(input)
					if err != nil {
						argsJSON = []byte("{}")
					}
					sendEvent(ChatResponseEvent{
						Type: "tool_call",
						Data: map[string]interface{}{
							"tool_call_id":   currentToolCall.ID,
							"tool_name":      currentToolCall.Name,
							"arguments_json": string(argsJSON),
							"read_only":      s.isToolReadOnly(currentToolCall.Name),
						},
					})

					toolCalls = append(toolCalls, *currentToolCall)
					currentToolCall = nil
				}

			case "message_delta":
				if event.Delta != nil && event.Delta.StopReason != "" {
					stopReason = event.Delta.StopReason
				}
			}
		}

		// After eventCh is closed, the provider goroutine is done.
		// Do a blocking read on errCh to catch any error that was sent.
		if err := <-errCh; err != nil {
			// Log full error server-side; send sanitized message to client.
			glog.Errorf("AI provider error: %v", err)
			sendEvent(ChatResponseEvent{
				Type: "error",
				Data: map[string]interface{}{
					"message":   "An error occurred communicating with the AI provider. Please try again.",
					"code":      "provider_error",
					"retryable": true,
				},
			})
			return err
		}

		// Build assistant message
		var contentBlocks []provider.ContentBlock
		if textContent.Len() > 0 {
			contentBlocks = append(contentBlocks, provider.ContentBlock{
				Type: "text",
				Text: textContent.String(),
			})
		}
		contentBlocks = append(contentBlocks, toolCalls...)

		if len(contentBlocks) > 0 {
			if err := s.sessionManager.AddMessage(sess.ID, provider.Message{
				Role:    "assistant",
				Content: contentBlocks,
			}); err != nil {
				glog.Warningf("Failed to add assistant message to session %s: %v", sess.ID, err)
			}
		}

		if stopReason != "tool_use" || len(toolCalls) == 0 {
			break
		}

		// Execute tool calls
		var toolResults []provider.ContentBlock
		for _, tc := range toolCalls {
			result, err := s.executeToolCall(ctx, sess, mode, tc, sendEvent)
			if err != nil {
				toolResults = append(toolResults, provider.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   fmt.Sprintf("Error: %v", err),
					IsError:   true,
				})
			} else {
				toolResults = append(toolResults, provider.ContentBlock{
					Type:      "tool_result",
					ToolUseID: tc.ID,
					Content:   result.Content,
					IsError:   result.IsError,
				})
			}
		}

		if err := s.sessionManager.AddMessage(sess.ID, provider.Message{
			Role:    "user",
			Content: toolResults,
		}); err != nil {
			glog.Warningf("Failed to add tool results to session %s: %v", sess.ID, err)
		}
	}

	return nil
}

func (s *AIServer) executeToolCall(
	ctx context.Context,
	sess *session.Session,
	mode tools.ChatMode,
	tc provider.ContentBlock,
	sendEvent func(ChatResponseEvent),
) (*tools.ToolResult, error) {
	securedTool, ok := s.toolRegistry.Get(tc.Name)
	if !ok {
		return &tools.ToolResult{
			Content: fmt.Sprintf("Unknown tool: %s", tc.Name),
			IsError: true,
		}, nil
	}

	// Check if tool is blocked in current mode
	if securedTool.IsBlocked(mode) {
		return &tools.ToolResult{
			Content: fmt.Sprintf("Tool %s is not available in Ask mode. Switch to Agent mode to use mutating tools.", tc.Name),
			IsError: true,
		}, nil
	}

	// Check if confirmation is needed
	if securedTool.NeedsConfirmation(mode) {
		argsJSON, err := json.Marshal(tc.Input)
		if err != nil {
			argsJSON = []byte("{}")
		}

		// Send confirmation request
		sendEvent(ChatResponseEvent{
			Type: "confirmation_request",
			Data: map[string]interface{}{
				"tool_call_id":   tc.ID,
				"tool_name":      tc.Name,
				"description":    fmt.Sprintf("Execute %s with the provided arguments", tc.Name),
				"arguments_json": string(argsJSON),
			},
		})

		// Wait for confirmation
		pending := &session.PendingToolCall{
			ToolCallID: tc.ID,
			ToolName:   tc.Name,
			ResultCh:   make(chan session.ToolCallDecision, 1),
		}
		if args, ok := tc.Input.(map[string]interface{}); ok {
			pending.Arguments = args
		}

		if err := s.sessionManager.AddPendingConfirmation(sess.ID, pending); err != nil {
			return nil, fmt.Errorf("failed to add pending confirmation: %w", err)
		}

		// Block until user responds, context is cancelled, or timeout expires.
		// The timeout prevents goroutine leaks if the user never responds.
		select {
		case decision := <-pending.ResultCh:
			if !decision.Approved {
				return &tools.ToolResult{
					Content: fmt.Sprintf("Tool call %s was denied by the user.", tc.Name),
					IsError: true,
				}, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(confirmationTimeout):
			return &tools.ToolResult{
				Content: fmt.Sprintf("Tool call %s timed out waiting for user confirmation.", tc.Name),
				IsError: true,
			}, nil
		}
	}

	// Execute the tool with mode enforcement (defense-in-depth)
	args, ok := tc.Input.(map[string]interface{})
	if !ok {
		args = map[string]interface{}{}
	}

	sendEvent(ChatResponseEvent{
		Type: "progress",
		Data: map[string]interface{}{
			"message":    fmt.Sprintf("Executing %s...", tc.Name),
			"percentage": -1,
		},
	})

	result, err := securedTool.Execute(ctx, mode, args)
	if err != nil {
		result = &tools.ToolResult{
			Content: fmt.Sprintf("Tool execution error: %v", err),
			IsError: true,
		}
	}

	// Send tool result event
	sendEvent(ChatResponseEvent{
		Type: "tool_result",
		Data: map[string]interface{}{
			"tool_call_id": tc.ID,
			"result_json":  result.Content,
			"success":      !result.IsError,
		},
	})

	return result, nil
}

func (s *AIServer) isToolReadOnly(name string) bool {
	tool, ok := s.toolRegistry.Get(name)
	if !ok {
		return false
	}
	return tool.IsReadOnly()
}

// ApproveToolCall resolves a pending tool call confirmation.
func (s *AIServer) ApproveToolCall(req *ApproveToolCallRequest) error {
	return s.sessionManager.ResolveConfirmation(req.SessionID, req.ToolCallID, req.Approved)
}

// ValidateSessionOwner checks that the given userID matches the session's owner.
func (s *AIServer) ValidateSessionOwner(sessionID, userID string) error {
	return s.sessionManager.ValidateSessionOwner(sessionID, userID)
}

// ListRules returns all available rules.
func (s *AIServer) ListRules() []*rules.Rule {
	return s.ruleManager.ListRules()
}

// ToggleRule enables or disables a rule.
func (s *AIServer) ToggleRule(ruleID string, enabled bool) (*rules.Rule, error) {
	return s.ruleManager.ToggleRule(ruleID, enabled)
}

// GetModelName returns the configured model name.
func (s *AIServer) GetModelName() string {
	return s.chatModel.ModelName()
}

// GetToolNames returns the list of registered tool names.
func (s *AIServer) GetToolNames() []string {
	return s.toolRegistry.ListToolNames()
}
