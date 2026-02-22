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

package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
	"github.com/kubeflow/pipelines/backend/src/apiserver/common"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

type GetRunTool struct {
	resourceManager *resource.ResourceManager
}

func NewGetRunTool(rm *resource.ResourceManager) *GetRunTool {
	return &GetRunTool{resourceManager: rm}
}

func (t *GetRunTool) Name() string { return "get_run" }
func (t *GetRunTool) Description() string {
	return "Get detailed information about a specific pipeline run by its ID, including status, parameters, and task details."
}
func (t *GetRunTool) IsReadOnly() bool { return true }

func (t *GetRunTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"run_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the run to retrieve",
			},
		},
		"required": []string{"run_id"},
	}
}

func (t *GetRunTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	runID, ok := args["run_id"].(string)
	if !ok || runID == "" {
		return &tools.ToolResult{Content: "run_id is required", IsError: true}, nil
	}

	if err := checkRunAccess(ctx, t.resourceManager, runID, common.RbacResourceVerbGet); err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
	}

	run, err := t.resourceManager.GetRun(runID)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to get run: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"id":              run.UUID,
		"name":            run.DisplayName,
		"description":     run.Description,
		"state":           run.State.ToString(),
		"namespace":       run.Namespace,
		"experiment_id":   run.ExperimentId,
		"pipeline_spec":   run.PipelineSpec,
		"state_history":   run.StateHistory,
		"created_at":      run.CreatedAtInSec,
		"scheduled_at":    run.ScheduledAtInSec,
		"finished_at":     run.FinishedAtInSec,
		"run_details":     run.RunDetails,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to marshal result: %v", err), IsError: true}, nil
	}
	return &tools.ToolResult{Content: string(data)}, nil
}
