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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/golang/glog"
)

// Rule represents an AI behavior rule loaded from a markdown file.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Enabled     bool   `json:"enabled"`
}

// RuleManager loads and manages AI rules from markdown files.
type RuleManager struct {
	mu    sync.RWMutex
	rules map[string]*Rule
}

// NewRuleManager creates a new empty RuleManager.
func NewRuleManager() *RuleManager {
	return &RuleManager{
		rules: make(map[string]*Rule),
	}
}

// LoadRules reads all markdown files from the given directory and loads them as rules.
func (rm *RuleManager) LoadRules(dirPath string) error {
	if dirPath == "" {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			glog.Infof("Rules directory %s does not exist, skipping", dirPath)
			return nil
		}
		return fmt.Errorf("failed to read rules directory: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			glog.Warningf("Failed to read rule file %s: %v", filePath, err)
			continue
		}

		// Use filename without extension as ID
		id := strings.TrimSuffix(entry.Name(), ".md")
		name := strings.ReplaceAll(id, "-", " ")
		name = strings.ReplaceAll(name, "_", " ")

		// Extract description from first line if it starts with #
		description := ""
		contentStr := string(content)
		lines := strings.SplitN(contentStr, "\n", 2)
		if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
			name = strings.TrimPrefix(lines[0], "# ")
			if len(lines) > 1 {
				// Look for description in the next non-empty line
				remaining := strings.TrimSpace(lines[1])
				descLines := strings.SplitN(remaining, "\n", 2)
				if len(descLines) > 0 {
					description = strings.TrimSpace(descLines[0])
				}
			}
		}

		rm.rules[id] = &Rule{
			ID:          id,
			Name:        name,
			Description: description,
			Content:     contentStr,
			Enabled:     true,
		}
	}

	glog.Infof("Loaded %d AI rules", len(rm.rules))
	return nil
}

// ListRules returns copies of all rules.
func (rm *RuleManager) ListRules() []*Rule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*Rule, 0, len(rm.rules))
	for _, r := range rm.rules {
		copy := *r
		result = append(result, &copy)
	}
	return result
}

// ToggleRule enables or disables a rule by ID.
func (rm *RuleManager) ToggleRule(id string, enabled bool) (*Rule, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	r, ok := rm.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	r.Enabled = enabled
	return r, nil
}

// GetActiveRulesContent returns the concatenated content of all enabled rules.
func (rm *RuleManager) GetActiveRulesContent() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var parts []string
	for _, r := range rm.rules {
		if r.Enabled {
			parts = append(parts, r.Content)
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}
