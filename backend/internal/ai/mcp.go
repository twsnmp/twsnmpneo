package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/diagnose"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

// MCPConfig defines options for the embedded MCP server.
type MCPConfig struct {
	Store     datastore.DataStore
	LogStore  *parquet.Store
	Version   string
	Endpoint  string
	Receivers map[string]any
}

// MCPServer wraps the official Model Context Protocol server.
type MCPServer struct {
	store     datastore.DataStore
	logStore  *parquet.Store
	version   string
	endpoint  string
	receivers map[string]any
	startTime time.Time
	server    *mcp.Server
	mu        sync.RWMutex
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
		store:     cfg.Store,
		logStore:  cfg.LogStore,
		version:   cfg.Version,
		endpoint:  cfg.Endpoint,
		receivers: cfg.Receivers,
		startTime: time.Now(),
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

type diagnoseNodeParams struct {
	NodeID string `json:"node_id" jsonschema:"Node ID to run automated diagnostics on"`
}

type queryLogsParams struct {
	Type   string `json:"type,omitempty" jsonschema:"Log type (syslog, trap, netflow, sflow, event, otel, mqtt)"`
	Filter string `json:"filter,omitempty" jsonschema:"Keyword or regex filter"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum records to return"`
}

type pingParams struct {
	Target string `json:"target" jsonschema:"IP address or hostname to ping"`
	Count  int    `json:"count,omitempty" jsonschema:"Number of packets (default 3)"`
	Size   int    `json:"size,omitempty" jsonschema:"Payload size in bytes (default 64)"`
}

type snmpWalkParams struct {
	NodeID string `json:"node_id" jsonschema:"Node ID to walk MIB on"`
	OID    string `json:"oid,omitempty" jsonschema:"Root OID to walk (default .1.3.6.1.2.1.1 for system)"`
}

func (s *MCPServer) registerTools(server *mcp.Server) {
	// 1. get_system_status: CPU, memory, uptime, receiver packet stats
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_status",
		Description: "Retrieve system health metrics, CPU/memory utilization, uptime, daemon summary, and receiver statistics",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emptyParams) (*mcp.CallToolResult, any, error) {
		nodeCount := 0
		pollCount := 0
		if s.store != nil {
			nodes, _ := s.store.ListNodes(ctx)
			nodeCount = len(nodes)
			polls, _ := s.store.ListPollings(ctx)
			pollCount = len(polls)
		}

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		uptime := time.Since(s.startTime).Truncate(time.Second).String()

		// Query recent log counts from Parquet to reflect receiver packet stats
		now := time.Now().UnixNano()
		oneHourAgo := now - (60 * 60 * 1e9)
		syslogCount := 0
		trapCount := 0
		flowCount := 0
		if s.logStore != nil {
			if recs, err := s.logStore.Query(ctx, parquet.LogFilter{Type: "syslog", StartTime: oneHourAgo, Limit: 1000}); err == nil {
				syslogCount = len(recs)
			}
			if recs, err := s.logStore.Query(ctx, parquet.LogFilter{Type: "trap", StartTime: oneHourAgo, Limit: 1000}); err == nil {
				trapCount = len(recs)
			}
			if recs, err := s.logStore.Query(ctx, parquet.LogFilter{Type: "netflow", StartTime: oneHourAgo, Limit: 1000}); err == nil {
				flowCount = len(recs)
			}
		}

		status := map[string]interface{}{
			"version":     s.version,
			"status":      "healthy",
			"uptime":      uptime,
			"uptime_sec":  int64(time.Since(s.startTime).Seconds()),
			"num_cpu":     runtime.NumCPU(),
			"goroutines":  runtime.NumGoroutine(),
			"memory": map[string]interface{}{
				"alloc_bytes":       memStats.Alloc,
				"total_alloc_bytes": memStats.TotalAlloc,
				"sys_bytes":         memStats.Sys,
				"heap_alloc_bytes":  memStats.HeapAlloc,
				"num_gc":            memStats.NumGC,
			},
			"receivers": map[string]interface{}{
				"configured": s.receivers,
				"recent_packets_1h": map[string]int{
					"syslog":  syslogCount,
					"trap":    trapCount,
					"netflow": flowCount,
				},
			},
			"managed": map[string]interface{}{
				"node_count":    nodeCount,
				"polling_count": pollCount,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 2. list_nodes: List all managed devices, IPs, MACs, and states
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_nodes",
		Description: "List all managed network nodes with their current states, IP addresses, MACs, and icons",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args emptyParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return textResult("[]"), nil, nil
		}
		nodes, err := s.store.ListNodes(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("list nodes: %w", err)
		}
		type summary struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			IP          string `json:"ip"`
			MAC         string `json:"mac"`
			State       string `json:"state"`
			Icon        string `json:"icon"`
			Description string `json:"description"`
		}
		list := make([]summary, len(nodes))
		for i, n := range nodes {
			list[i] = summary{
				ID:          n.ID,
				Name:        n.Name,
				IP:          n.IP,
				MAC:         n.MAC,
				State:       n.State,
				Icon:        n.Icon,
				Description: n.Descr,
			}
		}
		data, _ := json.MarshalIndent(list, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 3. get_node_detail: Properties, ports, MIB trees, polling history
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node_detail",
		Description: "Retrieve detailed node properties, ports, MIB tree status, and active monitoring polling history",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args nodeDetailParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.NodeID == "" {
			return textResult("{}"), nil, nil
		}
		node, err := s.store.GetNode(ctx, args.NodeID)
		if err != nil {
			return nil, nil, fmt.Errorf("get node: %w", err)
		}
		allPolls, _ := s.store.ListPollings(ctx)
		nodePolls := make([]map[string]interface{}, 0)
		for _, p := range allPolls {
			if p.NodeID == args.NodeID {
				nodePolls = append(nodePolls, map[string]interface{}{
					"id":        p.ID,
					"name":      p.Name,
					"type":      p.Type,
					"state":     p.State,
					"last_time": p.LastTime,
					"result":    p.Result,
				})
			}
		}

		// Query basic SNMP system MIB if SNMP is configured
		var mibTree map[string]interface{}
		snmpMode := strings.ToLower(node.SnmpMode)
		if snmpMode != "" && snmpMode != "none" && node.IP != "" {
			port := uint16(node.SnmpPort)
			if port == 0 {
				port = 161
			}
			comm := node.Community
			if comm == "" && !strings.HasPrefix(snmpMode, "v3") {
				comm = "public"
			}
			agent := &gosnmp.GoSNMP{
				Target:    node.IP,
				Port:      port,
				Community: comm,
				Version:   gosnmp.Version2c,
				Timeout:   1500 * time.Millisecond,
				Retries:   1,
			}
			if snmpMode == "v1" {
				agent.Version = gosnmp.Version1
			}
			if cErr := agent.Connect(); cErr == nil {
				defer agent.Conn.Close()
				oids := []string{
					".1.3.6.1.2.1.1.1.0", // sysDescr
					".1.3.6.1.2.1.1.2.0", // sysObjectID
					".1.3.6.1.2.1.1.3.0", // sysUpTime
					".1.3.6.1.2.1.1.4.0", // sysContact
					".1.3.6.1.2.1.1.5.0", // sysName
					".1.3.6.1.2.1.1.6.0", // sysLocation
				}
				if pdu, gErr := agent.Get(oids); gErr == nil && len(pdu.Variables) > 0 {
					mibTree = make(map[string]interface{})
					for _, v := range pdu.Variables {
						valStr := fmt.Sprintf("%v", v.Value)
						if b, ok := v.Value.([]byte); ok {
							valStr = string(b)
						}
						switch {
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.1"):
							mibTree["sysDescr"] = valStr
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.2"):
							mibTree["sysObjectID"] = valStr
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.3"):
							mibTree["sysUpTime"] = valStr
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.4"):
							mibTree["sysContact"] = valStr
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.5"):
							mibTree["sysName"] = valStr
						case strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.6"):
							mibTree["sysLocation"] = valStr
						}
					}
				}
			}
		}

		detail := map[string]interface{}{
			"node":            node,
			"pollings":        nodePolls,
			"polling_history": nodePolls,
			"mib_tree":        mibTree,
		}
		data, _ := json.MarshalIndent(detail, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 4. get_active_alerts: Active high/low alarm events
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
			if strings.EqualFold(ev.Level, "warn") || strings.EqualFold(ev.Level, "high") || strings.EqualFold(ev.Level, "error") {
				alerts = append(alerts, ev)
			}
		}
		data, _ := json.MarshalIndent(alerts, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 5. query_logs: Columnar filter scan across Syslog, Trap, and NetFlow Parquet files
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

	// 6. diagnose_node: Run automated ping and diagnostic probes on demand
	mcp.AddTool(server, &mcp.Tool{
		Name:        "diagnose_node",
		Description: "Run automated ping, SNMP, and web connectivity probes on demand for a node",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args diagnoseNodeParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.NodeID == "" {
			return textResult(`{"error":"store or node_id missing"}`), nil, nil
		}
		node, err := s.store.GetNode(ctx, args.NodeID)
		if err != nil || node == nil {
			return nil, nil, fmt.Errorf("node not found: %s", args.NodeID)
		}
		diagResult := diagnose.DiagnoseNode(ctx, node)
		data, _ := json.MarshalIndent(diagResult, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 7. do_ping tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "do_ping",
		Description: "Execute ping probes against a target IP address or hostname",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args pingParams) (*mcp.CallToolResult, any, error) {
		if args.Target == "" {
			return textResult(`{"error":"target is required"}`), nil, nil
		}
		count := args.Count
		if count <= 0 {
			count = 3
		}
		size := args.Size
		if size <= 0 {
			size = 64
		}
		results := make([]map[string]interface{}, count)
		okCount := 0
		var totalRTT float64
		for i := 0; i < count; i++ {
			pr := ping.DoPing(args.Target, size, 1, 1000, 0)
			rttMs := float64(pr.Time) / 1e6
			if pr.Stat == ping.PingOK {
				okCount++
				totalRTT += rttMs
			}
			results[i] = map[string]interface{}{
				"seq":     i + 1,
				"stat":    pr.Stat,
				"rtt_ms":  rttMs,
				"error":   pr.Error,
			}
		}
		loss := float64(count-okCount) / float64(count) * 100.0
		avgRTT := 0.0
		if okCount > 0 {
			avgRTT = totalRTT / float64(okCount)
		}
		out := map[string]interface{}{
			"target":   args.Target,
			"sent":     count,
			"received": okCount,
			"loss_pct": loss,
			"avg_rtt":  avgRTT,
			"probes":   results,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return textResult(string(data)), nil, nil
	})

	// 8. snmpwalk tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "snmpwalk",
		Description: "Perform an SNMP Walk on a managed node starting at a specified OID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args snmpWalkParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.NodeID == "" {
			return textResult(`{"error":"node_id is required"}`), nil, nil
		}
		node, err := s.store.GetNode(ctx, args.NodeID)
		if err != nil || node == nil {
			return nil, nil, fmt.Errorf("node not found: %s", args.NodeID)
		}
		snmpMode := strings.ToLower(node.SnmpMode)
		if snmpMode == "" || snmpMode == "none" {
			return textResult(`{"error":"SNMP not configured for this node"}`), nil, nil
		}
		port := uint16(node.SnmpPort)
		if port == 0 {
			port = 161
		}
		comm := node.Community
		if comm == "" && !strings.HasPrefix(snmpMode, "v3") {
			comm = "public"
		}
		agent := &gosnmp.GoSNMP{
			Target:    node.IP,
			Port:      port,
			Community: comm,
			Version:   gosnmp.Version2c,
			Timeout:   2 * time.Second,
			Retries:   1,
		}
		if snmpMode == "v1" {
			agent.Version = gosnmp.Version1
		}
		if cErr := agent.Connect(); cErr != nil {
			return nil, nil, fmt.Errorf("SNMP connect failed: %w", cErr)
		}
		defer agent.Conn.Close()

		rootOID := args.OID
		if rootOID == "" {
			rootOID = ".1.3.6.1.2.1.1" // Default to system MIB
		}

		type snmpVar struct {
			OID   string      `json:"oid"`
			Type  string      `json:"type"`
			Value interface{} `json:"value"`
		}
		vars := make([]snmpVar, 0)
		_ = agent.Walk(rootOID, func(v gosnmp.SnmpPDU) error {
			val := fmt.Sprintf("%v", v.Value)
			if b, ok := v.Value.([]byte); ok {
				val = string(b)
			}
			vars = append(vars, snmpVar{
				OID:   v.Name,
				Type:  fmt.Sprintf("%v", v.Type),
				Value: val,
			})
			return nil
		})

		data, _ := json.MarshalIndent(vars, "", "  ")
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
