package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/glog"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	defaultMaxTokens    = 4096
)

// AnthropicProvider implements ChatModel using the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey    string
	model     string
	maxTokens int
	client    *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, model string, maxTokens int) *AnthropicProvider {
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	return &AnthropicProvider{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		client:    &http.Client{},
	}
}

// ModelName returns the model identifier.
func (p *AnthropicProvider) ModelName() string {
	return p.model
}

// anthropicRequest is the request body for the Anthropic API.
type anthropicRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system,omitempty"`
	Messages  []anthropicMsg   `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	Stream    bool             `json:"stream"`
}

type anthropicMsg struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// StreamChat sends messages to the Anthropic API and streams back events via channels.
func (p *AnthropicProvider) StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (<-chan StreamEvent, <-chan error) {
	eventCh := make(chan StreamEvent, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		// Convert messages to Anthropic format
		var anthropicMsgs []anthropicMsg
		for _, m := range messages {
			anthropicMsgs = append(anthropicMsgs, anthropicMsg{
				Role:    m.Role,
				Content: m.Content,
			})
		}

		reqBody := anthropicRequest{
			Model:     p.model,
			MaxTokens: p.maxTokens,
			System:    systemPrompt,
			Messages:  anthropicMsgs,
			Stream:    true,
		}
		if len(tools) > 0 {
			reqBody.Tools = tools
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			errCh <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(bodyBytes))
		if err != nil {
			errCh <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", p.apiKey)
		req.Header.Set("anthropic-version", anthropicAPIVersion)

		resp, err := p.client.Do(req)
		if err != nil {
			errCh <- fmt.Errorf("failed to send request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
			return
		}

		// Parse SSE stream
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var sseEvent struct {
				Type         string          `json:"type"`
				Index        int             `json:"index"`
				ContentBlock json.RawMessage `json:"content_block"`
				Delta        json.RawMessage `json:"delta"`
				Message      json.RawMessage `json:"message"`
				Usage        json.RawMessage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &sseEvent); err != nil {
				glog.Warningf("Failed to parse SSE event: %v", err)
				continue
			}

			event := StreamEvent{Type: sseEvent.Type}

			if sseEvent.ContentBlock != nil {
				var cb ContentBlock
				if err := json.Unmarshal(sseEvent.ContentBlock, &cb); err == nil {
					event.ContentBlock = &cb
				}
			}

			if sseEvent.Delta != nil {
				var d Delta
				if err := json.Unmarshal(sseEvent.Delta, &d); err == nil {
					event.Delta = &d
				}
			}

			if sseEvent.Message != nil {
				var msg MessageResponse
				if err := json.Unmarshal(sseEvent.Message, &msg); err == nil {
					event.Message = &msg
				}
			}

			if sseEvent.Usage != nil {
				var u Usage
				if err := json.Unmarshal(sseEvent.Usage, &u); err == nil {
					event.Usage = &u
				}
			}

			select {
			case eventCh <- event:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			errCh <- fmt.Errorf("SSE stream error: %w", err)
		}
	}()

	return eventCh, errCh
}
