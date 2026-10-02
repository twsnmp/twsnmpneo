export type LogCategory = "event" | "syslog" | "trap" | "netflow" | "sflow" | "arp";

export interface ColumnDef {
  key: string;
  label: string;
  width?: string;
  align?: "left" | "center" | "right";
  sortable?: boolean;
}

export interface FilterState {
  start: string;
  end: string;
  level: string;
  type: string;
  source: string;
  keyword: string;
  single?: boolean;
  srcPort?: string;
  dstAddr?: string;
  dstPort?: string;
  protocol?: string;
  tcpFlags?: string;
  mac?: string;
  state?: string;
}

export interface LogItem {
  raw: any;
  time: number;
  level: string;
  type?: string;
  node?: string;
  event?: string;
  src?: string;
  host?: string;
  tag?: string;
  message?: string;
  trapType?: string;
  enterprise?: string;
  variables?: string;
  dst?: string;
  netflowSrc?: string;
  srcAddr?: string;
  srcPort?: number;
  srcLoc?: string;
  srcMac?: string;
  dstAddr?: string;
  dstPort?: number;
  dstLoc?: string;
  dstMac?: string;
  tcpFlags?: string;
  dur?: number;
  protocol?: string;
  packets?: number;
  bytes?: number;
  reason?: number;
  remote?: string;
  counterType?: string;
  counterData?: string;
  info?: string;
  state?: string;
  ip?: string;
  newMac?: string;
  newVendor?: string;
  oldMac?: string;
  topic?: string;
  client?: string;
  payload?: string;
  scope?: string;
  log?: string;
  fullText: string;
  [key: string]: any;
}
