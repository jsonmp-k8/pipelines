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

package ai

import "github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"

// AIExtension allows plugins to register custom tools with the AI server.
type AIExtension interface {
	// Name returns the extension name.
	Name() string
	// Tools returns the tools provided by this extension.
	Tools() []tools.Tool
}

// RegisterExtension registers all tools from an AI extension.
func (s *AIServer) RegisterExtension(ext AIExtension) {
	for _, tool := range ext.Tools() {
		s.toolRegistry.Register(tool)
	}
}
