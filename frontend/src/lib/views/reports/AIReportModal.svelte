<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import { X, BarChart3, PieChart, Activity } from "@lucide/svelte";
  import { fetchAIResult, type AIResultEnt } from "../../api";

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

  const isDarkMode = () => document.documentElement.classList.contains("dark");

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
    const theme = isDarkMode() ? "dark" : undefined;
    chartInstance = echarts.init(chartDom, theme);

    const scores = aiResult?.ScoreData || [];
    const textColor = isDarkMode() ? "#cbd5e1" : "#475569";
    const lineColor = isDarkMode() ? "#334155" : "#e2e8f0";

    if (activeTab === "heatmap") {
      const hours = Array.from({ length: 24 }, (_, i) => String(i));
      const option: any = {
        backgroundColor: "transparent",
        title: { show: false },
        grid: {
          left: "8%",
          right: "5%",
          top: 30,
          bottom: 70,
        },
        toolbox: {
          iconStyle: { color: textColor },
          feature: { dataZoom: {} },
        },
        dataZoom: [{ bottom: 15, height: 18 }],
        tooltip: {
          trigger: "item",
          formatter(params: any) {
            return `${params.name} ${params.data[1]}:00 : ${Number(params.data[2]).toFixed(2)}`;
          },
        },
        xAxis: {
          type: "category",
          name: $_("AIReport.HeatmapDate") || "Date",
          nameTextStyle: { color: textColor, fontSize: 11 },
          axisLabel: { color: textColor, fontSize: 10 },
          axisLine: { lineStyle: { color: lineColor } },
          data: [],
        },
        yAxis: {
          type: "category",
          name: $_("AIReport.HeatmapHour") || "Hour",
          nameTextStyle: { color: textColor, fontSize: 11 },
          axisLabel: { color: textColor, fontSize: 10 },
          axisLine: { lineStyle: { color: lineColor } },
          data: hours,
        },
        visualMap: {
          min: 40,
          max: 80,
          textStyle: { color: textColor, fontSize: 10 },
          calculable: true,
          realtime: false,
          inRange: {
            color: [
              "#313695",
              "#4575b4",
              "#74add1",
              "#abd9e9",
              "#e0f3f8",
              "#ffffbf",
              "#fee090",
              "#fdae61",
              "#f46d43",
              "#d73027",
              "#a50026",
            ],
          },
        },
        series: [
          {
            name: "Score",
            type: "heatmap",
            data: [],
            emphasis: {
              itemStyle: {
                borderColor: "#fff",
                borderWidth: 1,
              },
            },
          },
        ],
      };

      let nD = 0;
      let x = -1;
      scores.forEach((e: any) => {
        const t = new Date(e[0] * 1000);
        if (nD !== t.getDate()) {
          const dStr = `${t.getFullYear()}/${String(t.getMonth() + 1).padStart(2, "0")}/${String(t.getDate()).padStart(2, "0")}`;
          option.xAxis.data.push(dStr);
          nD = t.getDate();
          x++;
        }
        option.series[0].data.push([x, t.getHours(), e[1]]);
      });

      chartInstance.setOption(option);
    } else if (activeTab === "pie") {
      let normal = 0,
        warn = 0,
        anomaly = 0;
      scores.forEach((e: any) => {
        const s = e[1];
        if (s > 66.0) anomaly++;
        else if (s > 50.0) warn++;
        else normal++;
      });

      const option: any = {
        backgroundColor: "transparent",
        title: { show: false },
        color: ["#10b981", "#f59e0b", "#ef4444"],
        tooltip: {
          trigger: "item",
          formatter: "{a} <br/>{b} : {c} ({d}%)",
        },
        legend: {
          top: 20,
          textStyle: { color: textColor, fontSize: 12 },
          data: [
            $_("common.normal") || "Normal",
            $_("common.warn") || "Warning",
            $_("common.high") || "Anomaly",
          ],
        },
        series: [
          {
            name: $_("AIList.AnomaryScore") || "Anomaly Score",
            type: "pie",
            radius: ["40%", "70%"],
            center: ["50%", "55%"],
            label: { color: textColor, fontSize: 11 },
            data: [
              { name: $_("common.normal") || "Normal", value: normal },
              { name: $_("common.warn") || "Warning", value: warn },
              { name: $_("common.high") || "Anomaly", value: anomaly },
            ],
          },
        ],
      };
      chartInstance.setOption(option);
    } else if (activeTab === "time") {
      const option: any = {
        backgroundColor: "transparent",
        title: { show: false },
        grid: {
          left: "8%",
          right: "5%",
          top: 30,
          bottom: 70,
        },
        toolbox: {
          iconStyle: { color: textColor },
          feature: { dataZoom: {} },
        },
        dataZoom: [{ bottom: 15, height: 18 }],
        tooltip: {
          trigger: "axis",
          axisPointer: { type: "line" },
        },
        xAxis: {
          type: "time",
          nameTextStyle: { color: textColor },
          axisLabel: { color: textColor, fontSize: 10 },
          axisLine: { lineStyle: { color: lineColor } },
        },
        yAxis: {
          type: "value",
          name: $_("AIList.AnomaryScore") || "Anomaly Score",
          nameTextStyle: { color: textColor, fontSize: 11 },
          axisLabel: { color: textColor, fontSize: 10 },
          axisLine: { lineStyle: { color: lineColor } },
          splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
        },
        series: [
          {
            name: $_("AIList.AnomaryScore") || "Anomaly Score",
            type: "line",
            smooth: true,
            color: "#38bdf8",
            showSymbol: false,
            data: scores.map((e: any) => [new Date(e[0] * 1000), e[1]]),
            markLine: {
              silent: true,
              data: [
                { yAxis: 50, lineStyle: { color: "#10b981", type: "dashed" }, label: { formatter: "50 (Normal)" } },
                { yAxis: 66, lineStyle: { color: "#ef4444", type: "dashed" }, label: { formatter: "66 (Anomaly)" } },
              ],
            },
          },
        ],
      };
      chartInstance.setOption(option);
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
