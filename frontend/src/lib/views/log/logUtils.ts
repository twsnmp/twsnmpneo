import type { EventLogEnt } from "../../api";
import { formatTimeStr, renderTimeMili, renderBytes, getSyslogType } from "../../common";
import type { ColumnDef, LogCategory, LogItem } from "./types";

export const parseJsonSafe = (str: string): any => {
  try {
    return JSON.parse(str);
  } catch {
    return null;
  }
};

export const formatCounterData = (dataVal: any): string => {
  if (!dataVal) return "-";
  try {
    let o = dataVal;
    if (typeof o === "string") {
      try {
        o = JSON.parse(o);
      } catch {
        return o;
      }
    }
    if (typeof o !== "object" || o === null) return String(o);
    const parts: string[] = [];
    for (const k of Object.keys(o)) {
      parts.push(`${k}=${o[k]}`);
    }
    return parts.join(" ");
  } catch {
    return String(dataVal);
  }
};

export const parseEventLogs = (logs: EventLogEnt[]): LogItem[] => {
  return (logs || []).map((el) => ({
    raw: el,
    time: el.time ?? (el as any).Time ?? 0,
    level: (el.level ?? (el as any).Level ?? "info").toLowerCase(),
    type: el.type ?? (el as any).Type ?? "",
    node: el.node_name ?? (el as any).NodeName ?? el.node_id ?? (el as any).NodeID ?? "-",
    event: el.event ?? (el as any).Event ?? "",
    fullText: `${el.event || ""} ${el.node_name || ""} ${el.type || ""} ${el.level || ""}`,
  }));
};

