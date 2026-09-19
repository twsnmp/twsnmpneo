# TWSNMP NEO (twsnmpneo) Development Specification (SPEC.md)

This specification defines the architectural design, implementation requirements, and operational workflows for **TWSNMP NEO**, the successor edition to the containerized network manager "TWSNMP FC".
Google Antigravity 2.0 must treat this document as the Single Source of Truth (SSOT) to autonomously drive implementation, testing, and deployment configurations.

---

## 1. Project Overview & Objectives

* **Project Name**: TWSNMP NEO
* **Container / Repository Name**: `twsnmpneo`
* **Core Objectives**:
  1. **Complete Tech Stack Modernization**: Overhaul the 6-year-old architecture and outdated dependencies of TWSNMP FC to ensure long-term maintainability.
  2. **Inheritance of TWSNMP FK Assets**: Port and optimize proven Go monitoring engines, built-in servers, Apache Parquet storage, and AI integrations from the desktop edition (TWSNMP FK) to a server/container platform.
  3. **Visual & Rendering Compatibility (Maps, Panels, and Charts)**:
     * Preserve **p5.js** canvas rendering sketches to guarantee 100% layout and operational compatibility with TWSNMP FC/FK maps and device panels.
     * Retain **Apache ECharts** for metric trends, logs, and traffic graphs.
     * Ensure total compatibility for built-in and custom device icon specifications.
  4. **Seamless Data Migration**: Provide an import pipeline allowing existing TWSNMP FC users to import databases, configurations (nodes, maps, polling), and historical logs.
  5. **Native AI & MCP (Model Context Protocol) Integration**: Combine multi-LLM orchestration via the `tensai` package with a native built-in MCP server, enabling autonomous operations and external agent queries.
  6. **Test-Driven Refactoring**: Execute rigorous unit and mock tests on all ported logic to identify regressions and edge cases during refactoring.

---

## 2. Tech Stack & Environment

### 2.1 Backend (Go)
* **Language Version**: Go Latest Stable (1.23+)
* **Data Storage**:
  * **bbolt**: System configuration, network maps, node/line topologies, polling configurations, credentials, and event logs.
  * **Apache Parquet**: Syslog, SNMP TRAP, NetFlow/IPFIX, and polling metric time-series logs (columnar compression, fast filtering, safe disk rotation).
* **AI & Agent Integrations**:
  * `tensai` (Go package): Unified abstraction across multiple LLM providers (Gemini, OpenAI, Claude, Ollama).
  * **Built-in MCP Server**: Expose internal network topology and monitoring tools via JSON-RPC / SSE / Stdio.
* **Built-in Protocols / Servers**:
  * MQTT broker, OpenTelemetry (OTel) receiver, Private PKI (autonomous TLS/client certificate management).
  * Syslog (UDP/TCP/TLS), SNMP TRAP (v1/v2c/v3), NetFlow/IPFIX, and ARP Watch.

### 2.2 Frontend (SPA)
* **Language / Framework**: Svelte 5 (Runes-based) + Vite + TypeScript
* **Map & Device Panel Rendering**: **p5.js** (direct port of canvas drawing algorithms from FC/FK)
* **Charts & Visualizations**: **Apache ECharts** (polling response times, resource utilization, traffic metrics, log histograms)
* **Styling & UI Components**: Tailwind CSS + `shadcn-svelte` (Bits UI) + Lucide Icons
* **Distribution**: Bundled into the final Go binary via `embed.FS` (single self-contained binary).

### 2.3 Development & CI/CD Toolchain
* **Local Toolchain & Task Runner**: `mise` (`mise.toml`)
* **Container Runtime**: Docker / Podman (Linux amd64, arm64)
* **CI/CD & Releases**: GitHub Actions
  * Binary Distribution: Compressed release packages for Linux, macOS, and Windows.
  * Container Image Distribution: GitHub Packages (GHCR: `ghcr.io/<owner>/twsnmpneo`).

---

## 3. Directory Layout

