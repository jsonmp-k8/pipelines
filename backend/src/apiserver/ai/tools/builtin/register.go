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
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
	"github.com/kubeflow/pipelines/backend/src/apiserver/resource"
)

// RegisterAll registers all built-in tools with the given registry.
func RegisterAll(registry *tools.ToolRegistry, rm *resource.ResourceManager) {
	// Read-only tools
	registry.Register(NewListRunsTool(rm))
	registry.Register(NewGetRunTool(rm))
	registry.Register(NewGetRunLogsTool(rm))
	registry.Register(NewListPipelinesTool(rm))
	registry.Register(NewGetPipelineTool(rm))
	registry.Register(NewGetPipelineSpecTool(rm))
	registry.Register(NewListExperimentsTool(rm))

	// Mutating tools
	registry.Register(NewCreateRunTool(rm))
	registry.Register(NewCreateExperimentTool(rm))
	registry.Register(NewCreatePipelineVersionTool(rm))
	registry.Register(NewStopRunTool(rm))
	registry.Register(NewDeleteRunTool(rm))
}