export const parseParquetLogs = (
  rawParquetLogs: any[],
  activeTab: LogCategory,
  sflowCounter: boolean
): LogItem[] => {
  return (rawParquetLogs || []).map((pl) => {
    const rawLog = pl.log ?? pl.Log ?? "";
    const parsed = parseJsonSafe(rawLog) || {};
    const time = pl.time ?? pl.Time ?? parsed.time ?? parsed.Time ?? 0;
    const src = pl.src ?? pl.Src ?? parsed.src ?? parsed.host ?? parsed.srcIP ?? parsed.IP ?? "";

    // Syslog
    let sv = typeof parsed.severity === "number" ? parsed.severity : (typeof parsed.Severity === "number" ? parsed.Severity : -1);
    let fac = typeof parsed.facility === "number" ? parsed.facility : (typeof parsed.Facility === "number" ? parsed.Facility : -1);
    let level = (parsed.level ?? parsed.Level ?? "").toLowerCase();
    if (sv >= 0) {
      if (sv < 3) level = "high";
      else if (sv < 4) level = "low";
      else if (sv === 4) level = "warn";
      else if (sv === 7) level = "debug";
      else level = "info";
    } else if (!level) {
      level = "info";
    }
    const host = parsed.hostname ?? parsed.Hostname ?? parsed.host ?? parsed.Host ?? src;
    const syslogType = parsed.type ?? parsed.Type ?? (sv >= 0 && fac >= 0 ? getSyslogType(sv, fac) : "");
    const tag = parsed.tag ?? parsed.Tag ?? parsed.app_name ?? parsed.appName ?? "";
    let message = parsed.content ?? parsed.Content ?? "";
    if (!message) {
      const parts = [parsed.proc_id, parsed.msg_id, parsed.message ?? parsed.Message, parsed.structured_data].filter(
        (x) => typeof x === "string" && x.length > 0 && x !== "-"
      );
      if (parts.length > 0) {
        message = parts.join(" ");
      } else {
        message = parsed.message ?? parsed.Message ?? rawLog;
      }
    }

    // Trap
    const trapType = parsed.TrapType ?? parsed.trapType ?? "";
    const trapFrom = parsed.FromAddress ?? parsed.fromAddress ?? parsed.srcIP ?? src;
    const variables = parsed.Variables ?? parsed.variables ?? "";

    // NetFlow
    const nfSrcAddr = parsed.SrcAddr ?? parsed.srcIP ?? src;
    const nfSrcPort = parsed.SrcPort ?? parsed.srcPort ?? 0;
    const nfSrcLoc = parsed.SrcLoc ?? parsed.srcLoc ?? "";
    const nfSrcMac = parsed.SrcMAC ?? parsed.srcMAC ?? "";
    const nfDstAddr = parsed.DstAddr ?? parsed.dstIP ?? "-";
    const nfDstPort = parsed.DstPort ?? parsed.dstPort ?? 0;
    const nfDstLoc = parsed.DstLoc ?? parsed.dstLoc ?? "";
    const nfDstMac = parsed.DstMAC ?? parsed.dstMAC ?? "";
    const nfProtocol = parsed.Protocol ?? parsed.protocol ?? "-";
    const nfTcpFlags = parsed.TCPFlags ?? parsed.tcpFlags ?? "";
    const nfPackets = parsed.Packets ?? parsed.packets ?? 0;
    const nfBytes = parsed.Bytes ?? parsed.bytes ?? 0;
    const nfDur = parsed.Dur ?? parsed.dur ?? 0;
    const dst = parsed.dstIP ? `${parsed.dstIP}:${parsed.dstPort || 0}` : (parsed.DstAddr ? `${parsed.DstAddr}:${parsed.DstPort || 0}` : "-");
    const netflowSrc = parsed.srcIP ? `${parsed.srcIP}:${parsed.srcPort || 0}` : (parsed.SrcAddr ? `${parsed.SrcAddr}:${parsed.SrcPort || 0}` : src);
    const protocol = nfProtocol;
    const packets = nfPackets;
    const bytes = nfBytes;
    const info = parsed.info ?? parsed.Info ?? rawLog;

    // ARP
    let state = parsed.state ?? parsed.State ?? "info";
    let ip = parsed.ip ?? parsed.IP ?? src;
    let node = parsed.node ?? parsed.Node ?? "-";
    let newMac = parsed.newMAC ?? parsed.NewMAC ?? "";
    let newVendor = parsed.newVendor ?? parsed.NewVendor ?? "";
    let oldMac = parsed.oldMAC ?? parsed.OldMAC ?? "";

    if (!newMac && rawLog.includes(",")) {
      const parts = rawLog.split(",");
      if (parts.length >= 3) {
        state = parts[0];
        ip = parts[1];
        newMac = parts[2];
        if (parts.length > 3) oldMac = parts[3];
      }
    }

    // MQTT
    const topic = parsed.topic ?? parsed.Topic ?? "";
    const client = parsed.clientID ?? parsed.ClientID ?? src;
    const payload = parsed.payload ?? parsed.Payload ?? rawLog;

    // OTel
    const scope = parsed.scope ?? parsed.service ?? parsed.Scope ?? "";

    // sFlow fields
    const sfReason = typeof parsed.Reason === "number" ? parsed.Reason : (typeof parsed.reason === "number" ? parsed.reason : 0);
    const sfRemote = parsed.Remote ?? parsed.remote ?? src;
    const sfCounterType = parsed.Type ?? parsed.type ?? "";
    let sfCounterData = parsed.Data ?? parsed.data ?? "";
    if (!sfCounterData && activeTab === "sflow" && sflowCounter) {
      sfCounterData = rawLog;
    }
    const formattedCounterData = formatCounterData(sfCounterData);

    return {
      raw: pl,
      time,
      src: activeTab === "trap" ? trapFrom : (activeTab === "netflow" ? nfSrcAddr : (activeTab === "sflow" ? (sflowCounter ? sfRemote : nfSrcAddr) : src)),
      level,
      host,
      type: activeTab === "syslog" ? syslogType : (activeTab === "sflow" && sflowCounter ? sfCounterType : (parsed.type ?? parsed.Type ?? "")),
      tag,
      message,
      trapType,
      variables,
      dst,
      netflowSrc,
      srcAddr: nfSrcAddr,
      srcPort: nfSrcPort,
      srcLoc: nfSrcLoc,
      srcMac: nfSrcMac,
      dstAddr: nfDstAddr,
      dstPort: nfDstPort,
      dstLoc: nfDstLoc,
      dstMac: nfDstMac,
      tcpFlags: nfTcpFlags,
      dur: nfDur,
      protocol,
      packets,
      bytes,
      reason: sfReason,
      remote: sfRemote,
      counterType: sfCounterType,
      counterData: sfCounterData,
      info,
      state,
      ip,
      node,
      newMac,
      newVendor,
      oldMac,
      topic,
      client,
      payload,
      scope,
      log: rawLog,
      fullText: `${rawLog} ${src} ${tag} ${host} ${syslogType} ${topic} ${ip} ${nfSrcAddr} ${nfDstAddr} ${sfRemote} ${sfCounterType} ${formattedCounterData}`,
    };
  });
};

