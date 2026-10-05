<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Radio,
    Eye,
    Trash2,
    Play,
    Square,
    RefreshCw,
    GitFork,
    X,
    TrendingUp,
    Cpu,
    Network,
    Activity,
    AlertCircle,
    CheckCircle2,
    AlertTriangle,
    Shield,
    Layers,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    BarChart3,
  } from "@lucide/svelte";
  import {
    fetchSensors,
    fetchSensorStats,
    fetchSensorMonitors,
    deleteSensor,
    toggleSensor,
    type SensorEnt,
    type SensorStatsEnt,
    type SensorMonitorEnt,
    type NodeEnt,
  } from "../../api";
  import { getStateColor, getStateName, formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import ReportPagination from "./components/ReportPagination.svelte";

  let {
    searchQuery = "",
    nodes = [],
    onRefresh = () => {},
    loading: parentLoading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  let sensors = $state<SensorEnt[]>([]);
  let loading = $state(false);
  let filterHost = $state("");
  let filterType = $state("");
  let filterState = $state<string>("all");

  // Sorting and Pagination
  let sortColumn = $state<string>("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state<number>(25);
  let currentPage = $state<number>(1);

  // Selected sensor for details & delete
  let selected = $state<SensorEnt | null>(null);

  // Modals
  let showInfoModal = $state(false);
  let showDeleteModal = $state(false);
  let selectedTreeType = $state<string>("");

  // Chart modals
  let showStatsChartModal = $state(false);
  let showCpuMemChartModal = $state(false);
  let showNetChartModal = $state(false);
  let showProcChartModal = $state(false);

  // Chart data
  let currentStats = $state<SensorStatsEnt[]>([]);
  let currentMonitors = $state<SensorMonitorEnt[]>([]);

  // DOM elements for ECharts
  let statsChartElem = $state<HTMLDivElement | null>(null);
  let cpuMemChartElem = $state<HTMLDivElement | null>(null);
  let netChartElem = $state<HTMLDivElement | null>(null);
  let procChartElem = $state<HTMLDivElement | null>(null);
  let treeChartElem = $state<HTMLDivElement | null>(null);

  let statsChartInstance: echarts.ECharts | null = null;
  let cpuMemChartInstance: echarts.ECharts | null = null;
  let netChartInstance: echarts.ECharts | null = null;
  let procChartInstance: echarts.ECharts | null = null;
  let treeChartInstance: echarts.ECharts | null = null;

  // Notification alerts
  let alertMessage = $state<{ type: "success" | "error"; text: string } | null>(null);

  const showAlert = (type: "success" | "error", text: string) => {
    alertMessage = { type, text };
    setTimeout(() => {
      if (alertMessage?.text === text) {
        alertMessage = null;
      }
    }, 4000);
  };

  export const refresh = async () => {
    loading = true;
    try {
      const data = await fetchSensors();
      sensors = data;
      if (!selectedTreeType && sensors.length > 0) {
        const hasSyslog = sensors.some((s) => s.Type === "syslog");
        selectedTreeType = hasSyslog ? "syslog" : sensors[0].Type;
      }
    } catch (err: any) {
      console.error("Failed to load sensors:", err);
      showAlert("error", $_("report.sensorFetchFailed", { values: { error: err.message || err } }));
    } finally {
      loading = false;
    }
  };

  onMount(() => {
    refresh();
    const handleResize = () => {
      statsChartInstance?.resize();
      cpuMemChartInstance?.resize();
      netChartInstance?.resize();
      procChartInstance?.resize();
      treeChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      statsChartInstance?.dispose();
      cpuMemChartInstance?.dispose();
      netChartInstance?.dispose();
      procChartInstance?.dispose();
      treeChartInstance?.dispose();
    };
  });

  const uniqueHosts = $derived.by(() => {
    const set = new Set<string>();
    for (const s of sensors) {
      if (s.Host) set.add(s.Host);
    }
    return Array.from(set).sort();
  });

  const uniqueTypes = $derived.by(() => {
    const set = new Set<string>();
    for (const s of sensors) {
      if (s.Type) set.add(s.Type);
    }
    return Array.from(set).sort();
  });

  const kpiCounts = $derived.by(() => {
    let normal = 0;
    let high = 0;
    let low = 0;
    let warn = 0;
    let off = 0;
    let totalMessages = 0;
    let totalSend = 0;

    for (const s of sensors) {
      if (s.Ignore || s.State === "off") {
        off++;
      } else if (s.State === "high") {
        high++;
      } else if (s.State === "low") {
        low++;
      } else if (s.State === "warn") {
        warn++;
      } else {
        normal++;
      }
      totalMessages += s.Total || 0;
      totalSend += s.Send || 0;
    }
    return {
      normal,
      high,
      low,
      warn,
      problem: high + low + warn,
      off,
      totalMessages,
      totalSend,
    };
  });

  const filteredSensors = $derived.by(() => {
    let list = [...sensors];

    // Global query
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      list = list.filter(
        (s) =>
          s.Host?.toLowerCase().includes(q) ||
          s.Type?.toLowerCase().includes(q) ||
          s.Param?.toLowerCase().includes(q) ||
          s.State?.toLowerCase().includes(q)
      );
    }

    // Host filter dropdown
    if (filterHost.trim()) {
      list = list.filter((s) => s.Host === filterHost);
    }

    // Type filter dropdown
    if (filterType.trim()) {
      list = list.filter((s) => s.Type === filterType);
    }

    // State subfilter
    if (filterState === "normal") {
      list = list.filter((s) => s.State === "normal" && !s.Ignore);
    } else if (filterState === "problem") {
      list = list.filter((s) => (s.State === "high" || s.State === "low" || s.State === "warn") && !s.Ignore);
    } else if (filterState === "high") {
      list = list.filter((s) => s.State === "high" && !s.Ignore);
    } else if (filterState === "low") {
      list = list.filter((s) => s.State === "low" && !s.Ignore);
    } else if (filterState === "warn") {
      list = list.filter((s) => s.State === "warn" && !s.Ignore);
    } else if (filterState === "off") {
      list = list.filter((s) => s.State === "off" || s.Ignore);
    }

    // Sort
    list.sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];

      if (typeof valA === "string") {
        const cmp = valA.localeCompare(valB || "");
        return sortDirection === "asc" ? cmp : -cmp;
      }
      valA = Number(valA || 0);
      valB = Number(valB || 0);
      return sortDirection === "asc" ? valA - valB : valB - valA;
    });

    return list;
  });

  const paginatedSensors = $derived.by(() => {
    if (pageSize === -1) return filteredSensors;
    const start = (currentPage - 1) * pageSize;
    return filteredSensors.slice(start, start + pageSize);
  });

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const openInfo = (item: SensorEnt) => {
    selected = item;
    showInfoModal = true;
  };

  const openDelete = (item: SensorEnt) => {
    selected = item;
    showDeleteModal = true;
  };

  const handleConfirmDelete = async () => {
    if (!selected) return;
    try {
      await deleteSensor(selected.ID);
      showDeleteModal = false;
      showAlert("success", $_("report.sensorDeletedSuccess"));
      await refresh();
      onRefresh();
    } catch (err: any) {
      showAlert("error", $_("report.sensorDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  const handleToggle = async (item: SensorEnt) => {
    try {
      await toggleSensor(item.ID);
      await refresh();
      onRefresh();
    } catch (err: any) {
      showAlert("error", $_("report.sensorDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  // Open Charts
  const openStatsChart = async () => {
    if (!selected) return;
    try {
      currentStats = await fetchSensorStats(selected.ID);
      showStatsChartModal = true;
      await tick();
      renderStatsChart();
    } catch (err: any) {
      showAlert("error", $_("report.sensorStatsFetchFailed", { values: { error: err.message || err } }));
    }
  };

  const openCpuMemChart = async () => {
    if (!selected) return;
    try {
      currentMonitors = await fetchSensorMonitors(selected.ID);
      showCpuMemChartModal = true;
      await tick();
      renderCpuMemChart();
    } catch (err: any) {
      showAlert("error", $_("report.sensorMonitorsFetchFailed", { values: { error: err.message || err } }));
    }
  };

  const openNetChart = async () => {
    if (!selected) return;
    try {
      currentMonitors = await fetchSensorMonitors(selected.ID);
      showNetChartModal = true;
      await tick();
      renderNetChart();
    } catch (err: any) {
      showAlert("error", $_("report.sensorMonitorsFetchFailed", { values: { error: err.message || err } }));
    }
  };

  const openProcChart = async () => {
    if (!selected) return;
    try {
      currentMonitors = await fetchSensorMonitors(selected.ID);
      showProcChartModal = true;
      await tick();
      renderProcChart();
    } catch (err: any) {
      showAlert("error", $_("report.sensorMonitorsFetchFailed", { values: { error: err.message || err } }));
    }
  };

  // Render Stats Chart (PS Line + Count Bar)
  const renderStatsChart = () => {
    if (!statsChartElem) return;
    if (statsChartInstance) statsChartInstance.dispose();
    statsChartInstance = echarts.init(statsChartElem);

    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const psData: [Date, number][] = [];
    const countData: [Date, number][] = [];

    currentStats.forEach((s) => {
      const t = new Date(s.Time / 1e6);
      psData.push([t, Number(s.PS.toFixed(2))]);
      countData.push([t, s.Count]);
    });

    statsChartInstance.setOption({
      title: {
        text: selected ? `${selected.Host} (${selected.Type}) ${$_("report.sensorStatsChart")}` : $_("report.sensorStatsChart"),
        left: "center",
        textStyle: { color: textColor, fontSize: 14 },
      },
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
      },
      legend: {
        data: ["PS (rate/sec)", "Count"],
        top: 30,
        textStyle: { color: textColor },
      },
      grid: { left: "5%", right: "5%", top: 70, bottom: 40, containLabel: true },
      dataZoom: [{ type: "slider", bottom: 5 }, { type: "inside" }],
      toolbox: {
        feature: { saveAsImage: { name: "sensor_stats" } },
        iconStyle: { borderColor: textColor },
      },
      xAxis: {
        type: "time",
        axisLabel: {
          color: textColor,
          formatter: (val: number) => echarts.time.format(val, "{yyyy}/{MM}/{dd} {HH}:{mm}", false),
        },
        axisLine: { lineStyle: { color: splitLineColor } },
        splitLine: { show: false },
      },
      yAxis: [
        {
          type: "value",
          name: "PS",
          nameTextStyle: { color: textColor },
          axisLabel: { color: textColor },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        {
          type: "value",
          name: "Count",
          nameTextStyle: { color: textColor },
          axisLabel: { color: textColor },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: "PS (rate/sec)",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: psData,
          itemStyle: { color: "#3b82f6" },
        },
        {
          name: "Count",
          type: "bar",
          yAxisIndex: 1,
          data: countData,
          itemStyle: { color: "rgba(16, 185, 129, 0.7)" },
        },
      ],
    });
  };

  // Render CPU / Mem Chart
  const renderCpuMemChart = () => {
    if (!cpuMemChartElem) return;
    if (cpuMemChartInstance) cpuMemChartInstance.dispose();
    cpuMemChartInstance = echarts.init(cpuMemChartElem);

    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const cpuData: [Date, number][] = [];
    const memData: [Date, number][] = [];

    currentMonitors.forEach((m) => {
      const t = new Date(m.Time / 1e6);
      cpuData.push([t, Number(m.CPU.toFixed(2))]);
      memData.push([t, Number(m.Mem.toFixed(2))]);
    });

    cpuMemChartInstance.setOption({
      title: {
        text: selected ? `${selected.Host} CPU / Memory` : "CPU / Memory",
        left: "center",
        textStyle: { color: textColor, fontSize: 14 },
      },
      tooltip: {
        trigger: "axis",
        formatter: (params: any) => {
          if (!params || params.length === 0) return "";
          const t = echarts.time.format(params[0].value[0], "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}", false);
          let res = `<b>${t}</b><br/>`;
          params.forEach((p: any) => {
            res += `${p.marker} ${p.seriesName}: ${p.value[1]} %<br/>`;
          });
          return res;
        },
      },
      legend: {
        data: ["CPU (%)", "Memory (%)"],
        top: 30,
        textStyle: { color: textColor },
      },
      grid: { left: "5%", right: "5%", top: 70, bottom: 40, containLabel: true },
      dataZoom: [{ type: "slider", bottom: 5 }, { type: "inside" }],
      toolbox: {
        feature: { saveAsImage: { name: "sensor_cpu_mem" } },
        iconStyle: { borderColor: textColor },
      },
      xAxis: {
        type: "time",
        axisLabel: {
          color: textColor,
          formatter: (val: number) => echarts.time.format(val, "{yyyy}/{MM}/{dd} {HH}:{mm}", false),
        },
        axisLine: { lineStyle: { color: splitLineColor } },
        splitLine: { show: false },
      },
      yAxis: {
        type: "value",
        name: "%",
        max: 100,
        min: 0,
        nameTextStyle: { color: textColor },
        axisLabel: { color: textColor },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: "CPU (%)",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: cpuData,
          itemStyle: { color: "#ef4444" },
        },
        {
          name: "Memory (%)",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: memData,
          itemStyle: { color: "#3b82f6" },
        },
      ],
    });
  };

  // Render Network Traffic Chart
  const renderNetChart = () => {
    if (!netChartElem) return;
    if (netChartInstance) netChartInstance.dispose();
    netChartInstance = echarts.init(netChartElem);

    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const txData: [Date, number][] = [];
    const rxData: [Date, number][] = [];

    currentMonitors.forEach((m) => {
      const t = new Date(m.Time / 1e6);
      txData.push([t, Number(m.TxSpeed.toFixed(2))]);
      rxData.push([t, Number(m.RxSpeed.toFixed(2))]);
    });

    netChartInstance.setOption({
      title: {
        text: selected ? `${selected.Host} ${$_("report.sensorNetChart")}` : $_("report.sensorNetChart"),
        left: "center",
        textStyle: { color: textColor, fontSize: 14 },
      },
      tooltip: {
        trigger: "axis",
        formatter: (params: any) => {
          if (!params || params.length === 0) return "";
          const t = echarts.time.format(params[0].value[0], "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}", false);
          let res = `<b>${t}</b><br/>`;
          params.forEach((p: any) => {
            res += `${p.marker} ${p.seriesName}: ${p.value[1]} Mbps<br/>`;
          });
          return res;
        },
      },
      legend: {
        data: ["TxSpeed", "RxSpeed"],
        top: 30,
        textStyle: { color: textColor },
      },
      grid: { left: "5%", right: "5%", top: 70, bottom: 40, containLabel: true },
      dataZoom: [{ type: "slider", bottom: 5 }, { type: "inside" }],
      toolbox: {
        feature: { saveAsImage: { name: "sensor_net" } },
        iconStyle: { borderColor: textColor },
      },
      xAxis: {
        type: "time",
        axisLabel: {
          color: textColor,
          formatter: (val: number) => echarts.time.format(val, "{yyyy}/{MM}/{dd} {HH}:{mm}", false),
        },
        axisLine: { lineStyle: { color: splitLineColor } },
        splitLine: { show: false },
      },
      yAxis: {
        type: "value",
        name: "Mbps",
        nameTextStyle: { color: textColor },
        axisLabel: { color: textColor },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: "TxSpeed",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: txData,
          itemStyle: { color: "#8b5cf6" },
        },
        {
          name: "RxSpeed",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: rxData,
          itemStyle: { color: "#06b6d4" },
        },
      ],
    });
  };

  // Render Process & Load Chart
  const renderProcChart = () => {
    if (!procChartElem) return;
    if (procChartInstance) procChartInstance.dispose();
    procChartInstance = echarts.init(procChartElem);

    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const loadData: [Date, number][] = [];
    const procData: [Date, number][] = [];

    currentMonitors.forEach((m) => {
      const t = new Date(m.Time / 1e6);
      loadData.push([t, Number(m.Load.toFixed(2))]);
      procData.push([t, m.Process]);
    });

    procChartInstance.setOption({
      title: {
        text: selected ? `${selected.Host} ${$_("report.sensorProcChart")}` : $_("report.sensorProcChart"),
        left: "center",
        textStyle: { color: textColor, fontSize: 14 },
      },
      tooltip: {
        trigger: "axis",
      },
      legend: {
        data: ["Load Average", "Process Count"],
        top: 30,
        textStyle: { color: textColor },
      },
      grid: { left: "5%", right: "5%", top: 70, bottom: 40, containLabel: true },
      dataZoom: [{ type: "slider", bottom: 5 }, { type: "inside" }],
      toolbox: {
        feature: { saveAsImage: { name: "sensor_proc" } },
        iconStyle: { borderColor: textColor },
      },
      xAxis: {
        type: "time",
        axisLabel: {
          color: textColor,
          formatter: (val: number) => echarts.time.format(val, "{yyyy}/{MM}/{dd} {HH}:{mm}", false),
        },
        axisLine: { lineStyle: { color: splitLineColor } },
        splitLine: { show: false },
      },
      yAxis: [
        {
          type: "value",
          name: "Load",
          nameTextStyle: { color: textColor },
          axisLabel: { color: textColor },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        {
          type: "value",
          name: "Process",
          nameTextStyle: { color: textColor },
          axisLabel: { color: textColor },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: "Load Average",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: loadData,
          itemStyle: { color: "#f59e0b" },
        },
        {
          name: "Process Count",
          type: "bar",
          yAxisIndex: 1,
          data: procData,
          itemStyle: { color: "rgba(59, 130, 246, 0.7)" },
        },
      ],
    });
  };

  // Render Sensor Tree Diagram
  const renderTreeChart = () => {
    if (!treeChartElem) return;
    if (treeChartInstance) treeChartInstance.dispose();

    const dark = isDarkMode();
    treeChartInstance = echarts.init(treeChartElem, dark ? "dark" : undefined);
    const textColor = dark ? "#f1f5f9" : "#1e293b";

    const type = selectedTreeType || (uniqueTypes.length > 0 ? uniqueTypes[0] : "syslog");
    const treeData: any = { name: type, children: [] };
    let max = 1;

    sensors.forEach((s) => {
      if (s.Type !== type) return;
      if (max < s.Total) max = s.Total;
      treeData.children.push({
        name: `${s.Host}${s.Param ? ` (${s.Param})` : ""}`,
        value: s.Total,
      });
    });

    treeChartInstance.setOption({
      backgroundColor: "transparent",
      tooltip: {
        trigger: "item",
        triggerOn: "mousemove",
        formatter: (params: any) => {
          const val = params.value != null ? `${$_("report.colTotal")}: ${Number(params.value).toLocaleString()}` : "";
          return `<b>${params.name}</b><br/>${val}`;
        },
      },
      series: [
        {
          type: "tree",
          data: [treeData],
          top: "8%",
          left: "12%",
          bottom: "8%",
          right: "18%",
          symbolSize: (v: number) => (v ? Math.max(8, Math.min(24, (16 * v) / max + 8)) : 10),
          orient: "LR",
          label: {
            position: "left",
            verticalAlign: "middle",
            align: "right",
            fontSize: 11,
            color: textColor,
          },
          leaves: {
            label: {
              position: "right",
              verticalAlign: "middle",
              align: "left",
              color: textColor,
            },
          },
          itemStyle: {
            color: "#06b6d4",
            borderColor: "#38bdf8",
          },
          lineStyle: {
            color: dark ? "#334155" : "#cbd5e1",
            curveness: 0.5,
          },
          emphasis: {
            focus: "descendant",
          },
          expandAndCollapse: true,
          animationDuration: 400,
          animationDurationUpdate: 500,
        },
      ],
    });
  };

  $effect(() => {
    if (sensors.length > 0 && treeChartElem) {
      if (!selectedTreeType || !uniqueTypes.includes(selectedTreeType)) {
        selectedTreeType = uniqueTypes.includes("syslog") ? "syslog" : uniqueTypes[0] || "";
      }
      tick().then(() => renderTreeChart());
    }
  });

  // Export CSV matching other reports
  export function exportCSV(): void {
    let csv = `"${$_("report.colState")}","${$_("report.colHost")}","${$_("report.colType")}","${$_("report.colParam")}","${$_("report.colTotal")}","${$_("report.colSend")}","${$_("report.colStatsCount")}","${$_("report.colMonitorsCount")}","${$_("report.colFirstTime")}","${$_("report.colLastTime")}"\n`;
    filteredSensors.forEach((s) => {
      const stateName = getStateName(s.State);
      const first = s.FirstTime ? formatTimeStr(s.FirstTime) : "-";
      const last = s.LastTime ? formatTimeStr(s.LastTime) : "-";
      const host = `"${(s.Host || "").replace(/"/g, '""')}"`;
      const type = `"${(s.Type || "").replace(/"/g, '""')}"`;
      const param = `"${(s.Param || "").replace(/"/g, '""')}"`;

      csv += `"${stateName}",${host},${type},${param},${s.Total},${s.Send},${s.StatsLen},${s.MonitorsLen},"${first}","${last}"\n`;
    });

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_sensor_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Top Alerts -->
  {#if alertMessage}
    <div
      class="p-3 rounded-xl border flex items-center justify-between text-xs font-medium transition-all {alertMessage.type === 'success' ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 border-rose-500/30 text-rose-600 dark:text-rose-400'}"
    >
      <div class="flex items-center gap-2">
        {#if alertMessage.type === 'success'}
          <CheckCircle2 class="w-4 h-4 shrink-0" />
        {:else}
          <AlertCircle class="w-4 h-4 shrink-0" />
        {/if}
        <span>{alertMessage.text}</span>
      </div>
      <button onclick={() => (alertMessage = null)} class="opacity-60 hover:opacity-100 p-0.5 cursor-pointer">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  <!-- Header Title -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Radio class="w-5 h-5 text-cyan-400" />
        {$_("report.sensorMainTitle")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.sensorMainSubtitle")}</p>
    </div>
  </div>

  <!-- KPI Cards Grid -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <!-- Total Sensors -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalSensors")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {sensors.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitSensors")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.sensorSummarySub", { values: { types: uniqueTypes.length, hosts: uniqueHosts.length } })}
        </div>
      </div>

      <!-- Active Normal Sensors -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.runningSensors")}</span>
        <div class="text-2xl font-bold font-mono text-emerald-400">
          {kpiCounts.normal} <span class="text-xs font-normal text-slate-400">{$_("report.unitSensors")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.runningSensorsSub")}</div>
      </div>

      <!-- Alert Sensors -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.alertSensors")}</span>
        <div class="text-2xl font-bold font-mono text-rose-400">
          {kpiCounts.problem} <span class="text-xs font-normal text-slate-400">{$_("report.unitSensors")}</span>
        </div>
        <div class="text-[10px] text-slate-400 flex items-center gap-1.5">
          <span class="text-rose-400">{$_("report.stateHigh")}: {kpiCounts.high}</span>
          <span>•</span>
          <span class="text-amber-400">{$_("report.stateLow")}: {kpiCounts.low}</span>
          <span>•</span>
          <span class="text-yellow-400">{$_("report.stateWarn")}: {kpiCounts.warn}</span>
        </div>
      </div>
    </div>

    <!-- Total Received Messages / Events -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2 flex flex-col justify-between">
      <span class="text-xs font-semibold text-slate-400">{$_("report.totalSensorEvents")}</span>
      <div class="text-2xl font-bold font-mono text-indigo-400">
        {kpiCounts.totalMessages.toLocaleString()} <span class="text-xs font-normal text-slate-400">{$_("report.unitEvents")}</span>
      </div>
      <div class="text-[10px] text-slate-400">
        {$_("report.totalSensorSendSub", { values: { send: kpiCounts.totalSend.toLocaleString() } })}
      </div>
    </div>
  </div>

  <!-- Sensor Tree Hierarchy Graph Card (Under KPI) -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
    <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-2">
      <div class="flex items-center gap-2">
        <GitFork class="w-4 h-4 text-cyan-400" />
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("report.sensorTreeTitle")}</span>
      </div>
      <div class="flex items-center gap-2">
        <span class="text-xs text-slate-500 dark:text-slate-400">{$_("report.sensorTreeType")}:</span>
        <select
          bind:value={selectedTreeType}
          onchange={() => renderTreeChart()}
          class="text-xs rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-800 px-2.5 py-1 text-slate-800 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-cyan-500 cursor-pointer"
        >
          {#each uniqueTypes as t}
            <option value={t}>{t}</option>
          {/each}
        </select>
      </div>
    </div>
    <div bind:this={treeChartElem} class="h-64 w-full"></div>
  </div>

  <!-- Subfilter & Dropdown Filters Bar -->
  <div class="flex flex-wrap items-center justify-between gap-3">
    <!-- State filter buttons -->
    <div class="flex flex-wrap items-center gap-1.5">
      <button
        type="button"
        onclick={() => (filterState = "all")}
        class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {filterState === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <Layers class="w-3.5 h-3.5" />
        <span>{$_("report.subAllSensors")} ({sensors.length})</span>
      </button>
      <button
        type="button"
        onclick={() => (filterState = "normal")}
        class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {filterState === 'normal' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <CheckCircle2 class="w-3.5 h-3.5 text-emerald-300" />
        <span>{$_("report.stateNormal")} ({kpiCounts.normal})</span>
      </button>
      <button
        type="button"
        onclick={() => (filterState = "problem")}
        class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {filterState === 'problem' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <AlertTriangle class="w-3.5 h-3.5 text-rose-300" />
        <span>{$_("report.subProblemSensors")} ({kpiCounts.problem})</span>
      </button>
      <button
        type="button"
        onclick={() => (filterState = "off")}
        class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {filterState === 'off' ? 'bg-slate-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <Shield class="w-3.5 h-3.5 text-slate-300" />
        <span>{$_("report.stateOff")} ({kpiCounts.off})</span>
      </button>
    </div>

    <!-- Select Dropdowns for Host and Type -->
    <div class="flex flex-wrap items-center gap-3">
      <!-- Host Select -->
      <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <span>{$_("report.colHost")}:</span>
        <select
          bind:value={filterHost}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs text-slate-700 dark:text-slate-200 focus:outline-none focus:border-cyan-500 cursor-pointer"
        >
          <option value="">{$_("report.allHosts")}</option>
          {#each uniqueHosts as host}
            <option value={host}>{host}</option>
          {/each}
        </select>
      </div>

      <!-- Type Select -->
      <div class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
        <span>{$_("report.colType")}:</span>
        <select
          bind:value={filterType}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs text-slate-700 dark:text-slate-200 focus:outline-none focus:border-cyan-500 cursor-pointer"
        >
          <option value="">{$_("report.allTypes")}</option>
          {#each uniqueTypes as t}
            <option value={t}>{t}</option>
          {/each}
        </select>
      </div>
    </div>
  </div>

  <!-- Sensor Data Table Container -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <div class="overflow-x-auto min-h-[350px]">
      <table class="w-full text-left text-xs border-collapse">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <!-- State -->
            <th class="py-2.5 px-3 w-[110px] cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("State")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colState")}</span>
                {#if sortColumn === "State"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Host -->
            <th class="py-2.5 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("Host")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colHost")}</span>
                {#if sortColumn === "Host"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Type -->
            <th class="py-2.5 px-3 w-[120px] cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("Type")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colType")}</span>
                {#if sortColumn === "Type"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Parameter -->
            <th class="py-2.5 px-3 w-[130px] cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("Param")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colParam")}</span>
                {#if sortColumn === "Param"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Total -->
            <th class="py-2.5 px-3 text-right cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("Total")}>
              <div class="inline-flex items-center justify-end gap-1 w-full">
                <span>{$_("report.colTotal")}</span>
                {#if sortColumn === "Total"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Send -->
            <th class="py-2.5 px-3 text-right cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("Send")}>
              <div class="inline-flex items-center justify-end gap-1 w-full">
                <span>{$_("report.colSend")}</span>
                {#if sortColumn === "Send"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- FirstTime -->
            <th class="py-2.5 px-3 w-[140px] cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("FirstTime")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colFirstTime")}</span>
                {#if sortColumn === "FirstTime"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- LastTime -->
            <th class="py-2.5 px-3 w-[140px] cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("LastTime")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLastTime")}</span>
                {#if sortColumn === "LastTime"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                  {:else}
                    <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                  {/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>

            <!-- Action -->
            <th class="py-2.5 px-3 text-center w-[110px]">
              {$_("report.colAction")}
            </th>
          </tr>
        </thead>

        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
          {#if loading || parentLoading}
            <tr>
              <td colspan="9" class="text-center py-12 text-slate-400 dark:text-slate-500 font-sans">
                <div class="inline-flex items-center gap-2">
                  <RefreshCw class="w-4 h-4 animate-spin text-cyan-500" />
                  {$_("report.loadingSensors")}
                </div>
              </td>
            </tr>
          {:else if paginatedSensors.length === 0}
            <tr>
              <td colspan="9" class="text-center py-12 text-slate-400 dark:text-slate-500 font-sans">
                {$_("report.noSensors")}
              </td>
            </tr>
          {:else}
            {#each paginatedSensors as item}
              <tr class="hover:bg-slate-50/60 dark:hover:bg-slate-800/40 transition-colors group">
                <!-- State -->
                <td class="py-2 px-3 font-sans">
                  <div class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-semibold border"
                    style="border-color: {getStateColor(item.State)}40; background-color: {getStateColor(item.State)}15; color: {getStateColor(item.State)};"
                  >
                    <span class="w-1.5 h-1.5 rounded-full" style="background-color: {getStateColor(item.State)};"></span>
                    <span>{getStateName(item.State)}</span>
                  </div>
                </td>

                <!-- Host -->
                <td class="py-2 px-3 font-semibold text-slate-900 dark:text-slate-100 text-xs">
                  <span class="select-all">{item.Host}</span>
                </td>

                <!-- Type -->
                <td class="py-2 px-3 text-xs">
                  <span class="px-2 py-0.5 rounded-md bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 font-medium">
                    {item.Type}
                  </span>
                </td>

                <!-- Parameter -->
                <td class="py-2 px-3 text-slate-500 dark:text-slate-400 truncate max-w-[130px] text-xs">
                  {item.Param || "-"}
                </td>

                <!-- Total -->
                <td class="py-2 px-3 text-right text-slate-800 dark:text-slate-200 text-xs font-mono">
                  {item.Total.toLocaleString()}
                </td>

                <!-- Send -->
                <td class="py-2 px-3 text-right text-slate-800 dark:text-slate-200 text-xs font-mono">
                  {item.Send.toLocaleString()}
                </td>

                <!-- FirstTime -->
                <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {item.FirstTime ? formatTimeStr(item.FirstTime) : "-"}
                </td>

                <!-- LastTime -->
                <td class="py-2 px-3 text-slate-700 dark:text-slate-300 text-[11px] whitespace-nowrap">
                  {item.LastTime ? formatTimeStr(item.LastTime) : "-"}
                </td>

                <!-- Action Buttons -->
                <td class="py-2 px-3 text-center font-sans">
                  <div class="inline-flex items-center gap-1">
                    <!-- Info Dialog Button -->
                    <button
                      type="button"
                      title={$_("report.btnDetail")}
                      onclick={() => openInfo(item)}
                      class="p-1 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-300 transition-colors cursor-pointer"
                    >
                      <Eye class="w-3.5 h-3.5" />
                    </button>

                    <!-- Delete Dialog Button -->
                    <button
                      type="button"
                      title={$_("report.btnDelete")}
                      onclick={() => openDelete(item)}
                      class="p-1 rounded-lg hover:bg-rose-100 dark:hover:bg-rose-900/40 text-rose-600 dark:text-rose-400 transition-colors cursor-pointer"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>

                    <!-- Toggle Button -->
                    {#if item.Ignore}
                      <button
                        type="button"
                        title={$_("report.btnEnable")}
                        onclick={() => handleToggle(item)}
                        class="p-1 rounded-lg hover:bg-emerald-100 dark:hover:bg-emerald-900/40 text-emerald-600 dark:text-emerald-400 transition-colors cursor-pointer"
                      >
                        <Play class="w-3.5 h-3.5 fill-current" />
                      </button>
                    {:else}
                      <button
                        type="button"
                        title={$_("report.btnDisable")}
                        onclick={() => handleToggle(item)}
                        class="p-1 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 dark:text-slate-400 transition-colors cursor-pointer"
                      >
                        <Square class="w-3.5 h-3.5 fill-current" />
                      </button>
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Standard Report Pagination Component -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={filteredSensors.length}
    />
  </div>
</div>

<!-- Sensor Info & Graph Launcher Modal -->
{#if showInfoModal && selected}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Modal Header -->
      <div class="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Radio class="w-5 h-5 text-cyan-500" />
          {$_("report.sensorInfoTitle")}
        </h3>
        <button
          type="button"
          onclick={() => (showInfoModal = false)}
          class="p-1 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Modal Body (Table of attributes) -->
      <div class="p-6 overflow-y-auto space-y-4">
        <table class="w-full text-left text-xs border-collapse">
          <thead>
            <tr class="border-b border-slate-200 dark:border-slate-800 text-slate-500 dark:text-slate-400">
              <th class="py-2.5 px-3 w-1/3">{$_("report.sensorItem")}</th>
              <th class="py-2.5 px-3">{$_("report.sensorValue")}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colHost")}</td>
              <td class="py-2 px-3 font-semibold text-slate-900 dark:text-slate-100">{selected.Host}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colType")}</td>
              <td class="py-2 px-3">{selected.Type}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colParam")}</td>
              <td class="py-2 px-3">{selected.Param || "-"}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colTotal")}</td>
              <td class="py-2 px-3">{selected.Total.toLocaleString()}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colSend")}</td>
              <td class="py-2 px-3">{selected.Send.toLocaleString()}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colStatsCount")}</td>
              <td class="py-2 px-3">{selected.StatsLen.toLocaleString()}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colMonitorsCount")}</td>
              <td class="py-2 px-3">{selected.MonitorsLen.toLocaleString()}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colFirstTime")}</td>
              <td class="py-2 px-3">{selected.FirstTime ? formatTimeStr(selected.FirstTime) : "-"}</td>
            </tr>
            <tr>
              <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400">{$_("report.colLastTime")}</td>
              <td class="py-2 px-3">{selected.LastTime ? formatTimeStr(selected.LastTime) : "-"}</td>
            </tr>
          </tbody>
        </table>

        <!-- Graph selection buttons if telemetry available -->
        {#if selected.StatsLen > 0 || selected.MonitorsLen > 0}
          <div class="pt-3 border-t border-slate-200 dark:border-slate-800 space-y-2">
            <div class="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
              <TrendingUp class="w-4 h-4 text-cyan-500" />
              {$_("report.sensorCharts")}
            </div>
            <div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
              {#if selected.StatsLen > 0}
                <button
                  type="button"
                  onclick={openStatsChart}
                  class="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-400 border border-cyan-500/30 transition-colors cursor-pointer"
                >
                  <BarChart3 class="w-4 h-4" />
                  {$_("report.sensorStatsChart")}
                </button>
              {/if}

              {#if selected.MonitorsLen > 0}
                <button
                  type="button"
                  onclick={openCpuMemChart}
                  class="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold bg-rose-500/10 hover:bg-rose-500/20 text-rose-600 dark:text-rose-400 border border-rose-500/30 transition-colors cursor-pointer"
                >
                  <Cpu class="w-4 h-4" />
                  CPU/Memory
                </button>

                <button
                  type="button"
                  onclick={openNetChart}
                  class="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold bg-blue-500/10 hover:bg-blue-500/20 text-blue-600 dark:text-blue-400 border border-blue-500/30 transition-colors cursor-pointer"
                >
                  <Network class="w-4 h-4" />
                  {$_("report.sensorNetChart")}
                </button>

                <button
                  type="button"
                  onclick={openProcChart}
                  class="flex items-center justify-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold bg-amber-500/10 hover:bg-amber-500/20 text-amber-600 dark:text-amber-400 border border-amber-500/30 transition-colors cursor-pointer"
                >
                  <Activity class="w-4 h-4" />
                  {$_("report.sensorProcChart")}
                </button>
              {/if}
            </div>
          </div>
        {/if}
      </div>

      <!-- Modal Footer -->
      <div class="px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50 flex justify-end">
        <button
          type="button"
          onclick={() => (showInfoModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("report.btnClose")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Sensor Delete Confirmation Modal -->
{#if showDeleteModal && selected}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-md bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl p-6 space-y-4">
      <div class="flex items-center gap-3 text-rose-500">
        <div class="p-3 rounded-2xl bg-rose-500/10 border border-rose-500/20">
          <Trash2 class="w-6 h-6" />
        </div>
        <div>
          <h3 class="text-base font-bold text-slate-900 dark:text-slate-100">{$_("report.sensorDeleteTitle")}</h3>
          <p class="text-xs text-slate-500 dark:text-slate-400">{$_("report.sensorDeleteConfirm")}</p>
        </div>
      </div>

      <div class="p-3 rounded-2xl bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-800 text-xs space-y-1 font-mono">
        <div><span class="text-slate-500">{$_("report.colHost")}:</span> <span class="font-semibold text-slate-900 dark:text-slate-100">{selected.Host}</span></div>
        <div><span class="text-slate-500">{$_("report.colType")}:</span> <span>{selected.Type}</span></div>
        {#if selected.Param}
          <div><span class="text-slate-500">{$_("report.colParam")}:</span> <span>{selected.Param}</span></div>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 pt-2">
        <button
          type="button"
          onclick={() => (showDeleteModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_("report.btnCancel")}
        </button>
        <button
          type="button"
          onclick={handleConfirmDelete}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-rose-600 hover:bg-rose-500 text-white shadow-sm transition-colors cursor-pointer"
        >
          {$_("report.btnDelete")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Stats Chart Modal -->
{#if showStatsChartModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-5xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl p-6 space-y-4 flex flex-col">
      <div class="flex items-center justify-between">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <BarChart3 class="w-5 h-5 text-cyan-500" />
          {$_("report.sensorStatsChart")}
        </h3>
        <button
          type="button"
          onclick={() => (showStatsChartModal = false)}
          class="p-1 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
      <div bind:this={statsChartElem} class="w-full h-[55vh]"></div>
      <div class="flex justify-end pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showStatsChartModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("report.btnClose")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- CPU / Memory Chart Modal -->
{#if showCpuMemChartModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-5xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl p-6 space-y-4 flex flex-col">
      <div class="flex items-center justify-between">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Cpu class="w-5 h-5 text-rose-500" />
          {$_("report.sensorCpuMemChart")}
        </h3>
        <button
          type="button"
          onclick={() => (showCpuMemChartModal = false)}
          class="p-1 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
      <div bind:this={cpuMemChartElem} class="w-full h-[55vh]"></div>
      <div class="flex justify-end pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showCpuMemChartModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("report.btnClose")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Network Traffic Chart Modal -->
{#if showNetChartModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-5xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl p-6 space-y-4 flex flex-col">
      <div class="flex items-center justify-between">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Network class="w-5 h-5 text-blue-500" />
          {$_("report.sensorNetChart")}
        </h3>
        <button
          type="button"
          onclick={() => (showNetChartModal = false)}
          class="p-1 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
      <div bind:this={netChartElem} class="w-full h-[55vh]"></div>
      <div class="flex justify-end pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showNetChartModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("report.btnClose")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Process Chart Modal -->
{#if showProcChartModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-sm animate-in fade-in duration-200">
    <div class="w-full max-w-5xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-3xl shadow-2xl p-6 space-y-4 flex flex-col">
      <div class="flex items-center justify-between">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Activity class="w-5 h-5 text-amber-500" />
          {$_("report.sensorProcChart")}
        </h3>
        <button
          type="button"
          onclick={() => (showProcChartModal = false)}
          class="p-1 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>
      <div bind:this={procChartElem} class="w-full h-[55vh]"></div>
      <div class="flex justify-end pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showProcChartModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold bg-slate-200 dark:bg-slate-800 hover:bg-slate-300 dark:hover:bg-slate-700 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("report.btnClose")}
        </button>
      </div>
    </div>
  </div>
{/if}
