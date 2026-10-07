package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

type mcpEventLogEnt struct {
	Time  string `json:"time"`
	Type  string `json:"type"`
	Level string `json:"level"`
	Node  string `json:"node"`
	Event string `json:"event"`
}

type mcpSearchEventLogParams struct {
	NodeFilter  string `json:"node_filter,omitempty" jsonschema:"regex filter for node name"`
	TypeFilter  string `json:"type_filter,omitempty" jsonschema:"regex filter for event type (system, user, polling, trap, syslog, etc.)"`
	LevelFilter string `json:"level_filter,omitempty" jsonschema:"regex filter for event level (info, normal, warn, low, high, error)"`
	EventFilter string `json:"event_filter,omitempty" jsonschema:"regex filter for event message text"`
	StartTime   string `json:"start_time,omitempty" jsonschema:"start date/time or duration from now (default -1h)"`
	EndTime     string `json:"end_time,omitempty" jsonschema:"end date/time or duration (default now)"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum number of log records to return (100 - 10000, default 500)"`
}

type mcpAddEventLogParams struct {
	Level  string `json:"level" jsonschema:"event level (info, warn, low, high, error)"`
	Type   string `json:"type,omitempty" jsonschema:"event type (default 'user')"`
	NodeID string `json:"node_id,omitempty" jsonschema:"target node ID or node name"`
	Event  string `json:"event" jsonschema:"event message text"`
}

type mcpSyslogEnt struct {
	Time     string `json:"time"`
	Level    string `json:"level"`
	Host     string `json:"host"`
	Type     string `json:"type"`
	Tag      string `json:"tag"`
	Message  string `json:"message"`
	Severity int    `json:"severity"`
	Facility int    `json:"facility"`
}

type mcpSearchSyslogParams struct {
	HostFilter    string `json:"host_filter,omitempty" jsonschema:"regex filter for syslog host/IP"`
	TagFilter     string `json:"tag_filter,omitempty" jsonschema:"regex filter for syslog tag/program"`
	LevelFilter   string `json:"level_filter,omitempty" jsonschema:"regex filter for syslog severity/level"`
	MessageFilter string `json:"message_filter,omitempty" jsonschema:"regex filter for syslog message content"`
	StartTime     string `json:"start_time,omitempty" jsonschema:"start date/time or duration (default -1h)"`
	EndTime       string `json:"end_time,omitempty" jsonschema:"end date/time or duration (default now)"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of records (100 - 10000, default 500)"`
}

type mcpSyslogSummaryItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mcpGetSyslogSummaryParams struct {
	SummaryType   string `json:"summary_type" jsonschema:"summary type (host, tag, severity)"`
	HostFilter    string `json:"host_filter,omitempty" jsonschema:"regex filter for host"`
	TagFilter     string `json:"tag_filter,omitempty" jsonschema:"regex filter for tag"`
	LevelFilter   string `json:"level_filter,omitempty" jsonschema:"regex filter for level/severity"`
	MessageFilter string `json:"message_filter,omitempty" jsonschema:"regex filter for message"`
	StartTime     string `json:"start_time,omitempty" jsonschema:"start date/time or duration (default -1h)"`
	EndTime       string `json:"end_time,omitempty" jsonschema:"end date/time or duration (default now)"`
}

type mcpSnmpTrapLogEnt struct {
	Time      string `json:"time"`
	From      string `json:"from"`
	TrapType  string `json:"trap_type"`
	Variables string `json:"variables"`
}

type mcpSearchSnmpTrapLogParams struct {
	FromFilter    string `json:"from_filter,omitempty" jsonschema:"regex filter for trap sender IP/host"`
	TypeFilter    string `json:"type_filter,omitempty" jsonschema:"regex filter for trap type"`
	MessageFilter string `json:"message_filter,omitempty" jsonschema:"regex filter for trap variables/message"`
	StartTime     string `json:"start_time,omitempty" jsonschema:"start date/time or duration (default -1h)"`
	EndTime       string `json:"end_time,omitempty" jsonschema:"end date/time or duration (default now)"`
	Limit         int    `json:"limit,omitempty" jsonschema:"maximum number of records (100 - 10000, default 500)"`
}

