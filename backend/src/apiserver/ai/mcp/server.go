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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

// MCPServer exposes KFP built-in tools to external AI agents via Streamable HTTP.
type MCPServer struct {
	toolRegistry *tools.ToolRegistry
}

// NewMCPServer creates a new MCP server.
func NewMCPServer(registry *tools.ToolRegistry) *MCPServer {
	return &MCPServer{toolRegistry: registry}
}

// Handler returns an http.HandlerFunc that handles MCP JSON-RPC requests.
// Registered at /apis/v2beta1/ai/mcp
func (s *MCPServer) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit request body size to 1MB.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONRPCError(w, -1, -32700, "Request too large")
			return
		}
		defer r.Body.Close()

		var req struct {
			JSONRPC string                 `json:"jsonrpc"`
			ID      interface{}            `json:"id"`
			Method  string                 `json:"method"`
			Params  map[string]interface{} `json:"params"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONRPCError(w, -1, -32700, "Parse error")
			return
		}

		switch req.Method {
		case "tools/list":
			s.handleListTools(w, req.ID)
		case "tools/call":
			s.handleCallTool(w, r.Context(), req.ID, req.Params)
		case "initialize":
			s.handleInitialize(w, req.ID)
		default:
			writeJSONRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
		}
	}
}

func (s *MCPServer) handleInitialize(w http.ResponseWriter, id interface{}) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "kubeflow-pipelines",
			"version": "1.0.0",
		},
	}
	writeJSONRPCResult(w, id, result)
}

func (s *MCPServer) handleListTools(w http.ResponseWriter, id interface{}) {
	defs := s.toolRegistry.ListForMode(tools.ChatModeAgent)
	var mcpTools []map[string]interface{}
	for _, d := range defs {
		mcpTools = append(mcpTools, map[string]interface{}{
			"name":        d.Name,
			"description": d.Description,
			"inputSchema": d.InputSchema,
		})
	}
	writeJSONRPCResult(w, id, map[string]interface{}{"tools": mcpTools})
}

func (s *MCPServer) handleCallTool(w http.ResponseWriter, ctx context.Context, id interface{}, params map[string]interface{}) {
	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	tool, ok := s.toolRegistry.Get(name)
	if !ok {
		writeJSONRPCError(w, id, -32602, fmt.Sprintf("Unknown tool: %s", name))
		return
	}

	// Block mutating tools via MCP since there is no confirmation flow.
	if !tool.IsReadOnly() {
		writeJSONRPCError(w, id, -32602, fmt.Sprintf("Tool %s is mutating and cannot be called via MCP without a confirmation flow", name))
		return
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		glog.Errorf("MCP tool execution error for %s: %v", name, err)
		writeJSONRPCResult(w, id, map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
			},
			"isError": true,
		})
		return
	}

	writeJSONRPCResult(w, id, map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": result.Content},
		},
		"isError": result.IsError,
	})
}

func writeJSONRPCResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}
