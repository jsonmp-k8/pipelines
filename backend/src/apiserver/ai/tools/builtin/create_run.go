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
	"github.com/kubeflow/pipelines/backend/src/apiserver/model"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

type CreateRunTool struct {
	resourceManager *resource.ResourceManager
}

func NewCreateRunTool(rm *resource.ResourceManager) *CreateRunTool {
	return &CreateRunTool{resourceManager: rm}
}

func (t *CreateRunTool) Name() string { return "create_run" }
func (t *CreateRunTool) Description() string {
	return "Create and start a new pipeline run. Requires a pipeline version ID and experiment ID. This is a mutating operation that requires user confirmation in Agent mode."
}
func (t *CreateRunTool) IsReadOnly() bool { return false }

func (t *CreateRunTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Display name for the run",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the run",
			},
			"pipeline_version_id": map[string]interface{}{
				"type":        "string",
				"description": "The pipeline version ID to run",
			},
			"experiment_id": map[string]interface{}{
				"type":        "string",
				"description": "The experiment ID to associate with the run",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "The namespace to create the run in",
			},
			"parameters": map[string]interface{}{
				"type":        "object",
				"description": "Runtime parameters as key-value pairs",
			},
		},
		"required": []string{"name", "pipeline_version_id", "experiment_id"},
	}
}

func (t *CreateRunTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	pipelineVersionID, _ := args["pipeline_version_id"].(string)
	experimentID, _ := args["experiment_id"].(string)
	namespace, _ := args["namespace"].(string)

	if name == "" || pipelineVersionID == "" || experimentID == "" {
		return &tools.ToolResult{Content: "name, pipeline_version_id, and experiment_id are required", IsError: true}, nil
	}

	if err := checkAccess(ctx, t.resourceManager, namespace, common.RbacResourceVerbCreate, common.RbacResourceTypeRuns); err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
	}

	run := &model.Run{
		DisplayName:  name,
		Description:  description,
		ExperimentId: experimentID,
		Namespace:    namespace,
		PipelineSpec: model.PipelineSpec{
			PipelineVersionId: pipelineVersionID,
		},
	}

	// Handle runtime parameters if provided
	if params, ok := args["parameters"].(map[string]interface{}); ok {
		paramsJSON, err := json.Marshal(params)
		if err == nil {
			run.PipelineSpec.RuntimeConfig.Parameters = model.LargeText(string(paramsJSON))
		}
	}

	createdRun, err := t.resourceManager.CreateRun(ctx, run)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create run: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"id":        createdRun.UUID,
		"name":      createdRun.DisplayName,
		"state":     createdRun.State.ToString(),
		"namespace": createdRun.Namespace,
	}
	data, _ := json.Marshal(result)
	return &tools.ToolResult{Content: string(data)}, nil
}