```text
twsnmpneo/
├── .github/
│   └── workflows/
│       ├── test.yml              # CI: Unit tests, linting, and type checking
│       └── release.yml           # CD: Cross-compilation binaries & GHCR push
├── backend/
│   ├── cmd/
│   │   └── twsnmpneo/
│   │       └── main.go           # Application entrypoint
│   ├── internal/
│   │   ├── ai/                   # tensai wrapper & MCP server implementation
│   │   ├── api/                  # REST, WebSocket, and SSE endpoints
│   │   ├── datastore/
│   │   │   ├── bbolt/            # Configuration, node metadata, and event DB
│   │   │   └── parquet/          # High-performance log store & rotation
│   │   ├── importer/             # Legacy TWSNMP FC database migration logic
│   │   ├── pki/                  # Private PKI, CA, and certificate management
│   │   ├── polling/              # Monitoring engine (Ping, SNMP, HTTP, Script, etc.)
│   │   ├── receiver/             # Syslog, TRAP, NetFlow, OTel, and MQTT servers
│   │   └── notify/               # Webhooks, Slack, LINE, and email notifications
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/       # shadcn-svelte UI components
│   │   │   ├── map/              # p5.js network map & hardware panel sketches
│   │   │   ├── charts/           # Apache ECharts wrapper components
│   │   │   ├── mcp/              # MCP & AI assistant chat interface
│   │   │   └── stores/           # Svelte 5 Runes ($state, $derived) stores
│   │   ├── static/
│   │   │   └── icons/            # Backward-compatible device icon sets
│   │   ├── App.svelte
│   │   └── main.ts
│   ├── package.json
│   ├── vite.config.ts
│   └── svelte.config.js
├── Dockerfile                    # Multi-stage build (Node build -> Go build -> Minimal image)
├── docker-compose.yml
├── mise.toml                     # Local tooling and task runner definitions
├── SPEC.md                       # This specification file
└── README.md
```

---

## 4. Functional Requirements

### 4.1 Topology Map, Panels & Graphics (p5.js & ECharts)
* **p5.js Map & Panel Rendering**:
  * Port existing `p5.js` canvas routines from FC/FK directly into Svelte 5 wrappers (`frontend/src/lib/map/`).
  * Maintain exact behavior for node positioning, connecting lines, traffic flow animations, alert blink indicators, zoom/pan navigation, and background images.
  * Replicate rack and hardware panel views with identical port-status rendering.
* **Device Icon Compatibility**:
  * Retain default built-in icon sets (routers, switches, servers, workstations, cloud endpoints, sensors) in `frontend/src/static/icons/`.
  * Support user-uploaded custom icons, maintaining legacy icon ID mappings.
* **Apache ECharts Visualization**:
  * Deliver charts for response times, CPU/Memory telemetry, interface traffic rates, and log density histograms.
  * Support synchronized tooltips, data zooming, and automatic dark/light theme switching.

### 4.2 Data Migration (TWSNMP FC Importer)
* **Input**: Legacy TWSNMP FC bbolt database backups or exported archive bundles.
* **Pipeline**:
  1. Validate and parse the legacy schema.
  2. Map legacy nodes, lines, map layout coordinates, icon identifiers, polling tasks, and user profiles into TWSNMP NEO schemas.
  3. Optionally import historical event logs and operational logs.
  4. Generate a detailed migration audit summary (imported records, skipped entries, warnings).

### 4.3 Log Management (Apache Parquet)
* Buffer Syslog, SNMP TRAP, NetFlow, and polling metrics in memory before writing to disk as Parquet files.
* Provide fast querying and filtering APIs powered by columnar scans across time windows, source IPs, severity tags, and message patterns.
* Enforce automated retention schedules (file-level deletion/archival) based on age and disk quota.

### 4.4 AI & MCP (Model Context Protocol)
* **Multi-LLM via `tensai`**:
  * Manage API credentials and endpoints for Gemini, OpenAI, Claude, and local Ollama instances.
  * Deliver contextual troubleshooting, root-cause inference, remediation suggestions, and regex/script generation helpers.
* **Integrated MCP Server**:
  * Expose tools over SSE (HTTP) and Stdio transports.
  * Standard MCP Tools:
    * `get_system_status`: Retrieve system health metrics and daemon summary.
    * `list_nodes`: List managed nodes and overall status.
    * `get_node_detail`: Retrieve node properties, MIB trees, and active polling tasks.
    * `get_active_alerts`: Query unresolved anomalies and current alerts.
    * `query_logs`: Execute structured filter queries against the Parquet data lake.

