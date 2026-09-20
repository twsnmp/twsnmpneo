package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// MCPConfig defines options for the embedded MCP server.
type MCPConfig struct {
	Store    datastore.DataStore
	LogStore *parquet.Store
	Version  string
	Endpoint string
}

// MCPServer wraps the official Model Context Protocol server.
type MCPServer struct {
	store    datastore.DataStore
	logStore *parquet.Store
	version  string
	endpoint string
	server   *mcp.Server
	mu       sync.RWMutex
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

// NewMCPServer initializes the MCP server and registers all monitoring tools.
func NewMCPServer(cfg MCPConfig) *MCPServer {
	s := &MCPServer{
		store:    cfg.Store,
		logStore: cfg.LogStore,
		version:  cfg.Version,
		endpoint: cfg.Endpoint,
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "twsnmpneo",
			Version: cfg.Version,
		},
		nil,
	)

	s.registerTools(server)
	s.server = server
	return s
}

// Tool parameter structures
type emptyParams struct{}

type nodeDetailParams struct {
	NodeID string `json:"node_id" jsonschema:"Node ID to retrieve details for"`
}

type queryLogsParams struct {
	Type   string `json:"type,omitempty" jsonschema:"Log type"`
	Filter string `json:"filter,omitempty" jsonschema:"Keyword or regex filter"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum records to return"`
}

func (s *MCPServer) registerTools(server *mcp.Server) {
	// 1. get_system_status
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_status",
		Description: "Retrieve system health metrics, daemon summary, and managed node counts",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emptyParams) (*mcp.CallToolResult, any, error) {
		nodeCount := 0
		if s.store != nil {
			nodes, _ := s.store.ListNodes(ctx)
			nodeCount = len(nodes)
		}
		status := map[string]interface{}{
			"version":    s.version,
			"node_count": nodeCount,
			"status":     "healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 2. list_nodes
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_nodes",
		Description: "List all managed network nodes with their current states and IP addresses",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emptyParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return textResult("[]"), nil, nil
		}
		nodes, err := s.store.ListNodes(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list nodes: %w", err)
		}
		type summary struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			IP    string `json:"ip"`
			State string `json:"state"`
		}
		list := make([]summary, len(nodes))
		for i, n := range nodes {
			list[i] = summary{
				ID:    n.ID,
				Name:  n.Name,
				IP:    n.IP,
				State: n.State,
			}
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 3. get_node_detail
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node_detail",
		Description: "Retrieve detailed node properties and its active monitoring polling tasks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args nodeDetailParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.NodeID == "" {
			return textResult("{}"), nil, nil
		}
		node, err := s.store.GetNode(ctx, args.NodeID)
		if err != nil {
			return nil, nil, fmt.Errorf("get node: %w", err)
		}
		allPolls, _ := s.store.ListPollings(ctx)
		nodePolls := make([]*datastore.PollingEnt, 0)
		for _, p := range allPolls {
			if p.NodeID == args.NodeID {
				nodePolls = append(nodePolls, p)
			}
		}
		detail := map[string]interface{}{
			"node":     node,
			"pollings": nodePolls,
		}
		data, _ := json.MarshalIndent(detail, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 4. get_active_alerts
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_active_alerts",
		Description: "Query unresolved anomalies and current alert events",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emptyParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return textResult("[]"), nil, nil
		}
		logs, err := s.store.ListEventLogs(ctx, 50)
		if err != nil {
			return nil, nil, fmt.Errorf("list event logs: %w", err)
		}
		alerts := make([]*datastore.EventLogEnt, 0)
		for _, ev := range logs {
			if strings.EqualFold(ev.Level, "warn") || strings.EqualFold(ev.Level, "high") {
				alerts = append(alerts, ev)
			}
		}
		data, _ := json.MarshalIndent(alerts, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 5. query_logs
	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_logs",
		Description: "Execute structured filter queries against the Parquet data lake",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args queryLogsParams) (*mcp.CallToolResult, any, error) {
		if s.logStore == nil {
			return textResult("[]"), nil, nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 50
		}
		records, err := s.logStore.Query(ctx, parquet.LogFilter{
			Type:   args.Type,
			Filter: args.Filter,
			Limit:  limit,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("query logs: %w", err)
		}
		data, _ := json.MarshalIndent(records, "", "  ")
		return textResult(string(data)), nil, nil
	})
}

// GetServer returns the underlying mcp.Server.
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}

// ServeHTTP provides an SSE / HTTP transport handler for integration into Web API.
func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("MCP request received", "path", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"server":  "twsnmpneo",
		"version": s.version,
		"mcp":     "1.0",
	})
}
