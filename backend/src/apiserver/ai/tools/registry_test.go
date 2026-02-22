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
	"context"
	"sort"
	"testing"
)

// mockTool implements the Tool interface for testing purposes.
type mockTool struct {
	name        string
	description string
	readOnly    bool
	result      *ToolResult
	err         error
}

func (m *mockTool) Name() string                    { return m.name }
func (m *mockTool) Description() string             { return m.description }
func (m *mockTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "The resource ID",
			},
		},
	}
}
func (m *mockTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	return m.result, m.err
}
func (m *mockTool) IsReadOnly() bool { return m.readOnly }

func newReadOnlyTool() *mockTool {
	return &mockTool{
		name:        "list_items",
		description: "Lists all items",
		readOnly:    true,
		result:      &ToolResult{Content: "item1, item2", IsError: false},
	}
}

func newMutatingTool() *mockTool {
	return &mockTool{
		name:        "delete_item",
		description: "Deletes an item",
		readOnly:    false,
		result:      &ToolResult{Content: "deleted", IsError: false},
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewToolRegistry()
	ro := newReadOnlyTool()
	mut := newMutatingTool()

	reg.Register(ro)
	reg.Register(mut)

	st, ok := reg.Get("list_items")
	if !ok {
		t.Fatalf("expected Get(%q) to return true, got false", "list_items")
	}
	if st.Name() != "list_items" {
		t.Errorf("expected tool name %q, got %q", "list_items", st.Name())
	}

	st, ok = reg.Get("delete_item")
	if !ok {
		t.Fatalf("expected Get(%q) to return true, got false", "delete_item")
	}
	if st.Name() != "delete_item" {
		t.Errorf("expected tool name %q, got %q", "delete_item", st.Name())
	}
}

func TestRegistryGetNonExistent(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(newReadOnlyTool())

	_, ok := reg.Get("nonexistent_tool")
	if ok {
		t.Errorf("expected Get(%q) to return false for non-existent tool, got true", "nonexistent_tool")
	}
}

func TestRegistryListForModeAsk(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(newReadOnlyTool())
	reg.Register(newMutatingTool())

	defs := reg.ListForMode(ChatModeAsk)

	if len(defs) != 1 {
		t.Fatalf("expected 1 tool in Ask mode, got %d", len(defs))
	}
	if defs[0].Name != "list_items" {
		t.Errorf("expected tool name %q in Ask mode, got %q", "list_items", defs[0].Name)
	}
}

func TestRegistryListForModeAgent(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(newReadOnlyTool())
	reg.Register(newMutatingTool())

	defs := reg.ListForMode(ChatModeAgent)

	if len(defs) != 2 {
		t.Fatalf("expected 2 tools in Agent mode, got %d", len(defs))
	}

	names := make([]string, len(defs))
	for i, d := range defs {
		names[i] = d.Name
	}
	sort.Strings(names)

	expected := []string{"delete_item", "list_items"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected tool name %q at index %d, got %q", name, i, names[i])
		}
	}
}

func TestRegistryListToolNames(t *testing.T) {
	reg := NewToolRegistry()
	reg.Register(newReadOnlyTool())
	reg.Register(newMutatingTool())

	names := reg.ListToolNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 tool names, got %d", len(names))
	}

	sort.Strings(names)
	expected := []string{"delete_item", "list_items"}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected tool name %q at index %d, got %q", name, i, names[i])
		}
	}
}

func TestSecuredToolNeedsConfirmationReadOnly(t *testing.T) {
	st := NewSecuredTool(newReadOnlyTool())

	if st.NeedsConfirmation(ChatModeAsk) {
		t.Errorf("read-only tool should not need confirmation in Ask mode")
	}
	if st.NeedsConfirmation(ChatModeAgent) {
		t.Errorf("read-only tool should not need confirmation in Agent mode")
	}
}

func TestSecuredToolNeedsConfirmationMutating(t *testing.T) {
	st := NewSecuredTool(newMutatingTool())

	if st.NeedsConfirmation(ChatModeAsk) {
		t.Errorf("mutating tool should not need confirmation in Ask mode (it is blocked instead)")
	}
	if !st.NeedsConfirmation(ChatModeAgent) {
		t.Errorf("mutating tool should need confirmation in Agent mode")
	}
}

