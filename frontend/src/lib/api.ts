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
  state: string;
  last_time?: number;
  last_val?: number;

  // Compatibility aliases
  ID?: string;
  NodeID?: string;
  Name?: string;
  Type?: string;
  State?: string;
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

  // Compatibility aliases
  ID?: string;
  Name?: string;
  IP?: string;
  X?: number;
  Y?: number;
  W?: number;
  H?: number;
  Ports?: PortEnt[];
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
  const target = raw.target || raw.Target || '';
  const last_time = raw.last_time || raw.LastTime || 0;

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
    state, State: state,
    target, Target: target,
    last_time, LastTime: last_time,
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

export async function fetchNotifyConf(): Promise<any> {
  const res = await fetch(`${API_BASE}/notify/conf`);
  if (!res.ok) throw new Error(`Fetch notify conf failed: ${res.statusText}`);
  return res.json();
}

export async function saveNotifyConf(conf: any): Promise<any> {
  const res = await fetch(`${API_BASE}/notify/conf`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(conf),
  });
  if (!res.ok) throw new Error(`Save notify conf failed: ${res.statusText}`);
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
    State: poll.state || poll.State || 'normal',
    Target: poll.target || (poll as any).Target || '',
    PollInt: (poll as any).poll_int || (poll as any).PollInt || 60,
    Timeout: (poll as any).timeout || (poll as any).Timeout || 1,
    Retry: (poll as any).retry || (poll as any).Retry || 1,
    Params: (poll as any).params || (poll as any).Params || '',
    Filter: (poll as any).filter || (poll as any).Filter || '',
    Extractor: (poll as any).extractor || (poll as any).Extractor || '',
    Script: (poll as any).script || (poll as any).Script || '',
  };
  if ((poll as any).Result || (poll as any).result) {
    payload.Result = (poll as any).Result || (poll as any).result;
  }
  if ((poll as any).LastTime || (poll as any).last_time) {
    payload.LastTime = (poll as any).LastTime || (poll as any).last_time;
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

export async function sendWol(mac: string, ip = '255.255.255.255'): Promise<{ status: string }> {
  const res = await fetch(`${API_BASE}/tools/wol`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mac, ip }),
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
  return await res.json();
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




