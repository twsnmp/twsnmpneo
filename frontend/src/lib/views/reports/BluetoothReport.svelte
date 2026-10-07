<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Bluetooth,
    Building2,
    Radio,
    Signal,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Layers,
    ChevronDown,
    ChevronRight,
    LineChart,
    BarChart3,
    PieChart,
    Eye,
    Pencil,
    Download,
    X,
    Server,
    Activity,
    Calendar,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    deleteLogReportItem,
    updateLogReportName,
    type BlueDeviceEnt,
    type NodeEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";

  let {
    searchQuery = "",
    nodes = [],
    onRefresh = () => {},
    loading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  let devices = $state<BlueDeviceEnt[]>([]);
  let internalLoading = $state(false);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // ECharts container & instance for Top Vendors and RSSI Breakdown
  let vendorChartElem = $state<HTMLDivElement | null>(null);
  let vendorChartInstance: echarts.ECharts | null = null;
  let rssiChartElem = $state<HTMLDivElement | null>(null);
  let rssiChartInstance: echarts.ECharts | null = null;

  // Modals
  let detailModalOpen = $state(false);
  let detailDevice = $state<BlueDeviceEnt | null>(null);
  let detailChartElem = $state<HTMLDivElement | null>(null);
  let detailChartInstance: echarts.ECharts | null = null;

  let editNameModalOpen = $state(false);
  let editingDevice = $state<BlueDeviceEnt | null>(null);
  let editingNameValue = $state("");

  let deleteModalOpen = $state(false);
  let deletingDevice = $state<BlueDeviceEnt | null>(null);

  const loadAll = async () => {
    internalLoading = true;
    try {
      devices = await fetchLogReport<BlueDeviceEnt>("blueDevice");
      await tick();
      renderOverviewCharts();
    } catch (err) {
      console.error("Failed to load Bluetooth reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const ok = await showConfirm({
      title: $_('common.confirmClear') || 'データ消去の確認',
      message: $_("report.confirmClearBluetooth") ||
        $_("report.confirmClearItem", { values: { name: "Bluetoothデバイス" } }) ||
        "Bluetoothデバイスのレポートデータを全消去しますか？",
      type: 'warning',
      confirmText: $_('common.clear') || '消去',
    });
    if (!ok) {
      return;
    }
    try {
      await resetLogReport("blueDevice");
      await loadAll();
      onRefresh();
    } catch (err: any) {
      showAlert({
        title: $_('common.error') || 'エラー',
        message: $_("report.alertResetFailed", { values: { error: err.message || err } }),
        type: 'danger',
      });
    }
  }

  onMount(() => {
    loadAll();

    const handleResize = () => {
      vendorChartInstance?.resize();
      rssiChartInstance?.resize();
      detailChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      vendorChartInstance?.dispose();
      rssiChartInstance?.dispose();
      detailChartInstance?.dispose();
    };
  });

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const getLatestRSSI = (d: BlueDeviceEnt): number => {
    if (d.RSSI && d.RSSI.length > 0) {
      return d.RSSI[d.RSSI.length - 1].Value;
    }
    return -100;
  };

  const isPersistentAddress = (addr: string): boolean => {
    if (!addr) return false;
    return addr.startsWith("NAME:") || addr.includes(":TYPE:");
  };

  const getRSSIBadge = (rssi: number) => {
    if (rssi >= -60) {
      return { label: `${rssi} dBm`, color: "text-emerald-500 bg-emerald-500/10 border-emerald-500/20" };
    }
    if (rssi >= -75) {
      return { label: `${rssi} dBm`, color: "text-teal-500 bg-teal-500/10 border-teal-500/20" };
    }
    if (rssi >= -85) {
      return { label: `${rssi} dBm`, color: "text-amber-500 bg-amber-500/10 border-amber-500/20" };
    }
    return { label: `${rssi} dBm`, color: "text-rose-500 bg-rose-500/10 border-rose-500/20" };
  };

  const stats = $derived.by(() => {
    const total = devices.length;
    let publicCount = 0;
    let randomCount = 0;
    let strongCount = 0;
    let totalRssi = 0;
    let rssiValidCount = 0;
    let totalPackets = 0;

    const vendorMap = new Map<string, number>();
    const hostSet = new Set<string>();

    for (const d of devices) {
      const type = (d.AddressType || "").toLowerCase();
      if (type.includes("public")) {
        publicCount++;
      } else {
        randomCount++;
      }

      const v = d.Vendor || "Unknown";
      vendorMap.set(v, (vendorMap.get(v) || 0) + 1);

      if (d.Host) hostSet.add(d.Host);
      totalPackets += d.Count || 0;

      const latestR = getLatestRSSI(d);
      if (latestR > -100) {
        totalRssi += latestR;
        rssiValidCount++;
      }
      if (latestR >= -75) {
        strongCount++;
      }
    }

    let topVendor = "-";
    let maxVCount = 0;
    for (const [v, c] of vendorMap.entries()) {
      if (c > maxVCount && v !== "Unknown") {
        maxVCount = c;
        topVendor = `${v} (${c})`;
      }
    }
    if (topVendor === "-" && vendorMap.size > 0) {
      const first = [...vendorMap.entries()].sort((a, b) => b[1] - a[1])[0];
      topVendor = `${first[0]} (${first[1]})`;
    }

    const avgRssi = rssiValidCount > 0 ? (totalRssi / rssiValidCount).toFixed(1) : "-";
    const uniqueVendors = vendorMap.size;
    const uniqueHosts = hostSet.size;

    return {
      total,
      publicCount,
      randomCount,
      uniqueVendors,
      topVendor,
      strongCount,
      avgRssi,
      uniqueHosts,
      totalPackets,
    };
  });

  const renderOverviewCharts = () => {
    if (devices.length === 0) return;
    const dark = isDarkMode();
    const textColor = dark ? "#94a3b8" : "#64748b";
    const titleColor = dark ? "#f8fafc" : "#0f172a";
    const splitLineColor = dark ? "#1e293b" : "#f1f5f9";
    const axisLineColor = dark ? "#334155" : "#cbd5e1";
    const tooltipBg = dark ? "rgba(15, 23, 42, 0.95)" : "rgba(255, 255, 255, 0.95)";
    const tooltipBorder = dark ? "#334155" : "#e2e8f0";

    // 1. Top 10 Vendors Bar Chart
    if (vendorChartElem) {
      if (!vendorChartInstance) {
        vendorChartInstance = echarts.init(vendorChartElem);
      }
      const vendorMap = new Map<string, number>();
      for (const d of devices) {
        const v = d.Vendor || "Unknown";
        vendorMap.set(v, (vendorMap.get(v) || 0) + 1);
      }
      const sortedVendors = [...vendorMap.entries()]
        .sort((a, b) => b[1] - a[1])
        .slice(0, 10);

      const optionVendor: echarts.EChartsOption = {
        backgroundColor: "transparent",
        tooltip: {
          trigger: "axis",
          axisPointer: { type: "shadow" },
          backgroundColor: tooltipBg,
          borderColor: tooltipBorder,
          textStyle: { color: titleColor, fontSize: 12 },
        },
        grid: { top: "12%", left: "3%", right: "4%", bottom: "6%", containLabel: true },
        xAxis: {
          type: "category",
          data: sortedVendors.map((v) => v[0]),
          axisLabel: {
            color: textColor,
            fontSize: 10,
            interval: 0,
            rotate: sortedVendors.length > 5 ? 25 : 0,
            formatter: (val: string) => (val.length > 14 ? val.slice(0, 12) + "…" : val),
          },
          axisLine: { lineStyle: { color: axisLineColor } },
        },
        yAxis: {
          type: "value",
          minInterval: 1,
          axisLabel: { color: textColor, fontSize: 11 },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        series: [
          {
            name: $_("report.bluetoothDeviceCount") || "Device Count",
            type: "bar",
            data: sortedVendors.map((v) => v[1]),
            itemStyle: {
              color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: "#3b82f6" },
                { offset: 1, color: "#1d4ed8" },
              ]),
              borderRadius: [4, 4, 0, 0],
            },
            barMaxWidth: 28,
          },
        ],
      };
      vendorChartInstance.setOption(optionVendor);
    }

    // 2. RSSI Level Breakdown Donut Chart
    if (rssiChartElem) {
      if (!rssiChartInstance) {
        rssiChartInstance = echarts.init(rssiChartElem);
      }

      let cExcellent = 0; // >= -60
      let cGood = 0;      // -60 ~ -75
      let cFair = 0;      // -75 ~ -85
      let cPoor = 0;      // < -85

      for (const d of devices) {
        const r = getLatestRSSI(d);
        if (r >= -60) cExcellent++;
        else if (r >= -75) cGood++;
        else if (r >= -85) cFair++;
        else cPoor++;
      }

      const rssiData = [
        {
          name: $_("report.rssiExcellent") || "極めて良好 (≧ -60dBm)",
          value: cExcellent,
          itemStyle: { color: "#10b981" },
        },
        {
          name: $_("report.rssiGood") || "良好 (-60 〜 -75dBm)",
          value: cGood,
          itemStyle: { color: "#14b8a6" },
        },
        {
          name: $_("report.rssiFair") || "普通 (-75 〜 -85dBm)",
          value: cFair,
          itemStyle: { color: "#f59e0b" },
        },
        {
          name: $_("report.rssiPoor") || "微弱 (< -85dBm)",
          value: cPoor,
          itemStyle: { color: "#f43f5e" },
        },
      ].filter((item) => item.value > 0);

      const optionRssi: echarts.EChartsOption = {
        backgroundColor: "transparent",
        tooltip: {
          trigger: "item",
          backgroundColor: tooltipBg,
          borderColor: tooltipBorder,
          textStyle: { color: titleColor, fontSize: 12 },
          formatter: "{b}: <b>{c}</b> ({d}%)",
        },
        legend: {
          orient: "vertical",
          right: "4%",
          top: "center",
          textStyle: { color: textColor, fontSize: 11 },
          itemWidth: 10,
          itemHeight: 10,
        },
        series: [
          {
            name: $_("report.bluetoothRssiDistribution") || "RSSI Distribution",
            type: "pie",
            radius: ["45%", "72%"],
            center: ["38%", "50%"],
            avoidLabelOverlap: false,
            itemStyle: {
              borderRadius: 6,
              borderColor: dark ? "#0f172a" : "#ffffff",
              borderWidth: 2,
            },
            label: { show: false },
            emphasis: {
              label: {
                show: true,
                fontSize: 12,
                fontWeight: "bold",
                color: titleColor,
              },
            },
            data: rssiData,
          },
        ],
      };
      rssiChartInstance.setOption(optionRssi);
    }
  };

  const filteredList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = devices;
    if (q) {
      res = res.filter(
        (d) =>
          d.Name?.toLowerCase().includes(q) ||
          d.Address?.toLowerCase().includes(q) ||
          d.Vendor?.toLowerCase().includes(q) ||
          d.Host?.toLowerCase().includes(q) ||
          d.AddressType?.toLowerCase().includes(q) ||
          d.Info?.toLowerCase().includes(q)
      );
    }
    return [...res].sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];
      if (sortColumn === "RSSI") {
        valA = getLatestRSSI(a);
        valB = getLatestRSSI(b);
      }
      if (typeof valA === "string") {
        const cmp = valA.localeCompare(valB || "");
        return sortDirection === "asc" ? cmp : -cmp;
      }
      valA = Number(valA || 0);
      valB = Number(valB || 0);
      return sortDirection === "asc" ? valA - valB : valB - valA;
    });
  });

  const paginatedList = $derived.by(() => {
    if (pageSize === -1) return filteredList;
    const start = (currentPage - 1) * pageSize;
    return filteredList.slice(start, start + pageSize);
  });

  const openEditNameModal = (d: BlueDeviceEnt) => {
    editingDevice = d;
    editingNameValue = d.Name || "";
    editNameModalOpen = true;
  };

  const handleSaveName = async () => {
    if (!editingDevice) return;
    try {
      await updateLogReportName("blueDevice", editingDevice.ID, editingNameValue);
      editNameModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert("名前の変更に失敗しました: " + (err.message || err));
    }
  };

  const openDeleteModal = (d: BlueDeviceEnt) => {
    deletingDevice = d;
    deleteModalOpen = true;
  };

  const handleConfirmDelete = async () => {
    if (!deletingDevice) return;
    try {
      await deleteLogReportItem("blueDevice", deletingDevice.ID);
      deleteModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert($_("report.alertDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  const openDetailModal = async (d: BlueDeviceEnt) => {
    detailDevice = d;
    detailModalOpen = true;
    await tick();
    renderDetailChart();
  };

  const renderDetailChart = () => {
    if (!detailChartElem || !detailDevice) return;
    if (detailChartInstance) {
      detailChartInstance.dispose();
    }
    detailChartInstance = echarts.init(detailChartElem);
    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const rssiData = (detailDevice.RSSI || []).map((r) => [
      new Date(r.Time * 1e6),
      r.Value,
    ]);

    detailChartInstance.setOption({
      tooltip: { trigger: "axis" },
      legend: {
        data: [$_("report.bluetoothRssiLegend") || "信号強度 (RSSI dBm)"],
        textStyle: { color: textColor },
      },
      grid: { left: "4%", right: "4%", bottom: "12%", top: "18%", containLabel: true },
      xAxis: {
        type: "time",
        axisLabel: { color: textColor, fontSize: 11 },
        axisLine: { lineStyle: { color: splitLineColor } },
      },
      yAxis: {
        type: "value",
        name: "dBm",
        max: -30,
        min: -100,
        axisLabel: { color: textColor },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: $_("report.bluetoothRssiLegend") || "信号強度 (RSSI dBm)",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: rssiData,
          itemStyle: { color: "#3b82f6" },
          areaStyle: { opacity: 0.15 },
        },
      ],
    });
  };

  export function exportCSV(): void {
    const colAddr = $_("report.sensorAddress") || "Address";
    const colType = $_("report.colAddressType") || "Address Type";
    const colName = $_("report.sensorName") || "Name";
    const colVendor = $_("report.colVendor") || "Vendor";
    const colHost = $_("report.sensorHost") || "Scanner Host";
    const colSignal = $_("report.sensorSignal") || "Signal Level(dBm)";
    const colCount = $_("report.sensorCount") || "Count";
    const colInfo = $_("report.colInfo") || "Additional Info";
    const colFirst = $_("report.bluetoothFirstSeen") || "First Seen";
    const colLast = $_("report.bluetoothLastSeen") || "Last Seen";

    let csv = `"${colAddr}","${colType}","${colName}","${colVendor}","${colHost}","${colSignal}","${colCount}","${colInfo}","${colFirst}","${colLast}"\n`;
    filteredList.forEach((d) => {
      const lastR = getLatestRSSI(d);
      csv += `"${d.Address}","${d.AddressType || ""}","${d.Name || ""}","${d.Vendor || ""}","${d.Host || ""}",${lastR},${d.Count || 0},"${(d.Info || "").replace(/"/g, '""')}","${formatTimeStr(d.FirstTime * 1e6)}","${formatTimeStr(d.LastTime * 1e6)}"\n`;
    });

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_bluetooth_report_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <!-- Title & Actions -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Bluetooth class="w-5 h-5 text-blue-500" />
        {$_("report.tabBluetooth")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        {$_("report.bluetoothSubtitle")}
      </p>
    </div>

    <div class="flex items-center gap-2">
      <button
        type="button"
        onclick={exportCSV}
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-200 transition-colors"
      >
        <Download class="w-3.5 h-3.5" />
        {$_("report.exportCsv")}
      </button>
    </div>
  </div>

  <!-- Summary KPI Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Card 1: Total Devices -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-blue-50 dark:bg-blue-950/50 border border-blue-200 dark:border-blue-800/60 text-blue-600 dark:text-blue-400">
        <Bluetooth class="w-5 h-5" />
      </div>
      <div class="min-w-0">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.bluetoothTotalDevices")}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.total.toLocaleString()}
        </div>
        <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate mt-0.5 font-mono">
          {$_("report.bluetoothPublicAndRandom", { values: { public: stats.publicCount, random: stats.randomCount } })}
        </div>
      </div>
    </div>

    <!-- Card 2: Vendor Count -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/50 border border-emerald-200 dark:border-emerald-800/60 text-emerald-600 dark:text-emerald-400">
        <Building2 class="w-5 h-5" />
      </div>
      <div class="min-w-0">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.bluetoothVendorCount")}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.uniqueVendors.toLocaleString()}
        </div>
        <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate mt-0.5" title={stats.topVendor}>
          {$_("report.bluetoothTopVendor", { values: { vendor: stats.topVendor } })}
        </div>
      </div>
    </div>

    <!-- Card 3: Good Signal Strength -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/50 border border-amber-200 dark:border-amber-800/60 text-amber-600 dark:text-amber-400">
        <Signal class="w-5 h-5" />
      </div>
      <div class="min-w-0">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.bluetoothGoodSignal")}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.strongCount.toLocaleString()}
        </div>
        <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate mt-0.5 font-mono">
          {$_("report.bluetoothAvgRssi", { values: { rssi: stats.avgRssi } })}
        </div>
      </div>
    </div>

    <!-- Card 4: Scanner Hosts -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-purple-50 dark:bg-purple-950/50 border border-purple-200 dark:border-purple-800/60 text-purple-600 dark:text-purple-400">
        <Radio class="w-5 h-5" />
      </div>
      <div class="min-w-0">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.bluetoothScannerHosts")}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.uniqueHosts.toLocaleString()}
        </div>
        <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate mt-0.5 font-mono">
          {$_("report.bluetoothTotalPackets", { values: { count: stats.totalPackets } })}
        </div>
      </div>
    </div>
  </div>

  <!-- Analytics Charts (Top Vendors & Signal Strength Distribution) -->
  {#if devices.length > 0}
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <!-- Chart 1: Top Vendors -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm">
        <div class="text-xs font-bold text-slate-700 dark:text-slate-300 mb-2 flex items-center gap-2">
          <BarChart3 class="w-4 h-4 text-blue-500" />
          {$_("report.bluetoothVendorDistribution")}
        </div>
        <div bind:this={vendorChartElem} class="h-48 w-full"></div>
      </div>

      <!-- Chart 2: Signal Strength Distribution -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm">
        <div class="text-xs font-bold text-slate-700 dark:text-slate-300 mb-2 flex items-center gap-2">
          <PieChart class="w-4 h-4 text-emerald-500" />
          {$_("report.bluetoothRssiDistribution")}
        </div>
        <div bind:this={rssiChartElem} class="h-48 w-full"></div>
      </div>
    </div>
  {/if}

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden flex flex-col">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Bluetooth class="w-4 h-4 text-blue-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {$_("report.bluetoothList")}
        </span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 font-mono text-slate-600 dark:text-slate-300">
          {filteredList.length}
        </span>
      </div>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
        <thead class="bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 font-medium uppercase tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("RSSI")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorSignal")}</span>
                {#if sortColumn === "RSSI"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Address")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorAddress")}</span>
                {#if sortColumn === "Address"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Name")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorName")}</span>
                {#if sortColumn === "Name"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Vendor")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.colVendor")}</span>
                {#if sortColumn === "Vendor"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Host")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorHost")}</span>
                {#if sortColumn === "Host"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("AddressType")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.colAddressType")}</span>
                {#if sortColumn === "AddressType"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5">{$_("report.colInfo")}</th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Count")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorCount")}</span>
                {#if sortColumn === "Count"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("LastTime")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorLast")}</span>
                {#if sortColumn === "LastTime"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 text-right">{$_("common.action")}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="10" class="text-center py-12 text-slate-400">
                <div class="flex flex-col items-center justify-center gap-2">
                  <Bluetooth class="w-8 h-8 opacity-20 text-blue-500" />
                  <p class="text-sm">{$_("report.bluetoothNoData")}</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each paginatedList as item}
              {@const lastRSSI = getLatestRSSI(item)}
              {@const rssiBadge = getRSSIBadge(lastRSSI)}
              <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/40 transition-colors group">
                <td class="px-3 py-2 whitespace-nowrap">
                  <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-medium border ${rssiBadge.color}`}>
                    <Signal class="w-3 h-3" />
                    {rssiBadge.label}
                  </span>
                </td>
                <td class="px-3 py-2 font-mono text-[11px] font-medium text-slate-900 dark:text-slate-100 whitespace-nowrap">
                  <button
                    type="button"
                    onclick={() => openDetailModal(item)}
                    class="hover:underline hover:text-blue-500 text-left cursor-pointer"
                    title={item.Address}
                  >
                    {#if isPersistentAddress(item.Address)}
                      <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-sans font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700">
                        {$_("report.sensorNamedId")}
                      </span>
                    {:else}
                      {item.Address}
                    {/if}
                  </button>
                </td>
                <td class="px-3 py-2 font-medium text-slate-800 dark:text-slate-200 max-w-[160px] truncate">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate">{item.Name || "-"}</span>
                    <button
                      type="button"
                      onclick={() => openEditNameModal(item)}
                      class="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-blue-500 transition-opacity p-0.5"
                      title={$_("report.sensorEditName")}
                    >
                      <Pencil class="w-3 h-3" />
                    </button>
                  </div>
                </td>
                <td class="px-3 py-2 text-slate-600 dark:text-slate-400 max-w-[140px] truncate">
                  {item.Vendor || "-"}
                </td>
                <td class="px-3 py-2 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                  {item.Host || "-"}
                </td>
                <td class="px-3 py-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {item.AddressType || "-"}
                </td>
                <td class="px-3 py-2 text-slate-500 dark:text-slate-400 text-[11px] max-w-[200px] truncate" title={item.Info || ""}>
                  {item.Info || "-"}
                </td>
                <td class="px-3 py-2 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap font-mono">
                  {item.Count || 0}
                </td>
                <td class="px-3 py-2 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap font-mono">
                  {formatTimeStr(item.LastTime * 1e6)}
                </td>
                <td class="px-3 py-2 text-right whitespace-nowrap">
                  <div class="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      onclick={() => openDetailModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-blue-500 transition-colors"
                      title={$_("report.sensorDetailAndChart")}
                    >
                      <Eye class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => openEditNameModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors"
                      title={$_("report.sensorEditName")}
                    >
                      <Pencil class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => openDeleteModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 transition-colors"
                      title={$_("common.delete")}
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    {#if filteredList.length > 0}
      <div class="p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/30">
        <ReportPagination
          totalCount={filteredList.length}
          bind:pageSize
          bind:currentPage
        />
      </div>
    {/if}
  </div>
</div>

<!-- Bluetooth Device Detail Modal -->
{#if detailModalOpen && detailDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-3xl max-h-[90vh] flex flex-col overflow-hidden">
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-500">
            <Bluetooth class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              {$_("report.bluetoothDetailTitle")} - {detailDevice.Name || (isPersistentAddress(detailDevice.Address) ? $_("report.sensorNamedId") : detailDevice.Address)}
            </h3>
            <p class="text-xs text-slate-500 font-mono mt-0.5">
              {#if isPersistentAddress(detailDevice.Address)}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-sans font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 mr-1">
                  {$_("report.sensorNamedId")}
                </span>
              {:else}
                {detailDevice.Address}
              {/if}
              ({detailDevice.AddressType || "Public"})
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800">
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorAddress")}</div>
            <div class="font-mono font-medium text-slate-800 dark:text-slate-200 mt-0.5 break-all">
              {#if isPersistentAddress(detailDevice.Address)}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-sans font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 mr-1">
                  {$_("report.sensorNamedId")}
                </span>
                <span class="text-[10px] text-slate-400">({detailDevice.Address})</span>
              {:else}
                {detailDevice.Address}
              {/if}
            </div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorName")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailDevice.Name || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.colVendor")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailDevice.Vendor || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorSignal")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5 font-mono">{getLatestRSSI(detailDevice)} dBm</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorHost")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{detailDevice.Host || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorCount")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5 font-mono">{detailDevice.Count || 0}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.bluetoothFirstSeen")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5 font-mono">{formatTimeStr(detailDevice.FirstTime * 1e6)}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.bluetoothLastSeen")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5 font-mono">{formatTimeStr(detailDevice.LastTime * 1e6)}</div>
          </div>
        </div>

        {#if detailDevice.Info}
          <div class="p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800">
            <div class="text-[10px] text-slate-400 uppercase font-semibold mb-1">{$_("report.colInfo")}</div>
            <div class="font-mono text-slate-700 dark:text-slate-300 break-all">{detailDevice.Info}</div>
          </div>
        {/if}

        <div>
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 mb-2 flex items-center gap-2">
            <LineChart class="w-4 h-4 text-blue-500" />
            {$_("report.bluetoothRssiTrend")}
          </h4>
          <div
            bind:this={detailChartElem}
            class="w-full h-56 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40"
          ></div>
        </div>
      </div>

      <div class="px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30 flex justify-end">
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="px-4 py-2 rounded-lg text-xs font-semibold bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-800 dark:text-slate-100 transition-colors"
        >
          {$_("common.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Name Modal -->
{#if editNameModalOpen && editingDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Pencil class="w-4 h-4 text-blue-500" />
          {$_("report.bluetoothEditNameTitle")}
        </h3>
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-2">
        <label for="ble-device-name-input" class="block text-xs font-semibold text-slate-600 dark:text-slate-300">
          {$_("report.bluetoothDeviceNameLabel")}
        </label>
        <input
          id="ble-device-name-input"
          type="text"
          bind:value={editingNameValue}
          placeholder={$_("report.bluetoothNamePlaceholder")}
          class="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-xs text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
        />
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleSaveName}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white shadow-sm transition-colors"
        >
          {$_("common.save")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteModalOpen && deletingDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-500 shrink-0">
          <Trash2 class="w-5 h-5" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            {$_("report.bluetoothDeleteTitle")}
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            {$_("report.bluetoothDeleteMsg")}
          </p>
        </div>
      </div>

      <div class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-lg text-xs space-y-1 font-mono text-slate-700 dark:text-slate-300">
        <div>
          {$_("report.sensorAddress")}:
          {#if isPersistentAddress(deletingDevice.Address)}
            <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-sans font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 border border-slate-200 dark:border-slate-700 mr-1">
              {$_("report.sensorNamedId")}
            </span>
            <span class="font-bold">{deletingDevice.Name || deletingDevice.Address}</span>
          {:else}
            <span class="font-bold">{deletingDevice.Address}</span>
          {/if}
        </div>
        {#if deletingDevice.Name && !isPersistentAddress(deletingDevice.Address)}
          <div>{$_("report.sensorName")}: <span class="font-bold">{deletingDevice.Name}</span></div>
        {/if}
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (deleteModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleConfirmDelete}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-rose-600 hover:bg-rose-500 text-white shadow-sm transition-colors"
        >
          {$_("common.delete")}
        </button>
      </div>
    </div>
  </div>
{/if}
