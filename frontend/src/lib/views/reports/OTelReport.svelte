<script lang="ts">
  import { onMount } from "svelte";
  import { _, locale } from "svelte-i18n";
  import {
    Activity,
    Server,
    Layers,
    Clock,
    Download,
    ExternalLink,
    Sparkles,
    Search,
  } from "@lucide/svelte";
  import {
    fetchOTelMetrics,
    fetchOTelTraces,
    type OTelMetricEnt,
    type OTelTraceSummaryEnt,
  } from "../../api";
  import { formatTimeStr } from "../../common";
  import ReportAIExplainModal from "../../components/ReportAIExplainModal.svelte";

  interface Props {
    searchQuery?: string;
    loading?: boolean;
    onRefresh?: () => void;
  }

  let {
    searchQuery = "",
    loading = false,
    onRefresh,
  }: Props = $props();

  let localSearch = $state("");
  let showAiModal = $state(false);
  let metrics = $state<OTelMetricEnt[]>([]);
  let traces = $state<OTelTraceSummaryEnt[]>([]);
  let dataLoading = $state(false);

  let page = $state(1);
  let pageSize = $state(20);
  let sortKey = $state<string>("Count");
  let sortDir = $state<"asc" | "desc">("desc");

  const loadOTelReportData = async () => {
    dataLoading = true;
    try {
      const [m, tr] = await Promise.all([
        fetchOTelMetrics().catch(() => []),
        fetchOTelTraces(undefined, 500).catch(() => []),
      ]);
      metrics = m;
      traces = tr;
    } catch {
      // ignore
    } finally {
      dataLoading = false;
    }
  };

  onMount(() => {
    void loadOTelReportData();
  });

  export const refresh = () => {
    void loadOTelReportData();
  };

  // KPIs
  const uniqueServices = $derived(
    new Set(metrics.map((m) => m.Service).filter(Boolean)).size
  );
  const uniqueHosts = $derived(
    new Set(metrics.map((m) => m.Host).filter(Boolean)).size
  );
  const slowTraces = $derived(
    traces.filter((t) => t.Dur * 1000 >= 500)
  );

  // Filtered & Sorted Metrics
  const filteredMetrics = $derived.by(() => {
    const q = (localSearch || searchQuery || "").toLowerCase();
    let res = (metrics || []).filter((m) => {
      if (!q) return true;
      return (
        m.Name.toLowerCase().includes(q) ||
        m.Service.toLowerCase().includes(q) ||
        m.Host.toLowerCase().includes(q) ||
        m.Type.toLowerCase().includes(q)
      );
    });

    res = [...res].sort((a: any, b: any) => {
      const valA = a[sortKey] ?? 0;
      const valB = b[sortKey] ?? 0;
      if (typeof valA === "number" && typeof valB === "number") {
        return sortDir === "asc" ? valA - valB : valB - valA;
      }
      return sortDir === "asc"
        ? String(valA).localeCompare(String(valB))
        : String(valB).localeCompare(String(valA));
    });

    return res;
  });

  const pagedMetrics = $derived(
    filteredMetrics.slice((page - 1) * pageSize, page * pageSize)
  );

  const totalPages = $derived(
    Math.max(1, Math.ceil(filteredMetrics.length / pageSize))
  );

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);

  // AI Summary & Items
  const aiSummaryText = $derived(
    isJaLocale
      ? `OpenTelemetry 総合メトリクス系列数: ${metrics.length}系列, 稼働サービス数: ${uniqueServices}サービス, 監視ホスト数: ${uniqueHosts}台, 収集トレース数: ${traces.length}件, レイテンシ遅延トレース(≥500ms): ${slowTraces.length}件`
      : `Total OTel Metric Series: ${metrics.length}, Active Services: ${uniqueServices}, Monitored Hosts: ${uniqueHosts}, Collected Traces: ${traces.length}, High Latency Traces (≥500ms): ${slowTraces.length}`
  );

  const aiItemsList = $derived.by(() => {
    const items: string[] = [];

    // Service aggregation
    const serviceMap = new Map<string, { metrics: number; hosts: Set<string>; types: Set<string> }>();
    for (const m of metrics) {
      const s = m.Service || "(Unknown)";
      const ex = serviceMap.get(s) || { metrics: 0, hosts: new Set<string>(), types: new Set<string>() };
      ex.metrics += 1;
      if (m.Host) ex.hosts.add(m.Host);
      if (m.Type) ex.types.add(m.Type);
      serviceMap.set(s, ex);
    }

    for (const [srv, info] of [...serviceMap.entries()].sort((a, b) => b[1].metrics - a[1].metrics).slice(0, 10)) {
      items.push(isJaLocale
        ? `[サービス概況] Service: ${srv}, メトリック系列数: ${info.metrics}, ホスト数: ${info.hosts.size}, 種別: ${Array.from(info.types).join("/")}`
        : `[Service Overview] Service: ${srv}, Metric Series: ${info.metrics}, Hosts: ${info.hosts.size}, Types: ${Array.from(info.types).join("/")}`);
    }

    // Slow traces
    for (const tr of slowTraces.slice(0, 10)) {
      items.push(isJaLocale
        ? `[遅延トレース警告] TraceID: ${tr.TraceID}, 所要時間: ${(tr.Dur * 1000).toFixed(2)} ms, スパン数: ${tr.NumSpan || 0}, サービス: ${tr.Services || "-"}`
        : `[Slow Trace Warning] TraceID: ${tr.TraceID}, Duration: ${(tr.Dur * 1000).toFixed(2)} ms, Spans: ${tr.NumSpan || 0}, Services: ${tr.Services || "-"}`);
    }

    // Top metrics by update count
    const topMetrics = [...metrics].sort((a, b) => (b.Count || 0) - (a.Count || 0)).slice(0, 10);
    for (const m of topMetrics) {
      items.push(isJaLocale
        ? `[頻出メトリック] Name: ${m.Name}, Service: ${m.Service}, Host: ${m.Host}, Type: ${m.Type}, Count: ${m.Count}`
        : `[Top Metric] Name: ${m.Name}, Service: ${m.Service}, Host: ${m.Host}, Type: ${m.Type}, Count: ${m.Count}`);
    }

    return items;
  });

  const handleSort = (key: string) => {
    if (sortKey === key) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      sortKey = key;
      sortDir = key === "Count" || key === "Last" || key === "First" ? "desc" : "asc";
    }
  };

  const jumpToOTelView = () => {
    window.dispatchEvent(new CustomEvent("twsnmp:change-page", { detail: "otel" }));
  };

  export const exportCSV = () => {
    const header = ["Metric Name", "Type", "Service", "Host", "Scope", "Unit", "Count", "Last Time"];
    const rows = filteredMetrics.map((m) => [
      `"${m.Name.replace(/"/g, '""')}"`,
      m.Type,
      `"${(m.Service || "").replace(/"/g, '""')}"`,
      `"${m.Host || ""}"`,
      `"${m.Scope || ""}"`,
      `"${m.Unit || ""}"`,
      m.Count || 0,
      formatTimeStr(m.Last),
    ]);
    const csvContent = "\uFEFF" + [header.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `otel_report_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };
</script>

<div class="space-y-6">
  <!-- Top KPI Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Card 1: Metric Series -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("otel.metricSeries") || "メトリック系列数"}</p>
        <h3 class="text-2xl font-bold font-mono text-slate-800 dark:text-slate-100 mt-1">{metrics.length.toLocaleString()}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("otel.metricSeriesDesc") || "収集メトリクス系列"}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-600 dark:text-cyan-400">
        <Activity class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 2: Services -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("otel.serviceCount") || "稼働サービス数"}</p>
        <h3 class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400 mt-1">{uniqueServices}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("otel.microservicesDesc") || "マイクロサービス"}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
        <Layers class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 3: Hosts -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("otel.hostCount") || "監視ホスト数"}</p>
        <h3 class="text-2xl font-bold font-mono text-indigo-600 dark:text-indigo-400 mt-1">{uniqueHosts}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("otel.sourceNodesDesc") || "送信元ノード"}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
        <Server class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 4: Slow Traces -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("otel.slowTraces") || "遅延トレース (≥500ms)"}</p>
        <h3 class="text-2xl font-bold font-mono {slowTraces.length > 0 ? 'text-amber-500 dark:text-amber-400' : 'text-slate-700 dark:text-slate-200'} mt-1">
          {slowTraces.length}
        </h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("otel.ofTotalTraces", { values: { total: traces.length } }) || `全 ${traces.length} トレース中`}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl {slowTraces.length > 0 ? 'bg-amber-500/10 border-amber-500/30 text-amber-500' : 'bg-slate-500/10 border-slate-500/30 text-slate-400'} flex items-center justify-center">
        <Clock class="h-5 w-5" />
      </div>
    </div>
  </div>

  <!-- Action Header Bar -->
  <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm">
    <div class="flex items-center gap-3">
      <div class="relative w-64">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder={$_("otel.searchPlaceholderReport") || "メトリック、サービス、ホスト検索..."}
          bind:value={localSearch}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>
      <span class="text-xs text-slate-500">
        {$_("otel.allSeriesCount", { values: { count: filteredMetrics.length } }) || `全 ${filteredMetrics.length} 系列`}
      </span>
    </div>

    <div class="flex items-center gap-2.5">
      <button
        type="button"
        onclick={() => (showAiModal = true)}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-indigo-600/25 transition-all cursor-pointer"
      >
        <Sparkles class="h-3.5 w-3.5 text-cyan-200" />
        <span>{$_("report.btnAiOtel") || "AI OTel総合診断"}</span>
      </button>

      <button
        type="button"
        onclick={jumpToOTelView}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
        title={$_("otel.btnOpenOTelMonitorTooltip") || "独立したOpenTelemetryモニター画面（トレースDAGや詳細時系列チャート）を開きます"}
      >
        <ExternalLink class="h-3.5 w-3.5 text-slate-500 dark:text-slate-400" />
        <span>{$_("otel.btnOpenOTelMonitor") || "OTelモニター画面へ"}</span>
      </button>

      <button
        type="button"
        onclick={exportCSV}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
      >
        <Download class="h-3.5 w-3.5" />
        <span>CSV</span>
      </button>
    </div>
  </div>

  <!-- Metrics Table -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
        <thead class="bg-slate-100/90 dark:bg-slate-950/80 text-slate-600 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800 uppercase font-semibold">
          <tr>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Name")}>
              {$_("otel.colMetricName") || "メトリック名 (Metric Name)"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Type")}>
              {$_("otel.colType") || "種別"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Service")}>
              {$_("otel.colService") || "サービス"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Host")}>
              {$_("otel.colHost") || "ホスト"}
            </th>
            <th class="py-2.5 px-3 text-right cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Count")}>
              {$_("otel.colCount") || "受信回数"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Last")}>
              {$_("otel.colLastTime") || "最終更新"}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60 font-mono text-[11px]">
          {#if pagedMetrics.length === 0}
            <tr>
              <td colspan="6" class="py-8 text-center text-slate-500">
                {$_("otel.noMetricData") || "メトリックデータがありません"}
              </td>
            </tr>
          {:else}
            {#each pagedMetrics as m}
              <tr class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-100 truncate max-w-[280px]" title={m.Name}>
                  {m.Name}
                  {#if m.Unit}
                    <span class="ml-1 text-[10px] text-cyan-600 dark:text-cyan-400 font-normal">({m.Unit})</span>
                  {/if}
                </td>
                <td class="py-1 px-2">
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700">
                    {m.Type}
                  </span>
                </td>
                <td class="py-1 px-2 text-cyan-600 dark:text-cyan-300 font-sans truncate max-w-[160px]">
                  {m.Service}
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-400">{m.Host}</td>
                <td class="py-1 px-2 text-right text-emerald-600 dark:text-emerald-400 font-bold">
                  {(m.Count || 0).toLocaleString()}
                </td>
                <td class="py-1 px-2 text-slate-500 whitespace-nowrap">
                  {formatTimeStr(m.Last)}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    {#if totalPages > 1}
      <div class="flex items-center justify-between p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40 text-xs">
        <span class="text-slate-500">
          {(page - 1) * pageSize + 1} - {Math.min(page * pageSize, filteredMetrics.length)} / {filteredMetrics.length} 系列
        </span>
        <div class="flex items-center gap-1">
          <button
            disabled={page <= 1}
            onclick={() => (page = page - 1)}
            class="px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-50 cursor-pointer"
          >
            {$_("mqtt.prev") || "前へ"}
          </button>
          <span class="px-2 text-slate-600 dark:text-slate-400">{page} / {totalPages}</span>
          <button
            disabled={page >= totalPages}
            onclick={() => (page = page + 1)}
            class="px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-50 cursor-pointer"
          >
            {$_("mqtt.next") || "次へ"}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

{#if showAiModal}
  <ReportAIExplainModal
    bind:show={showAiModal}
    reportType="otel"
    title={$_("report.aiOtelTitle") || "OpenTelemetry 総合テレメトリ AI診断"}
    summary={aiSummaryText}
    items={aiItemsList}
  />
{/if}
