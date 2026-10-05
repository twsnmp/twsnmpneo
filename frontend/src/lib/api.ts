import { get } from 'svelte/store';
import { locale } from 'svelte-i18n';

export interface NodeEnt {
  id: string;
  name: string;
  ip: string;
  mac?: string;
  icon?: string;
  image?: string;
  state: string; // normal, warn, low, high, error, unknown
  x: number;
  y: number;
  descr?: string;
  vendor?: string;
  snmp_mode?: string;
  community?: string;
  user?: string;
  password?: string;
  addr_mode?: string;
  snmp_port?: number;
  auto_ack?: boolean;
  url?: string;
  ssh_user?: string;
  public_key?: string;
  gnmi_port?: string;
  gnmi_encoding?: string;
  gnmi_user?: string;
  gnmi_password?: string;
  loc?: string;

  // Compatibility aliases matching Go backend
  ID?: string;
  Name?: string;
  IP?: string;
  MAC?: string;
  Vendor?: string;
  Icon?: string;
  Image?: string;
  State?: string;
  X?: number;
  Y?: number;
  Descr?: string;
  GNMIPort?: string;
  GNMIEncoding?: string;
  GNMIUser?: string;
  GNMIPassword?: string;
  SnmpPort?: number;
  AutoAck?: boolean;
  URL?: string;
  SSHUser?: string;
  PublicKey?: string;
  Loc?: string;
}

export interface LineEnt {
  id: string;
  node_id1: string;
  node_id2: string;
  state: string;
  width: number;
  polling_id1?: string;
  polling_id2?: string;
  polling_id?: string;
  info?: string;
  port?: string;
  state1?: string;
  state2?: string;

  // Compatibility aliases
  ID?: string;
  NodeID1?: string;
  NodeID2?: string;
  PollingID1?: string;
  PollingID2?: string;
  PollingID?: string;
  State?: string;
  State1?: string;
  State2?: string;
  Width?: number;
  Info?: string;
  Port?: string;
}

export interface NeighborLineEnt extends LineEnt {
  Confidence?: string; // "strict" | "speculative"
  Reason?: string;     // "LLDP" | "CDP" | "STP" | "FDB-Edge" | "Subnet Heuristic" | "AI"
}

export interface FindNeighborNetworksAndLinesResp {
  Networks: NetworkEnt[];
  Lines: NeighborLineEnt[];
}

export interface PollingEnt {
  id: string;
  node_id: string;
  name: string;
  type: string; // ping, http, tcp, dns, ntp, snmp
  target?: string;
  mode?: string;
  params?: string;
  filter?: string;
  extractor?: string;
  script?: string;
  level?: string;
  poll_int?: number;
  timeout?: number;
  retry?: number;
  log_mode?: number;
  fail_action?: string;
  repair_action?: string;
  ai_mode?: string;
  vector_cols?: string;
  mqtt_url?: string;
  mqtt_topic?: string;
  mqtt_cols?: string;
  state: string;
  next_time?: number;
  last_time?: number;
  fail_time?: number;
  last_val?: number;
  result?: Record<string, unknown>;

  // Compatibility aliases
  ID?: string;
  NodeID?: string;
  Name?: string;
  Type?: string;
  Target?: string;
  Mode?: string;
  Params?: string;
  Filter?: string;
  Extractor?: string;
  Script?: string;
  Level?: string;
  PollInt?: number;
  Timeout?: number;
  Retry?: number;
  LogMode?: number;
  State?: string;
  NextTime?: number;
  LastTime?: number;
  FailTime?: number;
  Result?: Record<string, unknown>;
  FailAction?: string;
  RepairAction?: string;
  AIMode?: string;
  VectorCols?: string;
  MqttURL?: string;
  MqttTopic?: string;
  MqttCols?: string;
}

export interface PortEnt {
  id: string;
  name: string;
  x: number;
  y: number;
  state: string;
  polling?: string;
  index?: string;

  ID?: string;
  Name?: string;
  X?: number;
  Y?: number;
  State?: string;
  Index?: string;
}

export interface NetworkEnt {
  id: string;
  name: string;
  ip: string;
  x: number;
  y: number;
  w: number;
  h: number;
  h_ports?: number;
  ports: PortEnt[];
  descr?: string;
  snmp_mode?: string;
  community?: string;
  user?: string;
  password?: string;
  snmp_port?: number;
  unmanaged?: boolean;
  error?: string;

  // Compatibility aliases
  ID?: string;
  Name?: string;
  IP?: string;
  X?: number;
  Y?: number;
  W?: number;
  H?: number;
  Ports?: PortEnt[];
  Descr?: string;
  SnmpMode?: string;
  Community?: string;
  User?: string;
  Password?: string;
  SnmpPort?: number;
  Unmanaged?: boolean;
  Error?: string;
}

export interface DrawItemEnt {
  id: string;
  type: number;
  x: number;
  y: number;
  w: number;
  h: number;
  text?: string;
  color?: string;
  size?: number;
  node_id?: string;
  polling_id?: string;
  path?: string;
  var_name?: string;
  format?: string;
  value?: number;
  scale?: number;
  values?: number[];
  cond?: number;
  formatted_text?: string;

  ID?: string;
  Type?: number;
  X?: number;
  Y?: number;
  W?: number;
  H?: number;
  Text?: string;
  Color?: string;
  Size?: number;
  NodeID?: string;
  PollingID?: string;
  Path?: string;
  VarName?: string;
  Format?: string;
  Value?: number;
  Scale?: number;
  Values?: number[];
  Cond?: number;
  FormattedText?: string;
}

export interface EventLogEnt {
  time: number;
  type: string;
  level: string; // info, warn, low, high, error
  node_id?: string;
  node_name?: string;
  event: string;

  Time?: number;
  Type?: string;
  Level?: string;
  NodeID?: string;
  NodeName?: string;
  Event?: string;
}

export interface ParquetLogRecord {
  time: number;
  type: string;
  src: string;
  log: string;

  Time?: number;
  Type?: string;
  Src?: string;
  Log?: string;
}

export interface SystemHealth {
  status: string;
  time: string;
  version: string;
}

export interface MonitorDataEnt {
  Time: number;
  CPU: number;
  Mem: number;
  MyCPU: number;
  MyMem: number;
  Swap: number;
  Disk: number;
  Load: number;
  Bytes: number;
  Net: number;
  Conn: number;
  Proc: number;
  DBSize: number;
  HeapAlloc: number;
  Sys: number;
  NumGoroutine: number;
}

export interface ReceiverStatusInfo {
  port?: string;
  status?: string;
  range?: string;
}

export interface SystemInfo {
  version: string;
  commit?: string;
  status: string;
  time: string;
  uptime: string;
  num_cpu?: number;
  node_count: number;
  poll_count: number;
  ping_mode?: string;
  receivers?: Record<string, ReceiverStatusInfo>;
}

const API_BASE = '/api';

export interface PKIStatus {
  ready: boolean;
  commonName?: string;
  expiresAt?: number;
  certificate?: string;
}

export interface PKICAOptions {
  commonName: string;
  organization: string;
  sans: string[];
  keyType: string;
  validYears: number;
  acmeBaseURL: string;
  httpBaseURL: string;
  crlIntervalHours: number;
  certValidityHours: number;
  httpPort: number;
  acmePort: number;
}

export interface PKISettings extends PKICAOptions {
  enableHTTP: boolean;
  enableACME: boolean;
}

export interface PKICertificate {
  serial: string;
  subject: string;
  type: string;
  certPEM: string;
  createdAt: number;
  expiresAt: number;
  revokedAt?: number;
}

export interface PKICAOptions {
  commonName: string;
  organization: string;
  keyType: string;
  validYears: number;
}

async function pkiRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const endpoint = `${API_BASE}/pki${path}`;
  const response = await fetch(endpoint, init);
  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = await response.json();
      if (typeof body.error === 'string') message = body.error;
    } catch {
      // Keep the HTTP status text when the server does not return JSON.
    }
    throw new Error(`${endpoint}: ${message || `request failed with HTTP ${response.status}`}`);
  }
  if (response.status === 204) return undefined as T;
  if (!response.headers.get('content-type')?.includes('application/json')) {
    throw new Error(`${endpoint} returned a non-JSON response (HTTP ${response.status}); the server may need to be updated and restarted.`);
  }
  try {
    return await response.json();
  } catch (cause) {
    throw new Error(`${endpoint} returned invalid JSON (HTTP ${response.status}).`, { cause });
  }
}

export function fetchPKIStatus(): Promise<PKIStatus> {
  return pkiRequest<PKIStatus>('/status');
}

export function fetchPKISettings(): Promise<PKISettings> {
  return pkiRequest<PKISettings>('/settings');
}

export function fetchPKICertificates(): Promise<PKICertificate[]> {
  return pkiRequest<PKICertificate[]>('/certificates');
}

export function initializePKICA(options: PKICAOptions): Promise<PKIStatus> {
  return pkiRequest<PKIStatus>('/ca', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(options),
  });
}

export function updatePKISettings(settings: PKISettings): Promise<PKISettings> {
  return pkiRequest<PKISettings>('/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  });
}

export function resetPKICA(): Promise<void> {
  return pkiRequest<void>('/ca', { method: 'DELETE' });
}

export function issuePKICertificateFromCSR(csrPEM: string): Promise<PKICertificate> {
  return pkiRequest<PKICertificate>('/certificates/csr', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ csrPEM }),
  });
}

