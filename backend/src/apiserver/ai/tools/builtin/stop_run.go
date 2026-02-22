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

type StopRunTool struct {
	resourceManager *resource.ResourceManager
}

func NewStopRunTool(rm *resource.ResourceManager) *StopRunTool {
	return &StopRunTool{resourceManager: rm}
}

func (t *StopRunTool) Name() string { return "stop_run" }
func (t *StopRunTool) Description() string {
	return "Stop/terminate a running pipeline run. This is a mutating operation that requires user confirmation in Agent mode."
}
func (t *StopRunTool) IsReadOnly() bool { return false }

func (t *StopRunTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"run_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the run to stop",
			},
		},
		"required": []string{"run_id"},
	}
}

func (t *StopRunTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	runID, ok := args["run_id"].(string)
	if !ok || runID == "" {
		return &tools.ToolResult{Content: "run_id is required", IsError: true}, nil
	}

	if err := checkRunAccess(ctx, t.resourceManager, runID, common.RbacResourceVerbTerminate); err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
	}

	err := t.resourceManager.TerminateRun(ctx, runID)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to stop run: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"run_id":  runID,
		"message": "Run terminated successfully",
	}
	data, err := json.Marshal(result)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to marshal result: %v", err), IsError: true}, nil
	}
	return &tools.ToolResult{Content: string(data)}, nil
}
