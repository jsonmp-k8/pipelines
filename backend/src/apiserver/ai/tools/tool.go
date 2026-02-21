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

package tools

import "context"

// Tool defines the interface for an AI assistant tool.
type Tool interface {
	// Name returns the unique tool name.
	Name() string
	// Description returns a human-readable description.
	Description() string
	// InputSchema returns the JSON schema for the tool's input parameters.
	InputSchema() map[string]interface{}
	// Execute runs the tool with the given JSON arguments and returns a result.
	Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error)
	// IsReadOnly returns true if the tool only reads data.
	IsReadOnly() bool
}

// ToolResult represents the output of a tool execution.
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// ChatMode represents the AI assistant interaction mode.
type ChatMode int

const (
	ChatModeAsk   ChatMode = 1
	ChatModeAgent ChatMode = 2
)