export function createPKICertificate(request: {
  commonName: string;
  dnsNames: string[];
  ipAddresses: string[];
  validDays: number;
}): Promise<PKICertificate> {
  return pkiRequest<PKICertificate>('/certificates', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
}

export function revokePKICertificate(serial: string): Promise<void> {
  return pkiRequest<void>(`/certificates/${encodeURIComponent(serial)}`, { method: 'DELETE' });
}

export async function downloadPKICertificate(serial: string): Promise<void> {
  const response = await fetch(`${API_BASE}/pki/certificates/${encodeURIComponent(serial)}/download`);
  if (!response.ok) throw new Error(`Certificate download failed: ${response.statusText}`);
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `${serial}.pem`;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function normalizeNode(raw: any): NodeEnt {
  if (!raw) return { id: '', name: '', ip: '', state: 'unknown', x: 300, y: 250 };
  const id = raw.id || raw.ID || '';
  const name = raw.name || raw.Name || 'Node';
  const ip = raw.ip || raw.IP || '';
  const mac = raw.mac || raw.MAC || '';
  const vendor = raw.vendor || raw.Vendor || '';
  const icon = raw.icon || raw.Icon || 'desktop';
  const image = raw.image || raw.Image || '';
  const state = raw.state || raw.State || 'normal';
  const x = typeof raw.x === 'number' ? raw.x : (typeof raw.X === 'number' ? raw.X : 300);
  const y = typeof raw.y === 'number' ? raw.y : (typeof raw.Y === 'number' ? raw.Y : 250);
  const descr = raw.descr || raw.Descr || '';
  const snmp_mode = raw.snmp_mode || raw.SnmpMode || 'v2c';
  const community = raw.community || raw.Community || 'public';
  const user = raw.user || raw.User || '';
  const password = raw.password || raw.Password || '';
  const addr_mode = raw.addr_mode || raw.AddrMode || 'ip';
  const snmp_port = raw.snmp_port ?? raw.SnmpPort ?? 161;
  const auto_ack = raw.auto_ack ?? raw.AutoAck ?? false;
  const url = raw.url || raw.URL || '';
  const ssh_user = raw.ssh_user || raw.SSHUser || '';
  const public_key = raw.public_key || raw.PublicKey || '';
  const gnmi_port = raw.gnmi_port || raw.GNMIPort || '';
  const gnmi_encoding = raw.gnmi_encoding || raw.GNMIEncoding || '';
  const gnmi_user = raw.gnmi_user || raw.GNMIUser || '';
  const gnmi_password = raw.gnmi_password || raw.GNMIPassword || '';

  return {
    ...raw,
    id, ID: id,
    name, Name: name,
    ip, IP: ip,
    mac, MAC: mac,
    vendor, Vendor: vendor,
    icon, Icon: icon,
    image, Image: image,
    state, State: state,
    x, X: x,
    y, Y: y,
    descr, Descr: descr,
    snmp_mode, SnmpMode: snmp_mode,
    community, Community: community,
    user, User: user,
    password, Password: password,
    addr_mode, AddrMode: addr_mode,
    snmp_port, SnmpPort: snmp_port,
    auto_ack, AutoAck: auto_ack,
    url, URL: url,
    ssh_user, SSHUser: ssh_user,
    public_key, PublicKey: public_key,
    gnmi_port, GNMIPort: gnmi_port,
    gnmi_encoding, GNMIEncoding: gnmi_encoding,
    gnmi_user, GNMIUser: gnmi_user,
    gnmi_password, GNMIPassword: gnmi_password,
  };
}

export function normalizeNetwork(raw: any): NetworkEnt {
  if (!raw) return { id: '', name: '', ip: '', x: 300, y: 150, w: 420, h: 90, ports: [] };
  const id = raw.id || raw.ID || '';
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const name = raw.name || raw.Name || (isJa ? 'ネットワーク' : 'Network');
  const ip = raw.ip || raw.IP || '';
  const x = typeof raw.x === 'number' ? raw.x : (typeof raw.X === 'number' ? raw.X : 300);
  const y = typeof raw.y === 'number' ? raw.y : (typeof raw.Y === 'number' ? raw.Y : 150);
  const rawPorts = raw.ports || raw.Ports || [];
  const hPorts = typeof raw.h_ports === 'number' && raw.h_ports > 0 ? raw.h_ports : 8;
  const ports: PortEnt[] = rawPorts.map((p: any, idx: number) => {
    const pid = p.id || p.ID || `p${idx + 1}`;
    const pname = p.name || p.Name || `Port ${idx + 1}`;
    const px = typeof p.x === 'number' ? p.x : (typeof p.X === 'number' ? p.X : idx % hPorts);
    const py = typeof p.y === 'number' ? p.y : (typeof p.Y === 'number' ? p.Y : Math.floor(idx / hPorts));
    const pstate = p.state || p.State || 'none';
    return {
      id: pid, ID: pid,
      name: pname, Name: pname,
      x: px, X: px,
      y: py, Y: py,
      state: pstate, State: pstate,
    };
  });

  let xMax = 5;
  let yMax = 0;
  for (const p of ports) {
    if (xMax < p.x) xMax = p.x;
    if (yMax < p.y) yMax = p.y;
  }
  const calcW = (xMax + 1) * 45 + 20;
  const calcH = (yMax + 1) * 55 + 12 + 20;

  const w = (typeof raw.w === 'number' && raw.w > 0) ? raw.w : ((typeof raw.W === 'number' && raw.W > 0) ? raw.W : calcW);
  const h = (typeof raw.h === 'number' && raw.h > 0) ? raw.h : ((typeof raw.H === 'number' && raw.H > 0) ? raw.H : calcH);

  return {
    ...raw,
    id, ID: id,
    name, Name: name,
    ip, IP: ip,
    x, X: x,
    y, Y: y,
    w, W: w,
    h, H: h,
    descr: raw.descr || raw.Descr || '',
    Descr: raw.descr || raw.Descr || '',
    snmp_mode: raw.snmp_mode || raw.SnmpMode || '',
    SnmpMode: raw.snmp_mode || raw.SnmpMode || '',
    community: raw.community || raw.Community || '',
    Community: raw.community || raw.Community || '',
    user: raw.user || raw.User || '',
    User: raw.user || raw.User || '',
    password: raw.password || raw.Password || '',
    Password: raw.password || raw.Password || '',
    snmp_port: raw.snmp_port || raw.SnmpPort || 0,
    SnmpPort: raw.snmp_port || raw.SnmpPort || 0,
    unmanaged: raw.unmanaged ?? raw.Unmanaged ?? false,
    Unmanaged: raw.unmanaged ?? raw.Unmanaged ?? false,
    error: raw.error || raw.Error || '',
    Error: raw.error || raw.Error || '',
    ports, Ports: ports,
  };
}

export function normalizeLine(raw: any): LineEnt {
  if (!raw) return { id: '', node_id1: '', node_id2: '', state: 'normal', width: 2 };
  const id = raw.id || raw.ID || '';
  const node_id1 = raw.node_id1 || raw.NodeID1 || '';
  const node_id2 = raw.node_id2 || raw.NodeID2 || '';
  const polling_id1 = raw.polling_id1 || raw.PollingID1 || '';
  const polling_id2 = raw.polling_id2 || raw.PollingID2 || '';
  const polling_id = raw.polling_id || raw.PollingID || '';
  const state = raw.state || raw.State || 'normal';
  const state1 = raw.state1 || raw.State1 || '';
  const state2 = raw.state2 || raw.State2 || '';
  const width = typeof raw.width === 'number' ? raw.width : (typeof raw.Width === 'number' ? raw.Width : 2);
  const info = raw.info || raw.Info || '';
  const port = raw.port || raw.Port || '';

  return {
    ...raw,
    id, ID: id,
    node_id1, NodeID1: node_id1,
    node_id2, NodeID2: node_id2,
    polling_id1, PollingID1: polling_id1,
    polling_id2, PollingID2: polling_id2,
    polling_id, PollingID: polling_id,
    state, State: state,
    state1, State1: state1,
    state2, State2: state2,
    width, Width: width,
    info, Info: info,
    port, Port: port,
  };
}

export function normalizeDrawItem(raw: any): DrawItemEnt {
  if (!raw) return { id: '', type: 0, x: 200, y: 200, w: 120, h: 40, text: '' };
  const id = raw.id || raw.ID || '';
  const type = typeof raw.type === 'number' ? raw.type : (typeof raw.Type === 'number' ? raw.Type : 0);
  const x = typeof raw.x === 'number' ? raw.x : (typeof raw.X === 'number' ? raw.X : 200);
  const y = typeof raw.y === 'number' ? raw.y : (typeof raw.Y === 'number' ? raw.Y : 200);
  const w = typeof raw.w === 'number' ? raw.w : (typeof raw.W === 'number' ? raw.W : 120);
  const h = typeof raw.h === 'number' ? raw.h : (typeof raw.H === 'number' ? raw.H : 40);
  const text = raw.text ?? raw.Text ?? '';
  const color = raw.color ?? raw.Color ?? '#38bdf8';
  const size = typeof raw.size === 'number' ? raw.size : (typeof raw.Size === 'number' ? raw.Size : 14);
  const path = raw.path ?? raw.Path ?? '';
  const node_id = raw.node_id ?? raw.NodeID ?? '';
  const polling_id = raw.polling_id ?? raw.PollingID ?? '';
  const var_name = raw.var_name ?? raw.VarName ?? '';
  const format = raw.format ?? raw.Format ?? '';
  const value = typeof raw.value === 'number' ? raw.value : (typeof raw.Value === 'number' ? raw.Value : 0);
  const scale = typeof raw.scale === 'number' ? raw.scale : (typeof raw.Scale === 'number' ? raw.Scale : 1.0);
  const cond = typeof raw.cond === 'number' ? raw.cond : (typeof raw.Cond === 'number' ? raw.Cond : 0);
  const values = raw.values ?? raw.Values ?? [];
  const formatted_text = raw.formatted_text ?? raw.FormattedText ?? '';

  return {
    ...raw,
    id, ID: id,
    type, Type: type,
    x, X: x,
    y, Y: y,
    w, W: w,
    h, H: h,
    text, Text: text,
    color, Color: color,
    size, Size: size,
    path, Path: path,
    node_id, NodeID: node_id,
    polling_id, PollingID: polling_id,
    var_name, VarName: var_name,
    format, Format: format,
    value, Value: value,
    scale, Scale: scale,
    cond, Cond: cond,
    values, Values: values,
    formatted_text, FormattedText: formatted_text,
  };
}

export function normalizePolling(raw: any): PollingEnt {
  if (!raw) return { id: '', node_id: '', name: '', type: 'ping', state: 'normal' };
  const id = raw.id || raw.ID || '';
  const node_id = raw.node_id || raw.NodeID || '';
  const name = raw.name || raw.Name || '';
  const type = raw.type || raw.Type || 'ping';
  const state = raw.state || raw.State || 'normal';
  const mode = raw.mode ?? raw.Mode ?? '';
  const params = raw.params ?? raw.Params ?? '';
  const filter = raw.filter ?? raw.Filter ?? '';
  const extractor = raw.extractor ?? raw.Extractor ?? '';
  const script = raw.script ?? raw.Script ?? '';
  const level = raw.level ?? raw.Level ?? '';
  const poll_int = raw.poll_int ?? raw.PollInt ?? 60;
  const timeout = raw.timeout ?? raw.Timeout ?? 1;
  const retry = raw.retry ?? raw.Retry ?? 1;
  const log_mode = raw.log_mode ?? raw.LogMode ?? 0;
  const fail_action = raw.fail_action ?? raw.FailAction ?? '';
  const repair_action = raw.repair_action ?? raw.RepairAction ?? '';
  const ai_mode = raw.ai_mode ?? raw.AIMode ?? '';
  const vector_cols = raw.vector_cols ?? raw.VectorCols ?? '';
  const mqtt_url = raw.mqtt_url ?? raw.MqttURL ?? '';
  const mqtt_topic = raw.mqtt_topic ?? raw.MqttTopic ?? '';
  const mqtt_cols = raw.mqtt_cols ?? raw.MqttCols ?? '';
  const next_time = raw.next_time ?? raw.NextTime ?? 0;
  const last_time = raw.last_time || raw.LastTime || 0;
  const fail_time = raw.fail_time ?? raw.FailTime ?? 0;

  let last_val = raw.last_val !== undefined ? raw.last_val : (raw.LastVal !== undefined ? raw.LastVal : undefined);
  const result = raw.Result || raw.result;
  if (last_val === undefined && result && typeof result === 'object') {
    if (result.rtt !== undefined) {
      const num = Number(result.rtt);
      if (!isNaN(num)) {
        last_val = num > 1000 ? num / 1000000 : num;
      }
    } else if (result.bps !== undefined) {
      const num = Number(result.bps);
      if (!isNaN(num)) last_val = num;
    } else if (result.cpu !== undefined) {
      const num = Number(result.cpu);
      if (!isNaN(num)) last_val = num;
    } else if (result.val !== undefined) {
      const num = Number(result.val);
      if (!isNaN(num)) last_val = num;
    }
  }

  return {
    ...raw,
    id, ID: id,
    node_id, NodeID: node_id,
    name, Name: name,
    type, Type: type,
    mode, Mode: mode,
    params, Params: params,
    filter, Filter: filter,
    extractor, Extractor: extractor,
    script, Script: script,
    level, Level: level,
    poll_int, PollInt: poll_int,
    timeout, Timeout: timeout,
    retry, Retry: retry,
    log_mode, LogMode: log_mode,
    fail_action, FailAction: fail_action,
    repair_action, RepairAction: repair_action,
    ai_mode, AIMode: ai_mode,
    vector_cols, VectorCols: vector_cols,
    mqtt_url, MqttURL: mqtt_url,
    mqtt_topic, MqttTopic: mqtt_topic,
    mqtt_cols, MqttCols: mqtt_cols,
    state, State: state,
    next_time, NextTime: next_time,
    last_time, LastTime: last_time,
    fail_time, FailTime: fail_time,
    last_val, LastVal: last_val,
    Result: result,
    result: result,
  };
}

export function normalizeEventLog(raw: any): EventLogEnt {
  if (!raw) return { time: Date.now() * 1000000, type: 'system', level: 'info', event: '' };
  const time = typeof raw.time === 'number' ? raw.time : (typeof raw.Time === 'number' ? raw.Time : Date.now() * 1000000);
  const type = raw.type || raw.Type || 'system';
  const level = raw.level || raw.Level || 'info';
  const node_id = raw.node_id || raw.NodeID || '';
  const node_name = raw.node_name || raw.NodeName || '';
  const event = raw.event || raw.Event || '';

  return {
    ...raw,
    time, Time: time,
    type, Type: type,
    level, Level: level,
    node_id, NodeID: node_id,
    node_name, NodeName: node_name,
    event, Event: event,
  };
}

export async function fetchHealth(): Promise<SystemHealth> {
  const res = await fetch(`${API_BASE}/health`);
  if (!res.ok) throw new Error(`Health check failed: ${res.statusText}`);
  return res.json();
}

export async function fetchSystemInfo(): Promise<SystemInfo> {
  const res = await fetch(`${API_BASE}/system/info`);
  if (!res.ok) throw new Error(`Fetch system info failed: ${res.statusText}`);
  return res.json();
}

export async function fetchMonitorData(): Promise<MonitorDataEnt[]> {
  const res = await fetch(`${API_BASE}/system/monitor`);
  if (!res.ok) throw new Error(`Fetch monitor data failed: ${res.statusText}`);
  const list = await res.json();
  return Array.isArray(list) ? list : [];
}

export async function updateMonitorData(): Promise<MonitorDataEnt> {
  const res = await fetch(`${API_BASE}/system/monitor/update`, { method: 'POST' });
  if (!res.ok) throw new Error(`Update monitor data failed: ${res.statusText}`);
  return res.json();
}

export async function execBackup(): Promise<{ file: string; size: number; time: string }> {
  const res = await fetch(`${API_BASE}/system/backup`, { method: 'POST' });
  if (!res.ok) throw new Error(`Backup failed: ${res.statusText}`);
  return res.json();
}

export async function fetchNodes(): Promise<NodeEnt[]> {
  const res = await fetch(`${API_BASE}/nodes`);
  if (!res.ok) throw new Error(`Fetch nodes failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeNode);
}

export interface ArpEnt {
  IP: string;
  MAC: string;
  Vendor?: string;
  NodeID?: string;
  FirstTime?: number;
  LastTime?: number;
}

export async function fetchArpTable(): Promise<ArpEnt[]> {
  const res = await fetch(`${API_BASE}/arp`);
  if (!res.ok) throw new Error(`Fetch ARP table failed: ${res.statusText}`);
  const list = await res.json();
  return Array.isArray(list) ? list : [];
}

export async function deleteArpEntries(ips: string[]): Promise<any> {
  const res = await fetch(`${API_BASE}/arp`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ips }),
  });
  if (!res.ok) throw new Error(`Delete ARP entries failed: ${res.statusText}`);
  return res.json();
}

export async function resetArpTable(): Promise<any> {
  const res = await fetch(`${API_BASE}/arp?all=true`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error(`Reset ARP table failed: ${res.statusText}`);
  return res.json();
}

export async function saveNode(node: Partial<NodeEnt>): Promise<NodeEnt> {
  const payload = {
    ID: node.id || node.ID || '',
    Name: node.name || node.Name || '',
    IP: node.ip || node.IP || '',
    MAC: node.mac || node.MAC || '',
    Icon: node.icon || node.Icon || 'desktop',
    Image: node.image || node.Image || '',
    State: node.state || node.State || 'normal',
    X: typeof node.x === 'number' ? node.x : (typeof node.X === 'number' ? node.X : 300),
    Y: typeof node.y === 'number' ? node.y : (typeof node.Y === 'number' ? node.Y : 250),
    Descr: node.descr || node.Descr || '',
    SnmpMode: node.snmp_mode || (node as any).SnmpMode || 'v2c',
    Community: node.community || (node as any).Community || 'public',
    User: node.user || (node as any).User || '',
    Password: node.password || (node as any).Password || '',
    AddrMode: node.addr_mode || (node as any).AddrMode || 'ip',
    SnmpPort: node.snmp_port || node.SnmpPort || 161,
    AutoAck: node.auto_ack ?? node.AutoAck ?? false,
    URL: node.url || node.URL || '',
    SSHUser: node.ssh_user || node.SSHUser || '',
    PublicKey: node.public_key || node.PublicKey || '',
    GNMIPort: node.gnmi_port || node.GNMIPort || '',
    GNMIEncoding: node.gnmi_encoding || node.GNMIEncoding || '',
    GNMIUser: node.gnmi_user || node.GNMIUser || '',
    GNMIPassword: node.gnmi_password || node.GNMIPassword || '',
  };
  const res = await fetch(`${API_BASE}/nodes`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Save node failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizeNode(saved);
}

export async function deleteNode(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/nodes/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete node failed: ${res.statusText}`);
}

export async function fetchLines(): Promise<LineEnt[]> {
  const res = await fetch(`${API_BASE}/lines`);
  if (!res.ok) throw new Error(`Fetch lines failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeLine);
}

export async function saveLine(line: Partial<LineEnt>): Promise<LineEnt> {
  const payload = {
    ID: line.id || line.ID || '',
    NodeID1: line.node_id1 || line.NodeID1 || '',
    PollingID1: line.polling_id1 || line.PollingID1 || '',
    State1: line.state1 || line.State1 || '',
    NodeID2: line.node_id2 || line.NodeID2 || '',
    PollingID2: line.polling_id2 || line.PollingID2 || '',
    State2: line.state2 || line.State2 || '',
    PollingID: line.polling_id || line.PollingID || '',
    State: line.state || line.State || 'normal',
    Width: typeof line.width === 'number' ? line.width : (typeof line.Width === 'number' ? line.Width : 2),
    Info: line.info || line.Info || '',
    Port: line.port || line.Port || '',
  };
  const res = await fetch(`${API_BASE}/lines`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Save line failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizeLine(saved);
}

export async function deleteLine(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/lines/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete line failed: ${res.statusText}`);
}

export async function fetchNeighbors(id: string): Promise<FindNeighborNetworksAndLinesResp> {
  const cleanId = id.trim();
  const res = await fetch(`${API_BASE}/topology/neighbors/${cleanId}`);
  if (!res.ok) throw new Error(`Fetch neighbors failed: ${res.statusText}`);
  const data = await res.json();
  return {
    Networks: (data.Networks || []).map(normalizeNetwork),
    Lines: (data.Lines || []).map((l: any) => ({
      ...normalizeLine(l),
      Confidence: l.Confidence || 'speculative',
      Reason: l.Reason || l.Info || '',
    })),
  };
}

export async function connectLines(lines: Partial<LineEnt>[]): Promise<{ connected: number }> {
  const payload = lines.map((line) => ({
    ID: line.id || line.ID || '',
    NodeID1: line.node_id1 || line.NodeID1 || '',
    PollingID1: line.polling_id1 || line.PollingID1 || '',
    State1: line.state1 || line.State1 || '',
    NodeID2: line.node_id2 || line.NodeID2 || '',
    PollingID2: line.polling_id2 || line.PollingID2 || '',
    State2: line.state2 || line.State2 || '',
    PollingID: line.polling_id || line.PollingID || '',
    State: line.state || line.State || 'normal',
    Width: typeof line.width === 'number' ? line.width : (typeof line.Width === 'number' ? line.Width : 2),
    Info: line.info || line.Info || '',
    Port: line.port || line.Port || '',
  }));
  const res = await fetch(`${API_BASE}/topology/connect-lines`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Connect lines failed: ${res.statusText}`);
  return res.json();
}

export async function fetchNetworks(): Promise<NetworkEnt[]> {
  const res = await fetch(`${API_BASE}/networks`);
  if (!res.ok) throw new Error(`Fetch networks failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeNetwork);
}

export async function saveNetwork(net: Partial<NetworkEnt>): Promise<NetworkEnt> {
  const rawPorts = net.ports || (net as any).Ports || [];
  const ports = rawPorts.map((p: any, idx: number) => ({
    ID: p.id || p.ID || `p${idx + 1}`,
    Name: p.name || p.Name || `Port ${idx + 1}`,
    Index: p.index || p.Index || p.id || p.ID || '',
    Polling: p.polling || p.Polling || '',
    X: typeof p.x === 'number' ? p.x : (typeof p.X === 'number' ? p.X : idx % 8),
    Y: typeof p.y === 'number' ? p.y : (typeof p.Y === 'number' ? p.Y : Math.floor(idx / 8)),
    State: p.state || p.State || 'none',
  }));

  const payload = {
    ID: net.id || (net as any).ID || '',
    Name: net.name || (net as any).Name || '',
    IP: net.ip || (net as any).IP || '',
    X: typeof net.x === 'number' ? net.x : (typeof (net as any).X === 'number' ? (net as any).X : 300),
    Y: typeof net.y === 'number' ? net.y : (typeof (net as any).Y === 'number' ? (net as any).Y : 150),
    W: typeof net.w === 'number' ? net.w : (typeof (net as any).W === 'number' ? (net as any).W : 420),
    H: typeof net.h === 'number' ? net.h : (typeof (net as any).H === 'number' ? (net as any).H : 90),
    SnmpMode: (net as any).snmp_mode || (net as any).SnmpMode || '',
    Community: (net as any).community || (net as any).Community || '',
    User: (net as any).user || (net as any).User || '',
    Password: (net as any).password || (net as any).Password || '',
    SnmpPort: (net as any).snmp_port || (net as any).SnmpPort || 0,
    Unmanaged: (net as any).unmanaged ?? (net as any).Unmanaged ?? false,
    SystemID: (net as any).system_id || (net as any).SystemID || '',
    Descr: (net as any).descr || (net as any).Descr || '',
    HPorts: net.h_ports || (net as any).HPorts || 24,
    LLDP: (net as any).lldp ?? (net as any).LLDP ?? false,
    Ports: ports,
  };
  const res = await fetch(`${API_BASE}/networks`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Save network failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizeNetwork(saved);
}

export async function deleteNetwork(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/networks/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete network failed: ${res.statusText}`);
}

export async function refreshNetworkPorts(id: string): Promise<NetworkEnt> {
  const res = await fetch(`${API_BASE}/networks/${id}/ports`, { method: 'POST' });
  if (!res.ok) throw new Error(`Refresh network ports failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizeNetwork(saved);
}

export async function fetchDrawItems(): Promise<DrawItemEnt[]> {
  const res = await fetch(`${API_BASE}/drawitems`);
  if (!res.ok) throw new Error(`Fetch draw items failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeDrawItem);
}

export async function fetchDrawItem(id: string): Promise<DrawItemEnt> {
  const res = await fetch(`${API_BASE}/drawitems/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error(`Fetch draw item failed: ${res.statusText}`);
  const item = await res.json();
  return normalizeDrawItem(item);
}

export async function saveDrawItem(item: Partial<DrawItemEnt>): Promise<DrawItemEnt> {
  const payload = {
    ID: item.id || item.ID || '',
    Type: typeof item.type === 'number' ? item.type : (typeof item.Type === 'number' ? item.Type : 0),
    X: typeof item.x === 'number' ? item.x : (typeof item.X === 'number' ? item.X : 200),
    Y: typeof item.y === 'number' ? item.y : (typeof item.Y === 'number' ? item.Y : 200),
    W: typeof item.w === 'number' ? item.w : (typeof item.W === 'number' ? item.W : 120),
    H: typeof item.h === 'number' ? item.h : (typeof item.H === 'number' ? item.H : 40),
    Text: item.text ?? item.Text ?? '',
    Color: item.color ?? item.Color ?? '#38bdf8',
    Size: typeof item.size === 'number' ? item.size : (typeof item.Size === 'number' ? item.Size : 14),
    Path: item.path ?? item.Path ?? '',
    PollingID: item.polling_id ?? item.PollingID ?? '',
    VarName: item.var_name ?? item.VarName ?? '',
    Format: item.format ?? item.Format ?? '',
    Value: typeof item.value === 'number' ? item.value : (typeof item.Value === 'number' ? item.Value : 0),
    Scale: typeof item.scale === 'number' ? item.scale : (typeof item.Scale === 'number' ? item.Scale : 1.0),
    Cond: typeof item.cond === 'number' ? item.cond : (typeof item.Cond === 'number' ? item.Cond : 0),
    Values: item.values ?? item.Values ?? [],
    FormattedText: item.formatted_text ?? item.FormattedText ?? '',
  };
  const res = await fetch(`${API_BASE}/drawitems`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Save draw item failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizeDrawItem(saved);
}

export async function copyDrawItem(id: string): Promise<DrawItemEnt> {
  try {
    const res = await fetch(`${API_BASE}/drawitems/${encodeURIComponent(id)}/copy`, { method: 'POST' });
    if (res.ok) {
      const item = await res.json();
      return normalizeDrawItem(item);
    }
  } catch (err) {
    console.warn('POST /drawitems/:id/copy failed, attempting client-side clone:', err);
  }

  // Fallback: fetch existing items, find target, and save as new item with X + 100
  const allItems = await fetchDrawItems();
  const target = allItems.find((it) => (it.id || (it as any).ID) === id);
  if (!target) {
    throw new Error(`DrawItem not found: ${id}`);
  }
  const copyPayload: DrawItemEnt = {
    ...target,
    id: '',
    ID: '',
    x: (typeof target.x === 'number' ? target.x : (typeof (target as any).X === 'number' ? (target as any).X : 200)) + 100,
    X: (typeof target.x === 'number' ? target.x : (typeof (target as any).X === 'number' ? (target as any).X : 200)) + 100,
    y: typeof target.y === 'number' ? target.y : (typeof (target as any).Y === 'number' ? (target as any).Y : 200),
    Y: typeof target.y === 'number' ? target.y : (typeof (target as any).Y === 'number' ? (target as any).Y : 200),
  };
  return await saveDrawItem(copyPayload);
}

export async function deleteDrawItem(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/drawitems/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete draw item failed: ${res.statusText}`);
}

export async function fetchMapConf(): Promise<any> {
  const res = await fetch(`${API_BASE}/map/conf`);
  if (!res.ok) throw new Error(`Fetch map conf failed: ${res.statusText}`);
  return res.json();
}
export const getMapConf = fetchMapConf;

export async function saveMapConf(conf: any): Promise<any> {
  const res = await fetch(`${API_BASE}/map/conf`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(conf),
  });
  if (!res.ok) throw new Error(`Save map conf failed: ${res.statusText}`);
  return res.json();
}

export interface NotifyConfEnt {
  Provider?: string;
  MailServer?: string;
  InsecureSkipVerify?: boolean;
  User?: string;
  Password?: string;
  MailTo?: string;
  MailFrom?: string;
  Subject?: string;
  Interval?: number;
  Level?: string;
  Report?: boolean;
  LLMSummary?: boolean;
  NotifyRepair?: boolean;
  CheckDependency?: boolean;
  ExecCmd?: string;
  WebHookNotify?: string;
  WebHookReport?: string;
  ClientID?: string;
  ClientSecret?: string;
  MSTenant?: string;
}

export async function fetchNotifyConf(): Promise<NotifyConfEnt> {
  const res = await fetch(`${API_BASE}/notify/conf`);
  if (!res.ok) throw new Error(`Fetch notify conf failed: ${res.statusText}`);
  return res.json();
}

export async function saveNotifyConf(conf: NotifyConfEnt): Promise<NotifyConfEnt> {
  const res = await fetch(`${API_BASE}/notify/conf`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(conf),
  });
  if (!res.ok) throw new Error(`Save notify conf failed: ${res.statusText}`);
  return res.json();
}

export async function testNotifyMail(conf: NotifyConfEnt): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/notify/test/mail`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(conf),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Test mail failed: ${res.statusText}`);
  }
  return res.json();
}

export async function testNotifyWebhook(conf: NotifyConfEnt): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/notify/test/webhook`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(conf),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Test webhook failed: ${res.statusText}`);
  }
  return res.json();
}

export async function startNotifyOAuth2(): Promise<{ url: string }> {
  const res = await fetch(`${API_BASE}/notify/oauth2/start`, {
    method: 'POST',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Start OAuth2 failed: ${res.statusText}`);
  }
  return res.json();
}

export async function deleteNotifyOAuth2Token(): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/notify/oauth2/token`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error(`Delete OAuth2 token failed: ${res.statusText}`);
  return res.json();
}

export async function fetchNotifyOAuth2Status(): Promise<{ hasToken: boolean }> {
  const res = await fetch(`${API_BASE}/notify/oauth2/status`);
  if (!res.ok) throw new Error(`Fetch OAuth2 status failed: ${res.statusText}`);
  return res.json();
}

export async function uploadGeoIP(file: File): Promise<any> {
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE}/conf/geoip`, {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Upload GeoIP DB failed: ${res.statusText}`);
  }
  return res.json();
}

export async function deleteGeoIP(): Promise<any> {
  const res = await fetch(`${API_BASE}/conf/geoip`, {
    method: 'DELETE',
  });
  if (!res.ok) throw new Error(`Delete GeoIP DB failed: ${res.statusText}`);
  return res.json();
}

export async function fetchPollings(): Promise<PollingEnt[]> {
  const res = await fetch(`${API_BASE}/pollings`);
  if (!res.ok) throw new Error(`Fetch pollings failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizePolling);
}

export async function savePolling(poll: Partial<PollingEnt>): Promise<PollingEnt> {
  const payload: any = {
    ID: poll.id || poll.ID || '',
    NodeID: poll.node_id || poll.NodeID || '',
    Name: poll.name || poll.Name || '',
    Type: poll.type || poll.Type || 'ping',
    State: poll.state ?? poll.State ?? '',
    Mode: poll.mode ?? poll.Mode ?? '',
    Params: poll.params ?? poll.Params ?? '',
    Filter: poll.filter ?? poll.Filter ?? '',
    Extractor: poll.extractor ?? poll.Extractor ?? '',
    Script: poll.script ?? poll.Script ?? '',
    Level: poll.level ?? poll.Level ?? '',
    PollInt: poll.poll_int ?? poll.PollInt ?? 60,
    Timeout: poll.timeout ?? poll.Timeout ?? 1,
    Retry: poll.retry ?? poll.Retry ?? 1,
    LogMode: poll.log_mode ?? poll.LogMode ?? 0,
    FailAction: poll.fail_action ?? poll.FailAction ?? '',
    RepairAction: poll.repair_action ?? poll.RepairAction ?? '',
    AIMode: poll.ai_mode ?? poll.AIMode ?? '',
    VectorCols: poll.vector_cols ?? poll.VectorCols ?? '',
    MqttURL: poll.mqtt_url ?? poll.MqttURL ?? '',
    MqttTopic: poll.mqtt_topic ?? poll.MqttTopic ?? '',
    MqttCols: poll.mqtt_cols ?? poll.MqttCols ?? '',
  };
  if (poll.Result || poll.result) {
    payload.Result = poll.Result ?? poll.result;
  }
  if (poll.LastTime !== undefined || poll.last_time !== undefined) {
    payload.LastTime = poll.LastTime ?? poll.last_time;
  }
  const res = await fetch(`${API_BASE}/pollings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Save polling failed: ${res.statusText}`);
  const saved = await res.json();
  return normalizePolling(saved);
}

export async function deletePolling(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/pollings/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete polling failed: ${res.statusText}`);
}

export interface PollingTemplateEnt {
  ID: number;
  Name: string;
  Level: string;
  Type: string;
  Mode: string;
  Params: string;
  Filter: string;
  Extractor: string;
  Script: string;
  Descr: string;
  AutoParam: string;
}

export async function fetchPollingTemplates(lang = ''): Promise<PollingTemplateEnt[]> {
  const q = lang ? `?lang=${encodeURIComponent(lang)}` : '';
  const res = await fetch(`${API_BASE}/polling/templates${q}`);
  if (!res.ok) throw new Error(`Fetch polling templates failed: ${res.statusText}`);
  const list = await res.json();
  return Array.isArray(list) ? list : [];
}

export async function fetchPollingTemplate(id: number, lang = ''): Promise<PollingTemplateEnt> {
  const q = lang ? `?lang=${encodeURIComponent(lang)}` : '';
  const res = await fetch(`${API_BASE}/polling/template/${id}${q}`);
  if (!res.ok) throw new Error(`Fetch polling template failed: ${res.statusText}`);
  return res.json();
}

export async function generateAutoPollings(nodeID: string, templateID: number, lang = ''): Promise<PollingEnt[]> {
  const res = await fetch(`${API_BASE}/polling/auto`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ nodeID, templateID, lang }),
  });
  if (!res.ok) throw new Error(`Generate auto pollings failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizePolling);
}

export async function fetchAutoGrok(testData: string): Promise<string> {
  const res = await fetch(`${API_BASE}/polling/autogrok`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ testData }),
  });
  if (!res.ok) return '';
  const data = await res.json();
  return data.pattern || '';
}

