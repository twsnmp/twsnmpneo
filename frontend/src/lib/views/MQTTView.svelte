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
  import { renderBytes, renderTime } from "../common";
  import { showLogCountChart, resizeLogCountChart, disposeLogCountChart } from "../charts/logcount";
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
    CheckSquare,
    Square,
    ChevronDown,
    ChevronRight,
    Plus,
    Download,
    Eye,
    X,
    ChevronLeft,
    ChevronsLeft,
    ChevronsRight,
    AlertTriangle,
    CheckCircle2,
    Clock,
    Flame,
    BarChart3,
    RotateCcw,
  } from "@lucide/svelte";

  type TabType = "stats" | "logs";

  let activeTab = $state<TabType>("stats");
  let loading = $state(false);

  // --- Stats State ---
  let stats = $state<MqttStatEnt[]>([]);
  let statSearch = $state("");
  let selectedIds = $state<Set<string>>(new Set());
  let expandedRows = $state<Set<string>>(new Set());
  let statPage = $state(1);
  let statPageSize = $state(25);
  let showReportModal = $state(false);
  let copied = $state(false);
  let showDeleteAllConfirm = $state(false);

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
      await refreshStats();
      try {
        nodes = await fetchNodes();
      } catch {
        // ignore
      }
    })();

    const handleResize = () => {
      if (showChart && activeTab === "logs") {
        resizeLogCountChart();
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      disposeLogCountChart();
    };
  });

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

  // --- Stats Filtering & Pagination ---
  const filteredStats = $derived(
    stats.filter((s) => {
      if (!statSearch.trim()) return true;
      const q = statSearch.toLowerCase();
      return (
        (s.Topic && s.Topic.toLowerCase().includes(q)) ||
        (s.ClientID && s.ClientID.toLowerCase().includes(q)) ||
        (s.Remote && s.Remote.toLowerCase().includes(q)) ||
        (s.Value && s.Value.toLowerCase().includes(q))
      );
    })
  );

  const totalStatPages = $derived(Math.max(1, Math.ceil(filteredStats.length / statPageSize)));
  const paginatedStats = $derived(
    filteredStats.slice((statPage - 1) * statPageSize, statPage * statPageSize)
  );

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredStats.length && filteredStats.length > 0) {
      selectedIds.clear();
      selectedIds = new Set(selectedIds);
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

  const copySelectedTopics = () => {
    const topics: string[] = [];
    stats.forEach((s) => {
      if (selectedIds.has(s.ID)) {
        topics.push(s.Topic);
      }
    });
    if (topics.length === 0) return;
    navigator.clipboard.writeText(topics.join("\n"));
    copied = true;
    setTimeout(() => (copied = false), 2000);
  };

  const handleDeleteSelected = async () => {
    if (selectedIds.size === 0) return;
    if (!confirm(`選択した ${selectedIds.size} 件のMQTT統計を削除しますか？`)) return;
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

    // Find candidate node matching Remote IP
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
    if (!raw) return '<div class="text-slate-500 italic">ペイロードなし</div>';
    try {
      const parsed = JSON.parse(raw);
      const pretty = JSON.stringify(parsed, null, 2);
      return `<pre class="font-mono text-xs text-emerald-400 whitespace-pre-wrap"><code>${escapeHtml(pretty)}</code></pre>`;
    } catch {
      return `<pre class="font-mono text-xs text-slate-300 whitespace-pre-wrap"><code>${escapeHtml(raw)}</code></pre>`;
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

  const filteredLogs = $derived(
    parsedLogs.filter((l) => {
      // 1. Chart zoom range filter
      if (chartZoomRange && chartZoomRange.st > 0 && chartZoomRange.et > 0) {
        const itemTime = l.time;
        if (itemTime < chartZoomRange.st || itemTime > chartZoomRange.et) {
          return false;
        }
      }

      // 2. Search query filter
      if (!logSearch.trim()) return true;
      const q = logSearch.toLowerCase();
      return (
        l.topic.toLowerCase().includes(q) ||
        l.clientID.toLowerCase().includes(q) ||
        l.src.toLowerCase().includes(q) ||
        l.payload.toLowerCase().includes(q)
      );
    })
  );

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
    const headers = ["日時", "送信元", "クライアントID", "トピック", "ペイロード"];
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

<div class="flex h-full w-full flex-col overflow-hidden bg-slate-950 text-slate-100">
  <!-- Top View Header & Navigation -->
  <header class="flex shrink-0 items-center justify-between border-b border-slate-800/80 bg-slate-900/90 px-6 py-2.5 backdrop-blur-md">
    <div class="flex items-center gap-4">
      <div class="flex items-center gap-2">
        <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
          <Radio class="h-4 w-4 animate-pulse" />
        </div>
        <div>
          <h2 class="text-sm font-bold tracking-tight text-white flex items-center gap-2">
            MQTT モニター
            <span class="text-[11px] px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 font-mono border border-slate-700">
              Port: 1883
            </span>
          </h2>
        </div>
      </div>

      <!-- Tab Switcher (Requirement 1 & 4) -->
      <div class="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800">
        <button
          onclick={() => switchTab("stats")}
          class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold transition-all {activeTab === 'stats' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
        >
          <Radio class="h-3.5 w-3.5" />
          <span>統計 (Stats)</span>
          <span class="ml-1 text-[10px] px-1.5 py-0.2 rounded-full {activeTab === 'stats' ? 'bg-cyan-700 text-cyan-100' : 'bg-slate-800 text-slate-400'}">
            {stats.length}
          </span>
        </button>
        <button
          onclick={() => switchTab("logs")}
          class="flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold transition-all {activeTab === 'logs' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
        >
          <FileText class="h-3.5 w-3.5" />
          <span>ログ (Logs)</span>
          {#if logs.length > 0}
            <span class="ml-1 text-[10px] px-1.5 py-0.2 rounded-full {activeTab === 'logs' ? 'bg-cyan-700 text-cyan-100' : 'bg-slate-800 text-slate-400'}">
              {logs.length}
            </span>
          {/if}
        </button>
      </div>
    </div>

    <!-- Right Controls -->
    <div class="flex items-center gap-2">
      {#if activeTab === "stats"}
        <button
          onclick={refreshStats}
          disabled={loading}
          class="flex items-center gap-1 rounded-xl border border-slate-800 bg-slate-900 px-3 py-1.5 text-xs font-semibold text-slate-300 hover:bg-slate-800 hover:text-white transition-all disabled:opacity-50"
        >
          <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
          <span>再読込</span>
        </button>
      {:else}
        <button
          onclick={refreshLogs}
          disabled={loading}
          class="flex items-center gap-1 rounded-xl border border-slate-800 bg-slate-900 px-3 py-1.5 text-xs font-semibold text-slate-300 hover:bg-slate-800 hover:text-white transition-all disabled:opacity-50"
        >
          <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
          <span>再読込</span>
        </button>
      {/if}
    </div>
  </header>

  <!-- Content Area -->
  <div class="flex-1 flex flex-col overflow-hidden p-4">
    {#if activeTab === "stats"}
      <!-- STATS VIEW (Requirement 2 matching TWSNMP FK) -->
      <div class="flex-1 flex flex-col rounded-2xl border border-slate-800 bg-slate-900/60 shadow-xl overflow-hidden backdrop-blur-sm">
        <!-- Top Toolbar matching DataTables header -->
        <div class="flex shrink-0 items-center justify-between border-b border-slate-800/80 px-4 py-3 bg-slate-900/40">
          <div class="flex items-center gap-2">
            <span class="text-xs text-slate-400">表示件数:</span>
            <select
              bind:value={statPageSize}
              onchange={() => (statPage = 1)}
              class="rounded-lg border border-slate-800 bg-slate-950 px-2.5 py-1 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none"
            >
              <option value={10}>10</option>
              <option value={25}>25</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
            <span class="text-xs text-slate-400">件</span>
          </div>

          <div class="flex items-center gap-2">
            <div class="relative w-64">
              <Search class="absolute left-2.5 top-2 h-3.5 w-3.5 text-slate-500" />
              <input
                type="text"
                placeholder="トピック / クライアント検索..."
                bind:value={statSearch}
                oninput={() => (statPage = 1)}
                class="w-full rounded-xl border border-slate-800 bg-slate-950/80 pl-8 pr-3 py-1 text-xs text-slate-200 placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
              />
              {#if statSearch}
                <button
                  onclick={() => (statSearch = "")}
                  class="absolute right-2.5 top-2 text-slate-400 hover:text-white"
                >
                  <X class="h-3.5 w-3.5" />
                </button>
              {/if}
            </div>
          </div>
        </div>

        <!-- Main Table matching FK Screenshot Columns -->
        <div class="flex-1 overflow-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead class="sticky top-0 z-10 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400 border-b border-slate-800">
              <tr>
                <th class="w-8 py-1 px-2 text-center whitespace-nowrap">
                  <button onclick={toggleSelectAll} class="text-slate-400 hover:text-white">
                    {#if selectedIds.size > 0 && selectedIds.size === filteredStats.length}
                      <CheckSquare class="h-3.5 w-3.5 text-cyan-400" />
                    {:else}
                      <Square class="h-3.5 w-3.5" />
                    {/if}
                  </button>
                </th>
                <th class="w-6 py-1 px-1 text-center whitespace-nowrap"></th>
                <th class="w-20 py-1 px-2.5 whitespace-nowrap">State</th>
                <th class="w-44 py-1 px-2.5 whitespace-nowrap">Client ID</th>
                <th class="w-32 py-1 px-2.5 whitespace-nowrap">Remote</th>
                <th class="py-1 px-2.5">Topic</th>
                <th class="w-20 py-1 px-2.5 text-right whitespace-nowrap">Count</th>
                <th class="w-24 py-1 px-2.5 text-right whitespace-nowrap">Bytes</th>
                <th class="w-44 py-1 px-2.5 whitespace-nowrap">First Time</th>
                <th class="w-44 py-1 px-2.5 whitespace-nowrap">Last time</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/40 font-mono text-slate-300">
              {#if paginatedStats.length === 0}
                <tr>
                  <td colspan="10" class="py-12 text-center text-slate-500">
                    {#if loading}
                      <div class="flex items-center justify-center gap-2">
                        <RefreshCw class="h-4 w-4 animate-spin text-cyan-400" />
                        <span>データを読み込み中...</span>
                      </div>
                    {:else}
                      MQTTトピック統計データはありません
                    {/if}
                  </td>
                </tr>
              {:else}
                {#each paginatedStats as s (s.ID)}
                  <tr
                    class="hover:bg-slate-800/40 transition-colors {selectedIds.has(s.ID) ? 'bg-cyan-950/20' : ''}"
                  >
                    <!-- Checkbox -->
                    <td class="py-1 px-2 text-center">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(s.ID)}
                        onchange={() => toggleSelectRow(s.ID)}
                        class="h-3.5 w-3.5 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20 cursor-pointer"
                      />
                    </td>

                    <!-- Expand Row Toggle -->
                    <td class="py-1 px-1 text-center">
                      <button
                        onclick={() => toggleExpand(s.ID)}
                        title="ペイロード詳細展開"
                        class="p-0.5 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                      >
                        {#if expandedRows.has(s.ID)}
                          <ChevronDown class="h-3 w-3 text-cyan-400" />
                        {:else}
                          <ChevronRight class="h-3 w-3" />
                        {/if}
                      </button>
                    </td>

                    <!-- State Badge matching FK -->
                    <td class="py-1 px-2.5 font-sans">
                      {#if s.State === "normal"}
                        <span class="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-400 leading-tight">
                          <CheckCircle2 class="h-3 w-3" />
                          Normal
                        </span>
                      {:else if s.State === "warn"}
                        <span class="inline-flex items-center gap-1 text-[11px] font-medium text-amber-400 leading-tight">
                          <AlertTriangle class="h-3 w-3" />
                          Warn
                        </span>
                      {:else}
                        <span class="inline-flex items-center gap-1 text-[11px] font-medium text-rose-400 leading-tight">
                          <AlertTriangle class="h-3 w-3" />
                          Low
                        </span>
                      {/if}
                    </td>

                    <!-- Client ID -->
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 truncate max-w-[180px] leading-tight" title={s.ClientID}>
                      {s.ClientID || "-"}
                    </td>

                    <!-- Remote IP -->
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 leading-tight">
                      {s.Remote || "-"}
                    </td>

                    <!-- Topic -->
                    <td class="py-1 px-2.5 text-cyan-300 break-all font-medium text-[11px] leading-tight">
                      {s.Topic}
                    </td>

                    <!-- Count -->
                    <td class="py-1 px-2.5 text-right text-slate-200 text-[11px] leading-tight">
                      {(s.Count || 0).toLocaleString()}
                    </td>

                    <!-- Bytes -->
                    <td class="py-1 px-2.5 text-right font-sans text-slate-300 text-[11px] leading-tight">
                      {renderBytes(s.Bytes)}
                    </td>

                    <!-- First Time -->
                    <td class="py-1 px-2.5 text-[11px] text-slate-400 leading-tight whitespace-nowrap">
                      {renderTime(s.First)}
                    </td>

                    <!-- Last time -->
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 leading-tight whitespace-nowrap">
                      {renderTime(s.Last)}
                    </td>
                  </tr>

                  <!-- Child Row showing payload when expanded -->
                  {#if expandedRows.has(s.ID)}
                    <tr class="bg-slate-950/80 border-b border-slate-800/80">
                      <td colspan="10" class="px-6 py-3">
                        <div class="flex flex-col gap-1.5">
                          <div class="flex items-center justify-between text-[11px] text-slate-400">
                            <span class="font-semibold text-slate-300 flex items-center gap-1.5">
                              <Eye class="h-3.5 w-3.5 text-cyan-400" />
                              最新ペイロード (Payload):
                            </span>
                            <span class="font-mono text-[10px] text-slate-500">
                              Topic: {s.Topic} ({s.Bytes} bytes)
                            </span>
                          </div>
                          <div class="p-3 bg-slate-900 border border-slate-800 rounded-xl overflow-x-auto max-h-60">
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

        <!-- Bottom Action Bar & Pagination matching TWSNMP FK -->
        <div class="shrink-0 flex flex-wrap items-center justify-between gap-3 border-t border-slate-800/80 px-4 py-3 bg-slate-950/70">
          <!-- Entries info & Pagination -->
          <div class="flex items-center gap-4">
            <span class="text-xs text-slate-400">
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
                  class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-300 hover:bg-slate-800 disabled:opacity-30"
                >
                  Previous
                </button>
                {#each Array.from({ length: totalStatPages }, (_, i) => i + 1) as p}
                  {#if Math.abs(p - statPage) < 3 || p === 1 || p === totalStatPages}
                    <button
                      onclick={() => (statPage = p)}
                      class="h-7 w-7 rounded-lg text-xs font-semibold {statPage === p ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}"
                    >
                      {p}
                    </button>
                  {/if}
                {/each}
                <button
                  onclick={() => (statPage = Math.min(totalStatPages, statPage + 1))}
                  disabled={statPage >= totalStatPages}
                  class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-300 hover:bg-slate-800 disabled:opacity-30"
                >
                  Next
                </button>
              </div>
            {/if}
          </div>

          <!-- Bottom Action Buttons matching FK Screenshot Buttons -->
          <div class="flex items-center gap-2">
            <!-- Select All (purple) -->
            <button
              onclick={toggleSelectAll}
              class="flex items-center gap-1.5 rounded-xl bg-purple-600 hover:bg-purple-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
            >
              <CheckSquare class="h-3.5 w-3.5" />
              <span>Select All</span>
            </button>

            <!-- Deselect All (purple) -->
            {#if selectedIds.size > 0}
              <button
                onclick={() => { selectedIds.clear(); selectedIds = new Set(); }}
                class="flex items-center gap-1.5 rounded-xl bg-purple-700/80 hover:bg-purple-600 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
              >
                <Square class="h-3.5 w-3.5" />
                <span>Deselect All</span>
              </button>
            {/if}

            <!-- Create Polling / AI Assist (blue/pink when 1 selected) -->
            {#if selectedIds.size === 1}
              <button
                onclick={handleCreatePolling}
                class="flex items-center gap-1.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all animate-in fade-in"
              >
                <Plus class="h-3.5 w-3.5" />
                <span>ポーリング作成</span>
              </button>
            {/if}

            <!-- Copy Topic (cyan when >= 1 selected) -->
            {#if selectedIds.size > 0}
              <button
                onclick={copySelectedTopics}
                class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
              >
                {#if copied}
                  <Check class="h-3.5 w-3.5" />
                  <span>コピー完了</span>
                {:else}
                  <Copy class="h-3.5 w-3.5" />
                  <span>トピックコピー</span>
                {/if}
              </button>

              <!-- Delete Selected (red) -->
              <button
                onclick={handleDeleteSelected}
                class="flex items-center gap-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
              >
                <Trash2 class="h-3.5 w-3.5" />
                <span>削除</span>
              </button>
            {/if}

            <!-- Delete All (red) -->
            <button
              onclick={() => (showDeleteAllConfirm = true)}
              class="flex items-center gap-1.5 rounded-xl bg-rose-600/90 hover:bg-rose-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
            >
              <Trash2 class="h-3.5 w-3.5" />
              <span>全データ削除</span>
            </button>

            <!-- Report (green) -->
            {#if stats.length > 0}
              <button
                onclick={() => (showReportModal = true)}
                class="flex items-center gap-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
              >
                <PieChart class="h-3.5 w-3.5" />
                <span>レポート</span>
              </button>
            {/if}

            <!-- Reload (teal) -->
            <button
              onclick={refreshStats}
              class="flex items-center gap-1.5 rounded-xl bg-teal-600 hover:bg-teal-500 text-white px-3 py-1.5 text-xs font-semibold shadow-md transition-all"
            >
              <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
              <span>Reload</span>
            </button>
          </div>
        </div>
      </div>

    {:else}
      <!-- LOGS VIEW (Requirement 3 & 4) -->
      <div class="flex-1 flex flex-col gap-3 overflow-hidden">
        <!-- Reception Count Chart (Collapsible) -->
        {#if showChart}
          <div class="relative rounded-2xl border border-slate-800 bg-slate-900/90 p-3 shadow-lg shrink-0 transition-all">
            <div class="flex items-center justify-between mb-1 px-1">
              <div class="flex items-center gap-2">
                <span class="text-[11px] font-bold text-slate-300">MQTT ログ受信件数推移 (時系列グラフ)</span>
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
              class="flex items-center gap-1 text-[11px] text-cyan-400 hover:text-cyan-300 cursor-pointer"
            >
              <BarChart3 class="h-3.5 w-3.5" />
              <span>受信状況推移グラフを表示 ▼</span>
            </button>
          </div>
        {/if}

        <div class="flex-1 flex flex-col rounded-2xl border border-slate-800 bg-slate-900/60 shadow-xl overflow-hidden backdrop-blur-sm min-h-0">
          <!-- Logs Filter Toolbar -->
          <div class="flex shrink-0 items-center justify-between border-b border-slate-800/80 px-4 py-3 bg-slate-900/40 gap-4 flex-wrap">
          <div class="flex items-center gap-3">
            <div class="relative w-72">
              <Search class="absolute left-2.5 top-2 h-3.5 w-3.5 text-slate-500" />
              <input
                type="text"
                placeholder="ログ / トピック / ペイロード検索..."
                bind:value={logSearch}
                oninput={() => (logPage = 1)}
                class="w-full rounded-xl border border-slate-800 bg-slate-950/80 pl-8 pr-3 py-1 text-xs text-slate-200 placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
              />
              {#if logSearch}
                <button
                  onclick={() => (logSearch = "")}
                  class="absolute right-2.5 top-2 text-slate-400 hover:text-white"
                >
                  <X class="h-3.5 w-3.5" />
                </button>
              {/if}
            </div>

            <!-- Time range presets -->
            <div class="flex items-center gap-1 bg-slate-950 p-0.5 rounded-xl border border-slate-800 text-[11px]">
              {#each [["1h", "1時間"], ["24h", "24時間"], ["7d", "7日間"], ["30d", "30日間"], ["all", "全期間"]] as [val, label]}
                <button
                  onclick={() => { logTimeRange = val; refreshLogs(); }}
                  class="px-2.5 py-1 rounded-lg font-medium transition-all {logTimeRange === val ? 'bg-cyan-600 text-white shadow-sm font-semibold' : 'text-slate-400 hover:text-slate-200'}"
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
              class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800/80 hover:bg-slate-700 text-slate-200 px-3 py-1.5 text-xs font-semibold transition-all disabled:opacity-40"
            >
              <Download class="h-3.5 w-3.5" />
              <span>CSV出力</span>
            </button>

            <!-- Clear Logs -->
            <button
              onclick={() => (showClearLogsConfirm = true)}
              disabled={logs.length === 0}
              class="flex items-center gap-1.5 rounded-xl border border-rose-900/60 bg-rose-950/40 hover:bg-rose-900 text-rose-300 px-3 py-1.5 text-xs font-semibold transition-all disabled:opacity-40"
            >
              <Trash2 class="h-3.5 w-3.5" />
              <span>ログ消去</span>
            </button>
          </div>
        </div>

        <!-- Logs Table -->
        <div class="flex-1 overflow-auto">
          <table class="w-full text-left text-xs border-collapse">
            <thead class="sticky top-0 z-10 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400 border-b border-slate-800">
              <tr>
                <th class="w-44 py-1 px-2.5">日時 (Time)</th>
                <th class="w-36 py-1 px-2.5">送信元 (Remote)</th>
                <th class="w-44 py-1 px-2.5">クライアントID</th>
                <th class="w-64 py-1 px-2.5">トピック (Topic)</th>
                <th class="py-1 px-2.5">ペイロード (Payload)</th>
                <th class="w-10 py-1 px-2 text-center"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/40 font-mono text-slate-300">
              {#if paginatedLogs.length === 0}
                <tr>
                  <td colspan="6" class="py-12 text-center text-slate-500">
                    {#if loading}
                      <div class="flex items-center justify-center gap-2">
                        <RefreshCw class="h-4 w-4 animate-spin text-cyan-400" />
                        <span>Parquetログを読み込み中...</span>
                      </div>
                    {:else}
                      MQTT受信ログは見つかりませんでした
                    {/if}
                  </td>
                </tr>
              {:else}
                {#each paginatedLogs as l}
                  <tr
                    onclick={() => (selectedLog = l)}
                    class="hover:bg-slate-800/40 transition-colors cursor-pointer"
                  >
                    <td class="py-1 px-2.5 text-[11px] text-slate-400 leading-tight">
                      {renderTime(l.time)}
                    </td>
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 leading-tight">
                      {l.src}
                    </td>
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 truncate max-w-[160px] leading-tight" title={l.clientID}>
                      {l.clientID}
                    </td>
                    <td class="py-1 px-2.5 text-[11px] text-cyan-300 truncate max-w-[240px] leading-tight" title={l.topic}>
                      {l.topic || "-"}
                    </td>
                    <td class="py-1 px-2.5 text-[11px] text-slate-300 truncate max-w-[400px] leading-tight">
                      {l.payload}
                    </td>
                    <td class="py-1 px-2 text-center">
                      <button
                        onclick={(e) => { e.stopPropagation(); selectedLog = l; }}
                        class="p-0.5 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
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
        <div class="shrink-0 flex items-center justify-between border-t border-slate-800/80 px-4 py-3 bg-slate-950/70">
          <span class="text-xs text-slate-400">
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
                class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-300 hover:bg-slate-800 disabled:opacity-30"
              >
                Previous
              </button>
              {#each Array.from({ length: totalLogPages }, (_, i) => i + 1) as p}
                {#if Math.abs(p - logPage) < 3 || p === 1 || p === totalLogPages}
                  <button
                    onclick={() => (logPage = p)}
                    class="h-7 w-7 rounded-lg text-xs font-semibold {logPage === p ? 'bg-cyan-600 text-white' : 'text-slate-400 hover:bg-slate-800 hover:text-white'}"
                  >
                    {p}
                  </button>
                {/if}
              {/each}
              <button
                onclick={() => (logPage = Math.min(totalLogPages, logPage + 1))}
                disabled={logPage >= totalLogPages}
                class="rounded-lg px-2 py-1 text-xs font-semibold text-slate-300 hover:bg-slate-800 disabled:opacity-30"
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
    <div class="w-full max-w-md rounded-2xl border border-rose-800/60 bg-slate-900 p-6 shadow-2xl">
      <div class="flex items-center gap-3 text-rose-400 mb-3">
        <AlertTriangle class="h-6 w-6" />
        <h3 class="text-sm font-bold text-white">すべてのMQTT統計を削除しますか？</h3>
      </div>
      <p class="text-xs text-slate-400 mb-6">
        蓄積されたすべてのMQTTトピック・クライアント統計がデータベースから完全に削除されます。この操作は取り消せません。
      </p>
      <div class="flex justify-end gap-2">
        <button
          onclick={() => (showDeleteAllConfirm = false)}
          class="rounded-xl border border-slate-700 bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-300 hover:bg-slate-700"
        >
          キャンセル
        </button>
        <button
          onclick={handleDeleteAll}
          class="rounded-xl bg-rose-600 hover:bg-rose-500 px-4 py-2 text-xs font-semibold text-white shadow-md"
        >
          全データ削除
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Clear Logs Confirmation Modal -->
{#if showClearLogsConfirm}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="w-full max-w-md rounded-2xl border border-rose-800/60 bg-slate-900 p-6 shadow-2xl">
      <div class="flex items-center gap-3 text-rose-400 mb-3">
        <AlertTriangle class="h-6 w-6" />
        <h3 class="text-sm font-bold text-white">MQTTログをすべて消去しますか？</h3>
      </div>
      <p class="text-xs text-slate-400 mb-6">
        Parquetファイルに保存されているMQTT受信ログがすべて削除されます。統計データは削除されません。
      </p>
      <div class="flex justify-end gap-2">
        <button
          onclick={() => (showClearLogsConfirm = false)}
          class="rounded-xl border border-slate-700 bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-300 hover:bg-slate-700"
        >
          キャンセル
        </button>
        <button
          onclick={handleClearLogs}
          class="rounded-xl bg-rose-600 hover:bg-rose-500 px-4 py-2 text-xs font-semibold text-white shadow-md"
        >
          ログ消去
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Single Log Payload Inspection Modal -->
{#if selectedLog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in">
    <div class="w-full max-w-2xl rounded-2xl border border-slate-700 bg-slate-900 shadow-2xl overflow-hidden">
      <div class="flex items-center justify-between border-b border-slate-800 px-6 py-3.5 bg-slate-950/60">
        <div class="flex items-center gap-2 text-cyan-400">
          <FileText class="h-4 w-4" />
          <h3 class="text-sm font-bold text-white">MQTT ログ詳細</h3>
        </div>
        <button
          onclick={() => (selectedLog = null)}
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-white"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="p-6 space-y-4">
        <div class="grid grid-cols-2 gap-4 text-xs">
          <div>
            <span class="text-slate-500 block mb-0.5">受信日時</span>
            <span class="font-mono text-slate-200">{renderTime(selectedLog.time)}</span>
          </div>
          <div>
            <span class="text-slate-500 block mb-0.5">送信元 (Remote)</span>
            <span class="font-mono text-slate-200">{selectedLog.src}</span>
          </div>
          <div>
            <span class="text-slate-500 block mb-0.5">クライアントID</span>
            <span class="font-mono text-slate-200">{selectedLog.clientID}</span>
          </div>
          <div>
            <span class="text-slate-500 block mb-0.5">トピック (Topic)</span>
            <span class="font-mono text-cyan-300 font-semibold">{selectedLog.topic || "-"}</span>
          </div>
        </div>

        <div>
          <span class="text-xs text-slate-500 block mb-1">ペイロード内容 (Payload)</span>
          <div class="p-3 bg-slate-950 border border-slate-800 rounded-xl overflow-x-auto max-h-72">
            {@html formatJsonOrText(selectedLog.payload)}
          </div>
        </div>
      </div>

      <div class="flex justify-end border-t border-slate-800 bg-slate-950/40 px-6 py-3">
        <button
          onclick={() => (selectedLog = null)}
          class="rounded-xl border border-slate-700 bg-slate-800 px-4 py-1.5 text-xs font-semibold text-slate-300 hover:bg-slate-700"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}
