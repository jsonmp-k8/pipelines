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
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/kubeflow/pipelines/backend/src/apiserver/ai/tools"
)

// MCPServerConfig represents configuration for an external MCP server.
type MCPServerConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// MCPServerStatus represents the status of a configured MCP server.
type MCPServerStatus struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Status string   `json:"status"` // "connected", "disconnected", "error"
	Tools  []string `json:"tools"`
}

// mcpToolAdapter wraps an MCP tool to implement the tools.Tool interface.
type mcpToolAdapter struct {
	name        string
	remoteName  string // The original tool name on the MCP server
	description string
	schema      map[string]interface{}
	client      *MCPClient
}

func (t *mcpToolAdapter) Name() string                         { return t.name }
func (t *mcpToolAdapter) Description() string                  { return t.description }
func (t *mcpToolAdapter) InputSchema() map[string]interface{}  { return t.schema }
func (t *mcpToolAdapter) IsReadOnly() bool                     { return false } // All external MCP tools require confirmation

func (t *mcpToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (*tools.ToolResult, error) {
	// Use remoteName (original tool name) when calling the MCP server
	result, err := t.client.CallTool(ctx, t.remoteName, args)
	if err != nil {
		return &tools.ToolResult{Content: fmt.Sprintf("MCP tool error: %v", err), IsError: true}, nil
	}

	var content string
	for _, c := range result.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	return &tools.ToolResult{Content: content, IsError: result.IsError}, nil
}

// validateMCPServerURL validates that the URL is safe to connect to (SSRF prevention).
func validateMCPServerURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported scheme %q: only http and https are allowed", scheme)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	// Block well-known internal hostnames
	blockedHosts := []string{
		"localhost",
		"metadata.google.internal",
		"metadata.google",
		"kubernetes.default",
		"kubernetes.default.svc",
	}
	for _, blocked := range blockedHosts {
		if host == blocked {
			return fmt.Errorf("connecting to %s is not allowed", host)
		}
	}

	// Resolve hostname to IP addresses and check each one
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateOrReservedIP(ip) {
			return fmt.Errorf("connecting to %s (%s) is not allowed: resolves to private/reserved IP", host, ip)
		}
	}
	return nil
}

// isPrivateOrReservedIP checks if an IP address is private, loopback, link-local, or otherwise reserved.
func isPrivateOrReservedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() ||
		ip.IsMulticast() || ip.Equal(net.IPv4bcast)
}

// MCPManager manages lifecycle of MCP client/server connections.
type MCPManager struct {
	mu           sync.RWMutex
	clients      map[string]*MCPClient
	configs      []MCPServerConfig
	toolRegistry *tools.ToolRegistry
	mcpServer    *MCPServer
}

// NewMCPManager creates a new MCP manager.
func NewMCPManager(toolRegistry *tools.ToolRegistry) *MCPManager {
	return &MCPManager{
		clients:      make(map[string]*MCPClient),
		toolRegistry: toolRegistry,
		mcpServer:    NewMCPServer(toolRegistry),
	}
}

// LoadConfig loads MCP server configurations from a JSON string (typically from ConfigMap).
func (m *MCPManager) LoadConfig(configJSON string) error {
	if configJSON == "" {
		return nil
	}

	var configs []MCPServerConfig
	if err := json.Unmarshal([]byte(configJSON), &configs); err != nil {
		return fmt.Errorf("failed to parse MCP config: %w", err)
	}

	m.mu.Lock()
	m.configs = configs
	m.mu.Unlock()

	return nil
}

// ConnectAll connects to all configured MCP servers and registers their tools.
func (m *MCPManager) ConnectAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cfg := range m.configs {
		if err := validateMCPServerURL(cfg.URL); err != nil {
			glog.Warningf("Skipping MCP server %s: invalid URL %s: %v", cfg.Name, cfg.URL, err)
			continue
		}

		client := NewMCPClient(cfg.URL)
		if err := client.Connect(ctx); err != nil {
			glog.Warningf("Failed to connect to MCP server %s at %s: %v", cfg.Name, cfg.URL, err)
			continue
		}

		m.clients[cfg.Name] = client

		// Register discovered tools (all external tools are treated as mutating and require confirmation)
		for _, toolDef := range client.ListTools() {
			adapter := &mcpToolAdapter{
				name:        fmt.Sprintf("mcp_%s_%s", cfg.Name, toolDef.Name),
				remoteName:  toolDef.Name,
				description: fmt.Sprintf("[MCP:%s] %s", cfg.Name, toolDef.Description),
				schema:      toolDef.InputSchema,
				client:      client,
			}
			m.toolRegistry.Register(adapter)
		}

		glog.Infof("Connected to MCP server %s at %s", cfg.Name, cfg.URL)
	}
}

// ListServers returns the status of all configured MCP servers.
func (m *MCPManager) ListServers() []MCPServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var servers []MCPServerStatus
	for _, cfg := range m.configs {
		status := MCPServerStatus{
			Name: cfg.Name,
			URL:  cfg.URL,
		}

		client, ok := m.clients[cfg.Name]
		if ok {
			status.Status = "connected"
			for _, t := range client.ListTools() {
				status.Tools = append(status.Tools, t.Name)
			}
		} else {
			status.Status = "disconnected"
		}

		servers = append(servers, status)
	}
	return servers
}

// GetServer returns the MCP server for exposing KFP tools.
func (m *MCPManager) GetServer() *MCPServer {
	return m.mcpServer
}

// Close disconnects all MCP clients.
func (m *MCPManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, client := range m.clients {
		client.Close()
		glog.Infof("Disconnected from MCP server %s", name)
	}
	m.clients = make(map[string]*MCPClient)
}