export const getCategoryColumns = (t: (key: string) => string): Record<string, ColumnDef[]> => ({
  event: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "level", label: t('log.col.level'), width: "w-28", align: "center", sortable: true },
    { key: "type", label: t('log.col.type'), width: "w-28", sortable: true },
    { key: "node", label: t('log.col.node'), width: "w-48", sortable: true },
    { key: "event", label: t('log.col.event'), sortable: true },
  ],
  syslog: [
    { key: "level", label: t('log.col.level'), width: "w-24", align: "center", sortable: true },
    { key: "time", label: t('log.col.time'), width: "w-48", sortable: true },
    { key: "host", label: t('log.col.host'), width: "w-40", sortable: true },
    { key: "type", label: t('log.col.type'), width: "w-28", sortable: true },
    { key: "tag", label: t('log.col.tag'), width: "w-32", sortable: true },
    { key: "message", label: t('log.col.message'), sortable: true },
  ],
  trap: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "src", label: t('log.col.src'), width: "w-48", sortable: true },
    { key: "trapType", label: t('log.col.trapType'), width: "w-40", sortable: true },
    { key: "variables", label: t('log.col.variables'), sortable: true },
  ],
  netflow: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "srcAddr", label: t('log.col.srcAddr'), width: "w-40", sortable: true },
    { key: "srcPort", label: t('log.col.srcPort'), width: "w-16", align: "right", sortable: true },
    { key: "srcLoc", label: t('log.col.srcLoc'), width: "w-28", sortable: true },
    { key: "srcMac", label: t('log.col.srcMac'), width: "w-36", sortable: true },
    { key: "dstAddr", label: t('log.col.dstAddr'), width: "w-40", sortable: true },
    { key: "dstPort", label: t('log.col.dstPort'), width: "w-16", align: "right", sortable: true },
    { key: "dstLoc", label: t('log.col.dstLoc'), width: "w-28", sortable: true },
    { key: "dstMac", label: t('log.col.dstMac'), width: "w-36", sortable: true },
    { key: "protocol", label: t('log.col.protocol'), width: "w-20", align: "center", sortable: true },
    { key: "tcpFlags", label: t('log.col.tcpFlags'), width: "w-24", sortable: true },
    { key: "packets", label: t('log.col.packets'), width: "w-20", align: "right", sortable: true },
    { key: "bytes", label: t('log.col.bytes'), width: "w-24", align: "right", sortable: true },
    { key: "dur", label: t('log.col.dur'), width: "w-20", align: "right", sortable: true },
  ],
  sflow: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "srcAddr", label: t('log.col.srcAddr'), width: "w-36", sortable: true },
    { key: "srcPort", label: t('log.col.srcPort'), width: "w-16", align: "right", sortable: true },
    { key: "srcLoc", label: t('log.col.srcLoc'), width: "w-28", sortable: true },
    { key: "srcMac", label: t('log.col.srcMac'), width: "w-36", sortable: true },
    { key: "dstAddr", label: t('log.col.dstAddr'), width: "w-36", sortable: true },
    { key: "dstPort", label: t('log.col.dstPort'), width: "w-16", align: "right", sortable: true },
    { key: "dstLoc", label: t('log.col.dstLoc'), width: "w-28", sortable: true },
    { key: "dstMac", label: t('log.col.dstMac'), width: "w-36", sortable: true },
    { key: "protocol", label: t('log.col.protocol'), width: "w-20", align: "center", sortable: true },
    { key: "tcpFlags", label: t('log.col.tcpFlags'), width: "w-24", sortable: true },
    { key: "bytes", label: t('log.col.bytes'), width: "w-24", align: "right", sortable: true },
    { key: "reason", label: t('log.col.reason'), width: "w-28", align: "right", sortable: true },
  ],
  sflowCounter: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "remote", label: t('log.col.srcAddr'), width: "w-36", sortable: true },
    { key: "counterType", label: t('log.col.counterType'), width: "w-44", sortable: true },
    { key: "counterData", label: t('log.col.counterData'), width: "w-44", sortable: true },
  ],
  arp: [
    { key: "time", label: t('log.col.time'), width: "w-44", sortable: true },
    { key: "state", label: t('log.col.state'), width: "w-24", align: "center", sortable: true },
    { key: "ip", label: t('log.col.ip'), width: "w-36", sortable: true },
    { key: "node", label: t('log.col.node'), width: "w-40", sortable: true },
    { key: "newMac", label: t('log.col.newMac'), width: "w-36", sortable: true },
    { key: "newVendor", label: t('log.col.newVendor'), width: "w-36", sortable: true },
    { key: "oldMac", label: t('log.col.oldMac'), width: "w-36", sortable: true },
  ],
});