export interface EventLogQueryFilter {
  start?: number;
  end?: number;
  level?: string;
  type?: string;
  nodeId?: string;
  nodeName?: string;
  filter?: string;
  limit?: number;
}

export async function fetchEventLogs(filter?: EventLogQueryFilter): Promise<EventLogEnt[]> {
  const params = new URLSearchParams();
  if (filter) {
    if (filter.start) params.set('start', String(filter.start));
    if (filter.end) params.set('end', String(filter.end));
    if (filter.level && filter.level !== 'all') params.set('level', filter.level);
    if (filter.type && filter.type !== 'all') params.set('type', filter.type);
    if (filter.nodeId) params.set('nodeId', filter.nodeId);
    if (filter.nodeName) params.set('nodeName', filter.nodeName);
    if (filter.filter) params.set('filter', filter.filter);
    if (filter.limit) params.set('limit', String(filter.limit));
  }
  const queryStr = params.toString();
  const res = await fetch(`${API_BASE}/logs/events${queryStr ? '?' + queryStr : ''}`);
  if (!res.ok) throw new Error(`Fetch event logs failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeEventLog);
}

export async function deleteEventLogs(): Promise<void> {
  const res = await fetch(`${API_BASE}/logs/events`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete event logs failed: ${res.statusText}`);
}

