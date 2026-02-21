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

package context

import (
	"strings"
	"testing"
)

func TestBuildSystemPrompt_NilPageCtxEmptyRules_ReturnsBasePromptOnly(t *testing.T) {
	cb := NewContextBuilder(nil)
	result := cb.BuildSystemPrompt(nil, "")

	if result != systemPromptBase {
		t.Errorf("expected base prompt only, got:\n%s", result)
	}
}

func TestBuildSystemPrompt_WithRules_AppendsRulesSection(t *testing.T) {
	cb := NewContextBuilder(nil)
	rules := "Always respond in JSON format."
	result := cb.BuildSystemPrompt(nil, rules)

	if !strings.Contains(result, systemPromptBase) {
		t.Error("expected result to contain the base system prompt")
	}
	if !strings.Contains(result, "## Custom Rules") {
		t.Error("expected result to contain '## Custom Rules' header")
	}
	if !strings.Contains(result, rules) {
		t.Errorf("expected result to contain rules content %q", rules)
	}
}

func TestGatherPageContext_Nil_ReturnsEmpty(t *testing.T) {
	cb := NewContextBuilder(nil)
	result := cb.GatherPageContext(nil)

	if result != "" {
		t.Errorf("expected empty string for nil page context, got %q", result)
	}
}

func TestGatherPageContext_RunList_ReturnsRunListContext(t *testing.T) {
	cb := NewContextBuilder(nil)
	pageCtx := &PageContext{
		PageType:     "run_list",
		Namespace:    "test-namespace",
		ExperimentID: "exp-123",
	}
	result := cb.GatherPageContext(pageCtx)

	if !strings.Contains(result, "list of pipeline runs") {
		t.Error("expected run list context message")
	}
	if !strings.Contains(result, "test-namespace") {
		t.Error("expected namespace in run list context")
	}
	if !strings.Contains(result, "exp-123") {
		t.Error("expected experiment ID in run list context")
	}
}

func TestGatherPageContext_PipelineList_ReturnsPipelineListContext(t *testing.T) {
	cb := NewContextBuilder(nil)
	pageCtx := &PageContext{
		PageType:  "pipeline_list",
		Namespace: "prod-ns",
	}
	result := cb.GatherPageContext(pageCtx)

	if !strings.Contains(result, "list of pipelines") {
		t.Error("expected pipeline list context message")
	}
	if !strings.Contains(result, "prod-ns") {
		t.Error("expected namespace in pipeline list context")
	}
}

func TestGatherPageContext_UnknownPageType_ReturnsGenericMessage(t *testing.T) {
	cb := NewContextBuilder(nil)
	pageCtx := &PageContext{
		PageType: "settings",
	}
	result := cb.GatherPageContext(pageCtx)

	if !strings.Contains(result, "settings") {
		t.Error("expected generic message to include the page type")
	}
	expected := "The user is on a settings page."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGatherPageContext_RunDetails_EmptyRunID_ReturnsFallback(t *testing.T) {
	cb := NewContextBuilder(nil)
	pageCtx := &PageContext{
		PageType: "run_details",
		RunID:    "",
	}
	result := cb.GatherPageContext(pageCtx)

	expected := "The user is viewing run details but no run ID is available."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGatherPageContext_PipelineDetails_EmptyPipelineID_ReturnsFallback(t *testing.T) {
	cb := NewContextBuilder(nil)
	pageCtx := &PageContext{
		PageType:   "pipeline_details",
		PipelineID: "",
	}
	result := cb.GatherPageContext(pageCtx)

	expected := "The user is viewing pipeline details but no pipeline ID is available."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
