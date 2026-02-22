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

type ListRunsTool struct {
	resourceManager *resource.ResourceManager
}

func NewListRunsTool(rm *resource.ResourceManager) *ListRunsTool {
	return &ListRunsTool{resourceManager: rm}
}

func (t *ListRunsTool) Name() string { return "list_runs" }
func (t *ListRunsTool) Description() string {
	return "List pipeline runs with optional filtering by namespace or experiment. Returns run IDs, names, statuses, and timestamps."
}
func (t *ListRunsTool) IsReadOnly() bool { return true }

func (t *ListRunsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter runs by namespace",
			},
			"experiment_id": map[string]interface{}{
				"type":        "string",
				"description": "Filter runs by experiment ID",
			},
			"page_size": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of runs to return (default 10)",
			},
		},
	}
}

func (t *ListRunsTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pageSize := 10
	if ps, ok := args["page_size"].(float64); ok && ps > 0 {
		pageSize = int(ps)
	}

	filterContext := &model.FilterContext{}
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		filterContext.ReferenceKey = &model.ReferenceKey{Type: model.NamespaceResourceType, ID: ns}
	}
	if expID, ok := args["experiment_id"].(string); ok && expID != "" {
		filterContext.ReferenceKey = &model.ReferenceKey{Type: model.ExperimentResourceType, ID: expID}
	}

	// Authorization checks
	if ns, ok := args["namespace"].(string); ok && ns != "" {
		if err := checkAccess(ctx, t.resourceManager, ns, common.RbacResourceVerbList, common.RbacResourceTypeRuns); err != nil {
			return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
		}
	}
	if expID, ok := args["experiment_id"].(string); ok && expID != "" {
		if err := checkExperimentAccess(ctx, t.resourceManager, expID, common.RbacResourceVerbList); err != nil {
			return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
		}
	}

	opts, err := list.NewOptions(&model.Run{}, pageSize, "", nil)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create list options: %v", err), IsError: true}, nil
	}

	runs, totalSize, _, err := t.resourceManager.ListRuns(filterContext, opts)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to list runs: %v", err), IsError: true}, nil
	}

	result := map[string]interface{}{
		"total_count": totalSize,
		"runs":        formatRuns(runs),
	}
	data, _ := json.Marshal(result)
	return &tools.ToolResult{Content: string(data)}, nil
}

func formatRuns(runs []*model.Run) []map[string]interface{} {
	var result []map[string]interface{}
	for _, r := range runs {
		result = append(result, map[string]interface{}{
			"id":           r.UUID,
			"name":         r.DisplayName,
			"state":        r.State.ToString(),
			"namespace":    r.Namespace,
			"experiment_id": r.ExperimentId,
			"created_at":   r.CreatedAtInSec,
			"finished_at":  r.FinishedAtInSec,
		})
	}
	return result
}
