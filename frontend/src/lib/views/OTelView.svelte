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
  import { showConfirm, showLoading, hideLoading } from "../stores/modalStore";
  import { formatTimeStr, renderTimeMili } from "../common";
  import {
    showOTelTrace,
    showOTelDAG,
    showOTelTimeline,
    showOTelTimeChart,
    showOTelHistogram,
    showOTelMetricTypePie,
    showOTelServiceMetricBar,
    showOTelLogChart,
  } from "../charts/otel";
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
    Layers,
    Server,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Clock,
    Zap,
    AlertCircle,
    AlertTriangle,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

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
  let selectedAttributeFilter = $state<string>("all");

  // Metric Sorting & Pagination
  let metricSortKey = $state("Last");
  let metricSortDir = $state<"asc" | "desc">("desc");
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
  let traceTimePreset = $state<"1h" | "6h" | "24h" | "all">("24h");
  let traceLimit = $state(5000);
  let traceLatencyFilter = $state<"all" | "slow100" | "slow500" | "slow1000">("all");
  let traceZoomRange = $state<{ st: number; et: number } | null>(null);
  let traceSortKey = $state("Start");
  let traceSortDir = $state<"asc" | "desc">("desc");

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

  // Log Sorting & Pagination
  let logSortKey = $state("time");
  let logSortDir = $state<"asc" | "desc">("desc");
  let logPage = $state(1);
  let logPageSize = $state(25);

  // Charts
  let metricTypeChartInstance: any = null;
  let metricServiceChartInstance: any = null;
  let traceScatterChart: any = null;
  let dagChartInstance: any = null;
  let timelineChartInstance: any = null;
  let metricTimeChartInstance: any = null;
  let logLevelChartInstance: any = null;

  onMount(() => {
    void refresh(true);
    const handleResize = () => {
      metricTypeChartInstance?.resize();
      metricServiceChartInstance?.resize();
      traceScatterChart?.resize();
      logLevelChartInstance?.resize();
      dagChartInstance?.resize();
      timelineChartInstance?.resize();
      metricTimeChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      metricTypeChartInstance?.dispose();
      metricServiceChartInstance?.dispose();
      traceScatterChart?.dispose();
      logLevelChartInstance?.dispose();
      dagChartInstance?.dispose();
      timelineChartInstance?.dispose();
      metricTimeChartInstance?.dispose();
    };
  });

  const loadMetrics = async (render = true) => {
    try {
      const m = await fetchOTelMetrics();
      metrics = Array.isArray(m) ? m : [];
      if (render && activeTab === "metric") {
        selectedMetric = null;
        await tick();
        renderMetricOverviewCharts();
      }
    } catch (e) {
      console.error("OTel loadMetrics error:", e);
    }
  };

  const loadTraces = async (render = true) => {
    try {
      const bks = await fetchOTelTraceBuckets();
      traceBuckets = Array.isArray(bks) ? bks : [];
      let queryBuckets: string[] | undefined = undefined;
      if (traceTimePreset !== "all" && traceBuckets.length > 0) {
        const hours = traceTimePreset === "1h" ? 1 : traceTimePreset === "6h" ? 6 : 24;
        const cutoff = new Date(Date.now() - hours * 3600 * 1000).toISOString().slice(0, 16);
        queryBuckets = traceBuckets.filter((b) => b >= cutoff);
        if (queryBuckets.length === 0 && traceBuckets.length > 0) {
          queryBuckets = traceBuckets.slice(-Math.min(traceBuckets.length, hours * 60));
        }
      }
      selectedBuckets = queryBuckets || [];
      const tr = await fetchOTelTraces(queryBuckets, traceLimit);
      traces = Array.isArray(tr) ? tr : [];
      if (render && activeTab === "trace") {
        selectedTrace = null;
        traceZoomRange = null;
        await tick();
        renderTraceScatter();
      }
    } catch (e) {
      console.error("OTel loadTraces error:", e);
    }
  };

  const loadLogs = async (render = true) => {
    try {
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
                parsed.message = `${prefix} ${$_('otel.msgMetricData')}`;
              } else if (inner.resource_spans || inner.resourceSpans) {
                parsed.message = `${prefix} ${$_('otel.msgTraceData')}`;
              } else if (inner.resource_logs || inner.resourceLogs) {
                parsed.message = `${prefix} ${$_('otel.msgLogData')}`;
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
        const rawSevText = (parsed.severityText || parsed.SeverityText || "").toUpperCase();

        let level = "INFO";
        if (rawSevText.includes("ERR") || rawSevText.includes("FATAL") || rawSevText.includes("CRIT") || sev <= 3) {
          level = "ERROR";
        } else if (rawSevText.includes("WARN") || sev === 4) {
          level = "WARN";
        } else if (rawSevText.includes("INFO") || sev === 5 || sev === 6) {
          level = "INFO";
        } else {
          level = "DEBUG";
        }

        const sevText = rawSevText || level;
        const message = parsed.message || parsed.Message || rawText;

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
      if (render && activeTab === "log") {
        await tick();
        renderLogLevelChart();
      }
    } catch (e) {
      console.error("OTel loadLogs error:", e);
    }
  };

  const refresh = async (all = false) => {
    loading = true;
    try {
      if (all) {
        await Promise.allSettled([
          loadMetrics(activeTab === "metric"),
          loadTraces(activeTab === "trace"),
          loadLogs(activeTab === "log"),
        ]);
      } else {
        if (activeTab === "metric") {
          await loadMetrics(true);
        } else if (activeTab === "trace") {
          await loadTraces(true);
        } else if (activeTab === "log") {
          await loadLogs(true);
        }
      }
    } finally {
      loading = false;
    }
  };

  const handleTabChange = async (tab: TabType) => {
    activeTab = tab;
    await refresh(false);
  };

  const renderMetricOverviewCharts = () => {
    const pieEl = document.getElementById("metricTypeChart");
    if (pieEl) {
      if (metricTypeChartInstance) metricTypeChartInstance.dispose();
      metricTypeChartInstance = showOTelMetricTypePie(pieEl, metrics || []);
    }
    const barEl = document.getElementById("metricServiceChart");
    if (barEl) {
      if (metricServiceChartInstance) metricServiceChartInstance.dispose();
      metricServiceChartInstance = showOTelServiceMetricBar(barEl, metrics || []);
    }
  };

  // --- Metric Handlers & Derived ---
  const metricKPIs = $derived.by(() => {
    const list = metrics || [];
    const totalMetrics = list.length;
    const services = new Set(list.map((m) => m.Service).filter(Boolean));
    const hosts = new Set(list.map((m) => m.Host).filter(Boolean));
    const totalCount = list.reduce((acc, m) => acc + (m.Count || 0), 0);
    return {
      totalMetrics,
      serviceCount: services.size,
      hostCount: hosts.size,
      totalCount,
    };
  });

  const metricAttributesList = $derived.by(() => {
    if (!metricDetail?.DataPoints) return [];
    const set = new Set<string>();
    for (const dp of metricDetail.DataPoints) {
      if (dp.Attributes && dp.Attributes.length > 0) {
        set.add(dp.Attributes.join(" "));
      }
    }
    return Array.from(set);
  });

  const filteredModalDataPoints = $derived.by(() => {
    if (!metricDetail?.DataPoints) return [];
    if (selectedAttributeFilter === "all") return metricDetail.DataPoints;
    return metricDetail.DataPoints.filter(
      (dp) => (dp.Attributes?.join(" ") || "") === selectedAttributeFilter
    );
  });

  const handleMetricSort = (key: string) => {
    if (metricSortKey === key) {
      metricSortDir = metricSortDir === "asc" ? "desc" : "asc";
    } else {
      metricSortKey = key;
      metricSortDir = key === "Count" || key === "First" || key === "Last" ? "desc" : "asc";
    }
  };

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

  const sortedMetrics = $derived.by(() => {
    const list = [...filteredMetrics];
    const key = metricSortKey;
    const dir = metricSortDir === "asc" ? 1 : -1;
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

  const paginatedMetrics = $derived(
    sortedMetrics.slice((metricPage - 1) * metricPageSize, metricPage * metricPageSize)
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
      selectedAttributeFilter = "all";
      selectedDataPoint = d?.DataPoints && d.DataPoints.length > 0 ? d.DataPoints[0] : null;
      metricChartMode = d?.Type === "Histogram" || d?.Type === "ExponentialHistogram" ? "histogram" : "time";
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
    const el = document.getElementById("metricReportChart");
    if (!el || !metricDetail?.DataPoints || metricDetail.DataPoints.length === 0) return;
    if (metricTimeChartInstance) {
      metricTimeChartInstance.dispose();
      metricTimeChartInstance = null;
    }
    if (metricChartMode === "histogram") {
      const dp = selectedDataPoint || metricDetail.DataPoints[0];
      metricTimeChartInstance = showOTelHistogram(el, dp);
    } else {
      metricTimeChartInstance = showOTelTimeChart(el, metricDetail.DataPoints, selectedAttributeFilter);
    }
    metricTimeChartInstance?.resize();
  };

  // --- Trace Handlers & Derived ---
  const traceKPIs = $derived.by(() => {
    const list = traces || [];
    const total = list.length;
    if (total === 0) {
      return { total: 0, avgMs: 0, maxMs: 0, slowCount: 0 };
    }
    let sumMs = 0;
    let maxMs = 0;
    let slowCount = 0;
    for (const t of list) {
      const ms = (t.Dur || 0) * 1000;
      sumMs += ms;
      if (ms > maxMs) maxMs = ms;
      if (ms >= 500) slowCount++;
    }
    return {
      total,
      avgMs: sumMs / total,
      maxMs,
      slowCount,
    };
  });

  const handleTraceSort = (key: string) => {
    if (traceSortKey === key) {
      traceSortDir = traceSortDir === "asc" ? "desc" : "asc";
    } else {
      traceSortKey = key;
      traceSortDir = key === "Start" || key === "End" || key === "Dur" || key === "NumSpan" ? "desc" : "asc";
    }
  };

  const filteredTraces = $derived(
    (traces || []).filter((t) => {
      if (traceZoomRange) {
        if (t.Start > traceZoomRange.et || t.End < traceZoomRange.st) return false;
      }
      const durMs = (t.Dur || 0) * 1000;
      if (traceLatencyFilter === "slow100" && durMs < 100) return false;
      if (traceLatencyFilter === "slow500" && durMs < 500) return false;
      if (traceLatencyFilter === "slow1000" && durMs < 1000) return false;

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

  const sortedTraces = $derived.by(() => {
    const list = [...filteredTraces];
    const key = traceSortKey;
    const dir = traceSortDir === "asc" ? 1 : -1;
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

  const paginatedTraces = $derived(
    sortedTraces.slice((tracePage - 1) * tracePageSize, tracePage * tracePageSize)
  );

  const renderTraceScatter = () => {
    const el = document.getElementById("traceScatterChart");
    if (!el) return;
    if (traceScatterChart) {
      traceScatterChart.dispose();
    }
    traceScatterChart = showOTelTrace(el, traces || [], (st, et) => {
      if (st && et && st < et) {
        traceZoomRange = { st, et };
      } else {
        traceZoomRange = null;
      }
      tracePage = 1;
    });
  };

  const openDAGModal = async () => {
    loading = true;
    try {
      dagData = await fetchOTelDAG(selectedBuckets.length > 0 ? selectedBuckets : undefined);
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
  const logKPIs = $derived.by(() => {
    const list = parsedLogs || [];
    const total = list.length;
    let errorCount = 0;
    let warnCount = 0;
    let infoCount = 0;
    let debugCount = 0;
    const services = new Set<string>();
    const hosts = new Set<string>();

    for (const l of list) {
      if (l.level === "ERROR") {
        errorCount++;
      } else if (l.level === "WARN") {
        warnCount++;
      } else if (l.level === "INFO") {
        infoCount++;
      } else {
        debugCount++;
      }
      if (l.service && l.service !== "-") services.add(l.service);
      if (l.host && l.host !== "-") hosts.add(l.host);
    }

    return {
      total,
      errorCount,
      warnCount,
      infoCount,
      debugCount,
      serviceCount: services.size,
      hostCount: hosts.size,
    };
  });

  const handleLogSort = (key: string) => {
    if (logSortKey === key) {
      logSortDir = logSortDir === "asc" ? "desc" : "asc";
    } else {
      logSortKey = key;
      logSortDir = key === "time" ? "desc" : "asc";
    }
  };

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

  const sortedLogs = $derived.by(() => {
    const list = [...filteredLogs];
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

  const paginatedLogs = $derived(
    sortedLogs.slice((logPage - 1) * logPageSize, logPage * logPageSize)
  );

  const renderLogLevelChart = () => {
    const el = document.getElementById("otelLogChart");
    if (!el) return;
    if (logLevelChartInstance) {
      logLevelChartInstance.dispose();
      logLevelChartInstance = null;
    }
    logLevelChartInstance = showOTelLogChart(el, parsedLogs || []);
  };

  // Global Delete
  const handleDeleteAll = async () => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'OTelデータ全削除の確認',
      message: $_('otel.confirmDeleteAll'),
      type: 'danger',
      confirmText: $_('common.delete') || '全削除',
    });
    if (!ok) {
      return;
    }
    showLoading({
      title: $_('common.processing') || '処理中...',
      message: $_('otel.deletingAll') || 'OTelデータをすべて削除しています...',
    });
    loading = true;
    try {
      await deleteAllOTelData();
      await refresh();
    } finally {
      loading = false;
      hideLoading();
    }
  };

  // Duration color coding for traces
  const getTraceDurationClass = (durSec: number) => {
    const ms = (durSec || 0) * 1000;
    if (ms >= 1000) return "text-rose-600 dark:text-rose-400 font-bold";
    if (ms >= 500) return "text-amber-600 dark:text-amber-400 font-semibold";
    if (ms >= 100) return "text-amber-500 dark:text-amber-300 font-medium";
    return "text-emerald-600 dark:text-emerald-400";
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-50 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans transition-colors">
  <!-- Left Sidebar (Matching LogView / ListView / ReportView) -->
  <div class="w-60 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/80 p-3 space-y-1.5 shrink-0 flex flex-col justify-between transition-colors">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
        {$_('otel.telemetryType')}
      </div>

      {#each [
        { id: "metric", name: $_('otel.tabMetric'), icon: BarChart3, count: metrics.length },
        { id: "trace", name: $_('otel.tabTrace'), icon: Eye, count: traces.length },
        { id: "log", name: $_('otel.tabLog'), icon: FileText, count: parsedLogs.length }
      ] as item}
        <button
          type="button"
          onclick={() => handleTabChange(item.id as any)}
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

    <!-- Receiver Endpoint Info -->
    <div class="p-3 rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50/60 dark:bg-slate-900/60 text-[10px] text-slate-500 dark:text-slate-400 space-y-1">
      <div class="flex items-center justify-between font-semibold text-slate-700 dark:text-slate-300">
        <span>{$_('otel.receiver')}</span>
        <span class="inline-flex items-center gap-1 text-emerald-600 dark:text-emerald-400">
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
          {$_('otel.running')}
        </span>
      </div>
      <div class="font-mono text-cyan-600 dark:text-cyan-400 font-semibold">HTTP :4318 (OTLP)</div>
      <div>Traces / Metrics / Logs</div>
    </div>
  </div>

  <!-- Main Content Area -->
  <div class="flex-1 flex flex-col min-w-0 overflow-hidden">
    <!-- Top Action Header -->
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 px-6 py-2.5 shrink-0 shadow-xs">
      <div>
        <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          {#if activeTab === "metric"}
            <BarChart3 class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('otel.titleMetric')}</span>
          {:else if activeTab === "trace"}
            <Eye class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('otel.titleTrace')}</span>
          {:else}
            <FileText class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('otel.titleLog')}</span>
          {/if}
        </h2>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2">
        {#if activeTab === "metric" && selectedMetric}
          <button
            type="button"
            onclick={openMetricInfo}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <Info class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <span>{$_('otel.btnMetricInfo')}</span>
          </button>

          <button
            type="button"
            onclick={openMetricReport}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-emerald-300 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/60 text-emerald-800 dark:text-emerald-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <Activity class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
            <span>{$_('otel.btnReport')}</span>
          </button>
        {/if}

        {#if activeTab === "trace"}
          <button
            type="button"
            onclick={openDAGModal}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-indigo-300 dark:border-indigo-800/60 bg-indigo-50 dark:bg-indigo-950/40 hover:bg-indigo-100 dark:hover:bg-indigo-900/60 text-indigo-800 dark:text-indigo-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
          >
            <GitBranch class="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
            <span>{$_('otel.btnServiceDag')}</span>
          </button>

          {#if selectedTrace}
            <button
              type="button"
              onclick={openTraceReport}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-emerald-300 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/60 text-emerald-800 dark:text-emerald-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
            >
              <Activity class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              <span>{$_('otel.btnReport')}</span>
            </button>
          {/if}
        {/if}

        <button
          type="button"
          onclick={handleDeleteAll}
          title={$_('otel.btnDeleteAllTitle')}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 text-rose-800 dark:text-rose-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
        >
          <Trash2 class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
          <span>{$_('otel.btnDeleteAll')}</span>
        </button>

        <button
          type="button"
          onclick={() => refresh(true)}
          title={$_('otel.btnReloadTitle')}
          disabled={loading}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
        >
          <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin text-cyan-600 dark:text-cyan-400' : 'text-cyan-600 dark:text-cyan-400'}" />
          <span>{$_('otel.btnReload')}</span>
        </button>
      </div>
    </div>

    <!-- Content Body -->
    <div class="flex-1 flex flex-col min-h-0 overflow-hidden p-4">
      <!-- ================= METRIC TAB ================= -->
      {#if activeTab === "metric"}
        <div class="flex flex-col h-full gap-3 overflow-hidden">
          <!-- Top Overview: 4 Vertical KPIs + Charts -->
          <div class="grid grid-cols-1 lg:grid-cols-12 gap-3 shrink-0 h-64">
            <!-- 4 Vertical KPI Cards (col-span-3) -->
            <div class="lg:col-span-3 flex flex-col gap-2 h-full">
              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-sm dark:shadow-lg flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-cyan-50 dark:bg-cyan-950/60 border border-cyan-200 dark:border-cyan-800/80">
                    <BarChart3 class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('otel.metricSeries')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-cyan-600 dark:text-cyan-300">{metricKPIs.totalMetrics.toLocaleString()}</span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-sm dark:shadow-lg flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-indigo-50 dark:bg-indigo-950/60 border border-indigo-200 dark:border-indigo-800/80">
                    <Server class="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('otel.serviceCount')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-indigo-600 dark:text-indigo-300">{metricKPIs.serviceCount.toLocaleString()}</span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-sm dark:shadow-lg flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800/80">
                    <Layers class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('otel.hostCount')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-emerald-600 dark:text-emerald-300">{metricKPIs.hostCount.toLocaleString()}</span>
              </div>

              <div class="flex-1 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl px-3.5 py-2 shadow-sm dark:shadow-lg flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <div class="p-1.5 rounded-lg bg-amber-50 dark:bg-amber-950/60 border border-amber-200 dark:border-amber-800/80">
                    <Activity class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                  </div>
                  <span class="text-xs font-medium text-slate-600 dark:text-slate-400">{$_('otel.totalReceived')}</span>
                </div>
                <span class="text-lg font-bold font-mono text-amber-600 dark:text-amber-300">{metricKPIs.totalCount.toLocaleString()}</span>
              </div>
            </div>

            <!-- Metric Types Chart (col-span-3) -->
            <div class="lg:col-span-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2.5 relative shadow-sm dark:shadow-lg h-full flex flex-col">
              <div class="text-[11px] font-semibold text-slate-500 dark:text-slate-400 px-2 pt-1 z-10 flex items-center gap-1.5">
                <span>{$_('otel.metricTypeBreakdown')}</span>
              </div>
              <div id="metricTypeChart" class="flex-1 w-full min-h-0"></div>
            </div>

            <!-- Top Services Chart (col-span-6) -->
            <div class="lg:col-span-6 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2.5 relative shadow-sm dark:shadow-lg h-full flex flex-col">
              <div class="text-[11px] font-semibold text-slate-500 dark:text-slate-400 px-2 pt-1 z-10 flex items-center gap-1.5">
                <span>{$_('otel.metricsByService')}</span>
              </div>
              <div id="metricServiceChart" class="flex-1 w-full min-h-0"></div>
            </div>
          </div>

        <!-- Metric Table -->
        <div class="flex-1 flex flex-col bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm dark:shadow-lg min-h-0">
          <!-- Search and Filter Toolbar -->
          <div class="flex items-center justify-between p-3 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60">
            <div class="relative w-80">
              <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
              <input
                type="text"
                bind:value={metricSearch}
                placeholder={$_("otel.searchMetricPlaceholder")}
                class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-slate-950 border border-slate-300 dark:border-slate-800 rounded-lg text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-cyan-500"
              />
            </div>
            <div class="text-xs text-slate-400">
              {$_("otel.totalRecords", { values: { count: filteredMetrics.length } })}
            </div>
          </div>

          <!-- Metric Table -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800 z-10">
                <tr>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Host")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colHost")}</span>
                      {#if metricSortKey === "Host"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Service")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colService")}</span>
                      {#if metricSortKey === "Service"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Scope")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colScope")}</span>
                      {#if metricSortKey === "Scope"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Name")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colMetricName")}</span>
                      {#if metricSortKey === "Name"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 text-center cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Type")}>
                    <div class="flex items-center justify-center gap-1">
                      <span>{$_("otel.colType")}</span>
                      {#if metricSortKey === "Type"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Count")}>
                    <div class="flex items-center justify-end gap-1">
                      <span>{$_("otel.colCount")}</span>
                      {#if metricSortKey === "Count"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("First")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colFirstTime")}</span>
                      {#if metricSortKey === "First"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleMetricSort("Last")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colLastTime")}</span>
                      {#if metricSortKey === "Last"}
                        {#if metricSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60">
                {#if paginatedMetrics.length === 0}
                  <tr>
                    <td colspan="8" class="py-8 text-center text-slate-500">
                      {$_("otel.noMetricData")}
                    </td>
                  </tr>
                {:else}
                  {#each paginatedMetrics as m}
                    <tr
                      onclick={() => (selectedMetric = m)}
                      class="cursor-pointer transition-colors {selectedMetric === m ? 'bg-cyan-100 dark:bg-cyan-950/60 text-cyan-900 dark:text-cyan-200 font-medium' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40 text-slate-700 dark:text-slate-300'}"
                    >
                      <td class="py-1 px-2 font-mono text-slate-700 dark:text-slate-300">{m.Host}</td>
                      <td class="py-1 px-2 text-slate-800 dark:text-slate-200">{m.Service}</td>
                      <td class="py-1 px-2 text-slate-600 dark:text-slate-400 max-w-[200px] truncate" title={m.Scope}>{m.Scope}</td>
                      <td class="py-1 px-2 font-medium text-slate-900 dark:text-slate-100">{m.Name}</td>
                      <td class="py-1 px-2 text-center">
                        <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700">
                          {m.Type}
                        </span>
                      </td>
                      <td class="py-1 px-2 text-right font-mono text-slate-700 dark:text-slate-300">{m.Count}</td>
                      <td class="py-1 px-2 font-mono text-slate-600 dark:text-slate-400">{formatTimeStr(m.First)}</td>
                      <td class="py-1 px-2 font-mono text-slate-600 dark:text-slate-400">{formatTimeStr(m.Last)}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between p-2.5 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 text-xs text-slate-400">
            <div>
              {$_("otel.pageInfo", { values: { page: metricPage, total: Math.max(1, Math.ceil(filteredMetrics.length / metricPageSize)) } })}
            </div>
            <div class="flex items-center gap-1">
              <button
                disabled={metricPage <= 1}
                onclick={() => (metricPage = 1)}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                disabled={metricPage <= 1}
                onclick={() => metricPage--}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                disabled={metricPage * metricPageSize >= filteredMetrics.length}
                onclick={() => metricPage++}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                disabled={metricPage * metricPageSize >= filteredMetrics.length}
                onclick={() => (metricPage = Math.ceil(filteredMetrics.length / metricPageSize))}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsRight class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}

    <!-- ================= TRACE TAB ================= -->
    {#if activeTab === "trace"}
      <div class="flex flex-col h-full gap-3 overflow-hidden">
        <!-- Top KPIs -->
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 shrink-0">
          <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Activity class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
              {$_("otel.totalTraces")}
            </span>
            <span class="text-xl font-bold font-mono text-cyan-600 dark:text-cyan-300 mt-1">{traceKPIs.total.toLocaleString()}</span>
          </div>

          <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Clock class="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              {$_("otel.avgDuration")}
            </span>
            <span class="text-xl font-bold font-mono text-indigo-600 dark:text-indigo-300 mt-1">
              {traceKPIs.avgMs.toFixed(2)} <span class="text-xs font-normal text-slate-400">ms</span>
            </span>
          </div>

          <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Zap class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
              {$_("otel.maxDuration")}
            </span>
            <span class="text-xl font-bold font-mono text-amber-600 dark:text-amber-300 mt-1">
              {#if traceKPIs.maxMs >= 1000}
                {(traceKPIs.maxMs / 1000).toFixed(3)} <span class="text-xs font-normal text-slate-400">s</span>
              {:else}
                {traceKPIs.maxMs.toFixed(2)} <span class="text-xs font-normal text-slate-400">ms</span>
              {/if}
            </span>
          </div>

          <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Activity class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
              {$_("otel.slowTraces")}
            </span>
            <span class="text-xl font-bold font-mono {traceKPIs.slowCount > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-slate-700 dark:text-slate-300'} mt-1">
              {traceKPIs.slowCount.toLocaleString()}
            </span>
          </div>
        </div>

        <!-- Scatter Chart -->
        <div class="h-80 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2 relative shadow-sm dark:shadow-lg shrink-0">
          <div class="text-[11px] font-semibold text-slate-400 absolute top-2 left-4 z-10 flex items-center gap-2">
            <span>{$_("otel.scatterTitle")}</span>
            <span class="text-[10px] text-slate-500 font-normal">{$_("otel.scatterSubtitle")}</span>
            {#if traceZoomRange}
              <span class="px-2 py-0.5 rounded text-[10px] bg-amber-500/20 text-amber-300 border border-amber-500/40">
                {$_("otel.zoomedRange", { values: { st: renderTimeMili(traceZoomRange.st), et: renderTimeMili(traceZoomRange.et) } })}
              </span>
              <button
                onclick={() => {
                  traceZoomRange = null;
                  renderTraceScatter();
                }}
                class="px-2 py-0.5 rounded text-[10px] bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-colors"
              >
                {$_("otel.resetZoom")}
              </button>
            {/if}
          </div>
          <div id="traceScatterChart" class="h-full w-full"></div>
        </div>

        <!-- Trace Table & Filters -->
        <div class="flex-1 flex flex-col bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm dark:shadow-lg min-h-0">
          <div class="flex flex-wrap items-center justify-between p-3 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60 gap-3">
            <div class="flex flex-wrap items-center gap-3">
              <!-- Search -->
              <div class="relative w-64">
                <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  type="text"
                  bind:value={traceSearch}
                  placeholder={$_("otel.searchTracePlaceholder")}
                  class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-slate-950 border border-slate-300 dark:border-slate-800 rounded-lg text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                />
              </div>

              <!-- Time Range Presets -->
              <div class="flex items-center gap-1 bg-slate-100 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800/80 p-0.5 rounded-lg">
                {#each [
                  { id: "1h", label: $_("otel.period1h") },
                  { id: "6h", label: $_("otel.period6h") },
                  { id: "24h", label: $_("otel.period24h") },
                  { id: "all", label: $_("otel.periodAll") }
                ] as opt}
                  <button
                    onclick={async () => {
                      traceTimePreset = opt.id as any;
                      await refresh();
                    }}
                    class="px-2.5 py-1 rounded text-xs font-medium transition-all {traceTimePreset === opt.id ? 'bg-cyan-600 text-white font-semibold shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/60 dark:hover:bg-slate-800/60'}"
                  >
                    {opt.label}
                  </button>
                {/each}
              </div>

              <!-- Limit Dropdown -->
              <div class="flex items-center gap-1.5">
                <span class="text-xs text-slate-500 dark:text-slate-400">{$_("otel.limitLabel")}</span>
                <select
                  bind:value={traceLimit}
                  onchange={() => refresh()}
                  class="h-7 text-xs bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-800 rounded-lg text-slate-800 dark:text-slate-200 px-2 focus:outline-none focus:border-cyan-500"
                >
                  <option value={1000}>{$_("otel.limitCount", { values: { count: "1,000" } })}</option>
                  <option value={5000}>{$_("otel.limitCount", { values: { count: "5,000" } })}</option>
                  <option value={10000}>{$_("otel.limitCount", { values: { count: "10,000" } })}</option>
                </select>
              </div>

              <!-- Latency Filter Buttons -->
              <div class="flex items-center gap-1 bg-slate-100 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800/80 p-0.5 rounded-lg">
                {#each [
                  { id: "all", label: $_("otel.filterAll") },
                  { id: "slow100", label: "> 100ms" },
                  { id: "slow500", label: "> 500ms" },
                  { id: "slow1000", label: $_("otel.filterSlow1s") }
                ] as lat}
                  <button
                    onclick={() => (traceLatencyFilter = lat.id as any)}
                    class="px-2.5 py-1 rounded text-xs font-medium transition-all {traceLatencyFilter === lat.id ? 'bg-amber-600 dark:bg-amber-500/30 text-white dark:text-amber-300 border border-amber-600 dark:border-amber-500/50 font-semibold shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/60 dark:hover:bg-slate-800/60'}"
                  >
                    {lat.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="text-xs text-slate-500 dark:text-slate-400 flex items-center gap-2">
              <span>{$_("otel.displayCount", { values: { filtered: sortedTraces.length, total: traces.length } })}</span>
            </div>
          </div>

          <!-- Trace Table -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800 z-10 select-none">
                <tr>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("Start")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colStartTime")}</span>
                      {#if traceSortKey === "Start"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("End")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colEndTime")}</span>
                      {#if traceSortKey === "End"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 text-right cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("Dur")}>
                    <div class="flex items-center justify-end gap-1">
                      <span>{$_("otel.colDuration")}</span>
                      {#if traceSortKey === "Dur"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("TraceID")}>
                    <div class="flex items-center gap-1">
                      <span>TraceID</span>
                      {#if traceSortKey === "TraceID"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("Hosts")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colHost")}</span>
                      {#if traceSortKey === "Hosts"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("Services")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colService")}</span>
                      {#if traceSortKey === "Services"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 text-center cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("NumSpan")}>
                    <div class="flex items-center justify-center gap-1">
                      <span>Span</span>
                      {#if traceSortKey === "NumSpan"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-200 transition-colors" onclick={() => handleTraceSort("Scopes")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colScope")}</span>
                      {#if traceSortKey === "Scopes"}
                        {#if traceSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60">
                {#if paginatedTraces.length === 0}
                  <tr>
                    <td colspan="8" class="py-8 text-center text-slate-500">
                      {$_("otel.noTraceData")}
                    </td>
                  </tr>
                {:else}
                  {#each paginatedTraces as t}
                    <tr
                      onclick={() => (selectedTrace = t)}
                      class="cursor-pointer transition-colors {selectedTrace === t ? 'bg-cyan-100 dark:bg-cyan-950/60 text-cyan-900 dark:text-cyan-200 font-medium' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40 text-slate-700 dark:text-slate-300'}"
                    >
                      <td class="py-1 px-2 font-mono text-slate-700 dark:text-slate-300">{renderTimeMili(t.Start)}</td>
                      <td class="py-1 px-2 font-mono text-slate-700 dark:text-slate-300">{renderTimeMili(t.End)}</td>
                      <td class="py-1 px-2 text-right font-mono {getTraceDurationClass(t.Dur)}">
                        {(t.Dur * 1000).toFixed(3)}
                      </td>
                      <td class="py-1 px-2 font-mono text-slate-600 dark:text-slate-400 truncate max-w-[140px]" title={t.TraceID}>{t.TraceID}</td>
                      <td class="py-1 px-2 font-mono text-slate-700 dark:text-slate-300">{t.Hosts}</td>
                      <td class="py-1 px-2 font-semibold text-slate-900 dark:text-slate-100">{t.Services}</td>
                      <td class="py-1 px-2 text-center">
                        <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-cyan-100 dark:bg-cyan-500/20 text-cyan-800 dark:text-cyan-300 border border-cyan-300 dark:border-cyan-500/30">
                          {t.NumSpan}
                        </span>
                      </td>
                      <td class="py-1 px-2 text-slate-600 dark:text-slate-400 max-w-[180px] truncate" title={t.Scopes}>{t.Scopes}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between p-2.5 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 text-xs text-slate-400">
            <div>
              {$_("otel.pageInfo", { values: { page: tracePage, total: Math.max(1, Math.ceil(sortedTraces.length / tracePageSize)) } })}
            </div>
            <div class="flex items-center gap-1">
              <button
                disabled={tracePage <= 1}
                onclick={() => (tracePage = 1)}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage <= 1}
                onclick={() => tracePage--}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage * tracePageSize >= sortedTraces.length}
                onclick={() => tracePage++}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                disabled={tracePage * tracePageSize >= sortedTraces.length}
                onclick={() => (tracePage = Math.ceil(sortedTraces.length / tracePageSize))}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
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
        <!-- Top KPIs -->
        <div class="grid grid-cols-2 md:grid-cols-4 gap-3 shrink-0">
          <button
            type="button"
            onclick={() => (logLevelFilter = "all")}
            class="text-left bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between cursor-pointer hover:border-slate-700 transition-colors {logLevelFilter === 'all' ? 'ring-1 ring-cyan-500/50' : ''}"
          >
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <FileText class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
              {$_("otel.totalLogs")}
            </span>
            <span class="text-xl font-bold font-mono text-cyan-600 dark:text-cyan-300 mt-1">{logKPIs.total.toLocaleString()}</span>
          </button>

          <button
            type="button"
            onclick={() => (logLevelFilter = logLevelFilter === "ERROR" ? "all" : "ERROR")}
            class="text-left bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between cursor-pointer hover:border-slate-700 transition-colors {logLevelFilter === 'ERROR' ? 'ring-1 ring-rose-500/50' : ''}"
          >
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <AlertCircle class="h-3.5 w-3.5 text-rose-600 dark:text-rose-400" />
              {$_("otel.errorLogs")}
            </span>
            <div class="flex items-baseline justify-between mt-1">
              <span class="text-xl font-bold font-mono {logKPIs.errorCount > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-slate-700 dark:text-slate-300'}">
                {logKPIs.errorCount.toLocaleString()}
              </span>
              {#if logKPIs.total > 0 && logKPIs.errorCount > 0}
                <span class="text-[11px] font-mono text-rose-600 dark:text-rose-400/80">
                  {((logKPIs.errorCount / logKPIs.total) * 100).toFixed(1)}%
                </span>
              {/if}
            </div>
          </button>

          <button
            type="button"
            onclick={() => (logLevelFilter = logLevelFilter === "WARN" ? "all" : "WARN")}
            class="text-left bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between cursor-pointer hover:border-slate-700 transition-colors {logLevelFilter === 'WARN' ? 'ring-1 ring-amber-500/50' : ''}"
          >
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <AlertTriangle class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
              {$_("otel.warnLogs")}
            </span>
            <div class="flex items-baseline justify-between mt-1">
              <span class="text-xl font-bold font-mono {logKPIs.warnCount > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-slate-700 dark:text-slate-300'}">
                {logKPIs.warnCount.toLocaleString()}
              </span>
              {#if logKPIs.total > 0 && logKPIs.warnCount > 0}
                <span class="text-[11px] font-mono text-amber-600 dark:text-amber-400/80">
                  {((logKPIs.warnCount / logKPIs.total) * 100).toFixed(1)}%
                </span>
              {/if}
            </div>
          </button>

          <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <span class="text-[11px] text-slate-500 dark:text-slate-400 flex items-center gap-1.5 font-medium">
              <Server class="h-3.5 w-3.5 text-indigo-600 dark:text-indigo-400" />
              {$_("otel.servicesHosts")}
            </span>
            <div class="flex items-baseline gap-2 mt-1">
              <span class="text-xl font-bold font-mono text-indigo-600 dark:text-indigo-300">
                {logKPIs.serviceCount}
                <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("otel.serviceUnit")}</span>
              </span>
              <span class="text-sm font-semibold font-mono text-slate-600 dark:text-slate-400">
                / {logKPIs.hostCount}
                <span class="text-xs font-normal text-slate-500 dark:text-slate-500">{$_("otel.hostUnit")}</span>
              </span>
            </div>
          </div>
        </div>

        <!-- Top Log Histogram -->
        <div class="h-52 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-2 relative shadow-sm dark:shadow-lg shrink-0">
          <div class="text-[11px] font-semibold text-slate-400 absolute top-2 left-4 z-10">
            {$_("otel.logTrendTitle")}
          </div>
          <div id="otelLogChart" class="h-full w-full"></div>
        </div>

        <!-- Log Table -->
        <div class="flex-1 flex flex-col bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm dark:shadow-lg min-h-0">
          <div class="flex items-center justify-between p-3 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60">
            <div class="flex items-center gap-3">
              <div class="relative w-80">
                <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  type="text"
                  bind:value={logSearch}
                  placeholder={$_("otel.searchLogPlaceholder")}
                  class="w-full pl-8 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-slate-950 border border-slate-300 dark:border-slate-800 rounded-lg text-slate-800 dark:text-slate-200 placeholder-slate-400 dark:placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                />
              </div>

              <!-- Level Filter Badges -->
              <div class="flex items-center gap-1">
                {#each [
                  { id: "all", label: $_("otel.filterAll") },
                  { id: "ERROR", label: "ERROR" },
                  { id: "WARN", label: "WARN" },
                  { id: "INFO", label: "INFO" },
                  { id: "DEBUG", label: "DEBUG" }
                ] as lvl}
                  <button
                    onclick={() => (logLevelFilter = lvl.id)}
                    class="px-2.5 py-1 rounded text-xs font-medium transition-all {logLevelFilter === lvl.id ? 'bg-cyan-600 text-white font-semibold' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
                  >
                    {lvl.label}
                  </button>
                {/each}
              </div>
            </div>

            <div class="text-xs text-slate-400">
              {$_("otel.totalRecords", { values: { count: filteredLogs.length } })}
            </div>
          </div>

          <!-- Log Table -->
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800 z-10">
                <tr>
                  <th class="py-1 px-2 text-center cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("level")}>
                    <div class="flex items-center justify-center gap-1">
                      <span>{$_("otel.colLevel")}</span>
                      {#if logSortKey === "level"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("time")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colTime")}</span>
                      {#if logSortKey === "time"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("host")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colHost")}</span>
                      {#if logSortKey === "host"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("service")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colServiceScope")}</span>
                      {#if logSortKey === "service"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("message")}>
                    <div class="flex items-center gap-1">
                      <span>{$_("otel.colMessage")}</span>
                      {#if logSortKey === "message"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1 px-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200 select-none transition-colors" onclick={() => handleLogSort("traceId")}>
                    <div class="flex items-center gap-1">
                      <span>TraceID</span>
                      {#if logSortKey === "traceId"}
                        {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-400" />{/if}
                      {:else}
                        <ArrowUpDown class="h-3 w-3 opacity-30" />
                      {/if}
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60 font-mono">
                {#if paginatedLogs.length === 0}
                  <tr>
                    <td colspan="6" class="py-8 text-center text-slate-500 font-sans">
                      {$_("otel.noLogData")}
                    </td>
                  </tr>
                {:else}
                  {#each paginatedLogs as l}
                    <tr
                      onclick={() => { selectedLog = l; showLogModal = true; }}
                      class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors cursor-pointer"
                    >
                      <td class="py-1 px-2 text-center">
                        <span class="px-2 py-0.5 rounded text-[10px] font-bold font-sans {
                          l.level === 'ERROR'
                            ? 'bg-rose-100 text-rose-800 border-rose-300 dark:bg-rose-500/20 dark:text-rose-300 dark:border-rose-500/40'
                            : l.level === 'WARN'
                            ? 'bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-500/20 dark:text-amber-300 dark:border-amber-500/40'
                            : l.level === 'INFO'
                            ? 'bg-cyan-100 text-cyan-800 border-cyan-300 dark:bg-cyan-500/20 dark:text-cyan-300 dark:border-cyan-500/40'
                            : 'bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-700/30 dark:text-slate-300 dark:border-slate-600/40'
                        }">
                          {l.severityText}
                        </span>
                      </td>
                      <td class="py-1 px-2 text-slate-700 dark:text-slate-300 whitespace-nowrap">{formatTimeStr(l.time)}</td>
                      <td class="py-1 px-2 text-slate-700 dark:text-slate-300 whitespace-nowrap">{l.host}</td>
                      <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-sans whitespace-nowrap">
                        {l.service} <span class="text-slate-400 dark:text-slate-500">/</span> {l.scope}
                      </td>
                      <td class="py-1 px-2 text-slate-900 dark:text-slate-100 font-sans break-all max-w-[400px]">{l.message}</td>
                      <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[10px] truncate max-w-[120px]" title={l.traceId}>{l.traceId}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>

          <!-- Pagination -->
          <div class="flex items-center justify-between p-2.5 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 text-xs text-slate-400">
            <div>
              {$_("otel.pageInfo", { values: { page: logPage, total: Math.max(1, Math.ceil(filteredLogs.length / logPageSize)) } })}
            </div>
            <div class="flex items-center gap-1">
              <button
                disabled={logPage <= 1}
                onclick={() => (logPage = 1)}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                disabled={logPage <= 1}
                onclick={() => logPage--}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                disabled={logPage * logPageSize >= filteredLogs.length}
                onclick={() => logPage++}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                disabled={logPage * logPageSize >= filteredLogs.length}
                onclick={() => (logPage = Math.ceil(filteredLogs.length / logPageSize))}
                class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 disabled:pointer-events-none"
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
</div>

<!-- ================= MODALS ================= -->

<!-- 1. Metric Info Modal -->
{#if showMetricInfo && selectedMetric}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="w-full max-w-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
          <Info class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
          <span>{$_("otel.metricInfoModalTitle")}</span>
        </h3>
        <button onclick={() => (showMetricInfo = false)} class="p-1 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer">
          <X class="h-4 w-4" />
        </button>
      </div>
      <div class="p-4 space-y-2 text-xs">
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colHost")}</span>
          <span class="col-span-2 font-mono text-slate-800 dark:text-slate-200">{selectedMetric.Host}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colService")}</span>
          <span class="col-span-2 text-slate-800 dark:text-slate-200 font-semibold">{selectedMetric.Service}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colScope")}</span>
          <span class="col-span-2 text-slate-700 dark:text-slate-300">{selectedMetric.Scope}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colMetricName")}</span>
          <span class="col-span-2 font-bold text-cyan-600 dark:text-cyan-300">{selectedMetric.Name}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colType")}</span>
          <span class="col-span-2 font-mono text-slate-800 dark:text-slate-200">{selectedMetric.Type}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.unit")}</span>
          <span class="col-span-2 text-slate-800 dark:text-slate-200">{selectedMetric.Unit || "-"}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.description")}</span>
          <span class="col-span-2 text-slate-700 dark:text-slate-300">{selectedMetric.Description || "-"}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.cumulativeReceived")}</span>
          <span class="col-span-2 font-mono text-cyan-600 dark:text-cyan-400 font-bold">{selectedMetric.Count}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5 border-b border-slate-200 dark:border-slate-800/60">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.colFirstTime")}</span>
          <span class="col-span-2 font-mono text-slate-700 dark:text-slate-300">{formatTimeStr(selectedMetric.First)}</span>
        </div>
        <div class="grid grid-cols-3 py-1.5">
          <span class="text-slate-500 dark:text-slate-400 font-medium">{$_("otel.lastTime")}</span>
          <span class="col-span-2 font-mono text-slate-700 dark:text-slate-300">{formatTimeStr(selectedMetric.Last)}</span>
        </div>
      </div>
      <div class="flex justify-end p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40">
        <button
          onclick={() => (showMetricInfo = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_("otel.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 2. Metric Report Modal (Time Chart / Histogram) -->
{#if showMetricReport && metricDetail}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
    <div class="w-[94vw] max-w-6xl h-[88vh] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <!-- Header -->
      <div class="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60 shrink-0">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
            <Activity class="h-5 w-5" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">{metricDetail.Name}</h3>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700">
                {metricDetail.Type}
              </span>
              {#if metricDetail.Unit}
                <span class="px-1.5 py-0.5 rounded text-[10px] bg-cyan-100 dark:bg-cyan-950 text-cyan-800 dark:text-cyan-300 border border-cyan-300 dark:border-cyan-800 font-mono">
                  {metricDetail.Unit}
                </span>
              {/if}
            </div>
            <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
              {$_("otel.colHost")}: <span class="text-slate-700 dark:text-slate-300 font-mono">{metricDetail.Host}</span> | 
              {$_("otel.colService")}: <span class="text-slate-700 dark:text-slate-300">{metricDetail.Service}</span> | 
              {$_("otel.colScope")}: <span class="text-slate-700 dark:text-slate-300">{metricDetail.Scope || "-"}</span>
              {#if metricDetail.Description}
                | <span class="text-slate-500 dark:text-slate-400 italic">{metricDetail.Description}</span>
              {/if}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <!-- Attribute Filter if more than 1 attribute exists -->
          {#if metricAttributesList.length > 1}
            <div class="flex items-center gap-1.5 text-xs text-slate-400">
              <span>{$_("otel.attributesLabel")}</span>
              <select
                bind:value={selectedAttributeFilter}
                onchange={() => tick().then(renderMetricChart)}
                class="px-2 py-1 rounded bg-slate-950 border border-slate-800 text-slate-200 text-xs focus:outline-none focus:border-cyan-500 max-w-[200px] truncate"
              >
                <option value="all">{$_("otel.allAttributes", { values: { count: metricAttributesList.length } })}</option>
                {#each metricAttributesList as attr}
                  <option value={attr}>{attr}</option>
                {/each}
              </select>
            </div>
          {/if}

          {#if metricDetail.Type === "Histogram" || metricDetail.Type === "ExponentialHistogram"}
            <div class="flex items-center bg-slate-950 border border-slate-800 rounded-lg p-0.5 text-xs">
              <button
                onclick={() => {
                  metricChartMode = "time";
                  tick().then(renderMetricChart);
                }}
                class="px-2.5 py-1 rounded {metricChartMode === 'time' ? 'bg-cyan-600 text-white font-medium shadow' : 'text-slate-400 hover:text-slate-200'}"
              >
                {$_("otel.btnTimeSeries")}
              </button>
              <button
                onclick={() => {
                  metricChartMode = "histogram";
                  tick().then(renderMetricChart);
                }}
                class="px-2.5 py-1 rounded {metricChartMode === 'histogram' ? 'bg-cyan-600 text-white font-medium shadow' : 'text-slate-400 hover:text-slate-200'}"
              >
                {$_("otel.btnHistogram")}
              </button>
            </div>
          {/if}

          <button onclick={() => (showMetricReport = false)} class="p-1.5 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer">
            <X class="h-4 w-4" />
          </button>
        </div>
      </div>

      <!-- Modal Body (Chart + Table balanced layout) -->
      <div class="flex-1 min-h-0 p-4 flex flex-col gap-3 overflow-hidden">
        <!-- Chart Container (Fixed height approx 45%, does not shrink) -->
        <div class="h-[340px] shrink-0 flex flex-col bg-slate-950/60 rounded-xl border border-slate-800/80 p-2.5 relative shadow-inner">
          <div class="flex items-center justify-between px-2 pt-1 text-[11px] text-slate-400 shrink-0">
            <span class="font-semibold text-slate-300 flex items-center gap-1.5">
              {#if metricChartMode === "histogram"}
                <BarChart3 class="h-3.5 w-3.5 text-emerald-400" />
                {$_("otel.histogramTitle", { values: { time: selectedDataPoint ? formatTimeStr(selectedDataPoint.Time) : '' } })}
              {:else}
                <Activity class="h-3.5 w-3.5 text-cyan-400" />
                {$_("otel.timeSeriesTitle", { values: { count: filteredModalDataPoints.length } })}
              {/if}
            </span>
            {#if metricChartMode === "histogram"}
              <span class="text-slate-500 text-[10px]">
                {$_("otel.histogramHint")}
              </span>
            {/if}
          </div>
          <div id="metricReportChart" class="flex-1 w-full min-h-0 mt-1"></div>
        </div>

        <!-- DataPoints Table (Flex-1, scrollable, takes remaining 55%) -->
        <div class="flex-1 min-h-0 flex flex-col border border-slate-800 rounded-xl bg-slate-950/60 overflow-hidden shadow-inner">
          <div class="px-3 py-2 border-b border-slate-800 bg-slate-950/80 flex items-center justify-between shrink-0">
            <span class="text-xs font-semibold text-slate-300">
              {$_("otel.dataPointList", { values: { count: filteredModalDataPoints.length } })}
            </span>
            {#if selectedDataPoint && metricChartMode === "histogram"}
              <span class="text-[11px] text-emerald-400 font-mono">
                {$_("otel.selectedPoint", { values: { time: formatTimeStr(selectedDataPoint.Time) } })}
              </span>
            {/if}
          </div>
          <div class="flex-1 overflow-auto">
            <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300 border-collapse">
              <thead class="sticky top-0 bg-slate-950 text-slate-400 border-b border-slate-800 font-medium z-10">
                <tr>
                  <th class="py-2 px-3">{$_("otel.colTime")}</th>
                  <th class="py-2 px-3">{$_("otel.thAttributes")}</th>
                  <th class="py-2 px-3 text-right">{$_("otel.thVal")}</th>
                  {#if metricDetail.Type === "Histogram" || metricDetail.Type === "ExponentialHistogram"}
                    <th class="py-2 px-3 text-right">{$_("otel.thCount")}</th>
                    <th class="py-2 px-3 text-right">{$_("otel.thMinMax")}</th>
                  {/if}
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60 font-mono">
                {#if filteredModalDataPoints.length === 0}
                  <tr>
                    <td colspan="5" class="py-6 text-center text-slate-500">
                      {$_("otel.noDataPoints")}
                    </td>
                  </tr>
                {:else}
                  {#each filteredModalDataPoints as dp}
                    <tr
                      onclick={() => {
                        selectedDataPoint = dp;
                        if (metricChartMode === "histogram") renderMetricChart();
                      }}
                      class="cursor-pointer hover:bg-slate-800/50 transition-colors {selectedDataPoint === dp ? 'bg-cyan-950/50 text-cyan-200 font-medium' : ''}"
                    >
                      <td class="py-1 px-2 text-slate-700 dark:text-slate-300 whitespace-nowrap">{formatTimeStr(dp.Time)}</td>
                      <td class="py-1 px-2 text-slate-400 font-sans truncate max-w-[320px]" title={dp.Attributes?.join(" ")}>
                        {dp.Attributes?.join(" ") || "-"}
                      </td>
                      <td class="py-1 px-2 text-right text-cyan-400 font-semibold">
                        {dp.Sum ?? dp.Gauge ?? "-"}
                      </td>
                      {#if metricDetail.Type === "Histogram" || metricDetail.Type === "ExponentialHistogram"}
                        <td class="py-1 px-2 text-right text-emerald-400">{dp.Count ?? "-"}</td>
                        <td class="py-1 px-2 text-right text-slate-400">
                          {dp.Min !== undefined && dp.Max !== undefined ? `${dp.Min} / ${dp.Max}` : "-"}
                        </td>
                      {/if}
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex justify-end p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 shrink-0">
        <button
          onclick={() => (showMetricReport = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_("otel.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 3. Service DAG Modal -->
{#if showDAGModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-5xl h-[85vh] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
          <GitBranch class="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
          <span>{$_("otel.dagTitle")}</span>
        </h3>
        <button onclick={() => (showDAGModal = false)} class="p-1 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="flex-1 p-2 relative bg-slate-50 dark:bg-slate-950/40 min-h-0">
        <div id="dagChart" class="h-full w-full"></div>
      </div>

      <div class="flex justify-between items-center p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 text-xs text-slate-600 dark:text-slate-400">
        <div>
          {$_("otel.dagNodes")} <span class="font-bold text-slate-800 dark:text-slate-200">{dagData?.Nodes?.length || 0}</span> /
          {$_("otel.dagLinks")} <span class="font-bold text-slate-800 dark:text-slate-200">{dagData?.Links?.length || 0}</span>
        </div>
        <button
          onclick={() => (showDAGModal = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_("otel.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 4. Trace Waterfall (Timeline) Report Modal -->
{#if showTraceReport && traceDetail}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-5xl h-[88vh] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60">
        <div>
          <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
            <Activity class="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
            <span>{$_("otel.traceWaterfallTitle", { values: { traceId: traceDetail.TraceID } })}</span>
          </h3>
          <p class="text-[11px] text-slate-500 dark:text-slate-400">
            {$_("otel.traceDuration")} <span class="font-bold text-cyan-600 dark:text-cyan-300">{(traceDetail.Dur * 1000).toFixed(3)} ms</span> /
            {$_("otel.traceSpanCount")} <span class="font-bold text-slate-800 dark:text-slate-200">{traceDetail.Spans?.length || 0}</span>
          </p>
        </div>
        <button onclick={() => (showTraceReport = false)} class="p-1 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="flex-1 flex flex-col p-4 gap-3 min-h-0 overflow-hidden">
        <!-- Waterfall Timeline Chart -->
        <div id="traceWaterfallChart" class="h-60 w-full bg-slate-50 dark:bg-slate-950/40 rounded-xl border border-slate-200 dark:border-slate-800/80 shrink-0"></div>

        <!-- Spans Detail Table -->
        <div class="flex-1 overflow-auto border border-slate-200 dark:border-slate-800 rounded-xl bg-white/80 dark:bg-slate-950/60">
          <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
              <tr>
                <th class="py-1 px-2">{$_("otel.colSpanName")}</th>
                <th class="py-1 px-2">{$_("otel.colService")}</th>
                <th class="py-1 px-2">{$_("otel.colStartTime")}</th>
                <th class="py-1 px-2">{$_("otel.colEndTime")}</th>
                <th class="py-1 px-2 text-right">{$_("otel.colDuration")}</th>
                <th class="py-1 px-2">Span ID</th>
                <th class="py-1 px-2">{$_("otel.colParentSpan")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60 font-mono">
              {#each (traceDetail.Spans || []) as sp}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                  <td class="py-1 px-2 font-sans font-medium text-slate-800 dark:text-slate-100">{sp.Name}</td>
                  <td class="py-1 px-2 font-sans text-cyan-600 dark:text-cyan-300">{sp.Service}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400">{renderTimeMili(sp.Start)}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400">{renderTimeMili(sp.End)}</td>
                  <td class="py-1 px-2 text-right text-emerald-600 dark:text-emerald-400 font-semibold">{(sp.Dur * 1000).toFixed(3)}</td>
                  <td class="py-1 px-2 text-slate-500 text-[10px]">{sp.SpanID}</td>
                  <td class="py-1 px-2 text-slate-500 text-[10px]">{sp.ParentSpanID || "-"}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

      <div class="flex justify-end p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40">
        <button
          onclick={() => (showTraceReport = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_("otel.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- 5. OTel Log Detail Modal -->
{#if showLogModal && selectedLog}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
    <div class="w-full max-w-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60">
        <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
          <FileText class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
          <span>{$_("otel.logDetailTitle")}</span>
        </h3>
        <button onclick={() => (showLogModal = false)} class="p-1 rounded-lg text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:white hover:bg-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer">
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="p-5 space-y-4 text-xs overflow-auto max-h-[75vh]">
        <div class="grid grid-cols-2 gap-3">
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">{$_("otel.colTime")}</span>
            <span class="text-slate-800 dark:text-slate-200 font-mono">{formatTimeStr(selectedLog.time)}</span>
          </div>
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">{$_("otel.severity")}</span>
            <span class="font-bold {selectedLog.level === 'ERROR' ? 'text-rose-500 dark:text-rose-400' : selectedLog.level === 'WARN' ? 'text-amber-500 dark:text-amber-400' : selectedLog.level === 'INFO' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-600 dark:text-slate-400'}">
              {selectedLog.severityText} (Level: {selectedLog.severity})
            </span>
          </div>
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">{$_("otel.colHost")}</span>
            <span class="text-slate-800 dark:text-slate-200 font-mono">{selectedLog.host}</span>
          </div>
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">{$_("otel.colServiceScope")}</span>
            <span class="text-slate-800 dark:text-slate-200 font-semibold">{selectedLog.service} / {selectedLog.scope}</span>
          </div>
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">Trace ID</span>
            <span class="text-slate-700 dark:text-slate-300 font-mono text-[11px] break-all">{selectedLog.traceId}</span>
          </div>
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80">
            <span class="text-slate-500 dark:text-slate-400 block mb-1">Span ID</span>
            <span class="text-slate-700 dark:text-slate-300 font-mono text-[11px] break-all">{selectedLog.spanId}</span>
          </div>
        </div>

        <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80 space-y-1">
          <span class="text-slate-500 dark:text-slate-400 block font-semibold">{$_("otel.colMessage")}</span>
          <p class="text-slate-800 dark:text-slate-200 whitespace-pre-wrap font-sans leading-relaxed">{selectedLog.message}</p>
        </div>

        {#if selectedLog.attributes && Object.keys(selectedLog.attributes).length > 0}
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80 space-y-2">
            <span class="text-slate-500 dark:text-slate-400 block font-semibold">{$_("otel.thAttributes")}</span>
            <div class="space-y-1">
              {#each Object.entries(selectedLog.attributes) as [k, v]}
                <div class="flex items-start gap-2 font-mono text-[11px]">
                  <span class="text-cyan-600 dark:text-cyan-400 shrink-0">{k}:</span>
                  <span class="text-slate-700 dark:text-slate-300 break-all">{v}</span>
                </div>
              {/each}
            </div>
          </div>
        {/if}

        {#if selectedLog.rawText && selectedLog.rawText !== selectedLog.message}
          <div class="p-3 bg-slate-50 dark:bg-slate-950/50 rounded-xl border border-slate-200 dark:border-slate-800/80 space-y-1">
            <span class="text-slate-500 dark:text-slate-400 block font-semibold">{$_("otel.rawLog")}</span>
            <pre class="text-slate-700 dark:text-slate-400 font-mono text-[10px] whitespace-pre-wrap break-all max-h-40 overflow-auto">{selectedLog.rawText}</pre>
          </div>
        {/if}
      </div>

      <div class="flex justify-end p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40">
        <button
          onclick={() => (showLogModal = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_("otel.close")}
        </button>
      </div>
    </div>
  </div>
{/if}
