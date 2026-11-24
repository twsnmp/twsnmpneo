package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *MCPServer) registerPrompts(server *mcp.Server) {
	// 1. get_node_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_node_list",
		Title:       "Get node list with filters",
		Description: "Get a list of nodes registered in TWSNMP NEO with regex filters.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "state_filter",
				Title:       "node state filter",
				Description: "node state filter (normal, repair, warn, low, high, unknown)",
				Required:    false,
			},
			{
				Name:        "name_filter",
				Title:       "node name filter",
				Description: "node name filter",
				Required:    false,
			},
			{
				Name:        "ip_filter",
				Title:       "node IP address filter",
				Description: "node IP address filter",
				Required:    false,
			},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		if state, ok := req.Params.Arguments["state_filter"]; ok && state != "" {
			c = append(c, fmt.Sprintf("- State: %s", state))
		}
		if ip, ok := req.Params.Arguments["ip_filter"]; ok && ip != "" {
			c = append(c, fmt.Sprintf("- IP address: %s", ip))
		}
		if name, ok := req.Params.Arguments["name_filter"]; ok && name != "" {
			c = append(c, fmt.Sprintf("- Name: %s", name))
		}
		p := "Get a list of nodes registered in TWSNMP NEO by using get_node_list tool"
		if len(c) > 0 {
			p += " with following filter:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Get node list prompt", p), nil
	})

	// 2. add_node
	server.AddPrompt(&mcp.Prompt{
		Name:        "add_node",
		Title:       "Add a new node",
		Description: "Add a new node to TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "name", Title: "Name", Description: "Node name", Required: true},
			{Name: "ip", Title: "IP address", Description: "IP address", Required: true},
			{Name: "icon", Title: "Icon", Description: "Icon (desktop, laptop, server, cloud, router, etc.)", Required: false},
			{Name: "description", Title: "Description", Description: "Node description", Required: false},
			{Name: "position", Title: "Position", Description: "Map position (e.g. x=100,y=200)", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		name, ok := req.Params.Arguments["name"]
		if !ok || name == "" {
			return nil, fmt.Errorf("name is required")
		}
		c = append(c, fmt.Sprintf("- Name: %s", name))
		ip, ok := req.Params.Arguments["ip"]
		if !ok || ip == "" {
			return nil, fmt.Errorf("ip is required")
		}
		c = append(c, fmt.Sprintf("- IP address: %s", ip))
		if icon, ok := req.Params.Arguments["icon"]; ok && icon != "" {
			c = append(c, fmt.Sprintf("- Icon: %s", icon))
		}
		if desc, ok := req.Params.Arguments["description"]; ok && desc != "" {
			c = append(c, fmt.Sprintf("- Description: %s", desc))
		}
		if pos, ok := req.Params.Arguments["position"]; ok && pos != "" {
			c = append(c, fmt.Sprintf("- Position: %s", pos))
		}
		p := "Add a new node to TWSNMP NEO by using add_node tool with the following information:\n" + strings.Join(c, "\n")
		return promptResult("Add node prompt", p), nil
	})

	// 3. update_node
	server.AddPrompt(&mcp.Prompt{
		Name:        "update_node",
		Title:       "Update node",
		Description: "Update node properties on TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Node ID / Name / IP", Description: "ID of node to update", Required: true},
			{Name: "name", Title: "New name", Description: "New name of node", Required: false},
			{Name: "ip", Title: "New IP", Description: "New IP address", Required: false},
			{Name: "icon", Title: "New icon", Description: "New icon", Required: false},
			{Name: "description", Title: "New description", Description: "New description", Required: false},
			{Name: "position", Title: "New position", Description: "New map position", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id, ok := req.Params.Arguments["id"]
		if !ok || id == "" {
			return nil, fmt.Errorf("id is required")
		}
		c := []string{fmt.Sprintf("- ID: %s", id)}
		if name, ok := req.Params.Arguments["name"]; ok && name != "" {
			c = append(c, fmt.Sprintf("- Name: %s", name))
		}
		if ip, ok := req.Params.Arguments["ip"]; ok && ip != "" {
			c = append(c, fmt.Sprintf("- IP address: %s", ip))
		}
		if icon, ok := req.Params.Arguments["icon"]; ok && icon != "" {
			c = append(c, fmt.Sprintf("- Icon: %s", icon))
		}
		if desc, ok := req.Params.Arguments["description"]; ok && desc != "" {
			c = append(c, fmt.Sprintf("- Description: %s", desc))
		}
		if pos, ok := req.Params.Arguments["position"]; ok && pos != "" {
			c = append(c, fmt.Sprintf("- Position: %s", pos))
		}
		p := "Update the node on TWSNMP NEO by using update_node tool with the following information:\n" + strings.Join(c, "\n")
		return promptResult("Update node prompt", p), nil
	})

	// 4. get_network_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_network_list",
		Title:       "Get network list with filters",
		Description: "Get list of networks registered in TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "name_filter", Title: "Name filter", Description: "Network name filter", Required: false},
			{Name: "ip_filter", Title: "IP filter", Description: "Network IP filter", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		if name, ok := req.Params.Arguments["name_filter"]; ok && name != "" {
			c = append(c, fmt.Sprintf("- Name: %s", name))
		}
		if ip, ok := req.Params.Arguments["ip_filter"]; ok && ip != "" {
			c = append(c, fmt.Sprintf("- IP address: %s", ip))
		}
		p := "Get a list of network nodes registered in TWSNMP NEO by using get_network_list tool"
		if len(c) > 0 {
			p += " with following filter:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Get network list prompt", p), nil
	})

	// 5. get_polling_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_polling_list",
		Title:       "Get polling list with filters",
		Description: "Get list of pollings registered in TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "type_filter", Title: "Type filter", Description: "Polling type (ping, snmp, http, tcp, syslog)", Required: false},
			{Name: "name_filter", Title: "Name filter", Description: "Polling name filter", Required: false},
			{Name: "node_name_filter", Title: "Node name filter", Description: "Target node name filter", Required: false},
			{Name: "state_filter", Title: "State filter", Description: "Polling state filter", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		if t, ok := req.Params.Arguments["type_filter"]; ok && t != "" {
			c = append(c, fmt.Sprintf("- Type: %s", t))
		}
		if name, ok := req.Params.Arguments["name_filter"]; ok && name != "" {
			c = append(c, fmt.Sprintf("- Name: %s", name))
		}
		if node, ok := req.Params.Arguments["node_name_filter"]; ok && node != "" {
			c = append(c, fmt.Sprintf("- Node name: %s", node))
		}
		if state, ok := req.Params.Arguments["state_filter"]; ok && state != "" {
			c = append(c, fmt.Sprintf("- State: %s", state))
		}
		p := "Get a list of polling registered in TWSNMP NEO by using get_polling_list tool"
		if len(c) > 0 {
			p += " with following filter:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Get polling list prompt", p), nil
	})

	// 6. get_polling_log
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_polling_log",
		Title:       "Get polling log",
		Description: "Get polling history logs from TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Polling ID", Description: "Polling ID", Required: true},
			{Name: "limit", Title: "Limit", Description: "Max number of logs to get", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id, ok := req.Params.Arguments["id"]
		if !ok || id == "" {
			return nil, fmt.Errorf("id is required")
		}
		limit := req.Params.Arguments["limit"]
		if limit == "" {
			limit = "100"
		}
		p := fmt.Sprintf("Get polling log from TWSNMP NEO by using get_polling_log tool with the following parameters:\n- ID: %s\n- Limit: %s", id, limit)
		return promptResult("Get polling log prompt", p), nil
	})

	// 7. do_ping
	server.AddPrompt(&mcp.Prompt{
		Name:        "do_ping",
		Title:       "Do ping",
		Description: "Execute ping to target.",
		Arguments: []*mcp.PromptArgument{
			{Name: "target", Title: "Target", Description: "Target IP or hostname or node name", Required: true},
			{Name: "size", Title: "Size", Description: "Ping payload size in bytes", Required: false},
			{Name: "count", Title: "Count", Description: "Number of ping packets", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		target, ok := req.Params.Arguments["target"]
		if !ok || target == "" {
			return nil, fmt.Errorf("target is required")
		}
		var c []string
		c = append(c, fmt.Sprintf("- Target: %s", target))
		if size, ok := req.Params.Arguments["size"]; ok && size != "" {
			c = append(c, fmt.Sprintf("- Size: %s", size))
		}
		if count, ok := req.Params.Arguments["count"]; ok && count != "" {
			c = append(c, fmt.Sprintf("- Count: %s", count))
		}
		p := "Do ping by using do_ping tool with the following parameters:\n" + strings.Join(c, "\n")
		return promptResult("Do ping prompt", p), nil
	})

	// 8. snmpwalk
	server.AddPrompt(&mcp.Prompt{
		Name:        "snmpwalk",
		Title:       "Do SNMP Walk",
		Description: "Perform SNMP walk on target.",
		Arguments: []*mcp.PromptArgument{
			{Name: "target", Title: "Target", Description: "Target IP or node name", Required: true},
			{Name: "oid", Title: "Root OID", Description: "Root OID or MIB name", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		target, ok := req.Params.Arguments["target"]
		if !ok || target == "" {
			return nil, fmt.Errorf("target is required")
		}
		c := []string{fmt.Sprintf("- Target: %s", target)}
		if oid, ok := req.Params.Arguments["oid"]; ok && oid != "" {
			c = append(c, fmt.Sprintf("- OID: %s", oid))
		}
		p := "Perform SNMP walk by using snmpwalk tool with the following parameters:\n" + strings.Join(c, "\n")
		return promptResult("SNMP walk prompt", p), nil
	})

	// 9. search_event_log
	server.AddPrompt(&mcp.Prompt{
		Name:        "search_event_log",
		Title:       "Search event log",
		Description: "Search event logs with filters.",
		Arguments: []*mcp.PromptArgument{
			{Name: "node_filter", Title: "Node filter", Description: "Node name regex filter", Required: false},
			{Name: "level_filter", Title: "Level filter", Description: "Level regex filter", Required: false},
			{Name: "event_filter", Title: "Event filter", Description: "Event text regex filter", Required: false},
			{Name: "start_time", Title: "Start time", Description: "Start time / duration (e.g. -1h)", Required: false},
			{Name: "end_time", Title: "End time", Description: "End time", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		for _, k := range []string{"node_filter", "level_filter", "event_filter", "start_time", "end_time"} {
			if v, ok := req.Params.Arguments[k]; ok && v != "" {
				c = append(c, fmt.Sprintf("- %s: %s", k, v))
			}
		}
		p := "Search event logs by using search_event_log tool"
		if len(c) > 0 {
			p += " with parameters:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Search event log prompt", p), nil
	})

	// 10. add_event_log
	server.AddPrompt(&mcp.Prompt{
		Name:        "add_event_log",
		Title:       "Add event log",
		Description: "Record an event log into TWSNMP NEO.",
		Arguments: []*mcp.PromptArgument{
			{Name: "level", Title: "Level", Description: "Event level (info, warn, high, error)", Required: true},
			{Name: "event", Title: "Event text", Description: "Event description text", Required: true},
			{Name: "node_id", Title: "Node ID / Name", Description: "Associated node", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		event, ok := req.Params.Arguments["event"]
		if !ok || event == "" {
			return nil, fmt.Errorf("event is required")
		}
		level, ok := req.Params.Arguments["level"]
		if !ok || level == "" {
			level = "info"
		}
		c := []string{fmt.Sprintf("- Level: %s", level), fmt.Sprintf("- Event: %s", event)}
		if node, ok := req.Params.Arguments["node_id"]; ok && node != "" {
			c = append(c, fmt.Sprintf("- Node: %s", node))
		}
		p := "Add event log to TWSNMP NEO by using add_event_log tool with parameters:\n" + strings.Join(c, "\n")
		return promptResult("Add event log prompt", p), nil
	})

	// 11. search_syslog
	server.AddPrompt(&mcp.Prompt{
		Name:        "search_syslog",
		Title:       "Search syslog",
		Description: "Search syslog messages with filters.",
		Arguments: []*mcp.PromptArgument{
			{Name: "host_filter", Title: "Host filter", Description: "Host/IP regex filter", Required: false},
			{Name: "tag_filter", Title: "Tag filter", Description: "Tag/Program regex filter", Required: false},
			{Name: "level_filter", Title: "Level filter", Description: "Level regex filter", Required: false},
			{Name: "message_filter", Title: "Message filter", Description: "Message regex filter", Required: false},
			{Name: "start_time", Title: "Start time", Description: "Start time", Required: false},
			{Name: "end_time", Title: "End time", Description: "End time", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		for _, k := range []string{"host_filter", "tag_filter", "level_filter", "message_filter", "start_time", "end_time"} {
			if v, ok := req.Params.Arguments[k]; ok && v != "" {
				c = append(c, fmt.Sprintf("- %s: %s", k, v))
			}
		}
		p := "Search syslog messages by using search_syslog tool"
		if len(c) > 0 {
			p += " with parameters:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Search syslog prompt", p), nil
	})

	// 12. get_syslog_summary
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_syslog_summary",
		Title:       "Get syslog summary",
		Description: "Get syslog aggregation summary grouped by host, tag, or severity.",
		Arguments: []*mcp.PromptArgument{
			{Name: "summary_type", Title: "Summary type", Description: "Summary type (host, tag, severity)", Required: false},
			{Name: "host_filter", Title: "Host filter", Description: "Host regex filter", Required: false},
			{Name: "tag_filter", Title: "Tag filter", Description: "Tag regex filter", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		for _, k := range []string{"summary_type", "host_filter", "tag_filter"} {
			if v, ok := req.Params.Arguments[k]; ok && v != "" {
				c = append(c, fmt.Sprintf("- %s: %s", k, v))
			}
		}
		p := "Get syslog summary by using get_syslog_summary tool"
		if len(c) > 0 {
			p += " with parameters:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Get syslog summary prompt", p), nil
	})

	// 13. search_snmp_trap_log
	server.AddPrompt(&mcp.Prompt{
		Name:        "search_snmp_trap_log",
		Title:       "Search SNMP trap log",
		Description: "Search SNMP trap log records with filters.",
		Arguments: []*mcp.PromptArgument{
			{Name: "from_filter", Title: "From filter", Description: "Trap sender IP filter", Required: false},
			{Name: "type_filter", Title: "Type filter", Description: "Trap type filter", Required: false},
			{Name: "message_filter", Title: "Message filter", Description: "Message filter", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var c []string
		for _, k := range []string{"from_filter", "type_filter", "message_filter"} {
			if v, ok := req.Params.Arguments[k]; ok && v != "" {
				c = append(c, fmt.Sprintf("- %s: %s", k, v))
			}
		}
		p := "Search SNMP trap logs by using search_snmp_trap_log tool"
		if len(c) > 0 {
			p += " with parameters:\n" + strings.Join(c, "\n")
		} else {
			p += "."
		}
		return promptResult("Search SNMP trap log prompt", p), nil
	})

	// 14. get_mib_tree
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_mib_tree",
		Title:       "Get MIB tree",
		Description: "Get MIB tree of TWSNMP NEO by using get_mib_tree tool.",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Get MIB tree prompt", "Get MIB tree of TWSNMP NEO by using get_mib_tree tool."), nil
	})

	// 15. get_ip_address_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_ip_address_list",
		Title:       "Get IP address list",
		Description: "Get the list of IP addresses managed by TWSNMP NEO.",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Get IP address list prompt", "Get the list of IP addresses managed by TWSNMP NEO by using get_ip_address_list tool."), nil
	})

	// 16. get_resource_monitor_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_resource_monitor_list",
		Title:       "Get resource monitor info",
		Description: "Get resource monitor info of TWSNMP NEO by using get_resource_monitor_list tool.",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Get resource monitor list prompt", "Get resource monitor info of TWSNMP NEO by using get_resource_monitor_list tool."), nil
	})

	// 17. get_server_certificate_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_server_certificate_list",
		Title:       "Get server certificate list",
		Description: "Get the list of server certificates managed by TWSNMP NEO.",
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return promptResult("Get server certificate list prompt", "Get the list of server certificates managed by TWSNMP NEO by using get_server_certificate_list tool."), nil
	})

	// 18. get_ip_address_info
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_ip_address_info",
		Title:       "Get IP address info",
		Description: "Get IP address information including DNS host, managed node, and GeoIP.",
		Arguments: []*mcp.PromptArgument{
			{Name: "ip", Title: "IP address", Description: "IP address to look up", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		ip, ok := req.Params.Arguments["ip"]
		if !ok || ip == "" {
			return nil, fmt.Errorf("ip is required")
		}
		p := fmt.Sprintf("Get IP address information by using get_ip_address_info tool. The IP address to look up is %s.", ip)
		return promptResult("Get IP address info prompt", p), nil
	})

	// 19. get_mac_address_info
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_mac_address_info",
		Title:       "Get MAC address info",
		Description: "Get MAC address information including IP, managed node, and vendor.",
		Arguments: []*mcp.PromptArgument{
			{Name: "mac", Title: "MAC address", Description: "MAC address to look up", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		mac, ok := req.Params.Arguments["mac"]
		if !ok || mac == "" {
			return nil, fmt.Errorf("mac is required")
		}
		p := fmt.Sprintf("Get MAC address information by using get_mac_address_info tool. The MAC address to look up is %s.", mac)
		return promptResult("Get MAC address info prompt", p), nil
	})

	// 20. get_node
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_node",
		Title:       "Get node details",
		Description: "Get detailed attributes and attached pollings of a node.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Node ID or Name", Description: "Node ID, node name, or IP address", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := strings.TrimSpace(req.Params.Arguments["id"])
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		return promptResult("Get node details prompt", fmt.Sprintf("Get detailed information for node %q using get_node tool.", id)), nil
	})

	// 21. delete_node
	server.AddPrompt(&mcp.Prompt{
		Name:        "delete_node",
		Title:       "Delete node",
		Description: "Delete a node and its attached pollings and lines.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Node ID or Name", Description: "Node ID or unique node name", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := strings.TrimSpace(req.Params.Arguments["id"])
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		return promptResult("Delete node prompt", fmt.Sprintf("Delete node %q from TWSNMP NEO using delete_node tool.", id)), nil
	})

	// 22. get_line_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_line_list",
		Title:       "Get topology lines",
		Description: "List topology connection lines between nodes and networks.",
		Arguments: []*mcp.PromptArgument{
			{Name: "node_filter", Title: "Node filter", Description: "Regex filter for node ID or name", Required: false},
			{Name: "state_filter", Title: "State filter", Description: "Regex filter for line state", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"node_filter", "Node filter"},
			{"state_filter", "State filter"},
		})
		return promptResult("Get line list prompt", formatToolPrompt("List topology lines using get_line_list tool", c)), nil
	})

	// 23. add_line
	server.AddPrompt(&mcp.Prompt{
		Name:        "add_line",
		Title:       "Add topology line",
		Description: "Connect two nodes on the map with a topology line.",
		Arguments: []*mcp.PromptArgument{
			{Name: "node_id1", Title: "Node 1", Description: "First node ID or unique name", Required: true},
			{Name: "node_id2", Title: "Node 2", Description: "Second node ID or unique name", Required: true},
			{Name: "polling_id1", Title: "Polling 1", Description: "Optional polling ID on Node 1", Required: false},
			{Name: "polling_id2", Title: "Polling 2", Description: "Optional polling ID on Node 2", Required: false},
			{Name: "info", Title: "Info", Description: "Optional line description", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		n1 := strings.TrimSpace(req.Params.Arguments["node_id1"])
		n2 := strings.TrimSpace(req.Params.Arguments["node_id2"])
		if n1 == "" || n2 == "" {
			return nil, fmt.Errorf("node_id1 and node_id2 are required")
		}
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"node_id1", "Node 1"},
			{"node_id2", "Node 2"},
			{"polling_id1", "Polling 1"},
			{"polling_id2", "Polling 2"},
			{"info", "Info"},
		})
		return promptResult("Add line prompt", formatToolPrompt("Connect two nodes using add_line tool", c)), nil
	})

	// 24. delete_line
	server.AddPrompt(&mcp.Prompt{
		Name:        "delete_line",
		Title:       "Delete topology line",
		Description: "Delete a topology connection line by ID.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Line ID", Description: "Line ID to delete", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := strings.TrimSpace(req.Params.Arguments["id"])
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		return promptResult("Delete line prompt", fmt.Sprintf("Delete topology line %q using delete_line tool.", id)), nil
	})

	// 25. get_polling
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_polling",
		Title:       "Get polling details",
		Description: "Get full configuration and result details for a polling.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Polling ID", Description: "Polling ID", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := strings.TrimSpace(req.Params.Arguments["id"])
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		return promptResult("Get polling prompt", fmt.Sprintf("Get full configuration of polling %q using get_polling tool.", id)), nil
	})

	// 26. add_polling
	server.AddPrompt(&mcp.Prompt{
		Name:        "add_polling",
		Title:       "Add a polling",
		Description: "Add a new monitoring polling task to an existing node.",
		Arguments: []*mcp.PromptArgument{
			{Name: "node_id", Title: "Node ID", Description: "Target node ID or unique name", Required: true},
			{Name: "name", Title: "Name", Description: "Polling name", Required: true},
			{Name: "type", Title: "Type", Description: "Polling type (ping, snmp, http, tcp, dns, tls, syslog, trap, netflow, etc.)", Required: true},
			{Name: "params", Title: "Params", Description: "Polling target parameters", Required: false},
			{Name: "script", Title: "Script", Description: "Normal condition expression", Required: false},
			{Name: "level", Title: "Level", Description: "Failure alert level (off, info, warn, low, high)", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		if strings.TrimSpace(req.Params.Arguments["node_id"]) == "" || strings.TrimSpace(req.Params.Arguments["name"]) == "" || strings.TrimSpace(req.Params.Arguments["type"]) == "" {
			return nil, fmt.Errorf("node_id, name, and type are required")
		}
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"node_id", "Node"},
			{"name", "Name"},
			{"type", "Type"},
			{"params", "Params"},
			{"script", "Script"},
			{"level", "Level"},
		})
		return promptResult("Add polling prompt", formatToolPrompt("Add a monitoring polling using add_polling tool", c)), nil
	})

	// 27. update_polling
	server.AddPrompt(&mcp.Prompt{
		Name:        "update_polling",
		Title:       "Update a polling",
		Description: "Modify an existing polling configuration.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Polling ID", Description: "Polling ID to update", Required: true},
			{Name: "name", Title: "Name", Description: "New polling name", Required: false},
			{Name: "params", Title: "Params", Description: "New target parameters", Required: false},
			{Name: "script", Title: "Script", Description: "New evaluation script", Required: false},
			{Name: "level", Title: "Level", Description: "New alert level", Required: false},
			{Name: "poll_int", Title: "Interval", Description: "New polling interval in seconds", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		if strings.TrimSpace(req.Params.Arguments["id"]) == "" {
			return nil, fmt.Errorf("id is required")
		}
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"id", "Polling ID"},
			{"name", "Name"},
			{"params", "Params"},
			{"script", "Script"},
			{"level", "Level"},
			{"poll_int", "Interval"},
		})
		return promptResult("Update polling prompt", formatToolPrompt("Update an existing polling using update_polling tool", c)), nil
	})

	// 28. delete_polling
	server.AddPrompt(&mcp.Prompt{
		Name:        "delete_polling",
		Title:       "Delete a polling",
		Description: "Delete a polling task by ID.",
		Arguments: []*mcp.PromptArgument{
			{Name: "id", Title: "Polling ID", Description: "Polling ID to delete", Required: true},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		id := strings.TrimSpace(req.Params.Arguments["id"])
		if id == "" {
			return nil, fmt.Errorf("id is required")
		}
		return promptResult("Delete polling prompt", fmt.Sprintf("Delete polling %q using delete_polling tool.", id)), nil
	})

	// 29. check_polling
	server.AddPrompt(&mcp.Prompt{
		Name:        "check_polling",
		Title:       "Execute polling recheck",
		Description: "Immediately execute a polling check for a polling ID or node.",
		Arguments: []*mcp.PromptArgument{
			{Name: "polling_id", Title: "Polling ID", Description: "Specific polling ID to run", Required: false},
			{Name: "node_id", Title: "Node ID", Description: "Node ID or name to recheck non-normal pollings", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"polling_id", "Polling ID"},
			{"node_id", "Node"},
		})
		return promptResult("Check polling prompt", formatToolPrompt("Run polling check immediately using check_polling tool", c)), nil
	})

	// 30. search_netflow
	server.AddPrompt(&mcp.Prompt{
		Name:        "search_netflow",
		Title:       "Search NetFlow/IPFIX logs",
		Description: "Search NetFlow and IPFIX traffic flow logs.",
		Arguments: []*mcp.PromptArgument{
			{Name: "src_filter", Title: "Source IP filter", Description: "Regex filter for source IP", Required: false},
			{Name: "dst_filter", Title: "Destination IP filter", Description: "Regex filter for destination IP", Required: false},
			{Name: "protocol_filter", Title: "Protocol filter", Description: "Regex filter for protocol", Required: false},
			{Name: "port_filter", Title: "Port filter", Description: "Regex filter for port", Required: false},
			{Name: "start_time", Title: "Start time", Description: "Start time or duration (e.g. -1h)", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"src_filter", "Source filter"},
			{"dst_filter", "Destination filter"},
			{"protocol_filter", "Protocol filter"},
			{"port_filter", "Port filter"},
			{"start_time", "Start time"},
		})
		return promptResult("Search NetFlow prompt", formatToolPrompt("Search NetFlow/IPFIX records using search_netflow tool", c)), nil
	})

	// 31. get_ai_anomaly_list
	server.AddPrompt(&mcp.Prompt{
		Name:        "get_ai_anomaly_list",
		Title:       "Get AI anomaly scores",
		Description: "Retrieve AI anomaly detection scores across AI-monitored pollings.",
		Arguments: []*mcp.PromptArgument{
			{Name: "node_filter", Title: "Node filter", Description: "Regex filter for node name", Required: false},
			{Name: "min_score", Title: "Minimum score", Description: "Minimum anomaly score threshold", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"node_filter", "Node filter"},
			{"min_score", "Minimum score"},
		})
		return promptResult("Get AI anomaly list prompt", formatToolPrompt("Retrieve AI anomaly detection scores using get_ai_anomaly_list tool", c)), nil
	})

	// 32. polling_correlation_analysis
	server.AddPrompt(&mcp.Prompt{
		Name:        "polling_correlation_analysis",
		Title:       "Analyze polling correlations",
		Description: "Discover correlated polling metrics across nodes or analyze a specific polling pair for cascading failures and leading indicators.",
		Arguments: []*mcp.PromptArgument{
			{Name: "base_polling_id", Title: "Base Polling ID", Description: "Optional base polling ID (leave empty for all pairs)", Required: false},
			{Name: "algorithm", Title: "Algorithm", Description: "pearson, spearman, kendall, crosscorr, or dcor", Required: false},
			{Name: "hours", Title: "Lookback Hours", Description: "Lookback window in hours (e.g. 24, 168)", Required: false},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		c := collectPromptFilterArgs(req.Params.Arguments, [][2]string{
			{"base_polling_id", "Base Polling ID"},
			{"algorithm", "Algorithm"},
			{"hours", "Lookback Hours"},
		})
		return promptResult("Polling correlation analysis prompt", formatToolPrompt("Discover and analyze polling correlations using discover_polling_correlations and analyze_polling_correlation tools", c)), nil
	})
}

func collectPromptFilterArgs(args map[string]string, fields [][2]string) []string {
	var out []string
	for _, f := range fields {
		if v := strings.TrimSpace(args[f[0]]); v != "" {
			out = append(out, fmt.Sprintf("- %s: %s", f[1], v))
		}
	}
	return out
}

func formatToolPrompt(header string, lines []string) string {
	if len(lines) == 0 {
		return header + "."
	}
	return header + " with the following parameters:\n" + strings.Join(lines, "\n")
}

func promptResult(desc, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: desc,
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: text,
				},
			},
		},
	}
}
