---
title: TWSNMP NEO MCP Server Specification
layout: default
---

[日本語マニュアルはこちら (Japanese)](./mcp_ja.html) | [Main Manual](./index.html)

# TWSNMP NEO MCP Server Specification

This document details the architecture, available tools, and AI client configuration for the built-in **Model Context Protocol (MCP)** server in TWSNMP NEO.

---

## 1. Overview

The Model Context Protocol (MCP) is an open standard that allows LLMs and autonomous AI agents to discover, interact with, and securely call operational tools and data sources.
With TWSNMP NEO's built-in MCP server, AI assistants can query network topologies, execute real-time pings, inspect SNMP trees, search big data Parquet logs, and assess TLS certificate validity.

---

## 2. Transport and Endpoints

TWSNMP NEO features a high-performance **Streamable HTTP** transport.

- **Endpoint**: `http://<TWSNMP_NEO_HOST>:<PORT>/api/mcp`
- **Protocol**: Streamable HTTP (JSON-RPC 2.0 / Server-Sent Events)
- **Port**: Shares the primary Web UI port (Default: `8080`)

### Security Controls
- **Enable MCP**: MCP is **disabled by default** (`EnableMCP: false`). It must be enabled in Web UI Map Settings (`EnableMCP`); when disabled, `/api/mcp` returns `404`.
1. **IP Whitelisting (`-mcpFrom` / `TWSNMP_MCPFROM` / Map Settings)**:
   - Restrict incoming connections to trusted IP addresses or CIDR ranges (comma-separated, e.g. `127.0.0.1, 192.168.1.50, 10.0.0.0/24`).
   - Loopback addresses (`127.0.0.1`, `::1`, `localhost`) are always permitted; unauthorized or unidentifiable IPs receive `401 Unauthorized`.
2. **Authentication Modes (`-mcpMode auth|noauth` / `TWSNMP_MCPMODE` / Map Settings)**:
   - `auth` mode (**default**): Requests must include an `Authorization: Bearer <API_TOKEN>` header (JWT obtained via the Web UI or `POST /api/login`).
   - `noauth` mode: Access is governed exclusively by IP/CIDR whitelisting without requiring a Bearer token (intended for trusted loopback/local agent setups).
   - Explicit CLI flags (`-mcpFrom`, `-mcpMode`) or environment variables (`TWSNMP_MCPFROM`, `TWSNMP_MCPMODE`) take precedence over stored Map Settings on startup.

---

## 3. Available Tools

The MCP server exposes 44 operational tools and 32 prompt templates:

### 3.1 Topology & Map Operations

| Tool | Description | Key Parameters |
|---|---|---|
| `get_node_list` | Retrieve list of network nodes | `state_filter`, `name_filter`, `ip_filter` |
| `get_node` | Get detailed node attributes and attached pollings | `id` (required: Node ID, name, or IP) |
| `add_node` | Register a new node on the map (node only; does not auto-create a PING polling—add pollings via `add_polling`) | `name` (required), `ip` (required), `icon`, `description`, `x`, `y` |
| `update_node` | Modify existing node configuration | `id` (required), `name`, `ip`, `icon`, `description`, `x`, `y` |
| `delete_node` | Delete a node and cascade-remove its attached pollings and connected lines | `id` (required: Node ID or unique node name) |
| `get_network_list` | Retrieve SW-HUB / network containers | `name_filter`, `ip_filter` |
| `get_line_list` | Retrieve topology connection lines | `node_filter`, `state_filter` |
| `add_line` | Create a topology line connecting two nodes or `NET:<network_id>` containers | `node_id1` (required), `node_id2` (required), `polling_id1`, `polling_id2`, `polling_id`, `width`, `port`, `info` |
| `delete_line` | Delete a topology connection line | `id` (required) |
| `get_system_status` | Retrieve system health, memory/goroutines, `uptime`/`uptime_sec`, node/polling counters, and configured receiver statuses | None |
| `diagnose_node` | Run live Ping, SNMP, and HTTP diagnostic probes | `node_id` (required) |

### 3.2 Monitoring, Polling & Diagnostics

| Tool | Description | Key Parameters |
|---|---|---|
| `get_polling_list` | Retrieve configured pollings | `node_id`, `type_filter`, `name_filter`, `node_name_filter`, `state_filter`, `level_filter` |
| `get_polling` | Retrieve full polling configuration and latest status | `id` (required) |
| `add_polling` | Create a monitoring polling task on an existing node | `node_id` (required), `name` (required), `type` (required), `mode`, `params`, `filter`, `extractor`, `script`, `level`, `poll_int`, `timeout`, `retry`, `log_mode`, `ai_mode`, `fail_action`, `repair_action` |
| `update_polling` | Modify an existing polling and schedule an immediate recheck | `id` (required), `name`, `type`, `mode`, `params`, `filter`, `extractor`, `script`, `level`, `poll_int`, `timeout`, `retry`, `log_mode`, `ai_mode`, `fail_action`, `repair_action`, `reset_state` |
| `delete_polling` | Delete a polling task and clean up bound topology lines | `id` (required) |
| `check_polling` | Execute a specific polling immediately, or queue non-normal pollings by node or globally | `polling_id`, `node_id`, `all` |
| `get_polling_log` | Retrieve execution log history | `id` (required), `limit` |
| `get_polling_log_data` | Retrieve numeric time-series metrics in CSV format | `id` (required), `limit` |
| `do_ping` | Execute real-time ICMP/UDP Ping | `target` (required), `size`, `count`, `timeout` |
| `snmpwalk` | Perform SNMP Walk on target (v1/v2c/v3) | `target` (required), `node_id`, `root_oid`, `snmp_mode`, `community`, `user`, `password` |
| `snmpset` | Set SNMP MIB variable on target (v1/v2c/v3) | `target` (required), `mib_object_name` (required), `type` (`integer`/`string`, required), `value` (required), `node_id`, `snmp_mode`, `community`, `user`, `password` |
| `get_mib_tree` | Retrieve compiled MIB OID tree | None |

