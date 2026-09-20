export interface NodeEnt {
  id: string;
  name: string;
  ip: string;
  mac?: string;
  icon?: string;
  state: string; // normal, warn, low, high, error, unknown
  x?: number;
  y?: number;
  descr?: string;
}

export interface LineEnt {
  id: string;
  node_id1: string;
  node_id2: string;
  state: string;
  width?: number;
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
}

export interface NetworkEnt {
  id: string;
  name: string;
  ip: string;
  x?: number;
  y?: number;
}

export interface EventLogEnt {
  time: number;
  type: string;
  level: string; // info, warn, low, high, error
  node_id?: string;
  node_name?: string;
  event: string;
}

export interface ParquetLogRecord {
  time: number;
  type: string;
  src: string;
  log: string;
}

export interface SystemHealth {
  status: string;
  time: string;
  version: string;
}

const API_BASE = '/api';

export async function fetchHealth(): Promise<SystemHealth> {
  const res = await fetch(`${API_BASE}/health`);
  if (!res.ok) throw new Error(`Health check failed: ${res.statusText}`);
  return res.json();
}

export async function fetchNodes(): Promise<NodeEnt[]> {
  const res = await fetch(`${API_BASE}/nodes`);
  if (!res.ok) throw new Error(`Fetch nodes failed: ${res.statusText}`);
  return res.json();
}

export async function saveNode(node: NodeEnt): Promise<NodeEnt> {
  const res = await fetch(`${API_BASE}/nodes`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(node),
  });
  if (!res.ok) throw new Error(`Save node failed: ${res.statusText}`);
  return res.json();
}

export async function deleteNode(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/nodes/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete node failed: ${res.statusText}`);
}

export async function fetchLines(): Promise<LineEnt[]> {
  const res = await fetch(`${API_BASE}/lines`);
  if (!res.ok) throw new Error(`Fetch lines failed: ${res.statusText}`);
  return res.json();
}

export async function saveLine(line: LineEnt): Promise<LineEnt> {
  const res = await fetch(`${API_BASE}/lines`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(line),
  });
  if (!res.ok) throw new Error(`Save line failed: ${res.statusText}`);
  return res.json();
}

export async function fetchPollings(): Promise<PollingEnt[]> {
  const res = await fetch(`${API_BASE}/pollings`);
  if (!res.ok) throw new Error(`Fetch pollings failed: ${res.statusText}`);
  return res.json();
}

export async function savePolling(poll: PollingEnt): Promise<PollingEnt> {
  const res = await fetch(`${API_BASE}/pollings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(poll),
  });
  if (!res.ok) throw new Error(`Save polling failed: ${res.statusText}`);
  return res.json();
}

export async function deletePolling(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/pollings/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(`Delete polling failed: ${res.statusText}`);
}

export async function fetchEventLogs(): Promise<EventLogEnt[]> {
  const res = await fetch(`${API_BASE}/logs/events`);
  if (!res.ok) throw new Error(`Fetch event logs failed: ${res.statusText}`);
  return res.json();
}

export async function queryParquetLogs(type = '', filter = ''): Promise<ParquetLogRecord[]> {
  const params = new URLSearchParams();
  if (type) params.set('type', type);
  if (filter) params.set('filter', filter);
  const res = await fetch(`${API_BASE}/logs/query?${params.toString()}`);
  if (!res.ok) throw new Error(`Query parquet logs failed: ${res.statusText}`);
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
