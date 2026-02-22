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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"
	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/kubeflow/pipelines/backend/src/apiserver/common"
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
// ctx is used for RBAC checks when fetching resource data for page context.
func (cb *ContextBuilder) BuildSystemPrompt(ctx context.Context, pageCtx *PageContext, rulesContent string) string {
	var parts []string
	parts = append(parts, systemPromptBase)

	// Add page-specific context
	if pageCtx != nil {
		pageContextStr := cb.GatherPageContext(ctx, pageCtx)
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
// ctx is used for RBAC checks — if the user lacks access, only generic context is returned.
func (cb *ContextBuilder) GatherPageContext(ctx context.Context, pageCtx *PageContext) string {
	if pageCtx == nil {
		return ""
	}

	switch pageCtx.PageType {
	case "run_details":
		return cb.gatherRunContext(ctx, pageCtx.RunID)
	case "pipeline_details":
		return cb.gatherPipelineContext(ctx, pageCtx.PipelineID)
	case "run_list":
		return cb.gatherRunListContext(pageCtx.Namespace, pageCtx.ExperimentID)
	case "pipeline_list":
		return cb.gatherPipelineListContext(pageCtx.Namespace)
	default:
		return fmt.Sprintf("The user is on a %s page.", pageCtx.PageType)
	}
}

func (cb *ContextBuilder) gatherRunContext(ctx context.Context, runID string) string {
	if runID == "" {
		return "The user is viewing run details but no run ID is available."
	}

	// RBAC check: verify the user has read access to this run before fetching.
	if err := cb.checkRunAccess(ctx, runID); err != nil {
		glog.Warningf("Run context access denied for %s: %v", runID, err)
		return fmt.Sprintf("The user is viewing run %s.", runID)
	}

	run, err := cb.resourceManager.GetRun(runID)
	if err != nil {
		glog.Warningf("Failed to fetch run context for %s: %v", runID, err)
		return fmt.Sprintf("The user is viewing run %s.", runID)
	}

	result := fmt.Sprintf("The user is viewing run details:\n- Run ID: %s\n- Name: %s\n- State: %s",
		run.UUID, run.DisplayName, run.State.ToString())

	if run.State.ToString() == "FAILED" {
		result += "\n- **This run has FAILED.** The user may want help debugging the failure."
		// Add state history for failure analysis
		if len(run.StateHistory) > 0 {
			detailsJSON, err := json.Marshal(run.StateHistory)
		if err != nil {
			detailsJSON = []byte("[]")
		}
			result += fmt.Sprintf("\n- State History: %s", string(detailsJSON))
		}
	}

	return result
}

func (cb *ContextBuilder) gatherPipelineContext(ctx context.Context, pipelineID string) string {
	if pipelineID == "" {
		return "The user is viewing pipeline details but no pipeline ID is available."
	}

	// RBAC check: verify the user has read access to this pipeline before fetching.
	if err := cb.checkPipelineAccess(ctx, pipelineID); err != nil {
		glog.Warningf("Pipeline context access denied for %s: %v", pipelineID, err)
		return fmt.Sprintf("The user is viewing pipeline %s.", pipelineID)
	}

	pipeline, err := cb.resourceManager.GetPipeline(pipelineID)
	if err != nil {
		glog.Warningf("Failed to fetch pipeline context for %s: %v", pipelineID, err)
		return fmt.Sprintf("The user is viewing pipeline %s.", pipelineID)
	}

	return fmt.Sprintf("The user is viewing pipeline details:\n- Pipeline ID: %s\n- Name: %s\n- Description: %s",
		pipeline.UUID, pipeline.Name, pipeline.Description)
}

// checkRunAccess verifies the user has read access to the given run.
// In single-user mode this is a no-op.
func (cb *ContextBuilder) checkRunAccess(ctx context.Context, runID string) error {
	if !common.IsMultiUserMode() || cb.resourceManager == nil {
		return nil
	}
	run, err := cb.resourceManager.GetRun(runID)
	if err != nil {
		return fmt.Errorf("failed to resolve run for authorization: %w", err)
	}
	namespace := run.Namespace
	if cb.resourceManager.IsEmptyNamespace(namespace) {
		experiment, err := cb.resourceManager.GetExperiment(run.ExperimentId)
		if err != nil {
			return fmt.Errorf("failed to resolve experiment namespace: %w", err)
		}
		namespace = experiment.Namespace
	}
	return cb.resourceManager.IsAuthorized(ctx, &authorizationv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      common.RbacResourceVerbGet,
		Group:     common.RbacPipelinesGroup,
		Version:   common.RbacPipelinesVersion,
		Resource:  common.RbacResourceTypeRuns,
	})
}

// checkPipelineAccess verifies the user has read access to the given pipeline.
// In single-user mode this is a no-op.
func (cb *ContextBuilder) checkPipelineAccess(ctx context.Context, pipelineID string) error {
	if !common.IsMultiUserMode() || cb.resourceManager == nil {
		return nil
	}
	pipeline, err := cb.resourceManager.GetPipeline(pipelineID)
	if err != nil {
		return fmt.Errorf("failed to resolve pipeline for authorization: %w", err)
	}
	return cb.resourceManager.IsAuthorized(ctx, &authorizationv1.ResourceAttributes{
		Namespace: pipeline.Namespace,
		Verb:      common.RbacResourceVerbGet,
		Group:     common.RbacPipelinesGroup,
		Version:   common.RbacPipelinesVersion,
		Resource:  common.RbacResourceTypePipelines,
	})
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
