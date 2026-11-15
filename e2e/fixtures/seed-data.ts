import dgram from "node:dgram";
import crypto from "node:crypto";
import type { ServerPorts } from "./test-server.js";

export interface SeededData {
  token: string;
  publicKey: string;
  viewerCredentials: {
    user: string;
    password: string;
  };
  nodes: {
    coreGw: Record<string, any>;
    appSrv: Record<string, any>;
    dbSrv: Record<string, any>;
    edgeFw: Record<string, any>;
    cloudApi: Record<string, any>;
  };
  network: Record<string, any>;
  pollings: {
    corePing: Record<string, any>;
    appHttp: Record<string, any>;
  };
}

async function apiReq<T = any>(
  baseURL: string,
  token: string,
  method: string,
  endpoint: string,
  body?: unknown
): Promise<T> {
  const headers: Record<string, string> = {
    Authorization: `Bearer ${token}`,
  };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(`${baseURL}${endpoint}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`${method} ${endpoint} failed (${res.status}): ${text}`);
  }
  return text ? (JSON.parse(text) as T) : ({} as T);
}

export async function loginAPI(
  baseURL: string,
  user = "twsnmp",
  password = "twsnmp"
): Promise<string> {
  const res = await fetch(`${baseURL}/api/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ user, password }),
  });
  if (!res.ok) {
    throw new Error(`Login failed for ${user}: ${res.status} ${await res.text()}`);
  }
  const data = (await res.json()) as { token?: string };
  if (!data.token) {
    throw new Error("Login succeeded but no token returned");
  }
  return data.token;
}

async function sendUdpSyslog(port: number, messages: string[]): Promise<void> {
  if (!port || port <= 0) return;
  const client = dgram.createSocket("udp4");
  try {
    for (const msg of messages) {
      const buf = Buffer.from(msg, "utf8");
      await new Promise<void>((resolve, reject) => {
        client.send(buf, port, "127.0.0.1", (err) => (err ? reject(err) : resolve()));
      });
    }
  } finally {
    client.close();
  }
}

async function sendOTelDemoData(otelPort: number): Promise<void> {
  if (!otelPort || otelPort <= 0) return;
  const nowNano = String(BigInt(Date.now()) * 1_000_000n);
  const startNano = String((BigInt(Date.now()) - 15_000n) * 1_000_000n);
  const traceId = crypto.randomBytes(16).toString("base64");
  const rootSpanId = crypto.randomBytes(8).toString("base64");
  const childSpanId = crypto.randomBytes(8).toString("base64");

  const metricsPayload = {
    resourceMetrics: [
      {
        resource: {
          attributes: [
            { key: "service.name", value: { stringValue: "e2e-checkout-service" } },
            { key: "host.name", value: { stringValue: "app-srv-01" } },
          ],
        },
        scopeMetrics: [
          {
            scope: { name: "twsnmpneo-e2e", version: "1.0.0" },
            metrics: [
              {
                name: "http.server.duration",
                unit: "ms",
                gauge: {
                  dataPoints: [
                    {
                      asDouble: 42.5,
                      timeUnixNano: nowNano,
                      attributes: [{ key: "http.method", value: { stringValue: "GET" } }],
                    },
                  ],
                },
              },
              {
                name: "system.cpu.utilization",
                unit: "1",
                gauge: {
                  dataPoints: [
                    {
                      asDouble: 0.34,
                      timeUnixNano: nowNano,
                    },
                  ],
                },
              },
            ],
          },
        ],
      },
    ],
  };

  const tracesPayload = {
    resourceSpans: [
      {
        resource: {
          attributes: [
            { key: "service.name", value: { stringValue: "e2e-checkout-service" } },
          ],
        },
        scopeSpans: [
          {
            scope: { name: "twsnmpneo-e2e-tracer" },
            spans: [
              {
                traceId,
                spanId: rootSpanId,
                name: "POST /api/v1/orders",
                kind: 2,
                startTimeUnixNano: startNano,
                endTimeUnixNano: nowNano,
                status: { code: 1 },
              },
              {
                traceId,
                spanId: childSpanId,
                parentSpanId: rootSpanId,
                name: "SELECT orders",
                kind: 3,
                startTimeUnixNano: startNano,
                endTimeUnixNano: nowNano,
                status: { code: 1 },
              },
            ],
          },
        ],
      },
    ],
  };

  const logsPayload = {
    resourceLogs: [
      {
        resource: {
          attributes: [
            { key: "service.name", value: { stringValue: "e2e-checkout-service" } },
          ],
        },
        scopeLogs: [
          {
            scope: { name: "twsnmpneo-e2e-logger" },
            logRecords: [
              {
                timeUnixNano: nowNano,
                severityNumber: 9,
                severityText: "INFO",
                body: { stringValue: "Order processed successfully in E2E seed" },
                traceId,
                spanId: rootSpanId,
              },
            ],
          },
        ],
      },
    ],
  };

  const baseOtel = `http://127.0.0.1:${otelPort}`;
  // Retry briefly because OTel HTTP listener binds asynchronously right after startup.
  for (let attempt = 0; attempt < 15; attempt++) {
    try {
      const mRes = await fetch(`${baseOtel}/v1/metrics`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(metricsPayload),
      });
      if (mRes.ok) {
        await fetch(`${baseOtel}/v1/traces`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(tracesPayload),
        });
        await fetch(`${baseOtel}/v1/logs`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(logsPayload),
        });
        return;
      }
    } catch {
      // Wait for listener bind
    }
    await new Promise((r) => setTimeout(r, 150));
  }
}

