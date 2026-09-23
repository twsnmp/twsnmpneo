<script lang="ts">
  import { onMount, tick } from "svelte";
  import {
    fetchOTelMetrics,
    fetchOTelMetricDetail,
    deleteOTelMetric,
    fetchOTelTraceBuckets,
    fetchOTelTraces,
    fetchOTelTraceDetail,
    fetchOTelDAG,
    fetchOTelLogs,
    deleteAllOTelData,
    type OTelMetricEnt,
    type OTelTraceSummaryEnt,
    type OTelTraceEnt,
    type OTelTraceDAGEnt,
  } from "../api";
  import { formatTimeStr, renderTimeMili } from "../common";
  import {
    showOTelTrace,
    showOTelDAG,
    showOTelTimeline,
    showOTelTimeChart,
    showOTelHistogram,
  } from "../charts/otel";
  import { showLogLevelChart } from "../charts/loglevel";
  import {
    Activity,
    BarChart3,
    Eye,
    FileText,
    GitBranch,
    Info,
    RefreshCw,
    Search,
    Trash2,
    X,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
  } from "@lucide/svelte";

  type TabType = "metric" | "trace" | "log";

  let activeTab = $state<TabType>("metric");
  let loading = $state(false);

  // --- Metric Tab State ---
  let metrics = $state<OTelMetricEnt[]>([]);
  let metricSearch = $state("");
  let selectedMetric = $state<OTelMetricEnt | null>(null);
  let showMetricInfo = $state(false);
  let showMetricReport = $state(false);
  let metricDetail = $state<OTelMetricEnt | null>(null);
  let metricChartMode = $state<"time" | "histogram">("time");
  let selectedDataPoint = $state<any>(null);

  // Metric Pagination
  let metricPage = $state(1);
  let metricPageSize = $state(20);

  // --- Trace Tab State ---
  let traceBuckets = $state<string[]>([]);
  let selectedBuckets = $state<string[]>([]);
  let traces = $state<OTelTraceSummaryEnt[]>([]);
  let traceSearch = $state("");
  let selectedTrace = $state<OTelTraceSummaryEnt | null>(null);
  let showTraceReport = $state(false);
  let traceDetail = $state<OTelTraceEnt | null>(null);
  let showDAGModal = $state(false);
  let dagData = $state<OTelTraceDAGEnt | null>(null);

  // Trace Pagination
  let tracePage = $state(1);
  let tracePageSize = $state(20);

  // --- Log Tab State ---
  let rawLogs = $state<any[]>([]);
  let parsedLogs = $state<any[]>([]);
  let logSearch = $state("");
  let logLevelFilter = $state<string>("all");
  let selectedLog = $state<any>(null);
  let showLogModal = $state(false);

  // Log Pagination
  let logPage = $state(1);
  let logPageSize = $state(25);

  // Charts
  let traceScatterChart: any = null;
  let dagChartInstance: any = null;
  let timelineChartInstance: any = null;
  let metricTimeChartInstance: any = null;
  let logLevelChartInstance: any = null;

  onMount(async () => {
    await refresh();
  });

  const refresh = async () => {
    loading = true;
    try {
      if (activeTab === "metric") {
        const m = await fetchOTelMetrics();
        metrics = Array.isArray(m) ? m : [];
        selectedMetric = null;
      } else if (activeTab === "trace") {
        const bks = await fetchOTelTraceBuckets();
        traceBuckets = Array.isArray(bks) ? bks : [];
        if (selectedBuckets.length === 0 && traceBuckets.length > 0) {
          selectedBuckets = [traceBuckets[traceBuckets.length - 1]];
        } else {
          selectedBuckets = selectedBuckets.filter((b) => traceBuckets.includes(b));
          if (selectedBuckets.length === 0 && traceBuckets.length > 0) {
            selectedBuckets = [traceBuckets[traceBuckets.length - 1]];
          }
        }
        if (selectedBuckets.length > 0) {
          const tr = await fetchOTelTraces(selectedBuckets);
          traces = Array.isArray(tr) ? tr : [];
        } else {
          traces = [];
        }
        selectedTrace = null;
        await tick();
        renderTraceScatter();
      } else if (activeTab === "log") {
        const res = await fetchOTelLogs({ limit: 5000 });
        rawLogs = Array.isArray(res) ? res : [];
        parsedLogs = rawLogs.map((r: any) => {
          let parsed: any = {};
          const rawText = r.log || r.Log || "";
          try {
            parsed = JSON.parse(rawText);
          } catch {
            parsed = {};
            const svcMatch = rawText.match(/service=([^\s|]+)/);
            if (svcMatch) parsed.service = svcMatch[1];
            const scopeMatch = rawText.match(/scope=([^\s|]+)/);
            if (scopeMatch) parsed.scope = scopeMatch[1];
            const tidMatch = rawText.match(/traceId=([^\s|]+)/i);
            if (tidMatch) parsed.traceId = tidMatch[1];

            const barIdx = rawText.indexOf(" | ");
            if (barIdx !== -1) {
              const prefix = rawText.substring(0, barIdx);
              const afterBar = rawText.substring(barIdx + 3).trim();
              try {
                const inner = JSON.parse(afterBar);
                if (inner.resource_metrics || inner.resourceMetrics) {
                  parsed.message = `${prefix} (メトリクスデータ)`;
                } else if (inner.resource_spans || inner.resourceSpans) {
                  parsed.message = `${prefix} (トレースデータ)`;
                } else if (inner.resource_logs || inner.resourceLogs) {
                  parsed.message = `${prefix} (ログデータ)`;
                } else {
                  parsed.message = afterBar;
                }
              } catch {
                parsed.message = rawText;
              }
            } else {
              parsed.message = rawText;
            }
          }

          const time = parsed.time || parsed.Time || r.time || r.Time || 0;
          const host = parsed.host || parsed.Host || r.src || r.Src || "-";
          const service = parsed.service || parsed.Service || "-";
          const scope = parsed.scope || parsed.Scope || "-";
          const traceId = parsed.traceId || parsed.TraceID || parsed.traceID || "-";
          const spanId = parsed.spanId || parsed.SpanID || parsed.spanID || "-";
          const sev = typeof parsed.severity === "number" ? parsed.severity : (typeof parsed.Severity === "number" ? parsed.Severity : 6);
          const sevText = parsed.severityText || parsed.SeverityText || (sev <= 3 ? "ERROR" : sev === 4 ? "WARN" : "INFO");
          const message = parsed.message || parsed.Message || rawText;

          let level = "other";
          if (sev <= 3) level = "high";
          else if (sev === 4) level = "warn";
          else if (sev <= 6) level = "low";

          return {
            time,
            host,
            service,
            scope,
            traceId,
            spanId,
            severity: sev,
            severityText: sevText,
            message,
            attributes: parsed.attributes || parsed.Attributes || {},
            rawText,
            level,
          };
        });
        await tick();
        renderLogLevelChart();
      }
    } catch (e) {
      console.error("OTel refresh error:", e);
    } finally {
      loading = false;
    }
  };

  const handleTabChange = async (tab: TabType) => {
    activeTab = tab;
    await refresh();
  };

  // --- Metric Handlers ---
  const filteredMetrics = $derived(
    (metrics || []).filter((m) => {
      if (!metricSearch) return true;
      const q = metricSearch.toLowerCase();
      return (
        m.Host.toLowerCase().includes(q) ||
        m.Service.toLowerCase().includes(q) ||
        m.Scope.toLowerCase().includes(q) ||
        m.Name.toLowerCase().includes(q) ||
        m.Type.toLowerCase().includes(q)
      );
    })
  );

  const paginatedMetrics = $derived(
    filteredMetrics.slice((metricPage - 1) * metricPageSize, metricPage * metricPageSize)
  );

  const openMetricInfo = () => {
    if (!selectedMetric) return;
    showMetricInfo = true;
  };

  const openMetricReport = async () => {
    if (!selectedMetric) return;
    loading = true;
    try {
      const d = await fetchOTelMetricDetail(
        selectedMetric.Host,
        selectedMetric.Service,
        selectedMetric.Scope,
        selectedMetric.Name
      );
      metricDetail = d;
      selectedDataPoint = d?.DataPoints && d.DataPoints.length > 0 ? d.DataPoints[0] : null;
      metricChartMode = d?.Type === "Histogram" ? "histogram" : "time";
      showMetricReport = true;
      await tick();
      renderMetricChart();
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  const renderMetricChart = () => {
    if (!metricDetail?.DataPoints || metricDetail.DataPoints.length === 0) return;
    if (metricChartMode === "histogram" && selectedDataPoint?.BucketCounts) {
      if (metricTimeChartInstance) {
        metricTimeChartInstance.dispose();
        metricTimeChartInstance = null;
      }
      showOTelHistogram("metricReportChart", selectedDataPoint);
    } else {
      showOTelTimeChart("metricReportChart", metricDetail.DataPoints);
    }
  };

  // --- Trace Handlers ---
  const filteredTraces = $derived(
    (traces || []).filter((t) => {
      if (!traceSearch) return true;
      const q = traceSearch.toLowerCase();
      return (
        t.TraceID.toLowerCase().includes(q) ||
        t.Hosts.toLowerCase().includes(q) ||
        t.Services.toLowerCase().includes(q) ||
        t.Scopes.toLowerCase().includes(q)
      );
    })
  );

  const paginatedTraces = $derived(
    filteredTraces.slice((tracePage - 1) * tracePageSize, tracePage * tracePageSize)
  );

  const renderTraceScatter = () => {
    const el = document.getElementById("traceScatterChart");
    if (!el) return;
    if (traceScatterChart) {
      traceScatterChart.dispose();
    }
    traceScatterChart = showOTelTrace(el, traces || []);
  };

  const openDAGModal = async () => {
    if (selectedBuckets.length === 0) return;
    loading = true;
    try {
      dagData = await fetchOTelDAG(selectedBuckets);
      showDAGModal = true;
      await tick();
      const el = document.getElementById("dagChart");
      if (el) {
        if (dagChartInstance) dagChartInstance.dispose();
        dagChartInstance = showOTelDAG(el, dagData);
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  const openTraceReport = async () => {
    if (!selectedTrace) return;
    loading = true;
    try {
      const d = await fetchOTelTraceDetail(selectedTrace.Bucket, selectedTrace.TraceID);
      traceDetail = d;
      showTraceReport = true;
      await tick();
      const el = document.getElementById("traceWaterfallChart");
      if (el && traceDetail) {
        if (timelineChartInstance) timelineChartInstance.dispose();
        timelineChartInstance = showOTelTimeline(el, traceDetail);
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  // --- Log Handlers ---
  const filteredLogs = $derived(
    (parsedLogs || []).filter((l) => {
      if (logLevelFilter !== "all" && l.level !== logLevelFilter) return false;
      if (!logSearch) return true;
      const q = logSearch.toLowerCase();
      return (
        l.host.toLowerCase().includes(q) ||
        l.service.toLowerCase().includes(q) ||
        l.scope.toLowerCase().includes(q) ||
        l.traceId.toLowerCase().includes(q) ||
        l.message.toLowerCase().includes(q)
      );
    })
  );

  const paginatedLogs = $derived(
    filteredLogs.slice((logPage - 1) * logPageSize, logPage * logPageSize)
  );

  const renderLogLevelChart = () => {
    const el = document.getElementById("otelLogChart");
    if (!el) return;
    if (logLevelChartInstance) {
      logLevelChartInstance.dispose();
    }
    logLevelChartInstance = showLogLevelChart(el, parsedLogs || []);
  };

  // Global Delete
  const handleDeleteAll = async () => {
    if (!confirm("保存されているすべてのOpenTelemetryデータ（メトリック、トレース、ログ）を一括消去しますか？")) {
      return;
    }
    loading = true;
    try {
      await deleteAllOTelData();
      await refresh();
    } finally {
      loading = false;
    }
  };
</script>

<div class="flex h-full w-full flex-col bg-slate-950 text-slate-100 overflow-hidden font-sans">
  <!-- Top View Subheader / Navigation Tabs -->
  <div class="flex items-center justify-between border-b border-slate-800 bg-slate-900/80 px-6 py-2 shrink-0">
    <div class="flex items-center gap-2">
      <button
        onclick={() => handleTabChange("metric")}
        class="flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'metric' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
      >
        <BarChart3 class="h-4 w-4" />
        <span>メトリック (Metric)</span>
      </button>

      <button
        onclick={() => handleTabChange("trace")}
        class="flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'trace' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
      >
        <Eye class="h-4 w-4" />
        <span>トレース (Trace)</span>
      </button>

      <button
        onclick={() => handleTabChange("log")}
        class="flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'log' ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
      >
        <FileText class="h-4 w-4" />
        <span>ログ (Log)</span>
      </button>
    </div>

    <!-- Action Buttons -->
    <div class="flex items-center gap-2">
      {#if activeTab === "metric" && selectedMetric}
        <button
          onclick={openMetricInfo}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-700 transition-colors shadow-sm"
        >
          <Info class="h-3.5 w-3.5 text-cyan-400" />
          <span>メトリック情報</span>
        </button>

        <button
          onclick={openMetricReport}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 text-xs font-medium border border-emerald-500/40 transition-colors shadow-sm"
        >
          <Activity class="h-3.5 w-3.5" />
          <span>レポート</span>
        </button>
      {/if}

      {#if activeTab === "trace"}
        <button
          onclick={openDAGModal}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600/20 hover:bg-indigo-600/30 text-indigo-300 text-xs font-medium border border-indigo-500/40 transition-colors shadow-sm"
        >
          <GitBranch class="h-3.5 w-3.5" />
          <span>サービス DAG</span>
        </button>

        {#if selectedTrace}
          <button
            onclick={openTraceReport}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 text-xs font-medium border border-emerald-500/40 transition-colors shadow-sm"
          >
            <Activity class="h-3.5 w-3.5" />
            <span>レポート</span>
          </button>
        {/if}
      {/if}

      <button
        onclick={handleDeleteAll}
        title="全OpenTelemetryデータを消去"
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 text-xs font-medium border border-rose-500/40 transition-colors shadow-sm"
      >
        <Trash2 class="h-3.5 w-3.5" />
        <span>全データ削除</span>
      </button>

      <button
        onclick={refresh}
        title="データを更新"
        disabled={loading}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium border border-slate-700 transition-colors shadow-sm"
      >
        <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin text-cyan-400' : ''}" />
        <span>再読み込み</span>
      </button>
    </div>
  </div>

  <!-- Content Body -->
  <div class="flex-1 flex flex-col min-h-0 overflow-hidden p-4">
    <!-- ================= METRIC TAB ================= -->
    {#if activeTab === "metric"}
      <div class="flex flex-col h-full bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg">
        <!-- Search and Filter Toolbar -->
        <div class="flex items-center justify-between p-3 border-b border-slate-800 bg-slate-900/60">
          <div class="relative w-80">
            <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
            <input
              type="text"
              bind:value={metricSearch}
              placeholder="ホスト、サービス、名前で検索..."
              class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-950 border border-slate-800 rounded-lg text-slate-200 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
            />
          </div>
          <div class="text-xs text-slate-400">
            全 <span class="font-bold text-slate-200">{filteredMetrics.length}</span> 件
          </div>
        </div>

        <!-- Metric Table -->
        <div class="flex-1 overflow-auto">
          <table class="w-full text-left text-xs text-slate-300 border-collapse">
            <thead class="sticky top-0 bg-slate-950 text-slate-400 font-semibold border-b border-slate-800 z-10">
              <tr>
                <th class="py-2.5 px-3">ホスト</th>
                <th class="py-2.5 px-3">サービス</th>
                <th class="py-2.5 px-3">スコープ</th>
                <th class="py-2.5 px-3">メトリック名</th>
                <th class="py-2.5 px-3 text-center">種別</th>
                <th class="py-2.5 px-3 text-right">回数</th>
                <th class="py-2.5 px-3">初回日時</th>
                <th class="py-2.5 px-3">最終受信</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60">
              {#if paginatedMetrics.length === 0}
                <tr>
                  <td colspan="8" class="py-8 text-center text-slate-500">
                    メトリックデータがありません
                  </td>
                </tr>
              {:else}
                {#each paginatedMetrics as m}
                  <tr
                    onclick={() => (selectedMetric = m)}
                    class="cursor-pointer transition-colors {selectedMetric === m ? 'bg-cyan-950/60 text-cyan-200 font-medium' : 'hover:bg-slate-800/40'}"
                  >
                    <td class="py-2 px-3 font-mono">{m.Host}</td>
                    <td class="py-2 px-3">{m.Service}</td>
                    <td class="py-2 px-3 text-slate-400 max-w-[200px] truncate" title={m.Scope}>{m.Scope}</td>
                    <td class="py-2 px-3 font-medium text-slate-100">{m.Name}</td>
                    <td class="py-2 px-3 text-center">
                      <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-800 text-slate-300 border border-slate-700">
                        {m.Type}
                      </span>
                    </td>
                    <td class="py-2 px-3 text-right font-mono text-cyan-400">{m.Count}</td>
                    <td class="py-2 px-3 font-mono text-slate-400">{formatTimeStr(m.First)}</td>
                    <td class="py-2 px-3 font-mono text-slate-400">{formatTimeStr(m.Last)}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Pagination -->
        <div class="flex items-center justify-between p-2.5 border-t border-slate-800 bg-slate-950/40 text-xs text-slate-400">
          <div>
            ページ {metricPage} / {Math.max(1, Math.ceil(filteredMetrics.length / metricPageSize))}
          </div>
          <div class="flex items-center gap-1">
            <button
              disabled={metricPage <= 1}
              onclick={() => (metricPage = 1)}
              class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
            >
              <ChevronsLeft class="h-4 w-4" />
            </button>
            <button
              disabled={metricPage <= 1}
              onclick={() => metricPage--}
              class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
            >
              <ChevronLeft class="h-4 w-4" />
            </button>
            <button
              disabled={metricPage * metricPageSize >= filteredMetrics.length}
              onclick={() => metricPage++}
              class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
            >
              <ChevronRight class="h-4 w-4" />
            </button>
            <button
              disabled={metricPage * metricPageSize >= filteredMetrics.length}
              onclick={() => (metricPage = Math.ceil(filteredMetrics.length / metricPageSize))}
              class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
            >
              <ChevronsRight class="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    {/if}

    <!-- ================= TRACE TAB ================= -->
    {#if activeTab === "trace"}
      <div class="flex flex-col h-full gap-3 overflow-hidden">
        <!-- Top Scatter Chart -->
        <div class="h-64 bg-slate-900 border border-slate-800 rounded-xl p-2 relative shadow-lg shrink-0">
          <div class="text-[11px] font-semibold text-slate-400 absolute top-2 left-4 z-10 flex items-center gap-2">
            <span>トレース応答時間 (秒) 散布図</span>
            <span class="text-[10px] text-slate-500 font-normal">色: 所要時間 / ドットサイズ: スパン数</span>
          </div>
          <div id="traceScatterChart" class="h-full w-full"></div>
        </div>

        <!-- Trace Table & Bucket Selector -->
        <div class="flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg min-h-0">
          <div class="flex items-center justify-between p-3 border-b border-slate-800 bg-slate-900/60">
            <div class="flex items-center gap-3">
              <div class="relative w-72">
                <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  type="text"
                  bind:value={traceSearch}
                  placeholder="TraceID、サービス名で検索..."
                  class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-950 border border-slate-800 rounded-lg text-slate-200 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                />
              </div>

              <!-- Bucket Multi-Select Dropdown -->
              <div class="flex items-center gap-1.5">
                <span class="text-xs text-slate-400">時間バケット:</span>
                <select
                  multiple
                  bind:value={selectedBuckets}
                  onchange={refresh}
                  class="h-8 max-h-8 text-xs bg-slate-950 border border-slate-800 rounded-lg text-slate-200 px-2 focus:outline-none focus:border-cyan-500"
                >
                  {#each traceBuckets as b}
                    <option value={b}>{b}</option>
                  {/each}
                </select>
              </div>
            </div>

            <div class="text-xs text-slate-400">
              全 <span class="font-bold text-slate-200">{filteredTraces.length}</span> 件
            </div>
          </div>

          <!-- Trace Table -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-950 text-slate-400 font-semibold border-b border-slate-800 z-10">
                <tr>
                  <th class="py-2.5 px-3">開始日時</th>
                  <th class="py-2.5 px-3">終了日時</th>
                  <th class="py-2.5 px-3 text-right">所要時間 (ms)</th>
                  <th class="py-2.5 px-3">TraceID</th>
                  <th class="py-2.5 px-3">送信元ホスト</th>
                  <th class="py-2.5 px-3">サービス</th>
                  <th class="py-2.5 px-3 text-center">Span</th>
                  <th class="py-2.5 px-3">スコープ</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60">
                {#if paginatedTraces.length === 0}
                  <tr>
                    <td colspan="8" class="py-8 text-center text-slate-500">
                      トレースデータがありません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedTraces as t}
                    <tr
                      onclick={() => (selectedTrace = t)}
                      class="cursor-pointer transition-colors {selectedTrace === t ? 'bg-cyan-950/60 text-cyan-200 font-medium' : 'hover:bg-slate-800/40'}"
                    >
                      <td class="py-2 px-3 font-mono text-slate-300">{renderTimeMili(t.Start)}</td>
                      <td class="py-2 px-3 font-mono text-slate-300">{renderTimeMili(t.End)}</td>
                      <td class="py-2 px-3 text-right font-mono text-cyan-400 font-semibold">
                        {(t.Dur * 1000).toFixed(3)}
                      </td>
                      <td class="py-2 px-3 font-mono text-slate-400 truncate max-w-[140px]" title={t.TraceID}>{t.TraceID}</td>
                      <td class="py-2 px-3 font-mono text-slate-300">{t.Hosts}</td>
                      <td class="py-2 px-3 font-semibold text-slate-100">{t.Services}</td>
                      <td class="py-2 px-3 text-center">
                        <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">
                          {t.NumSpan}
                        </span>
                      </td>
                      <td class="py-2 px-3 text-slate-400 max-w-[180px] truncate" title={t.Scopes}>{t.Scopes}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between p-2.5 border-t border-slate-800 bg-slate-950/40 text-xs text-slate-400">
            <div>
              ページ {tracePage} / {Math.max(1, Math.ceil(filteredTraces.length / tracePageSize))}
            </div>
            <div class="flex items-center gap-1">
              <button
                disabled={tracePage <= 1}
                onclick={() => (tracePage = 1)}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage <= 1}
                onclick={() => tracePage--}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage * tracePageSize >= filteredTraces.length}
                onclick={() => tracePage++}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage * tracePageSize >= filteredTraces.length}
                onclick={() => (tracePage = Math.ceil(filteredTraces.length / tracePageSize))}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsRight class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}

    <!-- ================= LOG TAB ================= -->
    {#if activeTab === "log"}
      <div class="flex flex-col h-full gap-3 overflow-hidden">
        <!-- Top Log Histogram -->
        <div class="h-56 bg-slate-900 border border-slate-800 rounded-xl p-2 relative shadow-lg shrink-0">
          <div class="text-[11px] font-semibold text-slate-400 absolute top-2 left-4 z-10">
            OpenTelemetry ログ受信件数推移
          </div>
          <div id="otelLogChart" class="h-full w-full"></div>
        </div>

        <!-- Log Table -->
        <div class="flex-1 flex flex-col bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-lg min-h-0">
          <div class="flex items-center justify-between p-3 border-b border-slate-800 bg-slate-900/60">
            <div class="flex items-center gap-3">
              <div class="relative w-80">
                <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  type="text"
                  bind:value={logSearch}
                  placeholder="メッセージ、ホスト、TraceIDで検索..."
                  class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-950 border border-slate-800 rounded-lg text-slate-200 placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                />
              </div>

              <!-- Level Filter Badges -->
              <div class="flex items-center gap-1">
                {#each [{ id: "all", label: "すべて" }, { id: "high", label: "重大" }, { id: "warn", label: "警告" }, { id: "low", label: "情報" }] as lvl}
                  <button
                    onclick={() => (logLevelFilter = lvl.id)}
                    class="px-2.5 py-1 rounded text-xs font-medium transition-all {logLevelFilter === lvl.id ? 'bg-cyan-600 text-white font-semibold' : 'bg-slate-800 text-slate-400 hover:text-slate-200'}"
                  >
                    {lvl.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="text-xs text-slate-400">
              全 <span class="font-bold text-slate-200">{filteredLogs.length}</span> 件
            </div>
          </div>

          <!-- Log Table -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-950 text-slate-400 font-semibold border-b border-slate-800 z-10">
                <tr>
                  <th class="py-2.5 px-3 text-center">レベル</th>
                  <th class="py-2.5 px-3">日時</th>
                  <th class="py-2.5 px-3">ホスト</th>
                  <th class="py-2.5 px-3">サービス / スコープ</th>
                  <th class="py-2.5 px-3">メッセージ</th>
                  <th class="py-2.5 px-3">TraceID</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60 font-mono">
                {#if paginatedLogs.length === 0}
                  <tr>
                    <td colspan="6" class="py-8 text-center text-slate-500 font-sans">
                      ログレコードがありません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedLogs as l}
                    <tr
                      onclick={() => { selectedLog = l; showLogModal = true; }}
                      class="hover:bg-slate-800/40 transition-colors cursor-pointer"
                    >
                      <td class="py-2 px-3 text-center">
                        <span class="px-2 py-0.5 rounded text-[10px] font-bold font-sans {l.level === 'high' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40' : l.level === 'warn' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40' : 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40'}">
                          {l.severityText}
                        </span>
                      </td>
                      <td class="py-2 px-3 text-slate-300 whitespace-nowrap">{formatTimeStr(l.time)}</td>
                      <td class="py-2 px-3 text-slate-300 whitespace-nowrap">{l.host}</td>
                      <td class="py-2 px-3 text-slate-400 font-sans whitespace-nowrap">
                        {l.service} <span class="text-slate-600">/</span> {l.scope}
                      </td>
                      <td class="py-2 px-3 text-slate-200 font-sans break-all max-w-[400px]">{l.message}</td>
                      <td class="py-2 px-3 text-slate-400 text-[10px] truncate max-w-[120px]" title={l.traceId}>{l.traceId}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between p-2.5 border-t border-slate-800 bg-slate-950/40 text-xs text-slate-400">
            <div>
              ページ {logPage} / {Math.max(1, Math.ceil(filteredLogs.length / logPageSize))}
            </div>
            <div class="flex items-center gap-1">
              <button
                disabled={logPage <= 1}
                onclick={() => (logPage = 1)}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                disabled={logPage <= 1}
                onclick={() => logPage--}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                disabled={logPage * logPageSize >= filteredLogs.length}
                onclick={() => logPage++}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                disabled={logPage * logPageSize >= filteredLogs.length}
                onclick={() => (logPage = Math.ceil(filteredLogs.length / logPageSize))}
                class="p-1 rounded hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsRight class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>

<!-- ================= MODALS ================= -->

<!-- 1. Metric Info Modal -->
{#if showMetricInfo && selectedMetric}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
          <Info class="h-4 w-4 text-cyan-400" />
          <span>メトリック情報</span>
        </h3>
        <button onclick={() => (showMetricInfo = false)} class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
          <X class="h-4 w-4" />
        </button>
      </div>
      <div class="p-4 space-y-2 text-xs">
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">送信元ホスト</span>
          <span class="col-span-2 font-mono text-slate-200">{selectedMetric.Host}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">サービス</span>
          <span class="col-span-2 text-slate-200 font-semibold">{selectedMetric.Service}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">スコープ</span>
          <span class="col-span-2 text-slate-300">{selectedMetric.Scope}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">メトリック名</span>
          <span class="col-span-2 font-bold text-cyan-300">{selectedMetric.Name}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">種別</span>
          <span class="col-span-2 font-mono text-slate-200">{selectedMetric.Type}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">単位</span>
          <span class="col-span-2 text-slate-200">{selectedMetric.Unit || "-"}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">説明</span>
          <span class="col-span-2 text-slate-300">{selectedMetric.Description || "-"}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">累計受信回数</span>
          <span class="col-span-2 font-mono text-cyan-400 font-bold">{selectedMetric.Count}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-800/60">
          <span class="text-slate-400 font-medium">初回日時</span>
          <span class="col-span-2 font-mono text-slate-300">{formatTimeStr(selectedMetric.First)}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5">
          <span class="text-slate-400 font-medium">最終日時</span>
          <span class="col-span-2 font-mono text-slate-300">{formatTimeStr(selectedMetric.Last)}</span>
        </div>
      </div>
      <div class="flex justify-end p-3 border-t border-slate-800 bg-slate-950/40">
        <button
          onclick={() => (showMetricInfo = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 2. Metric Report Modal (Time Chart / Histogram) -->
{#if showMetricReport && metricDetail}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
    <div class="w-full max-w-4xl max-h-[90vh] bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
        <div>
          <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
            <Activity class="h-4 w-4 text-emerald-400" />
            <span>メトリック レポート: {metricDetail.Name}</span>
          </h3>
          <p class="text-[11px] text-slate-400">
            {metricDetail.Host} / {metricDetail.Service} ({metricDetail.Type})
          </p>
        </div>
        <div class="flex items-center gap-2">
          {#if metricDetail.Type === "Histogram"}
            <div class="flex items-center bg-slate-950 border border-slate-800 rounded-lg p-0.5 text-xs">
              <button
                onclick={() => {
                  metricChartMode = "time";
                  tick().then(renderMetricChart);
                }}
                class="px-2.5 py-1 rounded {metricChartMode === 'time' ? 'bg-cyan-600 text-white font-medium' : 'text-slate-400'}"
              >
                時系列
              </button>
              <button
                onclick={() => {
                  metricChartMode = "histogram";
                  tick().then(renderMetricChart);
                }}
                class="px-2.5 py-1 rounded {metricChartMode === 'histogram' ? 'bg-cyan-600 text-white font-medium' : 'text-slate-400'}"
              >
                ヒストグラム
              </button>
            </div>
          {/if}
          <button onclick={() => (showMetricReport = false)} class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
            <X class="h-4 w-4" />
          </button>
        </div>
      </div>

      <!-- Chart View -->
      <div class="p-4 flex flex-col flex-1 min-h-0 overflow-hidden">
        <div id="metricReportChart" class="h-64 w-full bg-slate-950/40 rounded-xl border border-slate-800/80 mb-3"></div>

        <!-- DataPoints Table -->
        <div class="flex-1 overflow-auto border border-slate-800 rounded-xl bg-slate-950/60">
          <table class="w-full text-left text-xs text-slate-300">
            <thead class="sticky top-0 bg-slate-950 text-slate-400 border-b border-slate-800">
              <tr>
                <th class="py-2 px-3">日時</th>
                <th class="py-2 px-3">属性 (Attributes)</th>
                <th class="py-2 px-3 text-right">値 (Sum/Gauge)</th>
                {#if metricDetail.Type === "Histogram"}
                  <th class="py-2 px-3 text-right">回数 (Count)</th>
                {/if}
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono">
              {#each (metricDetail.DataPoints || []) as dp}
                <tr
                  onclick={() => {
                    selectedDataPoint = dp;
                    if (metricChartMode === "histogram") renderMetricChart();
                  }}
                  class="cursor-pointer hover:bg-slate-800/40 {selectedDataPoint === dp ? 'bg-cyan-950/40 text-cyan-300' : ''}"
                >
                  <td class="py-1.5 px-3 text-slate-300 whitespace-nowrap">{formatTimeStr(dp.Time)}</td>
                  <td class="py-1.5 px-3 text-slate-400 font-sans truncate max-w-[280px]" title={dp.Attributes?.join(" ")}>
                    {dp.Attributes?.join(" ") || "-"}
                  </td>
                  <td class="py-1.5 px-3 text-right text-cyan-400 font-semibold">
                    {dp.Sum ?? dp.Gauge ?? "-"}
                  </td>
                  {#if metricDetail.Type === "Histogram"}
                    <td class="py-1.5 px-3 text-right text-emerald-400">{dp.Count ?? "-"}</td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex justify-end p-3 border-t border-slate-800 bg-slate-950/40">
        <button
          onclick={() => (showMetricReport = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 3. Service DAG Modal -->
{#if showDAGModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-5xl h-[85vh] bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
          <GitBranch class="h-4 w-4 text-indigo-400" />
          <span>サービス間 呼び出し依存関係図 (DAG)</span>
        </h3>
        <button onclick={() => (showDAGModal = false)} class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="flex-1 p-2 relative bg-slate-950/40 min-h-0">
        <div id="dagChart" class="h-full w-full"></div>
      </div>

      <div class="flex justify-between items-center p-3 border-t border-slate-800 bg-slate-950/40 text-xs text-slate-400">
        <div>
          ノード: <span class="font-bold text-slate-200">{dagData?.Nodes?.length || 0}</span> /
          リンク: <span class="font-bold text-slate-200">{dagData?.Links?.length || 0}</span>
        </div>
        <button
          onclick={() => (showDAGModal = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 4. Trace Waterfall (Timeline) Report Modal -->
{#if showTraceReport && traceDetail}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-5xl h-[88vh] bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
        <div>
          <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
            <Activity class="h-4 w-4 text-emerald-400" />
            <span>トレース ウォーターフォール: {traceDetail.TraceID}</span>
          </h3>
          <p class="text-[11px] text-slate-400">
            所要時間: <span class="font-bold text-cyan-300">{(traceDetail.Dur * 1000).toFixed(3)} ms</span> /
            スパン数: <span class="font-bold text-slate-200">{traceDetail.Spans?.length || 0}</span>
          </p>
        </div>
        <button onclick={() => (showTraceReport = false)} class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="flex-1 flex flex-col p-4 gap-3 min-h-0 overflow-hidden">
        <!-- Waterfall Timeline Chart -->
        <div id="traceWaterfallChart" class="h-60 w-full bg-slate-950/40 rounded-xl border border-slate-800/80 shrink-0"></div>

        <!-- Spans Detail Table -->
        <div class="flex-1 overflow-auto border border-slate-800 rounded-xl bg-slate-950/60">
          <table class="w-full text-left text-xs text-slate-300">
            <thead class="sticky top-0 bg-slate-950 text-slate-400 border-b border-slate-800">
              <tr>
                <th class="py-2 px-3">スパン名</th>
                <th class="py-2 px-3">サービス</th>
                <th class="py-2 px-3">開始日時</th>
                <th class="py-2 px-3">終了日時</th>
                <th class="py-2 px-3 text-right">所要時間 (ms)</th>
                <th class="py-2 px-3">Span ID</th>
                <th class="py-2 px-3">親Span ID</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono">
              {#each (traceDetail.Spans || []) as sp}
                <tr class="hover:bg-slate-800/40">
                  <td class="py-1.5 px-3 font-sans font-medium text-slate-100">{sp.Name}</td>
                  <td class="py-1.5 px-3 font-sans text-cyan-300">{sp.Service}</td>
                  <td class="py-1.5 px-3 text-slate-400">{renderTimeMili(sp.Start)}</td>
                  <td class="py-1.5 px-3 text-slate-400">{renderTimeMili(sp.End)}</td>
                  <td class="py-1.5 px-3 text-right text-emerald-400 font-semibold">{(sp.Dur * 1000).toFixed(3)}</td>
                  <td class="py-1.5 px-3 text-slate-500 text-[10px]">{sp.SpanID}</td>
                  <td class="py-1.5 px-3 text-slate-500 text-[10px]">{sp.ParentSpanID || "-"}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex justify-end p-3 border-t border-slate-800 bg-slate-950/40">
        <button
          onclick={() => (showTraceReport = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 5. OTel Log Detail Modal -->
{#if showLogModal && selectedLog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
          <FileText class="h-4 w-4 text-cyan-400" />
          <span>OpenTelemetry ログ詳細</span>
        </h3>
        <button onclick={() => (showLogModal = false)} class="p-1 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="p-5 space-y-4 text-xs overflow-auto max-h-[75vh]">
        <div class="grid grid-cols-2 gap-3">
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">日時</span>
            <span class="text-slate-200 font-mono">{formatTimeStr(selectedLog.time)}</span>
          </div>
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">重要度</span>
            <span class="font-bold {selectedLog.level === 'high' ? 'text-rose-400' : selectedLog.level === 'warn' ? 'text-amber-400' : 'text-cyan-400'}">
              {selectedLog.severityText} (Level: {selectedLog.severity})
            </span>
          </div>
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">ホスト</span>
            <span class="text-slate-200 font-mono">{selectedLog.host}</span>
          </div>
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">サービス / スコープ</span>
            <span class="text-slate-200 font-semibold">{selectedLog.service} / {selectedLog.scope}</span>
          </div>
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">Trace ID</span>
            <span class="text-slate-300 font-mono text-[11px] break-all">{selectedLog.traceId}</span>
          </div>
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80">
            <span class="text-slate-500 block mb-1">Span ID</span>
            <span class="text-slate-300 font-mono text-[11px] break-all">{selectedLog.spanId}</span>
          </div>
        </div>

        <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80 space-y-1">
          <span class="text-slate-500 block font-semibold">メッセージ</span>
          <p class="text-slate-200 whitespace-pre-wrap font-sans leading-relaxed">{selectedLog.message}</p>
        </div>

        {#if selectedLog.attributes && Object.keys(selectedLog.attributes).length > 0}
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80 space-y-2">
            <span class="text-slate-500 block font-semibold">属性 (Attributes)</span>
            <div class="space-y-1">
              {#each Object.entries(selectedLog.attributes) as [k, v]}
                <div class="flex items-start gap-2 font-mono text-[11px]">
                  <span class="text-cyan-400 shrink-0">{k}:</span>
                  <span class="text-slate-300 break-all">{v}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if selectedLog.rawText && selectedLog.rawText !== selectedLog.message}
          <div class="p-3 bg-slate-950/50 rounded-xl border border-slate-800/80 space-y-1">
            <span class="text-slate-500 block font-semibold">Raw ログ</span>
            <pre class="text-slate-400 font-mono text-[10px] whitespace-pre-wrap break-all max-h-40 overflow-auto">{selectedLog.rawText}</pre>
          </div>
        {/if}
      </div>

      <div class="flex justify-end p-3 border-t border-slate-800 bg-slate-950/40">
        <button
          onclick={() => (showLogModal = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}