export function normalizeParquetLog(raw: any): ParquetLogRecord {
  if (!raw) return { time: 0, type: '', src: '', log: '' };
  const time = typeof raw.time === 'number' ? raw.time : (typeof raw.Time === 'number' ? raw.Time : 0);
  const type = raw.type || raw.Type || '';
  const src = raw.src || raw.Src || '';
  const log = raw.log || raw.Log || '';

  return {
    ...raw,
    time, Time: time,
    type, Type: type,
    src, Src: src,
    log, Log: log,
  };
}

export interface ParquetLogQueryFilter {
  type?: string;
  filter?: string;
  src?: string;
  start?: number;
  end?: number;
  limit?: number;
  level?: string;
  tag?: string;
}

export async function queryParquetLogs(typeOrFilter: string | ParquetLogQueryFilter = '', filterStr = ''): Promise<ParquetLogRecord[]> {
  const params = new URLSearchParams();
  if (typeof typeOrFilter === 'object') {
    if (typeOrFilter.type) params.set('type', typeOrFilter.type);
    if (typeOrFilter.filter) params.set('filter', typeOrFilter.filter);
    if (typeOrFilter.src) params.set('src', typeOrFilter.src);
    if (typeOrFilter.start) params.set('start', String(typeOrFilter.start));
    if (typeOrFilter.end) params.set('end', String(typeOrFilter.end));
    if (typeOrFilter.limit) params.set('limit', String(typeOrFilter.limit));
    if (typeOrFilter.level && typeOrFilter.level !== 'all') params.set('level', typeOrFilter.level);
    if (typeOrFilter.tag && typeOrFilter.tag !== 'all') params.set('tag', typeOrFilter.tag);
  } else {
    if (typeOrFilter) params.set('type', typeOrFilter);
    if (filterStr) params.set('filter', filterStr);
  }
  const res = await fetch(`${API_BASE}/logs/query?${params.toString()}`);
  if (!res.ok) throw new Error(`Query parquet logs failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeParquetLog);
}

export async function deleteParquetLogs(logType: string): Promise<void> {
  const res = await fetch(`${API_BASE}/logs/query?type=${encodeURIComponent(logType)}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete parquet logs failed: ${res.statusText}`);
}

export async function getLogCounts(): Promise<Record<string, number>> {
  const res = await fetch(`${API_BASE}/logs/counts`);
  if (!res.ok) return {};
  return res.json();
}

export async function askAI(prompt: string, system = ''): Promise<string> {
  const res = await fetch(`${API_BASE}/ai/ask`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ prompt, system }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  const data = await res.json();
  return data.answer;
}

export async function diagnoseAlert(alertEvent: string, nodeContext: string): Promise<string> {
  const res = await fetch(`${API_BASE}/ai/diagnose`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ alert_event: alertEvent, node_context: nodeContext }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  const data = await res.json();
  return data.diagnosis;
}

export interface PingResult {
  Stat: number;
  TimeStamp: number;
  Time: number; // nanoseconds
  Size: number;
  SendTTL: number;
  RecvTTL: number;
  RecvSrc: string;
  Loc: string;
}

export async function execPing(ip: string, size = 64, ttl = 64): Promise<PingResult> {
  const res = await fetch(`${API_BASE}/tools/ping`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ip, size, ttl }),
  });
  if (!res.ok) throw new Error(`Ping failed: ${res.statusText}`);
  return res.json();
}

export async function sendWol(mac: string, ip = '255.255.255.255', nodeId = ''): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/tools/wol`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mac, ip, node_id: nodeId }),
  });
  if (!res.ok) throw new Error(`WOL failed: ${res.statusText}`);
  return res.json();
}

