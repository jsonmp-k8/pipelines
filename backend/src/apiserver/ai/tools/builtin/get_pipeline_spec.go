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
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

type GetPipelineSpecTool struct {
	resourceManager *resource.ResourceManager
}

func NewGetPipelineSpecTool(rm *resource.ResourceManager) *GetPipelineSpecTool {
	return &GetPipelineSpecTool{resourceManager: rm}
}

func (t *GetPipelineSpecTool) Name() string { return "get_pipeline_spec" }
func (t *GetPipelineSpecTool) Description() string {
	return "Get the pipeline specification (template) for a pipeline. Returns the full pipeline definition including components, DAG, and parameters."
}
func (t *GetPipelineSpecTool) IsReadOnly() bool { return true }

func (t *GetPipelineSpecTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pipeline_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the pipeline to get the spec for",
			},
		},
		"required": []string{"pipeline_id"},
	}
}

func (t *GetPipelineSpecTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pipelineID, ok := args["pipeline_id"].(string)
	if !ok || pipelineID == "" {
		return &tools.ToolResult{Content: "pipeline_id is required", IsError: true}, nil
	}

	templateBytes, err := t.resourceManager.GetPipelineLatestTemplate(pipelineID)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to get pipeline spec: %v", err), IsError: true}, nil
	}

	// Try to parse as JSON for clean output
	var specJSON interface{}
	if err := json.Unmarshal(templateBytes, &specJSON); err == nil {
		data, _ := json.Marshal(map[string]interface{}{
			"pipeline_id": pipelineID,
			"spec":        specJSON,
		})
		return &tools.ToolResult{Content: string(data)}, nil
	}

	// Fall back to raw string
	return &tools.ToolResult{Content: string(templateBytes)}, nil
}
