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

type DeleteRunTool struct {
	resourceManager *resource.ResourceManager
}

func NewDeleteRunTool(rm *resource.ResourceManager) *DeleteRunTool {
	return &DeleteRunTool{resourceManager: rm}
}

func (t *DeleteRunTool) Name() string { return "delete_run" }
func (t *DeleteRunTool) Description() string {
	return "Permanently delete a pipeline run. This is a destructive mutating operation that requires user confirmation in Agent mode."
}
func (t *DeleteRunTool) IsReadOnly() bool { return false }

func (t *DeleteRunTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"run_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the run to delete",
			},
		},
		"required": []string{"run_id"},
	}
}

func (t *DeleteRunTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	runID, ok := args["run_id"].(string)
	if !ok || runID == "" {
		return &tools.ToolResult{Content: "run_id is required", IsError: true}, nil
	}

	if err := checkRunAccess(ctx, t.resourceManager, runID, common.RbacResourceVerbDelete); err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
	}

	err := t.resourceManager.DeleteRun(ctx, runID)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to delete run: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"run_id":  runID,
		"message": "Run deleted successfully",
	}
	data, _ := json.Marshal(result)
	return &tools.ToolResult{Content: string(data)}, nil
}
