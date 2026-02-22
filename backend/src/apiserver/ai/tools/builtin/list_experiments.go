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
	"github.com/kubeflow/pipelines/backend/src/apiserver/list"
	"github.com/kubeflow/pipelines/backend/src/apiserver/model"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

type ListExperimentsTool struct {
	resourceManager *resource.ResourceManager
}

func NewListExperimentsTool(rm *resource.ResourceManager) *ListExperimentsTool {
	return &ListExperimentsTool{resourceManager: rm}
}

func (t *ListExperimentsTool) Name() string { return "list_experiments" }
func (t *ListExperimentsTool) Description() string {
	return "List experiments with optional filtering by namespace. Returns experiment IDs, names, descriptions, and statuses."
}
func (t *ListExperimentsTool) IsReadOnly() bool { return true }

func (t *ListExperimentsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter experiments by namespace",
			},
			"page_size": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of experiments to return (default 10)",
			},
		},
	}
}

func (t *ListExperimentsTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pageSize := 10
	if ps, ok := args["page_size"].(float64); ok && ps > 0 {
		pageSize = int(ps)
	}

	filterContext := &model.FilterContext{}
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		filterContext.ReferenceKey = &model.ReferenceKey{Type: model.NamespaceResourceType, ID: ns}
	}

	// Authorization check
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		if err := checkAccess(ctx, t.resourceManager, ns, common.RbacResourceVerbList, common.RbacResourceTypeExperiments); err != nil {
			return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
		}
	}

	opts, err := list.NewOptions(&model.Experiment{}, pageSize, "", nil)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create list options: %v", err), IsError: true}, nil
	}

	experiments, totalSize, _, err := t.resourceManager.ListExperiments(filterContext, opts)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to list experiments: %v", err), IsError: true}, nil
	}

	var results []map[string]interface{}
	for _, e := range experiments {
		results = append(results, map[string]interface{}{
			"id":          e.UUID,
			"name":        e.Name,
			"description": e.Description,
			"namespace":   e.Namespace,
			"created_at":  e.CreatedAtInSec,
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total_count":  totalSize,
		"experiments":  results,
	})
	return &tools.ToolResult{Content: string(data)}, nil
}