// OpenTelemetry (OTel) Models
export interface OTelMetricDataPointEnt {
  Start: number;
  Time: number;
  Attributes: string[];
  Count?: number;
  BucketCounts?: number[];
  ExplicitBounds?: number[];
  Sum?: number;
  Min?: number;
  Max?: number;
  Gauge?: number;
  Positive?: number[];
  Negative?: number[];
  Scale?: number;
  ZeroCount?: number;
  ZeroThreshold?: number;
  Index: number;
}

export interface OTelMetricEnt {
  Host: string;
  Service: string;
  Scope: string;
  Name: string;
  Type: string;
  Description: string;
  Unit: string;
  DataPoints?: OTelMetricDataPointEnt[];
  Count: number;
  First: number;
  Last: number;
}

export interface OTelTraceSpanEnt {
  SpanID: string;
  ParentSpanID: string;
  Host: string;
  Service: string;
  Scope: string;
  Name: string;
  Start: number;
  End: number;
  Dur: number;
  Attributes: string[];
}

export interface OTelTraceEnt {
  Bucket: string;
  TraceID: string;
  Start: number;
  End: number;
  Dur: number;
  Spans: OTelTraceSpanEnt[];
  Last: number;
}

export interface OTelTraceSummaryEnt {
  Bucket: string;
  TraceID: string;
  Hosts: string;
  Services: string;
  Scopes: string;
  Start: number;
  End: number;
  Dur: number;
  NumSpan: number;
}

export interface OTelTraceDAGNodeEnt {
  Name: string;
  Count: number;
}

export interface OTelTraceDAGLinkEnt {
  Src: string;
  Dst: string;
  Count: number;
}

export interface OTelTraceDAGEnt {
  Nodes: OTelTraceDAGNodeEnt[];
  Links: OTelTraceDAGLinkEnt[];
}

export interface OTelLogEnt {
  time: number;
  host: string;
  service: string;
  scope: string;
  traceId: string;
  spanId: string;
  severity: number;
  severityText: string;
  message: string;
  attributes?: Record<string, string>;
}

