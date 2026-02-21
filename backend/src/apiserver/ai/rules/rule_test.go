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

package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRuleManager_StartsEmpty(t *testing.T) {
	rm := NewRuleManager()
	if rm == nil {
		t.Fatal("expected non-nil RuleManager")
	}
	rules := rm.ListRules()
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules, got %d", len(rules))
	}
}

func TestLoadRules_EmptyDirPath_IsNoOp(t *testing.T) {
	rm := NewRuleManager()
	err := rm.LoadRules("")
	if err != nil {
		t.Fatalf("expected nil error for empty dir path, got %v", err)
	}
	if len(rm.ListRules()) != 0 {
		t.Fatal("expected no rules after loading empty dir path")
	}
}

func TestLoadRules_NonExistentDir_ReturnsNil(t *testing.T) {
	rm := NewRuleManager()
	err := rm.LoadRules("/nonexistent/path/that/should/not/exist")
	if err != nil {
		t.Fatalf("expected nil error for non-existent dir, got %v", err)
	}
	if len(rm.ListRules()) != 0 {
		t.Fatal("expected no rules after loading non-existent dir")
	}
}

func TestLoadRules_LoadsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()

	// File with a heading line
	content1 := "# My Rule\nThis is the description.\n\nSome body content here."
	if err := os.WriteFile(filepath.Join(dir, "my-rule.md"), []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	// File without a heading line
	content2 := "Just plain content\nwith multiple lines."
	if err := os.WriteFile(filepath.Join(dir, "plain_rule.md"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rules := rm.ListRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	// Build a map for easier lookup
	ruleMap := make(map[string]*Rule)
	for _, r := range rules {
		ruleMap[r.ID] = r
	}

	// Verify rule loaded from file with heading
	r1, ok := ruleMap["my-rule"]
	if !ok {
		t.Fatal("expected rule with ID 'my-rule'")
	}
	if r1.Name != "My Rule" {
		t.Errorf("expected name 'My Rule', got %q", r1.Name)
	}
	if r1.Description != "This is the description." {
		t.Errorf("expected description 'This is the description.', got %q", r1.Description)
	}
	if r1.Content != content1 {
		t.Errorf("expected full content to match, got %q", r1.Content)
	}
	if !r1.Enabled {
		t.Error("expected rule to be enabled by default")
	}

	// Verify rule loaded from file without heading
	r2, ok := ruleMap["plain_rule"]
	if !ok {
		t.Fatal("expected rule with ID 'plain_rule'")
	}
	// Without heading, name is derived from filename with underscores replaced by spaces
	if r2.Name != "plain rule" {
		t.Errorf("expected name 'plain rule', got %q", r2.Name)
	}
	if !r2.Enabled {
		t.Error("expected rule to be enabled by default")
	}
}

func TestLoadRules_IgnoresNonMdFilesAndDirs(t *testing.T) {
	dir := t.TempDir()

	// Create a .md file
	if err := os.WriteFile(filepath.Join(dir, "valid.md"), []byte("# Valid Rule\nA description."), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a .txt file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a rule"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory (should be ignored)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create a .md file inside the subdir (should not be loaded since LoadRules is not recursive)
	if err := os.WriteFile(filepath.Join(dir, "subdir", "nested.md"), []byte("# Nested"), 0644); err != nil {
		t.Fatal(err)
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rules := rm.ListRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "valid" {
		t.Errorf("expected rule ID 'valid', got %q", rules[0].ID)
	}
}

func TestListRules_ReturnsAllRules(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"alpha.md", "beta.md", "gamma.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# "+strings.TrimSuffix(name, ".md")+"\nDesc"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatal(err)
	}

	rules := rm.ListRules()
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	ids := make(map[string]bool)
	for _, r := range rules {
		ids[r.ID] = true
	}
	for _, expected := range []string{"alpha", "beta", "gamma"} {
		if !ids[expected] {
			t.Errorf("expected rule ID %q in list", expected)
		}
	}
}

func TestToggleRule_DisablesAndEnables(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toggle-me.md"), []byte("# Toggle Me\nToggle desc."), 0644); err != nil {
		t.Fatal(err)
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatal(err)
	}

	// Disable the rule
	r, err := rm.ToggleRule("toggle-me", false)
	if err != nil {
		t.Fatalf("unexpected error disabling rule: %v", err)
	}
	if r.Enabled {
		t.Error("expected rule to be disabled")
	}

	// Enable the rule
	r, err = rm.ToggleRule("toggle-me", true)
	if err != nil {
		t.Fatalf("unexpected error enabling rule: %v", err)
	}
	if !r.Enabled {
		t.Error("expected rule to be enabled")
	}
}

func TestToggleRule_FailsForUnknownRule(t *testing.T) {
	rm := NewRuleManager()
	_, err := rm.ToggleRule("nonexistent", true)
	if err == nil {
		t.Fatal("expected error for unknown rule ID")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error to mention the rule ID, got %v", err)
	}
}

func TestGetActiveRulesContent_ReturnsOnlyEnabledRules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "enabled-rule.md"), []byte("# Enabled\nEnabled content."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "disabled-rule.md"), []byte("# Disabled\nDisabled content."), 0644); err != nil {
		t.Fatal(err)
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatal(err)
	}

	// Disable one rule
	if _, err := rm.ToggleRule("disabled-rule", false); err != nil {
		t.Fatal(err)
	}

	content := rm.GetActiveRulesContent()
	if !strings.Contains(content, "Enabled content.") {
		t.Error("expected active rules content to include the enabled rule")
	}
	if strings.Contains(content, "Disabled content.") {
		t.Error("expected active rules content to NOT include the disabled rule")
	}
}

func TestGetActiveRulesContent_EmptyWhenNoRulesEnabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "only-rule.md"), []byte("# Only\nOnly content."), 0644); err != nil {
		t.Fatal(err)
	}

	rm := NewRuleManager()
	if err := rm.LoadRules(dir); err != nil {
		t.Fatal(err)
	}

	// Disable the only rule
	if _, err := rm.ToggleRule("only-rule", false); err != nil {
		t.Fatal(err)
	}

	content := rm.GetActiveRulesContent()
	if content != "" {
		t.Errorf("expected empty content when no rules enabled, got %q", content)
	}
}
