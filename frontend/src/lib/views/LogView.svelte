<script lang="ts">
  import { onMount } from "svelte";
  import { fetchEventLogs, queryParquetLogs, getLogCounts, askAI, type EventLogEnt, type ParquetLogRecord } from "../api";
  import { getStateColor, getStateName, formatTimeStr } from "../common";
  import {
    Search,
    RefreshCw,
    FileText,
    Sparkles,
    Filter,
    Server,
    AlertTriangle,
    BarChart3,
    Download,
    Activity,
    Radio
  } from "@lucide/svelte";

  type LogCategory = "event" | "syslog" | "trap" | "netflow" | "otel" | "mqtt";

  let activeTab = $state<LogCategory>("event");
  let eventLogs = $state<EventLogEnt[]>([]);
  let parquetLogs = $state<ParquetLogRecord[]>([]);
  let logCounts = $state<Record<string, number>>({});
  let searchQuery = $state("");
  let levelFilter = $state("all");
  let loading = $state(false);

  // Categories definition
  const categories: { id: LogCategory; name: string; icon: any }[] = [
    { id: "event", name: "イベントログ", icon: FileText },
    { id: "syslog", name: "Syslog", icon: Server },
    { id: "trap", name: "SNMP TRAP", icon: AlertTriangle },
    { id: "netflow", name: "NetFlow", icon: BarChart3 },
    { id: "otel", name: "OpenTelemetry", icon: Activity },
    { id: "mqtt", name: "MQTT", icon: Radio },
  ];

  // AI Dialog state
  let showAIDialog = $state(false);
  let selectedLogText = $state("");
  let aiAnswer = $state("");
  let aiLoading = $state(false);

  const refreshCounts = async () => {
    try {
      const counts = await getLogCounts();
      if (counts) {
        logCounts = counts;
      }
    } catch (e) {
      console.error(e);
    }
  };

  const loadCurrentLogs = async () => {
    loading = true;
    try {
      refreshCounts();
      if (activeTab === "event") {
        eventLogs = await fetchEventLogs();
        logCounts["event"] = eventLogs.length;
      } else {
        parquetLogs = await queryParquetLogs(activeTab, searchQuery);
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  onMount(() => {
    loadCurrentLogs();
    refreshCounts();
  });

  $effect(() => {
    activeTab;
    loadCurrentLogs();
  });

  const handleAskAI = async (logText: string) => {
    selectedLogText = logText;
    showAIDialog = true;
    aiLoading = true;
    aiAnswer = "";
    try {
      aiAnswer = await askAI(`以下のネットワークログを解析し、根本原因と推奨対策を説明してください。\n\nログ:\n${logText}`);
    } catch (e: any) {
      aiAnswer = `AI解析エラー: ${e.message}`;
    } finally {
      aiLoading = false;
    }
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

  // Filtered Event Logs
  const filteredEventLogs = $derived(
    (eventLogs || []).filter((el) => {
      const q = searchQuery.toLowerCase();
      const eventText = (el.event || (el as any).Event || "").toLowerCase();
      const nodeText = (el.node_name || (el as any).NodeName || el.node_id || (el as any).NodeID || "").toLowerCase();
      const typeText = (el.type || (el as any).Type || "").toLowerCase();
      const levelText = (el.level || (el as any).Level || "").toLowerCase();

      const matchSearch =
        !q ||
        eventText.includes(q) ||
        nodeText.includes(q) ||
        typeText.includes(q) ||
        levelText.includes(q);

      const matchLevel = levelFilter === "all" || levelText === levelFilter.toLowerCase();
      return matchSearch && matchLevel;
    })
  );

  // Filtered Parquet Logs
  const filteredParquetLogs = $derived(
    (parquetLogs || []).filter((pl) => {
      const q = searchQuery.toLowerCase();
      const logText = (pl.log || (pl as any).Log || "").toLowerCase();
      const srcText = (pl.src || (pl as any).Src || "").toLowerCase();
      const typeText = (pl.type || (pl as any).Type || "").toLowerCase();
      return !q || logText.includes(q) || srcText.includes(q) || typeText.includes(q);
    })
  );

  const currentCount = $derived(
    activeTab === "event" ? filteredEventLogs.length : filteredParquetLogs.length
  );

  const exportCSV = () => {
    let csv = "";
    const filename = `twsnmp_${activeTab}_logs_${Date.now()}.csv`;
    if (activeTab === "event") {
      csv = "日時,レベル,種別,ノード,イベント内容\n" +
        filteredEventLogs.map((l) =>
          `"${formatTimeStr(l.time)}","${l.level}","${l.type}","${l.node_name || l.node_id || ''}","${(l.event || '').replace(/"/g, '""')}"`
        ).join("\n");
    } else {
      csv = "日時,送信元,ログデータ\n" +
        filteredParquetLogs.map((l) =>
          `"${formatTimeStr(l.time)}","${l.src}","${(l.log || '').replace(/"/g, '""')}"`
        ).join("\n");
    }
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = filename;
    link.click();
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Left Sidebar (Matching ListView / ReportView) -->
  <div class="w-64 border-r border-slate-800 bg-slate-950/70 p-3 space-y-1.5 shrink-0 flex flex-col justify-between">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
        ログ種別 (Log Type)
      </div>

      {#each categories as cat}
        {@const count = logCounts[cat.id] ?? (cat.id === "event" ? eventLogs.length : (activeTab === cat.id ? parquetLogs.length : 0))}
        <button
          type="button"
          onclick={() => {
            activeTab = cat.id;
            searchQuery = "";
            levelFilter = "all";
            loadCurrentLogs();
          }}
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
        >
          <div class="flex items-center gap-2.5 truncate">
            <cat.icon class="h-4 w-4 shrink-0 {activeTab === cat.id ? 'text-white' : 'text-cyan-400'}" />
            <span class="truncate">{cat.name}</span>
          </div>
          <span class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeTab === cat.id ? 'bg-white/20 text-white' : 'bg-slate-800 text-slate-400'}">
            {count}
          </span>
        </button>
      {/each}
    </div>

    <!-- Live Status & Stats Card -->
    <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-3 text-[11px] text-slate-400 space-y-2">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-200">データ同期</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-950/80 border border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-400">
          ● リアルタイム
        </span>
      </div>
      <div class="text-[10px] font-mono text-slate-400 pt-1 border-t border-slate-800/80">
        表示件数: <span class="text-cyan-400 font-bold">{currentCount}</span> 件
      </div>
    </div>
  </div>

  <!-- Right Main Content Canvas -->
  <div class="flex-1 overflow-hidden flex flex-col p-5 gap-4 min-w-0">
    <!-- Top Action Bar -->
    <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4 shadow-lg shrink-0">
      <div class="flex items-center gap-3">
        <!-- Search Input -->
        <div class="relative w-72">
          <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder={activeTab === "event" ? "イベント内容・ノード名で検索..." : "ログ内容・送信元で検索..."}
            bind:value={searchQuery}
            onkeydown={(e) => e.key === "Enter" && loadCurrentLogs()}
            class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
          />
        </div>

        <!-- Level Filter (for Event Logs) -->
        {#if activeTab === "event"}
          <select
            bind:value={levelFilter}
            class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none cursor-pointer"
          >
            <option value="all">すべてのレベル</option>
            <option value="info">INFO (情報)</option>
            <option value="warn">WARN (注意)</option>
            <option value="high">HIGH (重度障害)</option>
            <option value="normal">NORMAL (正常/復旧)</option>
          </select>
        {/if}
      </div>

      <div class="flex items-center gap-2.5">
        <button
          type="button"
          onclick={loadCurrentLogs}
          disabled={loading}
          class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
        >
          <RefreshCw class="h-3.5 w-3.5 text-cyan-400 {loading ? 'animate-spin' : ''}" />
          <span>更新</span>
        </button>
        <button
          type="button"
          onclick={exportCSV}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
        >
          <Download class="h-3.5 w-3.5" />
          <span>CSV出力</span>
        </button>
      </div>
    </div>

    <!-- Table Container -->
    <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
      <div class="h-full overflow-y-auto overflow-x-auto">
        {#if activeTab === "event"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-48">日時</th>
                <th class="py-2.5 px-3.5 w-28">レベル</th>
                <th class="py-2.5 px-3.5 w-28">種別</th>
                <th class="py-2.5 px-3.5 w-48">ノード</th>
                <th class="py-2.5 px-3.5">イベント内容</th>
                <th class="py-2.5 px-3.5 text-center w-14">AI</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredEventLogs as el}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3.5 text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(el.time)}</td>
                  <td class="py-2 px-3.5">
                    <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getLevelBadge(el.level)}">
                      <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(el.level)}"></span>
                      {el.level}
                    </span>
                  </td>
                  <td class="py-2 px-3.5 font-semibold text-cyan-400 font-sans">{el.type}</td>
                  <td class="py-2 px-3.5 font-sans text-slate-200">{el.node_name || "-"}</td>
                  <td class="py-2 px-3.5 text-slate-100 break-all font-sans">{el.event}</td>
                  <td class="py-2 px-3.5 text-center font-sans">
                    <button
                      type="button"
                      onclick={() => handleAskAI(`${el.type} (${el.level}): ${el.event} [Node: ${el.node_name || el.node_id}]`)}
                      title="AI分析"
                      aria-label="AI分析"
                      class="inline-flex items-center justify-center rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-1.5 text-cyan-300 hover:bg-cyan-500/20 hover:text-cyan-200 transition-all cursor-pointer"
                    >
                      <Sparkles class="h-3.5 w-3.5" />
                    </button>
                  </td>
                </tr>
              {/each}
              {#if filteredEventLogs.length === 0}
                <tr>
                  <td colspan="6" class="py-12 text-center text-slate-500 font-sans">
                    イベントログがありません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        {:else}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-48">日時</th>
                <th class="py-2.5 px-3.5 w-40">送信元 (Src)</th>
                <th class="py-2.5 px-3.5">ログデータ</th>
                <th class="py-2.5 px-3.5 text-center w-14">AI</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredParquetLogs as pl}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3.5 text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(pl.time ?? (pl as any).Time)}</td>
                  <td class="py-2 px-3.5 font-semibold text-cyan-400 font-mono">{pl.src || (pl as any).Src || "-"}</td>
                  <td class="py-2 px-3.5 text-slate-100 break-all font-mono text-[11px]">{pl.log || (pl as any).Log || "-"}</td>
                  <td class="py-2 px-3.5 text-center font-sans">
                    <button
                      type="button"
                      onclick={() => handleAskAI(`${pl.type || (pl as any).Type} from ${pl.src || (pl as any).Src}: ${pl.log || (pl as any).Log}`)}
                      title="AI分析"
                      aria-label="AI分析"
                      class="inline-flex items-center justify-center rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-1.5 text-cyan-300 hover:bg-cyan-500/20 hover:text-cyan-200 transition-all cursor-pointer"
                    >
                      <Sparkles class="h-3.5 w-3.5" />
                    </button>
                  </td>
                </tr>
              {/each}
              {#if filteredParquetLogs.length === 0}
                <tr>
                  <td colspan="4" class="py-12 text-center text-slate-500 font-sans">
                    該当するログは見つかりませんでした
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        {/if}
      </div>
    </div>
  </div>

  <!-- AI Analysis Modal -->
  {#if showAIDialog}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div class="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2 text-base font-bold text-cyan-400">
            <Sparkles class="h-5 w-5" />
            <span>AI ログ診断アシスタント</span>
          </div>
          <button onclick={() => (showAIDialog = false)} class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-white cursor-pointer">✕</button>
        </div>

        <div class="my-3 rounded-xl border border-slate-800 bg-slate-950 p-3 font-mono text-xs text-slate-300">
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
