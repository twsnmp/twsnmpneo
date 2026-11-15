import { spawn, type ChildProcess } from "node:child_process";
import dgram from "node:dgram";
import fs from "node:fs";
import net from "node:net";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const E2E_DIR = path.resolve(__dirname, "..");
export const REPO_ROOT = path.resolve(E2E_DIR, "..");

export interface ServerPorts {
  web: number;
  syslogUdp: number;
  syslogTcp: number;
  trap: number;
  netflow: number;
  sflow: number;
  otel: number;
  mqtt: number;
}

export interface TestServerHandle {
  baseURL: string;
  ports: ServerPorts;
  dataDir: string;
  isExternal: boolean;
  stop: () => Promise<void>;
}

async function getFreeTcpPort(): Promise<number> {
  return await new Promise((resolve, reject) => {
    const server = net.createServer();
    server.unref();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address();
      if (!addr || typeof addr === "string") {
        server.close();
        reject(new Error("Failed to allocate TCP port"));
        return;
      }
      const port = addr.port;
      server.close((err) => (err ? reject(err) : resolve(port)));
    });
  });
}

async function getFreeUdpPort(): Promise<number> {
  return await new Promise((resolve, reject) => {
    const socket = dgram.createSocket("udp4");
    socket.on("error", reject);
    socket.bind(0, "127.0.0.1", () => {
      const addr = socket.address();
      const port = addr.port;
      socket.close(() => resolve(port));
    });
  });
}

export function resolveBinaryPath(): string {
  const candidates = [
    process.env.TWSNMPNEO_BIN,
    path.join(REPO_ROOT, "bin", "twsnmpneo"),
    path.join(REPO_ROOT, "backend", "twsnmpneo"),
  ].filter((p): p is string => Boolean(p));

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }
  throw new Error(
    `twsnmpneo binary not found. Checked: ${candidates.join(", ")}. Run "mise run build" first.`
  );
}

async function waitForHealth(baseURL: string, timeoutMs: number, getLogs: () => string): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${baseURL}/api/health`);
      if (res.ok) {
        return;
      }
      lastError = new Error(`HTTP ${res.status}`);
    } catch (err) {
      lastError = err;
    }
    await new Promise((r) => setTimeout(r, 150));
  }
  throw new Error(
    `Timed out waiting for ${baseURL}/api/health (${String(lastError)}).\nServer logs:\n${getLogs()}`
  );
}

export async function startTestServer(options?: {
  lang?: "ja" | "en";
  webPort?: number;
}): Promise<TestServerHandle> {
  const externalURL = process.env.E2E_BASE_URL?.trim();
  if (externalURL) {
    await waitForHealth(externalURL, 15_000, () => "(external server)");
    return {
      baseURL: externalURL.replace(/\/$/, ""),
      ports: {
        web: Number(new URL(externalURL).port || 8080),
        syslogUdp: Number(process.env.E2E_SYSLOG_UDP_PORT || 514),
        syslogTcp: Number(process.env.E2E_SYSLOG_TCP_PORT || 514),
        trap: Number(process.env.E2E_TRAP_PORT || 162),
        netflow: Number(process.env.E2E_NETFLOW_PORT || 2055),
        sflow: Number(process.env.E2E_SFLOW_PORT || 6343),
        otel: Number(process.env.E2E_OTEL_PORT || 4318),
        mqtt: Number(process.env.E2E_MQTT_PORT || 1883),
      },
      dataDir: "",
      isExternal: true,
      stop: async () => {},
    };
  }

  const binPath = resolveBinaryPath();
  const dataDir = fs.mkdtempSync(path.join(E2E_DIR, ".tmp-data-"));

  const web = options?.webPort || (await getFreeTcpPort());
  const ports: ServerPorts = {
    web,
    syslogUdp: await getFreeUdpPort(),
    syslogTcp: await getFreeTcpPort(),
    trap: await getFreeUdpPort(),
    netflow: await getFreeUdpPort(),
    sflow: await getFreeUdpPort(),
    otel: await getFreeTcpPort(),
    mqtt: await getFreeTcpPort(),
  };

  const args = [
    "-datadir",
    dataDir,
    "-port",
    String(ports.web),
    "-lang",
    options?.lang ?? "ja",
    "-http-port",
    "0",
    "-syslog-udp",
    String(ports.syslogUdp),
    "-syslog-tcp",
    String(ports.syslogTcp),
    "-trap-port",
    String(ports.trap),
    "-netflow-port",
    String(ports.netflow),
    "-sflow-port",
    String(ports.sflow),
    "-otel-port",
    String(ports.otel),
    "-mqtt-port",
    String(ports.mqtt),
    "-mcpMode",
    "noauth",
    ...(process.env.E2E_DEBUG ? ["-debug"] : []),
  ];

  const logChunks: string[] = [];
  let child: ChildProcess | null = spawn(binPath, args, {
    cwd: REPO_ROOT,
    stdio: ["ignore", "pipe", "pipe"],
  });

  child.stdout?.on("data", (d) => logChunks.push(String(d)));
  child.stderr?.on("data", (d) => logChunks.push(String(d)));

  let exitedEarly = false;
  child.on("exit", (code, signal) => {
    exitedEarly = true;
    logChunks.push(`\n[process exited code=${code} signal=${signal}]\n`);
  });

  const baseURL = `http://127.0.0.1:${ports.web}`;

  const stop = async () => {
    const proc = child;
    child = null;
    if (proc && !exitedEarly) {
      await new Promise<void>((resolve) => {
        const killTimer = setTimeout(() => {
          proc.kill("SIGKILL");
        }, 5000);
        proc.once("exit", () => {
          clearTimeout(killTimer);
          resolve();
        });
        proc.kill("SIGTERM");
      });
    }
    if (!process.env.E2E_KEEP_DATA) {
      fs.rmSync(dataDir, { recursive: true, force: true });
    }
  };

  try {
    await waitForHealth(baseURL, 25_000, () => logChunks.join(""));
  } catch (err) {
    await stop();
    throw err;
  }

  return {
    baseURL,
    ports,
    dataDir,
    isExternal: false,
    stop,
  };
}