// OTel API Functions
export async function fetchOTelMetrics(): Promise<OTelMetricEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/otel/metrics`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchOTelMetricDetail(host: string, service: string, scope: string, name: string): Promise<OTelMetricEnt | null> {
  try {
    const params = new URLSearchParams({ host, service, scope, name });
    const res = await fetch(`${API_BASE}/otel/metrics/detail?${params}`);
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function deleteOTelMetric(host: string, service: string, scope: string, name: string): Promise<boolean> {
  try {
    const params = new URLSearchParams({ host, service, scope, name });
    const res = await fetch(`${API_BASE}/otel/metrics?${params}`, { method: 'DELETE' });
    return res.ok;
  } catch {
    return false;
  }
}

export async function fetchOTelTraceBuckets(): Promise<string[]> {
  try {
    const res = await fetch(`${API_BASE}/otel/traces/buckets`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchOTelTraces(buckets?: string[], limit = 5000): Promise<OTelTraceSummaryEnt[]> {
  try {
    const params = new URLSearchParams();
    if (buckets && buckets.length > 0) {
      for (const b of buckets) {
        params.append('bucket', b);
      }
    }
    if (limit) {
      params.set('limit', String(limit));
    }
    const res = await fetch(`${API_BASE}/otel/traces?${params}`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchOTelTraceDetail(bucket: string, traceId: string): Promise<OTelTraceEnt | null> {
  try {
    const params = new URLSearchParams({ bucket, traceId });
    const res = await fetch(`${API_BASE}/otel/traces/detail?${params}`);
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}

export async function fetchOTelDAG(buckets?: string[]): Promise<OTelTraceDAGEnt> {
  try {
    const res = await fetch(`${API_BASE}/otel/traces/dag`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ buckets: buckets || [] }),
    });
    if (!res.ok) return { Nodes: [], Links: [] };
    const data = await res.json();
    return {
      Nodes: Array.isArray(data?.Nodes) ? data.Nodes : [],
      Links: Array.isArray(data?.Links) ? data.Links : [],
    };
  } catch {
    return { Nodes: [], Links: [] };
  }
}

export async function fetchOTelLogs(params: { filter?: string; src?: string; start?: number; end?: number; limit?: number }): Promise<ParquetLogRecord[]> {
  try {
    const q = new URLSearchParams();
    if (params.filter) q.set('filter', params.filter);
    if (params.src) q.set('src', params.src);
    if (params.start) q.set('start', String(params.start));
    if (params.end) q.set('end', String(params.end));
    if (params.limit) q.set('limit', String(params.limit));
    const res = await fetch(`${API_BASE}/otel/logs?${q}`);
    if (!res.ok) return [];
    const data = await res.json();
    return (Array.isArray(data) ? data : []).map(normalizeParquetLog);
  } catch {
    return [];
  }
}

export async function deleteAllOTelData(): Promise<boolean> {
  const res = await fetch(`${API_BASE}/otel/all`, { method: 'DELETE' });
  return res.ok;
}

// MQTT Statistics
export interface MqttStatEnt {
  ID: string;
  State: string; // normal, warn, low
  ClientID: string;
  Topic: string;
  Remote: string;
  Count: number;
  Bytes: number;
  First: number;
  Last: number;
  Value: string;
}

export async function fetchMqttStats(): Promise<MqttStatEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/mqtt/stats`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function deleteMqttStats(ids: string[]): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/mqtt/stats`, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    });
    return res.ok;
  } catch {
    return false;
  }
}

export async function deleteAllMqttStats(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/mqtt/stats/all`, { method: 'DELETE' });
    return res.ok;
  } catch {
    return false;
  }
}

export async function fetchMqttLogs(params: {
  filter?: string;
  src?: string;
  start?: number;
  end?: number;
  limit?: number;
}): Promise<ParquetLogRecord[]> {
  try {
    const q = new URLSearchParams();
    q.set('type', 'mqtt');
    if (params.filter) q.set('filter', params.filter);
    if (params.src) q.set('src', params.src);
    if (params.start) q.set('start', String(params.start));
    if (params.end) q.set('end', String(params.end));
    if (params.limit) q.set('limit', String(params.limit));
    const res = await fetch(`${API_BASE}/logs/query?${q}`);
    if (!res.ok) return [];
    const data = await res.json();
    return (Array.isArray(data) ? data : []).map(normalizeParquetLog);
  } catch {
    return [];
  }
}

export async function deleteMqttLogs(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/logs/query?type=mqtt`, { method: 'DELETE' });
    return res.ok;
  } catch {
    return false;
  }
}

export interface IPAMSubnetBlock {
  Subnet: string;
  Size: number;
  Used: number;
  Usage: number;
}

export interface IPAMRangeEnt {
  Range: string;
  StartIP: string;
  EndIP: string;
  Size: number;
  Used: number;
  Usage: number;
  UsedIP: number[]; // 100 slots (0..99%)
  Subnets?: IPAMSubnetBlock[];
}

export interface IPAMReportResp {
  Ranges: IPAMRangeEnt[];
  TotalRanges: number;
  TotalSize: number;
  TotalUsed: number;
  TotalUsage: number;
}

export async function fetchIPAM(): Promise<IPAMReportResp> {
  try {
    const res = await fetch(`${API_BASE}/ipam`);
    if (!res.ok) {
      return { Ranges: [], TotalRanges: 0, TotalSize: 0, TotalUsed: 0, TotalUsage: 0 };
    }
    return await res.json();
  } catch {
    return { Ranges: [], TotalRanges: 0, TotalSize: 0, TotalUsed: 0, TotalUsage: 0 };
  }
}

export interface VPanelPortEnt {
  Index: number;
  Name: string;
  State: string; // up, down, off
  Speed: number;
  InBytes?: number;
  OutBytes?: number;
  InError?: number;
  OutError?: number;
}

export interface NodePortsResp {
  supported: boolean;
  reason?: string;
  message?: string;
  error?: string;
  ports?: VPanelPortEnt[];
}

export interface HrSystem {
  Index: number;
  Key: string;
  Value: string;
}

export interface HrStorage {
  Index: string;
  Type: string;
  Descr: string;
  Size: number;
  Used: number;
  Unit: number;
  Rate: number;
}

export interface HrDevice {
  Index: string;
  Type: string;
  Descr: string;
  Status: string;
  Errors: string;
}

export interface HrFileSystem {
  Index: string;
  Type: string;
  Mount: string;
  Remote: string;
  Bootable: number;
  Access: number;
}

export interface HrProcess {
  PID: string;
  Name: string;
  Type: string;
  Status: string;
  Path: string;
  Param: string;
  CPU: number;
  Mem: number;
}

export interface HostResourceEnt {
  System: HrSystem[];
  Storage: HrStorage[];
  Device: HrDevice[];
  FileSystem: HrFileSystem[];
  Process: HrProcess[];
}

export interface NodeHostResourceResp {
  supported: boolean;
  reason?: string;
  message?: string;
  error?: string;
  data?: HostResourceEnt;
}

export async function fetchNodePorts(nodeId: string): Promise<NodePortsResp> {
  try {
    const res = await fetch(`${API_BASE}/nodes/${encodeURIComponent(nodeId)}/ports`);
    if (!res.ok) {
      return { supported: false, error: `HTTP ${res.status}` };
    }
    return await res.json();
  } catch (err: any) {
    return { supported: false, error: String(err?.message || err) };
  }
}

export async function fetchNodeHostResource(nodeId: string): Promise<NodeHostResourceResp> {
  try {
    const res = await fetch(`${API_BASE}/nodes/${encodeURIComponent(nodeId)}/hostresource`);
    if (!res.ok) {
      return { supported: false, error: `HTTP ${res.status}` };
    }
    return await res.json();
  } catch (err: any) {
    return { supported: false, error: String(err?.message || err) };
  }
}

export interface BackImageEnt {
  X: number;
  Y: number;
  Width: number;
  Height: number;
  Path: string;
}

export async function checkAllPollings(): Promise<{ status: string; count: number }> {
  const res = await fetch(`${API_BASE}/polling/check-all`, { method: "POST" });
  if (!res.ok) throw new Error(`Check all pollings failed: ${res.statusText}`);
  return await res.json();
}

export async function checkNodePollings(nodeId: string): Promise<{ status: string; count: number }> {
  const res = await fetch(`${API_BASE}/polling/check/${encodeURIComponent(nodeId)}`, { method: "POST" });
  if (!res.ok) throw new Error(`Check node pollings failed: ${res.statusText}`);
  return res.json();
}

export interface MIBInfoEnt {
  oid: string;
  type: string;
  description: string;
  units?: string;
  enum?: string;
}

export interface MIBTreeEnt {
  oid: string;
  name: string;
  mibInfo?: MIBInfoEnt;
  children?: MIBTreeEnt[];
}

export interface SNMPToolResult {
  name?: string;
  oid: string;
  type: string;
  value: string;
  mib?: MIBInfoEnt;
}

export interface MIBModuleEnt {
  type: string;
  file: string;
  name: string;
  error?: string;
  Type?: string;
  File?: string;
  Name?: string;
  Error?: string;
}

export async function fetchMIBTree(): Promise<MIBTreeEnt[]> {
  const res = await fetch(`${API_BASE}/mib/tree`);
  if (!res.ok) throw new Error(`Fetch MIB tree failed: ${res.statusText}`);
  return res.json();
}

export async function fetchMIBModules(): Promise<MIBModuleEnt[]> {
  const res = await fetch(`${API_BASE}/mib/modules`);
  if (!res.ok) throw new Error(`Fetch MIB modules failed: ${res.statusText}`);
  return res.json();
}

export async function uploadMIBModule(file: File): Promise<MIBModuleEnt[]> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(`${API_BASE}/mib/upload`, {
    method: "POST",
    body: fd,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `Upload MIB failed: ${res.statusText}`);
  }
  return res.json();
}

export async function deleteMIBModule(file: string): Promise<MIBModuleEnt[]> {
  const res = await fetch(`${API_BASE}/mib/modules`, {
    method: "DELETE",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ file }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `Delete MIB failed: ${res.statusText}`);
  }
  return res.json();
}

export async function reloadMIBModules(): Promise<MIBModuleEnt[]> {
  const res = await fetch(`${API_BASE}/mib/reload`, {
    method: "POST",
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `Reload MIBs failed: ${res.statusText}`);
  }
  return res.json();
}

export async function runSNMPTool(
  target: { nodeId?: string; networkId?: string },
  oid: string,
  mode: "get" | "getnext" | "walk" | "table",
  raw = false
): Promise<SNMPToolResult[]> {
  const res = await fetch(`${API_BASE}/tools/snmp`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ node_id: target.nodeId, network_id: target.networkId, oid, mode, raw }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `SNMP request failed: ${res.statusText}`);
  }
  return res.json();
}

export async function checkNetwork(id: string): Promise<NetworkEnt> {
  const res = await fetch(`${API_BASE}/networks/${encodeURIComponent(id)}/check`, { method: "POST" });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `Network check failed: ${res.statusText}`);
  }
  return normalizeNetwork(await res.json());
}

export interface GNMICapabilitiesEnt {
  version: string;
  encodings: string;
  models: { name: string; organization: string; version: string }[];
}

export interface GNMIValueEnt {
  Path: string;
  Value: string;
  Index?: string;
}

export async function fetchGNMICapabilities(nodeId: string): Promise<GNMICapabilitiesEnt> {
  const res = await fetch(`${API_BASE}/tools/gnmi/capabilities`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ node_id: nodeId }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `gNMI capabilities failed: ${res.statusText}`);
  }
  return res.json();
}

export async function runGNMIGet(nodeId: string, path: string, encoding: string): Promise<GNMIValueEnt[]> {
  const res = await fetch(`${API_BASE}/tools/gnmi/get`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ node_id: nodeId, path, encoding }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `gNMI Get failed: ${res.statusText}`);
  }
  return res.json();
}

export async function updateNodePositions(positions: { ID: string; X: number; Y: number }[]): Promise<any> {
  const res = await fetch(`${API_BASE}/nodes/positions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(positions),
  });
  return await res.json();
}

