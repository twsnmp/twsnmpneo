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
    renderDuration,
    renderBytes,
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
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "level", label: "レベル", width: "w-24", align: "center", sortable: true },
      { key: "host", label: "ホスト (Src)", width: "w-40", sortable: true },
      { key: "tag", label: "タグ (Tag)", width: "w-32", sortable: true },
      { key: "message", label: "メッセージ", sortable: true },
    ],
    trap: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "src", label: "送信元 (From)", width: "w-40", sortable: true },
      { key: "trapType", label: "TRAP種別", width: "w-36", sortable: true },
      { key: "enterprise", label: "Enterprise", width: "w-44", sortable: true },
      { key: "variables", label: "変数 (Variables)", sortable: true },
    ],
    netflow: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "src", label: "送信元 (Src)", width: "w-44", sortable: true },
      { key: "dst", label: "宛先 (Dst)", width: "w-44", sortable: true },
      { key: "protocol", label: "プロトコル", width: "w-24", align: "center", sortable: true },
      { key: "packets", label: "パケット", width: "w-24", align: "right", sortable: true },
      { key: "bytes", label: "バイト数", width: "w-28", align: "right", sortable: true },
      { key: "info", label: "情報", sortable: true },
    ],
    sflow: [
      { key: "time", label: "日時", width: "w-44", sortable: true },
      { key: "agent", label: "エージェント", width: "w-40", sortable: true },
      { key: "src", label: "送信元 (Src)", width: "w-44", sortable: true },
      { key: "dst", label: "宛先 (Dst)", width: "w-44", sortable: true },
      { key: "protocol", label: "プロトコル", width: "w-24", align: "center", sortable: true },
      { key: "bytes", label: "バイト数", width: "w-28", align: "right", sortable: true },
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

  const currentColumns = $derived(categoryColumns[activeTab] || []);

  const visibleColumns = $derived(
    currentColumns.filter((col) => {
      const key = `${activeTab}_${col.key}`;
      return columnVisibility[key] !== false;
    })
  );

  const toggleColumn = (key: string) => {
    const colKey = `${activeTab}_${key}`;
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

        const pq = await queryParquetLogs({
          type: activeTab === "arp" ? "arplog" : activeTab,
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
          const level = (parsed.severity !== undefined ? (parsed.severity <= 2 ? "high" : parsed.severity <= 4 ? "warn" : "info") : (parsed.level ?? "info")).toLowerCase();
          const host = parsed.host ?? parsed.Host ?? src;
          const tag = parsed.tag ?? parsed.Tag ?? "";
          const message = parsed.message ?? parsed.Message ?? rawLog;

          // Trap
          const trapType = parsed.trapType ?? parsed.TrapType ?? "";
          const enterprise = parsed.enterprise ?? parsed.Enterprise ?? "";
          const variables = parsed.variables ? JSON.stringify(parsed.variables) : "";

          // NetFlow
          const dst = parsed.dstIP ? `${parsed.dstIP}:${parsed.dstPort || 0}` : (parsed.DstAddr ? `${parsed.DstAddr}:${parsed.DstPort || 0}` : "-");
          const netflowSrc = parsed.srcIP ? `${parsed.srcIP}:${parsed.srcPort || 0}` : (parsed.SrcAddr ? `${parsed.SrcAddr}:${parsed.SrcPort || 0}` : src);
          const protocol = parsed.protocol ?? parsed.Protocol ?? "-";
          const packets = parsed.packets ?? parsed.Packets ?? 0;
          const bytes = parsed.bytes ?? parsed.Bytes ?? 0;
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

          return {
            raw: pl,
            time,
            src,
            level,
            host,
            tag,
            message,
            trapType,
            enterprise,
            variables,
            dst,
            netflowSrc,
            protocol,
            packets,
            bytes,
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
            fullText: `${rawLog} ${src} ${tag} ${host} ${topic} ${ip}`,
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
    const filename = `twsnmp_${activeTab}_logs_${Date.now()}.csv`;
    const cols = visibleColumns;
    let csv = cols.map((c) => `"${c.label}"`).join(",") + "\n";

    csv += sortedLogs
      .map((row: any) =>
        cols
          .map((c) => {
            let val = row[c.key];
            if (c.key === "time") val = formatTimeStr(val);
            if (c.key === "bytes" && typeof val === "number") val = renderBytes(val);
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
        <div id="logReceptionChart" class="h-32 w-full"></div>
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
                  class="py-2.5 px-3.5 {col.width || ''} {col.align === 'center' ? 'text-center' : col.align === 'right' ? 'text-right' : 'text-left'} {col.sortable ? 'cursor-pointer select-none hover:text-slate-200' : ''}"
                  onclick={() => col.sortable && handleSort(col.key)}
                >
                  <div class="inline-flex items-center gap-1.5">
                    <span>{col.label}</span>
                    {#if col.sortable}
                      {#if sortColumn === col.key}
                        {#if sortDirection === "asc"}
                          <ArrowUp class="h-3 w-3 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-3 w-3 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 text-slate-600" />
                      {/if}
                    {/if}
                  </div>
                </th>
              {/each}
              <th class="py-2.5 px-3.5 text-center w-14">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
            {#each paginatedLogs as item}
              <tr class="hover:bg-slate-800/40 transition-colors">
                {#each visibleColumns as col}
                  <td class="py-2 px-3.5 {col.align === 'center' ? 'text-center' : col.align === 'right' ? 'text-right' : 'text-left'}">
                    {#if col.key === "time"}
                      <span class="text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(item.time)}</span>
                    {:else if col.key === "level" || col.key === "state"}
                      <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getLevelBadge(item.level || item.state)}">
                        <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(item.level || item.state)}"></span>
                        {item.level || item.state}
                      </span>
                    {:else if col.key === "bytes" && typeof item.bytes === "number"}
                      <span class="font-sans text-[11px] text-slate-300">{renderBytes(item.bytes)}</span>
                    {:else if col.key === "packets" && typeof item.packets === "number"}
                      <span class="text-[11px] text-slate-300">{item.packets.toLocaleString()}</span>
                    {:else if col.key === "event" || col.key === "message" || col.key === "payload" || col.key === "log"}
                      <span class="font-sans text-slate-100 break-all text-[11px] line-clamp-2">{item[col.key] || "-"}</span>
                    {:else if col.key === "node" || col.key === "host" || col.key === "src" || col.key === "ip"}
                      <span class="font-semibold text-cyan-400 text-[11px] truncate">{item[col.key] || "-"}</span>
                    {:else}
                      <span class="text-slate-200 text-[11px] font-sans truncate">{item[col.key] || "-"}</span>
                    {/if}
                  </td>
                {/each}
                <td class="py-2 px-3.5 text-center font-sans">
                  <button
                    type="button"
                    onclick={() => handleAskAI(item.fullText)}
                    title="AIログ診断"
                    aria-label="AIログ診断"
                    class="inline-flex items-center justify-center rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-1.5 text-cyan-300 hover:bg-cyan-500/20 hover:text-cyan-200 transition-all cursor-pointer"
                  >
                    <Sparkles class="h-3.5 w-3.5" />
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