export const getLevelBadge = (level: string): string => {
  switch (level?.toLowerCase()) {
    case "normal":
      return "bg-emerald-100 text-emerald-800 border-emerald-300 dark:bg-emerald-500/10 dark:text-emerald-400 dark:border-emerald-500/30";
    case "warn":
    case "low":
      return "bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30";
    case "high":
    case "error":
    case "change":
      return "bg-rose-100 text-rose-800 border-rose-300 dark:bg-rose-500/10 dark:text-rose-400 dark:border-rose-500/30";
    case "info":
    case "new":
      return "bg-sky-100 text-sky-800 border-sky-300 dark:bg-sky-500/10 dark:text-sky-400 dark:border-sky-500/30";
    default:
      return "bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700";
  }
};

export const exportLogsCSV = (
  sortedLogs: LogItem[],
  visibleColumns: ColumnDef[],
  activeTab: LogCategory,
  sflowCounter: boolean
): void => {
  const tabName = activeTab === "sflow" && sflowCounter ? "sflow_counter" : activeTab;
  const filename = `twsnmp_${tabName}_logs_${Date.now()}.csv`;
  const cols = visibleColumns;
  let csv = "\uFEFF" + cols.map((c) => `"${c.label}"`).join(",") + "\n";

  csv += sortedLogs
    .map((row: any) =>
      cols
        .map((c) => {
          let val = row[c.key];
          if (c.key === "time") {
            val = (activeTab === "syslog" || activeTab === "netflow" || activeTab === "sflow")
              ? renderTimeMili(val)
              : formatTimeStr(val);
          }
          if (c.key === "bytes" && typeof val === "number") val = renderBytes(val);
          if (c.key === "dur" && typeof val === "number") val = val === 0 ? "0" : val.toFixed(2);
          if (c.key === "counterData") val = formatCounterData(val);
          return `"${String(val ?? "").replace(/"/g, '""')}"`;
        })
        .join(",")
    )
    .join("\n");

  const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = filename;
  link.click();
};