export async function applyAutoLayout(mode: number): Promise<{ count: number; hasUndo: boolean }> {
  const res = await fetch(`${API_BASE}/map/autolayout`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode }),
  });
  return await res.json();
}

export async function undoAutoLayout(): Promise<{ count: number; hasUndo: boolean }> {
  const res = await fetch(`${API_BASE}/map/autolayout/undo`, {
    method: "POST",
  });
  return await res.json();
}

export async function checkUndoAutoLayout(): Promise<{ hasUndo: boolean }> {
  const res = await fetch(`${API_BASE}/map/autolayout/undo`);
  return await res.json();
}

export async function fetchBackImage(): Promise<BackImageEnt> {
  const res = await fetch(`${API_BASE}/map/backimage`);
  return await res.json();
}

export async function saveBackImage(bi: BackImageEnt): Promise<BackImageEnt> {
  const res = await fetch(`${API_BASE}/map/backimage`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(bi),
  });
  return await res.json();
}

export async function deleteBackImage(): Promise<any> {
  const res = await fetch(`${API_BASE}/map/backimage`, {
    method: "DELETE",
  });
  return await res.json();
}

export async function uploadBackImage(file: File): Promise<{ path: string }> {
  const fd = new FormData();
  fd.append("image", file);
  const res = await fetch(`${API_BASE}/map/backimage/upload`, {
    method: "POST",
    body: fd,
  });
  return await res.json();
}

export async function importMapData(data: {
  nodes?: any[];
  lines?: any[];
  networks?: any[];
  drawItems?: any[];
}): Promise<any> {
  const res = await fetch(`${API_BASE}/map/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });
  return await res.json();
}

// Discovery Interfaces and APIs
export interface SnmpConfEnt {
  SnmpMode: string;
  Community: string;
  SnmpUser: string;
  SnmpPassword?: string;
}

export interface DiscoverConfEnt {
  StartIP: string;
  EndIP: string;
  Timeout: number;
  Retry: number;
  X: number;
  Y: number;
  AddPolling: boolean;
  PortScan: boolean;
  ReCheck: boolean;
  AddNetwork: boolean;
  AutoDetect: boolean;
  AutoDetectAI: boolean;
  AutoLine: number;
  AutoLayout: number;
  SnmpConfigs: SnmpConfEnt[];
}

export interface DiscoverStat {
  Running: boolean;
  Total: number;
  Sent: number;
  Found: number;
  Snmp: number;
  Web: number;
  Mail: number;
  SSH: number;
  File: number;
  RDP: number;
  LDAP: number;
  Wait: number;
  StartTime: number;
  Now: number;
}

export async function getDiscoverConf(): Promise<DiscoverConfEnt> {
  const res = await fetch(`${API_BASE}/discover/conf`);
  return await res.json();
}

export async function saveDiscoverConf(conf: DiscoverConfEnt): Promise<DiscoverConfEnt> {
  const res = await fetch(`${API_BASE}/discover/conf`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(conf),
  });
  return await res.json();
}

export async function startDiscover(conf: DiscoverConfEnt): Promise<{ ok: boolean }> {
  const res = await fetch(`${API_BASE}/discover/start`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(conf),
  });
  return await res.json();
}

export async function stopDiscover(): Promise<{ ok: boolean }> {
  const res = await fetch(`${API_BASE}/discover/stop`, {
    method: "POST",
  });
  return await res.json();
}

export async function getDiscoverStats(): Promise<DiscoverStat> {
  const res = await fetch(`${API_BASE}/discover/stat`);
  return await res.json();
}

export async function getDiscoverAddressRange(): Promise<string[]> {
  const res = await fetch(`${API_BASE}/discover/ranges`);
  return await res.json();
}

// RMON Statistics
export interface RmonEtherStatsEnt {
  Index: number;
  DataSource: string;
  DropEvents: number;
  Octets: number;
  Pkts: number;
  BroadcastPkts: number;
  MulticastPkts: number;
  CRCAlignErrors: number;
  UndersizePkts: number;
  OversizePkts: number;
  Fragments: number;
  Jabbers: number;
  Collisions: number;
  Pkts64Octets: number;
  Pkts65to127Octets: number;
  Pkts128to255Octets: number;
  Pkts256to511Octets: number;
  Pkts512to1023Octets: number;
  Pkts1024to1518Octets: number;
  Status: string;
}

export interface NodeRmonResp {
  supported: boolean;
  reason?: string;
  message?: string;
  error?: string;
  stats?: RmonEtherStatsEnt[];
}

export async function fetchNodeRmon(nodeId: string): Promise<NodeRmonResp> {
  try {
    const res = await fetch(`${API_BASE}/nodes/${encodeURIComponent(nodeId)}/rmon`);
    if (!res.ok) {
      return { supported: false, error: res.statusText, stats: [] };
    }
    return await res.json();
  } catch (err: any) {
    return { supported: false, error: err?.message || String(err), stats: [] };
  }
}

// Node Diagnostic Health Probes
export interface PingDiagnoseResult {
  success: boolean;
  sent: number;
  received: number;
  loss: number;
  avgRtt: number;
  error?: string;
}

export interface SNMPDiagnoseResult {
  configured: boolean;
  success: boolean;
  sysDescr?: string;
  sysUpTime?: string;
  rtt: number;
  error?: string;
}

export interface WebPortDiagnose {
  port: number;
  success: boolean;
  statusCode: number;
  rtt: number;
  certIssuer?: string;
  remainingDays?: number;
  error?: string;
}

export interface NodeDiagnoseResult {
  nodeId: string;
  nodeName: string;
  ip: string;
  status: "healthy" | "warning" | "critical";
  ping: PingDiagnoseResult;
  snmp: SNMPDiagnoseResult;
  web: {
    http?: WebPortDiagnose;
    https?: WebPortDiagnose;
  };
  summary: string;
  time: string;
}

export async function diagnoseNode(nodeId: string): Promise<NodeDiagnoseResult> {
  const res = await fetch(`${API_BASE}/nodes/${encodeURIComponent(nodeId)}/diagnose`, {
    method: "POST",
  });
  if (!res.ok) {
    throw new Error(`Diagnose failed: ${res.statusText}`);
  }
  return await res.json();
}

// Certificate Monitors (External TLS Endpoints)
export interface CertMonitorEnt {
  id: string;
  state: "normal" | "warn" | "error";
  target: string;
  port: number;
  subject: string;
  issuer: string;
  serialNumber: string;
  verify: boolean;
  notAfter: number;
  notBefore: number;
  error: string;
  firstTime: number;
  lastTime: number;
}

export async function fetchCertMonitors(): Promise<CertMonitorEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/cert_monitors`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function saveCertMonitor(ent: Partial<CertMonitorEnt>): Promise<CertMonitorEnt> {
  const res = await fetch(`${API_BASE}/cert_monitors`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(ent),
  });
  if (!res.ok) throw new Error(`Save cert monitor failed: ${res.statusText}`);
  return await res.json();
}

export async function deleteCertMonitor(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/cert_monitors/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(`Delete cert monitor failed: ${res.statusText}`);
}

export async function checkCertMonitors(): Promise<CertMonitorEnt[]> {
  const res = await fetch(`${API_BASE}/cert_monitors/check`, {
    method: "POST",
  });
  if (!res.ok) throw new Error(`Check cert monitors failed: ${res.statusText}`);
  return await res.json();
}

export interface LocConfEnt {
  Style: string;
  Center: string;
  Zoom: number;
  IconSize: number;
  Url?: string;
}

export async function fetchLocConf(): Promise<LocConfEnt> {
  try {
    const res = await fetch(`${API_BASE}/conf/loc`);
    if (!res.ok) throw new Error();
    return await res.json();
  } catch {
    return {
      Style: "osm",
      Center: "139.6917,35.6895",
      Zoom: 10,
      IconSize: 24,
    };
  }
}

export async function saveLocConf(conf: LocConfEnt): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/conf/loc`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(conf),
    });
    return res.ok;
  } catch {
    return false;
  }
}

export interface AIListEnt {
  ID: string;
  Node: string;
  Polling: string;
  Score: number;
  Count: number;
  LastTime: number;
}

export interface AIResultEnt {
  PollingID: string;
  ScoreData: [number, number][];
  LastTime: number;
}

export async function fetchAIList(): Promise<AIListEnt[]> {
  const res = await fetch(`${API_BASE}/ai/list`);
  if (!res.ok) {
    throw new Error(`Failed to fetch AI list: ${res.statusText}`);
  }
  return await res.json();
}

export async function fetchAIResult(id: string): Promise<AIResultEnt> {
  const res = await fetch(`${API_BASE}/ai/result/${encodeURIComponent(id)}`);
  if (!res.ok) {
    throw new Error(`Failed to fetch AI result: ${res.statusText}`);
  }
  return await res.json();
}

export async function deleteAIResult(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/ai/result/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Failed to delete AI result: ${res.statusText}`);
  }
}

