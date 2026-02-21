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

// SecuredTool wraps a Tool and enforces confirmation for mutating operations.
type SecuredTool struct {
	inner Tool
}

// NewSecuredTool wraps a tool with security enforcement.
func NewSecuredTool(tool Tool) *SecuredTool {
	return &SecuredTool{inner: tool}
}

func (s *SecuredTool) Name() string                    { return s.inner.Name() }
func (s *SecuredTool) Description() string             { return s.inner.Description() }
func (s *SecuredTool) InputSchema() map[string]interface{} { return s.inner.InputSchema() }
func (s *SecuredTool) IsReadOnly() bool                { return s.inner.IsReadOnly() }

func (s *SecuredTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return s.inner.Execute(ctx, args)
}

// NeedsConfirmation returns true if the tool requires user confirmation in the given mode.
// In Ask mode, mutating tools are blocked entirely.
// In Agent mode, mutating tools require explicit confirmation.
func (s *SecuredTool) NeedsConfirmation(mode ChatMode) bool {
	if s.inner.IsReadOnly() {
		return false
	}
	return mode == ChatModeAgent
}

// IsBlocked returns true if the tool cannot be executed in the given mode.
// Mutating tools are blocked in Ask mode.
func (s *SecuredTool) IsBlocked(mode ChatMode) bool {
	if s.inner.IsReadOnly() {
		return false
	}
	return mode == ChatModeAsk
}
