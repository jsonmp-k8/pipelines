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

import (
	"sync"

	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/provider"
)

// ToolRegistry manages available tools and provides mode-filtered access.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*SecuredTool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*SecuredTool),
	}
}

// Register adds a tool to the registry, wrapping it with SecuredTool.
func (r *ToolRegistry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Name()] = NewSecuredTool(tool)
}

// Get retrieves a secured tool by name.
func (r *ToolRegistry) Get(name string) (*SecuredTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// ListForMode returns tool definitions filtered by the chat mode.
// In Ask mode, only read-only tools are returned.
// In Agent mode, all tools are returned.
func (r *ToolRegistry) ListForMode(mode ChatMode) []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []provider.ToolDefinition
	for _, st := range r.tools {
		if mode == ChatModeAsk && !st.IsReadOnly() {
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Name:        st.Name(),
			Description: st.Description(),
			InputSchema: st.InputSchema(),
		})
	}
	return defs
}

// ListToolNames returns all registered tool names.
func (r *ToolRegistry) ListToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
