package provider

import "context"

// Message represents a chat message.
type Message struct {
	Role    string      `json:"role"`    // "user", "assistant", "system"
	Content interface{} `json:"content"` // string or []ContentBlock
}

// ContentBlock represents a content block in a message (for tool use).
type ContentBlock struct {
	Type      string      `json:"type"` // "text", "tool_use", "tool_result"
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`
}

// ToolDefinition describes a tool available to the LLM.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// StreamEvent represents a streaming event from the provider.
type StreamEvent struct {
	Type         string // "content_block_start", "content_block_delta", "content_block_stop", "message_start", "message_delta", "message_stop"
	ContentBlock *ContentBlock
	Delta        *Delta
	Message      *MessageResponse
	Usage        *Usage
}

// Delta represents an incremental update.
type Delta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// Usage tracks token usage.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// MessageResponse is the full message response.
type MessageResponse struct {
	ID         string         `json:"id"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
	Model      string         `json:"model"`
}

// ChatModel is the interface for chat-based LLM providers.
type ChatModel interface {
	// StreamChat sends messages and streams back events via a channel.
	StreamChat(ctx context.Context, messages []Message, tools []ToolDefinition, systemPrompt string) (<-chan StreamEvent, <-chan error)
	// ModelName returns the model identifier.
	ModelName() string
}
