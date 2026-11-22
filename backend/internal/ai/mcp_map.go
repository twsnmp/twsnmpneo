package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"github.com/twsnmp/twsnmpneo/backend/internal/diagnose"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

type mcpNodeEnt struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IP          string `json:"ip"`
	MAC         string `json:"mac"`
	State       string `json:"state"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type mcpGetNodeListParams struct {
	NameFilter  string `json:"name_filter,omitempty" jsonschema:"name_filter specifies the search criteria for node names using regular expressions. If blank, all nodes are searched."`
	IPFilter    string `json:"ip_filter,omitempty" jsonschema:"ip_filter specifies the search criteria for node IP address using regular expressions. If blank, all nodes are searched."`
	StateFilter string `json:"state_filter,omitempty" jsonschema:"state_filter uses a regular expression to specify search criteria for node state names (normal,warn,low,high,repair,unknown). If blank, all nodes are searched."`
}

type mcpPortEnt struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type mcpNetworkEnt struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	IP          string       `json:"ip"`
	Ports       []mcpPortEnt `json:"ports"`
	X           int          `json:"x"`
	Y           int          `json:"y"`
	Description string       `json:"description"`
}

type mcpGetNetworkListParams struct {
	NameFilter string `json:"name_filter,omitempty" jsonschema:"name_filter specifies the search criteria for network names using regular expressions. If blank, all networks are searched."`
	IPFilter   string `json:"ip_filter,omitempty" jsonschema:"ip_filter specifies the search criteria for network IP address using regular expressions. If blank, all networks are searched."`
}

type mcpPollingEnt struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	NodeName string         `json:"node_name"`
	NodeID   string         `json:"node_id"`
	Type     string         `json:"type"`
	Level    string         `json:"level"`
	State    string         `json:"state"`
	LastTime string         `json:"last_time"`
	Result   map[string]any `json:"result"`
}

type mcpGetPollingListParams struct {
	NodeID         string `json:"node_id,omitempty" jsonschema:"Target node ID to filter pollings"`
	TypeFilter     string `json:"type_filter,omitempty" jsonschema:"type_filter uses a regular expression to specify search criteria for polling type names (ping,snmp,http,tcp,dns,syslog,tls,etc.)."`
	NameFilter     string `json:"name_filter,omitempty" jsonschema:"name_filter specifies search criteria for polling names using regular expressions."`
	NodeNameFilter string `json:"node_name_filter,omitempty" jsonschema:"node_name_filter specifies search criteria for node names of pollings using regular expressions."`
	StateFilter    string `json:"state_filter,omitempty" jsonschema:"state_filter uses a regular expression to specify search criteria for polling state (normal,warn,low,high,repair,unknown)."`
	LevelFilter    string `json:"level_filter,omitempty" jsonschema:"level_filter specifies criteria for polling level (info,warn,low,high)."`
}

type mcpPollingLogEnt struct {
	Time   string         `json:"time"`
	State  string         `json:"state"`
	Result map[string]any `json:"result"`
}

type mcpGetPollingLogParams struct {
	ID    string `json:"id" jsonschema:"The ID of the polling to retrieve logs for"`
	Limit int    `json:"limit,omitempty" jsonschema:"Limit on number of logs retrieved (100 - 2000, default 100)"`
}

type mcpDoPingParams struct {
	Target  string `json:"target" jsonschema:"target IP address, hostname, or node name to ping"`
	Size    int    `json:"size,omitempty" jsonschema:"ping packet size (default 64)"`
	Count   int    `json:"count,omitempty" jsonschema:"number of ping packets to send (default 3)"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"ping timeout in seconds (default 1)"`
}

type mcpMIBEnt struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type mcpSnmpWalkParams struct {
	Target    string `json:"target" jsonschema:"target IP address, hostname, or node name"`
	NodeID    string `json:"node_id,omitempty" jsonschema:"node ID to look up SNMP configuration"`
	RootOID   string `json:"root_oid,omitempty" jsonschema:"root OID or MIB name to walk from (default .1.3.6.1.2.1.1)"`
	Community string `json:"community,omitempty" jsonschema:"community name for SNMP v2c mode"`
	User      string `json:"user,omitempty" jsonschema:"user name for SNMP v3 mode"`
	Password  string `json:"password,omitempty" jsonschema:"password for SNMP v3 mode"`
	SnmpMode  string `json:"snmp_mode,omitempty" jsonschema:"snmp mode (v1, v2c, v3auth, v3authpriv, v3authprivex)"`
}

type mcpSnmpSetParams struct {
	Target        string `json:"target" jsonschema:"target IP address, hostname, or node name"`
	NodeID        string `json:"node_id,omitempty" jsonschema:"node ID to look up SNMP configuration"`
	MIBObjectName string `json:"mib_object_name" jsonschema:"MIB object name or OID to set"`
	Type          string `json:"type" jsonschema:"Type of set value (integer or string)"`
	Value         string `json:"value" jsonschema:"Value to set"`
	Community     string `json:"community,omitempty" jsonschema:"community name for SNMP v2c mode"`
	User          string `json:"user,omitempty" jsonschema:"user name for SNMP v3 mode"`
	Password      string `json:"password,omitempty" jsonschema:"password for SNMP v3 mode"`
	SnmpMode      string `json:"snmp_mode,omitempty" jsonschema:"snmp mode (v1, v2c, v3auth, v3authpriv, v3authprivex)"`
}

type mcpAddNodeParams struct {
	Name        string `json:"name" jsonschema:"node name"`
	IP          string `json:"ip" jsonschema:"node IP address"`
	Icon        string `json:"icon,omitempty" jsonschema:"icon of node (default 'desktop')"`
	Description string `json:"description,omitempty" jsonschema:"description of node"`
	X           int    `json:"x,omitempty" jsonschema:"X position of node on map (64 - 1000)"`
	Y           int    `json:"y,omitempty" jsonschema:"Y position of node on map (64 - 1000)"`
}

type mcpUpdateNodeParams struct {
	ID          string `json:"id" jsonschema:"node ID, current name, or current IP"`
	Name        string `json:"name,omitempty" jsonschema:"new node name"`
	IP          string `json:"ip,omitempty" jsonschema:"new IP address"`
	Icon        string `json:"icon,omitempty" jsonschema:"new icon"`
	Description string `json:"description,omitempty" jsonschema:"new description of node"`
	X           int    `json:"x,omitempty" jsonschema:"X position of node on map (64 - 1000, 0 to skip)"`
	Y           int    `json:"y,omitempty" jsonschema:"Y position of node on map (64 - 1000, 0 to skip)"`
}

func (s *MCPServer) findNode(ctx context.Context, key string) (*datastore.NodeEnt, error) {
	if s.store == nil || key == "" {
		return nil, fmt.Errorf("node not found")
	}
	if n, err := s.store.GetNode(ctx, key); err == nil && n != nil {
		return n, nil
	}
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, n := range nodes {
		if n.ID == key || n.Name == key || n.IP == key {
			return n, nil
		}
	}
	return nil, fmt.Errorf("node not found: %s", key)
}

func (s *MCPServer) resolveTargetIP(ctx context.Context, target string) (string, *datastore.NodeEnt) {
	if s.store != nil {
		if n, err := s.findNode(ctx, target); err == nil && n != nil {
			return n.IP, n
		}
	}
	if ip := net.ParseIP(target); ip != nil {
		return target, nil
	}
	if ips, err := net.LookupHost(target); err == nil && len(ips) > 0 {
		return ips[0], nil
	}
	return target, nil
}

type mcpNodeDetailEnt struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	IP           string          `json:"ip"`
	MAC          string          `json:"mac"`
	Vendor       string          `json:"vendor,omitempty"`
	State        string          `json:"state"`
	X            int             `json:"x"`
	Y            int             `json:"y"`
	Icon         string          `json:"icon"`
	Description  string          `json:"description"`
	SnmpMode     string          `json:"snmp_mode,omitempty"`
	URL          string          `json:"url,omitempty"`
	AddrMode     string          `json:"addr_mode,omitempty"`
	AutoAck      bool            `json:"auto_ack"`
	PollingCount int             `json:"polling_count"`
	Pollings     []mcpPollingEnt `json:"pollings,omitempty"`
}

type mcpGetNodeParams struct {
	ID string `json:"id" jsonschema:"node ID, node name, or IP address"`
}

type mcpDeleteNodeParams struct {
	ID string `json:"id" jsonschema:"node ID or unique node name to delete"`
}

type mcpLineEnt struct {
	ID           string `json:"id"`
	NodeID1      string `json:"node_id1"`
	NodeName1    string `json:"node_name1,omitempty"`
	PollingID1   string `json:"polling_id1,omitempty"`
	PollingName1 string `json:"polling_name1,omitempty"`
	State1       string `json:"state1"`
	NodeID2      string `json:"node_id2"`
	NodeName2    string `json:"node_name2,omitempty"`
	PollingID2   string `json:"polling_id2,omitempty"`
	PollingName2 string `json:"polling_name2,omitempty"`
	State2       string `json:"state2"`
	PollingID    string `json:"polling_id,omitempty"`
	State        string `json:"state"`
	Width        int    `json:"width"`
	Port         string `json:"port,omitempty"`
	Info         string `json:"info,omitempty"`
}

type mcpGetLineListParams struct {
	NodeFilter  string `json:"node_filter,omitempty" jsonschema:"regex filter matching either connected node ID or node name"`
	StateFilter string `json:"state_filter,omitempty" jsonschema:"regex filter matching line state, state1, or state2 (normal, warn, low, high, repair, unknown)"`
}

type mcpAddLineParams struct {
	NodeID1    string `json:"node_id1" jsonschema:"first node ID, unique node name, or NET:<network_id>"`
	PollingID1 string `json:"polling_id1,omitempty" jsonschema:"polling ID on first node"`
	NodeID2    string `json:"node_id2" jsonschema:"second node ID, unique node name, or NET:<network_id>"`
	PollingID2 string `json:"polling_id2,omitempty" jsonschema:"polling ID on second node"`
	PollingID  string `json:"polling_id,omitempty" jsonschema:"optional bandwidth/traffic polling ID bound to the line"`
	Width      int    `json:"width,omitempty" jsonschema:"line width in pixels (default 2)"`
	Port       string `json:"port,omitempty" jsonschema:"port label"`
	Info       string `json:"info,omitempty" jsonschema:"line description or bandwidth label"`
}

type mcpDeleteLineParams struct {
	ID string `json:"id" jsonschema:"line ID to delete"`
}

type mcpPollingDetailEnt struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	NodeID       string         `json:"node_id"`
	NodeName     string         `json:"node_name"`
	Type         string         `json:"type"`
	Mode         string         `json:"mode,omitempty"`
	Params       string         `json:"params,omitempty"`
	Filter       string         `json:"filter,omitempty"`
	Extractor    string         `json:"extractor,omitempty"`
	Script       string         `json:"script,omitempty"`
	Level        string         `json:"level"`
	PollInt      int            `json:"poll_int"`
	Timeout      int            `json:"timeout"`
	Retry        int            `json:"retry"`
	LogMode      int            `json:"log_mode"`
	LogModeName  string         `json:"log_mode_name"`
	State        string         `json:"state"`
	LastTime     string         `json:"last_time"`
	NextTime     string         `json:"next_time,omitempty"`
	FailAction   string         `json:"fail_action,omitempty"`
	RepairAction string         `json:"repair_action,omitempty"`
	AIMode       string         `json:"ai_mode,omitempty"`
	Result       map[string]any `json:"result,omitempty"`
}

type mcpGetPollingParams struct {
	ID string `json:"id" jsonschema:"polling ID"`
}

type mcpAddPollingParams struct {
	NodeID       string `json:"node_id" jsonschema:"target node ID or unique node name"`
	Name         string `json:"name" jsonschema:"polling display name"`
	Type         string `json:"type" jsonschema:"polling type (ping, snmp, http, tcp, dns, tls, syslog, trap, netflow, sflow, twsnmp, vmware, lxi, gnmi, cmd, mqtt, stun, env)"`
	Mode         string `json:"mode,omitempty" jsonschema:"polling sub-mode (e.g. 'get', 'stats', 'rtt', 'smoke', 'status', 'https', 'expire')"`
	Params       string `json:"params,omitempty" jsonschema:"polling target parameters (e.g. OID/MIB name, URL, port, packet options)"`
	Filter       string `json:"filter,omitempty" jsonschema:"log or value filter expression"`
	Extractor    string `json:"extractor,omitempty" jsonschema:"grok/value extractor expression"`
	Script       string `json:"script,omitempty" jsonschema:"evaluation expression that returns true when NORMAL (e.g. 'rtt < 100 * 1000 * 1000')"`
	Level        string `json:"level,omitempty" jsonschema:"failure alert level (off, info, warn, low, high; default 'low')"`
	PollInt      int    `json:"poll_int,omitempty" jsonschema:"polling interval in seconds (default 60)"`
	Timeout      int    `json:"timeout,omitempty" jsonschema:"timeout in seconds (default 1)"`
	Retry        int    `json:"retry,omitempty" jsonschema:"retry count (default 1)"`
	LogMode      string `json:"log_mode,omitempty" jsonschema:"log recording mode (none, always, on_change, ai; default 'none')"`
	FailAction   string `json:"fail_action,omitempty" jsonschema:"action command/webhook on failure"`
	RepairAction string `json:"repair_action,omitempty" jsonschema:"action command/webhook on recovery"`
	AIMode       string `json:"ai_mode,omitempty" jsonschema:"AI anomaly detection algorithm when log_mode=ai"`
}

type mcpUpdatePollingParams struct {
	ID           string `json:"id" jsonschema:"polling ID to update"`
	Name         string `json:"name,omitempty" jsonschema:"new polling name"`
	Type         string `json:"type,omitempty" jsonschema:"new polling type"`
	Mode         string `json:"mode,omitempty" jsonschema:"new polling sub-mode"`
	Params       string `json:"params,omitempty" jsonschema:"new polling target parameters"`
	Filter       string `json:"filter,omitempty" jsonschema:"new filter expression"`
	Extractor    string `json:"extractor,omitempty" jsonschema:"new extractor expression"`
	Script       string `json:"script,omitempty" jsonschema:"new evaluation script"`
	Level        string `json:"level,omitempty" jsonschema:"new alert level (off, info, warn, low, high)"`
	PollInt      int    `json:"poll_int,omitempty" jsonschema:"new polling interval in seconds"`
	Timeout      int    `json:"timeout,omitempty" jsonschema:"new timeout in seconds"`
	Retry        int    `json:"retry,omitempty" jsonschema:"new retry count"`
	LogMode      string `json:"log_mode,omitempty" jsonschema:"new log mode (none, always, on_change, ai)"`
	FailAction   string `json:"fail_action,omitempty" jsonschema:"new failure action"`
	RepairAction string `json:"repair_action,omitempty" jsonschema:"new recovery action"`
	AIMode       string `json:"ai_mode,omitempty" jsonschema:"new AI anomaly mode"`
	ResetState   bool   `json:"reset_state,omitempty" jsonschema:"reset polling state to unknown and clear fail time"`
}

type mcpDeletePollingParams struct {
	ID string `json:"id" jsonschema:"polling ID to delete"`
}

type mcpCheckPollingParams struct {
	PollingID string `json:"polling_id,omitempty" jsonschema:"specific polling ID to execute immediately"`
	NodeID    string `json:"node_id,omitempty" jsonschema:"node ID or name whose non-normal pollings should be rechecked"`
	All       bool   `json:"all,omitempty" jsonschema:"set true to trigger recheck across all non-normal pollings"`
}

type mcpAIAnomalyEnt struct {
	PollingID string  `json:"polling_id"`
	Polling   string  `json:"polling"`
	NodeID    string  `json:"node_id"`
	Node      string  `json:"node"`
	Score     float64 `json:"score"`
	Count     int     `json:"count"`
	LastTime  string  `json:"last_time"`
}

type mcpGetAIAnomalyListParams struct {
	NodeFilter    string  `json:"node_filter,omitempty" jsonschema:"regex filter for node name"`
	PollingFilter string  `json:"polling_filter,omitempty" jsonschema:"regex filter for polling name"`
	MinScore      float64 `json:"min_score,omitempty" jsonschema:"minimum anomaly score threshold"`
	Limit         int     `json:"limit,omitempty" jsonschema:"maximum number of results (1 - 500, default 100)"`
}

func toMCPNodeEnt(n *datastore.NodeEnt) mcpNodeEnt {
	return mcpNodeEnt{
		ID:          n.ID,
		Name:        n.Name,
		IP:          n.IP,
		MAC:         n.MAC,
		State:       n.State,
		X:           n.X,
		Y:           n.Y,
		Icon:        n.Icon,
		Description: n.Descr,
	}
}

func toMCPPollingEnt(p *datastore.PollingEnt, nodeName string) mcpPollingEnt {
	return mcpPollingEnt{
		ID:       p.ID,
		Name:     p.Name,
		NodeName: nodeName,
		NodeID:   p.NodeID,
		Type:     p.Type,
		Level:    p.Level,
		State:    p.State,
		LastTime: formatUnixTimestamp(p.LastTime, time.RFC3339Nano),
		Result:   p.Result,
	}
}

func pollingLogModeName(mode int) string {
	switch mode {
	case datastore.LogModeAlways:
		return "always"
	case datastore.LogModeOnChange:
		return "on_change"
	case datastore.LogModeAI:
		return "ai"
	default:
		return "none"
	}
}

func parsePollingLogMode(raw string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none", "0":
		return datastore.LogModeNone, nil
	case "always", "1":
		return datastore.LogModeAlways, nil
	case "on_change", "onchange", "2":
		return datastore.LogModeOnChange, nil
	case "ai", "3":
		return datastore.LogModeAI, nil
	default:
		return 0, fmt.Errorf("invalid log_mode %q (expected none, always, on_change, ai)", raw)
	}
}

func validatePollingLevel(level string) error {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "off", "info", "warn", "low", "high":
		return nil
	default:
		return fmt.Errorf("invalid polling level %q (expected off, info, warn, low, high)", level)
	}
}

func toMCPPollingDetailEnt(p *datastore.PollingEnt, nodeName string) mcpPollingDetailEnt {
	return mcpPollingDetailEnt{
		ID:           p.ID,
		Name:         p.Name,
		NodeID:       p.NodeID,
		NodeName:     nodeName,
		Type:         p.Type,
		Mode:         p.Mode,
		Params:       p.Params,
		Filter:       p.Filter,
		Extractor:    p.Extractor,
		Script:       p.Script,
		Level:        p.Level,
		PollInt:      p.PollInt,
		Timeout:      p.Timeout,
		Retry:        p.Retry,
		LogMode:      p.LogMode,
		LogModeName:  pollingLogModeName(p.LogMode),
		State:        p.State,
		LastTime:     formatUnixTimestamp(p.LastTime, time.RFC3339Nano),
		NextTime:     formatUnixTimestamp(p.NextTime, time.RFC3339Nano),
		FailAction:   p.FailAction,
		RepairAction: p.RepairAction,
		AIMode:       p.AIMode,
		Result:       p.Result,
	}
}

// resolveUniqueNode resolves a node by exact ID or unique exact Name (never by IP to avoid ambiguous mutations).
func (s *MCPServer) resolveUniqueNode(ctx context.Context, key string) (*datastore.NodeEnt, error) {
	if s.store == nil || key == "" {
		return nil, fmt.Errorf("node not found")
	}
	if n, err := s.store.GetNode(ctx, key); err == nil && n != nil {
		return n, nil
	}
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	var matches []*datastore.NodeEnt
	for _, n := range nodes {
		if n.Name == key {
			matches = append(matches, n)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("ambiguous node name %q matches %d nodes; specify node ID", key, len(matches))
	}
	return nil, fmt.Errorf("node not found: %s", key)
}

func (s *MCPServer) buildLineEntity(ctx context.Context, l *datastore.LineEnt, nodeMap map[string]string, netMap map[string]string, pollMap map[string]string) mcpLineEnt {
	resolveEndpointName := func(id string) string {
		if strings.HasPrefix(id, "NET:") {
			netID := strings.TrimPrefix(id, "NET:")
			if name, ok := netMap[netID]; ok {
				return name
			}
			return id
		}
		if name, ok := nodeMap[id]; ok {
			return name
		}
		return ""
	}
	return mcpLineEnt{
		ID:           l.ID,
		NodeID1:      l.NodeID1,
		NodeName1:    resolveEndpointName(l.NodeID1),
		PollingID1:   l.PollingID1,
		PollingName1: pollMap[l.PollingID1],
		State1:       l.State1,
		NodeID2:      l.NodeID2,
		NodeName2:    resolveEndpointName(l.NodeID2),
		PollingID2:   l.PollingID2,
		PollingName2: pollMap[l.PollingID2],
		State2:       l.State2,
		PollingID:    l.PollingID,
		State:        l.State,
		Width:        l.Width,
		Port:         l.Port,
		Info:         l.Info,
	}
}

func isNumericOID(oid string) bool {
	if !strings.HasPrefix(oid, ".") || len(oid) < 2 {
		return false
	}
	parts := strings.Split(oid[1:], ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
		}
	}
	return true
}

func resolveSNMPOID(name, defaultOID string) (string, error) {
	oid := strings.TrimSpace(name)
	if oid == "" {
		if defaultOID == "" {
			return "", fmt.Errorf("OID or MIB object name is required")
		}
		return defaultOID, nil
	}
	if !strings.HasPrefix(oid, ".") {
		oid = mib.NameToOID(oid)
	}
	if !isNumericOID(oid) {
		return "", fmt.Errorf("unknown MIB object name or invalid OID: %s", name)
	}
	return oid, nil
}

func (s *MCPServer) buildSNMPAgent(ctx context.Context, target, nodeID, comm, user, pass, mode string) (*gosnmp.GoSNMP, error) {
	if target == "" && nodeID != "" {
		target = nodeID
	}
	if target == "" {
		return nil, fmt.Errorf("target or node_id is required")
	}
	targetIP, node := s.resolveTargetIP(ctx, target)
	if node == nil && nodeID != "" {
		if n, err := s.findNode(ctx, nodeID); err == nil {
			node = n
		}
	}

	port := uint16(161)
	if node != nil {
		if comm == "" {
			comm = node.Community
		}
		if user == "" {
			user = node.User
		}
		if pass == "" {
			pass = node.Password
		}
		if mode == "" {
			mode = node.SnmpMode
		}
		if node.SnmpPort > 0 {
			port = uint16(node.SnmpPort)
		}
	}

	modeLower := strings.ToLower(strings.TrimSpace(mode))
	if !strings.HasPrefix(modeLower, "v3") && comm == "" {
		comm = "public"
	}

	agent := &gosnmp.GoSNMP{
		Target:    targetIP,
		Port:      port,
		Community: comm,
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   1,
	}

	switch modeLower {
	case "", "v2", "v2c":
		agent.Version = gosnmp.Version2c
	case "v1":
		agent.Version = gosnmp.Version1
	case "v3", "v3noauth":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.NoAuthNoPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: user,
		}
	case "v3auth":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthNoPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 user,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: pass,
		}
	case "v3authpriv":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 user,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: pass,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        pass,
		}
	case "v3authprivex":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 user,
			AuthenticationProtocol:   gosnmp.SHA256,
			AuthenticationPassphrase: pass,
			PrivacyProtocol:          gosnmp.AES256,
			PrivacyPassphrase:        pass,
		}
	default:
		return nil, fmt.Errorf("unsupported snmp_mode: %s", mode)
	}

	return agent, nil
}

func (s *MCPServer) connectSNMPAgent(ctx context.Context, target, nodeID, comm, user, pass, mode string) (*gosnmp.GoSNMP, error) {
	agent, err := s.buildSNMPAgent(ctx, target, nodeID, comm, user, pass, mode)
	if err != nil {
		return nil, err
	}
	if err := agent.Connect(); err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	return agent, nil
}

func (s *MCPServer) registerMapTools(server *mcp.Server) {
	// 1. get_node_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node_list",
		Description: "Get list of managed nodes with regex filtering for name, IP, and state",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetNodeListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpNodeEnt{})
		}
		nodes, err := s.store.ListNodes(ctx)
		if err != nil {
			return nil, nil, err
		}
		nameFilter := makeRegexFilter(args.NameFilter)
		ipFilter := makeRegexFilter(args.IPFilter)
		stateFilter := makeRegexFilter(args.StateFilter)

		list := make([]mcpNodeEnt, 0)
		for _, n := range nodes {
			if nameFilter != nil && !nameFilter.MatchString(n.Name) {
				continue
			}
			if ipFilter != nil && !ipFilter.MatchString(n.IP) {
				continue
			}
			if stateFilter != nil && !stateFilter.MatchString(n.State) {
				continue
			}
			list = append(list, toMCPNodeEnt(n))
		}
		return jsonResult(list)
	})

	// 2. get_network_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_network_list",
		Description: "Get list of configured networks and their ports",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetNetworkListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpNetworkEnt{})
		}
		networks, err := s.store.ListNetworks(ctx)
		if err != nil {
			return nil, nil, err
		}
		nameFilter := makeRegexFilter(args.NameFilter)
		ipFilter := makeRegexFilter(args.IPFilter)

		list := make([]mcpNetworkEnt, 0)
		for _, nw := range networks {
			if nameFilter != nil && !nameFilter.MatchString(nw.Name) {
				continue
			}
			if ipFilter != nil && !ipFilter.MatchString(nw.IP) {
				continue
			}
			ports := make([]mcpPortEnt, 0, len(nw.Ports))
			for _, p := range nw.Ports {
				ports = append(ports, mcpPortEnt{
					Name:  p.Name,
					State: p.State,
				})
			}
			list = append(list, mcpNetworkEnt{
				ID:          nw.ID,
				Name:        nw.Name,
				IP:          nw.IP,
				Ports:       ports,
				X:           nw.X,
				Y:           nw.Y,
				Description: nw.Descr,
			})
		}
		return jsonResult(list)
	})

	// 3. get_polling_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_polling_list",
		Description: "Get list of pollings with regex filtering for type, name, node name, state, and level",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetPollingListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpPollingEnt{})
		}
		pollings, err := s.store.ListPollings(ctx)
		if err != nil {
			return nil, nil, err
		}
		nodes, _ := s.store.ListNodes(ctx)
		nodeMap := make(map[string]*datastore.NodeEnt)
		for _, n := range nodes {
			nodeMap[n.ID] = n
		}

		nameFilter := makeRegexFilter(args.NameFilter)
		nodeNameFilter := makeRegexFilter(args.NodeNameFilter)
		typeFilter := makeRegexFilter(args.TypeFilter)
		stateFilter := makeRegexFilter(args.StateFilter)
		levelFilter := makeRegexFilter(args.LevelFilter)

		list := make([]mcpPollingEnt, 0)
		for _, p := range pollings {
			if args.NodeID != "" && p.NodeID != args.NodeID {
				continue
			}
			if nameFilter != nil && !nameFilter.MatchString(p.Name) {
				continue
			}
			if typeFilter != nil && !typeFilter.MatchString(p.Type) {
				continue
			}
			if stateFilter != nil && !stateFilter.MatchString(p.State) {
				continue
			}
			if levelFilter != nil && !levelFilter.MatchString(p.Level) {
				continue
			}
			nodeName := ""
			if n, ok := nodeMap[p.NodeID]; ok {
				nodeName = n.Name
			}
			if nodeNameFilter != nil && !nodeNameFilter.MatchString(nodeName) {
				continue
			}

			list = append(list, toMCPPollingEnt(p, nodeName))
		}
		return jsonResult(list)
	})

	// 4. get_polling_log
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_polling_log",
		Description: "Retrieve recent log records for a specific polling ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetPollingLogParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or polling id missing")
		}
		p, err := s.store.GetPolling(ctx, args.ID)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("polling not found: %s", args.ID)
		}

		limit := args.Limit
		if limit < 10 || limit > 2000 {
			limit = 100
		}

		// Retrieve from Parquet log store or Polling results
		list := make([]mcpPollingLogEnt, 0)
		if s.logStore != nil {
			recs, err := s.logStore.Query(ctx, parquet.LogFilter{
				Type:   "polling",
				Filter: args.ID,
				Limit:  limit,
			})
			if err == nil {
				for _, r := range recs {
					state := "normal"
					var resMap map[string]any
					if err := json.Unmarshal([]byte(r.Log), &resMap); err == nil {
						if s, ok := resMap["state"].(string); ok {
							state = s
						}
					} else {
						resMap = map[string]any{"raw": r.Log}
					}
					list = append(list, mcpPollingLogEnt{
						Time:   time.Unix(0, r.Time).Format(time.RFC3339),
						State:  state,
						Result: resMap,
					})
				}
			}
		}
		if len(list) == 0 && len(p.Result) > 0 {
			list = append(list, mcpPollingLogEnt{
				Time:   time.Unix(0, p.LastTime).Format(time.RFC3339),
				State:  p.State,
				Result: p.Result,
			})
		}

		return jsonResult(list)
	})

	// 5. get_polling_log_data (CSV formatted values for charts)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_polling_log_data",
		Description: "Retrieve structured time-series polling metric data in CSV format",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetPollingLogParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or polling id missing")
		}
		p, err := s.store.GetPolling(ctx, args.ID)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("polling not found: %s", args.ID)
		}
		keys := make([]string, 0)
		for k, v := range p.Result {
			if k == "lastTime" {
				continue
			}
			switch v.(type) {
			case float64, float32, int, int64:
				keys = append(keys, k)
			}
		}
		csv := []string{"time,state," + strings.Join(keys, ",")}
		vals := make([]string, 0, len(keys))
		for _, k := range keys {
			vals = append(vals, fmt.Sprintf("%v", p.Result[k]))
		}
		csv = append(csv, fmt.Sprintf("%s,%s,%s", time.Unix(0, p.LastTime).Format(time.RFC3339), p.State, strings.Join(vals, ",")))
		return textResult(strings.Join(csv, "\n")), nil, nil
	})

	// 6. do_ping
	mcp.AddTool(server, &mcp.Tool{
		Name:        "do_ping",
		Description: "Execute ping probes against a target IP address, hostname, or managed node name",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpDoPingParams) (*mcp.CallToolResult, any, error) {
		if args.Target == "" {
			return nil, nil, fmt.Errorf("target is required")
		}
		targetIP, _ := s.resolveTargetIP(ctx, args.Target)
		count := args.Count
		if count <= 0 {
			count = 3
		}
		size := args.Size
		if size <= 0 {
			size = 64
		}
		timeoutSec := args.Timeout
		if timeoutSec <= 0 {
			timeoutSec = 1
		}

		results := make([]map[string]any, count)
		okCount := 0
		var totalRTT float64
		for i := 0; i < count; i++ {
			pr := ping.DoPing(targetIP, size, 1, timeoutSec*1000, 0)
			rttMs := float64(pr.Time) / 1e6
			if pr.Stat == ping.PingOK {
				okCount++
				totalRTT += rttMs
			}
			results[i] = map[string]any{
				"seq":    i + 1,
				"stat":   pr.Stat,
				"rtt_ms": rttMs,
				"error":  pr.Error,
			}
		}
		loss := float64(count-okCount) / float64(count) * 100.0
		avgRTT := 0.0
		if okCount > 0 {
			avgRTT = totalRTT / float64(okCount)
		}
		out := map[string]any{
			"target":   args.Target,
			"ip":       targetIP,
			"sent":     count,
			"received": okCount,
			"loss_pct": loss,
			"avg_rtt":  avgRTT,
			"probes":   results,
		}
		return jsonResult(out)
	})

	// 7. get_mib_tree
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mib_tree",
		Description: "Retrieve MIB tree hierarchy and definitions from TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		tree := mib.GetMIBTree()
		return jsonResult(tree)
	})

	// 8. snmpwalk
	mcp.AddTool(server, &mcp.Tool{
		Name:        "snmpwalk",
		Description: "Perform an SNMP Walk on a managed node or IP starting at a specified OID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSnmpWalkParams) (*mcp.CallToolResult, any, error) {
		rootOID, err := resolveSNMPOID(args.RootOID, ".1.3.6.1.2.1.1")
		if err != nil {
			return nil, nil, err
		}

		agent, err := s.connectSNMPAgent(ctx, args.Target, args.NodeID, args.Community, args.User, args.Password, args.SnmpMode)
		if err != nil {
			return nil, nil, err
		}
		defer agent.Conn.Close()

		type snmpVar struct {
			Name  string `json:"name"`
			OID   string `json:"oid"`
			Type  string `json:"type"`
			Value string `json:"value"`
		}
		vars := make([]snmpVar, 0)
		if err := agent.Walk(rootOID, func(v gosnmp.SnmpPDU) error {
			name := mib.OIDToName(v.Name)
			valStr := mib.GetMIBValueString(name, &v, false)
			vars = append(vars, snmpVar{
				Name:  name,
				OID:   v.Name,
				Type:  fmt.Sprintf("%v", v.Type),
				Value: valStr,
			})
			return nil
		}); err != nil && len(vars) == 0 {
			return nil, nil, fmt.Errorf("SNMP walk failed: %w", err)
		}

		return jsonResult(vars)
	})

	// 9. snmpset
	mcp.AddTool(server, &mcp.Tool{
		Name:        "snmpset",
		Description: "Set an SNMP MIB variable on a managed node or target IP",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSnmpSetParams) (*mcp.CallToolResult, any, error) {
		oid, err := resolveSNMPOID(args.MIBObjectName, "")
		if err != nil {
			return nil, nil, err
		}

		var pdu gosnmp.SnmpPDU
		switch strings.ToLower(strings.TrimSpace(args.Type)) {
		case "integer", "int":
			valInt, err := strconv.Atoi(args.Value)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid integer value: %w", err)
			}
			pdu = gosnmp.SnmpPDU{
				Name:  oid,
				Type:  gosnmp.Integer,
				Value: valInt,
			}
		case "", "string":
			pdu = gosnmp.SnmpPDU{
				Name:  oid,
				Type:  gosnmp.OctetString,
				Value: []byte(args.Value),
			}
		default:
			return nil, nil, fmt.Errorf("unsupported type %q (expected integer or string)", args.Type)
		}

		agent, err := s.connectSNMPAgent(ctx, args.Target, args.NodeID, args.Community, args.User, args.Password, args.SnmpMode)
		if err != nil {
			return nil, nil, err
		}
		defer agent.Conn.Close()

		resp, err := agent.Set([]gosnmp.SnmpPDU{pdu})
		if err != nil {
			return nil, nil, fmt.Errorf("SNMP set failed: %w", err)
		}
		if resp.Error != gosnmp.NoError {
			return nil, nil, fmt.Errorf("SNMP set error: %s", resp.Error.String())
		}

		res := make([]mcpMIBEnt, 0, len(resp.Variables))
		for _, v := range resp.Variables {
			name := mib.OIDToName(v.Name)
			res = append(res, mcpMIBEnt{
				Name:  name,
				Value: mib.GetMIBValueString(name, &v, false),
			})
		}
		return jsonResult(res)
	})

	// 10. add_node
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_node",
		Description: "Add a new node to TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpAddNodeParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return nil, nil, fmt.Errorf("datastore not available")
		}
		if args.Name == "" || args.IP == "" {
			return nil, nil, fmt.Errorf("name and IP are required")
		}
		icon := args.Icon
		if icon == "" {
			icon = "desktop"
		}
		x := args.X
		if x < 64 || x > 1000 {
			x = 100
		}
		y := args.Y
		if y < 64 || y > 1000 {
			y = 100
		}

		node := &datastore.NodeEnt{
			Name:  args.Name,
			IP:    args.IP,
			Icon:  icon,
			Descr: args.Description,
			X:     x,
			Y:     y,
			State: "unknown",
		}
		if err := s.store.SaveNode(ctx, node); err != nil {
			return nil, nil, fmt.Errorf("save node: %w", err)
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "user",
			Level:    "info",
			NodeID:   node.ID,
			NodeName: node.Name,
			Event:    fmt.Sprintf("Added node %s (%s) via MCP", node.Name, node.IP),
		})

		return jsonResult(toMCPNodeEnt(node))
	})

	// 11. update_node
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_node",
		Description: "Update node name, IP, position, icon, or description",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpUpdateNodeParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return nil, nil, fmt.Errorf("datastore not available")
		}
		node, err := s.findNode(ctx, args.ID)
		if err != nil {
			return nil, nil, err
		}

		if args.Name != "" {
			node.Name = args.Name
		}
		if args.IP != "" {
			node.IP = args.IP
		}
		if args.Icon != "" {
			node.Icon = args.Icon
		}
		if args.Description != "" {
			node.Descr = args.Description
		}
		if args.X >= 64 && args.X <= 1000 {
			node.X = args.X
		}
		if args.Y >= 64 && args.Y <= 1000 {
			node.Y = args.Y
		}

		if err := s.store.SaveNode(ctx, node); err != nil {
			return nil, nil, fmt.Errorf("save node: %w", err)
		}

		return jsonResult(toMCPNodeEnt(node))
	})

	// 12. get_system_status
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_system_status",
		Description: "Retrieve system health metrics, CPU/memory utilization, uptime, daemon summary, and receiver statistics",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
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

		syslogCount := 0
		trapCount := 0
		flowCount := 0
		if s.logStore != nil {
			now := time.Now().UnixNano()
			oneHourAgo := now - (60 * 60 * 1e9)
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

		status := map[string]any{
			"version":     s.version,
			"status":      "healthy",
			"uptime":      uptime,
			"uptime_sec":  int64(time.Since(s.startTime).Seconds()),
			"num_cpu":     runtime.NumCPU(),
			"goroutines":  runtime.NumGoroutine(),
			"memory": map[string]any{
				"alloc_bytes":       memStats.Alloc,
				"total_alloc_bytes": memStats.TotalAlloc,
				"sys_bytes":         memStats.Sys,
				"heap_alloc_bytes":  memStats.HeapAlloc,
				"num_gc":            memStats.NumGC,
			},
			"receivers": map[string]any{
				"configured": s.getReceiversStatus(),
				"recent_packets_1h": map[string]int{
					"syslog":  syslogCount,
					"trap":    trapCount,
					"netflow": flowCount,
				},
			},
			"managed": map[string]any{
				"node_count":    nodeCount,
				"polling_count": pollCount,
			},
			"timestamp": time.Now().Format(time.RFC3339),
		}
		return jsonResult(status)
	})

	// 13. diagnose_node
	mcp.AddTool(server, &mcp.Tool{
		Name:        "diagnose_node",
		Description: "Run automated ping, SNMP, and web connectivity probes on demand for a node",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		NodeID string `json:"node_id" jsonschema:"Node ID to run automated diagnostics on"`
	}) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.NodeID == "" {
			return nil, nil, fmt.Errorf("store or node_id missing")
		}
		node, err := s.store.GetNode(ctx, args.NodeID)
		if err != nil || node == nil {
			return nil, nil, fmt.Errorf("node not found: %s", args.NodeID)
		}
		diagResult := diagnose.DiagnoseNode(ctx, node)
		return jsonResult(diagResult)
	})

	// 14. get_node
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_node",
		Description: "Get detailed attributes and attached pollings of a specific node",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetNodeParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		node, err := s.findNode(ctx, args.ID)
		if err != nil {
			return nil, nil, err
		}
		polls := make([]mcpPollingEnt, 0)
		if allPolls, err := s.store.ListPollings(ctx); err == nil {
			for _, p := range allPolls {
				if p.NodeID == node.ID {
					polls = append(polls, toMCPPollingEnt(p, node.Name))
				}
			}
		}
		return jsonResult(mcpNodeDetailEnt{
			ID:           node.ID,
			Name:         node.Name,
			IP:           node.IP,
			MAC:          node.MAC,
			Vendor:       node.Vendor,
			State:        node.State,
			X:            node.X,
			Y:            node.Y,
			Icon:         node.Icon,
			Description:  node.Descr,
			SnmpMode:     node.SnmpMode,
			URL:          node.URL,
			AddrMode:     node.AddrMode,
			AutoAck:      node.AutoAck,
			PollingCount: len(polls),
			Pollings:     polls,
		})
	})

	// 15. delete_node
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_node",
		Description: "Delete a node and its attached pollings and connected topology lines",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpDeleteNodeParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		node, err := s.resolveUniqueNode(ctx, args.ID)
		if err != nil {
			return nil, nil, err
		}
		if err := s.store.DeleteNode(ctx, node.ID); err != nil {
			return nil, nil, fmt.Errorf("delete node: %w", err)
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "user",
			Level:    "warn",
			NodeID:   node.ID,
			NodeName: node.Name,
			Event:    fmt.Sprintf("Deleted node %s (%s) via MCP", node.Name, node.IP),
		})
		return jsonResult(map[string]string{
			"id":     node.ID,
			"name":   node.Name,
			"ip":     node.IP,
			"status": "deleted",
		})
	})

	// 16. get_line_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_line_list",
		Description: "Retrieve topology connection lines between nodes and networks",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetLineListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpLineEnt{})
		}
		lines, err := s.store.ListLines(ctx)
		if err != nil {
			return nil, nil, err
		}

		nodeMap := make(map[string]string)
		if nodes, err := s.store.ListNodes(ctx); err == nil {
			for _, n := range nodes {
				nodeMap[n.ID] = n.Name
			}
		}
		netMap := make(map[string]string)
		if nets, err := s.store.ListNetworks(ctx); err == nil {
			for _, nw := range nets {
				netMap[nw.ID] = nw.Name
			}
		}
		pollMap := make(map[string]string)
		if polls, err := s.store.ListPollings(ctx); err == nil {
			for _, p := range polls {
				pollMap[p.ID] = p.Name
			}
		}

		nodeFilter := makeRegexFilter(args.NodeFilter)
		stateFilter := makeRegexFilter(args.StateFilter)

		list := make([]mcpLineEnt, 0, len(lines))
		for _, l := range lines {
			ent := s.buildLineEntity(ctx, l, nodeMap, netMap, pollMap)
			if nodeFilter != nil &&
				!nodeFilter.MatchString(ent.NodeID1) && !nodeFilter.MatchString(ent.NodeName1) &&
				!nodeFilter.MatchString(ent.NodeID2) && !nodeFilter.MatchString(ent.NodeName2) {
				continue
			}
			if stateFilter != nil &&
				!stateFilter.MatchString(ent.State) &&
				!stateFilter.MatchString(ent.State1) &&
				!stateFilter.MatchString(ent.State2) {
				continue
			}
			list = append(list, ent)
		}
		return jsonResult(list)
	})

	// 17. add_line
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_line",
		Description: "Create a topology connection line between two nodes or network containers",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpAddLineParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return nil, nil, fmt.Errorf("datastore not available")
		}
		if args.NodeID1 == "" || args.NodeID2 == "" {
			return nil, nil, fmt.Errorf("node_id1 and node_id2 are required")
		}

		resolveEndpoint := func(rawID string) (string, string, error) {
			if strings.HasPrefix(rawID, "NET:") {
				netID := strings.TrimPrefix(rawID, "NET:")
				nw, err := s.store.GetNetwork(ctx, netID)
				if err != nil || nw == nil {
					return "", "", fmt.Errorf("network not found: %s", rawID)
				}
				return rawID, nw.Name, nil
			}
			n, err := s.resolveUniqueNode(ctx, rawID)
			if err != nil {
				return "", "", err
			}
			return n.ID, n.Name, nil
		}

		id1, name1, err := resolveEndpoint(args.NodeID1)
		if err != nil {
			return nil, nil, err
		}
		id2, name2, err := resolveEndpoint(args.NodeID2)
		if err != nil {
			return nil, nil, err
		}

		pollMap := make(map[string]string)
		validatePollingRef := func(pid string) error {
			if pid == "" {
				return nil
			}
			p, err := s.store.GetPolling(ctx, pid)
			if err != nil || p == nil {
				return fmt.Errorf("polling not found: %s", pid)
			}
			pollMap[p.ID] = p.Name
			return nil
		}
		if !strings.HasPrefix(id1, "NET:") {
			if err := validatePollingRef(args.PollingID1); err != nil {
				return nil, nil, err
			}
		}
		if !strings.HasPrefix(id2, "NET:") {
			if err := validatePollingRef(args.PollingID2); err != nil {
				return nil, nil, err
			}
		}
		if err := validatePollingRef(args.PollingID); err != nil {
			return nil, nil, err
		}

		width := args.Width
		if width <= 0 {
			width = 2
		}

		line := &datastore.LineEnt{
			ID:         datastore.GenerateID(),
			NodeID1:    id1,
			PollingID1: args.PollingID1,
			NodeID2:    id2,
			PollingID2: args.PollingID2,
			PollingID:  args.PollingID,
			Width:      width,
			Port:       args.Port,
			Info:       args.Info,
		}
		if err := s.store.SaveLine(ctx, line); err != nil {
			return nil, nil, fmt.Errorf("save line: %w", err)
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "user",
			Level: "info",
			Event: fmt.Sprintf("Added line %s - %s via MCP", id1, id2),
		})

		if saved, err := s.store.GetLine(ctx, line.ID); err == nil && saved != nil {
			line = saved
		}
		nodeMap := map[string]string{id1: name1, id2: name2}
		netMap := make(map[string]string)
		if strings.HasPrefix(id1, "NET:") {
			netMap[strings.TrimPrefix(id1, "NET:")] = name1
		}
		if strings.HasPrefix(id2, "NET:") {
			netMap[strings.TrimPrefix(id2, "NET:")] = name2
		}
		return jsonResult(s.buildLineEntity(ctx, line, nodeMap, netMap, pollMap))
	})

	// 18. delete_line
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_line",
		Description: "Delete a topology connection line by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpDeleteLineParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		line, err := s.store.GetLine(ctx, args.ID)
		if err != nil || line == nil {
			return nil, nil, fmt.Errorf("line not found: %s", args.ID)
		}
		if err := s.store.DeleteLine(ctx, line.ID); err != nil {
			return nil, nil, fmt.Errorf("delete line: %w", err)
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "user",
			Level: "info",
			Event: fmt.Sprintf("Deleted line %s (%s - %s) via MCP", line.ID, line.NodeID1, line.NodeID2),
		})
		return jsonResult(map[string]string{
			"id":       line.ID,
			"node_id1": line.NodeID1,
			"node_id2": line.NodeID2,
			"status":   "deleted",
		})
	})

	// 19. get_polling
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_polling",
		Description: "Get full configuration and latest status for a specific polling by ID",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetPollingParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		p, err := s.store.GetPolling(ctx, args.ID)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("polling not found: %s", args.ID)
		}
		nodeName := ""
		if n, err := s.store.GetNode(ctx, p.NodeID); err == nil && n != nil {
			nodeName = n.Name
		}
		return jsonResult(toMCPPollingDetailEnt(p, nodeName))
	})

	// 20. add_polling
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_polling",
		Description: "Add a new monitoring polling task to an existing node",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpAddPollingParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return nil, nil, fmt.Errorf("datastore not available")
		}
		if args.NodeID == "" || args.Name == "" || args.Type == "" {
			return nil, nil, fmt.Errorf("node_id, name, and type are required")
		}
		node, err := s.resolveUniqueNode(ctx, args.NodeID)
		if err != nil {
			return nil, nil, err
		}

		level := strings.ToLower(strings.TrimSpace(args.Level))
		if level == "" {
			level = "low"
		}
		if err := validatePollingLevel(level); err != nil {
			return nil, nil, err
		}

		logMode, err := parsePollingLogMode(args.LogMode)
		if err != nil {
			return nil, nil, err
		}

		pollInt := args.PollInt
		if pollInt <= 0 {
			pollInt = 60
		}
		timeout := args.Timeout
		if timeout <= 0 {
			timeout = 1
		}
		retry := args.Retry
		if retry <= 0 {
			retry = 1
		}

		p := &datastore.PollingEnt{
			ID:           datastore.GenerateID(),
			Name:         args.Name,
			NodeID:       node.ID,
			Type:         strings.ToLower(strings.TrimSpace(args.Type)),
			Mode:         args.Mode,
			Params:       args.Params,
			Filter:       args.Filter,
			Extractor:    args.Extractor,
			Script:       args.Script,
			Level:        level,
			PollInt:      pollInt,
			Timeout:      timeout,
			Retry:        retry,
			LogMode:      logMode,
			State:        "unknown",
			NextTime:     time.Now().UnixNano(),
			FailAction:   args.FailAction,
			RepairAction: args.RepairAction,
			AIMode:       args.AIMode,
		}
		if err := s.store.SavePolling(ctx, p); err != nil {
			return nil, nil, fmt.Errorf("save polling: %w", err)
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "user",
			Level:    "info",
			NodeID:   node.ID,
			NodeName: node.Name,
			Event:    fmt.Sprintf("Added polling %s (%s) via MCP", p.Name, p.Type),
		})
		return jsonResult(toMCPPollingDetailEnt(p, node.Name))
	})

	// 21. update_polling
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_polling",
		Description: "Modify an existing polling configuration and schedule an immediate recheck",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpUpdatePollingParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		p, err := s.store.GetPolling(ctx, args.ID)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("polling not found: %s", args.ID)
		}

		if args.Name != "" {
			p.Name = args.Name
		}
		if args.Type != "" {
			p.Type = strings.ToLower(strings.TrimSpace(args.Type))
		}
		if args.Mode != "" {
			p.Mode = args.Mode
		}
		if args.Params != "" {
			p.Params = args.Params
		}
		if args.Filter != "" {
			p.Filter = args.Filter
		}
		if args.Extractor != "" {
			p.Extractor = args.Extractor
		}
		if args.Script != "" {
			p.Script = args.Script
		}
		if args.Level != "" {
			lvl := strings.ToLower(strings.TrimSpace(args.Level))
			if err := validatePollingLevel(lvl); err != nil {
				return nil, nil, err
			}
			p.Level = lvl
		}
		if args.PollInt > 0 {
			p.PollInt = args.PollInt
		}
		if args.Timeout > 0 {
			p.Timeout = args.Timeout
		}
		if args.Retry > 0 {
			p.Retry = args.Retry
		}
		if args.LogMode != "" {
			lm, err := parsePollingLogMode(args.LogMode)
			if err != nil {
				return nil, nil, err
			}
			p.LogMode = lm
		}
		if args.FailAction != "" {
			p.FailAction = args.FailAction
		}
		if args.RepairAction != "" {
			p.RepairAction = args.RepairAction
		}
		if args.AIMode != "" {
			p.AIMode = args.AIMode
		}
		if args.ResetState {
			p.State = "unknown"
			p.FailTime = 0
		}
		p.NextTime = time.Now().UnixNano()

		if err := s.store.SavePolling(ctx, p); err != nil {
			return nil, nil, fmt.Errorf("save polling: %w", err)
		}
		nodeName := ""
		if n, err := s.store.GetNode(ctx, p.NodeID); err == nil && n != nil {
			nodeName = n.Name
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "user",
			Level:    "info",
			NodeID:   p.NodeID,
			NodeName: nodeName,
			Event:    fmt.Sprintf("Updated polling %s via MCP", p.Name),
		})
		return jsonResult(toMCPPollingDetailEnt(p, nodeName))
	})

	// 22. delete_polling
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_polling",
		Description: "Delete a polling task and clean up any topology lines bound to it",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpDeletePollingParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil || args.ID == "" {
			return nil, nil, fmt.Errorf("store or id missing")
		}
		p, err := s.store.GetPolling(ctx, args.ID)
		if err != nil || p == nil {
			return nil, nil, fmt.Errorf("polling not found: %s", args.ID)
		}
		if err := s.store.DeletePolling(ctx, p.ID); err != nil {
			return nil, nil, fmt.Errorf("delete polling: %w", err)
		}
		nodeName := ""
		if n, err := s.store.GetNode(ctx, p.NodeID); err == nil && n != nil {
			nodeName = n.Name
		}
		_ = s.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "user",
			Level:    "warn",
			NodeID:   p.NodeID,
			NodeName: nodeName,
			Event:    fmt.Sprintf("Deleted polling %s via MCP", p.Name),
		})
		return jsonResult(map[string]string{
			"id":      p.ID,
			"name":    p.Name,
			"node_id": p.NodeID,
			"status":  "deleted",
		})
	})

	// 23. check_polling
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_polling",
		Description: "Trigger immediate execution of a specific polling, non-normal pollings on a node, or all pollings",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpCheckPollingParams) (*mcp.CallToolResult, any, error) {
		s.mu.RLock()
		execFn := s.executePolling
		checkNodeFn := s.checkNode
		checkAllFn := s.checkAllPollings
		s.mu.RUnlock()

		if args.PollingID != "" {
			if s.store == nil {
				return nil, nil, fmt.Errorf("datastore not available")
			}
			if execFn == nil {
				return nil, nil, fmt.Errorf("polling execution engine is not available")
			}
			p, err := s.store.GetPolling(ctx, args.PollingID)
			if err != nil || p == nil {
				return nil, nil, fmt.Errorf("polling not found: %s", args.PollingID)
			}
			state, msg, result, err := execFn(ctx, p)
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(map[string]any{
				"id":      p.ID,
				"name":    p.Name,
				"node_id": p.NodeID,
				"state":   state,
				"message": msg,
				"result":  result,
			})
		}

		if args.NodeID != "" {
			if s.store == nil {
				return nil, nil, fmt.Errorf("datastore not available")
			}
			if checkNodeFn == nil {
				return nil, nil, fmt.Errorf("polling check engine is not available")
			}
			node, err := s.findNode(ctx, args.NodeID)
			if err != nil {
				return nil, nil, err
			}
			count, err := checkNodeFn(ctx, node.ID)
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(map[string]any{
				"node_id":      node.ID,
				"node_name":    node.Name,
				"queued_count": count,
				"status":       "queued",
			})
		}

		if args.All {
			if checkAllFn == nil {
				return nil, nil, fmt.Errorf("polling check engine is not available")
			}
			count := checkAllFn(ctx)
			return jsonResult(map[string]any{
				"queued_count": count,
				"status":       "queued",
			})
		}

		return nil, nil, fmt.Errorf("one of polling_id, node_id, or all=true is required")
	})

	// 24. get_ai_anomaly_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ai_anomaly_list",
		Description: "Retrieve AI anomaly detection scores for AI-enabled pollings",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetAIAnomalyListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpAIAnomalyEnt{})
		}
		limit := args.Limit
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		nodeFilter := makeRegexFilter(args.NodeFilter)
		pollingFilter := makeRegexFilter(args.PollingFilter)

		nodeMap := make(map[string]string)
		if nodes, err := s.store.ListNodes(ctx); err == nil {
			for _, n := range nodes {
				nodeMap[n.ID] = n.Name
			}
		}

		list := make([]mcpAIAnomalyEnt, 0)
		s.store.ForEachPollings(func(pe *datastore.PollingEnt) bool {
			if pe.LogMode != datastore.LogModeAI {
				return true
			}
			air, err := s.store.GetAIResult(pe.ID)
			if err != nil || air == nil || len(air.ScoreData) == 0 {
				return true
			}
			score := air.ScoreData[len(air.ScoreData)-1][1]
			if score < args.MinScore {
				return true
			}
			nodeName := nodeMap[pe.NodeID]
			if nodeFilter != nil && !nodeFilter.MatchString(nodeName) {
				return true
			}
			if pollingFilter != nil && !pollingFilter.MatchString(pe.Name) {
				return true
			}
			list = append(list, mcpAIAnomalyEnt{
				PollingID: pe.ID,
				Polling:   pe.Name,
				NodeID:    pe.NodeID,
				Node:      nodeName,
				Score:     score,
				Count:     len(air.ScoreData),
				LastTime:  formatUnixTimestamp(air.LastTime, time.RFC3339Nano),
			})
			return true
		})

		sort.Slice(list, func(i, j int) bool {
			return list[i].Score > list[j].Score
		})
		if len(list) > limit {
			list = list[:limit]
		}
		return jsonResult(list)
	})
}
