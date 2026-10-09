# TWSNMP NEO

[日本語版はこちら (Japanese)](README_ja.md)

[![Go Report Card](https://goreportcard.com/badge/github.com/twsnmp/twsnmpneo)](https://goreportcard.com/report/github.com/twsnmp/twsnmpneo)
![GitHub Go version](https://img.shields.io/github/go-mod/go-version/twsnmp/twsnmpneo)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/twsnmp/twsnmpneo)
![GitHub License](https://img.shields.io/github/license/twsnmp/twsnmpneo)
![GitHub Repo stars](https://img.shields.io/github/stars/twsnmp/twsnmpneo?style=social)

【Built with】
![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Svelte](https://img.shields.io/badge/svelte-%23f1413d.svg?style=for-the-badge&logo=svelte&logoColor=white)
![TypeScript](https://img.shields.io/badge/typescript-%23007ACC.svg?style=for-the-badge&logo=typescript&logoColor=white)
![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)

**TWSNMP NEO** is a next-generation container-native network management system (NMS). Combining the strengths of **TWSNMP FC** (Container/Web edition) and **TWSNMP FK** (Desktop edition), it provides high performance, modern web UI, advanced AI/MCP integrations, and columnar log storage with Apache Parquet.

![TWSNMP NEO Hero](docs/images/en/readme_hero_banner.png)

---

## Documentation

* **[English Manual (GitHub Pages)](https://twsnmp.github.io/twsnmpneo/)**
* **[日本語マニュアル (GitHub Pages)](https://twsnmp.github.io/twsnmpneo/index_ja.html)**
* **[MCP Server Specification & Guide](https://twsnmp.github.io/twsnmpneo/mcp.html)**
* **[AI Prompts & Diagnostics](https://twsnmp.github.io/twsnmpneo/prompt.html)**

---

## Key Features

### 1. Modern Web & 3D Visualization
- **p5.js Topology Canvas**: Node icons (MDI font/custom images), switching hub network port panels with LED indicators, bandwidth flow animation, and 11 distinct draw item types (KPI cards, sparklines, gauges, shapes, images).
- **3D Virtual Hardware Panel (`vpanel`)**: WebGL-accelerated 3D switch rendering with RJ45 port textures, Link UP/DOWN & 1Gbps+ speed LEDs, power LEDs, and interactive orbit controls.
- **Apache ECharts Analytics**: High-performance multi-axis time-series charts, IPAM utilization heatmaps, response time distribution, and traffic rate graphs.

### 2. Columnar Big Data Storage (Apache Parquet)
- High-speed compressed storage for **Syslog, SNMP TRAP, NetFlow v5/v9/IPFIX, sFlow v5, ARP Watch, OpenTelemetry, MQTT**, and polling logs.
- Lightning-fast multi-criteria filtering, pagination, and automated retention/rotation policies.

### 3. Native AI & Built-in MCP Server
- **Streamable HTTP MCP Server (`/api/mcp`)**: Native Model Context Protocol server exposing 22+ diagnostic/management tools and 19 prompt templates to AI agents (Cursor, Claude Desktop, Antigravity, etc.).
- **Multi-LLM Engine (`tensai`)**: Seamlessly connects to Gemini, OpenAI, Claude, and local Ollama models.
- **In-App AI Assistance**: One-click AI root cause diagnosis for event logs, natural language polling creation, and node health assessments.

### 4. Comprehensive Protocol Receivers & Built-in Services
- **Syslog**: UDP, TCP, TLS (RFC 3164 / RFC 5424)
- **SNMP TRAP**: v1, v2c, v3 with MIB name resolution
- **Flow Monitoring**: NetFlow v5/v9/IPFIX, sFlow v5 (Flow & Counter samples), Fumble anomaly detection
- **ARP Watch**: Continuous IP-to-MAC change and conflict detection
- **OpenTelemetry**: OTel OTLP gRPC/HTTP log & trace collector
- **MQTT Broker & Client**: Embedded broker for IoT sensors and metric polling
- **Built-in PKI Certificate Authority**: Root CA management, web-based CSR creation, certificate issuance, and CRL / OCSP / SCEP / ACME servers
- **Diagnostic Tools**: Interactive Ping (ICMP/UDP), Smokeping, MTR with AI analysis, MIB Browser / Tree, gNMI, and Wake-on-LAN (WOL)

---

## Quick Start

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

Open your browser and navigate to `http://localhost:8080`.

### Running with Docker Compose

Create a `docker-compose.yml` file:

```yaml
services:
  twsnmpneo:
    image: ghcr.io/twsnmp/twsnmpneo:latest
    container_name: twsnmpneo
    restart: always
    ports:
      - "8080:8080"
      - "514:514/udp"
      - "162:162/udp"
      - "2055:2055/udp"
      - "6343:6343/udp"
    volumes:
      - ./data:/data
```

Start the container:

```bash
docker compose up -d
```

### Running as a Standalone Binary

Download the executable for your platform from [GitHub Releases](https://github.com/twsnmp/twsnmpneo/releases).

#### Linux Special Note:
In Linux, privileged operations (raw socket ICMP ping, binding to ports below 1024 such as Syslog 514 or SNMP TRAP 162) require Linux Capabilities:

```bash
sudo setcap 'cap_net_bind_service,cap_net_raw+ep' ./twsnmpneo
./twsnmpneo -datastore ./data -port 8080
```

---

## Build from Source & Development

### Prerequisites
* **Go 1.27+**
* **Node.js 22+ & pnpm**
* **mise** (development tool & task manager)
* **air** (for backend live reload, managed automatically by mise)

### Building with mise

TWSNMP NEO uses [mise](https://mise.jdx.dev/) for streamlined builds and tasks:

```bash
# Install configured tool versions
mise install

# Build release binary (builds frontend assets & bundles into Go binary)
mise run build
# Output binary: bin/twsnmpneo
```

---

## Development & Debug Mode

### 1. Build Debug Binary
```bash
mise run build-debug
```

### 2. Run Backend with Live Reload (air)
Automatically watches Go files and restarts on changes:
```bash
mise run run-debug
```

### 3. Run Debug Binary Directly
```bash
mise run run-direct
# (Executes ./bin/twsnmpneo --datadir ./data --debug)
```

### 4. Frontend Development Server (Vite HMR)
Run the Vite development server with Hot Module Replacement:
```bash
mise run dev-frontend
# Starts at http://localhost:5173
```

---

## License

TWSNMP NEO is licensed under the [Apache License 2.0](LICENSE).