### 4.5 Built-in Servers & Protocols
* Direct port and refinement from TWSNMP FK:
  * **Syslog / TRAP / NetFlow**: High-throughput packet ingestion, decoding, alert triggers, and Parquet persistence.
  * **OpenTelemetry Receiver**: OTel trace and metric ingestion.
  * **MQTT Broker**: Ingest IoT telemetry and execute rule-based evaluation.
  * **Private PKI**: Automatic generation and rotation of TLS and mutual-auth certificates.

---

## 5. Testing & Code Migration Policy

All modules ported from TWSNMP FK must follow a test-first workflow:

1. **Test-First Porting**:
   * Author comprehensive unit tests (covering happy path, error handling, and boundary conditions) before finalizing ported Go code.
2. **Refactoring & Optimization**:
   * Refactor routines to remove legacy dependencies, enforce standardized error wrapping, and prevent goroutine/channel leaks.
3. **Coverage Standard**:
   * Achieve at least 80% branch coverage (C1) across critical packages (`internal/polling`, `internal/datastore`, `internal/importer`).
4. **Mocking & Isolation**:
   * Abstract external dependencies (SNMP agents, Syslog sources, LLM endpoints) behind Go interfaces to enable self-contained automated testing.

---

## 6. Development Workflow & `mise` Configuration (`mise.toml`)

Project dependencies and development tasks are managed via `mise`:

```toml
[tools]
go = "1.23"
node = "22"
pnpm = "latest"

[tasks.dev-frontend]
description = "Launch frontend development server with HMR (Vite)"
dir = "frontend"
run = "pnpm dev"

[tasks.build-frontend]
description = "Build static frontend assets for Go embed packaging"
dir = "frontend"
run = "pnpm build"

[tasks.test-backend]
description = "Run backend unit tests with race detection"
dir = "backend"
run = "go test -race -v ./..."

[tasks.build-debug]
description = "Compile debug binary bundling frontend assets"
depends = ["build-frontend"]
dir = "backend"
run = "go build -tags dev -o ../bin/twsnmpneo ./cmd/twsnmpneo"

[tasks.run-debug]
description = "Execute debug binary with local data directory"
depends = ["build-debug"]
run = "./bin/twsnmpneo --datadir ./data --debug"
```

---

## 7. CI/CD & Release Workflow (GitHub Actions)

### 7.1 CI Pipeline (`test.yml`)
* Trigger on Pull Requests and commits to `main`.
* Set up tools via `jdx/mise-action`.
* Execute frontend type checks, linting, and build verification.
* Run backend tests via `go test -race -cover ./...` alongside `golangci-lint`.

### 7.2 Release Pipeline (`release.yml`)
* Trigger on Git tag pushes matching `v*.*.*`.
* **Multi-Platform Binary Artifacts**:
  * Targets: Linux (amd64, arm64), macOS (darwin/amd64, darwin/arm64), Windows (amd64).
  * Package binaries into `.tar.gz` (`.zip` for Windows) and publish as GitHub Release assets.
* **Container Image Delivery**:
  * Build multi-architecture images (`linux/amd64`, `linux/arm64`) with `docker/build-push-action`.
  * Publish images to GitHub Container Registry (`ghcr.io/${{ github.repository_owner }}/twsnmpneo`).
  * Tag with `latest`, `vX.Y.Z`, and `vX.Y`.

---

## 8. Implementation Steps for Google Antigravity 2.0

1. **Step 1**: Scaffold the repository with `mise.toml`, set up the basic Svelte 5 frontend (Tailwind + shadcn-svelte + p5.js + Apache ECharts), and scaffold the Go application structure.
2. **Step 2**: Implement `internal/datastore` (bbolt and Parquet) and `internal/importer` (FC migration engine) backed by comprehensive unit test coverage.
3. **Step 3**: Port monitoring pollers, internal protocol servers (Syslog, TRAP, NetFlow, OTel, MQTT), and the private PKI package from TWSNMP FK while writing rigorous unit tests.
4. **Step 4**: Implement `tensai` LLM orchestration and configure the MCP server tools and transport layers.
5. **Step 5**: Build the management Web UI (p5.js topology map, ECharts metric views, alert grids, and AI assistant panel) and embed static outputs into the Go executable.
6. **Step 6**: Finalize the multi-stage Dockerfile and GitHub Actions pipelines (`test.yml`, `release.yml`), confirming readiness via `mise run test-backend` and `mise run run-debug`.
