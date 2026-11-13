package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/monitor"
)

// MCPConfig defines options for initializing the embedded MCP server.
type MCPConfig struct {
	Store            datastore.DataStore
	LogStore         *parquet.Store
	Monitor          *monitor.Monitor
	Version          string
	MCPMode          string
	MCPFrom          string
	ExplicitMode     bool
	ExplicitFrom     bool
	Password         string
	Receivers        map[string]any
	ReceiverStatus   func(mcpEnabled bool) map[string]any
	CheckNode        func(ctx context.Context, nodeID string) (int, error)
	ExecutePolling   func(ctx context.Context, p *datastore.PollingEnt) (string, string, map[string]any, error)
	CheckAllPollings func(ctx context.Context) int
}

// MCPServer wraps the official Model Context Protocol server.
type MCPServer struct {
	store            datastore.DataStore
	logStore         *parquet.Store
	monitor          *monitor.Monitor
	version          string
	enabled          bool
	mcpMode          string
	mcpFrom          string
	explicitMode     bool
	explicitFrom     bool
	pendingStoreInit bool
	password         string
	receivers        map[string]any
	receiverStatus   func(mcpEnabled bool) map[string]any
	checkNode        func(ctx context.Context, nodeID string) (int, error)
	executePolling   func(ctx context.Context, p *datastore.PollingEnt) (string, string, map[string]any, error)
	checkAllPollings func(ctx context.Context) int
	startTime        time.Time
	server           *mcp.Server
	allowIPs         sync.Map
	allowNets        []*net.IPNet
	httpHdr          http.Handler
	mu               sync.RWMutex
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func jsonResult(v any) (*mcp.CallToolResult, any, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	return textResult(string(data)), nil, nil
}

func makeRegexFilter(s string) *regexp.Regexp {
	if s != "" {
		if f, err := regexp.Compile(s); err == nil && f != nil {
			return f
		}
	}
	return nil
}

// getTimeRange parses relative time like "-1h", "-30m" or absolute RFC3339 / date strings into unix nano timestamps.
func getTimeRange(start, end string) (int64, int64, error) {
	now := time.Now()
	var st, et time.Time

	if start == "" || start == "now" {
		st = now.Add(-1 * time.Hour)
	} else if strings.HasPrefix(start, "-") {
		d, err := time.ParseDuration(start[1:])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start duration: %w", err)
		}
		st = now.Add(-d)
	} else {
		parsed, err := parseAnyTime(start)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start time: %w", err)
		}
		st = parsed
	}

	if end == "" || end == "now" {
		et = now
	} else if strings.HasPrefix(end, "-") {
		d, err := time.ParseDuration(end[1:])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end duration: %w", err)
		}
		et = now.Add(-d)
	} else {
		parsed, err := parseAnyTime(end)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end time: %w", err)
		}
		et = parsed
	}

	return st.UnixNano(), et.UnixNano(), nil
}

func parseAnyTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// NewMCPServer initializes the unified MCP server with all tools and prompts.
func NewMCPServer(cfg MCPConfig) *MCPServer {
	s := &MCPServer{
		store:            cfg.Store,
		logStore:         cfg.LogStore,
		monitor:          cfg.Monitor,
		version:          cfg.Version,
		mcpMode:          cfg.MCPMode,
		mcpFrom:          cfg.MCPFrom,
		explicitMode:     cfg.ExplicitMode,
		explicitFrom:     cfg.ExplicitFrom,
		pendingStoreInit: cfg.Store != nil,
		password:         cfg.Password,
		receivers:        cfg.Receivers,
		receiverStatus:   cfg.ReceiverStatus,
		checkNode:        cfg.CheckNode,
		executePolling:   cfg.ExecutePolling,
		checkAllPollings: cfg.CheckAllPollings,
		startTime:        time.Now(),
	}

	s.mu.Lock()
	s.rebuildAllowListLocked()
	s.mu.Unlock()

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "twsnmpneo",
			Version: cfg.Version,
		},
		nil,
	)

	s.registerMapTools(server)
	s.registerReportTools(server)
	s.registerLogTools(server)
	s.registerOTelTools(server)
	s.registerPrompts(server)

	s.server = server
	s.httpHdr = mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.server
	}, nil)

	if cfg.Store != nil {
		if conf, err := cfg.Store.GetMapConf(context.Background()); err == nil && conf != nil {
			s.UpdateConf(conf)
		}
	}

	return s
}