type mcpIPInfoResult struct {
	IP       string `json:"ip"`
	NodeName string `json:"node_name,omitempty"`
	NodeID   string `json:"node_id,omitempty"`
	DNSHost  string `json:"dns_host,omitempty"`
	Location string `json:"location,omitempty"`
	MAC      string `json:"mac,omitempty"`
}

type mcpMACInfoResult struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip,omitempty"`
	NodeName string `json:"node_name,omitempty"`
	NodeID   string `json:"node_id,omitempty"`
	Vendor   string `json:"vendor,omitempty"`
}

func (s *MCPServer) registerLogTools(server *mcp.Server) {
	// 1. search_event_log
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_event_log",
		Description: "Search system and monitoring event logs from TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSearchEventLogParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpEventLogEnt{})
		}
		st, et, err := getTimeRange(args.StartTime, args.EndTime)
		if err != nil {
			return nil, nil, err
		}
		limit := args.Limit
		if limit < 10 {
			limit = 500
		}
		if limit > 10000 {
			limit = 10000
		}

		nodeFilter := makeRegexFilter(args.NodeFilter)
		typeFilter := makeRegexFilter(args.TypeFilter)
		levelFilter := makeRegexFilter(args.LevelFilter)
		eventFilter := makeRegexFilter(args.EventFilter)

		logs, err := s.store.ListEventLogs(ctx, limit*2)
		if err != nil {
			return nil, nil, err
		}

		list := make([]mcpEventLogEnt, 0)
		for _, l := range logs {
			if l.Time < st || (et > 0 && l.Time > et) {
				continue
			}
			if nodeFilter != nil && !nodeFilter.MatchString(l.NodeName) {
				continue
			}
			if typeFilter != nil && !typeFilter.MatchString(l.Type) {
				continue
			}
			if levelFilter != nil && !levelFilter.MatchString(l.Level) {
				continue
			}
			if eventFilter != nil && !eventFilter.MatchString(l.Event) {
				continue
			}
			list = append(list, mcpEventLogEnt{
				Time:  time.Unix(0, l.Time).Format(time.RFC3339Nano),
				Type:  l.Type,
				Level: l.Level,
				Node:  l.NodeName,
				Event: l.Event,
			})
			if len(list) >= limit {
				break
			}
		}
		return jsonResult(list)
	})

	// 2. add_event_log
	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_event_log",
		Description: "Record a new custom event log entry in TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpAddEventLogParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return nil, nil, fmt.Errorf("store unavailable")
		}
		if args.Event == "" {
			return nil, nil, fmt.Errorf("event message is required")
		}
		level := args.Level
		if level == "" {
			level = "info"
		}
		logType := args.Type
		if logType == "" {
			logType = "user"
		}

		nodeName := ""
		nodeID := args.NodeID
		if nodeID != "" {
			if n, err := s.findNode(ctx, nodeID); err == nil && n != nil {
				nodeID = n.ID
				nodeName = n.Name
			}
		}

		ent := &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     logType,
			Level:    level,
			NodeID:   nodeID,
			NodeName: nodeName,
			Event:    args.Event,
		}
		if err := s.store.AddEventLog(ctx, ent); err != nil {
			return nil, nil, err
		}
		return jsonResult(mcpEventLogEnt{
			Time:  time.Unix(0, ent.Time).Format(time.RFC3339Nano),
			Type:  ent.Type,
			Level: ent.Level,
			Node:  ent.NodeName,
			Event: ent.Event,
		})
	})

	// 3. search_syslog
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_syslog",
		Description: "Search syslog messages from the Parquet data lake",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSearchSyslogParams) (*mcp.CallToolResult, any, error) {
		if s.logStore == nil {
			return jsonResult([]mcpSyslogEnt{})
		}
		st, et, err := getTimeRange(args.StartTime, args.EndTime)
		if err != nil {
			return nil, nil, err
		}
		limit := args.Limit
		if limit < 10 {
			limit = 500
		}
		if limit > 10000 {
			limit = 10000
		}

		recs, err := s.logStore.Query(ctx, parquet.LogFilter{
			Type:      "syslog",
			StartTime: st,
			EndTime:   et,
			Filter:    args.MessageFilter,
			Limit:     limit * 2,
		})
		if err != nil {
			return nil, nil, err
		}

		hostFilter := makeRegexFilter(args.HostFilter)
		tagFilter := makeRegexFilter(args.TagFilter)
		levelFilter := makeRegexFilter(args.LevelFilter)

		list := make([]mcpSyslogEnt, 0)
		for _, r := range recs {
			host := r.Src
			tag := ""
			level := ""
			msg := r.Log
			severity := 0
			facility := 0

			var parsed map[string]any
			if err := json.Unmarshal([]byte(r.Log), &parsed); err == nil {
				if h, ok := parsed["host"].(string); ok && h != "" {
					host = h
				}
				if t, ok := parsed["tag"].(string); ok {
					tag = t
				}
				if l, ok := parsed["level"].(string); ok {
					level = l
				}
				if c, ok := parsed["content"].(string); ok {
					msg = c
				} else if m, ok := parsed["message"].(string); ok {
					msg = m
				}
				if s, ok := parsed["severity"].(float64); ok {
					severity = int(s)
				}
				if f, ok := parsed["facility"].(float64); ok {
					facility = int(f)
				}
			}

			if hostFilter != nil && !hostFilter.MatchString(host) {
				continue
			}
			if tagFilter != nil && !tagFilter.MatchString(tag) {
				continue
			}
			if levelFilter != nil && !levelFilter.MatchString(level) {
				continue
			}
			list = append(list, mcpSyslogEnt{
				Time:     time.Unix(0, r.Time).Format(time.RFC3339Nano),
				Level:    level,
				Host:     host,
				Type:     r.Type,
				Tag:      tag,
				Message:  msg,
				Severity: severity,
				Facility: facility,
			})
			if len(list) >= limit {
				break
			}
		}
		return jsonResult(list)
	})

	// 4. get_syslog_summary
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_syslog_summary",
		Description: "Get syslog aggregation summary grouped by host, tag, or severity",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetSyslogSummaryParams) (*mcp.CallToolResult, any, error) {
		if s.logStore == nil {
			return jsonResult([]mcpSyslogSummaryItem{})
		}
		st, et, err := getTimeRange(args.StartTime, args.EndTime)
		if err != nil {
			return nil, nil, err
		}

		recs, err := s.logStore.Query(ctx, parquet.LogFilter{
			Type:      "syslog",
			StartTime: st,
			EndTime:   et,
			Filter:    args.MessageFilter,
			Limit:     50000,
		})
		if err != nil {
			return nil, nil, err
		}

		hostFilter := makeRegexFilter(args.HostFilter)
		tagFilter := makeRegexFilter(args.TagFilter)
		levelFilter := makeRegexFilter(args.LevelFilter)

		counts := make(map[string]int)
		for _, r := range recs {
			host := r.Src
			tag := ""
			level := ""

			var parsed map[string]any
			if err := json.Unmarshal([]byte(r.Log), &parsed); err == nil {
				if h, ok := parsed["host"].(string); ok && h != "" {
					host = h
				}
				if t, ok := parsed["tag"].(string); ok {
					tag = t
				}
				if l, ok := parsed["level"].(string); ok {
					level = l
				}
			}

			if hostFilter != nil && !hostFilter.MatchString(host) {
				continue
			}
			if tagFilter != nil && !tagFilter.MatchString(tag) {
				continue
			}
			if levelFilter != nil && !levelFilter.MatchString(level) {
				continue
			}

			key := ""
			switch strings.ToLower(args.SummaryType) {
			case "tag":
				key = tag
				if key == "" {
					key = "unknown"
				}
			case "severity", "level":
				key = level
				if key == "" {
					key = "info"
				}
			default: // host
				key = host
				if key == "" {
					key = "unknown"
				}
			}
			counts[key]++
		}

		items := make([]mcpSyslogSummaryItem, 0, len(counts))
		for k, v := range counts {
			items = append(items, mcpSyslogSummaryItem{Name: k, Count: v})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].Count > items[j].Count
		})

		return jsonResult(items)
	})

	// 5. search_snmp_trap_log
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_snmp_trap_log",
		Description: "Search SNMP Trap log messages from the Parquet data lake",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSearchSnmpTrapLogParams) (*mcp.CallToolResult, any, error) {
		if s.logStore == nil {
			return jsonResult([]mcpSnmpTrapLogEnt{})
		}
		st, et, err := getTimeRange(args.StartTime, args.EndTime)
		if err != nil {
			return nil, nil, err
		}
		limit := args.Limit
		if limit < 10 {
			limit = 500
		}
		if limit > 10000 {
			limit = 10000
		}

		recs, err := s.logStore.Query(ctx, parquet.LogFilter{
			Type:      "trap",
			StartTime: st,
			EndTime:   et,
			Filter:    args.MessageFilter,
			Limit:     limit * 2,
		})
		if err != nil {
			return nil, nil, err
		}

		fromFilter := makeRegexFilter(args.FromFilter)
		typeFilter := makeRegexFilter(args.TypeFilter)

		list := make([]mcpSnmpTrapLogEnt, 0)
		for _, r := range recs {
			from := r.Src
			trapType := ""
			vars := r.Log

			var parsed map[string]any
			if err := json.Unmarshal([]byte(r.Log), &parsed); err == nil {
				if f, ok := parsed["from"].(string); ok && f != "" {
					from = f
				} else if h, ok := parsed["host"].(string); ok && h != "" {
					from = h
				}
				if t, ok := parsed["type"].(string); ok {
					trapType = t
				} else if t, ok := parsed["tag"].(string); ok {
					trapType = t
				}
				if v, ok := parsed["variables"].(string); ok {
					vars = v
				} else if c, ok := parsed["content"].(string); ok {
					vars = c
				}
			}

			if fromFilter != nil && !fromFilter.MatchString(from) {
				continue
			}
			if typeFilter != nil && !typeFilter.MatchString(trapType) {
				continue
			}
			list = append(list, mcpSnmpTrapLogEnt{
				Time:      time.Unix(0, r.Time).Format(time.RFC3339Nano),
				From:      from,
				TrapType:  trapType,
				Variables: vars,
			})
			if len(list) >= limit {
				break
			}
		}
		return jsonResult(list)
	})

	// 6. get_ip_address_info
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ip_address_info",
		Description: "Retrieve detailed IP address information (DNS host, managed node, GeoIP location)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		IP string `json:"ip" jsonschema:"IP address to look up"`
	}) (*mcp.CallToolResult, any, error) {
		if args.IP == "" {
			return nil, nil, fmt.Errorf("ip is required")
		}

		res := mcpIPInfoResult{
			IP:       args.IP,
			Location: datastore.GetLoc(args.IP),
		}

		// Reverse DNS lookup
		if names, err := net.LookupAddr(args.IP); err == nil && len(names) > 0 {
			res.DNSHost = strings.TrimSuffix(names[0], ".")
		}

		// Managed Node search
		if s.store != nil {
			if nodes, err := s.store.ListNodes(ctx); err == nil {
				for _, n := range nodes {
					if n.IP == args.IP {
						res.NodeName = n.Name
						res.NodeID = n.ID
						res.MAC = n.MAC
						break
					}
				}
			}
			if res.MAC == "" {
				if arps, err := s.store.LoadArpTable(ctx); err == nil {
					for _, a := range arps {
						if a.IP == args.IP {
							res.MAC = a.MAC
							break
						}
					}
				}
			}
		}

		return jsonResult(res)
	})

	// 7. get_mac_address_info
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mac_address_info",
		Description: "Retrieve detailed MAC address information (IP, managed node, vendor)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct {
		MAC string `json:"mac" jsonschema:"MAC address to look up"`
	}) (*mcp.CallToolResult, any, error) {
		if args.MAC == "" {
			return nil, nil, fmt.Errorf("mac is required")
		}

		normMAC := strings.ToLower(args.MAC)
		res := mcpMACInfoResult{
			MAC: args.MAC,
		}

		if s.store != nil {
			if arps, err := s.store.LoadArpTable(ctx); err == nil {
				for _, a := range arps {
					if strings.ToLower(a.MAC) == normMAC {
						res.IP = a.IP
						break
					}
				}
			}
			if nodes, err := s.store.ListNodes(ctx); err == nil {
				for _, n := range nodes {
					if strings.ToLower(n.MAC) == normMAC || (res.IP != "" && n.IP == res.IP) {
						res.NodeName = n.Name
						res.NodeID = n.ID
						res.Vendor = n.Vendor
						if res.IP == "" {
							res.IP = n.IP
						}
						break
					}
				}
			}
		}

		return jsonResult(res)
	})
}
