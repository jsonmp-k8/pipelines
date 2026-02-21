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

type GetPipelineTool struct {
	resourceManager *resource.ResourceManager
}

func NewGetPipelineTool(rm *resource.ResourceManager) *GetPipelineTool {
	return &GetPipelineTool{resourceManager: rm}
}

func (t *GetPipelineTool) Name() string { return "get_pipeline" }
func (t *GetPipelineTool) Description() string {
	return "Get detailed information about a specific pipeline by its ID, including name, description, and version history."
}
func (t *GetPipelineTool) IsReadOnly() bool { return true }

func (t *GetPipelineTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pipeline_id": map[string]interface{}{
				"type":        "string",
				"description": "The ID of the pipeline to retrieve",
			},
		},
		"required": []string{"pipeline_id"},
	}
}

func (t *GetPipelineTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pipelineID, ok := args["pipeline_id"].(string)
	if !ok || pipelineID == "" {
		return &tools.ToolResult{Content: "pipeline_id is required", IsError: true}, nil
	}

	pipeline, err := t.resourceManager.GetPipeline(pipelineID)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to get pipeline: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"id":          pipeline.UUID,
		"name":        pipeline.Name,
		"description": pipeline.Description,
		"namespace":   pipeline.Namespace,
		"created_at":  pipeline.CreatedAtInSec,
	}
	data, _ := json.Marshal(result)
	return &tools.ToolResult{Content: string(data)}, nil
}
