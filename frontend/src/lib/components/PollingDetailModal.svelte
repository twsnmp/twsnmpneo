<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { _, locale } from "svelte-i18n";
  import type * as echarts from "echarts";
  import {
    getStateColor,
    getStateLabel,
    getStatusBadgeClass,
    formatTimeStr,
    getLogModeName,
    getLogModeBadgeClass,
    renderSLA,
  } from "../common";
  import {
    fetchPolling,
    fetchPollingLogs,
    deletePollingLogs,
    fetchPollingTimeAnalyze,
    fetchAIResult,
    fetchMe,
    type PollingEnt,
    type NodeEnt,
    type PollingLogEnt,
    type PollingTimeAnalyzeResp,
    type AIResultEnt,
  } from "../api";
  import { showConfirm, showAlert } from "../stores/modalStore";
  import ReportAIExplainModal from "./ReportAIExplainModal.svelte";
  import CorrelationModal from "./CorrelationModal.svelte";
  import {
    extractPollingMetricKeys,
    getMetricMeta,
    getMetricMul,
    calcSinglePollingDowntimeAndSLA,
    showSinglePollingDowntimeChart,
    showPollingLogStateChart,
    showPollingTimeChart,
    showPollingHistogram,
    showPollingQQPlot,
    showPollingSTLChart,
    showPollingFFTChart,
    type NumericStatsSummary,
  } from "../charts/polling";
  import {
    renderAIHeatmap,
    renderAIPieChart,
    renderAITimeChart,
  } from "../charts/ai";
  import {
    X,
    Activity,
    Clock,
    Terminal,
    Edit,
    Copy,
    Check,
    Layers,
    Sliders,
    RefreshCw,
    Trash2,
    Download,
    Sparkles,
    Maximize2,
    Minimize2,
    AlertTriangle,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    RotateCcw,
    Search,
    Info,
    GitCompareArrows,
  } from "@lucide/svelte";

  type TabId =
    | "basic"
    | "downtime"
    | "log"
    | "time"
    | "histogram"
    | "qqplot"
    | "stl"
    | "fft"
    | "ai";

  type RangePreset = "all" | "1h" | "6h" | "24h" | "7d" | "30d" | "custom";

  let {
    show = $bindable(false),
    polling = null,
    node = null,
    onEdit,
    onLogsCleared,
  }: {
    show: boolean;
    polling: PollingEnt | null;
    node?: NodeEnt | null;
    onEdit?: (p: PollingEnt) => void;
    onLogsCleared?: (pollingId: string) => void;
  } = $props();

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);
  const collator = $derived(
    new Intl.Collator($locale || undefined, { numeric: true, sensitivity: "base" })
  );

  let activeTab = $state<TabId>("basic");
  let maximized = $state(false);
  let copied = $state(false);
  let isReadonly = $state(false);

  // Active polling (refreshed from API when opened)
  let loadedPolling = $state<PollingEnt | null>(null);
  const activePolling = $derived(loadedPolling || polling);
  const pollingId = $derived(activePolling?.id || (activePolling as any)?.ID || "");
  const pollingName = $derived(activePolling?.name || (activePolling as any)?.Name || "");
  const pollingType = $derived(activePolling?.type || (activePolling as any)?.Type || "");
  const pollingState = $derived(activePolling?.state || (activePolling as any)?.State || "unknown");
  const pollingLogMode = $derived(activePolling?.log_mode ?? (activePolling as any)?.LogMode ?? 0);
  const pollingInt = $derived(activePolling?.poll_int || (activePolling as any)?.PollInt || 60);

  // Range filter state (Default: All Time per decision #3)
  let rangePreset = $state<RangePreset>("all");
  let customStartInput = $state("");
  let customEndInput = $state("");
  let appliedStartNs = $state(0);
  let appliedEndNs = $state(0);

  // Polling logs & analysis state
  let logs = $state<PollingLogEnt[]>([]);
  let logsTruncated = $state(false);
  let loadingLogs = $state(false);
  let logsError = $state("");

  let timeAnalyze = $state<PollingTimeAnalyzeResp | null>(null);
  let loadingTimeAnalyze = $state(false);
  let timeAnalyzeRangeKey = $state("");

  let aiResult = $state<AIResultEnt | null>(null);
  let loadingAI = $state(false);

  // Chart options
  let selectedMetric = $state("");
  let timeChartMode = $state<"raw" | "hourly">("raw");
  let showForecast = $state(false);
  let stlResolution = $state<"H" | "PX2">("H");
  let fftResolution = $state<"H" | "PX2">("H");
  let aiChartMode = $state<"heatmap" | "pie" | "time">("heatmap");
  let histStats = $state<NumericStatsSummary | null>(null);

  // Log table controls & zoom linkage
  let zoomRange = $state<[number, number]>([0, 0]);
  let logFilter = $state("");
  let logSortCol = $state<"time" | "state" | "result">("time");
  let logSortDir = $state<"asc" | "desc">("desc");
  let logPage = $state(1);
  const logPageSize = 50;

  // Downtime table controls
  let dtSortCol = $state<"start" | "duration" | "state">("start");
  let dtSortDir = $state<"asc" | "desc">("desc");

  // AI Explain sub-modal
  let showAiExplainModal = $state(false);
  let showCorrelationModal = $state(false);

  // Chart DOM refs & ECharts instance
  let chartEl: HTMLDivElement | null = $state(null);
  let chartInstance: echarts.ECharts | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let themeObserver: MutationObserver | null = null;

  const tabs: { id: TabId; labelKey: string }[] = [
    { id: "basic", labelKey: "pollingDetail.tabBasic" },
    { id: "downtime", labelKey: "pollingDetail.tabDowntime" },
    { id: "log", labelKey: "pollingDetail.tabLog" },
    { id: "time", labelKey: "pollingDetail.tabTimeSeries" },
    { id: "histogram", labelKey: "pollingDetail.tabHistogram" },
    { id: "qqplot", labelKey: "pollingDetail.tabQQPlot" },
    { id: "stl", labelKey: "pollingDetail.tabSTL" },
    { id: "fft", labelKey: "pollingDetail.tabFFT" },
    { id: "ai", labelKey: "pollingDetail.tabAI" },
  ];

  const metricKeys = $derived.by(() => extractPollingMetricKeys(logs));
  const downtimeResult = $derived.by(() => calcSinglePollingDowntimeAndSLA(logs, pollingInt));

  $effect(() => {
    if (metricKeys.length > 0 && !metricKeys.includes(selectedMetric)) {
      selectedMetric = metricKeys[0];
    } else if (metricKeys.length === 0) {
      selectedMetric = "";
    }
  });

  const filteredLogs = $derived.by(() => {
    let list = logs;
    if (zoomRange[0] > 0 && zoomRange[1] > 0) {
      list = list.filter((l) => l.Time >= zoomRange[0] && l.Time <= zoomRange[1]);
    }
    const q = logFilter.trim().toLowerCase();
    if (q) {
      list = list.filter((l) => {
        if ((l.State || "").toLowerCase().includes(q)) return true;
        return formatLogResultSummary(l.Result).toLowerCase().includes(q);
      });
    }
    return [...list].sort((a, b) => {
      let cmp = 0;
      if (logSortCol === "time") {
        cmp = a.Time - b.Time;
      } else if (logSortCol === "state") {
        cmp = collator.compare(a.State || "", b.State || "");
      } else {
        cmp = collator.compare(
          formatLogResultSummary(a.Result),
          formatLogResultSummary(b.Result)
        );
      }
      return logSortDir === "asc" ? cmp : -cmp;
    });
  });

  const logTotalPages = $derived(Math.max(1, Math.ceil(filteredLogs.length / logPageSize)));
  const paginatedLogs = $derived.by(() => {
    const page = Math.min(logPage, logTotalPages);
    const start = (page - 1) * logPageSize;
    return filteredLogs.slice(start, start + logPageSize);
  });

  const sortedDowntimeEvents = $derived.by(() => {
    return [...downtimeResult.events].sort((a, b) => {
      let cmp = 0;
      if (dtSortCol === "start") cmp = a.startNs - b.startNs;
      else if (dtSortCol === "duration") cmp = a.durationSec - b.durationSec;
      else cmp = collator.compare(a.state, b.state);
      return dtSortDir === "asc" ? cmp : -cmp;
    });
  });

  function formatLogResultSummary(res: Record<string, any> | undefined): string {
    if (!res || typeof res !== "object") return "-";
    const parts: string[] = [];
    for (const [k, v] of Object.entries(res)) {
      if (k === "lastTime" || k === "last_time" || k.startsWith("_")) continue;
      if ((k === "rtt" || k === "offset") && typeof v === "number" && Number.isFinite(v)) {
        parts.push(`${k}=${(v * 1e-6).toFixed(2)}ms`);
      } else if (typeof v === "number" && Number.isFinite(v)) {
        parts.push(`${k}=${Number.isInteger(v) ? v : v.toFixed(2)}`);
      } else if (v !== undefined && v !== null && v !== "") {
        parts.push(`${k}=${typeof v === "object" ? JSON.stringify(v) : String(v)}`);
      }
    }
    return parts.join(", ") || "-";
  }

  function formatDurationSec(sec: number): string {
    if (!Number.isFinite(sec) || sec <= 0) return "0s";
    const s = Math.round(sec);
    const d = Math.floor(s / 86400);
    const h = Math.floor((s % 86400) / 3600);
    const m = Math.floor((s % 3600) / 60);
    const rem = s % 60;
    const out: string[] = [];
    if (d > 0) out.push(`${d}d`);
    if (h > 0) out.push(`${h}h`);
    if (m > 0) out.push(`${m}m`);
    if (rem > 0 || out.length === 0) out.push(`${rem}s`);
    return out.join(" ");
  }

  function formatStatValue(val: number, key: string): string {
    if (!Number.isFinite(val)) return "-";
    const { unit } = getMetricMeta(key, isJaLocale);
    const txt = Math.abs(val) >= 100 ? val.toFixed(1) : val.toFixed(3);
    return unit ? `${txt} ${unit}` : txt;
  }

  function computeRangeNs(preset: RangePreset): { start: number; end: number } {
    if (preset === "all") return { start: 0, end: 0 };
    if (preset === "custom") {
      const s = customStartInput ? new Date(customStartInput).getTime() * 1e6 : 0;
      const e = customEndInput ? new Date(customEndInput).getTime() * 1e6 : 0;
      return {
        start: Number.isFinite(s) && s > 0 ? s : 0,
        end: Number.isFinite(e) && e > 0 ? e : 0,
      };
    }
    const nowNs = Date.now() * 1e6;
    const hoursMap: Record<Exclude<RangePreset, "all" | "custom">, number> = {
      "1h": 1,
      "6h": 6,
      "24h": 24,
      "7d": 24 * 7,
      "30d": 24 * 30,
    };
    const durNs = hoursMap[preset] * 3600 * 1e9;
    return { start: Math.max(1, nowNs - durNs), end: nowNs };
  }

  async function loadAllData(forceTimeAnalyze = false) {
    const pid = polling?.id || (polling as any)?.ID || "";
    if (!pid) return;

    const { start, end } = computeRangeNs(rangePreset);
    appliedStartNs = start;
    appliedEndNs = end;
    zoomRange = [0, 0];
    logPage = 1;

    loadingLogs = true;
    logsError = "";

    const [meRes, freshPoll, logsRes, aiRes] = await Promise.allSettled([
      fetchMe(),
      fetchPolling(pid),
      fetchPollingLogs(pid, { start, end, limit: 10000 }),
      fetchAIResult(pid),
    ]);

    if (meRes.status === "fulfilled" && meRes.value) {
      isReadonly = meRes.value.role === "readonly";
    }
    if (freshPoll.status === "fulfilled" && freshPoll.value) {
      loadedPolling = freshPoll.value;
    }
    if (logsRes.status === "fulfilled") {
      logs = logsRes.value.logs;
      logsTruncated = logsRes.value.truncated;
    } else {
      logs = [];
      logsTruncated = false;
      logsError = logsRes.reason?.message || String(logsRes.reason);
    }
    loadingLogs = false;

    loadingAI = true;
    if (aiRes.status === "fulfilled") {
      aiResult = aiRes.value;
    } else {
      aiResult = null;
    }
    loadingAI = false;

    const rangeKey = `${pid}:${start}:${end}`;
    if (forceTimeAnalyze || activeTab === "stl" || activeTab === "fft") {
      await ensureTimeAnalyze(rangeKey, forceTimeAnalyze);
    }

    await tick();
    renderActiveChart();
  }

  async function ensureTimeAnalyze(rangeKey?: string, force = false) {
    const pid = pollingId;
    if (!pid) return;
    const key = rangeKey || `${pid}:${appliedStartNs}:${appliedEndNs}`;
    if (!force && timeAnalyzeRangeKey === key && timeAnalyze) return;

    loadingTimeAnalyze = true;
    try {
      timeAnalyze = await fetchPollingTimeAnalyze(pid, {
        start: appliedStartNs,
        end: appliedEndNs,
        limit: 20000,
      });
      timeAnalyzeRangeKey = key;
    } catch (e) {
      console.error("Failed to fetch time analyze:", e);
      timeAnalyze = null;
    } finally {
      loadingTimeAnalyze = false;
    }
  }

  function disposeChart() {
    if (chartInstance) {
      chartInstance.dispose();
      chartInstance = null;
    }
  }

  function renderActiveChart() {
    disposeChart();
    histStats = null;
    if (!show || !chartEl) return;

    const pid = pollingId;
    if (!pid) return;

    if (activeTab === "downtime") {
      chartInstance =
        showSinglePollingDowntimeChart(chartEl, downtimeResult, {
          durationSec: $_("pollingChart.durationSec"),
          time: $_("pollingDetail.time"),
          noDowntime: $_("pollingChart.noDowntime"),
        }) || null;
    } else if (activeTab === "log") {
      if (logs.length === 0) return;
      chartInstance =
        showPollingLogStateChart(
          chartEl,
          logs,
          {
            normal: $_("common.normal") || "Normal",
            warn: $_("common.warn") || "Warn",
            low: $_("common.low") || "Low",
            high: $_("common.high") || "High",
            count: $_("pollingChart.count"),
          },
          (st, et) => {
            zoomRange = [st, et];
            logPage = 1;
          }
        ) || null;
    } else if (activeTab === "time") {
      if (!selectedMetric || logs.length === 0) return;
      chartInstance =
        showPollingTimeChart(
          chartEl,
          logs,
          selectedMetric,
          timeChartMode,
          showForecast,
          {
            mean: $_("pollingChart.mean"),
            max: $_("pollingChart.max"),
            min: $_("pollingChart.min"),
            median: $_("pollingChart.median"),
            variance: $_("pollingChart.variance"),
            forecast: $_("pollingChart.forecast"),
          },
          isJaLocale
        ) || null;
    } else if (activeTab === "histogram") {
      if (!selectedMetric || logs.length === 0) return;
      const res = showPollingHistogram(
        chartEl,
        logs,
        selectedMetric,
        {
          count: $_("pollingChart.count"),
          mean: $_("pollingChart.mean"),
        },
        isJaLocale
      );
      chartInstance = res.chart || null;
      histStats = res.stats;
    } else if (activeTab === "qqplot") {
      if (!selectedMetric || logs.length === 0) return;
      chartInstance =
        showPollingQQPlot(
          chartEl,
          logs,
          selectedMetric,
          {
            theoreticalQuantile: $_("pollingChart.theoreticalQuantile"),
            sampleQuantile: $_("pollingChart.sampleQuantile"),
            referenceLine: $_("pollingChart.referenceLine"),
          },
          isJaLocale
        ) || null;
    } else if (activeTab === "stl") {
      if (!timeAnalyze || !selectedMetric) return;
      const hasData =
        stlResolution === "H"
          ? Boolean(timeAnalyze.StlMapH?.[selectedMetric] && timeAnalyze.TimeH.length > 0)
          : Boolean(timeAnalyze.StlMapPX2?.[selectedMetric] && timeAnalyze.TimePX2.length > 0);
      if (!hasData) return;
      chartInstance =
        showPollingSTLChart(
          chartEl,
          timeAnalyze,
          selectedMetric,
          stlResolution,
          {
            observed: $_("pollingChart.observed"),
            trend: $_("pollingChart.trend"),
            seasonal: $_("pollingChart.seasonal"),
            residual: $_("pollingChart.residual"),
          },
          isJaLocale
        ) || null;
    } else if (activeTab === "fft") {
      if (!timeAnalyze || !selectedMetric) return;
      const pts =
        fftResolution === "H"
          ? timeAnalyze.FFTH?.[selectedMetric]
          : timeAnalyze.FFTPX2?.[selectedMetric];
      if (!pts || pts.length === 0) return;
      chartInstance =
        showPollingFFTChart(
          chartEl,
          timeAnalyze,
          selectedMetric,
          fftResolution,
          {
            frequency: $_("pollingChart.frequency"),
            amplitude: $_("pollingChart.amplitude"),
            period: $_("pollingChart.period"),
          },
          isJaLocale
        ) || null;
    } else if (activeTab === "ai") {
      const scores = aiResult?.ScoreData || [];
      if (scores.length === 0) return;
      if (aiChartMode === "heatmap") {
        chartInstance =
          renderAIHeatmap(chartEl, scores, {
            heatmapDate: $_("AIReport.HeatmapDate") || "Date",
            heatmapHour: $_("AIReport.HeatmapHour") || "Hour",
          }) || null;
      } else if (aiChartMode === "pie") {
        chartInstance =
          renderAIPieChart(chartEl, scores, {
            anomalyScore: $_("AIList.AnomaryScore") || "Anomaly Score",
            normal: $_("common.normal") || "Normal",
            warn: $_("common.warn") || "Warning",
            anomaly: $_("common.high") || "Anomaly",
          }) || null;
      } else {
        chartInstance =
          renderAITimeChart(chartEl, scores, {
            anomalyScore: $_("AIList.AnomaryScore") || "Anomaly Score",
            normal: $_("common.normal") || "Normal",
            anomaly: $_("common.high") || "Anomaly",
          }) || null;
      }
    }
  }

  let lastOpenKey = $state("");
  $effect(() => {
    const pid = polling?.id || (polling as any)?.ID || "";
    const openKey = show && pid ? pid : "";
    if (openKey && openKey !== lastOpenKey) {
      lastOpenKey = openKey;
      loadedPolling = null;
      activeTab = "basic";
      maximized = false;
      rangePreset = "all";
      customStartInput = "";
      customEndInput = "";
      timeChartMode = "raw";
      showForecast = false;
      stlResolution = "H";
      fftResolution = "H";
      aiChartMode = "heatmap";
      logFilter = "";
      zoomRange = [0, 0];
      timeAnalyze = null;
      timeAnalyzeRangeKey = "";
      aiResult = null;
      loadAllData();
    } else if (!openKey && lastOpenKey) {
      lastOpenKey = "";
      disposeChart();
    }
  });

  $effect(() => {
    if (!show || !chartEl) return;
    resizeObserver?.disconnect();
    resizeObserver = new ResizeObserver(() => {
      chartInstance?.resize();
    });
    resizeObserver.observe(chartEl);
    return () => {
      resizeObserver?.disconnect();
    };
  });

  onMount(() => {
    themeObserver = new MutationObserver(() => {
      if (show && activeTab !== "basic") {
        renderActiveChart();
      }
    });
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });
  });

  onDestroy(() => {
    disposeChart();
    resizeObserver?.disconnect();
    themeObserver?.disconnect();
  });

  async function selectTab(tab: TabId) {
    activeTab = tab;
    if (tab === "stl" || tab === "fft") {
      await ensureTimeAnalyze();
    }
    await tick();
    renderActiveChart();
  }

  function handleTabKeydown(e: KeyboardEvent, idx: number) {
    let nextIdx = -1;
    if (e.key === "ArrowRight") nextIdx = (idx + 1) % tabs.length;
    else if (e.key === "ArrowLeft") nextIdx = (idx - 1 + tabs.length) % tabs.length;
    else if (e.key === "Home") nextIdx = 0;
    else if (e.key === "End") nextIdx = tabs.length - 1;
    if (nextIdx >= 0) {
      e.preventDefault();
      const targetTab = tabs[nextIdx].id;
      selectTab(targetTab);
      document.getElementById(`polling-tab-${targetTab}`)?.focus();
    }
  }

  async function selectPreset(preset: RangePreset) {
    rangePreset = preset;
    if (preset === "custom") {
      if (!customEndInput) {
        const now = new Date();
        const prev = new Date(now.getTime() - 24 * 3600 * 1000);
        const fmtLocal = (d: Date) =>
          new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
        customStartInput = fmtLocal(prev);
        customEndInput = fmtLocal(now);
      }
      return;
    }
    await loadAllData(activeTab === "stl" || activeTab === "fft");
  }

  async function handleClearLogs() {
    if (isReadonly || !pollingId) return;
    const confirmed = await showConfirm({
      title: $_("pollingDetail.clearLogs"),
      message: $_("pollingDetail.clearLogsConfirm"),
      detail: pollingName,
      type: "danger",
    });
    if (!confirmed) return;

    try {
      await deletePollingLogs(pollingId);
      logs = [];
      logsTruncated = false;
      timeAnalyze = null;
      timeAnalyzeRangeKey = "";
      aiResult = null;
      zoomRange = [0, 0];
      await tick();
      renderActiveChart();
      onLogsCleared?.(pollingId);
      await showAlert({
        title: $_("pollingDetail.clearLogs"),
        message: $_("pollingDetail.logsCleared"),
        type: "info",
      });
    } catch (e: any) {
      await showAlert({
        title: $_("common.error") || "Error",
        message: e?.message || String(e),
        type: "danger",
      });
    }
  }

  function csvEscape(v: any): string {
    const s = String(v ?? "");
    if (s.includes(",") || s.includes('"') || s.includes("\n") || s.includes("\r")) {
      return `"${s.replace(/"/g, '""')}"`;
    }
    return s;
  }

  function downloadCsv(filename: string, rows: string[]) {
    const bom = "\uFEFF";
    const blob = new Blob([bom + rows.join("\r\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function exportLogsCsv() {
    const header = ["Time", "State", "Result"].map(csvEscape).join(",");
    const lines = [header];
    for (const l of filteredLogs) {
      lines.push(
        [formatTimeStr(l.Time), l.State || "", JSON.stringify(l.Result || {})]
          .map(csvEscape)
          .join(",")
      );
    }
    const safeName = (pollingId || "polling").replace(/[^a-zA-Z0-9_-]/g, "_");
    downloadCsv(`polling_logs_${safeName}.csv`, lines);
  }

  function exportNumericDataCsv() {
    const keys = metricKeys;
    const header = [
      "Time",
      "State",
      ...keys.map((k) => {
        const meta = getMetricMeta(k, isJaLocale);
        return meta.unit ? `${k} (${meta.unit})` : k;
      }),
    ]
      .map(csvEscape)
      .join(",");
    const lines = [header];
    for (const l of filteredLogs) {
      const row: any[] = [formatTimeStr(l.Time), l.State || ""];
      for (const k of keys) {
        const raw = l?.Result?.[k];
        if (typeof raw === "number" && Number.isFinite(raw)) {
          row.push(raw * getMetricMul(k));
        } else {
          row.push("");
        }
      }
      lines.push(row.map(csvEscape).join(","));
    }
    const safeName = (pollingId || "polling").replace(/[^a-zA-Z0-9_-]/g, "_");
    downloadCsv(`polling_data_${safeName}.csv`, lines);
  }

  const copyResultJson = async () => {
    if (!activePolling?.result) return;
    try {
      await navigator.clipboard.writeText(JSON.stringify(activePolling.result, null, 2));
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch (e) {
      console.error("Failed to copy result:", e);
    }
  };

  function toggleLogSort(col: "time" | "state" | "result") {
    if (logSortCol === col) {
      logSortDir = logSortDir === "asc" ? "desc" : "asc";
    } else {
      logSortCol = col;
      logSortDir = col === "time" ? "desc" : "asc";
    }
    logPage = 1;
  }

  function toggleDtSort(col: "start" | "duration" | "state") {
    if (dtSortCol === col) {
      dtSortDir = dtSortDir === "asc" ? "desc" : "asc";
    } else {
      dtSortCol = col;
      dtSortDir = col === "start" ? "desc" : "asc";
    }
  }

  const aiSummaryText = $derived.by(() => {
    const dt = downtimeResult;
    const lines = [
      `Polling: ${pollingName} (${pollingType}) on ${node?.name || node?.ip || ""}`,
      `State: ${pollingState}, LogMode: ${pollingLogMode}, Interval: ${pollingInt}s`,
      `Logs: ${logs.length}, SLA: ${renderSLA(dt.slaPercent)}%, Outages: ${dt.outageCount}, Downtime: ${formatDurationSec(dt.totalDowntimeSec)}`,
    ];
    if (selectedMetric && histStats && histStats.count > 0) {
      lines.push(
        `Metric ${selectedMetric}: mean=${formatStatValue(histStats.mean, selectedMetric)}, min=${formatStatValue(histStats.min, selectedMetric)}, max=${formatStatValue(histStats.max, selectedMetric)}`
      );
    }
    return lines.join(" | ");
  });

  const aiItemsList = $derived.by(() =>
    filteredLogs.slice(0, 60).map((l) => `${formatTimeStr(l.Time)} [${l.State}] ${formatLogResultSummary(l.Result)}`)
  );

  const hasCurrentStl = $derived.by(() => {
    if (!timeAnalyze || !selectedMetric) return false;
    return stlResolution === "H"
      ? Boolean(timeAnalyze.StlMapH?.[selectedMetric] && timeAnalyze.TimeH.length > 0)
      : Boolean(timeAnalyze.StlMapPX2?.[selectedMetric] && timeAnalyze.TimePX2.length > 0);
  });

  const hasCurrentFft = $derived.by(() => {
    if (!timeAnalyze || !selectedMetric) return false;
    const pts =
      fftResolution === "H"
        ? timeAnalyze.FFTH?.[selectedMetric]
        : timeAnalyze.FFTPX2?.[selectedMetric];
    return Boolean(pts && pts.length > 0);
  });
</script>

{#if show && activePolling}
  <div
    data-testid="pollingdetail-modal"
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/75 p-2 sm:p-4 backdrop-blur-md overscroll-contain"
    role="dialog"
    aria-modal="true"
    aria-labelledby="polling-detail-title"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape" && !showAiExplainModal) show = false;
    }}
  >
    <div
      class="flex flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200 transition-all {maximized
        ? 'w-[96vw] max-w-[96vw] h-[95vh] max-h-[95vh]'
        : 'w-full max-w-6xl h-[88vh] max-h-[88vh]'}"
    >
      <!-- Modal Header -->
      <div
        class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/70 px-5 py-3 shrink-0"
      >
        <div class="flex items-center gap-3 min-w-0">
          <div
            class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-blue-500/10 dark:bg-cyan-500/20 border border-blue-500/20 dark:border-cyan-500/30 text-blue-600 dark:text-cyan-400"
          >
            <Activity class="h-5 w-5" aria-hidden="true" />
          </div>
          <div class="min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <h2
                id="polling-detail-title"
                class="text-base font-bold text-slate-900 dark:text-slate-100 truncate"
              >
                {pollingName}
              </h2>
              <span
                class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadgeClass(pollingState)}"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  style="background-color: {getStateColor(pollingState)}"
                ></span>
                {getStateLabel(pollingState)}
              </span>
              <span
                class="rounded-md px-2 py-0.5 text-[10px] font-semibold uppercase bg-blue-50 dark:bg-cyan-500/10 text-blue-700 dark:text-cyan-300 border border-blue-200 dark:border-cyan-500/30"
              >
                {pollingType}
              </span>
              <span
                class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-semibold border {getLogModeBadgeClass(pollingLogMode)}"
              >
                {getLogModeName(pollingLogMode, $_)}
              </span>
            </div>
            <p class="text-[11px] font-mono text-slate-500 dark:text-cyan-400 truncate">
              {node?.name ? `${node.name}${node.ip ? ` (${node.ip})` : ""}` : node?.ip || ""}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1.5 shrink-0">
          <button
            type="button"
            onclick={() => (maximized = !maximized)}
            aria-label={maximized ? $_("pollingDetail.restore") : $_("pollingDetail.maximize")}
            title={maximized ? $_("pollingDetail.restore") : $_("pollingDetail.maximize")}
            class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
          >
            {#if maximized}
              <Minimize2 class="h-4 w-4" aria-hidden="true" />
            {:else}
              <Maximize2 class="h-4 w-4" aria-hidden="true" />
            {/if}
          </button>
          <button
            type="button"
            data-testid="pollingdetail-close"
            aria-label={$_("common.close")}
            onclick={() => (show = false)}
            class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
          >
            <X class="h-5 w-5" aria-hidden="true" />
          </button>
        </div>
      </div>

      <!-- Accessible Single-Row Tab Strip -->
      <div
        role="tablist"
        aria-label={$_("pollingDetail.title")}
        class="flex items-center gap-1 px-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-900/50 overflow-x-auto whitespace-nowrap shrink-0"
      >
        {#each tabs as tab, idx}
          {@const selected = activeTab === tab.id}
          <button
            id={`polling-tab-${tab.id}`}
            data-testid={`pollingdetail-tab-${tab.id}`}
            type="button"
            role="tab"
            aria-selected={selected}
            aria-controls={`polling-panel-${tab.id}`}
            tabindex={selected ? 0 : -1}
            onclick={() => selectTab(tab.id)}
            onkeydown={(e) => handleTabKeydown(e, idx)}
            class="relative inline-flex items-center gap-1.5 px-3 py-2.5 text-xs font-semibold whitespace-nowrap transition-colors cursor-pointer {selected
              ? 'text-blue-600 dark:text-cyan-400'
              : 'text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
          >
            <span>{$_(tab.labelKey)}</span>
            {#if tab.id === "log" && logs.length > 0}
              <span
                class="rounded-full px-1.5 py-0 text-[10px] font-mono bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300"
              >
                {logs.length}
              </span>
            {/if}
            {#if tab.id === "ai" && aiResult?.ScoreData?.length}
              <span
                class="rounded-full px-1.5 py-0 text-[10px] font-mono bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-300"
              >
                {aiResult.ScoreData.length}
              </span>
            {/if}
            {#if selected}
              <span
                aria-hidden="true"
                class="absolute bottom-0 left-2 right-2 h-0.5 rounded-full bg-blue-600 dark:bg-cyan-400"
              ></span>
            {/if}
          </button>
        {/each}
      </div>

      <!-- Time Range & Filter Toolbar (visible on log/chart tabs) -->
      {#if activeTab !== "basic"}
        <div
          class="flex flex-wrap items-center justify-between gap-2 px-5 py-2 border-b border-slate-200 dark:border-slate-800/80 bg-white dark:bg-slate-900/30 text-xs shrink-0"
        >
          <div class="flex flex-wrap items-center gap-1.5">
            {#each [
              { id: "all", key: "pollingDetail.rangeAll" },
              { id: "1h", key: "pollingDetail.range1h" },
              { id: "6h", key: "pollingDetail.range6h" },
              { id: "24h", key: "pollingDetail.range24h" },
              { id: "7d", key: "pollingDetail.range7d" },
              { id: "30d", key: "pollingDetail.range30d" },
              { id: "custom", key: "pollingDetail.rangeCustom" },
            ] as preset}
              <button
                type="button"
                onclick={() => selectPreset(preset.id as RangePreset)}
                class="px-2.5 py-1 rounded-lg text-[11px] font-medium border transition-colors cursor-pointer {rangePreset === preset.id
                  ? 'border-blue-500 bg-blue-500/10 text-blue-600 dark:border-cyan-500 dark:bg-cyan-500/15 dark:text-cyan-300'
                  : 'border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/60 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'}"
              >
                {$_(preset.key)}
              </button>
            {/each}

            {#if rangePreset === "custom"}
              <div class="flex flex-wrap items-center gap-1.5 ml-1">
                <input
                  type="datetime-local"
                  aria-label={$_("pollingDetail.startTime")}
                  bind:value={customStartInput}
                  class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-[11px] text-slate-800 dark:text-slate-200"
                />
                <span class="text-slate-400">~</span>
                <input
                  type="datetime-local"
                  aria-label={$_("pollingDetail.endTime")}
                  bind:value={customEndInput}
                  class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-[11px] text-slate-800 dark:text-slate-200"
                />
                <button
                  type="button"
                  onclick={() => loadAllData(activeTab === "stl" || activeTab === "fft")}
                  class="px-2.5 py-1 rounded-lg bg-blue-600 dark:bg-cyan-600 text-white text-[11px] font-semibold hover:opacity-90 cursor-pointer"
                >
                  {$_("pollingDetail.applyRange")}
                </button>
              </div>
            {/if}
          </div>

          <div class="flex items-center gap-2">
            {#if metricKeys.length > 0 && (activeTab === "time" || activeTab === "histogram" || activeTab === "qqplot" || activeTab === "stl" || activeTab === "fft")}
              <label class="inline-flex items-center gap-1.5 text-[11px] text-slate-500 dark:text-slate-400">
                <span>{$_("pollingDetail.metric")}:</span>
                <select
                  bind:value={selectedMetric}
                  onchange={() => {
                    tick().then(renderActiveChart);
                  }}
                  class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-[11px] font-mono text-slate-800 dark:text-slate-200"
                >
                  {#each metricKeys as mk}
                    <option value={mk}>{mk} — {getMetricMeta(mk, isJaLocale).label}</option>
                  {/each}
                </select>
              </label>
            {/if}

            {#if activeTab === "time"}
              <div class="inline-flex rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
                <button
                  type="button"
                  onclick={() => {
                    timeChartMode = "raw";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {timeChartMode === 'raw'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.rawMode")}
                </button>
                <button
                  type="button"
                  onclick={() => {
                    timeChartMode = "hourly";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {timeChartMode === 'hourly'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.hourlyMode")}
                </button>
              </div>
              <label class="inline-flex items-center gap-1.5 text-[11px] text-slate-600 dark:text-slate-300 cursor-pointer select-none">
                <input
                  type="checkbox"
                  bind:checked={showForecast}
                  onchange={() => tick().then(renderActiveChart)}
                  class="rounded border-slate-300 dark:border-slate-700 text-blue-600"
                />
                <span>{$_("pollingDetail.showForecast")}</span>
              </label>
            {/if}

            {#if activeTab === "stl"}
              <div class="inline-flex rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
                <button
                  type="button"
                  onclick={() => {
                    stlResolution = "H";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {stlResolution === 'H'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.resHourly")}
                </button>
                <button
                  type="button"
                  onclick={() => {
                    stlResolution = "PX2";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {stlResolution === 'PX2'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.resSubHourly", { values: { sec: timeAnalyze?.PX2 || 120 } })}
                </button>
              </div>
            {/if}

            {#if activeTab === "fft"}
              <div class="inline-flex rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
                <button
                  type="button"
                  onclick={() => {
                    fftResolution = "H";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {fftResolution === 'H'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.resHourly")}
                </button>
                <button
                  type="button"
                  onclick={() => {
                    fftResolution = "PX2";
                    tick().then(renderActiveChart);
                  }}
                  class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {fftResolution === 'PX2'
                    ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                    : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                >
                  {$_("pollingDetail.resSubHourly", { values: { sec: timeAnalyze?.PX2 || 120 } })}
                </button>
              </div>
            {/if}

            {#if activeTab === "ai"}
              <div class="inline-flex rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
                {#each [
                  { id: "heatmap", key: "pollingDetail.aiHeatmap" },
                  { id: "pie", key: "pollingDetail.aiPie" },
                  { id: "time", key: "pollingDetail.aiTime" },
                ] as modeBtn}
                  <button
                    type="button"
                    onclick={() => {
                      aiChartMode = modeBtn.id as "heatmap" | "pie" | "time";
                      tick().then(renderActiveChart);
                    }}
                    class="px-2.5 py-1 text-[11px] font-medium cursor-pointer {aiChartMode === modeBtn.id
                      ? 'bg-blue-600 dark:bg-cyan-600 text-white'
                      : 'bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
                  >
                    {$_(modeBtn.key)}
                  </button>
                {/each}
              </div>
            {/if}

            <button
              type="button"
              onclick={() => loadAllData(activeTab === "stl" || activeTab === "fft")}
              disabled={loadingLogs || loadingTimeAnalyze}
              class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-[11px] font-medium text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 disabled:opacity-50 cursor-pointer"
            >
              <RefreshCw
                class="h-3.5 w-3.5 {loadingLogs || loadingTimeAnalyze ? 'animate-spin' : ''}"
                aria-hidden="true"
              />
              <span>{$_("pollingDetail.reload")}</span>
            </button>
          </div>
        </div>
      {/if}

      <!-- Modal Tab Panels -->
      <div
        id={`polling-panel-${activeTab}`}
        role="tabpanel"
        aria-labelledby={`polling-tab-${activeTab}`}
        class="flex-1 overflow-y-auto p-5 space-y-4 text-xs min-h-0 bg-slate-50/70 dark:bg-slate-900/40 flex flex-col overscroll-contain"
      >
        {#if logsTruncated && activeTab !== "basic" && activeTab !== "ai"}
          <div class="flex items-center gap-2 px-3 py-2 rounded-xl border border-amber-300 dark:border-amber-700/60 bg-amber-50 dark:bg-amber-950/40 text-amber-800 dark:text-amber-200 text-[11px] shrink-0">
            <AlertTriangle class="h-4 w-4 shrink-0" aria-hidden="true" />
            <span>{$_("pollingDetail.truncatedNotice", { values: { count: logs.length } })}</span>
          </div>
        {/if}

        {#if logsError && activeTab !== "basic"}
          <div class="px-3 py-2 rounded-xl border border-rose-300 dark:border-rose-700/60 bg-rose-50 dark:bg-rose-950/40 text-rose-700 dark:text-rose-200 text-[11px] shrink-0">
            {logsError}
          </div>
        {/if}

        {#if activeTab === "basic"}
          <!-- 1. Status Overview Card -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
              <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
                {$_("nodeDetail.colLastVal")}
              </span>
              <span class="text-base font-bold font-mono text-blue-600 dark:text-cyan-400">
                {activePolling.last_val !== undefined && activePolling.last_val !== null ? activePolling.last_val : "-"}
              </span>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
              <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
                {$_("nodeDetail.colLastTime")}
              </span>
              <span class="text-xs font-mono text-slate-700 dark:text-slate-300">
                {activePolling.last_time ? formatTimeStr(activePolling.last_time) : "-"}
              </span>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
              <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
                {$_("pollingDetail.nextTime")}
              </span>
              <span class="text-xs font-mono text-slate-700 dark:text-slate-300">
                {activePolling.next_time ? formatTimeStr(activePolling.next_time) : "-"}
              </span>
            </div>
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
              <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
                {$_("pollingDetail.failTime")}
              </span>
              <span class="text-xs font-mono {activePolling.fail_time ? 'text-rose-500 font-semibold' : 'text-slate-400'}">
                {activePolling.fail_time ? formatTimeStr(activePolling.fail_time) : "-"}
              </span>
            </div>
          </div>

          <!-- 2. Parameters & Configuration Section -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden shrink-0">
            <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
              <Sliders class="h-4 w-4 text-blue-500" aria-hidden="true" />
              {$_("pollingDetail.params")}
            </div>
            <div class="p-4 grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <span class="text-slate-500 text-[11px] block mb-0.5">{$_("pollingDetail.paramsTarget")}</span>
                <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all select-all">
                  {activePolling.params || (activePolling as any).Params || activePolling.target || (activePolling as any).Target || "-"}
                </div>
              </div>
              <div>
                <span class="text-slate-500 text-[11px] block mb-0.5">{$_("pollingDetail.modeOption")}</span>
                <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                  {activePolling.mode || (activePolling as any).Mode || "-"}
                </div>
              </div>
              <div>
                <span class="text-slate-500 text-[11px] block mb-0.5">{$_("pollingDetail.filterCondition")}</span>
                <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                  {activePolling.filter || (activePolling as any).Filter || "-"}
                </div>
              </div>
              <div>
                <span class="text-slate-500 text-[11px] block mb-0.5">{$_("pollingDetail.extractorPattern")}</span>
                <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                  {activePolling.extractor || (activePolling as any).Extractor || "-"}
                </div>
              </div>
            </div>
          </div>

          <!-- 3. Evaluation Script Section -->
          {#if activePolling.script || (activePolling as any).Script}
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden shrink-0">
              <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
                <Terminal class="h-4 w-4 text-emerald-500" aria-hidden="true" />
                {$_("pollingDetail.script")}
              </div>
              <div class="p-3 bg-slate-950 text-slate-100 font-mono text-[11px] overflow-x-auto select-all rounded-b-xl leading-relaxed">
                <pre>{activePolling.script || (activePolling as any).Script}</pre>
              </div>
            </div>
          {/if}

          <!-- 4. Schedule & Execution Settings -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden shrink-0">
            <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
              <Clock class="h-4 w-4 text-indigo-500" aria-hidden="true" />
              {$_("pollingDetail.schedule")}
            </div>
            <div class="p-4 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
              <div>
                <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.pollInterval")}</span>
                <span class="font-mono font-semibold">{pollingInt} {$_("pollingDetail.unitSeconds")}</span>
              </div>
              <div>
                <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.timeout")}</span>
                <span class="font-mono font-semibold">{activePolling.timeout || (activePolling as any).Timeout || 1} {$_("pollingDetail.unitSeconds")}</span>
              </div>
              <div>
                <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.retryCount")}</span>
                <span class="font-mono font-semibold">{activePolling.retry || (activePolling as any).Retry || 1} {$_("pollingDetail.unitTimes")}</span>
              </div>
              <div>
                <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.logSaveMode")}</span>
                <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-semibold border {getLogModeBadgeClass(pollingLogMode)}">
                  {getLogModeName(pollingLogMode, $_)}
                </span>
              </div>
              {#if activePolling.fail_action || (activePolling as any).FailAction}
                <div class="col-span-2">
                  <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.failAction")}</span>
                  <span class="font-mono text-[11px] text-rose-600 dark:text-rose-400 break-all">{activePolling.fail_action || (activePolling as any).FailAction}</span>
                </div>
              {/if}
              {#if activePolling.repair_action || (activePolling as any).RepairAction}
                <div class="col-span-2">
                  <span class="text-slate-500 text-[10px] block">{$_("pollingDetail.repairAction")}</span>
                  <span class="font-mono text-[11px] text-emerald-600 dark:text-emerald-400 break-all">{activePolling.repair_action || (activePolling as any).RepairAction}</span>
                </div>
              {/if}
            </div>
          </div>

          <!-- 5. Latest Result Data Section -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden shrink-0">
            <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center justify-between">
              <div id="latest-result-heading" class="flex items-center gap-2">
                <Layers class="h-4 w-4 text-cyan-500" aria-hidden="true" />
                {$_("pollingDetail.latestResult")}
              </div>
              {#if activePolling.result && Object.keys(activePolling.result).length > 0}
                <button
                  type="button"
                  onclick={copyResultJson}
                  class="inline-flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 transition-colors cursor-pointer"
                >
                  {#if copied}
                    <Check class="h-3.5 w-3.5 text-emerald-500" aria-hidden="true" />
                    <span class="text-emerald-500">{$_("common.copied") || "Copied"}</span>
                  {:else}
                    <Copy class="h-3.5 w-3.5" aria-hidden="true" />
                    <span>{$_("pollingDetail.copyJson")}</span>
                  {/if}
                </button>
              {/if}
            </div>
            <div class="p-4">
              {#if activePolling.result && Object.keys(activePolling.result).length > 0}
                <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
                <div
                  tabindex="0"
                  role="region"
                  aria-labelledby="latest-result-heading"
                  class="overflow-x-auto max-h-56 focus-visible:outline-2 focus-visible:outline-blue-500 rounded-lg"
                >
                  <table class="w-full text-left text-xs border-collapse">
                    <thead class="bg-slate-50 dark:bg-slate-950/50 text-[10px] uppercase font-semibold text-slate-500 border-b border-slate-200 dark:border-slate-800">
                      <tr>
                        <th scope="col" class="py-1 px-3 w-1/3">{$_("pollingDetail.key")}</th>
                        <th scope="col" class="py-1 px-3">{$_("pollingDetail.value")}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-[11px]">
                      {#each Object.entries(activePolling.result) as [key, val]}
                        <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/20">
                          <td class="py-1.5 px-3 font-semibold text-slate-600 dark:text-slate-400 select-all">{key}</td>
                          <td class="py-1.5 px-3 text-slate-900 dark:text-slate-100 break-all select-all">
                            {typeof val === "object" ? JSON.stringify(val) : String(val)}
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {:else}
                <p class="text-slate-400 text-center py-3 text-xs font-sans">
                  {$_("pollingDetail.noResult")}
                </p>
              {/if}
            </div>
          </div>

        {:else if activeTab === "downtime"}
          {#if loadingLogs}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
              <span>{$_("pollingDetail.loadingLogs")}</span>
            </div>
          {:else if logs.length === 0}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p class="font-semibold text-slate-700 dark:text-slate-200">{$_("pollingDetail.noLogs")}</p>
              {#if pollingLogMode === 0}
                <p class="text-[11px]">{$_("pollingDetail.noLogsHint")}</p>
              {/if}
            </div>
          {:else}
            <!-- SLA Summary Cards -->
            <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 shrink-0">
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.sla")}</span>
                <span class="text-lg font-bold font-mono {downtimeResult.slaPercent >= 99.9 ? 'text-emerald-600 dark:text-emerald-400' : downtimeResult.slaPercent >= 99 ? 'text-amber-600 dark:text-amber-400' : 'text-rose-600 dark:text-rose-400'}">
                  {renderSLA(downtimeResult.slaPercent)}%
                </span>
              </div>
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.totalDowntime")}</span>
                <span class="text-base font-bold font-mono text-rose-600 dark:text-rose-400">
                  {formatDurationSec(downtimeResult.totalDowntimeSec)}
                </span>
              </div>
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.outageCount")}</span>
                <span class="text-base font-bold font-mono text-slate-800 dark:text-slate-100">
                  {downtimeResult.outageCount}
                </span>
              </div>
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.mtbf")}</span>
                <span class="text-base font-bold font-mono text-slate-800 dark:text-slate-100">
                  {formatDurationSec(downtimeResult.mtbfSec)}
                </span>
              </div>
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.mttr")}</span>
                <span class="text-base font-bold font-mono text-slate-800 dark:text-slate-100">
                  {formatDurationSec(downtimeResult.mttrSec)}
                </span>
              </div>
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
                <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase block">{$_("pollingDetail.monitoredSpan")}</span>
                <span class="text-base font-bold font-mono text-slate-800 dark:text-slate-100">
                  {formatDurationSec(downtimeResult.totalSpanSec)}
                </span>
              </div>
            </div>

            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm shrink-0">
              <div bind:this={chartEl} class="w-full h-[250px]"></div>
            </div>

            {#if sortedDowntimeEvents.length > 0}
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden">
                <div id="downtime-events-heading" class="px-4 py-2 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200">
                  {$_("pollingDetail.downtimeEvents")} ({sortedDowntimeEvents.length})
                </div>
                <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
                <div
                  tabindex="0"
                  role="region"
                  aria-labelledby="downtime-events-heading"
                  class="overflow-x-auto max-h-60 focus-visible:outline-2 focus-visible:outline-blue-500"
                >
                  <table class="w-full text-left text-xs border-collapse">
                    <thead class="bg-slate-50 dark:bg-slate-950/60 text-[10px] uppercase font-semibold text-slate-500 border-b border-slate-200 dark:border-slate-800 sticky top-0">
                      <tr>
                        <th scope="col" aria-sort={dtSortCol === "start" ? (dtSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3">
                          <button type="button" onclick={() => toggleDtSort("start")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                            <span>{$_("pollingDetail.startTime")}</span>
                            {#if dtSortCol === "start"}
                              {#if dtSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                            {:else}
                              <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                            {/if}
                          </button>
                        </th>
                        <th scope="col" class="py-2 px-3">{$_("pollingDetail.endTime")}</th>
                        <th scope="col" aria-sort={dtSortCol === "duration" ? (dtSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3">
                          <button type="button" onclick={() => toggleDtSort("duration")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                            <span>{$_("pollingDetail.duration")}</span>
                            {#if dtSortCol === "duration"}
                              {#if dtSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                            {:else}
                              <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                            {/if}
                          </button>
                        </th>
                        <th scope="col" aria-sort={dtSortCol === "state" ? (dtSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3">
                          <button type="button" onclick={() => toggleDtSort("state")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                            <span>{$_("pollingDetail.state")}</span>
                            {#if dtSortCol === "state"}
                              {#if dtSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                            {:else}
                              <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                            {/if}
                          </button>
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-[11px]">
                      {#each sortedDowntimeEvents as ev}
                        <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/30">
                          <td class="py-1.5 px-3 whitespace-nowrap">{formatTimeStr(ev.startNs)}</td>
                          <td class="py-1.5 px-3 whitespace-nowrap">{formatTimeStr(ev.endNs)}</td>
                          <td class="py-1.5 px-3 whitespace-nowrap font-semibold text-rose-600 dark:text-rose-400">{formatDurationSec(ev.durationSec)}</td>
                          <td class="py-1.5 px-3 whitespace-nowrap">
                            <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadgeClass(ev.state)}">
                              {getStateLabel(ev.state)}
                            </span>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              </div>
            {/if}
          {/if}

        {:else if activeTab === "log"}
          {#if loadingLogs}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
              <span>{$_("pollingDetail.loadingLogs")}</span>
            </div>
          {:else if logs.length === 0}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p class="font-semibold text-slate-700 dark:text-slate-200">{$_("pollingDetail.noLogs")}</p>
              {#if pollingLogMode === 0}
                <p class="text-[11px]">{$_("pollingDetail.noLogsHint")}</p>
              {/if}
            </div>
          {:else}
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm shrink-0">
              <div bind:this={chartEl} class="w-full h-[220px]"></div>
            </div>

            <div class="flex flex-wrap items-center justify-between gap-2 shrink-0">
              <div class="flex items-center gap-2 flex-1 min-w-[200px] max-w-sm">
                <div class="relative flex-1">
                  <Search class="h-3.5 w-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2 pointer-events-none" aria-hidden="true" />
                  <input
                    type="text"
                    bind:value={logFilter}
                    oninput={() => (logPage = 1)}
                    placeholder={$_("pollingDetail.searchLogs")}
                    class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200"
                  />
                </div>
                {#if zoomRange[0] > 0 && zoomRange[1] > 0}
                  <button
                    type="button"
                    onclick={() => {
                      zoomRange = [0, 0];
                      logPage = 1;
                    }}
                    class="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-cyan-500/40 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300 text-[11px] font-medium cursor-pointer whitespace-nowrap"
                  >
                    <RotateCcw class="h-3 w-3" aria-hidden="true" />
                    <span>{$_("pollingDetail.resetZoom")}</span>
                  </button>
                {/if}
              </div>

              <div class="flex items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
                {#if zoomRange[0] > 0 && zoomRange[1] > 0}
                  <span class="text-cyan-600 dark:text-cyan-400 font-medium">{$_("pollingDetail.zoomFiltered")}</span>
                {/if}
                <span class="font-mono">{$_("pollingDetail.recordsCount", { values: { count: filteredLogs.length } })}</span>
                {#if logTotalPages > 1}
                  <div class="inline-flex items-center gap-1">
                    <button
                      type="button"
                      disabled={logPage <= 1}
                      onclick={() => (logPage = Math.max(1, logPage - 1))}
                      class="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 disabled:opacity-40 cursor-pointer"
                    >
                      &lsaquo;
                    </button>
                    <span class="font-mono">{logPage} / {logTotalPages}</span>
                    <button
                      type="button"
                      disabled={logPage >= logTotalPages}
                      onclick={() => (logPage = Math.min(logTotalPages, logPage + 1))}
                      class="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 disabled:opacity-40 cursor-pointer"
                    >
                      &rsaquo;
                    </button>
                  </div>
                {/if}
              </div>
            </div>

            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden flex-1 min-h-[180px] flex flex-col">
              <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
              <div
                tabindex="0"
                role="region"
                aria-label={$_("pollingDetail.tabLog")}
                class="overflow-x-auto overflow-y-auto flex-1 focus-visible:outline-2 focus-visible:outline-blue-500"
              >
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="bg-slate-50 dark:bg-slate-950/70 text-[10px] uppercase font-semibold text-slate-500 border-b border-slate-200 dark:border-slate-800 sticky top-0 z-10">
                    <tr>
                      <th scope="col" aria-sort={logSortCol === "time" ? (logSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3 whitespace-nowrap">
                        <button type="button" onclick={() => toggleLogSort("time")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                          <span>{$_("pollingDetail.time")}</span>
                          {#if logSortCol === "time"}
                            {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                          {/if}
                        </button>
                      </th>
                      <th scope="col" aria-sort={logSortCol === "state" ? (logSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3 whitespace-nowrap">
                        <button type="button" onclick={() => toggleLogSort("state")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                          <span>{$_("pollingDetail.state")}</span>
                          {#if logSortCol === "state"}
                            {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                          {/if}
                        </button>
                      </th>
                      <th scope="col" aria-sort={logSortCol === "result" ? (logSortDir === "asc" ? "ascending" : "descending") : undefined} class="py-2 px-3">
                        <button type="button" onclick={() => toggleLogSort("result")} class="inline-flex items-center gap-1 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer">
                          <span>{$_("pollingDetail.result")}</span>
                          {#if logSortCol === "result"}
                            {#if logSortDir === "asc"}<ArrowUp class="h-3 w-3" aria-hidden="true" />{:else}<ArrowDown class="h-3 w-3" aria-hidden="true" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 opacity-50" aria-hidden="true" />
                          {/if}
                        </button>
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-[11px]">
                    {#each paginatedLogs as row}
                      <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/30">
                        <td class="py-1.5 px-3 whitespace-nowrap text-slate-700 dark:text-slate-300">{formatTimeStr(row.Time)}</td>
                        <td class="py-1.5 px-3 whitespace-nowrap">
                          <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadgeClass(row.State)}">
                            {getStateLabel(row.State)}
                          </span>
                        </td>
                        <td class="py-1.5 px-3 text-slate-800 dark:text-slate-200 break-all">{formatLogResultSummary(row.Result)}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          {/if}

        {:else if activeTab === "time" || activeTab === "histogram" || activeTab === "qqplot"}
          {#if loadingLogs}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
              <span>{$_("pollingDetail.loadingLogs")}</span>
            </div>
          {:else if logs.length === 0}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p class="font-semibold text-slate-700 dark:text-slate-200">{$_("pollingDetail.noLogs")}</p>
            </div>
          {:else if !selectedMetric}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400">
              {$_("pollingDetail.noNumericData")}
            </div>
          {:else}
            {#if activeTab === "histogram" && histStats && histStats.count > 0}
              <div class="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-2 shrink-0">
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.sampleCount")}</span>
                  <span class="font-mono font-bold text-slate-800 dark:text-slate-100">{histStats.count}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statMean")}</span>
                  <span class="font-mono font-bold text-cyan-600 dark:text-cyan-400">{formatStatValue(histStats.mean, selectedMetric)}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statMedian")}</span>
                  <span class="font-mono font-bold text-slate-800 dark:text-slate-100">{formatStatValue(histStats.median, selectedMetric)}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statMin")}</span>
                  <span class="font-mono font-bold text-emerald-600 dark:text-emerald-400">{formatStatValue(histStats.min, selectedMetric)}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statMax")}</span>
                  <span class="font-mono font-bold text-rose-600 dark:text-rose-400">{formatStatValue(histStats.max, selectedMetric)}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statStddev")}</span>
                  <span class="font-mono font-bold text-slate-800 dark:text-slate-100">{formatStatValue(histStats.stddev, selectedMetric)}</span>
                </div>
                <div class="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 px-3 py-2">
                  <span class="text-[10px] text-slate-500 block">{$_("pollingDetail.statVariance")}</span>
                  <span class="font-mono font-bold text-slate-800 dark:text-slate-100">{histStats.variance.toFixed(3)}</span>
                </div>
              </div>
            {/if}
            <div class="flex-1 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm min-h-[340px] flex flex-col">
              <div bind:this={chartEl} class="w-full flex-1 min-h-[320px]"></div>
            </div>
          {/if}

        {:else if activeTab === "stl"}
          {#if loadingTimeAnalyze}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
              <span>{$_("pollingDetail.loadingSTL")}</span>
            </div>
          {:else if !hasCurrentStl}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p>{$_("pollingDetail.noSTLData")}</p>
            </div>
          {:else}
            <div class="flex-1 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm min-h-[420px] flex flex-col">
              <div bind:this={chartEl} class="w-full flex-1 min-h-[400px]"></div>
            </div>
          {/if}

        {:else if activeTab === "fft"}
          {#if loadingTimeAnalyze}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
              <span>{$_("pollingDetail.loadingSTL")}</span>
            </div>
          {:else if !hasCurrentFft}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p>{$_("pollingDetail.noSTLData")}</p>
            </div>
          {:else}
            <div class="flex-1 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm min-h-[360px] flex flex-col">
              <div bind:this={chartEl} class="w-full flex-1 min-h-[340px]"></div>
            </div>
          {/if}

        {:else if activeTab === "ai"}
          {#if loadingAI}
            <div class="flex-1 flex items-center justify-center text-slate-500 dark:text-slate-400 gap-2">
              <RefreshCw class="h-4 w-4 animate-spin" aria-hidden="true" />
            </div>
          {:else if !aiResult?.ScoreData || aiResult.ScoreData.length === 0}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-8 text-slate-500 dark:text-slate-400 gap-2">
              <Info class="h-8 w-8 text-slate-400" aria-hidden="true" />
              <p>{$_("pollingDetail.noAIData")}</p>
            </div>
          {:else}
            <div class="flex-1 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm min-h-[360px] flex flex-col">
              <div bind:this={chartEl} class="w-full flex-1 min-h-[340px]"></div>
            </div>
          {/if}
        {/if}
      </div>

      <!-- Modal Footer -->
      <div
        class="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-5 py-2.5 shrink-0"
      >
        <div class="flex flex-wrap items-center gap-2">
          {#if onEdit && !isReadonly}
            <button
              type="button"
              onclick={() => {
                const p = activePolling;
                show = false;
                if (p) onEdit?.(p);
              }}
              class="inline-flex items-center gap-1.5 rounded-xl border border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-cyan-400 hover:bg-blue-500/20 px-3.5 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
            >
              <Edit class="h-3.5 w-3.5" aria-hidden="true" />
              <span>{$_("common.edit")}</span>
            </button>
          {/if}

          <button
            type="button"
            onclick={() => (showAiExplainModal = true)}
            class="inline-flex items-center gap-1.5 rounded-xl border border-indigo-500/30 bg-indigo-500/10 text-indigo-600 dark:text-indigo-300 hover:bg-indigo-500/20 px-3.5 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
          >
            <Sparkles class="h-3.5 w-3.5" aria-hidden="true" />
            <span>{$_("pollingDetail.aiExplain")}</span>
          </button>

          {#if (polling?.LogMode ?? 0) !== 0}
            <button
              type="button"
              onclick={() => (showCorrelationModal = true)}
              class="inline-flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 text-cyan-700 dark:text-cyan-300 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
            >
              <GitCompareArrows class="h-3.5 w-3.5" aria-hidden="true" />
              <span>{$_("Correlation.findCorrelationBtn")}</span>
            </button>
          {/if}

          {#if logs.length > 0}
            <button
              type="button"
              onclick={exportLogsCsv}
              class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 px-3 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
            >
              <Download class="h-3.5 w-3.5" aria-hidden="true" />
              <span>{$_("pollingDetail.csvExport")}</span>
            </button>
            {#if metricKeys.length > 0}
              <button
                type="button"
                onclick={exportNumericDataCsv}
                class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 px-3 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
              >
                <Download class="h-3.5 w-3.5" aria-hidden="true" />
                <span>{$_("pollingDetail.csvDataExport")}</span>
              </button>
            {/if}
          {/if}

          {#if !isReadonly && (logs.length > 0 || aiResult?.ScoreData?.length)}
            <button
              type="button"
              data-testid="pollingdetail-clear-logs"
              onclick={handleClearLogs}
              class="inline-flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400 hover:bg-rose-500/20 px-3.5 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
            >
              <Trash2 class="h-3.5 w-3.5" aria-hidden="true" />
              <span>{$_("pollingDetail.clearLogs")}</span>
            </button>
          {/if}
        </div>

        <button
          type="button"
          onclick={() => (show = false)}
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 shadow-sm hover:bg-slate-100 dark:hover:bg-slate-700 hover:text-slate-900 dark:hover:text-white transition-all cursor-pointer"
        >
          <X class="h-4 w-4" aria-hidden="true" />
          <span>{$_("common.close")}</span>
        </button>
      </div>
    </div>
  </div>

  <ReportAIExplainModal
    bind:show={showAiExplainModal}
    reportType="polling_detail"
    title={pollingName}
    summary={aiSummaryText}
    items={aiItemsList}
    extraData={{
      polling_id: pollingId,
      start_time: appliedStartNs,
      end_time: appliedEndNs,
    }}
    zIndexClass="z-[80]"
  />

  <CorrelationModal
    bind:show={showCorrelationModal}
    initialTab="discover"
    initialBasePollingID={pollingId}
  />
{/if}
