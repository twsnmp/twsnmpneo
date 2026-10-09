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
1. **IP Whitelisting (`-mcpFrom` / Map Settings)**:
   - Restrict incoming connections to trusted IP addresses (e.g., `127.0.0.1, 192.168.1.50`).
   - Defaults to local loopback access only.
2. **Authentication Modes (`-mcpMode auth|noauth`)**:
   - `auth` mode: Requests must include an `Authorization: Bearer <MCP_TOKEN>` header.
   - `noauth` mode: Access is governed exclusively by IP whitelisting.

---

## 3. Available Tools

The MCP server exposes 22+ operational tools:

### 3.1 Topology & Map Operations

| Tool | Description | Key Parameters |
|---|---|---|
| `get_node_list` | Retrieve list of network nodes | `state_filter`, `name_filter`, `ip_filter` |
| `get_node` | Get detailed node attributes | `id` (Node ID or Name) |
| `add_node` | Register a new node on the map | `name`, `ip`, `icon`, `description`, `position` |
| `update_node` | Modify existing node configuration | `id`, `name`, `ip`, `icon`, `description`, `position` |
| `delete_node` | Delete a node and its attached pollings | `id` |
| `get_network_list` | Retrieve SW-HUB / network containers | `name_filter`, `ip_filter` |
| `get_line_list` | Retrieve topology connection lines | `node_filter`, `state_filter` |

### 3.2 Monitoring & Diagnostics

| Tool | Description | Key Parameters |
|---|---|---|
| `get_polling_list` | Retrieve configured pollings | `type_filter`, `name_filter`, `node_name_filter`, `state_filter` |
| `get_polling_log` | Retrieve execution log history | `id` (required), `limit` |
| `get_polling_log_data`| Retrieve numeric time-series data | `id` (required), `limit` |
| `do_ping` | Execute real-time ICMP/UDP Ping | `target` (required), `size`, `count`, `timeout` |
| `snmpwalk` | Perform SNMP Walk on target | `target` (required), `oid` |
| `get_mib_tree` | Retrieve compiled MIB OID tree | None |

### 3.3 Log Search & Investigation (Parquet)

| Tool | Description | Key Parameters |
|---|---|---|
| `search_event_log` | Query system event logs | `node_filter`, `level_filter`, `event_filter`, `start_time`, `end_time` |
| `add_event_log` | Append an audit event to the log | `level`, `event`, `node_id` |
| `search_syslog` | High-speed Syslog cross-search | `host_filter`, `tag_filter`, `level_filter`, `message_filter`, `start_time` |
| `get_syslog_summary` | Aggregate Syslog by host/tag/level | `summary_type` (`host`/`tag`/`severity`), `host_filter` |
| `search_snmp_trap_log`| Query received SNMP TRAPs | `from_filter`, `type_filter`, `message_filter` |

### 3.4 Reports & Inventory

| Tool | Description | Key Parameters |
|---|---|---|
| `get_ip_address_list` | Inventory of discovered IP addresses | `ip_filter`, `mac_filter` |
| `get_ip_address_info` | Lookup specific IP address history | `ip` (required) |
| `get_mac_address_list`| MAC address table with OUI vendor lookup | `mac_filter`, `vendor_filter` |
| `get_sensor_list` | IoT & environmental sensor readings | `type_filter`, `name_filter` |
| `get_wifi_ap_list` | Discovered Wi-Fi APs and signal stats | `ssid_filter`, `bssid_filter` |
| `get_bluetooth_device_list` | Bluetooth beacons and devices | `name_filter`, `address_filter` |
| `get_server_certificate_list` | TLS/SSL certificate status & expiry | `host_filter`, `status_filter` |
| `get_resource_monitor_list` | Host CPU, memory, and disk telemetry | `host_filter` |

---

## 4. Client Integration Examples

### Cursor Configuration (`~/.cursor/mcp.json` or `.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "twsnmpneo": {
      "url": "http://127.0.0.1:8080/api/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_MCP_TOKEN"
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

1. **"Find any network nodes reporting errors in the past 2 hours"**
   - Agent invokes `search_event_log` and `search_syslog` to summarize failure causes.
2. **"Test connectivity to 192.168.1.1 and report latency"**
   - Agent triggers `do_ping` and returns round-trip statistics.
3. **"List all SSL certificates expiring within 14 days"**
   - Agent calls `get_server_certificate_list` and highlights certificates needing renewal.