export async function seedTestData(
  baseURL: string,
  ports: Pick<ServerPorts, "syslogUdp" | "otel">
): Promise<SeededData> {
  const token = await loginAPI(baseURL, "twsnmp", "twsnmp");
  const publicKey = "e2e-demo-key";
  const viewerCredentials = {
    user: "e2e_viewer",
    password: "ViewerPass123!",
  };

  // Ensure readonly user exists for RBAC tests
  try {
    const users = await apiReq<any[]>(baseURL, token, "GET", "/api/users");
    const hasViewer = Array.isArray(users) && users.some((u) => u.user === viewerCredentials.user);
    if (!hasViewer) {
      await apiReq(baseURL, token, "POST", "/api/users", {
        user: viewerCredentials.user,
        name: "E2E Readonly Viewer",
        password: viewerCredentials.password,
        role: "readonly",
      });
    }
  } catch {
    // Best effort if user creation differs on external servers
  }

  // Enable receivers and Public Dashboard
  const currentConf = await apiReq<Record<string, any>>(baseURL, token, "GET", "/api/map/conf");
  await apiReq(baseURL, token, "POST", "/api/map/conf", {
    ...currentConf,
    MapName: "TWSNMP NEO E2E Lab",
    EnableSyslogd: true,
    EnableOTel: true,
    EnableMqtt: true,
    EnablePublicDashboard: true,
    PublicDashboardKey: publicKey,
    PublicDashboardAlert: {
      mode: "state",
      level: "warn",
      sound: false,
    },
  });

  // Check if nodes already seeded (idempotent when running against E2E_BASE_URL)
  const existingNodes = await apiReq<any[]>(baseURL, token, "GET", "/api/nodes");
  const byName = new Map<string, any>();
  for (const n of existingNodes || []) {
    byName.set(n.name || n.Name, n);
  }

  const ensureNode = async (payload: Record<string, any>) => {
    const found = byName.get(payload.Name);
    if (found) return found;
    return await apiReq(baseURL, token, "POST", "/api/nodes", payload);
  };

  const coreGw = await ensureNode({
    Name: "Core Gateway",
    IP: "192.168.1.1",
    MAC: "00:11:22:33:44:01",
    Icon: "router",
    State: "normal",
    X: 340,
    Y: 160,
    SnmpMode: "v2c",
    Community: "public",
    Loc: "139.7671,35.6812",
    Descr: "Core Router - Tokyo DC",
    AddrMode: "ip",
  });

  const appSrv = await ensureNode({
    Name: "App Server 01",
    IP: "192.168.1.10",
    MAC: "00:11:22:33:44:10",
    Icon: "server",
    State: "normal",
    X: 580,
    Y: 160,
    SnmpMode: "v2c",
    Community: "public",
    Loc: "139.7016,35.6595",
    Descr: "Primary Application Node",
    AddrMode: "ip",
  });

  const dbSrv = await ensureNode({
    Name: "Database 01",
    IP: "192.168.1.20",
    MAC: "00:11:22:33:44:20",
    Icon: "db",
    State: "warn",
    X: 580,
    Y: 360,
    Loc: "139.7745,35.7141",
    Descr: "Time-Series & Analytics DB",
    AddrMode: "ip",
  });

  const edgeFw = await ensureNode({
    Name: "Edge Firewall",
    IP: "192.168.1.254",
    MAC: "00:11:22:33:44:fe",
    Icon: "firewall",
    State: "high",
    X: 120,
    Y: 260,
    Loc: "139.6917,35.6895",
    Descr: "Perimeter Security Appliance",
    AddrMode: "ip",
  });

  const cloudApi = await ensureNode({
    Name: "Cloud API",
    IP: "10.20.0.10",
    MAC: "00:11:22:33:44:99",
    Icon: "cloud",
    State: "normal",
    X: 800,
    Y: 240,
    Loc: "139.7454,35.6586",
    Descr: "External API Endpoint",
    AddrMode: "ip",
  });

  const existingNets = await apiReq<any[]>(baseURL, token, "GET", "/api/networks");
  let network = (existingNets || []).find((n) => (n.name || n.Name) === "Core L2 Switch");
  if (!network) {
    network = await apiReq(baseURL, token, "POST", "/api/networks", {
      Name: "Core L2 Switch",
      IP: "192.168.1.2",
      X: 290,
      Y: 340,
      W: 220,
      H: 60,
      Unmanaged: true,
      HPorts: 4,
      Ports: [
        { ID: "p1", Name: "Gi0/1", Index: "1", X: 0, Y: 0, State: "up" },
        { ID: "p2", Name: "Gi0/2", Index: "2", X: 1, Y: 0, State: "up" },
        { ID: "p3", Name: "Gi0/3", Index: "3", X: 2, Y: 0, State: "down" },
        { ID: "p4", Name: "Gi0/4", Index: "4", X: 3, Y: 0, State: "up" },
      ],
    });
  }

  const existingLines = await apiReq<any[]>(baseURL, token, "GET", "/api/lines");
  if (!existingLines || existingLines.length === 0) {
    const coreId = coreGw.id || coreGw.ID;
    const appId = appSrv.id || appSrv.ID;
    const dbId = dbSrv.id || dbSrv.ID;
    const fwId = edgeFw.id || edgeFw.ID;
    const cloudId = cloudApi.id || cloudApi.ID;
    const netId = `NET:${network.id || network.ID}`;

    const pairs: Array<[string, string, string, string]> = [
      [coreId, appId, "normal", "10Gbps Fiber"],
      [coreId, fwId, "high", "WAN Uplink"],
      [appId, dbId, "warn", "DB Sync Link"],
      [appId, cloudId, "normal", "TLS Trunk"],
      [coreId, netId, "normal", "Gi0/1 Trunk"],
    ];
    for (const [n1, n2, state, info] of pairs) {
      await apiReq(baseURL, token, "POST", "/api/lines", {
        NodeID1: n1,
        NodeID2: n2,
        State: state,
        Width: 2,
        Info: info,
      });
    }
  }

  const existingDrawItems = await apiReq<any[]>(baseURL, token, "GET", "/api/drawitems");
  if (!existingDrawItems || existingDrawItems.length === 0) {
    await apiReq(baseURL, token, "POST", "/api/drawitems", {
      Type: 2,
      X: 80,
      Y: 60,
      W: 260,
      H: 48,
      Text: "Tokyo Core Network",
      Color: "#38bdf8",
      Size: 18,
    });
  }

  const existingPollings = await apiReq<any[]>(baseURL, token, "GET", "/api/pollings");
  let corePing = (existingPollings || []).find((p) => (p.name || p.Name) === "Core Ping");
  if (!corePing) {
    corePing = await apiReq(baseURL, token, "POST", "/api/pollings", {
      NodeID: coreGw.id || coreGw.ID,
      Name: "Core Ping",
      Type: "ping",
      Mode: "",
      Level: "high",
      PollInt: 60,
      Timeout: 1,
      Retry: 1,
      LogMode: 1,
      State: "normal",
    });
  }

  let appHttp = (existingPollings || []).find((p) => (p.name || p.Name) === "App Health Check");
  if (!appHttp) {
    appHttp = await apiReq(baseURL, token, "POST", "/api/pollings", {
      NodeID: appSrv.id || appSrv.ID,
      Name: "App Health Check",
      Type: "http",
      Mode: "http",
      Params: `${baseURL}/api/health`,
      Level: "warn",
      PollInt: 60,
      Timeout: 2,
      Retry: 1,
      LogMode: 1,
      State: "normal",
    });
  }

  // Configure Public Dashboard layout with Map + Polling card
  const latestConf = await apiReq<Record<string, any>>(baseURL, token, "GET", "/api/map/conf");
  await apiReq(baseURL, token, "POST", "/api/map/conf", {
    ...latestConf,
    EnablePublicDashboard: true,
    PublicDashboardKey: publicKey,
    PublicDashboardItems: [
      {
        id: "pub-map",
        type: "map",
        title: "Core Network Map",
      },
      {
        id: "pub-ping",
        type: "polling",
        pollingId: corePing.id || corePing.ID,
        title: "Core Gateway Latency",
      },
    ],
  });

  // Ingest sample telemetry into Syslog & OpenTelemetry receivers
  await Promise.all([
    sendUdpSyslog(ports.syslogUdp, [
      "<134>Nov 14 07:00:01 core-gw kernel: Interface eth0 link up 10000Mb/s",
      "<131>Nov 14 07:00:02 edge-fw sshd[1024]: Failed password for invalid user admin from 203.0.113.50 port 54211 ssh2",
      "<132>Nov 14 07:00:03 db-srv-01 postgres[442]: High buffer pool usage detected (86%)",
    ]),
    sendOTelDemoData(ports.otel),
  ]);

  return {
    token,
    publicKey,
    viewerCredentials,
    nodes: {
      coreGw,
      appSrv,
      dbSrv,
      edgeFw,
      cloudApi,
    },
    network,
    pollings: {
      corePing,
      appHttp,
    },
  };
}