export async function recheckAI(): Promise<AIListEnt[]> {
  const res = await fetch(`${API_BASE}/ai/recheck`, {
    method: "POST",
  });
  if (!res.ok) {
    throw new Error(`Failed to recheck AI: ${res.statusText}`);
  }
  return await res.json();
}

// Log-derived Reports Types & Functions
export interface RSSIEnt {
  Value: number;
  Time: number;
}

export interface ScoreInfo {
  Penalty: number;
  Score: number;
  ValidScore: boolean;
}

export interface WifiAPEnt {
  ID: string;
  Host: string;
  BSSID: string;
  SSID: string;
  Channel: string;
  Info: string;
  Vendor: string;
  Count: number;
  Change: number;
  RSSI: RSSIEnt[];
  FirstTime: number;
  LastTime: number;
}

export interface BlueDeviceEnt {
  ID: string;
  Host: string;
  Address: string;
  AddressType: string;
  Name: string;
  Vendor: string;
  Info: string;
  Count: number;
  RSSI: RSSIEnt[];
  FirstTime: number;
  LastTime: number;
}

export interface EnvDataEnt {
  Time: number;
  RSSI: number;
  Temp: number;
  Humidity: number;
  Illuminance: number;
  BarometricPressure: number;
  Sound: number;
  ETVOC: number;
  ECo2: number;
  Battery: number;
}

export interface EnvMonitorEnt {
  ID: string;
  Host: string;
  Address: string;
  Name: string;
  Count: number;
  EnvData: EnvDataEnt[];
  FirstTime: number;
  LastTime: number;
}

export interface PowerMonitorDataEnt {
  Time: number;
  Load: number;
  Switch: boolean;
  Over: boolean;
  RSSI: number;
}

export interface PowerMonitorEnt {
  ID: string;
  Host: string;
  Address: string;
  Name: string;
  Count: number;
  Data: PowerMonitorDataEnt[];
  FirstTime: number;
  LastTime: number;
}

export interface MotionSensorDataEnt {
  Time: number;
  Event: string;
  Moving: boolean;
  Light: boolean;
  Battery: number;
  LastMove: number;
  LastMoveDiff: number;
  RSSI: number;
}

export interface MotionSensorEnt {
  ID: string;
  Host: string;
  Address: string;
  Name: string;
  Count: number;
  Data: MotionSensorDataEnt[];
  FirstTime: number;
  LastTime: number;
}

export interface EtherTypeEnt {
  ID: string;
  Host: string;
  Type: string;
  Name: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export interface DNSQEnt {
  ID: string;
  Host: string;
  Type: string;
  Server: string;
  Name: string;
  Count: number;
  Change: number;
  LastClient: string;
  LastMAC: string;
  FirstTime: number;
  LastTime: number;
}

export interface RADIUSFlowEnt extends ScoreInfo {
  ID: string;
  Client: string;
  ClientName: string;
  ClientNodeID: string;
  Server: string;
  ServerName: string;
  ServerNodeID: string;
  Accept: number;
  Reject: number;
  Request: number;
  Challenge: number;
  Count: number;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export interface TLSFlowEnt extends ScoreInfo {
  ID: string;
  Client: string;
  ClientName: string;
  ClientNodeID: string;
  ClientLoc: string;
  Server: string;
  ServerName: string;
  ServerNodeID: string;
  ServerLoc: string;
  Service: string;
  Version: string;
  Cipher: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export interface WinEventIDEnt {
  ID: string;
  Level: string;
  Provider: string;
  EventID: number;
  Computer: string;
  Channel: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export interface WinLogonEnt extends ScoreInfo {
  ID: string;
  Target: string;
  Computer: string;
  IP: string;
  Count: number;
  Logon: number;
  Logoff: number;
  Failed: number;
  LogonType: Record<string, number>;
  FailedCode: Record<string, number>;
  FirstTime: number;
  LastTime: number;
}

export interface WinAccountEnt {
  ID: string;
  Subject: string;
  Target: string;
  Computer: string;
  Count: number;
  Edit: number;
  Password: number;
  Other: number;
  FirstTime: number;
  LastTime: number;
}

export interface WinKerberosEnt extends ScoreInfo {
  ID: string;
  Target: string;
  Computer: string;
  IP: string;
  Service: string;
  TicketType: string;
  Count: number;
  Failed: number;
  FirstTime: number;
  LastTime: number;
}

export interface WinPrivilegeEnt {
  ID: string;
  Subject: string;
  Computer: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export interface WinProcessEnt {
  ID: string;
  Process: string;
  Computer: string;
  Count: number;
  Start: number;
  Exit: number;
  LastSubject: string;
  LastParent: string;
  LastStatus: string;
  FirstTime: number;
  LastTime: number;
}

export interface WinTaskEnt {
  ID: string;
  TaskName: string;
  Computer: string;
  Subject: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export async function fetchLogReport<T = any>(kind: string): Promise<T[]> {
  try {
    const res = await fetch(`${API_BASE}/report/log/${encodeURIComponent(kind)}`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function resetLogReport(kind: string): Promise<void> {
  const res = await fetch(`${API_BASE}/report/log/${encodeURIComponent(kind)}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Reset log report failed: ${res.statusText}`);
  }
}

export async function deleteLogReportItem(kind: string, id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/report/log/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Delete log report item failed: ${res.statusText}`);
  }
}

export async function updateLogReportName(kind: string, id: string, name: string): Promise<void> {
  const res = await fetch(`${API_BASE}/report/log/name`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ Kind: kind, ID: id, Name: name }),
  });
  if (!res.ok) {
    throw new Error(`Update log report name failed: ${res.statusText}`);
  }
}

export interface SensorStatsEnt {
  Time: number; // unix nano
  Total: number;
  Count: number;
  PS: number;
  Send: number;
  LastSend: number;
}

export interface SensorMonitorEnt {
  Time: number; // unix nano
  CPU: number;
  Mem: number;
  Load: number;
  Process: number;
  Recv: number;
  Sent: number;
  TxSpeed: number;
  RxSpeed: number;
}

export interface SensorEnt {
  ID: string;
  Host: string;
  Type: string;
  Param: string;
  Total: number;
  Send: number;
  State: string;
  Ignore: boolean;
  Stats?: SensorStatsEnt[];
  Monitors?: SensorMonitorEnt[];
  StatsLen: number;
  MonitorsLen: number;
  FirstTime: number; // unix nano
  LastTime: number; // unix nano
}

export async function fetchSensors(): Promise<SensorEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/sensors`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchSensorStats(id: string): Promise<SensorStatsEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/sensor/stats/${encodeURIComponent(id)}`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchSensorMonitors(id: string): Promise<SensorMonitorEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/sensor/monitors/${encodeURIComponent(id)}`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function deleteSensor(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/report/sensor/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Delete sensor failed: ${res.statusText}`);
  }
}

export async function toggleSensor(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/report/sensor/${encodeURIComponent(id)}`, {
    method: "POST",
  });
  if (!res.ok) {
    throw new Error(`Toggle sensor failed: ${res.statusText}`);
  }
}

export interface FlowEnt {
  ID: string;
  Client: string;
  ClientName?: string;
  ClientNodeID?: string;
  ClientLoc?: string;
  Server: string;
  ServerName?: string;
  ServerNodeID?: string;
  ServerLoc?: string;
  Services: Record<string, number>;
  Count: number;
  Bytes: number;
  Packets: number;
  Duration: number;
  Penalty?: number;
  Score?: number;
  ValidScore?: boolean;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export interface ServerEnt {
  ID: string;
  Server: string;
  ServerName?: string;
  ServerNodeID?: string;
  Loc?: string;
  Services: Record<string, number>;
  Count: number;
  Bytes: number;
  Packets: number;
  Penalty?: number;
  Score?: number;
  ValidScore?: boolean;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export interface FumbleEnt {
  ID: string;
  TCPCount: number;
  IcmpCount: number;
  FirstTime: number;
  LastTime: number;
}

export interface SyslogHostStat {
  Host: string;
  NodeName?: string;
  Count: number;
  ErrorCount: number;
  WarnCount: number;
  NormalCount: number;
  FirstTime: number;
  LastTime: number;
}

export interface SyslogTagStat {
  Tag: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export interface SyslogStatsSummary {
  ID: string;
  Total: number;
  ErrorCount: number;
  WarnCount: number;
  NormalCount: number;
  Hosts: Record<string, SyslogHostStat>;
  Tags: Record<string, SyslogTagStat>;
  Facilities: Record<number, number>;
  Severities: Record<number, number>;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export interface TrapHostStat {
  Host: string;
  NodeName?: string;
  Count: number;
  ErrorCount: number;
  WarnCount: number;
  NormalCount: number;
  FirstTime: number;
  LastTime: number;
}

export interface TrapTypeStat {
  TrapType: string;
  Count: number;
  FirstTime: number;
  LastTime: number;
}

export interface TrapStatsSummary {
  ID: string;
  Total: number;
  ErrorCount: number;
  WarnCount: number;
  NormalCount: number;
  Hosts: Record<string, TrapHostStat>;
  Types: Record<string, TrapTypeStat>;
  Enterprises: Record<string, number>;
  FirstTime: number;
  LastTime: number;
  UpdateTime: number;
}

export async function fetchFlowReport(): Promise<FlowEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/flow`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchServerReport(): Promise<ServerEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/server`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function fetchFumbleReport(): Promise<FumbleEnt[]> {
  try {
    const res = await fetch(`${API_BASE}/report/fumble`);
    if (!res.ok) return [];
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function resetFlowReport(): Promise<void> {
  const res = await fetch(`${API_BASE}/report/flow`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Reset flow report failed: ${res.statusText}`);
  }
}

export async function fetchSyslogStats(): Promise<SyslogStatsSummary | null> {
  try {
    const res = await fetch(`${API_BASE}/report/syslog/stats`);
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export async function resetSyslogStats(): Promise<void> {
  const res = await fetch(`${API_BASE}/report/syslog/stats`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Reset syslog stats failed: ${res.statusText}`);
  }
}

export async function fetchTrapStats(): Promise<TrapStatsSummary | null> {
  try {
    const res = await fetch(`${API_BASE}/report/trap/stats`);
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

export async function resetTrapStats(): Promise<void> {
  const res = await fetch(`${API_BASE}/report/trap/stats`, {
    method: "DELETE",
  });
  if (!res.ok) {
    throw new Error(`Reset trap stats failed: ${res.statusText}`);
  }
}



