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
  value?: number;
  values?: number[];
  cond?: number;

  ID?: string;
  Type?: number;
  X?: number;
  Y?: number;
  W?: number;
  H?: number;
  Text?: string;
  Color?: string;
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

const API_BASE = '/api';

export function normalizeNode(raw: any): NodeEnt {
  if (!raw) return { id: '', name: '', ip: '', state: 'unknown', x: 300, y: 250 };
  const id = raw.id || raw.ID || '';
  const name = raw.name || raw.Name || 'Node';
  const ip = raw.ip || raw.IP || '';
  const mac = raw.mac || raw.MAC || '';
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
  const name = raw.name || raw.Name || 'ネットワーク';
  const ip = raw.ip || raw.IP || '';
  const x = typeof raw.x === 'number' ? raw.x : (typeof raw.X === 'number' ? raw.X : 300);
  const y = typeof raw.y === 'number' ? raw.y : (typeof raw.Y === 'number' ? raw.Y : 150);
  const w = typeof raw.w === 'number' ? raw.w : (typeof raw.W === 'number' ? raw.W : 420);
  const h = typeof raw.h === 'number' ? raw.h : (typeof raw.H === 'number' ? raw.H : 90);
  const h_ports = raw.h_ports || raw.HPorts || 8;

  const rawPorts = raw.ports || raw.Ports || [];
  const ports: PortEnt[] = rawPorts.map((p: any, idx: number) => {
    const pid = p.id || p.ID || `p${idx + 1}`;
    const pname = p.name || p.Name || `Port ${idx + 1}`;
    const px = typeof p.x === 'number' ? p.x : (typeof p.X === 'number' ? p.X : idx % h_ports);
    const py = typeof p.y === 'number' ? p.y : (typeof p.Y === 'number' ? p.Y : Math.floor(idx / h_ports));
    const pstate = p.state || p.State || 'none';
    return {
      id: pid, ID: pid,
      name: pname, Name: pname,
      x: px, X: px,
      y: py, Y: py,
      state: pstate, State: pstate,
    };
  });

  return {
    ...raw,
    id, ID: id,
    name, Name: name,
    ip, IP: ip,
    x, X: x,
    y, Y: y,
    w, W: w,
    h, H: h,
    h_ports,
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
  const text = raw.text || raw.Text || '';
  const color = raw.color || raw.Color || '#38bdf8';

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
  };
}

export function normalizePolling(raw: any): PollingEnt {
  if (!raw) return { id: '', node_id: '', name: '', type: 'ping', state: 'normal' };
  const id = raw.id || raw.ID || '';
  const node_id = raw.node_id || raw.NodeID || '';
  const name = raw.name || raw.Name || '';
  const type = raw.type || raw.Type || 'ping';
  const state = raw.state || raw.State || 'normal';

  return {
    ...raw,
    id, ID: id,
    node_id, NodeID: node_id,
    name, Name: name,
    type, Type: type,
    state, State: state,
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
  const rawPorts = net.ports || net.Ports || [];
  const ports = rawPorts.map((p: any, idx: number) => ({
    ID: p.id || p.ID || `p${idx + 1}`,
    Name: p.name || p.Name || `Port ${idx + 1}`,
    X: typeof p.x === 'number' ? p.x : (typeof p.X === 'number' ? p.X : idx % 8),
    Y: typeof p.y === 'number' ? p.y : (typeof p.Y === 'number' ? p.Y : Math.floor(idx / 8)),
    State: p.state || p.State || 'none',
  }));

  const payload = {
    ID: net.id || net.ID || '',
    Name: net.name || net.Name || '',
    IP: net.ip || net.IP || '',
    X: typeof net.x === 'number' ? net.x : (typeof net.X === 'number' ? net.X : 300),
    Y: typeof net.y === 'number' ? net.y : (typeof net.Y === 'number' ? net.Y : 150),
    W: typeof net.w === 'number' ? net.w : (typeof net.W === 'number' ? net.W : 420),
    H: typeof net.h === 'number' ? net.h : (typeof net.H === 'number' ? net.H : 90),
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

export async function fetchDrawItems(): Promise<DrawItemEnt[]> {
  const res = await fetch(`${API_BASE}/drawitems`);
  if (!res.ok) throw new Error(`Fetch draw items failed: ${res.statusText}`);
  const list = await res.json();
  return (Array.isArray(list) ? list : []).map(normalizeDrawItem);
}

export async function saveDrawItem(item: Partial<DrawItemEnt>): Promise<DrawItemEnt> {
  const payload = {
    ID: item.id || item.ID || '',
    Type: typeof item.type === 'number' ? item.type : (typeof item.Type === 'number' ? item.Type : 0),
    X: typeof item.x === 'number' ? item.x : (typeof item.X === 'number' ? item.X : 200),
    Y: typeof item.y === 'number' ? item.y : (typeof item.Y === 'number' ? item.Y : 200),
    W: typeof item.w === 'number' ? item.w : (typeof item.W === 'number' ? item.W : 120),
    H: typeof item.h === 'number' ? item.h : (typeof item.H === 'number' ? item.H : 40),
    Text: item.text || item.Text || '',
    Color: item.color || item.Color || '#38bdf8',
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

export async function deleteDrawItem(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/drawitems/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete draw item failed: ${res.statusText}`);
}

export async function fetchMapConf(): Promise<any> {
  const res = await fetch(`${API_BASE}/map/conf`);
  if (!res.ok) throw new Error(`Fetch map conf failed: ${res.statusText}`);
  return res.json();
}

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
  const payload = {
    ID: poll.id || poll.ID || '',
    NodeID: poll.node_id || poll.NodeID || '',
    Name: poll.name || poll.Name || '',
    Type: poll.type || poll.Type || 'ping',
    State: poll.state || poll.State || 'normal',
  };
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