func TestSecuredToolIsBlockedReadOnly(t *testing.T) {
	st := NewSecuredTool(newReadOnlyTool())

	if st.IsBlocked(ChatModeAsk) {
		t.Errorf("read-only tool should not be blocked in Ask mode")
	}
	if st.IsBlocked(ChatModeAgent) {
		t.Errorf("read-only tool should not be blocked in Agent mode")
	}
}

func TestSecuredToolIsBlockedMutating(t *testing.T) {
	st := NewSecuredTool(newMutatingTool())

	if !st.IsBlocked(ChatModeAsk) {
		t.Errorf("mutating tool should be blocked in Ask mode")
	}
	if st.IsBlocked(ChatModeAgent) {
		t.Errorf("mutating tool should not be blocked in Agent mode")
	}
}

func TestSecuredToolExecuteDelegatesToInner(t *testing.T) {
	inner := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		readOnly:    true,
		result:      &ToolResult{Content: "execution result", IsError: false},
		err:         nil,
	}
	st := NewSecuredTool(inner)

	args := map[string]interface{}{"id": "abc123"}
	result, err := st.Execute(context.Background(), ChatModeAsk, args)
	if err != nil {
		t.Fatalf("expected no error from Execute, got %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result from Execute")
	}
	if result.Content != "execution result" {
		t.Errorf("expected content %q, got %q", "execution result", result.Content)
	}
	if result.IsError {
		t.Errorf("expected IsError to be false")
	}
}

func TestSecuredToolExecuteReturnsError(t *testing.T) {
	expectedErr := context.DeadlineExceeded
	inner := &mockTool{
		name:        "failing_tool",
		description: "A tool that fails",
		readOnly:    false,
		result:      nil,
		err:         expectedErr,
	}
	st := NewSecuredTool(inner)

	result, err := st.Execute(context.Background(), ChatModeAgent, map[string]interface{}{})
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result when error occurs, got %+v", result)
	}
}

func TestSecuredToolDelegatesMetadata(t *testing.T) {
	inner := newMutatingTool()
	st := NewSecuredTool(inner)

	if st.Name() != inner.Name() {
		t.Errorf("Name() mismatch: expected %q, got %q", inner.Name(), st.Name())
	}
	if st.Description() != inner.Description() {
		t.Errorf("Description() mismatch: expected %q, got %q", inner.Description(), st.Description())
	}
	if st.IsReadOnly() != inner.IsReadOnly() {
		t.Errorf("IsReadOnly() mismatch: expected %v, got %v", inner.IsReadOnly(), st.IsReadOnly())
	}

	schema := st.InputSchema()
	if schema == nil {
		t.Fatalf("expected non-nil InputSchema")
	}
	if schema["type"] != "object" {
		t.Errorf("expected InputSchema type %q, got %v", "object", schema["type"])
	}
}

func TestRegistryListForModeAskExcludesAllMutating(t *testing.T) {
	reg := NewToolRegistry()

	// Register multiple mutating tools and one read-only tool.
	reg.Register(&mockTool{name: "create_item", description: "Creates", readOnly: false, result: &ToolResult{Content: "ok"}})
	reg.Register(&mockTool{name: "update_item", description: "Updates", readOnly: false, result: &ToolResult{Content: "ok"}})
	reg.Register(&mockTool{name: "view_item", description: "Views", readOnly: true, result: &ToolResult{Content: "ok"}})

	defs := reg.ListForMode(ChatModeAsk)
	if len(defs) != 1 {
		t.Fatalf("expected 1 tool in Ask mode with multiple mutating tools registered, got %d", len(defs))
	}
	if defs[0].Name != "view_item" {
		t.Errorf("expected only read-only tool %q, got %q", "view_item", defs[0].Name)
	}
}

func TestRegistryListToolNamesEmpty(t *testing.T) {
	reg := NewToolRegistry()

	names := reg.ListToolNames()
	if len(names) != 0 {
		t.Errorf("expected 0 tool names for empty registry, got %d", len(names))
	}
}

func TestRegistryListForModeEmpty(t *testing.T) {
	reg := NewToolRegistry()

	defs := reg.ListForMode(ChatModeAgent)
	if len(defs) != 0 {
		t.Errorf("expected 0 tool definitions for empty registry, got %d", len(defs))
	}
}
