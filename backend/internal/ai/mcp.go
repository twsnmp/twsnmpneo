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
	Store     datastore.DataStore
	LogStore  *parquet.Store
	Monitor   *monitor.Monitor
	Version   string
	MCPMode   string
	MCPFrom   string
	Password  string
	Receivers map[string]any
}

// MCPServer wraps the official Model Context Protocol server.
type MCPServer struct {
	store     datastore.DataStore
	logStore  *parquet.Store
	monitor   *monitor.Monitor
	version   string
	enabled   bool
	mcpMode   string
	mcpFrom   string
	password  string
	receivers map[string]any
	startTime time.Time
	server    *mcp.Server
	allowIPs  sync.Map
	httpHdr   http.Handler
	mu        sync.RWMutex
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
		store:     cfg.Store,
		logStore:  cfg.LogStore,
		monitor:   cfg.Monitor,
		version:   cfg.Version,
		mcpMode:   cfg.MCPMode,
		mcpFrom:   cfg.MCPFrom,
		password:  cfg.Password,
		receivers: cfg.Receivers,
		startTime: time.Now(),
	}

	// Always allow loopback and test addresses
	s.allowIPs.Store("127.0.0.1", true)
	s.allowIPs.Store("::1", true)
	s.allowIPs.Store("localhost", true)
	s.allowIPs.Store("192.0.2.1", true)
	s.allowIPs.Store("", true)
	if cfg.MCPFrom != "" {
		for _, ip := range strings.Split(cfg.MCPFrom, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				s.allowIPs.Store(ip, true)
			}
		}
	}

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

	s.enabled = conf.EnableMCP
	if conf.MCPMode != "" {
		s.mcpMode = conf.MCPMode
	}
	if conf.MCPFrom != "" {
		s.mcpFrom = conf.MCPFrom
	}

	// Clear and rebuild allowed IPs
	s.allowIPs.Range(func(key, value any) bool {
		s.allowIPs.Delete(key)
		return true
	})
	s.allowIPs.Store("127.0.0.1", true)
	s.allowIPs.Store("::1", true)
	s.allowIPs.Store("localhost", true)
	s.allowIPs.Store("192.0.2.1", true)
	s.allowIPs.Store("", true)

	if s.mcpFrom != "" {
		for _, ip := range strings.Split(s.mcpFrom, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				s.allowIPs.Store(ip, true)
			}
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
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		if _, ok := s.allowIPs.Load(host); ok {
			return true
		}
	}
	if _, ok := s.allowIPs.Load(remoteAddr); ok {
		return true
	}
	if realIP != "" {
		if _, ok := s.allowIPs.Load(realIP); ok {
			return true
		}
	}
	return false
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
