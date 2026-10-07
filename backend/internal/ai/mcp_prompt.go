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
		Description: "Add a new node to TWSNMP NEO (automatically adds a PING poller).",
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