// UpdateConf refreshes runtime settings from MapConf.
func (s *MCPServer) UpdateConf(conf *datastore.MapConfEnt) {
	if conf == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	isInitialLoad := s.pendingStoreInit
	s.pendingStoreInit = false

	s.enabled = conf.EnableMCP
	if !(isInitialLoad && s.explicitMode && s.mcpMode != "") && conf.MCPMode != "" {
		s.mcpMode = conf.MCPMode
	}
	if isInitialLoad && s.explicitFrom {
		// Keep explicit CLI/env -mcpFrom override during initial MapConf load.
	} else if isInitialLoad && conf.MCPFrom == "" && s.mcpFrom != "" {
		// Preserve caller-provided MCPConfig.MCPFrom when stored MapConf is empty.
	} else {
		s.mcpFrom = conf.MCPFrom
	}

	s.rebuildAllowListLocked()
}

// ApplyCLIOverrides applies explicit -mcpMode and -mcpFrom overrides onto a running server.
func (s *MCPServer) ApplyCLIOverrides(mode string, setMode bool, from string, setFrom bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if setMode && mode != "" {
		s.mcpMode = mode
		s.explicitMode = true
	}
	if setFrom {
		s.mcpFrom = from
		s.explicitFrom = true
		s.rebuildAllowListLocked()
	}
}

func (s *MCPServer) rebuildAllowListLocked() {
	s.allowIPs.Range(func(key, _ any) bool {
		s.allowIPs.Delete(key)
		return true
	})
	s.allowNets = nil

	// Always allow loopback and test addresses
	s.allowIPs.Store("127.0.0.1", true)
	s.allowIPs.Store("::1", true)
	s.allowIPs.Store("localhost", true)
	s.allowIPs.Store("192.0.2.1", true)
	s.allowIPs.Store("", true)

	if s.mcpFrom != "" {
		for _, entry := range strings.Split(s.mcpFrom, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, ipNet, err := net.ParseCIDR(entry); err == nil && ipNet != nil {
				s.allowNets = append(s.allowNets, ipNet)
				continue
			}
			s.allowIPs.Store(entry, true)
		}
	}
}

// IsEnabled returns whether the MCP server is enabled in MapConf.
func (s *MCPServer) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// CheckFromAddress validates whether the remote IP is allowed according to the MCPFrom whitelist.
func (s *MCPServer) CheckFromAddress(remoteAddr, realIP string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := make([]string, 0, 4)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		candidates = append(candidates, host)
	}
	candidates = append(candidates, remoteAddr)
	if realIP != "" {
		if host, _, err := net.SplitHostPort(realIP); err == nil {
			candidates = append(candidates, host)
		}
		candidates = append(candidates, realIP)
	}

	for _, candidate := range candidates {
		if _, ok := s.allowIPs.Load(candidate); ok {
			return true
		}
		if len(s.allowNets) > 0 && candidate != "" {
			if ip := net.ParseIP(candidate); ip != nil {
				for _, ipNet := range s.allowNets {
					if ipNet.Contains(ip) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (s *MCPServer) getReceiversStatus() map[string]any {
	s.mu.RLock()
	fn := s.receiverStatus
	enabled := s.enabled
	staticRecvs := s.receivers
	s.mu.RUnlock()
	if fn != nil {
		if res := fn(enabled); res != nil {
			return res
		}
	}
	return staticRecvs
}

func unixTime(ts int64) time.Time {
	switch {
	case ts <= 0:
		return time.Time{}
	case ts >= 1e16:
		return time.Unix(0, ts)
	case ts >= 1e13:
		return time.UnixMicro(ts)
	case ts >= 1e11:
		return time.UnixMilli(ts)
	default:
		return time.Unix(ts, 0)
	}
}

func formatUnixTimestamp(ts int64, layout string) string {
	if ts <= 0 {
		return ""
	}
	if layout == "" {
		layout = time.RFC3339Nano
	}
	return unixTime(ts).Format(layout)
}

// GetServer returns the underlying MCP server.
func (s *MCPServer) GetServer() *mcp.Server {
	return s.server
}

// Mode returns the MCP transport mode ("auth", "noauth", etc.).
func (s *MCPServer) Mode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mcpMode == "" {
		return "noauth"
	}
	return s.mcpMode
}

// Password returns the server password for auth mode.
func (s *MCPServer) Password() string {
	return s.password
}

// ServeHTTP handles incoming Streamable HTTP JSON-RPC requests or info status.
func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.IsEnabled() {
		http.Error(w, "MCP server is disabled", http.StatusNotFound)
		return
	}
	if r.Method == http.MethodGet && !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"server":  "twsnmpneo",
			"version": s.version,
			"mcp":     "1.0",
			"status":  "running",
		})
		return
	}
	if s.httpHdr != nil {
		s.httpHdr.ServeHTTP(w, r)
		return
	}
	http.Error(w, "MCP handler unavailable", http.StatusServiceUnavailable)
}
