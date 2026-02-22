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

type CreateExperimentTool struct {
	resourceManager *resource.ResourceManager
}

func NewCreateExperimentTool(rm *resource.ResourceManager) *CreateExperimentTool {
	return &CreateExperimentTool{resourceManager: rm}
}

func (t *CreateExperimentTool) Name() string { return "create_experiment" }
func (t *CreateExperimentTool) Description() string {
	return "Create a new experiment to organize pipeline runs. This is a mutating operation that requires user confirmation in Agent mode."
}
func (t *CreateExperimentTool) IsReadOnly() bool { return false }

func (t *CreateExperimentTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Display name for the experiment",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Description of the experiment",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "The namespace to create the experiment in",
			},
		},
		"required": []string{"name"},
	}
}

func (t *CreateExperimentTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	namespace, _ := args["namespace"].(string)

	if name == "" {
		return &tools.ToolResult{Content: "name is required", IsError: true}, nil
	}

	// In multi-user mode, namespace is required to scope the experiment.
	if common.IsMultiUserMode() && namespace == "" {
		return &tools.ToolResult{Content: "namespace is required in multi-user mode", IsError: true}, nil
	}

	if err := checkAccess(ctx, t.resourceManager, namespace, common.RbacResourceVerbCreate, common.RbacResourceTypeExperiments); err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
	}

	experiment := &model.Experiment{
		Name:        name,
		Description: description,
		Namespace:   namespace,
	}

	created, err := t.resourceManager.CreateExperiment(experiment)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create experiment: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"id":        created.UUID,
		"name":      created.Name,
		"namespace": created.Namespace,
	}
	data, err := json.Marshal(result)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to marshal result: %v", err), IsError: true}, nil
	}
	return &tools.ToolResult{Content: string(data)}, nil
}
