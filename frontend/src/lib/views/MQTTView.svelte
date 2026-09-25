<script lang="ts">
  import { onMount, tick } from "svelte";
  import {
    fetchMqttStats,
    deleteMqttStats,
    deleteAllMqttStats,
    fetchMqttLogs,
    deleteMqttLogs,
    fetchNodes,
    type MqttStatEnt,
    type NodeEnt,
    type PollingEnt,
    type ParquetLogRecord,
  } from "../api";
  import { _ } from "svelte-i18n";
  import { renderBytes, renderTime, getStateColor } from "../common";
  import { showLogCountChart, resizeLogCountChart, disposeLogCountChart } from "../charts/logcount";
  import {
    showMqttOverviewStatePie,
    showMqttOverviewTopicBar,
  } from "../charts/mqtt";
  import MQTTReportModal from "../components/MQTTReportModal.svelte";
  import PollingDialog from "../components/PollingDialog.svelte";
  import {
    Radio,
    FileText,
    Search,
    RefreshCw,
    Trash2,
    PieChart,
    Copy,
    Check,
    ChevronDown,
    ChevronRight,
    Plus,
    Download,
    Eye,
    X,
    AlertTriangle,
    CheckCircle2,
    BarChart3,
    RotateCcw,
    Activity,
    Users,
    HardDrive,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";

  type TabType = "stats" | "logs";

  let activeTab = $state<TabType>("stats");
  let loading = $state(false);

  // --- Stats State ---
  let stats = $state<MqttStatEnt[]>([]);
  let statSearch = $state("");
  let statSortKey = $state("Last");
  let statSortDir = $state<"asc" | "desc">("desc");
  let selectedIds = $state<Set<string>>(new Set());
  let expandedRows = $state<Set<string>>(new Set());
  let statPage = $state(1);
  let statPageSize = $state(25);
  let showReportModal = $state(false);
  let copiedTopicId = $state<string | null>(null);
  let showDeleteAllConfirm = $state(false);

  // --- Logs Sorting State ---
  let logSortKey = $state("time");
  let logSortDir = $state<"asc" | "desc">("desc");

  const getStateBadge = (state: string) => {
    switch (state?.toLowerCase()) {
      case "normal":
        return "bg-emerald-100 text-emerald-800 border-emerald-300 dark:bg-emerald-500/10 dark:text-emerald-400 dark:border-emerald-500/30";
      case "warn":
        return "bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30";
      case "low":
        return "bg-rose-100 text-rose-800 border-rose-300 dark:bg-rose-500/10 dark:text-rose-400 dark:border-rose-500/30";
      default:
        return "bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-700/30 dark:text-slate-300 dark:border-slate-600/40";
    }
  };

  // Chart instances for Overview
  let mqttStateChartInstance: any = null;
  let mqttTopicChartInstance: any = null;

  // Polling Dialog integration
  let showPollingDialog = $state(false);
  let pollingTemplate = $state<PollingEnt | null>(null);
  let nodes = $state<NodeEnt[]>([]);

  // --- Logs State ---
  let logs = $state<ParquetLogRecord[]>([]);
  let logSearch = $state("");
  let logTimeRange = $state<string>("24h");
  let logPage = $state(1);
  let logPageSize = $state(25);
  let selectedLog = $state<any | null>(null);
  let showClearLogsConfirm = $state(false);

  // Reception Chart State
  let showChart = $state(true);
  let chartZoomRange = $state<{ st: number; et: number } | null>(null);

  onMount(() => {
    (async () => {
      await Promise.allSettled([
        refreshStats(),
        refreshLogs(),
        (async () => {
          try {
            nodes = await fetchNodes();
          } catch {
            // ignore
          }
        })(),
      ]);
    })();

    const handleResize = () => {
      if (activeTab === "stats") {
        mqttStateChartInstance?.resize();
        mqttTopicChartInstance?.resize();
      } else if (showChart && activeTab === "logs") {
        resizeLogCountChart();
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      disposeLogCountChart();
      mqttStateChartInstance?.dispose();
      mqttTopicChartInstance?.dispose();
    };
  });

  const renderStatsOverviewCharts = async () => {
    await tick();
    if (activeTab !== "stats") return;
    const stateEl = document.getElementById("mqttStateOverviewChart");
    if (stateEl) {
      mqttStateChartInstance?.dispose();
      mqttStateChartInstance = showMqttOverviewStatePie(stateEl, stats);
    }
    const topicEl = document.getElementById("mqttTopicOverviewChart");
    if (topicEl) {
      mqttTopicChartInstance?.dispose();
      mqttTopicChartInstance = showMqttOverviewTopicBar(topicEl, stats);
    }
  };

  const renderReceptionChart = async () => {
    await tick();
    if (!showChart || activeTab !== "logs") return;
    const container = document.getElementById("mqttLogReceptionChart");
    if (!container) return;

    showLogCountChart(container, parsedLogs, (st, et) => {
      if (st && et) {
        chartZoomRange = { st, et };
      } else {
        chartZoomRange = null;
      }
    });
  };

  const refreshStats = async () => {
    loading = true;
    try {
      stats = await fetchMqttStats();
      selectedIds.clear();
      selectedIds = new Set();
      await renderStatsOverviewCharts();
    } finally {
      loading = false;
    }
  };

  const refreshLogs = async () => {
    loading = true;
    try {
      const now = Date.now();
      let start = 0;
      if (logTimeRange === "1h") start = (now - 3600 * 1000) * 1e6;
      else if (logTimeRange === "24h") start = (now - 24 * 3600 * 1000) * 1e6;
      else if (logTimeRange === "7d") start = (now - 7 * 24 * 3600 * 1000) * 1e6;
      else if (logTimeRange === "30d") start = (now - 30 * 24 * 3600 * 1000) * 1e6;

      logs = await fetchMqttLogs({
        filter: logSearch,
        start: start > 0 ? start : undefined,
        limit: 2000,
      });
      logPage = 1;
      await renderReceptionChart();
    } finally {
      loading = false;
    }
  };

  const switchTab = async (t: TabType) => {
    activeTab = t;
    if (t === "stats") {
      disposeLogCountChart();
      await refreshStats();
    } else {
      chartZoomRange = null;
      await refreshLogs();
    }
  };

  // --- MQTT Overview KPIs ---
  const mqttKPIs = $derived.by(() => {
    const totalTopics = stats.length;
    let totalCount = 0;
    let totalBytes = 0;
    const clientSet = new Set<string>();
    const remoteSet = new Set<string>();

    for (const s of stats) {
      totalCount += s.Count || 0;
      totalBytes += s.Bytes || 0;
      if (s.ClientID) clientSet.add(s.ClientID);
      if (s.Remote) remoteSet.add(s.Remote);
    }

    return {
      totalTopics,
      clientCount: clientSet.size,
      remoteCount: remoteSet.size,
      totalCount,
      totalBytes,
    };
  });

  const handleStatSort = (key: string) => {
    if (statSortKey === key) {
      statSortDir = statSortDir === "asc" ? "desc" : "asc";
    } else {
      statSortKey = key;
      statSortDir = key === "Count" || key === "Bytes" || key === "First" || key === "Last" ? "desc" : "asc";
    }
  };

  // --- Stats Filtering & Pagination ---
  const filteredStats = $derived.by(() => {
    let list = stats.filter((s) => {
      if (!statSearch.trim()) return true;
      const q = statSearch.toLowerCase();
      return (
        (s.Topic && s.Topic.toLowerCase().includes(q)) ||
        (s.ClientID && s.ClientID.toLowerCase().includes(q)) ||
        (s.Remote && s.Remote.toLowerCase().includes(q)) ||
        (s.Value && s.Value.toLowerCase().includes(q))
      );
    });

    const key = statSortKey;
    const dir = statSortDir === "asc" ? 1 : -1;
    list.sort((a: any, b: any) => {
      const va = a[key];
      const vb = b[key];
      if (va === vb) return 0;
      if (va === undefined || va === null) return 1;
      if (vb === undefined || vb === null) return -1;
      if (typeof va === "number" && typeof vb === "number") {
        return (va - vb) * dir;
      }
      return String(va).localeCompare(String(vb)) * dir;
    });

    return list;
  });

  const totalStatPages = $derived(Math.max(1, Math.ceil(filteredStats.length / statPageSize)));
  const paginatedStats = $derived(
    filteredStats.slice((statPage - 1) * statPageSize, statPage * statPageSize)
  );

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredStats.length && filteredStats.length > 0) {
      selectedIds.clear();
      selectedIds = new Set();
    } else {
      const newSet = new Set<string>();
      filteredStats.forEach((s) => newSet.add(s.ID));
      selectedIds = newSet;
    }
  };

  const toggleSelectRow = (id: string) => {
    const newSet = new Set(selectedIds);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    selectedIds = newSet;
  };

  const toggleExpand = (id: string) => {
    const newSet = new Set(expandedRows);
    if (newSet.has(id)) {
      newSet.delete(id);
    } else {
      newSet.add(id);
    }
    expandedRows = newSet;
  };

  const copyTopic = (topic: string, id: string) => {
    if (!topic) return;
    navigator.clipboard.writeText(topic);
    copiedTopicId = id;
    setTimeout(() => {
      if (copiedTopicId === id) copiedTopicId = null;
    }, 1500);
  };

  const handleDeleteSelected = async () => {
    if (selectedIds.size === 0) return;
    if (!confirm($_('mqtt.confirmDeleteSelected', { values: { count: selectedIds.size } }))) return;
    const ids = Array.from(selectedIds);
    await deleteMqttStats(ids);
    await refreshStats();
  };

  const handleDeleteAll = async () => {
    await deleteAllMqttStats();
    showDeleteAllConfirm = false;
    await refreshStats();
  };

  const handleCreatePolling = () => {
    const selected = stats.find((s) => selectedIds.has(s.ID));
    if (!selected) return;

    let targetNode = nodes.find((n) => n.ip === selected.Remote || n.IP === selected.Remote);
    if (!targetNode) {
      targetNode = nodes.find((n) => n.ip === "127.0.0.1" || n.ip === "localhost" || (n.name && n.name.toLowerCase().includes("twsnmp")));
    }
    if (!targetNode && nodes.length > 0) {
      targetNode = nodes[0];
    }

    const nId = targetNode ? (targetNode.id || targetNode.ID || "") : "";

    pollingTemplate = {
      id: "",
      name: `MQTT ${selected.Topic}`,
      node_id: nId,
      type: "mqtt",
      target: "tcp://localhost:1883",
      state: "normal",
      ...( { Filter: selected.Topic, Topic: selected.Topic } as any )
    };
    showPollingDialog = true;
  };

  const formatJsonOrText = (raw: string) => {
    if (!raw) return `<div class="text-slate-400 italic">${$_('mqtt.noPayload')}</div>`;
    try {
      const parsed = JSON.parse(raw);
      const pretty = JSON.stringify(parsed, null, 2);
      return `<pre class="font-mono text-xs text-emerald-600 dark:text-emerald-400 whitespace-pre-wrap"><code>${escapeHtml(pretty)}</code></pre>`;
    } catch {
      return `<pre class="font-mono text-xs text-slate-700 dark:text-slate-300 whitespace-pre-wrap"><code>${escapeHtml(raw)}</code></pre>`;
    }
  };

  const escapeHtml = (str: string) => {
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  };

  // --- Logs Parsing & Filtering ---
  const parsedLogs = $derived(
    logs.map((l) => {
      let topic = "";
      let client = l.src;
      let remote = l.src;
      let payload = l.log;

      try {
        const p = JSON.parse(l.log);
        if (p.topic) topic = p.topic;
        if (p.clientID) client = p.clientID;
        if (p.remote) remote = p.remote;
        if (p.payload !== undefined) payload = typeof p.payload === "string" ? p.payload : JSON.stringify(p.payload);
      } catch {
        // Fallback for legacy [topic] payload format
        if (l.log.startsWith("[") && l.log.includes("]")) {
          const endIdx = l.log.indexOf("]");
          topic = l.log.substring(1, endIdx);
          payload = l.log.substring(endIdx + 1).trim();
        }
      }

      return {
        time: l.time,
        src: remote,
        clientID: client,
        topic: topic,
        payload: payload,
        raw: l,
      };
    })
  );

  const handleLogSort = (key: string) => {
    if (logSortKey === key) {
      logSortDir = logSortDir === "asc" ? "desc" : "asc";
    } else {
      logSortKey = key;
      logSortDir = key === "time" ? "desc" : "asc";
    }
  };

  const filteredLogs = $derived.by(() => {
    let list = parsedLogs.filter((l) => {
      if (chartZoomRange && chartZoomRange.st > 0 && chartZoomRange.et > 0) {
        const itemTime = l.time;
        if (itemTime < chartZoomRange.st || itemTime > chartZoomRange.et) {
          return false;
        }
      }

      if (!logSearch.trim()) return true;
      const q = logSearch.toLowerCase();
      return (
        l.topic.toLowerCase().includes(q) ||
        l.clientID.toLowerCase().includes(q) ||
        l.src.toLowerCase().includes(q) ||
        l.payload.toLowerCase().includes(q)
      );
    });

    const key = logSortKey;
    const dir = logSortDir === "asc" ? 1 : -1;
    list.sort((a: any, b: any) => {
      const va = a[key];
      const vb = b[key];
      if (va === vb) return 0;
      if (va === undefined || va === null) return 1;
      if (vb === undefined || vb === null) return -1;
      if (typeof va === "number" && typeof vb === "number") {
        return (va - vb) * dir;
      }
      return String(va).localeCompare(String(vb)) * dir;
    });

    return list;
  });

  const totalLogPages = $derived(Math.max(1, Math.ceil(filteredLogs.length / logPageSize)));
  const paginatedLogs = $derived(
    filteredLogs.slice((logPage - 1) * logPageSize, logPage * logPageSize)
  );

  const handleClearLogs = async () => {
    await deleteMqttLogs();
    showClearLogsConfirm = false;
    await refreshLogs();
  };

  const exportLogsCSV = () => {
    if (filteredLogs.length === 0) return;
    const headers = [$_('mqtt.csvTime'), $_('mqtt.csvRemote'), $_('mqtt.csvClientId'), $_('mqtt.csvTopic'), $_('mqtt.csvPayload')];
    const rows = filteredLogs.map((l) => [
      renderTime(l.time),
      `"${l.src}"`,
      `"${l.clientID}"`,
      `"${l.topic}"`,
      `"${l.payload.replace(/"/g, '""')}"`,
    ]);
    const csvContent = "\uFEFF" + [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `mqtt_logs_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-50 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans transition-colors">
  <!-- Left Sidebar (Matching OTelView / LogView) -->
  <div class="w-60 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/80 p-3 space-y-1.5 shrink-0 flex flex-col justify-between transition-colors">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
        MQTT
      </div>

      {#each [
        { id: "stats", name: $_('mqtt.navStats'), icon: Radio, count: stats.length },
        { id: "logs", name: $_('mqtt.navLogs'), icon: FileText, count: logs.length }
      ] as item}
        <button
          type="button"
          onclick={() => switchTab(item.id as any)}
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === item.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <div class="flex items-center gap-2.5 truncate">
            <item.icon class="h-4 w-4 shrink-0 {activeTab === item.id ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{item.name}</span>
          </div>
          <span class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeTab === item.id ? 'bg-white/20 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-transparent'}">
            {item.count.toLocaleString()}
          </span>
        </button>
      {/each}
    </div>

    <!-- Broker Endpoint Info -->
    <div class="p-3 rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50/60 dark:bg-slate-900/60 text-[10px] text-slate-500 dark:text-slate-400 space-y-1">
      <div class="flex items-center justify-between font-semibold text-slate-700 dark:text-slate-300">
        <span>{$_('mqtt.broker')}</span>
        <span class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
          {$_('mqtt.running')}
        </span>
      </div>
      <div class="font-mono text-cyan-600 dark:text-cyan-400 font-semibold">TCP :1883</div>
      <div>Topics / Messages / Clients</div>
    </div>
  </div>

  <!-- Main Content Area -->
  <div class="flex-1 flex flex-col min-w-0 overflow-hidden">
    <!-- Top Action Header -->
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 px-6 py-2.5 shrink-0 shadow-xs">
      <div>
        <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          {#if activeTab === "stats"}
            <Radio class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('mqtt.navStats')}</span>
          {:else}
            <FileText class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('mqtt.navLogs')}</span>
          {/if}
          <span class="text-[11px] px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 font-mono border border-slate-200 dark:border-slate-700">
            Port: 1883
          </span>
        </h2>
      </div>

      <!-- Action Buttons placed near Reload button -->
      <div class="flex items-center gap-2">
        {#if activeTab === "stats"}
          {#if selectedIds.size === 1}
            <button
              type="button"
              onclick={handleCreatePolling}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-blue-300 dark:border-blue-800/60 bg-blue-50 dark:bg-blue-950/40 hover:bg-blue-100 dark:hover:bg-blue-900/60 text-blue-800 dark:text-blue-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
            >
              <Plus class="h-3.5 w-3.5 text-blue-600 dark:text-blue-400" />
              <span>{$_('mqtt.btnCreatePolling')}</span>
            </button>
          {/if}

          {#if selectedIds.size > 0}
            <button
              type="button"
              onclick={handleDeleteSelected}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 text-rose-800 dark:text-rose-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
            >
              <Trash2 class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
              <span>{$_('mqtt.btnDeleteSelected', { values: { count: selectedIds.size } })}</span>
            </button>
          {/if}

          {#if stats.length > 0}
            <button
              type="button"
              onclick={() => (showReportModal = true)}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-emerald-300 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/60 text-emerald-800 dark:text-emerald-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
            >
              <PieChart class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              <span>{$_('mqtt.btnReport')}</span>
            </button>
          {/if}

          <button
            type="button"
            onclick={() => (showDeleteAllConfirm = true)}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 text-rose-800 dark:text-rose-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <Trash2 class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
            <span>{$_('mqtt.btnDeleteAll')}</span>
          </button>

          <button
            type="button"
            onclick={refreshStats}
            disabled={loading}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin text-cyan-600 dark:text-cyan-400' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span>{$_('mqtt.btnReload')}</span>
          </button>
        {:else}
          <button
            type="button"
            onclick={refreshLogs}
            disabled={loading}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin text-cyan-600 dark:text-cyan-400' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span>{$_('mqtt.btnReload')}</span>
          </button>
        {/if}
      </div>
    </div>

    <!-- Content Body -->
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden p-4">
      {#if activeTab === "stats"}
        <div class="flex flex-col h-full gap-3 overflow-hidden">
          <!-- Top Overview: 4 Vertical KPIs + 2 Charts -->
          <div class="grid grid-cols-1 lg:grid-cols-12 gap-3 shrink-0 h-64">
            <!-- 4 Vertical KPI Cards (col-span-3) -->
            <div class="lg:col-span-3 flex flex-col gap-2 h-full">
              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-xs dark:shadow-md flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-cyan-50 dark:bg-cyan-950/60 border border-cyan-200 dark:border-cyan-800/80">
                    <Radio class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('mqtt.totalTopics')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-cyan-600 dark:text-cyan-300">{mqttKPIs.totalTopics.toLocaleString()}</span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-xs dark:shadow-md flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-indigo-50 dark:bg-indigo-950/60 border border-indigo-200 dark:border-indigo-800/80">
                    <Users class="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('mqtt.remoteClients')}</span>
                </div>
                <span class="text-base font-bold font-mono text-indigo-600 dark:text-indigo-300">
                  {mqttKPIs.remoteCount} <span class="text-xs font-normal text-slate-400">/</span> {mqttKPIs.clientCount}
                </span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-xs dark:shadow-md flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800/80">
                    <Activity class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('mqtt.totalReceived')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-emerald-600 dark:text-emerald-300">{mqttKPIs.totalCount.toLocaleString()}</span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-xs dark:shadow-md flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-amber-50 dark:bg-amber-950/60 border border-amber-200 dark:border-amber-800/80">
                    <HardDrive class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('mqtt.totalData')}</span>
                </div>
                <span class="text-base font-bold font-mono text-amber-600 dark:text-amber-300">{renderBytes(mqttKPIs.totalBytes)}</span>
              </div>
            </div>

            <!-- State Donut Chart (col-span-3) -->
            <div class="lg:col-span-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2.5 relative shadow-xs dark:shadow-md h-full flex flex-col">
              <div class="text-[11px] font-semibold text-slate-500 dark:text-slate-400 px-2 pt-1 z-10 flex items-center gap-1.5">
                <span>{$_('mqtt.stateDistribution')}</span>
              </div>
              <div id="mqttStateOverviewChart" class="flex-1 w-full min-h-0"></div>
            </div>

            <!-- Top 10 Topics Bar Chart (col-span-6) -->
            <div class="lg:col-span-6 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2.5 relative shadow-xs dark:shadow-md h-full flex flex-col">
              <div class="text-[11px] font-semibold text-slate-500 dark:text-slate-400 px-2 pt-1 z-10 flex items-center gap-1.5">
                <span>{$_('mqtt.topTopics')}</span>
              </div>
              <div id="mqttTopicOverviewChart" class="flex-1 w-full min-h-0"></div>
            </div>
          </div>

          <!-- Main Table Container -->
          <div class="flex-1 flex flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/60 shadow-xs dark:shadow-xl overflow-hidden backdrop-blur-sm min-h-0">
            <!-- Top Toolbar -->
            <div class="flex shrink-0 items-center justify-between border-b border-slate-200 dark:border-slate-800/80 px-4 py-3 bg-slate-50 dark:bg-slate-900/40">
              <div class="flex items-center gap-2">
                <span class="text-xs text-slate-500 dark:text-slate-400">{$_('mqtt.rowsPerPage')}</span>
                <select
                  bind:value={statPageSize}
                  onchange={() => (statPage = 1)}
                  class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2.5 py-1 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                >
                  <option value={10}>10</option>
                  <option value={25}>25</option>
                  <option value={50}>50</option>
                  <option value={100}>100</option>
                </select>
                <span class="text-xs text-slate-500 dark:text-slate-400">{$_('mqtt.items')}</span>
              </div>

              <div class="flex items-center gap-2">
                <div class="relative w-64">
                  <Search class="absolute left-2.5 top-2 h-3.5 w-3.5 text-slate-400" />
                  <input
                    type="text"
                    placeholder={$_('mqtt.searchPlaceholder')}
                    bind:value={statSearch}
                    oninput={() => (statPage = 1)}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950/80 pl-8 pr-3 py-1 text-xs text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
                  />
                  {#if statSearch}
                    <button
                      onclick={() => (statSearch = "")}
                      class="absolute right-2.5 top-2 text-slate-400 hover:text-slate-700 dark:hover:text-white"
                    >
                      <X class="h-3.5 w-3.5" />
                    </button>
                  {/if}
                </div>
              </div>
            </div>

            <!-- Main Table -->
            <div class="flex-1 overflow-auto">
              <table class="w-full text-left text-xs border-collapse">
                <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-[10px] font-semibold uppercase text-slate-600 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <tr>
                    <th class="w-8 py-1 px-2 text-center whitespace-nowrap">
                      <input
                        type="checkbox"
                        checked={selectedIds.size > 0 && selectedIds.size === filteredStats.length}
                        onchange={toggleSelectAll}
                        class="h-3.5 w-3.5 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-600 accent-cyan-600 focus:ring-cyan-500/20 cursor-pointer"
                      />
                    </th>
                    <th class="w-6 py-1 px-1 text-center whitespace-nowrap"></th>
                    <th class="w-20 py-1 px-2.5 whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("State")}>
                      <div class="flex items-center gap-1">
                        <span>State</span>
                        {#if statSortKey === "State"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-44 py-1 px-2.5 whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("ClientID")}>
                      <div class="flex items-center gap-1">
                        <span>Client ID</span>
                        {#if statSortKey === "ClientID"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-32 py-1 px-2.5 whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("Remote")}>
                      <div class="flex items-center gap-1">
                        <span>Remote</span>
                        {#if statSortKey === "Remote"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("Topic")}>
                      <div class="flex items-center gap-1">
                        <span>Topic</span>
                        {#if statSortKey === "Topic"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-20 py-1 px-2.5 text-right whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("Count")}>
                      <div class="flex items-center justify-end gap-1">
                        <span>Count</span>
                        {#if statSortKey === "Count"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-24 py-1 px-2.5 text-right whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("Bytes")}>
                      <div class="flex items-center justify-end gap-1">
                        <span>Bytes</span>
                        {#if statSortKey === "Bytes"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-44 py-1 px-2.5 whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("First")}>
                      <div class="flex items-center gap-1">
                        <span>First Time</span>
                        {#if statSortKey === "First"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-44 py-1 px-2.5 whitespace-nowrap cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleStatSort("Last")}>
                      <div class="flex items-center gap-1">
                        <span>Last time</span>
                        {#if statSortKey === "Last"}
                          {#if statSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
                  {#if paginatedStats.length === 0}
                    <tr>
                      <td colspan="10" class="py-12 text-center text-slate-500">
                        {#if loading}
                          <div class="flex items-center justify-center gap-2">
                            <RefreshCw class="h-4 w-4 animate-spin text-cyan-600 dark:text-cyan-400" />
                            <span>{$_('mqtt.loading')}</span>
                          </div>
                        {:else}
                          {$_('mqtt.noStats')}
                        {/if}
                      </td>
                    </tr>
                    {#each paginatedStats as s (s.ID)}
                      <tr
                        class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors {selectedIds.has(s.ID) ? 'bg-cyan-50 dark:bg-cyan-950/40 text-cyan-900 dark:text-cyan-200' : ''}"
                      >
                        <!-- Checkbox -->
                        <td class="py-1 px-2 text-center">
                          <input
                            type="checkbox"
                            checked={selectedIds.has(s.ID)}
                            onchange={() => toggleSelectRow(s.ID)}
                            class="h-3.5 w-3.5 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-600 accent-cyan-600 focus:ring-cyan-500/20 cursor-pointer"
                          />
                        </td>

                        <!-- Expand Row Toggle -->
                        <td class="py-1 px-1 text-center">
                          <button
                            onclick={() => toggleExpand(s.ID)}
                            title={$_('mqtt.expandPayload')}
                            class="p-0.5 rounded text-slate-400 hover:text-slate-800 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer"
                          >
                            {#if expandedRows.has(s.ID)}
                              <ChevronDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ChevronRight class="h-3 w-3" />
                            {/if}
                          </button>
                        </td>

                        <!-- State Badge matching LogView/OTelView level display -->
                        <td class="py-1 px-2.5 font-sans">
                          <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {getStateBadge(s.State)}">
                            <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(s.State)}"></span>
                            {s.State || 'normal'}
                          </span>
                        </td>

                        <!-- Client ID -->
                        <td class="py-1 px-2.5 text-[11px] text-slate-700 dark:text-slate-300 truncate max-w-[180px] leading-tight" title={s.ClientID}>
                          {s.ClientID || "-"}
                        </td>

                        <!-- Remote IP -->
                        <td class="py-1 px-2.5 text-[11px] text-slate-700 dark:text-slate-300 leading-tight">
                          {s.Remote || "-"}
                        </td>

                        <!-- Topic with Copy Button -->
                        <td class="py-1 px-2.5 font-medium text-[11px] leading-tight">
                          <div class="flex items-center gap-1.5 group">
                            <span class="text-cyan-600 dark:text-cyan-300 break-all select-all font-mono">{s.Topic}</span>
                            <button
                              type="button"
                              onclick={(e) => { e.stopPropagation(); copyTopic(s.Topic, s.ID); }}
                              title={$_('mqtt.copyTopic')}
                              class="opacity-0 group-hover:opacity-100 focus:opacity-100 p-0.5 rounded hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 transition-all cursor-pointer shrink-0"
                            >
                              {#if copiedTopicId === s.ID}
                                <Check class="h-3 w-3 text-emerald-600 dark:text-emerald-400" />
                              {:else}
                                <Copy class="h-3 w-3" />
                              {/if}
                            </button>
                          </div>
                        </td>

                        <!-- Count -->
                        <td class="py-1 px-2.5 text-right text-slate-800 dark:text-slate-200 text-[11px] leading-tight">
                          {(s.Count || 0).toLocaleString()}
                        </td>

                        <!-- Bytes -->
                        <td class="py-1 px-2.5 text-right font-sans text-slate-600 dark:text-slate-300 text-[11px] leading-tight">
                          {renderBytes(s.Bytes)}
                        </td>

                        <!-- First Time -->
                        <td class="py-1 px-2.5 text-[11px] text-slate-500 dark:text-slate-400 leading-tight whitespace-nowrap">
                          {renderTime(s.First)}
                        </td>

                        <!-- Last time -->
                        <td class="py-1 px-2.5 text-[11px] text-slate-700 dark:text-slate-300 leading-tight whitespace-nowrap">
                          {renderTime(s.Last)}
                        </td>
                      </tr>

                      <!-- Child Row showing payload when expanded -->
                      {#if expandedRows.has(s.ID)}
                        <tr class="bg-slate-50/80 dark:bg-slate-950/80 border-b border-slate-200 dark:border-slate-800/80">
                          <td colspan="10" class="px-6 py-3">
                            <div class="flex flex-col gap-1.5">
                              <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400">
                                <span class="font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                                  <Eye class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
                                  {$_('mqtt.latestPayload')}
                                </span>
                                <span class="font-mono text-[10px] text-slate-500">
                                  Topic: {s.Topic} ({s.Bytes} bytes)
                                </span>
                              </div>
                              <div class="p-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-x-auto max-h-60 text-slate-800 dark:text-slate-200">
                                {@html formatJsonOrText(s.Value)}
                              </div>
                            </div>
                          </td>
                        </tr>
                      {/if}
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>

            <!-- Bottom Pagination (Clean, actions moved to top) -->
            <div class="shrink-0 flex items-center justify-between border-t border-slate-200 dark:border-slate-800/80 px-4 py-3 bg-slate-50 dark:bg-slate-950/70">
              <span class="text-xs text-slate-500 dark:text-slate-400">
                {#if filteredStats.length > 0}
                  Showing {(statPage - 1) * statPageSize + 1} to {Math.min(statPage * statPageSize, filteredStats.length)} of {filteredStats.length} entries
                {:else}
                  Showing 0 to 0 of 0 entries
                {/if}
              </span>

              {#if totalStatPages > 1}
                <div class="flex items-center gap-1">
                  <button
                    onclick={() => (statPage = Math.max(1, statPage - 1))}
                    disabled={statPage <= 1}
                    class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
                  >
                    Previous
                  </button>
                  {#each Array.from({ length: totalStatPages }, (_, i) => i + 1) as p}
                    {#if Math.abs(p - statPage) < 3 || p === 1 || p === totalStatPages}
                      <button
                        onclick={() => (statPage = p)}
                        class="h-7 w-7 rounded-lg text-xs font-semibold {statPage === p ? 'bg-cyan-600 text-white' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white'} cursor-pointer"
                      >
                        {p}
                      </button>
                    {/if}
                  {/each}
                  <button
                    onclick={() => (statPage = Math.min(totalStatPages, statPage + 1))}
                    disabled={statPage >= totalStatPages}
                    class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
                  >
                    Next
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>

      {:else}
        <!-- ================= LOGS VIEW ================= -->
        <div class="flex-1 flex flex-col gap-3 overflow-hidden">
          <!-- Reception Count Chart (Collapsible) -->
          {#if showChart}
            <div class="relative rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-xs dark:shadow-lg shrink-0 transition-all">
              <div class="flex items-center justify-between mb-1 px-1">
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-bold text-slate-800 dark:text-slate-300">{$_('mqtt.logTrendTitle')}</span>
                  {#if chartZoomRange}
                    <span class="inline-flex items-center gap-1 rounded-full bg-cyan-100 dark:bg-cyan-950/80 border border-cyan-300 dark:border-cyan-800 px-2 py-0.5 text-[10px] text-cyan-800 dark:text-cyan-300 font-mono">
                      {$_('mqtt.filterApplied')}
                    </span>
                    <button
                      type="button"
                      onclick={() => (chartZoomRange = null)}
                      class="flex items-center gap-1 rounded-md bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 px-2 py-0.5 text-[10px] text-slate-700 dark:text-slate-300 cursor-pointer transition-colors"
                    >
                      <RotateCcw class="h-3 w-3" />
                      <span>{$_('mqtt.resetFilter')}</span>
                    </button>
                  {/if}
                </div>
                <button
                  type="button"
                  onclick={() => (showChart = false)}
                  class="text-[10px] text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 cursor-pointer transition-colors"
                >
                  {$_('mqtt.hideGraph')}
                </button>
              </div>
              <div id="mqttLogReceptionChart" class="h-52 min-h-[200px] w-full"></div>
            </div>
          {:else}
            <div class="flex justify-end shrink-0">
              <button
                type="button"
                onclick={() => {
                  showChart = true;
                  renderReceptionChart();
                }}
                class="flex items-center gap-1 text-[11px] text-cyan-600 dark:text-cyan-400 hover:text-cyan-500 dark:hover:text-cyan-300 cursor-pointer"
              >
                <BarChart3 class="h-3.5 w-3.5" />
                <span>{$_('mqtt.showGraph')}</span>
              </button>
            </div>
          {/if}

          <!-- Logs Table Container -->
          <div class="flex-1 flex flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/60 shadow-xs dark:shadow-xl overflow-hidden backdrop-blur-sm min-h-0">
            <!-- Logs Filter Toolbar -->
            <div class="flex shrink-0 items-center justify-between border-b border-slate-200 dark:border-slate-800/80 px-4 py-3 bg-slate-50 dark:bg-slate-900/40 gap-4 flex-wrap">
              <div class="flex items-center gap-3">
                <div class="relative w-72">
                  <Search class="absolute left-2.5 top-2 h-3.5 w-3.5 text-slate-400" />
                  <input
                    type="text"
                    placeholder={$_('mqtt.searchLogPlaceholder')}
                    bind:value={logSearch}
                    oninput={() => (logPage = 1)}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950/80 pl-8 pr-3 py-1 text-xs text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
                  />
                  {#if logSearch}
                    <button
                      onclick={() => (logSearch = "")}
                      class="absolute right-2.5 top-2 text-slate-400 hover:text-slate-700 dark:hover:text-white"
                    >
                      <X class="h-3.5 w-3.5" />
                    </button>
                  {/if}
                </div>

                <!-- Time range presets -->
                <div class="flex items-center gap-1 bg-slate-100 dark:bg-slate-950 p-0.5 rounded-xl border border-slate-200 dark:border-slate-800 text-[11px]">
                  {#each [["1h", $_('mqtt.period1h')], ["24h", $_('mqtt.period24h')], ["7d", $_('mqtt.period7d')], ["30d", $_('mqtt.period30d')], ["all", $_('mqtt.periodAll')]] as [val, label]}
                    <button
                      onclick={() => { logTimeRange = val; refreshLogs(); }}
                      class="px-2.5 py-1 rounded-lg font-medium transition-all {logTimeRange === val ? 'bg-cyan-600 text-white shadow-xs font-semibold' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
                    >
                      {label}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="flex items-center gap-2">
                <!-- CSV Export -->
                <button
                  onclick={exportLogsCSV}
                  disabled={filteredLogs.length === 0}
                  class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 px-3 py-1.5 text-xs font-semibold transition-all disabled:opacity-40 shadow-xs cursor-pointer"
                >
                  <Download class="h-3.5 w-3.5" />
                  <span>{$_('mqtt.btnExportCsv')}</span>
                </button>

                <!-- Clear Logs -->
                <button
                  onclick={() => (showClearLogsConfirm = true)}
                  disabled={logs.length === 0}
                  class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-900/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900 text-rose-800 dark:text-rose-300 px-3 py-1.5 text-xs font-semibold transition-all disabled:opacity-40 shadow-xs cursor-pointer"
                >
                  <Trash2 class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
                  <span>{$_('mqtt.btnClearLogs')}</span>
                </button>
              </div>
            </div>

            <!-- Logs Table -->
            <div class="flex-1 overflow-auto">
              <table class="w-full text-left text-xs border-collapse">
                <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-[10px] font-semibold uppercase text-slate-600 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <tr>
                    <th class="w-44 py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("time")}>
                      <div class="flex items-center gap-1">
                        <span>{$_('mqtt.colTime')}</span>
                        {#if logSortKey === "time"}
                          {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-36 py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("src")}>
                      <div class="flex items-center gap-1">
                        <span>{$_('mqtt.colRemote')}</span>
                        {#if logSortKey === "src"}
                          {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-44 py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("clientID")}>
                      <div class="flex items-center gap-1">
                        <span>{$_('mqtt.colClientId')}</span>
                        {#if logSortKey === "clientID"}
                          {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-64 py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("topic")}>
                      <div class="flex items-center gap-1">
                        <span>{$_('mqtt.colTopic')}</span>
                        {#if logSortKey === "topic"}
                          {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="py-1 px-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("payload")}>
                      <div class="flex items-center gap-1">
                        <span>{$_('mqtt.colPayload')}</span>
                        {#if logSortKey === "payload"}
                          {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                    <th class="w-10 py-1 px-2 text-center"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
                  {#if paginatedLogs.length === 0}
                    <tr>
                      <td colspan="6" class="py-12 text-center text-slate-500">
                        {#if loading}
                          <div class="flex items-center justify-center gap-2">
                            <RefreshCw class="h-4 w-4 animate-spin text-cyan-600 dark:text-cyan-400" />
                            <span>{$_('mqtt.loadingLogs')}</span>
                          </div>
                        {:else}
                          {$_('mqtt.noLogs')}
                        {/if}
                      </td>
                    </tr>
                  {:else}
                    {#each paginatedLogs as l}
                      <tr
                        onclick={() => (selectedLog = l)}
                        class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors cursor-pointer"
                      >
                        <td class="py-1 px-2.5 text-[11px] text-slate-500 dark:text-slate-400 leading-tight">
                          {renderTime(l.time)}
                        </td>
                        <td class="py-1 px-2.5 text-[11px] text-slate-700 dark:text-slate-300 leading-tight">
                          {l.src}
                        </td>
                        <td class="py-1 px-2.5 text-[11px] truncate max-w-[160px] leading-tight text-slate-700 dark:text-slate-300" title={l.clientID}>
                          {l.clientID}
                        </td>
                        <td class="py-1 px-2.5 text-[11px] text-cyan-600 dark:text-cyan-300 truncate max-w-[240px] leading-tight font-medium" title={l.topic}>
                          {l.topic || "-"}
                        </td>
                        <td class="py-1 px-2.5 text-[11px] truncate max-w-[400px] leading-tight text-slate-700 dark:text-slate-300">
                          {l.payload}
                        </td>
                        <td class="py-1 px-2 text-center">
                          <button
                            onclick={(e) => { e.stopPropagation(); selectedLog = l; }}
                            class="p-0.5 rounded text-slate-400 hover:text-slate-700 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer"
                          >
                            <Eye class="h-3 w-3" />
                          </button>
                        </td>
                      </tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>

            <!-- Logs Pagination Footer -->
            <div class="shrink-0 flex items-center justify-between border-t border-slate-200 dark:border-slate-800/80 px-4 py-3 bg-slate-50 dark:bg-slate-950/70">
              <span class="text-xs text-slate-500 dark:text-slate-400">
                {#if filteredLogs.length > 0}
                  Showing {(logPage - 1) * logPageSize + 1} to {Math.min(logPage * logPageSize, filteredLogs.length)} of {filteredLogs.length} logs
                {:else}
                  Showing 0 to 0 of 0 logs
                {/if}
              </span>

              {#if totalLogPages > 1}
                <div class="flex items-center gap-1">
                  <button
                    onclick={() => (logPage = Math.max(1, logPage - 1))}
                    disabled={logPage <= 1}
                    class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
                  >
                    Previous
                  </button>
                  {#each Array.from({ length: totalLogPages }, (_, i) => i + 1) as p}
                    {#if Math.abs(p - logPage) < 3 || p === 1 || p === totalLogPages}
                      <button
                        onclick={() => (logPage = p)}
                        class="h-7 w-7 rounded-lg text-xs font-semibold {logPage === p ? 'bg-cyan-600 text-white' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white'} cursor-pointer"
                      >
                        {p}
                      </button>
                    {/if}
                  {/each}
                  <button
                    onclick={() => (logPage = Math.min(totalLogPages, logPage + 1))}
                    disabled={logPage >= totalLogPages}
                    class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
                  >
                    Next
                  </button>
                </div>
              {/if}
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
</div>

<!-- MQTT Report Modal -->
<MQTTReportModal bind:show={showReportModal} stats={stats} />

<!-- Polling Create Dialog -->
<PollingDialog
  bind:show={showPollingDialog}
  bind:polling={pollingTemplate}
  {nodes}
  onSave={() => {
    showPollingDialog = false;
  }}
/>

<!-- Delete All Stats Confirmation Modal -->
{#if showDeleteAllConfirm}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="w-full max-w-md rounded-2xl border border-rose-300 dark:border-rose-800/60 bg-white dark:bg-slate-900 p-6 shadow-2xl">
      <div class="flex items-center gap-3 text-rose-600 dark:text-rose-400 mb-3">
        <AlertTriangle class="h-6 w-6" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">{$_('mqtt.confirmDeleteAllTitle')}</h3>
      </div>
      <p class="text-xs text-slate-600 dark:text-slate-400 mb-6">
        {$_('mqtt.confirmDeleteAllDesc')}
      </p>
      <div class="flex justify-end gap-2">
        <button
          onclick={() => (showDeleteAllConfirm = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer shadow-xs"
        >
          {$_('mqtt.cancel')}
        </button>
        <button
          onclick={handleDeleteAll}
          class="rounded-xl bg-rose-600 hover:bg-rose-500 px-4 py-2 text-xs font-semibold text-white shadow-md cursor-pointer"
        >
          {$_('mqtt.btnDeleteAll')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Clear Logs Confirmation Modal -->
{#if showClearLogsConfirm}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="w-full max-w-md rounded-2xl border border-rose-300 dark:border-rose-800/60 bg-white dark:bg-slate-900 p-6 shadow-2xl">
      <div class="flex items-center gap-3 text-rose-600 dark:text-rose-400 mb-3">
        <AlertTriangle class="h-6 w-6" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-white">{$_('mqtt.confirmClearLogsTitle')}</h3>
      </div>
      <p class="text-xs text-slate-600 dark:text-slate-400 mb-6">
        {$_('mqtt.confirmClearLogsDesc')}
      </p>
      <div class="flex justify-end gap-2">
        <button
          onclick={() => (showClearLogsConfirm = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer shadow-xs"
        >
          {$_('mqtt.cancel')}
        </button>
        <button
          onclick={handleClearLogs}
          class="rounded-xl bg-rose-600 hover:bg-rose-500 px-4 py-2 text-xs font-semibold text-white shadow-md cursor-pointer"
        >
          {$_('mqtt.btnClearLogs')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Single Log Payload Inspection Modal -->
{#if selectedLog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="w-full max-w-2xl rounded-2xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-3.5 bg-slate-50 dark:bg-slate-950/60">
        <div class="flex items-center gap-2 text-cyan-600 dark:text-cyan-400">
          <FileText class="h-4 w-4" />
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">{$_('mqtt.logDetailTitle')}</h3>
        </div>
        <button
          onclick={() => (selectedLog = null)}
          class="rounded-lg p-1 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="p-6 space-y-4">
        <div class="grid grid-cols-2 gap-4 text-xs">
          <div>
            <span class="text-slate-500 dark:text-slate-400 block mb-0.5">{$_('mqtt.logDetailTime')}</span>
            <span class="font-mono text-slate-800 dark:text-slate-200">{renderTime(selectedLog.time)}</span>
          </div>
          <div>
            <span class="text-slate-500 dark:text-slate-400 block mb-0.5">{$_('mqtt.logDetailRemote')}</span>
            <span class="font-mono text-slate-800 dark:text-slate-200">{selectedLog.src}</span>
          </div>
          <div>
            <span class="text-slate-500 dark:text-slate-400 block mb-0.5">{$_('mqtt.logDetailClientId')}</span>
            <span class="font-mono text-slate-800 dark:text-slate-200">{selectedLog.clientID}</span>
          </div>
          <div>
            <span class="text-slate-500 dark:text-slate-400 block mb-0.5">{$_('mqtt.logDetailTopic')}</span>
            <span class="font-mono text-cyan-600 dark:text-cyan-300 font-semibold">{selectedLog.topic || "-"}</span>
          </div>
        </div>

        <div>
          <span class="text-xs text-slate-500 dark:text-slate-400 block mb-1">{$_('mqtt.logDetailPayload')}</span>
          <div class="p-3 bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 rounded-xl overflow-x-auto max-h-72">
            {@html formatJsonOrText(selectedLog.payload)}
          </div>
        </div>
      </div>

      <div class="flex justify-end border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 px-6 py-3">
        <button
          onclick={() => (selectedLog = null)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer shadow-xs"
        >
          {$_('mqtt.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
