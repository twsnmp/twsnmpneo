<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Thermometer,
    Zap,
    Footprints,
    Trash2,
    ArrowUpDown,
    Signal,
    Sun,
    ToggleLeft,
    ToggleRight,
    LineChart,
    Eye,
    Pencil,
    Download,
    X,
    AlertTriangle,
    Clock,
    Radio,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    deleteLogReportItem,
    updateLogReportName,
    type EnvMonitorEnt,
    type PowerMonitorEnt,
    type MotionSensorEnt,
    type EnvDataEnt,
    type PowerMonitorDataEnt,
    type MotionSensorDataEnt,
    type NodeEnt,
  } from "../../api";

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

  type TabType = "env" | "power" | "motion";
  let activeTab = $state<TabType>("env");

  let envs = $state<EnvMonitorEnt[]>([]);
  let powers = $state<PowerMonitorEnt[]>([]);
  let motions = $state<MotionSensorEnt[]>([]);
  let internalLoading = $state(false);

  // Sorting & Pagination
  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // Modals state
  let detailModalOpen = $state(false);
  let detailItem = $state<any>(null);
  let detailChartElem = $state<HTMLDivElement | null>(null);
  let detailChartInstance: echarts.ECharts | null = null;

  let editNameModalOpen = $state(false);
  let editingItem = $state<{ ID: string; Name: string; kind: string } | null>(null);
  let editingNameValue = $state("");

  let deleteModalOpen = $state(false);
  let deletingItem = $state<{ ID: string; Name: string; Address: string; kind: string } | null>(null);

  // Inline Sensor Analytics Chart
  let chartMetric = $state<string>("Temp");
  let inlineChartElem = $state<HTMLDivElement | null>(null);
  let inlineChartInstance: echarts.ECharts | null = null;

  const loadAll = async () => {
    internalLoading = true;
    try {
      const [e, p, m] = await Promise.all([
        fetchLogReport<EnvMonitorEnt>("envMonitor"),
        fetchLogReport<PowerMonitorEnt>("powerMonitor"),
        fetchLogReport<MotionSensorEnt>("motionSensor"),
      ]);
      envs = e;
      powers = p;
      motions = m;
    } catch (err) {
      console.error("Failed to load Environmental sensor reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      env: { kind: "envMonitor", name: $_("report.tabEnv") || "環境センサー" },
      power: { kind: "powerMonitor", name: $_("report.tabPower") || "電力センサー" },
      motion: { kind: "motionSensor", name: $_("report.tabMotion") || "人感センサー" },
    };
    const target = tabKindMap[activeTab];
    if (
      !confirm(
        $_("report.confirmClearItem", { values: { name: target.name } }) ||
          `${target.name}データを全消去しますか？`
      )
    ) {
      return;
    }
    try {
      await resetLogReport(target.kind);
      await loadAll();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  }

  onMount(() => {
    loadAll();

    const handleResize = () => {
      detailChartInstance?.resize();
      inlineChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      detailChartInstance?.dispose();
      inlineChartInstance?.dispose();
    };
  });

  const handleTabChange = (tab: TabType) => {
    activeTab = tab;
    currentPage = 1;
    sortColumn = "LastTime";
    sortDirection = "desc";
    if (tab === "env") chartMetric = "Temp";
    else if (tab === "power") chartMetric = "Load";
    else if (tab === "motion") chartMetric = "Moving";
  };

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const toTimestampMs = (t: number | string | Date | undefined | null): number => {
    if (!t) return 0;
    if (t instanceof Date) return t.getTime();
    const num = Number(t);
    if (isNaN(num) || num <= 0) return 0;
    if (num > 1e16) return Math.floor(num / 1e6); // nanoseconds (1.7e18) -> ms
    if (num > 1e13) return Math.floor(num / 1e3); // microseconds (1.7e15) -> ms
    if (num > 1e10) return num;                   // milliseconds (1.7e12)
    return num * 1000;                            // seconds (1.7e9) -> ms
  };

  const isPersistentAddress = (addr: string): boolean => {
    if (!addr) return false;
    return addr.startsWith("NAME:") || addr.includes(":TYPE:");
  };

  const getDeviceLegendLabel = (item: any): string => {
    const isPersist = isPersistentAddress(item.Address);
    if (isPersist) {
      if (item.Name) return item.Name;
      const match = item.Address.match(/^NAME:(.*?):TYPE:(.*)$/);
      if (match && match[1]) return match[1];
      return item.Address;
    }
    return item.Name ? `${item.Name} (${item.Address})` : (item.Address || "-");
  };

  const getLatestRSSI = (item: any): number => {
    if (activeTab === "env" && item.EnvData && item.EnvData.length > 0) {
      return item.EnvData[item.EnvData.length - 1].RSSI;
    }
    if ((activeTab === "power" || activeTab === "motion") && item.Data && item.Data.length > 0) {
      return item.Data[item.Data.length - 1].RSSI;
    }
    return -100;
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

  // Filtered & Sorted items per tab
  const filteredEnvList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = envs;
    if (q) {
      res = res.filter(
        (e) =>
          e.Name?.toLowerCase().includes(q) ||
          e.Address?.toLowerCase().includes(q) ||
          e.Host?.toLowerCase().includes(q)
      );
    }
    return [...res].sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];
      if (sortColumn === "RSSI") {
        valA = getLatestRSSI(a);
        valB = getLatestRSSI(b);
      } else if (sortColumn === "Temp") {
        valA = a.EnvData?.[a.EnvData.length - 1]?.Temp ?? -999;
        valB = b.EnvData?.[b.EnvData.length - 1]?.Temp ?? -999;
      } else if (sortColumn === "Humidity") {
        valA = a.EnvData?.[a.EnvData.length - 1]?.Humidity ?? -999;
        valB = b.EnvData?.[b.EnvData.length - 1]?.Humidity ?? -999;
      } else if (sortColumn === "DataCount") {
        valA = a.EnvData?.length ?? 0;
        valB = b.EnvData?.length ?? 0;
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

  const filteredPowerList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = powers;
    if (q) {
      res = res.filter(
        (p) =>
          p.Name?.toLowerCase().includes(q) ||
          p.Address?.toLowerCase().includes(q) ||
          p.Host?.toLowerCase().includes(q)
      );
    }
    return [...res].sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];
      if (sortColumn === "RSSI") {
        valA = getLatestRSSI(a);
        valB = getLatestRSSI(b);
      } else if (sortColumn === "Load") {
        valA = a.Data?.[a.Data.length - 1]?.Load ?? 0;
        valB = b.Data?.[b.Data.length - 1]?.Load ?? 0;
      } else if (sortColumn === "DataCount") {
        valA = a.Data?.length ?? 0;
        valB = b.Data?.length ?? 0;
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

  const filteredMotionList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = motions;
    if (q) {
      res = res.filter(
        (m) =>
          m.Name?.toLowerCase().includes(q) ||
          m.Address?.toLowerCase().includes(q) ||
          m.Host?.toLowerCase().includes(q)
      );
    }
    return [...res].sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];
      if (sortColumn === "RSSI") {
        valA = getLatestRSSI(a);
        valB = getLatestRSSI(b);
      } else if (sortColumn === "DataCount") {
        valA = a.Data?.length ?? 0;
        valB = b.Data?.length ?? 0;
      } else if (sortColumn === "LastMove") {
        valA = a.Data?.[a.Data.length - 1]?.LastMove ?? 0;
        valB = b.Data?.[b.Data.length - 1]?.LastMove ?? 0;
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

  const currentTotal = $derived.by(() => {
    if (activeTab === "env") return filteredEnvList.length;
    if (activeTab === "power") return filteredPowerList.length;
    return filteredMotionList.length;
  });

  const paginatedEnvList = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredEnvList.slice(start, start + pageSize);
  });

  const paginatedPowerList = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredPowerList.slice(start, start + pageSize);
  });

  const paginatedMotionList = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredMotionList.slice(start, start + pageSize);
  });

  // Action: Open Edit Name Modal
  const openEditNameModal = (item: any) => {
    const kind =
      activeTab === "env" ? "envMonitor" : activeTab === "power" ? "powerMonitor" : "motionSensor";
    editingItem = { ID: item.ID, Name: item.Name || "", kind };
    editingNameValue = item.Name || "";
    editNameModalOpen = true;
  };

  const handleSaveName = async () => {
    if (!editingItem) return;
    try {
      await updateLogReportName(editingItem.kind, editingItem.ID, editingNameValue);
      editNameModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert($_("report.sensorUpdateError") || "名前の変更に失敗しました: " + (err.message || err));
    }
  };

  // Action: Open Single Delete Modal
  const openDeleteModal = (item: any) => {
    const kind =
      activeTab === "env" ? "envMonitor" : activeTab === "power" ? "powerMonitor" : "motionSensor";
    deletingItem = { ID: item.ID, Name: item.Name || "", Address: item.Address || "", kind };
    deleteModalOpen = true;
  };

  const handleConfirmDelete = async () => {
    if (!deletingItem) return;
    try {
      await deleteLogReportItem(deletingItem.kind, deletingItem.ID);
      deleteModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert($_("report.sensorDeleteError") || "削除に失敗しました: " + (err.message || err));
    }
  };

  // Action: Open Detail Modal
  const openDetailModal = async (item: any) => {
    detailItem = item;
    detailModalOpen = true;
    await tick();
    renderDetailChart();
  };

  const renderDetailChart = () => {
    if (!detailChartElem || !detailItem) return;
    if (detailChartInstance) {
      detailChartInstance.dispose();
    }
    detailChartInstance = echarts.init(detailChartElem);
    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    if (activeTab === "env") {
      const dataList: EnvDataEnt[] = detailItem.EnvData || [];
      const times = dataList.map((d) =>
        formatTimeStr(d.Time, "{MM}/{dd} {HH}:{mm}")
      );
      const temps = dataList.map((d) => d.Temp);
      const hums = dataList.map((d) => d.Humidity);
      const co2s = dataList.map((d) => d.ECo2);

      const tempLabel = `${$_("report.sensorTemp")} (℃)`;
      const humLabel = `${$_("report.sensorHumidity")} (%)`;
      const co2Label = `${$_("report.sensorECo2")} (ppm)`;

      detailChartInstance.setOption({
        tooltip: { trigger: "axis" },
        legend: {
          data: [tempLabel, humLabel, co2Label],
          textStyle: { color: textColor, fontSize: 11 },
          top: 0,
        },
        grid: { left: "4%", right: "4%", bottom: "10%", top: 36, containLabel: true },
        xAxis: {
          type: "category",
          data: times,
          axisLabel: { color: textColor, fontSize: 11 },
          axisLine: { lineStyle: { color: splitLineColor } },
        },
        yAxis: [
          {
            type: "value",
            name: `${$_("report.sensorTemp")} / ${$_("report.sensorHumidity")}`,
            nameTextStyle: { color: textColor, fontSize: 10 },
            axisLabel: { color: textColor },
            splitLine: { lineStyle: { color: splitLineColor } },
          },
          {
            type: "value",
            name: co2Label,
            nameTextStyle: { color: textColor, fontSize: 10 },
            axisLabel: { color: textColor },
            splitLine: { show: false },
          },
        ],
        series: [
          {
            name: tempLabel,
            type: "line",
            smooth: true,
            data: temps,
            itemStyle: { color: "#f97316" },
          },
          {
            name: humLabel,
            type: "line",
            smooth: true,
            data: hums,
            itemStyle: { color: "#06b6d4" },
          },
          {
            name: co2Label,
            type: "line",
            smooth: true,
            yAxisIndex: 1,
            data: co2s,
            itemStyle: { color: "#8b5cf6" },
          },
        ],
      });
    } else if (activeTab === "power") {
      const dataList: PowerMonitorDataEnt[] = detailItem.Data || [];
      const times = dataList.map((d) =>
        formatTimeStr(d.Time, "{MM}/{dd} {HH}:{mm}")
      );
      const loads = dataList.map((d) => d.Load);
      const powerLabel = `${$_("report.sensorPower")} (W)`;

      detailChartInstance.setOption({
        tooltip: { trigger: "axis" },
        legend: {
          data: [powerLabel],
          textStyle: { color: textColor, fontSize: 11 },
          top: 0,
        },
        grid: { left: "4%", right: "4%", bottom: "10%", top: 36, containLabel: true },
        xAxis: {
          type: "category",
          data: times,
          axisLabel: { color: textColor, fontSize: 11 },
          axisLine: { lineStyle: { color: splitLineColor } },
        },
        yAxis: {
          type: "value",
          name: "W",
          nameTextStyle: { color: textColor, fontSize: 10 },
          axisLabel: { color: textColor },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        series: [
          {
            name: powerLabel,
            type: "line",
            smooth: true,
            areaStyle: { opacity: 0.15 },
            data: loads,
            itemStyle: { color: "#eab308" },
          },
        ],
      });
    } else if (activeTab === "motion") {
      const dataList: MotionSensorDataEnt[] = detailItem.Data || [];
      const times = dataList.map((d) =>
        formatTimeStr(d.Time, "{MM}/{dd} {HH}:{mm}")
      );
      const movings = dataList.map((d) => (d.Moving ? 1 : 0));
      const lights = dataList.map((d) => (d.Light ? 1 : 0));
      const batteries = dataList.map((d) => d.Battery);

      const movingLabel = `${$_("report.sensorMoving")} (0/1)`;
      const lightLabel = `${$_("report.sensorLight")} (0/1)`;
      const batteryLabel = `${$_("report.sensorBattery")} (%)`;

      detailChartInstance.setOption({
        tooltip: {
          trigger: "axis",
          formatter: (params: any) => {
            if (!params || params.length === 0) return "";
            let res = `<div class="font-bold border-b border-slate-700 pb-1 mb-1 text-xs">${params[0].axisValueLabel || ""}</div>`;
            params.forEach((p: any) => {
              let valDisplay = p.value;
              if (p.seriesName === movingLabel) {
                valDisplay = p.value === 1 ? `1 (${$_("report.sensorDetected")})` : `0 (${$_("report.sensorNotDetected")})`;
              } else if (p.seriesName === lightLabel) {
                valDisplay = p.value === 1 ? `1 (${$_("report.sensorLightBright")})` : `0 (${$_("report.sensorLightDark")})`;
              } else if (p.seriesName === batteryLabel) {
                valDisplay = `${p.value} %`;
              }
              res += `<div class="flex items-center gap-2 text-xs">
                <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background-color:${p.color}"></span>
                <span>${p.seriesName}:</span>
                <span class="font-semibold">${valDisplay}</span>
              </div>`;
            });
            return res;
          },
        },
        legend: {
          data: [movingLabel, lightLabel, batteryLabel],
          textStyle: { color: textColor, fontSize: 11 },
          top: 0,
        },
        grid: { left: "4%", right: "4%", bottom: "10%", top: 36, containLabel: true },
        xAxis: {
          type: "category",
          data: times,
          axisLabel: { color: textColor, fontSize: 11 },
          axisLine: { lineStyle: { color: splitLineColor } },
        },
        yAxis: [
          {
            type: "value",
            name: `${$_("report.sensorMoving")} / ${$_("report.sensorLight")}`,
            min: 0,
            max: 1.2,
            interval: 1,
            nameTextStyle: { color: textColor, fontSize: 10 },
            axisLabel: {
              color: textColor,
              formatter: (val: number) => (val === 1 ? "1" : val === 0 ? "0" : ""),
            },
            splitLine: { lineStyle: { color: splitLineColor } },
          },
          {
            type: "value",
            name: "%",
            max: 100,
            min: 0,
            nameTextStyle: { color: textColor, fontSize: 10 },
            axisLabel: { color: textColor },
            splitLine: { show: false },
          },
        ],
        series: [
          {
            name: movingLabel,
            type: "line",
            step: "end",
            data: movings,
            itemStyle: { color: "#f43f5e" },
            areaStyle: { opacity: 0.15 },
          },
          {
            name: lightLabel,
            type: "line",
            step: "end",
            data: lights,
            itemStyle: { color: "#eab308" },
          },
          {
            name: batteryLabel,
            type: "line",
            smooth: true,
            yAxisIndex: 1,
            data: batteries,
            itemStyle: { color: "#10b981" },
          },
        ],
      });
    }
  };

  // Export CSV for single sensor's recorded data points
  const exportDetailCSV = () => {
    if (!detailItem) return;
    let csv = "";
    if (activeTab === "env") {
      csv = "記録日時,気温(℃),湿度(%),照度(lx),気圧(hPa),騒音(dB),ETVOC(ppb),CO2(ppm),電池残量(%),RSSI\n";
      (detailItem.EnvData || []).forEach((d: EnvDataEnt) => {
        const time = formatTimeStr(d.Time, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}");
        csv += `"${time}",${d.Temp},${d.Humidity},${d.Illuminance},${d.BarometricPressure},${d.Sound},${d.ETVOC},${d.ECo2},${d.Battery},${d.RSSI}\n`;
      });
    } else if (activeTab === "power") {
      csv = "記録日時,電力(W),スイッチ,過負荷,RSSI\n";
      (detailItem.Data || []).forEach((d: PowerMonitorDataEnt) => {
        const time = formatTimeStr(d.Time, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}");
        csv += `"${time}",${d.Load},${d.Switch},${d.Over},${d.RSSI}\n`;
      });
    } else if (activeTab === "motion") {
      csv = "記録日時,検知,明暗,電池(%),最終検知日時,最終検知差(秒),RSSI\n";
      (detailItem.Data || []).forEach((d: MotionSensorDataEnt) => {
        const time = formatTimeStr(d.Time, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}");
        const lastMove = formatTimeStr(d.LastMove, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}");
        csv += `"${time}",${d.Moving ? "はい" : "いいえ"},${d.Light ? "明るい" : "暗い"},${d.Battery},"${lastMove}",${d.LastMoveDiff},${d.RSSI}\n`;
      });
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_sensor_${detailItem.Address || detailItem.ID}_${Date.now()}.csv`;
    link.click();
  };

  const envMetrics = $derived([
    { key: "Temp", label: `${$_("report.sensorTemp")} (℃)` },
    { key: "Humidity", label: `${$_("report.sensorHumidity")} (%)` },
    { key: "Illuminance", label: `${$_("report.sensorIlluminance")} (lx)` },
    { key: "BarometricPressure", label: `${$_("report.sensorPressure")} (hPa)` },
    { key: "Sound", label: `${$_("report.sensorSound")} (dB)` },
    { key: "ETVOC", label: `${$_("report.sensorETVOC")} (ppb)` },
    { key: "ECo2", label: `${$_("report.sensorECo2")} (ppm)` },
    { key: "Battery", label: `${$_("report.sensorBattery")} (%)` },
    { key: "RSSI", label: `${$_("report.sensorSignal")} (dBm)` },
  ]);

  const powerMetrics = $derived([
    { key: "Load", label: `${$_("report.sensorPower")} (W)` },
    { key: "RSSI", label: `${$_("report.sensorSignal")} (dBm)` },
  ]);

  const motionMetrics = $derived([
    { key: "Moving", label: `${$_("report.sensorMoving")} (0/1)` },
    { key: "Light", label: `${$_("report.sensorLight")} (0/1)` },
    { key: "Battery", label: `${$_("report.sensorBattery")} (%)` },
    { key: "LastMoveDiff", label: `${$_("report.sensorLastMoveDiff")} (sec)` },
    { key: "RSSI", label: `${$_("report.sensorSignal")} (dBm)` },
  ]);

  const renderInlineChart = () => {
    if (!inlineChartElem) return;
    if (!inlineChartInstance) {
      inlineChartInstance = echarts.init(inlineChartElem);
    }
    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const series: any[] = [];
    const legendData: string[] = [];

    if (activeTab === "env") {
      filteredEnvList.forEach((item) => {
        if (!item.EnvData || item.EnvData.length === 0) return;
        const name = getDeviceLegendLabel(item);
        legendData.push(name);
        const data = item.EnvData.map((d: any) => [
          toTimestampMs(d.Time),
          d[chartMetric],
        ]);
        series.push({
          name,
          type: "line",
          smooth: true,
          showSymbol: false,
          data,
        });
      });
    } else if (activeTab === "power") {
      filteredPowerList.forEach((item) => {
        if (!item.Data || item.Data.length === 0) return;
        const name = getDeviceLegendLabel(item);
        legendData.push(name);
        const data = item.Data.map((d: any) => [
          toTimestampMs(d.Time),
          chartMetric === "RSSI" ? d.RSSI : d.Load,
        ]);
        series.push({
          name,
          type: "line",
          smooth: true,
          showSymbol: false,
          data,
        });
      });
    } else if (activeTab === "motion") {
      filteredMotionList.forEach((item) => {
        if (!item.Data || item.Data.length === 0) return;
        const name = getDeviceLegendLabel(item);
        legendData.push(name);
        const data = item.Data.map((d: any) => [
          toTimestampMs(d.Time),
          chartMetric === "Moving"
            ? (d.Moving ? 1 : 0)
            : chartMetric === "Light"
            ? (d.Light ? 1 : 0)
            : chartMetric === "RSSI"
            ? d.RSSI
            : chartMetric === "LastMoveDiff"
            ? d.LastMoveDiff
            : d.Battery,
        ]);
        series.push({
          name,
          type: "line",
          step: (chartMetric === "Moving" || chartMetric === "Light") ? "end" : false,
          smooth: !(chartMetric === "Moving" || chartMetric === "Light"),
          showSymbol: false,
          data,
        });
      });
    }

    const yAxisName = (() => {
      if (activeTab === "env") {
        return envMetrics.find((m) => m.key === chartMetric)?.label || chartMetric;
      } else if (activeTab === "power") {
        return powerMetrics.find((m) => m.key === chartMetric)?.label || chartMetric;
      } else {
        return motionMetrics.find((m) => m.key === chartMetric)?.label || chartMetric;
      }
    })();

    inlineChartInstance.setOption(
      {
        tooltip: {
          trigger: "axis",
          formatter: (params: any) => {
            if (!params || params.length === 0) return "";
            let dateStr = "";
            if (params[0].value && params[0].value[0]) {
              dateStr = formatTimeStr(
                params[0].value[0],
                "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}"
              );
            }
            let res = `<div class="font-bold border-b border-slate-700 pb-1 mb-1 text-xs">${dateStr}</div>`;
            params.forEach((p: any) => {
              let val =
                p.value && p.value[1] !== null && p.value[1] !== undefined
                  ? p.value[1]
                  : "-";
              if (activeTab === "motion" && val !== "-") {
                if (chartMetric === "Moving") {
                  val = val === 1 ? `1 (${$_("report.sensorDetected")})` : `0 (${$_("report.sensorNotDetected")})`;
                } else if (chartMetric === "Light") {
                  val = val === 1 ? `1 (${$_("report.sensorLightBright")})` : `0 (${$_("report.sensorLightDark")})`;
                }
              }
              res += `<div class="flex items-center gap-2 text-xs">
                <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background-color:${p.color}"></span>
                <span class="truncate max-w-[200px]">${p.seriesName}:</span>
                <span class="font-semibold">${val}</span>
              </div>`;
            });
            return res;
          },
        },
        legend: {
          type: "scroll",
          data: legendData,
          textStyle: { color: textColor, fontSize: 11 },
          top: 0,
        },
        grid: {
          left: "3%",
          right: "4%",
          bottom: "16%",
          top: legendData.length > 0 ? 36 : 20,
          containLabel: true,
        },
        dataZoom: [
          { type: "inside" },
          { type: "slider", bottom: 2, height: 18 },
        ],
        xAxis: {
          type: "time",
          axisLabel: { color: textColor, fontSize: 10 },
          axisLine: { lineStyle: { color: splitLineColor } },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        yAxis: {
          type: "value",
          name: yAxisName,
          nameTextStyle: { color: textColor, fontSize: 10 },
          axisLabel: { color: textColor, fontSize: 10 },
          splitLine: { lineStyle: { color: splitLineColor } },
        },
        series,
      },
      true
    );
  };

  $effect(() => {
    const _t = activeTab;
    const _m = chartMetric;
    const _e = filteredEnvList;
    const _p = filteredPowerList;
    const _mo = filteredMotionList;
    if (inlineChartElem) {
      tick().then(() => renderInlineChart());
    }
  });

  const kpiStats = $derived.by(() => {
    // 1. Env Stats
    const envCount = envs.length;
    let tempSum = 0;
    let tempCount = 0;
    let humSum = 0;
    let humCount = 0;

    for (const e of envs) {
      if (e.EnvData && e.EnvData.length > 0) {
        const last = e.EnvData[e.EnvData.length - 1];
        if (last.Temp !== undefined && last.Temp !== null) {
          tempSum += last.Temp;
          tempCount++;
        }
        if (last.Humidity !== undefined && last.Humidity !== null) {
          humSum += last.Humidity;
          humCount++;
        }
      }
    }
    const avgTemp = tempCount > 0 ? tempSum / tempCount : null;
    const avgHum = humCount > 0 ? humSum / humCount : null;

    // 2. Power Stats
    const powerCount = powers.length;
    let totalLoad = 0;
    let onPowers = 0;
    let overPowers = 0;
    for (const p of powers) {
      if (p.Data && p.Data.length > 0) {
        const last = p.Data[p.Data.length - 1];
        if (last.Load !== undefined && last.Load !== null) {
          totalLoad += last.Load;
        }
        if (last.Switch) onPowers++;
        if (last.Over) overPowers++;
      }
    }

    // 3. Motion Stats
    const motionCount = motions.length;
    let movingCount = 0;
    let lightCount = 0;
    for (const m of motions) {
      if (m.Data && m.Data.length > 0) {
        const last = m.Data[m.Data.length - 1];
        if (last.Moving) movingCount++;
        if (last.Light) lightCount++;
      }
    }

    // 4. Host & Global Totals
    const hosts = new Set<string>();
    let totalPackets = 0;
    for (const e of envs) {
      if (e.Host) hosts.add(e.Host);
      totalPackets += e.Count || 0;
    }
    for (const p of powers) {
      if (p.Host) hosts.add(p.Host);
      totalPackets += p.Count || 0;
    }
    for (const m of motions) {
      if (m.Host) hosts.add(m.Host);
      totalPackets += m.Count || 0;
    }

    return {
      envCount,
      avgTemp,
      avgHum,
      powerCount,
      totalLoad,
      onPowers,
      overPowers,
      motionCount,
      movingCount,
      lightCount,
      totalSensors: envCount + powerCount + motionCount,
      hostCount: hosts.size,
      totalPackets,
    };
  });

  // Export CSV for active tab's table
  export function exportCSV(): void {
    let csv = "";
    if (activeTab === "env") {
      csv = "アドレス,名前,送信元ホスト,信号レベル(dBm),気温(℃),湿度(%),照度(lx),気圧(hPa),騒音(dB),ETVOC(ppb),CO2(ppm),電池残量(%),データ数,受信回数,初回日時,最終日時\n";
      filteredEnvList.forEach((e) => {
        const last = e.EnvData?.[e.EnvData.length - 1];
        csv += `"${e.Address}","${e.Name || ""}","${e.Host || ""}",${last?.RSSI ?? ""},${last?.Temp ?? ""},${last?.Humidity ?? ""},${last?.Illuminance ?? ""},${last?.BarometricPressure ?? ""},${last?.Sound ?? ""},${last?.ETVOC ?? ""},${last?.ECo2 ?? ""},${last?.Battery ?? ""},${e.EnvData?.length || 0},${e.Count || 0},"${formatTimeStr(e.FirstTime)}","${formatTimeStr(e.LastTime)}"\n`;
      });
    } else if (activeTab === "power") {
      csv = "アドレス,名前,送信元ホスト,信号レベル(dBm),電力(W),スイッチ,過負荷,データ数,受信回数,初回日時,最終日時\n";
      filteredPowerList.forEach((p) => {
        const last = p.Data?.[p.Data.length - 1];
        csv += `"${p.Address}","${p.Name || ""}","${p.Host || ""}",${last?.RSSI ?? ""},${last?.Load ?? ""},${last?.Switch ?? ""},${last?.Over ?? ""},${p.Data?.length || 0},${p.Count || 0},"${formatTimeStr(p.FirstTime)}","${formatTimeStr(p.LastTime)}"\n`;
      });
    } else if (activeTab === "motion") {
      csv = "アドレス,名前,送信元ホスト,信号レベル(dBm),人感検知,明暗,電池(%),最終検知日時,データ数,受信回数,初回日時,最終日時\n";
      filteredMotionList.forEach((m) => {
        const last = m.Data?.[m.Data.length - 1];
        csv += `"${m.Address}","${m.Name || ""}","${m.Host || ""}",${last?.RSSI ?? ""},${last?.Moving ? "はい" : "いいえ"},${last?.Light ? "明るい" : "暗い"},${last?.Battery ?? ""},"${last?.LastMove ? formatTimeStr(last.LastMove) : ""}",${m.Data?.length || 0},${m.Count || 0},"${formatTimeStr(m.FirstTime)}","${formatTimeStr(m.LastTime)}"\n`;
      });
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_sensor_report_${activeTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Header Title & Subtitle -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Thermometer class="w-5 h-5 text-cyan-500" />
        {$_("report.tabSensor")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        {$_("report.sensorSubtitle")}
      </p>
    </div>
  </div>

  <!-- KPI Summary Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Total & Hosts -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3.5">
      <div class="p-2.5 rounded-xl bg-purple-50 dark:bg-purple-950/50 border border-purple-200 dark:border-purple-800/60 text-purple-600 dark:text-purple-400 shrink-0">
        <Radio class="w-5 h-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.sensorKpiTotal")}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100 mt-0.5">
          {kpiStats.totalSensors} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[11px] text-slate-400 truncate mt-0.5">
          {$_("report.sensorKpiHosts", { values: { hosts: kpiStats.hostCount, packets: kpiStats.totalPackets } })}
        </div>
      </div>
    </div>

    <!-- Env Sensors KPI -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3.5">
      <div class="p-2.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/50 border border-cyan-200 dark:border-cyan-800/60 text-cyan-600 dark:text-cyan-400 shrink-0">
        <Thermometer class="w-5 h-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.tabEnv")}
        </div>
        <div class="text-xl font-bold font-mono text-cyan-600 dark:text-cyan-400 mt-0.5">
          {kpiStats.envCount} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[11px] text-slate-400 truncate mt-0.5">
          {#if kpiStats.avgTemp !== null && kpiStats.avgHum !== null}
            {$_("report.sensorKpiAvgEnv", { values: { temp: kpiStats.avgTemp.toFixed(1), hum: kpiStats.avgHum.toFixed(1) } })}
          {:else}
            {$_("report.sensorNoData")}
          {/if}
        </div>
      </div>
    </div>

    <!-- Power Sensors KPI -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3.5">
      <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/50 border border-amber-200 dark:border-amber-800/60 text-amber-600 dark:text-amber-400 shrink-0">
        <Zap class="w-5 h-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.tabPower")}
        </div>
        <div class="text-xl font-bold font-mono text-amber-600 dark:text-amber-400 mt-0.5">
          {kpiStats.totalLoad.toFixed(1)} <span class="text-xs font-normal text-slate-400">W</span>
        </div>
        <div class="text-[11px] text-slate-400 truncate mt-0.5">
          {$_("report.sensorKpiPowerSub", { values: { on: kpiStats.onPowers, over: kpiStats.overPowers } })}
        </div>
      </div>
    </div>

    <!-- Motion Sensors KPI -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3.5">
      <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/50 border border-emerald-200 dark:border-emerald-800/60 text-emerald-600 dark:text-emerald-400 shrink-0">
        <Footprints class="w-5 h-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider truncate">
          {$_("report.tabMotion")}
        </div>
        <div class="text-xl font-bold font-mono text-emerald-600 dark:text-emerald-400 mt-0.5">
          {kpiStats.motionCount} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[11px] text-slate-400 truncate mt-0.5">
          {$_("report.sensorKpiMotionSub", { values: { moving: kpiStats.movingCount, light: kpiStats.lightCount } })}
        </div>
      </div>
    </div>
  </div>

  <!-- Tab Switcher -->
  <div class="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2 overflow-x-auto">
    <button
      type="button"
      onclick={() => handleTabChange("env")}
      class={`flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-semibold transition-colors shrink-0 cursor-pointer ${
        activeTab === "env"
          ? "bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20 shadow-sm"
          : "text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60"
      }`}
    >
      <Thermometer class="w-4 h-4" />
      <span>{$_("report.tabEnv")}</span>
      <span class="ml-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
        {envs.length}
      </span>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("power")}
      class={`flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-semibold transition-colors shrink-0 cursor-pointer ${
        activeTab === "power"
          ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20 shadow-sm"
          : "text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60"
      }`}
    >
      <Zap class="w-4 h-4" />
      <span>{$_("report.tabPower")}</span>
      <span class="ml-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
        {powers.length}
      </span>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("motion")}
      class={`flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-semibold transition-colors shrink-0 cursor-pointer ${
        activeTab === "motion"
          ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 shadow-sm"
          : "text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60"
      }`}
    >
      <Footprints class="w-4 h-4" />
      <span>{$_("report.tabMotion")}</span>
      <span class="ml-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
        {motions.length}
      </span>
    </button>
  </div>

  <!-- Inline Time-Series Chart Section -->
  <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 p-4 shadow-sm space-y-3">
    <div class="flex items-center justify-between gap-3 flex-wrap">
      <div class="flex items-center gap-2">
        <LineChart class="w-4 h-4 text-cyan-500" />
        <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100">
          {$_("report.sensorTimeSeriesGraph")} - {activeTab === "env" ? $_("report.tabEnv") : activeTab === "power" ? $_("report.tabPower") : $_("report.tabMotion")}
        </h3>
      </div>

      <div class="flex items-center gap-2">
        <span class="text-xs font-medium text-slate-500">{$_("report.sensorDisplayMetric")}</span>
        <select
          bind:value={chartMetric}
          class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-slate-100 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 text-slate-800 dark:text-slate-200 cursor-pointer"
        >
          {#if activeTab === "env"}
            {#each envMetrics as m}
              <option value={m.key}>{m.label}</option>
            {/each}
          {:else if activeTab === "power"}
            {#each powerMetrics as m}
              <option value={m.key}>{m.label}</option>
            {/each}
          {:else if activeTab === "motion"}
            {#each motionMetrics as m}
              <option value={m.key}>{m.label}</option>
            {/each}
          {/if}
        </select>
      </div>
    </div>

    <div
      bind:this={inlineChartElem}
      class="w-full h-72 rounded-lg bg-slate-50/50 dark:bg-slate-950/40"
    ></div>
  </div>

  <!-- Data Table Container -->
  <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 shadow-sm overflow-hidden flex flex-col">
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

            {#if activeTab === "env"}
              <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Temp")}>
                <div class="flex items-center gap-1">
                  <span>{$_("report.sensorTemp")} (℃)</span>
                  {#if sortColumn === "Temp"}
                    {sortDirection === "asc" ? "▲" : "▼"}
                  {:else}
                    <ArrowUpDown class="w-3 h-3 opacity-30" />
                  {/if}
                </div>
              </th>
              <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Humidity")}>
                <div class="flex items-center gap-1">
                  <span>{$_("report.sensorHumidity")} (%)</span>
                  {#if sortColumn === "Humidity"}
                    {sortDirection === "asc" ? "▲" : "▼"}
                  {:else}
                    <ArrowUpDown class="w-3 h-3 opacity-30" />
                  {/if}
                </div>
              </th>
              <th class="px-3 py-2.5">{$_("report.sensorLastVal")}</th>
            {:else if activeTab === "power"}
              <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Load")}>
                <div class="flex items-center gap-1">
                  <span>{$_("report.sensorPower")} (W)</span>
                  {#if sortColumn === "Load"}
                    {sortDirection === "asc" ? "▲" : "▼"}
                  {:else}
                    <ArrowUpDown class="w-3 h-3 opacity-30" />
                  {/if}
                </div>
              </th>
              <th class="px-3 py-2.5">{$_("report.sensorSwitch")}</th>
              <th class="px-3 py-2.5">{$_("report.sensorOverload")}</th>
            {:else if activeTab === "motion"}
              <th class="px-3 py-2.5">{$_("report.sensorMoving")}</th>
              <th class="px-3 py-2.5">{$_("report.sensorLight")}</th>
              <th class="px-3 py-2.5">{$_("report.sensorBattery")}</th>
              <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("LastMove")}>
                <div class="flex items-center gap-1">
                  <span>{$_("report.sensorLastMoveTime")}</span>
                  {#if sortColumn === "LastMove"}
                    {sortDirection === "asc" ? "▲" : "▼"}
                  {:else}
                    <ArrowUpDown class="w-3 h-3 opacity-30" />
                  {/if}
                </div>
              </th>
            {/if}

            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("DataCount")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorDataCount")}</span>
                {#if sortColumn === "DataCount"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
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
          {#if activeTab === "env"}
            {#if paginatedEnvList.length === 0}
              <tr>
                <td colspan="12" class="text-center py-12 text-slate-400">
                  <div class="flex flex-col items-center justify-center gap-2">
                    <Thermometer class="w-8 h-8 opacity-20 text-slate-500" />
                    <p class="text-sm">{$_("report.sensorNoData") || "センサーデータはありません"}</p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each paginatedEnvList as item}
                {@const lastRSSI = getLatestRSSI(item)}
                {@const rssiBadge = getRSSIBadge(lastRSSI)}
                {@const latestEnv = item.EnvData?.[item.EnvData.length - 1]}
                <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/40 transition-colors group">
                  <td class="px-2 py-1 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-medium border ${rssiBadge.color}`}>
                      <Signal class="w-3 h-3" />
                      {rssiBadge.label}
                    </span>
                  </td>
                  <td class="px-2 py-1 font-mono text-[11px] font-medium text-slate-900 dark:text-slate-100 whitespace-nowrap">
                    <button
                      type="button"
                      onclick={() => openDetailModal(item)}
                      class="hover:underline hover:text-cyan-500 text-left cursor-pointer"
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
                  <td class="px-2 py-1 font-medium text-slate-800 dark:text-slate-200 max-w-[160px] truncate">
                    <div class="flex items-center gap-1.5">
                      <span class="truncate">{item.Name || "-"}</span>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-cyan-500 transition-opacity p-0.5 cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3 h-3" />
                      </button>
                    </div>
                  </td>
                  <td class="px-2 py-1 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Host || "-"}
                  </td>
                  <td class="px-2 py-1 font-semibold text-orange-500 dark:text-orange-400 whitespace-nowrap">
                    {latestEnv?.Temp !== undefined ? `${latestEnv.Temp} ℃` : "-"}
                  </td>
                  <td class="px-2 py-1 font-semibold text-cyan-500 dark:text-cyan-400 whitespace-nowrap">
                    {latestEnv?.Humidity !== undefined ? `${latestEnv.Humidity} %` : "-"}
                  </td>
                  <td class="px-2 py-1 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    <div class="flex items-center gap-2">
                      {#if latestEnv?.Illuminance}
                        <span title={$_("report.sensorIlluminance")}>☀️ {latestEnv.Illuminance} lx</span>
                      {/if}
                      {#if latestEnv?.ECo2}
                        <span title={$_("report.sensorECo2")}>🌱 {latestEnv.ECo2} ppm</span>
                      {/if}
                      {#if latestEnv?.Battery}
                        <span title={$_("report.sensorBattery")}>🔋 {latestEnv.Battery}%</span>
                      {/if}
                    </div>
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.EnvData?.length || 0}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Count || 0}
                  </td>
                  <td class="px-2 py-1 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                    {formatTimeStr(item.LastTime)}
                  </td>
                  <td class="px-2 py-1 text-right whitespace-nowrap">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onclick={() => openDetailModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-cyan-500 transition-colors cursor-pointer"
                        title={$_("report.sensorDetailAndChart")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openDeleteModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 transition-colors cursor-pointer"
                        title={$_("common.delete")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            {/if}
          {:else if activeTab === "power"}
            {#if paginatedPowerList.length === 0}
              <tr>
                <td colspan="12" class="text-center py-12 text-slate-400">
                  <div class="flex flex-col items-center justify-center gap-2">
                    <Zap class="w-8 h-8 opacity-20 text-slate-500" />
                    <p class="text-sm">{$_("report.sensorNoData") || "センサーデータはありません"}</p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each paginatedPowerList as item}
                {@const lastRSSI = getLatestRSSI(item)}
                {@const rssiBadge = getRSSIBadge(lastRSSI)}
                {@const latestPower = item.Data?.[item.Data.length - 1]}
                <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/40 transition-colors group">
                  <td class="px-2 py-1 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-medium border ${rssiBadge.color}`}>
                      <Signal class="w-3 h-3" />
                      {rssiBadge.label}
                    </span>
                  </td>
                  <td class="px-2 py-1 font-mono text-[11px] font-medium text-slate-900 dark:text-slate-100 whitespace-nowrap">
                    <button
                      type="button"
                      onclick={() => openDetailModal(item)}
                      class="hover:underline hover:text-amber-500 text-left cursor-pointer"
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
                  <td class="px-2 py-1 font-medium text-slate-800 dark:text-slate-200 max-w-[160px] truncate">
                    <div class="flex items-center gap-1.5">
                      <span class="truncate">{item.Name || "-"}</span>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-amber-500 transition-opacity p-0.5 cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3 h-3" />
                      </button>
                    </div>
                  </td>
                  <td class="px-2 py-1 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Host || "-"}
                  </td>
                  <td class="px-2 py-1 font-bold text-amber-500 dark:text-amber-400 whitespace-nowrap">
                    {latestPower?.Load !== undefined ? `${latestPower.Load} W` : "-"}
                  </td>
                  <td class="px-2 py-1 whitespace-nowrap">
                    {#if latestPower?.Switch}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-500 border border-emerald-500/20">
                        <ToggleRight class="w-3 h-3" /> ON
                      </span>
                    {:else}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-slate-500/10 text-slate-400 border border-slate-500/20">
                        <ToggleLeft class="w-3 h-3" /> OFF
                      </span>
                    {/if}
                  </td>
                  <td class="px-2 py-1 whitespace-nowrap">
                    {#if latestPower?.Over}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-rose-500/10 text-rose-500 border border-rose-500/20">
                        <AlertTriangle class="w-3 h-3" /> {$_("report.sensorOverloadState")}
                      </span>
                    {:else}
                      <span class="text-slate-400 text-xs">{$_("report.sensorNormalState")}</span>
                    {/if}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Data?.length || 0}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Count || 0}
                  </td>
                  <td class="px-2 py-1 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                    {formatTimeStr(item.LastTime)}
                  </td>
                  <td class="px-2 py-1 text-right whitespace-nowrap">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onclick={() => openDetailModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors cursor-pointer"
                        title={$_("report.sensorDetailAndChart")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openDeleteModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 transition-colors cursor-pointer"
                        title={$_("common.delete")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            {/if}
          {:else if activeTab === "motion"}
            {#if paginatedMotionList.length === 0}
              <tr>
                <td colspan="12" class="text-center py-12 text-slate-400">
                  <div class="flex flex-col items-center justify-center gap-2">
                    <Footprints class="w-8 h-8 opacity-20 text-slate-500" />
                    <p class="text-sm">{$_("report.sensorNoData") || "センサーデータはありません"}</p>
                  </div>
                </td>
              </tr>
            {:else}
              {#each paginatedMotionList as item}
                {@const lastRSSI = getLatestRSSI(item)}
                {@const rssiBadge = getRSSIBadge(lastRSSI)}
                {@const latestMotion = item.Data?.[item.Data.length - 1]}
                <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/40 transition-colors group">
                  <td class="px-2 py-1 whitespace-nowrap">
                    <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-medium border ${rssiBadge.color}`}>
                      <Signal class="w-3 h-3" />
                      {rssiBadge.label}
                    </span>
                  </td>
                  <td class="px-2 py-1 font-mono text-[11px] font-medium text-slate-900 dark:text-slate-100 whitespace-nowrap">
                    <button
                      type="button"
                      onclick={() => openDetailModal(item)}
                      class="hover:underline hover:text-emerald-500 text-left cursor-pointer"
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
                  <td class="px-2 py-1 font-medium text-slate-800 dark:text-slate-200 max-w-[160px] truncate">
                    <div class="flex items-center gap-1.5">
                      <span class="truncate">{item.Name || "-"}</span>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-emerald-500 transition-opacity p-0.5 cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3 h-3" />
                      </button>
                    </div>
                  </td>
                  <td class="px-2 py-1 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Host || "-"}
                  </td>
                  <td class="px-2 py-1 whitespace-nowrap">
                    {#if latestMotion?.Moving}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold bg-rose-500/10 text-rose-500 border border-rose-500/20 animate-pulse">
                        <Footprints class="w-3 h-3" /> {$_("report.sensorDetected")}
                      </span>
                    {:else}
                      <span class="text-slate-400 text-xs">{$_("report.sensorNotDetected")}</span>
                    {/if}
                  </td>
                  <td class="px-2 py-1 whitespace-nowrap">
                    {#if latestMotion?.Light}
                      <span class="text-amber-500 text-xs flex items-center gap-1">
                        <Sun class="w-3 h-3" /> {$_("report.sensorLightBright")}
                      </span>
                    {:else}
                      <span class="text-slate-400 text-xs">{$_("report.sensorLightDark")}</span>
                    {/if}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-300 whitespace-nowrap">
                    {latestMotion?.Battery !== undefined ? `${latestMotion.Battery}%` : "-"}
                  </td>
                  <td class="px-2 py-1 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                    {latestMotion?.LastMove ? formatTimeStr(latestMotion.LastMove) : "-"}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Data?.length || 0}
                  </td>
                  <td class="px-2 py-1 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                    {item.Count || 0}
                  </td>
                  <td class="px-2 py-1 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                    {formatTimeStr(item.LastTime)}
                  </td>
                  <td class="px-2 py-1 text-right whitespace-nowrap">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onclick={() => openDetailModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-emerald-500 transition-colors cursor-pointer"
                        title={$_("report.sensorDetailAndChart")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openEditNameModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors cursor-pointer"
                        title={$_("report.sensorEditName")}
                      >
                        <Pencil class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={() => openDeleteModal(item)}
                        class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 transition-colors cursor-pointer"
                        title={$_("common.delete")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
            {/if}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    {#if currentTotal > 0}
      <div class="p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/30">
        <ReportPagination
          totalCount={currentTotal}
          bind:pageSize
          bind:currentPage
        />
      </div>
    {/if}
  </div>
</div>

<!-- Sensor Detail Modal -->
{#if detailModalOpen && detailItem}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-[90vw] max-h-[90vh] flex flex-col overflow-hidden">
      <!-- Modal Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-500">
            <Thermometer class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
              {$_("report.sensorDetailTitle")} - {detailItem.Name || detailItem.Address}
            </h3>
            <p class="text-xs text-slate-500 font-mono mt-0.5">
              {detailItem.Address} | {$_("report.sensorHost")}: {detailItem.Host || "localhost"}
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
        <!-- Sensor Meta Info Table -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800">
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorAddress")}</div>
            <div class="font-mono font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailItem.Address}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorName")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailItem.Name || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorSignal")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{getLatestRSSI(detailItem)} dBm</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorDataCount")}</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">
              {activeTab === "env" ? (detailItem.EnvData?.length || 0) : (detailItem.Data?.length || 0)}
            </div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorFirst")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{formatTimeStr(detailItem.FirstTime)}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">{$_("report.sensorLast")}</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{formatTimeStr(detailItem.LastTime)}</div>
          </div>
        </div>

        <!-- History Chart -->
        <div>
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 mb-2 flex items-center gap-2">
            <LineChart class="w-4 h-4 text-cyan-500" />
            {$_("report.sensorTimeSeriesGraph")}
          </h4>
          <div
            bind:this={detailChartElem}
            class="w-full h-64 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40"
          ></div>
        </div>

        <!-- Recorded Data Points Table -->
        <div>
          <div class="flex items-center justify-between mb-2">
            <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              <Clock class="w-4 h-4 text-cyan-500" />
              {$_("report.sensorDataTitle")}
            </h4>
            <button
              type="button"
              onclick={exportDetailCSV}
              class="inline-flex items-center gap-1 px-2.5 py-1 rounded text-xs font-medium border border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
            >
              <Download class="w-3 h-3" />
              {$_("report.btnExportCsv")}
            </button>
          </div>

          <div class="max-h-60 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800">
            <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
              <thead class="bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 uppercase tracking-wider sticky top-0 border-b border-slate-200 dark:border-slate-800">
                {#if activeTab === "env"}
                  <tr>
                    <th class="px-3 py-2">{$_("report.sensorRecordTime")}</th>
                    <th class="px-3 py-2">{$_("report.sensorTemp")}(℃)</th>
                    <th class="px-3 py-2">{$_("report.sensorHumidity")}(%)</th>
                    <th class="px-3 py-2">{$_("report.sensorIlluminance")}(lx)</th>
                    <th class="px-3 py-2">{$_("report.sensorPressure")}(hPa)</th>
                    <th class="px-3 py-2">{$_("report.sensorSound")}(dB)</th>
                    <th class="px-3 py-2">{$_("report.sensorETVOC")}(ppb)</th>
                    <th class="px-3 py-2">{$_("report.sensorECo2")}(ppm)</th>
                    <th class="px-3 py-2">{$_("report.sensorBattery")}(%)</th>
                    <th class="px-3 py-2">{$_("report.sensorSignal")}</th>
                  </tr>
                {:else if activeTab === "power"}
                  <tr>
                    <th class="px-3 py-2">{$_("report.sensorRecordTime")}</th>
                    <th class="px-3 py-2">{$_("report.sensorPower")}(W)</th>
                    <th class="px-3 py-2">{$_("report.sensorSwitch")}</th>
                    <th class="px-3 py-2">{$_("report.sensorOverload")}</th>
                    <th class="px-3 py-2">{$_("report.sensorSignal")}</th>
                  </tr>
                {:else if activeTab === "motion"}
                  <tr>
                    <th class="px-3 py-2">{$_("report.sensorRecordTime")}</th>
                    <th class="px-3 py-2">{$_("report.sensorMoving")}</th>
                    <th class="px-3 py-2">{$_("report.sensorLight")}</th>
                    <th class="px-3 py-2">{$_("report.sensorBattery")}(%)</th>
                    <th class="px-3 py-2">{$_("report.sensorLastMoveTime")}</th>
                    <th class="px-3 py-2">{$_("report.sensorSignal")}</th>
                  </tr>
                {/if}
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
                {#if activeTab === "env"}
                  {#each (detailItem.EnvData || []).slice().reverse() as d}
                    <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                      <td class="px-2 py-1 font-mono text-[11px]">{formatTimeStr(d.Time)}</td>
                      <td class="px-2 py-1 font-semibold text-orange-500">{d.Temp}</td>
                      <td class="px-2 py-1 font-semibold text-cyan-500">{d.Humidity}</td>
                      <td class="px-2 py-1">{d.Illuminance}</td>
                      <td class="px-2 py-1">{d.BarometricPressure}</td>
                      <td class="px-2 py-1">{d.Sound}</td>
                      <td class="px-2 py-1">{d.ETVOC}</td>
                      <td class="px-2 py-1">{d.ECo2}</td>
                      <td class="px-2 py-1">{d.Battery}</td>
                      <td class="px-2 py-1 font-mono">{d.RSSI}</td>
                    </tr>
                  {/each}
                {:else if activeTab === "power"}
                  {#each (detailItem.Data || []).slice().reverse() as d}
                    <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                      <td class="px-2 py-1 font-mono text-[11px]">{formatTimeStr(d.Time)}</td>
                      <td class="px-2 py-1 font-semibold text-amber-500">{d.Load}</td>
                      <td class="px-2 py-1">{d.Switch ? "ON" : "OFF"}</td>
                      <td class="px-2 py-1">{d.Over ? $_("report.sensorOverloadState") : $_("report.sensorNormalState")}</td>
                      <td class="px-2 py-1 font-mono">{d.RSSI}</td>
                    </tr>
                  {/each}
                {:else if activeTab === "motion"}
                  {#each (detailItem.Data || []).slice().reverse() as d}
                    <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                      <td class="px-2 py-1 font-mono text-[11px]">{formatTimeStr(d.Time)}</td>
                      <td class="px-2 py-1">{d.Moving ? $_("report.sensorYes") : $_("report.sensorNo")}</td>
                      <td class="px-2 py-1">{d.Light ? $_("report.sensorLightBright") : $_("report.sensorLightDark")}</td>
                      <td class="px-2 py-1">{d.Battery}%</td>
                      <td class="px-2 py-1 font-mono text-[11px]">{d.LastMove ? formatTimeStr(d.LastMove) : "-"}</td>
                      <td class="px-2 py-1 font-mono">{d.RSSI}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30 flex justify-end">
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="px-4 py-2 rounded-lg text-xs font-semibold bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-800 dark:text-slate-100 transition-colors cursor-pointer"
        >
          {$_("common.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Name Modal -->
{#if editNameModalOpen && editingItem}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Pencil class="w-4 h-4 text-cyan-500" />
          {$_("report.sensorEditNameTitle")}
        </h3>
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-2">
        <label for="sensor-name-input" class="block text-xs font-semibold text-slate-600 dark:text-slate-300">
          {$_("report.sensorName")}
        </label>
        <input
          id="sensor-name-input"
          type="text"
          bind:value={editingNameValue}
          placeholder={$_("report.sensorNamePlaceholder")}
          class="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-xs text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-cyan-500/50"
        />
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleSaveName}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-cyan-600 hover:bg-cyan-500 text-white shadow-sm transition-colors cursor-pointer"
        >
          {$_("common.save")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteModalOpen && deletingItem}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-500 shrink-0">
          <Trash2 class="w-5 h-5" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            {$_("report.sensorDeleteConfirmTitle")}
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            {$_("report.sensorDeleteConfirmMsg")}
          </p>
        </div>
      </div>

      <div class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-lg text-xs space-y-1 font-mono text-slate-700 dark:text-slate-300">
        <div>{$_("report.sensorAddress")}: <span class="font-bold">{deletingItem.Address}</span></div>
        {#if deletingItem.Name}
          <div>{$_("report.sensorName")}: <span class="font-bold">{deletingItem.Name}</span></div>
        {/if}
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (deleteModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleConfirmDelete}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-rose-600 hover:bg-rose-500 text-white shadow-sm transition-colors cursor-pointer"
        >
          {$_("common.delete")}
        </button>
      </div>
    </div>
  </div>
{/if}
