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

type ListPipelinesTool struct {
	resourceManager *resource.ResourceManager
}

func NewListPipelinesTool(rm *resource.ResourceManager) *ListPipelinesTool {
	return &ListPipelinesTool{resourceManager: rm}
}

func (t *ListPipelinesTool) Name() string { return "list_pipelines" }
func (t *ListPipelinesTool) Description() string {
	return "List available pipelines with optional filtering by namespace. Returns pipeline IDs, names, descriptions, and creation timestamps."
}
func (t *ListPipelinesTool) IsReadOnly() bool { return true }

func (t *ListPipelinesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter pipelines by namespace",
			},
			"page_size": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of pipelines to return (default 10)",
			},
		},
	}
}

func (t *ListPipelinesTool) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	pageSize := 10
	if ps, ok := args["page_size"].(float64); ok && ps > 0 {
		pageSize = int(ps)
	}
	if pageSize > 100 {
		pageSize = 100
	}

	ns, _ := args["namespace"].(string)

	// In multi-user mode, require namespace to prevent cross-tenant data leakage
	if common.IsMultiUserMode() && ns == "" {
		return &tools.ToolResult{
			Content: "namespace is required in multi-user mode",
			IsError: true,
		}, nil
	}

	filterContext := &model.FilterContext{}
	if ns != "" {
		filterContext.ReferenceKey = &model.ReferenceKey{Type: model.NamespaceResourceType, ID: ns}
	}

	// Authorization check
	if ns != "" {
		if err := checkAccess(ctx, t.resourceManager, ns, common.RbacResourceVerbList, common.RbacResourceTypePipelines); err != nil {
			return &tools.ToolResult{Content: fmt.Sprintf("Authorization failed: %v", err), IsError: true}, nil
		}
	}

	opts, err := list.NewOptions(&model.Pipeline{}, pageSize, "", nil)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to create list options: %v", err), IsError: true}, nil
	}

	pipelines, totalSize, _, err := t.resourceManager.ListPipelines(filterContext, opts)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to list pipelines: %v", err), IsError: true}, nil
	}

	var results []map[string]interface{}
	for _, p := range pipelines {
		results = append(results, map[string]interface{}{
			"id":          p.UUID,
			"name":        p.Name,
			"description": p.Description,
			"namespace":   p.Namespace,
			"created_at":  p.CreatedAtInSec,
		})
	}

	data, err := json.Marshal(map[string]interface{}{
		"total_count": totalSize,
		"pipelines":   results,
	})
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("Failed to marshal result: %v", err), IsError: true}, nil
	}
	return &tools.ToolResult{Content: string(data)}, nil
}