### 3.3 Log Search, NetFlow & Investigation (Parquet)

| Tool | Description | Key Parameters |
|---|---|---|
| `search_event_log` | Query system event logs | `node_filter`, `type_filter`, `level_filter`, `event_filter`, `start_time`, `end_time`, `limit` |
| `add_event_log` | Append an audit event to the log | `level` (required), `event` (required), `type`, `node_id` |
| `search_syslog` | High-speed Syslog cross-search | `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time`, `end_time`, `limit` |
| `get_syslog_summary` | Aggregate Syslog by host/tag/severity | `summary_type` (`host`/`tag`/`severity`, required), `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time`, `end_time` |
| `search_snmp_trap_log` | Query received SNMP TRAPs | `from_filter`, `type_filter`, `message_filter`, `start_time`, `end_time`, `limit` |
| `search_netflow` | Query NetFlow / IPFIX traffic flow logs | `src_filter`, `dst_filter`, `protocol_filter`, `port_filter`, `start_time`, `end_time`, `limit` |

### 3.4 Reports, Inventory, OTel & AI

| Tool | Description | Key Parameters |
|---|---|---|
| `get_ip_address_list` | Inventory of discovered IP addresses | None |
| `get_ip_address_info` | Lookup specific IP address history | `ip` (required) |
| `get_mac_address_list` | MAC address table with OUI vendor lookup | None |
| `get_mac_address_info` | Lookup MAC address node and vendor info | `mac` (required) |
| `get_sensor_list` | Retrieve remote monitoring sensors (`twWifiScan`, `twBlueScan`, `twpcap`, `twwinlog`, `twagentneo`) | `type_filter`, `state_filter` |
| `get_wifi_ap_list` | Discovered Wi-Fi APs and signal stats | None |
| `get_bluetooth_device_list` | Bluetooth beacons and devices | `public_address_only` |
| `get_server_certificate_list` | TLS/SSL certificate status & validity periods | None |
| `get_resource_monitor_list` | Host CPU, memory, swap, disk, and load history | None |
| `get_report_summary` | Retrieve stored records for a specified report category | `report_type` (required: `wifiAP`, `blueDevice`, `envMonitor`, `dnsq`, `winEventID`, `twAgentAlert`, `twAgentInfo`, etc.), `limit` |
| `get_ai_anomaly_list` | Retrieve AI anomaly detection scores | `node_filter`, `polling_filter`, `min_score`, `limit` |
| `analyze_polling_correlation` | Analyze correlation between two pollings (statistical & `tensai` ML) | `polling_id_a`, `polling_id_b`, `algorithm`, `hours` |
| `discover_polling_correlations` | Auto-discover correlated polling pairs across nodes | `base_polling_id`, `algorithm`, `hours`, `min_score`, `limit` |
| `get_otel_metrics` | Retrieve OpenTelemetry metrics | `name_filter`, `service_filter`, `host_filter`, `limit` |
| `get_otel_traces` | Retrieve OpenTelemetry distributed traces | `service_filter`, `error_only`, `limit` |

### 3.5 Built-in MCP Prompts (32 Templates)

- **Topology & Node**: `get_node_list`, `get_node`, `add_node`, `update_node`, `delete_node`, `get_network_list`, `get_line_list`, `add_line`, `delete_line`
- **Polling & Diagnostics**: `get_polling_list`, `get_polling`, `add_polling`, `update_polling`, `delete_polling`, `check_polling`, `get_polling_log`, `do_ping`, `snmpwalk`, `get_mib_tree`
- **Logs, Flow & AI**: `search_event_log`, `add_event_log`, `search_syslog`, `get_syslog_summary`, `search_snmp_trap_log`, `search_netflow`, `get_ai_anomaly_list`, `polling_correlation_analysis`
- **Inventory & Reports**: `get_ip_address_list`, `get_ip_address_info`, `get_mac_address_info`, `get_resource_monitor_list`, `get_server_certificate_list`

---

## 4. Client Integration Examples

### Google Antigravity (`~/.gemini/config/mcp_config.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_TOKEN"
      }
    }
  }
}
```

- When connecting locally with `-mcpMode noauth` (loopback only), omit the `headers` block.
- After editing `mcp_config.json` or restarting TWSNMP NEO with new MCP tools, inspect/refresh the server entry via **Additional Options (`...`) > MCP Servers** in Antigravity.

### Cursor Configuration (`~/.cursor/mcp.json` or `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_API_TOKEN"
      }
    }
  }
}
```

### Claude Desktop Configuration (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp"
    }
  }
}
```

---

## 5. Typical Use Cases

1. **"Register host `app-01` (`192.168.1.50`), add a PING polling, and run an immediate check"**
   - Agent calls `add_node` to register the node, `add_polling` (`type: "ping"`) to attach monitoring, and `check_polling` to test reachability immediately.
2. **"Find any AI anomalies or suspicious NetFlow traffic in the past hour"**
   - Agent invokes `get_ai_anomaly_list` and `search_netflow` alongside `search_event_log` to correlate anomalous pollings and flows.
3. **"List all SSL certificates expiring soon"**
   - Agent calls `get_server_certificate_list` and highlights certificates needing renewal.
