---
title: Getting Started with TWSNMP NEO
layout: default
---

[日本語マニュアルはこちら (Japanese)](./index_ja.html)

# Getting Started with TWSNMP NEO
Next-Generation Container-Native & Web-Based Network Management System

![](./images/appicon.png){: width="200"}

---

## Table of Contents

1. [Introduction](#introduction)
2. [Installation & Startup](#installation--startup)
   - [Running with Docker](#running-with-docker)
   - [Running with Docker Compose](#running-with-docker-compose)
   - [Running as a Standalone Binary](#running-as-a-standalone-binary)
   - [Linux Capabilities Configuration](#linux-capabilities-configuration)
3. [User Interface Overview](#user-interface-overview)
4. [Map Screen](#map-screen)
   - [Nodes and Networks (SW-HUB Port Panel)](#nodes-and-networks-sw-hub-port-panel)
   - [Connection Lines & Packet Flow Animation](#connection-lines--packet-flow-animation)
   - [Draw Items (11 Types)](#draw-items-11-types)
   - [3D Virtual Hardware Panel (vpanel)](#3d-virtual-hardware-panel-vpanel)
   - [Map Settings & Background Images](#map-settings--background-images)
5. [Auto-Discovery & Topology Exploration](#auto-discovery--topology-exploration)
   - [IP Range Auto-Discovery](#ip-range-auto-discovery)
   - [Neighbor Topology Finder (Find Neighbor)](#neighbor-topology-finder-find-neighbor)
6. [Node & Polling Management](#node--polling-management)
   - [Node List & Properties](#node-list--properties)
   - [Extensive Polling Protocols](#extensive-polling-protocols)
   - [AI Polling Setup Assistant](#ai-polling-setup-assistant)
7. [Diagnostic & Operational Tools](#diagnostic--operational-tools)
   - [Ping / Smokeping / MTR](#ping--smokeping--mtr)
   - [MIB Browser & MIB Tree Explorer](#mib-browser--mib-tree-explorer)
   - [gNMI Tool](#gnmi-tool)
   - [Wake-on-LAN (WOL)](#wake-on-lan-wol)
8. [Analytics & Reporting Suite](#analytics--reporting-suite)
   - [Device Analytics (LAN / Wi-Fi / Bluetooth / FDB)](#device-analytics-lan--wi-fi--bluetooth--fdb)
   - [IPAM (IP Address Management Heatmap)](#ipam-ip-address-management-heatmap)
   - [Flow & Traffic Analysis (NetFlow / sFlow / Fumble)](#flow--traffic-analysis-netflow--sflow--fumble)
   - [Host Analysis (Windows Events / Processes)](#host-analysis-windows-events--processes)
   - [IoT & Environmental Sensors (Temperature / Power / MQTT)](#iot--environmental-sensors-temperature--power--mqtt)
   - [TLS/SSL Certificate Monitoring](#tlsssl-certificate-monitoring)
   - [AI Anomaly Detection Scores](#ai-anomaly-detection-scores)
9. [High-Speed Log Management (Apache Parquet)](#high-speed-log-management-apache-parquet)
   - [Supported Log Types](#supported-log-types)
   - [Receivers & Daemons Settings](#receivers--daemons-settings)
   - [AI Root Cause Diagnosis](#ai-root-cause-diagnosis)
10. [Built-in PKI (Certificate Authority)](#built-in-pki-certificate-authority)
    - [Root CA Setup & Certificate Ledger](#root-ca-setup--certificate-ledger)
    - [In-Browser CSR Generation](#in-browser-csr-generation)
    - [CRL / OCSP / SCEP / ACME Services](#crl--ocsp--scep--acme-services)
11. [AI & MCP Integration](#ai--mcp-integration)
    - [LLM Configuration (Local LLM / Cloud AI Services)](#llm-configuration-local-llm--cloud-ai-services)
    - [Native Streamable HTTP MCP Server](#native-streamable-http-mcp-server)
12. [Settings & System Administration](#settings--system-administration)
    - [System Status & Resource Monitor](#system-status--resource-monitor)
    - [Notifications (Email / Webhook)](#notifications-email--webhook)
    - [Backup & Automated Log Retention](#backup--automated-log-retention)
    - [Icon Management & Custom Assets](#icon-management--custom-assets)
    - [User Account Management & Access Control](#user-account-management--access-control)

---

## 1. Introduction

**TWSNMP NEO** represents the next generation of the renowned **TWSNMP** network management system series.
It consolidates the container portability of **TWSNMP FC** and the advanced visual and AI capabilities of **TWSNMP FK**, built from the ground up on modern web architecture (Go 1.27+, Svelte 5 with Runes, and TypeScript).

![](./images/en/readme_hero_banner.png)

### Core Highlights
- **Zero-Install Web Experience**: Full management capabilities accessible from any modern desktop or mobile browser.
- **p5.js & 3D WebGL Canvas**: Hardware-accurate switching hub panels with real-time port LEDs and full 3D interactive hardware panels (`vpanel`).
- **Apache Parquet Columnar Storage**: Millions of Syslog, TRAP, NetFlow, and sFlow records stored with high compression ratios and sub-second query performance.
- **Native AI & Built-in MCP Server**: Integrated multi-LLM client (`tensai`) and Model Context Protocol (MCP) server for instant root cause diagnosis and natural language polling creation.
- **Self-Contained Private PKI**: Issue, renew, and automate TLS certificates via ACME and SCEP directly within your network manager.

---

## 2. Installation & Startup

TWSNMP NEO runs either as a Docker container or as a standalone lightweight executable binary.

### Running with Docker

```bash
docker run -d \
  --name twsnmpneo \
  --restart always \
  -p 8080:8080 \
  -p 514:514/udp \
  -p 162:162/udp \
  -p 2055:2055/udp \
  -p 6343:6343/udp \
  -v $(pwd)/data:/data \
  ghcr.io/twsnmp/twsnmpneo:latest
```

After startup, open your browser and go to `http://<host-ip>:8080`.

### Running with Docker Compose

Create a `docker-compose.yml` file:

```yaml
services:
  twsnmpneo:
    image: ghcr.io/twsnmp/twsnmpneo:latest
    container_name: twsnmpneo
    restart: always
    ports:
      - "8080:8080"       # Web UI & REST / MCP API
      - "514:514/udp"     # Syslog UDP
      - "162:162/udp"     # SNMP TRAP UDP
      - "2055:2055/udp"   # NetFlow / IPFIX
      - "6343:6343/udp"   # sFlow
    volumes:
      - ./data:/data
```

Launch with:

```bash
docker compose up -d
```

### Running as a Standalone Binary

Download the executable binary for your OS from the [GitHub Releases page](https://github.com/twsnmp/twsnmpneo/releases).

```bash
./twsnmpneo -datastore ./data -port 8080
```

### Linux Capabilities Configuration

When running as an unprivileged user on Linux, grant Capabilities for raw sockets (ICMP Ping) and privileged ports (Syslog 514, TRAP 162):

```bash
# Grant capabilities
sudo setcap 'cap_net_bind_service,cap_net_raw+ep' ./twsnmpneo

# Run as normal user
./twsnmpneo -datastore ./data -port 8080
```

---

## 3. User Interface Overview

TWSNMP NEO is engineered as a responsive Single Page Application (SPA).

![](./images/en/map_overview.png)

- **Top Navigation Bar**:
  - Quick switching between **Map, Reports, PKI, Logs, and Settings**.
  - **Light / Dark Mode Toggle** with immediate theme adaptation.
  - **Global Severity Counters** (Critical, Warning, Low, Unknown).
- **Main Interactive Canvas**: Pan, zoom, drag-and-drop nodes, and multiselect.
- **Context Menus**: Right-click any node or network container for immediate diagnostic tools and editors.

---

## 4. Map Screen

### Nodes and Networks (SW-HUB Port Panel)

- **Nodes**: Physical servers, routers, endpoints, and appliances with status glows (Green: Normal, Yellow: Warning, Red: Critical, Gray: Inactive).
- **Networks (SW-HUB Container)**: Visualizes physical switches with RJ45 port images, Link UP/DOWN status LEDs, port numbers, and auto-wrapping layout.

![](./images/en/map_network_ports.png)

Switching hubs and networks can also be managed via the dedicated inventory list:

![](./images/en/network_list.png)

### Connection Lines & Packet Flow Animation

Connect nodes and switch ports via drag-and-drop or Shift+click. When bound to traffic polling metrics, line widths dynamically scale with bandwidth utilization, and packet flow dots animate in real time.

Lines can also be audited and managed in the Lines inventory table (Source, Destination, Width, Info/Port discovery reason, and Connection Health):

![](./images/en/line_list.png)

### Draw Items (11 Types)

Enrich your topology map with functional and aesthetic widgets:

![](./images/en/map_drawitems.png)

1. **Rectangles / Ellipses**: Zone grouping and network perimeter boundaries
2. **Text / Labels**: Custom headers and descriptions
3. **Static Images**: Floor blueprints, office maps, or rack layouts
4. **Classic Gauge**: Analog dial needle meters
5. **Radial Gauge**: Donut charts for CPU / memory utilization
6. **Bar Gauge**: Horizontal resource meters (storage, bandwidth)
7. **Sparkline**: Time-series micro-charts
8. **Real-Time KPI Cards**: High-impact numeric metric displays with color thresholding

All configured draw items can also be audited and managed in the Draw Items inventory list:

![](./images/en/drawitem_list.png)

### 3D Virtual Hardware Panel (vpanel)

Right-click any switch node and select **Virtual Panel** to launch the 3D WebGL hardware representation.

![](./images/en/map_vpanel_3d.png)

- Full 3D orbital rotation and zoom (Orbit Controls).
- High-resolution RJ45 port textures with active Link UP/DOWN green LEDs, 1Gbps+ speed amber LEDs, and Power LEDs.

### Map Settings & Background Images

Right-click empty canvas space or open Settings to configure map properties:
- **Map Name & Canvas Size**: Auto-scaling or fixed virtual dimensions.
- **Node Icon Size**: 1 (Tiny) to 5 (Huge) slider adjustment.
- **Background Image**: Overlay network topologies onto architectural blueprints or regional maps with custom coordinate offsets and dimensions.
- **Import Map Data**: Restore layouts exported from TWSNMP FC, FK, or NEO.

![](./images/en/settings_map.png)

---

## 5. Auto-Discovery & Topology Exploration

### IP Range Auto-Discovery

Scan subnets (e.g., `192.168.1.0/24`) using simultaneous ICMP Ping, SNMP walk, and ARP discovery to populate maps automatically.

![](./images/en/discover_dialog.png)

During execution, real-time discovery progress and categorized service detection counters (SNMP, Web, SSH, Mail, File, RDP, LDAP) are displayed:

![](./images/en/discover_running.png)

### Neighbor Topology Finder (Find Neighbor)

Inspects switch ARP caches, Bridge MIB FDB tables, and LLDP/CDP neighbor tables to automatically identify and connect nodes to their exact switch ports.

![](./images/en/find_neighbor_dialog.png)

---

## 6. Node & Polling Management

### Node List & Properties

View and manage all monitored nodes, IP/MAC bindings, hardware vendors, and operational states in a real-time searchable inventory list:

![](./images/en/node_list.png)

Configure hostname, IP, MAC address, SNMP credentials (v1/v2c/v3 with authPriv), and icons. Click **Auto Detect** to automatically inspect device roles and open ports.

![](./images/en/node_dialog.png)

### Extensive Polling Protocols

- **PING**: ICMP, UDP, Smokeping, MTR
- **SNMP**: v1 / v2c / v3 Get, Walk, and Table extractions
- **HTTP / HTTPS**: Response codes, latency, body regex matching, SSL validity
- **TCP / TLS**: Port reachability and handshake latency
- **DNS / NTP**: Query resolution and time offset
- **gNMI**: gRPC telemetry streaming and config inspection
- **STUN**: Public IP monitoring and NAT type detection
- **MQTT**: IoT topic subscription and metric assertion

![](./images/en/polling_list.png)

Global default polling parameters (interval, timeout, retry count, SNMP mode, community string) can be configured under System Configuration:

![](./images/en/settings_polling.png)

### AI Polling Setup Assistant

Describe monitoring requirements in natural language (e.g., *"Create a polling that alerts if CPU utilization exceeds 85%"*), and the AI automatically generates the appropriate OID, polling type, and threshold expression. *(Experimental feature under development)*

---

## 7. Diagnostic & Operational Tools

### Ping / Smokeping / MTR

- **Real-Time Ping**: Continuous ICMP/UDP ping with real-time response graphs and audio cues.
- **Smokeping**: Visualizes latency jitter and packet drop distributions.
- **MTR**: Per-hop traceroute latency statistics with AI diagnostic summaries.

![](./images/en/ping_tool.png)

### MIB Browser & MIB Tree Explorer

Browse standard RFC MIBs and vendor private MIB trees. Perform Get / GetNext / Walk / Table operations, sort table columns, and export data directly to CSV.

![](./images/en/mib_browser.png)

![](./images/en/mib_tree.png)

### gNMI Tool

Query modern telemetry streams and inspect configurations over gRPC.

![](./images/en/gnmi_tool.png)

### Wake-on-LAN (WOL)

Execute directly from the node's right-click context menu on the map or via the "WOL" button at the top of the Node Details modal. Broadcasts Magic Packets to the target MAC address for instant remote power-on.

---

## 8. Analytics & Reporting Suite

TWSNMP NEO incorporates 8 dedicated analytics engines:

![](./images/en/report_device_lan.png)

### Device Analytics (LAN / Wi-Fi / Bluetooth / FDB)
Vendor OUI breakdown, signal strength, and rogue device tracking.

### IPAM (IP Address Management Heatmap)
Multi-subnet utilization heatmaps and IP conflict detection.

![](./images/en/report_ipam_heatmap.png)

### Flow & Traffic Analysis (NetFlow / sFlow / Fumble)
Top talkers, top ports, and Fumble anomalous connection tracking.

### Host Analysis (Windows Events / Processes)
Windows Security event auditing, privilege escalation, and active processes.

### IoT & Environmental Sensors (Temperature / Power / MQTT)
Temperature, humidity, pressure, and energy consumption (Wh) trends.

### TLS/SSL Certificate Monitoring
Expiration countdown and revocation status verification.

### AI Anomaly Detection Scores
Statistical anomaly scoring and ranking across nodes and pollings.

![](./images/en/report_polling_sla.png)

![](./images/en/report_event_analytics.png)

---

## 9. High-Speed Log Management (Apache Parquet)

Built on **Apache Parquet** columnar compression, TWSNMP NEO processes massive log volumes with exceptional speed and minimal storage overhead.

![](./images/en/log_event.png)

![](./images/en/log_syslog.png)

### Supported Log Types
- **Event Logs**: System lifecycle events, status changes, and AI Root Cause analysis.
- **Syslog**: UDP/TCP/TLS reception, severity breakdowns, log normalization, and FFT periodicity analysis.
- **SNMP TRAP**: v1/v2c/v3 traps with MIB OID name translation.
- **NetFlow / IPFIX & sFlow**: Flow samples and counter telemetry.
- **ARP Watch**: Continuous IP-MAC binding change and conflict logs.
- **OpenTelemetry**: Distributed traces and OTLP log ingestion.
- **MQTT Logs**: Real-time IoT message histories.

![](./images/en/log_arp.png)

### Receivers & Daemons Settings
Log and packet receiving daemons (Syslog UDP/TCP:514, SNMP TRAP UDP:162, NetFlow UDP:2055, sFlow UDP:6343, ARP Watch CIDR, OpenTelemetry OTLP gRPC/HTTP:4318, MQTT Broker) can be enabled or configured under Receivers & Daemons settings:

![](./images/en/settings_receivers.png)

### AI Root Cause Diagnosis
When critical events occur, LLM-based intelligent analysis automatically summarizes correlated logs and provides immediate remediation recommendations. *(Experimental feature under development)*

---

## 10. Built-in PKI (Certificate Authority)

A complete, self-contained Private PKI system embedded directly within your network manager. Modeled after the production-grade TWSNMP FC Web server architecture, it offers an intuitive left-sidebar interface with full protocol server integration.

![](./images/en/pki_root_ca.png)

### Root CA Setup & Certificate Ledger
Generate Root CA keys and certificates securely (supporting RSA and ECDSA key curves) with explicit initialization. Features unified keyword search, column sorting, pagination, CSV export, individual PEM download, and revocation actions.

### In-Browser CSR Generation
Simultaneously generate private keys and CSRs with immediate ZIP package downloads. Issue and download signed PEM certificates from uploaded CSR files.

### CRL / OCSP / SCEP / ACME Services
- **Plain HTTP Listener (Port 8082)**: Serves Root CA certificates (`/ca.pem`), SCEP CA certificates (`/scepca.pem`), CRL (`/crl`), OCSP (`/ocsp`), and SCEP (`/scep`) (preventing TLS bootstrap issues and circular dependencies).
- **ACME Listener (Port 8083)**: RFC 8555-compliant Automated Certificate Management Environment (Let's Encrypt compatible).
- Real-time service status monitoring and immediate runtime configuration updates.

![](./images/en/pki_cert_manager.png)

---

## 11. AI & MCP Integration

### LLM Configuration (Local LLM / Cloud AI Services)

Configure AI providers (Local `tensai` LLM, Gemini, OpenAI, Claude, Ollama) and parameters directly from the settings.

![](./images/en/ai_settings.png)

#### Local LLM & WebGPU Acceleration
TWSNMP NEO includes a built-in offline inference engine requiring zero external cloud dependencies or API keys. Supports one-click Hugging Face model downloads and hardware-accelerated local WebGPU inference.

![](./images/en/ai_gpu_manager.png)

### Native Streamable HTTP MCP Server

TWSNMP NEO is designed to provide a **Model Context Protocol (MCP)** server over Streamable HTTP (`/api/mcp`).
Autonomous AI agents like Cursor, Claude Desktop, and Antigravity will connect directly to inspect network status, query logs, and troubleshoot incidents. *(Experimental feature under development)*

Refer to the **[MCP Specification & Guide](mcp.html)** and **[AI Prompts Guide](prompt.html)** for architecture details.

---

## 12. Settings & System Administration

### System Status & Resource Monitor

Monitor server daemons, CPU/memory/swap/disk consumption, TCP connections, and database capacity in real time:

![](./images/en/system_monitor.png)

- **Daemon Status**: Live operational states for Syslog, SNMP TRAP, NetFlow, sFlow, ARP Watch, OpenTelemetry, MQTT, and MCP Server.
- **Resource Trends**: CPU, memory heap, and process utilization time charts.
- **Capacity Forecast & DB Backup**: One-click database backups and storage growth predictions.

### Notifications (Email / Webhook)

Receive instant alerts and periodic reports across supported channels:
- **Email** (SMTP / Google OAuth2 / Microsoft 365 OAuth2): Formatted HTML alert summaries and daily operational reports.
- **Webhooks**: Real-time event notifications and daily report postings sent directly to your custom webhook endpoints or automation workflows.
- **External Command Execution**: Execute arbitrary OS scripts or commands triggered on fault level state changes.

![](./images/en/settings_notify.png)

### Backup & Automated Log Retention

![](./images/en/settings_backup.png)

- **Map Export**: Export topology, nodes, and polling configurations as JSON or ZIP archives.
- **Parquet Auto-Rotation**: Automatically prune historical records based on disk usage thresholds or retention days.

### Icon Management & Custom Assets

Manage built-in Material Design Icons (MDI) and import/export custom PNG/JPEG/SVG image icons for map nodes and topology elements.

![](./images/en/settings_icon.png)

### User Account Management & Access Control

Manage multi-user access with granular role-based permissions (`ADMINISTRATOR`, `OPERATOR`, `READ-ONLY`) and secure credential management.

![](./images/en/settings_user.png)

---

## Conclusion

TWSNMP NEO delivers a robust, container-native network observability platform with 3D visualization, big data log storage, and native AI integration. Start managing your network today using Docker or standalone binaries!



