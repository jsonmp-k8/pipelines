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
	"github.com/kubeflow/pipelines/backend/src/apiserver/model"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

type CreatePipelineVersionTool struct {
	resourceManager *resource.ResourceManager
}

func NewCreatePipelineVersionTool(rm *resource.ResourceManager) *CreatePipelineVersionTool {
	return &CreatePipelineVersionTool{resourceManager: rm}
}

func (t *CreatePipelineVersionTool) Name() string { return "create_pipeline_version" }
func (t *CreatePipelineVersionTool) Description() string {
	return "Create a new version of an existing pipeline. This is a mutating operation that requires user confirmation in Agent mode."
}
func (t *CreatePipelineVersionTool) IsReadOnly() bool { return false }

func (t *CreatePipelineVersionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pipeline_id": map[string]interface{}{
				"type":        "string",
				"description": "The pipeline ID to create a version for",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Display name for the pipeline version",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the pipeline version",
			},
		},
		"required": []string{"pipeline_id", "name"},
	}
}

func (t *CreatePipelineVersionTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pipelineID, _ := args["pipeline_id"].(string)
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)

	if pipelineID == "" || name == "" {
		return &tools.ToolResult{Content: "pipeline_id and name are required", IsError: true}, nil
	}

	pv := &model.PipelineVersion{
		PipelineId:  pipelineID,
		Name:        name,
		Description: model.LargeText(description),
	}

	created, err := t.resourceManager.CreatePipelineVersion(pv)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create pipeline version: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"id":          created.UUID,
		"name":        created.Name,
		"pipeline_id": created.PipelineId,
	}
	data, _ := json.Marshal(result)
	return &tools.ToolResult{Content: string(data)}, nil
}
