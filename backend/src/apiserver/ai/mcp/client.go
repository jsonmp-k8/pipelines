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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/golang/glog"
)

// MCPToolDefinition represents a tool discovered from an MCP server.
type MCPToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolResult represents the result of calling an MCP tool.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// MCPContent represents content returned from an MCP tool call.
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPClient connects to an external MCP server and discovers/calls tools.
type MCPClient struct {
	url    string
	client *http.Client
	mu     sync.RWMutex
	tools  []MCPToolDefinition
}

// NewMCPClient creates a new MCP client for the given server URL.
func NewMCPClient(url string) *MCPClient {
	return &MCPClient{
		url:    url,
		client: &http.Client{},
	}
}

// Connect initializes the connection and discovers available tools.
func (c *MCPClient) Connect(ctx context.Context) error {
	tools, err := c.discoverTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover tools from MCP server %s: %w", c.url, err)
	}

	c.mu.Lock()
	c.tools = tools
	c.mu.Unlock()

	glog.Infof("Discovered %d tools from MCP server %s", len(tools), c.url)
	return nil
}

// ListTools returns the discovered tools.
func (c *MCPClient) ListTools() []MCPToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPToolResult, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP tool: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp struct {
		Result *MCPToolResult `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return &MCPToolResult{
			IsError: true,
			Content: []MCPContent{{Type: "text", Text: rpcResp.Error.Message}},
		}, nil
	}

	return rpcResp.Result, nil
}

// Close closes the MCP client connection.
func (c *MCPClient) Close() {
	// HTTP-based MCP doesn't need explicit close
}

func (c *MCPClient) discoverTools(ctx context.Context) ([]MCPToolDefinition, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp struct {
		Result struct {
			Tools []MCPToolDefinition `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, err
	}

	return rpcResp.Result.Tools, nil
}
