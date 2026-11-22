<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import type * as echarts from "echarts";
  import { X, BarChart3, PieChart, Activity } from "@lucide/svelte";
  import { fetchAIResult, type AIResultEnt } from "../../api";
  import { renderAIHeatmap, renderAIPieChart, renderAITimeChart } from "../../charts/ai";

  let {
    show = $bindable(false),
    id = "",
    title = "",
  }: {
    show: boolean;
    id: string;
    title?: string;
  } = $props();

  let activeTab = $state<"heatmap" | "pie" | "time">("heatmap");
  let chartDom: HTMLDivElement | null = $state(null);
  let chartInstance: echarts.ECharts | null = null;
  let loading = $state(false);
  let aiResult: AIResultEnt | null = $state(null);

  const loadResultAndRender = async () => {
    if (!id || !show) return;
    loading = true;
    try {
      aiResult = await fetchAIResult(id);
      await tick();
      renderCurrentChart();
    } catch (e) {
      console.error("Failed to load AI result", e);
    } finally {
      loading = false;
    }
  };

  $effect(() => {
    if (show && id) {
      loadResultAndRender();
    } else {
      if (chartInstance) {
        chartInstance.dispose();
        chartInstance = null;
      }
    }
  });

  const setTab = async (tab: "heatmap" | "pie" | "time") => {
    activeTab = tab;
    await tick();
    renderCurrentChart();
  };

  const renderCurrentChart = () => {
    if (!chartDom) return;
    if (chartInstance) {
      chartInstance.dispose();
      chartInstance = null;
    }

    const scores = aiResult?.ScoreData || [];
    if (activeTab === "heatmap") {
      chartInstance =
        renderAIHeatmap(chartDom, scores, {
          heatmapDate: $_("AIReport.HeatmapDate") || "Date",
          heatmapHour: $_("AIReport.HeatmapHour") || "Hour",
        }) || null;
    } else if (activeTab === "pie") {
      chartInstance =
        renderAIPieChart(chartDom, scores, {
          anomalyScore: $_("AIList.AnomaryScore") || "Anomaly Score",
          normal: $_("common.normal") || "Normal",
          warn: $_("common.warn") || "Warning",
          anomaly: $_("common.high") || "Anomaly",
        }) || null;
    } else if (activeTab === "time") {
      chartInstance =
        renderAITimeChart(chartDom, scores, {
          anomalyScore: $_("AIList.AnomaryScore") || "Anomaly Score",
          normal: $_("common.normal") || "Normal",
          anomaly: $_("common.high") || "Anomaly",
        }) || null;
    }
  };

  const handleResize = () => {
    chartInstance?.resize();
  };

  onMount(() => {
    window.addEventListener("resize", handleResize);
  });

  onDestroy(() => {
    window.removeEventListener("resize", handleResize);
    chartInstance?.dispose();
  });
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-in fade-in duration-200">
    <div class="flex flex-col w-full max-w-5xl h-[85vh] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden">
      <!-- Modal Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950">
        <div class="flex items-center gap-3">
          <span class="mdi mdi-chart-bell-curve-cumulative text-cyan-600 dark:text-cyan-400 text-xl"></span>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-slate-100">
              {$_("AIReport.Report") || "AI Anomaly Detection Report"}
              {#if title}
                <span class="text-xs font-normal text-slate-500 dark:text-slate-400 ml-2">({title})</span>
              {/if}
            </h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              Polling ID: <span class="font-mono text-cyan-600 dark:text-cyan-400">{id}</span>
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Tab Switcher -->
      <div class="flex items-center gap-2 px-6 pt-3 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
        <button
          type="button"
          onclick={() => setTab("heatmap")}
          class="flex items-center gap-2 px-4 py-2 border-b-2 text-xs font-semibold transition-colors {activeTab === 'heatmap' ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400' : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
        >
          <BarChart3 class="w-4 h-4" />
          {$_("AIReport.Heatmap")}
        </button>
        <button
          type="button"
          onclick={() => setTab("pie")}
          class="flex items-center gap-2 px-4 py-2 border-b-2 text-xs font-semibold transition-colors {activeTab === 'pie' ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400' : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
        >
          <PieChart class="w-4 h-4" />
          {$_("AIReport.PieChart")}
        </button>
        <button
          type="button"
          onclick={() => setTab("time")}
          class="flex items-center gap-2 px-4 py-2 border-b-2 text-xs font-semibold transition-colors {activeTab === 'time' ? 'border-cyan-500 text-cyan-600 dark:text-cyan-400' : 'border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200'}"
        >
          <Activity class="w-4 h-4" />
          {$_("AIReport.TimeChart")}
        </button>
      </div>

      <!-- Chart Canvas Body -->
      <div class="flex-1 p-4 relative min-h-0">
        {#if loading}
          <div class="absolute inset-0 flex items-center justify-center bg-white/50 dark:bg-slate-900/50 backdrop-blur-xs z-10">
            <span class="mdi mdi-loading mdi-spin text-3xl text-cyan-500"></span>
          </div>
        {/if}
        <div bind:this={chartDom} class="w-full h-full min-h-[400px]"></div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 rounded-xl text-xs font-medium border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors"
        >
          {$_("AIReport.Close")}
        </button>
      </div>
    </div>
  </div>
{/if}
