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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

const systemPromptBase = `You are an AI assistant integrated into Kubeflow Pipelines (KFP). You help users understand, manage, and troubleshoot their ML pipelines.

Your capabilities include:
- Viewing and analyzing pipeline runs, their statuses, and logs
- Browsing pipeline definitions and specifications
- Listing and managing experiments
- Creating runs, experiments, and pipeline versions (in Agent mode with user confirmation)
- Analyzing failures and suggesting fixes
- Generating documentation for pipelines

Guidelines:
- Be concise and specific in your responses
- When analyzing failures, look at run details, task states, and error messages
- When suggesting fixes, provide actionable steps
- Reference specific run IDs, pipeline IDs, and other identifiers when relevant
- Format responses with markdown for readability
- Use tools to gather information before making conclusions`

// PageContext mirrors the proto PageContext message.
type PageContext struct {
	PageType          string `json:"page_type"`
	RunID             string `json:"run_id"`
	PipelineID        string `json:"pipeline_id"`
	PipelineVersionID string `json:"pipeline_version_id"`
	ExperimentID      string `json:"experiment_id"`
	Namespace         string `json:"namespace"`
}

// ContextBuilder constructs system prompts with relevant context.
type ContextBuilder struct {
	resourceManager *resource.ResourceManager
}

// NewContextBuilder creates a new ContextBuilder.
func NewContextBuilder(rm *resource.ResourceManager) *ContextBuilder {
	return &ContextBuilder{resourceManager: rm}
}

// GetResourceManager returns the underlying resource manager.
func (cb *ContextBuilder) GetResourceManager() *resource.ResourceManager {
	return cb.resourceManager
}

// BuildSystemPrompt constructs the full system prompt including page context and rules.
func (cb *ContextBuilder) BuildSystemPrompt(pageCtx *PageContext, rulesContent string) string {
	var parts []string
	parts = append(parts, systemPromptBase)

	// Add page-specific context
	if pageCtx != nil {
		pageContextStr := cb.GatherPageContext(pageCtx)
		if pageContextStr != "" {
			parts = append(parts, "\n## Current Page Context\n"+pageContextStr)
		}
	}

	// Append active rules
	if rulesContent != "" {
		parts = append(parts, "\n## Custom Rules\n"+rulesContent)
	}

	return strings.Join(parts, "\n")
}

// GatherPageContext fetches relevant data based on the current page type.
func (cb *ContextBuilder) GatherPageContext(pageCtx *PageContext) string {
	if pageCtx == nil {
		return ""
	}

	switch pageCtx.PageType {
	case "run_details":
		return cb.gatherRunContext(pageCtx.RunID)
	case "pipeline_details":
		return cb.gatherPipelineContext(pageCtx.PipelineID)
	case "run_list":
		return cb.gatherRunListContext(pageCtx.Namespace, pageCtx.ExperimentID)
	case "pipeline_list":
		return cb.gatherPipelineListContext(pageCtx.Namespace)
	default:
		return fmt.Sprintf("The user is on a %s page.", pageCtx.PageType)
	}
}

func (cb *ContextBuilder) gatherRunContext(runID string) string {
	if runID == "" {
		return "The user is viewing run details but no run ID is available."
	}

	run, err := cb.resourceManager.GetRun(runID)
	if err != nil {
		glog.Warningf("Failed to fetch run context for %s: %v", runID, err)
		return fmt.Sprintf("The user is viewing run %s.", runID)
	}

	ctx := fmt.Sprintf("The user is viewing run details:\n- Run ID: %s\n- Name: %s\n- State: %s",
		run.UUID, run.DisplayName, run.State.ToString())

	if run.State.ToString() == "FAILED" {
		ctx += "\n- **This run has FAILED.** The user may want help debugging the failure."
		// Add state history for failure analysis
		if len(run.StateHistory) > 0 {
			detailsJSON, _ := json.Marshal(run.StateHistory)
			ctx += fmt.Sprintf("\n- State History: %s", string(detailsJSON))
		}
	}

	return ctx
}

func (cb *ContextBuilder) gatherPipelineContext(pipelineID string) string {
	if pipelineID == "" {
		return "The user is viewing pipeline details but no pipeline ID is available."
	}

	pipeline, err := cb.resourceManager.GetPipeline(pipelineID)
	if err != nil {
		glog.Warningf("Failed to fetch pipeline context for %s: %v", pipelineID, err)
		return fmt.Sprintf("The user is viewing pipeline %s.", pipelineID)
	}

	return fmt.Sprintf("The user is viewing pipeline details:\n- Pipeline ID: %s\n- Name: %s\n- Description: %s",
		pipeline.UUID, pipeline.Name, pipeline.Description)
}

func (cb *ContextBuilder) gatherRunListContext(namespace, experimentID string) string {
	ctx := "The user is viewing a list of pipeline runs."
	if namespace != "" {
		ctx += fmt.Sprintf("\n- Namespace: %s", namespace)
	}
	if experimentID != "" {
		ctx += fmt.Sprintf("\n- Experiment ID: %s", experimentID)
	}
	return ctx
}

func (cb *ContextBuilder) gatherPipelineListContext(namespace string) string {
	ctx := "The user is viewing a list of pipelines."
	if namespace != "" {
		ctx += fmt.Sprintf("\n- Namespace: %s", namespace)
	}
	return ctx
}
