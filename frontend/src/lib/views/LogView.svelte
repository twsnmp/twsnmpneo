<script lang="ts">
  import { onMount, tick } from "svelte";
  import {
    fetchEventLogs,
    queryParquetLogs,
    getLogCounts,
    deleteEventLogs,
    deleteParquetLogs,
    askAI,
    type EventLogEnt,
    type ParquetLogRecord,
  } from "../api";
  import {
    getStateColor,
    getStateName,
    formatTimeStr,
    renderTimeMili,
    renderDuration,
    renderBytes,
    getSyslogType,
  } from "../common";
  import { showLogLevelChart, resizeLogLevelChart, disposeLogLevelChart } from "../charts/loglevel";
  import { showLogCountChart, resizeLogCountChart, disposeLogCountChart } from "../charts/logcount";
  import LogFilterModal from "../components/LogFilterModal.svelte";
  import LogReportModal from "../components/LogReportModal.svelte";
  import {
    Activity,
    AlertTriangle,
    ArrowDown,
    ArrowUp,
    ArrowUpDown,
    BarChart3,
    Check,
    ChevronDown,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    Columns,
    Download,
    FileText,
    Filter,
    Radio,
    RefreshCw,
    RotateCcw,
    Search,
    Server,
    Sparkles,
    Trash2,
    X,
  } from "@lucide/svelte";

  type LogCategory = "event" | "syslog" | "trap" | "netflow" | "sflow" | "arp" | "otel" | "mqtt";

  let activeTab = $state<LogCategory>("event");
  let eventLogs = $state<EventLogEnt[]>([]);
  let rawParquetLogs = $state<any[]>([]);
  let logCounts = $state<Record<string, number>>({});
  let loading = $state(false);

  // Reception Chart state
  let showChart = $state(true);
  let chartZoomRange = $state<{ st: number; et: number } | null>(null);

  // Search & Filter state
  let searchQuery = $state("");
  let fetchLimit = $state(10000);
  let showFilterModal = $state(false);
  let filterState = $state({
    start: "",
    end: "",
    level: "all",
    type: "",
    source: "",
    keyword: "",
  });

  // Report Modal state
  let showReportModal = $state(false);

  // AI Dialog state
  let showAIDialog = $state(false);
  let selectedLogText = $state("");
  let aiAnswer = $state("");
  let aiLoading = $state(false);

  // Column Visibility state (Category -> Set of column keys)
  let showColumnMenu = $state(false);
  let columnVisibility = $state<Record<string, boolean>>({});

  // Pagination state
  let currentPage = $state(1);
  let pageSize = $state(25);

  // Sorting state
  let sortColumn = $state("time");
  let sortDirection = $state<"asc" | "desc">("desc");

  // Categories definition
  const categories: { id: LogCategory; name: string; icon: any }[] = [
    { id: "event", name: "イベントログ", icon: FileText },
    { id: "syslog", name: "Syslog", icon: Server },
    { id: "trap", name: "SNMP TRAP", icon: AlertTriangle },
    { id: "netflow", name: "NetFlow", icon: BarChart3 },
    { id: "sflow", name: "sFlow", icon: Activity },
    { id: "arp", name: "ARP Watch", icon: Server },
    { id: "otel", name: "OpenTelemetry", icon: Activity },
    { id: "mqtt", name: "MQTT", icon: Radio },
  ];

  // Column Definitions per Category
  interface ColumnDef {
    key: string;
    label: string;
    width?: string;
    align?: "left" | "center" | "right";
    sortable?: boolean;
  }

  const categoryColumns: Record<LogCategory, ColumnDef[]> = {
    event: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "level", label: "レベル", width: "w-28", align: "center", sortable: true },
      { key: "type", label: "種別", width: "w-28", sortable: true },
      { key: "node", label: "ノード名", width: "w-48", sortable: true },
      { key: "event", label: "イベント内容", sortable: true },
    ],
    syslog: [
      { key: "level", label: "レベル", width: "w-24", align: "center", sortable: true },
      { key: "time", label: "日時", width: "w-48", sortable: true },
      { key: "host", label: "ホスト", width: "w-40", sortable: true },
      { key: "type", label: "タイプ", width: "w-28", sortable: true },
      { key: "tag", label: "タグ", width: "w-32", sortable: true },
      { key: "message", label: "メッセージ", sortable: true },
    ],
    trap: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "src", label: "送信元 (From)", width: "w-48", sortable: true },
      { key: "trapType", label: "TRAP種別", width: "w-40", sortable: true },
      { key: "variables", label: "変数 (Variables)", sortable: true },
    ],
    netflow: [
      { key: "time", label: "Time", width: "w-44", sortable: true },
      { key: "srcAddr", label: "Src Addr", width: "w-40", sortable: true },
      { key: "srcPort", label: "Port", width: "w-16", align: "right", sortable: true },
      { key: "srcLoc", label: "Location", width: "w-28", sortable: true },
      { key: "srcMac", label: "MAC", width: "w-36", sortable: true },
      { key: "dstAddr", label: "Dst Addr", width: "w-40", sortable: true },
      { key: "dstPort", label: "Port", width: "w-16", align: "right", sortable: true },
      { key: "dstLoc", label: "Location", width: "w-28", sortable: true },
      { key: "dstMac", label: "MAC", width: "w-36", sortable: true },
      { key: "protocol", label: "Protocol", width: "w-20", align: "center", sortable: true },
      { key: "tcpFlags", label: "TCP Flags", width: "w-24", sortable: true },
      { key: "packets", label: "Packets", width: "w-20", align: "right", sortable: true },
      { key: "bytes", label: "Bytes", width: "w-24", align: "right", sortable: true },
      { key: "dur", label: "Duration", width: "w-20", align: "right", sortable: true },
    ],
    sflow: [
      { key: "time", label: "Time", width: "w-44", sortable: true },
      { key: "srcAddr", label: "Src Addr", width: "w-36", sortable: true },
      { key: "srcPort", label: "Port", width: "w-16", align: "right", sortable: true },
      { key: "srcLoc", label: "Location", width: "w-28", sortable: true },
      { key: "srcMac", label: "MAC", width: "w-36", sortable: true },
      { key: "dstAddr", label: "Dst Addr", width: "w-36", sortable: true },
      { key: "dstPort", label: "Port", width: "w-16", align: "right", sortable: true },
      { key: "dstLoc", label: "Location", width: "w-28", sortable: true },
      { key: "dstMac", label: "MAC", width: "w-36", sortable: true },
      { key: "protocol", label: "Protocol", width: "w-20", align: "center", sortable: true },
      { key: "tcpFlags", label: "TCP Flags", width: "w-24", sortable: true },
      { key: "bytes", label: "Bytes", width: "w-24", align: "right", sortable: true },
      { key: "reason", label: "Discard reason", width: "w-28", align: "right", sortable: true },
    ],
    sflowCounter: [
      { key: "time", label: "Time", width: "w-44", sortable: true },
      { key: "remote", label: "Src Addr", width: "w-36", sortable: true },
      { key: "counterType", label: "Type", width: "w-44", sortable: true },
      { key: "counterData", label: "Data", sortable: true },
    ],
    arp: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "state", label: "状態", width: "w-24", align: "center", sortable: true },
      { key: "ip", label: "IPアドレス", width: "w-36", sortable: true },
      { key: "node", label: "ノード名", width: "w-40", sortable: true },
      { key: "newMac", label: "新MACアドレス", width: "w-36", sortable: true },
      { key: "newVendor", label: "新ベンダー", width: "w-36", sortable: true },
      { key: "oldMac", label: "旧MACアドレス", width: "w-36", sortable: true },
    ],
    otel: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "src", label: "送信元", width: "w-40", sortable: true },
      { key: "scope", label: "スコープ/サービス", width: "w-44", sortable: true },
      { key: "log", label: "テレメトリデータ", sortable: true },
    ],
    mqtt: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "topic", label: "トピック", width: "w-48", sortable: true },
      { key: "client", label: "クライアントID", width: "w-40", sortable: true },
      { key: "payload", label: "ペイロード", sortable: true },
    ],
  };

  let sflowCounter = $state(false);

  const currentColumns = $derived(
    activeTab === "sflow" && sflowCounter
      ? (categoryColumns as any)["sflowCounter"]
      : categoryColumns[activeTab] || []
  );

  const visibleColumns = $derived(
    currentColumns.filter((col: ColumnDef) => {
      const colTab = activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab;
      const key = `${colTab}_${col.key}`;
      return columnVisibility[key] !== false;
    })
  );

  const toggleColumn = (key: string) => {
    const colTab = activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab;
    const colKey = `${colTab}_${key}`;
    columnVisibility[colKey] = columnVisibility[colKey] === false ? true : false;
  };

  const refreshCounts = async () => {
    try {
      const counts = await getLogCounts();
      if (counts) logCounts = counts;
    } catch (e) {
      console.error(e);
    }
  };

  const parseJsonSafe = (str: string) => {
    try {
      return JSON.parse(str);
    } catch {
      return null;
    }
  };

  const formatCounterData = (dataVal: any) => {
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

  const loadCurrentLogs = async () => {
    loading = true;
    try {
      refreshCounts();
      if (activeTab === "event") {
        let startTime = 0;
        let endTime = 0;
        if (filterState.start) startTime = new Date(filterState.start).getTime() * 1e6;
        if (filterState.end) endTime = new Date(filterState.end).getTime() * 1e6;

        eventLogs = await fetchEventLogs({
          start: startTime || undefined,
          end: endTime || undefined,
          level: filterState.level !== "all" ? filterState.level : undefined,
          type: filterState.type || undefined,
          nodeName: filterState.source || undefined,
          filter: filterState.keyword || undefined,
          limit: fetchLimit,
        });
        logCounts["event"] = eventLogs.length;
      } else {
        let startTime = 0;
        let endTime = 0;
        if (filterState.start) startTime = new Date(filterState.start).getTime() * 1e6;
        if (filterState.end) endTime = new Date(filterState.end).getTime() * 1e6;

        const queryType = activeTab === "arp" ? "arplog" : (activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab);
        const pq = await queryParquetLogs({
          type: queryType,
          src: filterState.source || undefined,
          filter: filterState.keyword || undefined,
          start: startTime || undefined,
          end: endTime || undefined,
          limit: fetchLimit,
        });
        rawParquetLogs = pq;
      }
      currentPage = 1;
      await renderReceptionChart();
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  interface LogItem {
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

  // Structured Item Converter
  const structuredLogs = $derived<LogItem[]>(
    activeTab === "event"
      ? (eventLogs || []).map((el) => ({
          raw: el,
          time: el.time ?? (el as any).Time ?? 0,
          level: (el.level ?? (el as any).Level ?? "info").toLowerCase(),
          type: el.type ?? (el as any).Type ?? "",
          node: el.node_name ?? (el as any).NodeName ?? el.node_id ?? (el as any).NodeID ?? "-",
          event: el.event ?? (el as any).Event ?? "",
          fullText: `${el.event || ""} ${el.node_name || ""} ${el.type || ""} ${el.level || ""}`,
        }))
      : (rawParquetLogs || []).map((pl) => {
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
            const parts = [parsed.proc_id, parsed.msg_id, parsed.message ?? parsed.Message, parsed.structured_data].filter((x) => typeof x === "string" && x.length > 0 && x !== "-");
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
          const state = parsed.state ?? parsed.State ?? "info";
          const ip = parsed.ip ?? parsed.IP ?? src;
          const node = parsed.node ?? parsed.Node ?? "-";
          const newMac = parsed.newMAC ?? parsed.NewMAC ?? "";
          const newVendor = parsed.newVendor ?? parsed.NewVendor ?? "";
          const oldMac = parsed.oldMAC ?? parsed.OldMAC ?? "";

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
        })
  );

  // Filtered Logs
  const filteredLogs = $derived(
    structuredLogs.filter((item) => {
      // 1. Chart zoom range filter
      if (chartZoomRange && chartZoomRange.st > 0 && chartZoomRange.et > 0) {
        const itemTime = item.time;
        if (itemTime < chartZoomRange.st || itemTime > chartZoomRange.et) {
          return false;
        }
      }

      // 2. Incremental search filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        if (!item.fullText.toLowerCase().includes(q)) {
          return false;
        }
      }

      // 3. Level filter
      if (filterState.level !== "all" && item.level) {
        if (item.level.toLowerCase() !== filterState.level.toLowerCase()) {
          return false;
        }
      }

      return true;
    })
  );

  // Sorted Logs
  const sortedLogs = $derived(
    [...filteredLogs].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];

      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";

      let comparison = 0;
      if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }

      return sortDirection === "asc" ? comparison : -comparison;
    })
  );

  // Pagination Slice
  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedLogs.length / pageSize))
  );

  const paginatedLogs = $derived(
    pageSize === -1
      ? sortedLogs
      : sortedLogs.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  // Render Reception Chart
  const renderReceptionChart = async () => {
    await tick();
    if (!showChart) return;
    const container = document.getElementById("logReceptionChart");
    if (!container) return;

    if (activeTab === "event" || activeTab === "syslog") {
      showLogLevelChart(container, structuredLogs, (st, et) => {
        if (st && et) {
          chartZoomRange = { st, et };
        } else {
          chartZoomRange = null;
        }
      });
    } else {
      showLogCountChart(container, structuredLogs, (st, et) => {
        if (st && et) {
          chartZoomRange = { st, et };
        } else {
          chartZoomRange = null;
        }
      });
    }
  };

  const handleAskAI = async (logText: string) => {
    selectedLogText = logText;
    showAIDialog = true;
    aiLoading = true;
    aiAnswer = "";
    try {
      aiAnswer = await askAI(
        `以下のネットワークログを解析し、根本原因、重大度、および推奨される復旧対策を専門家として簡潔に説明してください。\n\nログ:\n${logText}`,
        "ネットワークインフラのトラブルシューティング支援AI"
      );
    } catch (e: any) {
      aiAnswer = `AI解析エラー: ${e.message}`;
    } finally {
      aiLoading = false;
    }
  };

  const handleDeleteAll = async () => {
    if (!confirm(`本当にすべての ${activeTab.toUpperCase()} ログを削除しますか？この操作は元に戻せません。`)) {
      return;
    }
    try {
      if (activeTab === "event") {
        await deleteEventLogs();
      } else if (activeTab === "sflow") {
        await deleteParquetLogs("sflow");
        await deleteParquetLogs("sflowCounter");
      } else {
        await deleteParquetLogs(activeTab === "arp" ? "arplog" : activeTab);
      }
      chartZoomRange = null;
      loadCurrentLogs();
    } catch (e) {
      alert(`削除に失敗しました: ${e}`);
    }
  };

  const exportCSV = () => {
    const tabName = activeTab === "sflow" && sflowCounter ? "sflow_counter" : activeTab;
    const filename = `twsnmp_${tabName}_logs_${Date.now()}.csv`;
    const cols = visibleColumns;
    let csv = "\uFEFF" + cols.map((c) => `"${c.label}"`).join(",") + "\n";

    csv += sortedLogs
      .map((row: any) =>
        cols
          .map((c) => {
            let val = row[c.key];
            if (c.key === "time") val = (activeTab === "syslog" || activeTab === "netflow" || activeTab === "sflow") ? renderTimeMili(val) : formatTimeStr(val);
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

  const exportExcel = () => {
    const tabName = activeTab === "sflow" && sflowCounter ? "sflow_counter" : activeTab;
    const filename = `twsnmp_${tabName}_logs_${Date.now()}.csv`;
    exportCSV();
  };

  const getLevelBadge = (level: string) => {
    switch (level?.toLowerCase()) {
      case "normal":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "warn":
      case "low":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "high":
      case "error":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      case "info":
        return "bg-sky-500/10 text-sky-400 border-sky-500/30";
      default:
        return "bg-slate-800 text-slate-400 border-slate-700";
    }
  };

  onMount(() => {
    loadCurrentLogs();
    const handleResize = () => {
      resizeLogLevelChart();
      resizeLogCountChart();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      disposeLogLevelChart();
      disposeLogCountChart();
    };
  });

  $effect(() => {
    activeTab;
    chartZoomRange = null;
    loadCurrentLogs();
  });
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Left Sidebar (Matching ListView / ReportView) -->
  <div class="w-60 border-r border-slate-800 bg-slate-950/80 p-3 space-y-1.5 shrink-0 flex flex-col justify-between">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
        ログ種別 (Log Type)
      </div>

      {#each categories as cat}
        {@const count = logCounts[cat.id] ?? (activeTab === cat.id ? structuredLogs.length : 0)}
        <button
          type="button"
          onclick={() => {
            activeTab = cat.id;
            searchQuery = "";
            currentPage = 1;
            sortColumn = "time";
            sortDirection = "desc";
          }}
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
        >
          <div class="flex items-center gap-2.5 truncate">
            <cat.icon class="h-4 w-4 shrink-0 {activeTab === cat.id ? 'text-white' : 'text-cyan-400'}" />
            <span class="truncate">{cat.name}</span>
          </div>
          <span class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeTab === cat.id ? 'bg-white/20 text-white' : 'bg-slate-800 text-slate-400'}">
            {count.toLocaleString()}
          </span>
        </button>
      {/each}
    </div>

    <!-- Live Status & Stats Card -->
    <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-3 text-[11px] text-slate-400 space-y-2">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-200">取得制限</span>
        <select
          bind:value={fetchLimit}
          onchange={loadCurrentLogs}
          class="rounded-lg border border-slate-700 bg-slate-950 px-2 py-0.5 text-[10px] text-cyan-400 font-mono focus:outline-none cursor-pointer"
        >
          <option value={1000}>1,000 件</option>
          <option value={5000}>5,000 件</option>
          <option value={10000}>10,000 件</option>
          <option value={20000}>20,000 件</option>
        </select>
      </div>
      <div class="text-[10px] font-mono text-slate-400 pt-1 border-t border-slate-800/80 flex items-center justify-between">
        <span>ヒット件数:</span>
        <span class="text-cyan-400 font-bold">{filteredLogs.length.toLocaleString()} 件</span>
      </div>
    </div>
  </div>

  <!-- Right Main Content Canvas -->
  <div class="flex-1 overflow-hidden flex flex-col p-4 gap-3 min-w-0">
    <!-- Top Reception Status Graph (Collapsible) -->
    {#if showChart}
      <div class="relative rounded-2xl border border-slate-800 bg-slate-900/90 p-3 shadow-lg shrink-0 transition-all">
        <div class="flex items-center justify-between mb-1 px-1">
          <div class="flex items-center gap-2">
            <span class="text-[11px] font-bold text-slate-300">受信状況推移 (時系列グラフ)</span>
            {#if chartZoomRange}
              <span class="inline-flex items-center gap-1 rounded-full bg-cyan-950/80 border border-cyan-800 px-2 py-0.5 text-[10px] text-cyan-300 font-mono">
                期間絞り込み適用中
              </span>
              <button
                type="button"
                onclick={() => (chartZoomRange = null)}
                class="flex items-center gap-1 rounded-md bg-slate-800 hover:bg-slate-700 px-2 py-0.5 text-[10px] text-slate-300 cursor-pointer"
              >
                <RotateCcw class="h-3 w-3" />
                <span>全期間に戻す</span>
              </button>
            {/if}
          </div>
          <button
            type="button"
            onclick={() => (showChart = false)}
            class="text-[10px] text-slate-400 hover:text-slate-200 cursor-pointer"
          >
            グラフを隠す ▲
          </button>
        </div>
        <div id="logReceptionChart" class="h-64 min-h-[250px] w-full"></div>
      </div>
    {:else}
      <div class="flex justify-end shrink-0">
        <button
          type="button"
          onclick={() => {
            showChart = true;
            renderReceptionChart();
          }}
          class="flex items-center gap-1 text-[11px] text-cyan-400 hover:text-cyan-300 cursor-pointer"
        >
          <BarChart3 class="h-3.5 w-3.5" />
          <span>受信状況推移グラフを表示 ▼</span>
        </button>
      </div>
    {/if}

    <!-- Action Bar: Search, Filters, Column Selector, Report, Export -->
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-800 bg-slate-900/90 p-3 shadow-md shrink-0">
      <!-- Search & Filters -->
      <div class="flex items-center gap-2.5">
        <!-- Quick Text Search Input -->
        <div class="relative w-64">
          <Search class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="イベント・メッセージを検索..."
            bind:value={searchQuery}
            class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-8 pr-7 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
          />
          {#if searchQuery}
            <button
              type="button"
              onclick={() => (searchQuery = "")}
              class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-white cursor-pointer"
            >
              <X class="h-3.5 w-3.5" />
            </button>
          {/if}
        </div>

        <!-- Filter Modal Button -->
        <button
          type="button"
          onclick={() => (showFilterModal = true)}
          class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
        >
          <Filter class="h-3.5 w-3.5 text-cyan-400" />
          <span>詳細フィルター</span>
          {#if filterState.start || filterState.end || filterState.level !== "all" || filterState.type || filterState.source || filterState.keyword}
            <span class="h-2 w-2 rounded-full bg-cyan-400"></span>
          {/if}
        </button>

        <!-- Column Visibility Toggle Dropdown -->
        <div class="relative">
          <button
            type="button"
            onclick={() => (showColumnMenu = !showColumnMenu)}
            class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
          >
            <Columns class="h-3.5 w-3.5 text-slate-400" />
            <span>列の選択</span>
            <ChevronDown class="h-3 w-3 text-slate-400" />
          </button>

          {#if showColumnMenu}
            <div class="absolute left-0 mt-2 z-30 w-48 rounded-xl border border-slate-800 bg-slate-950 p-2 shadow-xl space-y-1">
              <div class="text-[10px] font-bold text-slate-400 px-2 py-1 uppercase">表示カラム設定</div>
              {#each currentColumns as col}
                {@const isVis = columnVisibility[`${activeTab}_${col.key}`] !== false}
                <button
                  type="button"
                  onclick={() => toggleColumn(col.key)}
                  class="flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-xs text-left transition-colors cursor-pointer {isVis ? 'text-cyan-300 bg-cyan-950/40' : 'text-slate-400 hover:bg-slate-900'}"
                >
                  <span>{col.label}</span>
                  {#if isVis}
                    <Check class="h-3.5 w-3.5 text-cyan-400" />
                  {/if}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2">
        {#if activeTab === "sflow"}
          <label class="flex items-center gap-2 cursor-pointer select-none bg-slate-800 hover:bg-slate-700/80 px-3 py-1.5 rounded-xl border border-slate-700 transition-colors">
            <span class="text-xs font-semibold text-slate-300">Counter</span>
            <input
              type="checkbox"
              bind:checked={sflowCounter}
              onchange={() => {
                currentPage = 1;
                loadCurrentLogs();
              }}
              class="sr-only peer"
            />
            <div class="relative w-8 h-4 bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-cyan-500"></div>
          </label>
        {/if}

        <button
          type="button"
          onclick={() => (showReportModal = true)}
          class="flex items-center gap-1.5 rounded-xl border border-emerald-500/40 bg-emerald-500/10 hover:bg-emerald-500/20 px-3.5 py-1.5 text-xs font-bold text-emerald-300 transition-all cursor-pointer"
        >
          <BarChart3 class="h-3.5 w-3.5" />
          <span>レポート</span>
        </button>

        <button
          type="button"
          onclick={handleDeleteAll}
          title="全ログ削除"
          class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-1.5 text-xs font-semibold text-rose-300 transition-colors cursor-pointer"
        >
          <Trash2 class="h-3.5 w-3.5" />
          <span>全消去</span>
        </button>

        <button
          type="button"
          onclick={exportCSV}
          class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
        >
          <Download class="h-3.5 w-3.5 text-cyan-400" />
          <span>CSV</span>
        </button>

        <button
          type="button"
          onclick={exportExcel}
          class="flex items-center gap-1.5 rounded-xl border border-emerald-500/30 bg-emerald-950/40 hover:bg-emerald-900/40 px-3 py-1.5 text-xs font-semibold text-emerald-300 transition-colors cursor-pointer"
        >
          <FileText class="h-3.5 w-3.5 text-emerald-400" />
          <span>Excel</span>
        </button>

        <button
          type="button"
          onclick={loadCurrentLogs}
          disabled={loading}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
        >
          <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
          <span>更新</span>
        </button>
      </div>
    </div>

    <!-- Table Container -->
    <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
      <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
        <table class="w-full text-left text-xs">
          <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
            <tr>
              {#each visibleColumns as col}
                <th
                  class="py-1 px-2.5 {col.width || ''} {col.align === 'center' ? 'text-center' : col.align === 'right' ? 'text-right' : 'text-left'} {col.sortable ? 'cursor-pointer select-none hover:text-slate-200' : ''}"
                  onclick={() => col.sortable && handleSort(col.key)}
                >
                  <div class="inline-flex items-center gap-1">
                    <span>{col.label}</span>
                    {#if col.sortable}
                      {#if sortColumn === col.key}
                        {#if sortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    {/if}
                  </div>
                </th>
              {/each}
              <th class="py-1 px-2 text-center w-10">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/40 font-mono text-slate-300">
            {#each paginatedLogs as item}
              <tr class="hover:bg-slate-800/40 transition-colors">
                {#each visibleColumns as col}
                  <td class="py-1 px-2.5 {col.align === 'center' ? 'text-center' : col.align === 'right' ? 'text-right' : 'text-left'}">
                    {#if col.key === "time"}
                      <span class="text-slate-400 text-[11px] whitespace-nowrap leading-tight">{activeTab === "syslog" || activeTab === "netflow" || activeTab === "sflow" ? renderTimeMili(item.time) : formatTimeStr(item.time)}</span>
                    {:else if col.key === "level" || col.key === "state"}
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {getLevelBadge(item.level || item.state || 'info')}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(item.level || item.state || 'info')}"></span>
                        {item.level || item.state || 'info'}
                      </span>
                    {:else if col.key === "bytes" && typeof item.bytes === "number"}
                      <span class="font-sans text-[11px] text-slate-300 leading-tight">{renderBytes(item.bytes)}</span>
                    {:else if col.key === "packets" && typeof item.packets === "number"}
                      <span class="text-[11px] text-slate-300 leading-tight">{item.packets.toLocaleString()}</span>
                    {:else if col.key === "dur"}
                      <span class="text-[11px] text-slate-300 leading-tight">{typeof item.dur === 'number' ? (item.dur === 0 ? '0' : item.dur.toFixed(2)) : (item.dur || '0')}</span>
                    {:else if col.key === "reason"}
                      <span class="text-slate-300 text-[11px] font-sans leading-tight">{item.reason ? item.reason : ""}</span>
                    {:else if col.key === "srcLoc" || col.key === "dstLoc" || col.key === "srcMac" || col.key === "dstMac" || col.key === "tcpFlags"}
                      <span class="text-slate-400 text-[11px] font-sans truncate leading-tight">{item[col.key] || ""}</span>
                    {:else if col.key === "srcPort" || col.key === "dstPort"}
                      <span class="text-slate-300 text-[11px] font-sans leading-tight">{item[col.key] || 0}</span>
                    {:else if col.key === "srcAddr" || col.key === "dstAddr" || col.key === "remote"}
                      <span class="font-sans text-slate-200 text-[11px] truncate leading-tight">{item[col.key] || "-"}</span>
                    {:else if col.key === "counterType"}
                      <span class="font-semibold text-cyan-400 text-[11px] truncate leading-tight">{item.counterType || "-"}</span>
                    {:else if col.key === "counterData"}
                      <span class="font-sans text-slate-200 break-all text-[11px] leading-relaxed select-text">{formatCounterData(item.counterData)}</span>
                    {:else if col.key === "event" || col.key === "message" || col.key === "payload" || col.key === "log"}
                      <span class="font-sans text-slate-100 break-all text-[11px] leading-tight line-clamp-1">{item[col.key] || "-"}</span>
                    {:else if col.key === "node" || col.key === "host" || col.key === "src" || col.key === "ip"}
                      <span class="font-semibold text-cyan-400 text-[11px] truncate leading-tight">{item[col.key] || "-"}</span>
                    {:else}
                      <span class="text-slate-200 text-[11px] font-sans truncate leading-tight">{item[col.key] || "-"}</span>
                    {/if}
                  </td>
                {/each}
                <td class="py-1 px-2 text-center font-sans">
                  <button
                    type="button"
                    onclick={() => handleAskAI(item.fullText)}
                    title="AIログ診断"
                    aria-label="AIログ診断"
                    class="inline-flex items-center justify-center rounded border border-cyan-500/30 bg-cyan-500/10 p-0.5 text-cyan-300 hover:bg-cyan-500/20 hover:text-cyan-200 transition-all cursor-pointer"
                  >
                    <Sparkles class="h-3 w-3" />
                  </button>
                </td>
              </tr>
            {/each}

            {#if paginatedLogs.length === 0}
              <tr>
                <td colspan={visibleColumns.length + 1} class="py-16 text-center text-slate-500 font-sans">
                  {loading ? "ログを読み込み中..." : "該当するログは見つかりませんでした"}
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      </div>

      <!-- Pagination Footer -->
      <div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-800 bg-slate-950/80 px-4 py-2.5 text-xs text-slate-400 shrink-0">
        <div class="flex items-center gap-3">
          <span>表示件数:</span>
          <select
            bind:value={pageSize}
            onchange={() => (currentPage = 1)}
            class="rounded-lg border border-slate-700 bg-slate-900 px-2 py-1 text-xs text-slate-200 focus:outline-none cursor-pointer"
          >
            <option value={10}>10 件 / ページ</option>
            <option value={25}>25 件 / ページ</option>
            <option value={50}>50 件 / ページ</option>
            <option value={100}>100 件 / ページ</option>
            <option value={250}>250 件 / ページ</option>
            <option value={-1}>全件表示</option>
          </select>

          <span class="font-mono text-[11px] text-slate-400">
            {#if sortedLogs.length > 0}
              {sortedLogs.length.toLocaleString()} 件中 {(currentPage - 1) * pageSize + 1} 〜 {pageSize === -1 ? sortedLogs.length : Math.min(currentPage * pageSize, sortedLogs.length)} 件を表示
            {:else}
              0 件
            {/if}
          </span>
        </div>

        {#if pageSize !== -1 && totalPages > 1}
          <div class="flex items-center gap-1">
            <button
              type="button"
              disabled={currentPage <= 1}
              onclick={() => (currentPage = 1)}
              class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
              title="最初のページ"
            >
              <ChevronsLeft class="h-4 w-4" />
            </button>

            <button
              type="button"
              disabled={currentPage <= 1}
              onclick={() => currentPage--}
              class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
              title="前のページ"
            >
              <ChevronLeft class="h-4 w-4" />
            </button>

            <span class="px-2 font-mono text-xs text-slate-300">
              {currentPage} / {totalPages}
            </span>

            <button
              type="button"
              disabled={currentPage >= totalPages}
              onclick={() => currentPage++}
              class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
              title="次のページ"
            >
              <ChevronRight class="h-4 w-4" />
            </button>

            <button
              type="button"
              disabled={currentPage >= totalPages}
              onclick={() => (currentPage = totalPages)}
              class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
              title="最後のページ"
            >
              <ChevronsRight class="h-4 w-4" />
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>

  <!-- Detailed Filter Modal -->
  <LogFilterModal
    bind:show={showFilterModal}
    logCategory={activeTab}
    bind:filterState
    onApply={loadCurrentLogs}
  />

  <!-- Analytics Report Modal -->
  <LogReportModal
    bind:show={showReportModal}
    logs={structuredLogs.map((l) => l.raw)}
    logCategory={activeTab}
  />

  <!-- AI Analysis Modal -->
  {#if showAIDialog}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm">
      <div class="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl overflow-hidden text-slate-100">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2 text-base font-bold text-cyan-400">
            <Sparkles class="h-5 w-5" />
            <span>AI ログ診断アシスタント</span>
          </div>
          <button onclick={() => (showAIDialog = false)} class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-white cursor-pointer">
            <X class="h-5 w-5" />
          </button>
        </div>

        <div class="my-3 rounded-xl border border-slate-800 bg-slate-950 p-3 font-mono text-xs text-slate-300 break-all max-h-32 overflow-y-auto">
          {selectedLogText}
        </div>

        <div class="flex-1 overflow-y-auto rounded-xl border border-slate-800 bg-slate-950 p-4 text-xs leading-relaxed text-slate-200 whitespace-pre-wrap">
          {#if aiLoading}
            <div class="flex items-center gap-2 text-cyan-400">
              <RefreshCw class="h-4 w-4 animate-spin" />
              <span>マルチLLM推論中... ログとトポロジーを総合解析しています</span>
            </div>
          {:else}
            {aiAnswer}
          {/if}
        </div>

        <div class="mt-4 flex justify-end border-t border-slate-800 pt-3">
          <button onclick={() => (showAIDialog = false)} class="rounded-xl bg-slate-800 hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer">
            閉じる
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
