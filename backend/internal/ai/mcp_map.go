package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"runtime"
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
	Name        string `json:"name" jsonschema:"node name. A PING polling is also added automatically."`
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
			list = append(list, mcpNodeEnt{
				ID:          n.ID,
				Name:        n.Name,
				IP:          n.IP,
				MAC:         n.MAC,
				State:       n.State,
				X:           n.X,
				Y:           n.Y,
				Icon:        n.Icon,
				Description: n.Descr,
			})
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

			lastTimeStr := ""
			if p.LastTime > 0 {
				lastTimeStr = time.Unix(0, p.LastTime).Format(time.RFC3339Nano)
			}

			list = append(list, mcpPollingEnt{
				ID:       p.ID,
				Name:     p.Name,
				NodeName: nodeName,
				NodeID:   p.NodeID,
				Type:     p.Type,
				Level:    p.Level,
				State:    p.State,
				LastTime: lastTimeStr,
				Result:   p.Result,
			})
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
		target := args.Target
		if target == "" && args.NodeID != "" {
			target = args.NodeID
		}
		if target == "" {
			return nil, nil, fmt.Errorf("target or node_id is required")
		}
		targetIP, node := s.resolveTargetIP(ctx, target)

		port := uint16(161)
		comm := args.Community
		user := args.User
		pass := args.Password
		mode := args.SnmpMode

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
		if comm == "" && !strings.HasPrefix(strings.ToLower(mode), "v3") {
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
		modeLower := strings.ToLower(mode)
		if modeLower == "v1" {
			agent.Version = gosnmp.Version1
		} else if strings.HasPrefix(modeLower, "v3") {
			agent.Version = gosnmp.Version3
			agent.SecurityModel = gosnmp.UserSecurityModel
			switch modeLower {
			case "v3auth":
				agent.MsgFlags = gosnmp.AuthNoPriv
				agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
					UserName:                 user,
					AuthenticationProtocol:   gosnmp.SHA,
					AuthenticationPassphrase: pass,
				}
			case "v3authpriv":
				agent.MsgFlags = gosnmp.AuthPriv
				agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
					UserName:                 user,
					AuthenticationProtocol:   gosnmp.SHA,
					AuthenticationPassphrase: pass,
					PrivacyProtocol:          gosnmp.AES,
					PrivacyPassphrase:        pass,
				}
			case "v3authprivex":
				agent.MsgFlags = gosnmp.AuthPriv
				agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
					UserName:                 user,
					AuthenticationProtocol:   gosnmp.SHA256,
					AuthenticationPassphrase: pass,
					PrivacyProtocol:          gosnmp.AES256,
					PrivacyPassphrase:        pass,
				}
			}
		}

		if err := agent.Connect(); err != nil {
			return nil, nil, fmt.Errorf("SNMP connect failed: %w", err)
		}
		defer agent.Conn.Close()

		rootOID := args.RootOID
		if rootOID == "" {
			rootOID = ".1.3.6.1.2.1.1"
		} else if !strings.HasPrefix(rootOID, ".") {
			rootOID = mib.NameToOID(rootOID)
		}

		type snmpVar struct {
			Name  string `json:"name"`
			OID   string `json:"oid"`
			Type  string `json:"type"`
			Value string `json:"value"`
		}
		vars := make([]snmpVar, 0)
		_ = agent.Walk(rootOID, func(v gosnmp.SnmpPDU) error {
			name := mib.OIDToName(v.Name)
			valStr := mib.GetMIBValueString(name, &v, false)
			vars = append(vars, snmpVar{
				Name:  name,
				OID:   v.Name,
				Type:  fmt.Sprintf("%v", v.Type),
				Value: valStr,
			})
			return nil
		})

		return jsonResult(vars)
	})

	// 9. snmpset
	mcp.AddTool(server, &mcp.Tool{
		Name:        "snmpset",
		Description: "Set an SNMP MIB variable on a managed node or target IP",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpSnmpSetParams) (*mcp.CallToolResult, any, error) {
		target := args.Target
		if target == "" && args.NodeID != "" {
			target = args.NodeID
		}
		if target == "" {
			return nil, nil, fmt.Errorf("target or node_id is required")
		}
		targetIP, node := s.resolveTargetIP(ctx, target)

		port := uint16(161)
		comm := args.Community
		user := args.User
		pass := args.Password
		mode := args.SnmpMode

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

		agent := &gosnmp.GoSNMP{
			Target:    targetIP,
			Port:      port,
			Community: comm,
			Version:   gosnmp.Version2c,
			Timeout:   2 * time.Second,
			Retries:   1,
		}

		if err := agent.Connect(); err != nil {
			return nil, nil, fmt.Errorf("SNMP connect failed: %w", err)
		}
		defer agent.Conn.Close()

		oid := args.MIBObjectName
		if !strings.HasPrefix(oid, ".") {
			oid = mib.NameToOID(oid)
		}

		var pdu gosnmp.SnmpPDU
		switch strings.ToLower(args.Type) {
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
		default:
			pdu = gosnmp.SnmpPDU{
				Name:  oid,
				Type:  gosnmp.OctetString,
				Value: []byte(args.Value),
			}
		}

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
		Description: "Add a new node to TWSNMP NEO (a PING polling is automatically registered)",
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

		// Automatically add a Ping polling
		pingPoll := &datastore.PollingEnt{
			Name:   "PING",
			Type:   "ping",
			NodeID: node.ID,
			State:  "unknown",
			Params: "64,1,1",
		}
		_ = s.store.SavePolling(ctx, pingPoll)

		return jsonResult(mcpNodeEnt{
			ID:          node.ID,
			Name:        node.Name,
			IP:          node.IP,
			MAC:         node.MAC,
			State:       node.State,
			X:           node.X,
			Y:           node.Y,
			Icon:        node.Icon,
			Description: node.Descr,
		})
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

		return jsonResult(mcpNodeEnt{
			ID:          node.ID,
			Name:        node.Name,
			IP:          node.IP,
			MAC:         node.MAC,
			State:       node.State,
			X:           node.X,
			Y:           node.Y,
			Icon:        node.Icon,
			Description: node.Descr,
		})
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
				"configured": s.receivers,
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
}
