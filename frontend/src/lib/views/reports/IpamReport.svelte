<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _, locale } from "svelte-i18n";
  import { get } from "svelte/store";
  import * as echarts from "echarts";
  import {
    Network,
    FolderTree,
    Layers,
    CheckCircle2,
    Activity,
    Grid,
    ChevronRight,
    CornerUpLeft,
  } from "@lucide/svelte";
  import { isDarkMode } from "../../charts/utils";
  import { getVendor } from "./utils";
  import type { IPAMReportResp, IPAMRangeEnt } from "../../api";

  let {
    searchQuery = "",
    ipamReport = { Ranges: [], TotalRanges: 0, TotalSize: 0, TotalUsed: 0, TotalUsage: 0 },
    nodes = [],
    arpList = [],
    allDevices = [],
  }: {
    searchQuery?: string;
    ipamReport?: IPAMReportResp;
    nodes?: any[];
    arpList?: any[];
    allDevices?: any[];
  } = $props();

  let selectedRangeIndex = $state<number>(0);
  let selectedSubnetBlock = $state<string | null>(null);
  let selectedHostInfo = $state<any | null>(null);

  let ipamChartElem = $state<HTMLElement | null>(null);
  let ipamChartInstance: echarts.ECharts | null = null;

  const effectiveDevices = $derived.by(() => {
    if (allDevices && allDevices.length > 0) return allDevices;
    const list: any[] = [];
    for (const n of nodes) {
      if (n.ip) list.push({ ip: n.ip, name: n.name, mac: n.mac || "", vendor: n.vendor || "", isManaged: true });
    }
    for (const a of arpList) {
      if (a.IP && !list.some(d => d.ip === a.IP)) {
        list.push({ ip: a.IP, name: a.Vendor || a.IP, mac: a.MAC || "", vendor: a.Vendor || "", isManaged: false });
      }
    }
    return list;
  });

  const currentRange = $derived.by<IPAMRangeEnt | null>(() => {
    if (ipamReport.Ranges.length === 0) return null;
    const idx = Math.min(Math.max(0, selectedRangeIndex), ipamReport.Ranges.length - 1);
    return ipamReport.Ranges[idx] || null;
  });

  const isLargeRange = $derived.by(() => {
    return currentRange ? currentRange.Size > 256 : false;
  });

  const activeSubnetPrefix = $derived.by(() => {
    if (selectedSubnetBlock) {
      return selectedSubnetBlock.replace(/\.0\/24$/, "");
    }
    if (currentRange && currentRange.Size <= 256) {
      const parts = currentRange.StartIP.split(".");
      if (parts.length === 4) {
        return `${parts[0]}.${parts[1]}.${parts[2]}`;
      }
    }
    return "";
  });

  // Map of 1..254 host entries for the currently viewed /24 subnet
  const currentSubnetHosts = $derived.by(() => {
    const prefix = activeSubnetPrefix;
    const hostMap = new Map<number, any>();
    if (!prefix) return { prefix: "", hostMap };

    for (const d of effectiveDevices) {
      if (d.ip && d.ip.startsWith(prefix + ".")) {
        const last = parseInt(d.ip.substring(prefix.length + 1), 10);
        if (!isNaN(last) && last >= 1 && last <= 254) {
          hostMap.set(last, d);
        }
      }
    }
    return { prefix, hostMap };
  });

  const renderIPAMHeatmap = () => {
    if (!ipamChartElem || ipamReport.Ranges.length === 0) return;
    if (ipamChartInstance) {
      ipamChartInstance.dispose();
    }
    const isDark = isDarkMode();
    ipamChartInstance = echarts.init(ipamChartElem, isDark ? "dark" : undefined);
    const yData = ipamReport.Ranges.map((r) => r.Range).reverse();
    const xData: number[] = [];
    for (let i = 0; i < 100; i++) xData.push(i);

    const seriesData: [number, number, number][] = [];
    let maxVal = 1;
    for (let y = 0; y < ipamReport.Ranges.length; y++) {
      const r = ipamReport.Ranges[ipamReport.Ranges.length - 1 - y];
      for (let x = 0; x < 100; x++) {
        const val = r.UsedIP ? r.UsedIP[x] || 0 : 0;
        seriesData.push([x, y, val]);
        if (val > maxVal) maxVal = val;
      }
    }

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        position: "top",
        formatter: (params: any) => {
          const val = params.value;
          const rangeName = yData[val[1]];
          const isJa = (get(locale) || "ja").startsWith("ja");
          return `<b>${rangeName}</b><br/>${isJa ? "相対位置: " : "Relative Pos: "}${val[0]}% 〜 ${val[0] + 1}%<br/>${isJa ? "使用中ホスト: " : "Active Hosts: "}${val[2]}${isJa ? " 件" : ""}`;
        },
      },
      grid: {
        left: 110,
        right: 80,
        top: 20,
        bottom: 26,
      },
      xAxis: {
        type: "category",
        data: xData.map((x) => `${x}%`),
        axisLabel: { color: isDark ? "#94a3b8" : "#64748b", fontSize: 9, interval: 9 },
        axisLine: { lineStyle: { color: isDark ? "#334155" : "#cbd5e1" } },
        splitLine: { show: false },
      },
      yAxis: {
        type: "category",
        data: yData,
        axisLabel: { color: isDark ? "#cbd5e1" : "#334155", fontSize: 10 },
        axisLine: { lineStyle: { color: isDark ? "#334155" : "#cbd5e1" } },
        splitLine: { show: false },
      },
      visualMap: {
        min: 0,
        max: Math.max(1, maxVal),
        calculable: true,
        orient: "vertical",
        right: 12,
        top: "middle",
        itemWidth: 10,
        itemHeight: 90,
        text: (get(locale) || "ja").startsWith("ja") ? ["高 (ホスト)", "低"] : ["High", "Low"],
        textGap: 8,
        textStyle: { color: isDark ? "#94a3b8" : "#475569", fontSize: 9 },
        inRange: {
          color: isDark
            ? [
                "#091e3a",
                "#0e3a6c",
                "#0284c7",
                "#10b981",
                "#eab308",
                "#f43f5e",
              ]
            : [
                "#e2e8f0",
                "#93c5fd",
                "#38bdf8",
                "#10b981",
                "#f59e0b",
                "#ef4444",
              ],
        },
      },
      series: [
        {
          name: (get(locale) || "ja").startsWith("ja") ? "IP利用密度" : "IP Utilization Density",
          type: "heatmap",
          data: seriesData,
          emphasis: {
            itemStyle: {
              borderColor: "#38bdf8",
              borderWidth: 1.5,
            },
          },
          progressive: 1000,
          animation: false,
        },
      ],
    };

    ipamChartInstance.setOption(option);
    ipamChartInstance.on("click", (params: any) => {
      if (params.value && Array.isArray(params.value)) {
        const yIdx = params.value[1];
        const clickedRange = yData[yIdx];
        const origIdx = ipamReport.Ranges.findIndex((r) => r.Range === clickedRange);
        if (origIdx !== -1) {
          selectedRangeIndex = origIdx;
          selectedSubnetBlock = null;
        }
      }
    });
  };

  $effect(() => {
    if (ipamReport.Ranges.length > 0 && ipamChartElem) {
      tick().then(() => {
        renderIPAMHeatmap();
      });
    }
  });

  onMount(() => {
    const handleResize = () => {
      if (ipamChartInstance) ipamChartInstance.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      if (ipamChartInstance) ipamChartInstance.dispose();
    };
  });

  const getUsageBadgeColor = (usage: number) => {
    if (usage < 50) return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
    if (usage < 80) return "bg-cyan-500/10 text-cyan-400 border-cyan-500/30";
    if (usage < 95) return "bg-amber-500/10 text-amber-400 border-amber-500/30";
    return "bg-rose-500/10 text-rose-400 border-rose-500/30";
  };

  const getUsageProgressBarColor = (usage: number) => {
    if (usage < 50) return "bg-emerald-500";
    if (usage < 80) return "bg-cyan-500";
    if (usage < 95) return "bg-amber-500";
    return "bg-rose-500";
  };

  export function exportCSV(): void {
    const csv = "Range,StartIP,EndIP,Size,Used,Usage(%)\n" + ipamReport.Ranges.map((r) => `"${r.Range}","${r.StartIP}","${r.EndIP}",${r.Size},${r.Used},${r.Usage.toFixed(2)}`).join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_ipam_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Network class="w-5 h-5 text-cyan-400" />
      {$_("report.ipamTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">
      {$_("report.ipamSubtitle")}
    </p>
  </div>

  <!-- KPI Summary Cards across all ranges -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
        <span>{$_("report.targetRangeCount")}</span>
        <FolderTree class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
      </div>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
        {ipamReport.TotalRanges} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitRanges")}</span>
      </div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">
        {currentRange ? $_("report.selectedRange", { values: { range: currentRange.Range } }) : $_("report.noSubnetDetected")}
      </div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
        <span>{$_("report.totalPoolAddresses")}</span>
        <Layers class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
      </div>
      <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
        {ipamReport.TotalSize.toLocaleString()} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitAddresses")}</span>
      </div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.poolSub")}</div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
        <span>{$_("report.usedIpCount")}</span>
        <CheckCircle2 class="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
      </div>
      <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
        {ipamReport.TotalUsed.toLocaleString()} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ {ipamReport.TotalSize.toLocaleString()}</span>
      </div>
      <div class="text-[10px] text-emerald-600 dark:text-emerald-400/80">{$_("report.usedSub")}</div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
        <span>{$_("report.avgUsageRate")}</span>
        <Activity class="w-4 h-4 text-cyan-600 dark:text-cyan-300" />
      </div>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-300">
        {ipamReport.TotalUsage.toFixed(1)} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">%</span>
      </div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">
        {$_("report.freeAddresses", { values: { count: (ipamReport.TotalSize - ipamReport.TotalUsed).toLocaleString() } })}
      </div>
    </div>
  </div>

  <!-- Section 1: ECharts Multi-Range 100-Slot Heatmap -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
    <div class="flex flex-wrap items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 gap-2">
      <div>
        <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Network class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
          {$_("report.heatmapDensityTitle")}
        </h3>
        <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
          {$_("report.heatmapDensityDesc")}
        </p>
      </div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">
        {$_("report.heatmapDensityHint")}
      </div>
    </div>

    {#if ipamReport.Ranges.length === 0}
      <div class="p-8 text-center text-slate-500 text-xs font-sans">
        {$_("report.noIpamRanges")}
      </div>
    {:else}
      <div bind:this={ipamChartElem} class="w-full h-48"></div>
    {/if}
  </div>

  <!-- Section 2: Subnets List Table -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 flex items-center justify-between">
      <span class="text-xs font-bold text-slate-800 dark:text-slate-200 flex items-center gap-2">
        <FolderTree class="w-4 h-4 text-cyan-400" />
        {$_("report.subnetListTitle", { values: { count: ipamReport.Ranges.length } })}
      </span>
      <span class="text-[11px] text-slate-400">{$_("report.subnetListHint")}</span>
    </div>

    <table class="w-full text-left text-xs border-collapse font-mono">
      <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
        <tr>
          <th class="py-2 px-3">{$_("report.colSelect")}</th>
          <th class="py-2 px-3">{$_("report.colRange")}</th>
          <th class="py-2 px-3">{$_("report.colStartIp")}</th>
          <th class="py-1 px-2">{$_("report.colEndIp")}</th>
          <th class="py-1 px-2 text-right">{$_("report.colSize")}</th>
          <th class="py-1 px-2 text-right">{$_("report.colUsed")}</th>
          <th class="py-1 px-2 w-48">{$_("report.colUsage")}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
        {#if ipamReport.Ranges.length === 0}
          <tr>
            <td colspan="7" class="p-6 text-center text-slate-500 font-sans">
              {$_("report.noRangesRegistered")}
            </td>
          </tr>
        {:else}
          {#each ipamReport.Ranges as r, idx}
            {@const isSelected = selectedRangeIndex === idx}
            <tr
              onclick={() => { selectedRangeIndex = idx; selectedSubnetBlock = null; }}
              class="cursor-pointer transition-colors {isSelected ? 'bg-cyan-950/40 text-white font-semibold' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'}"
            >
              <td class="py-1 px-2 text-center w-10">
                {#if isSelected}
                  <span class="h-2 w-2 rounded-full bg-cyan-400 inline-block animate-pulse"></span>
                {:else}
                  <span class="h-1.5 w-1.5 rounded-full bg-slate-700 inline-block"></span>
                {/if}
              </td>
              <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-bold flex items-center gap-1.5">
                <span>{r.Range}</span>
                {#if r.Size > 256}
                  <span class="rounded bg-indigo-100 dark:bg-indigo-950 border border-indigo-300 dark:border-indigo-800/70 px-1 text-[9px] text-indigo-700 dark:text-indigo-300 leading-none">
                    {$_("report.wideArea")}
                  </span>
                {/if}
              </td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-400">{r.StartIP}</td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-400">{r.EndIP}</td>
              <td class="py-1 px-2 text-right text-slate-800 dark:text-slate-200">{r.Size.toLocaleString()}</td>
              <td class="py-1 px-2 text-right text-emerald-600 dark:text-emerald-400 font-bold">{r.Used.toLocaleString()}</td>
              <td class="py-1 px-2">
                <div class="flex items-center gap-2.5">
                  <div class="flex-1 h-2 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                    <div
                      class="h-full rounded-full transition-all {getUsageProgressBarColor(r.Usage)}"
                      style="width: {Math.min(100, Math.max(r.Used > 0 ? 3 : 0, r.Usage))}%"
                    ></div>
                  </div>
                  <span class="text-[10px] w-12 text-right font-mono {r.Usage >= 90 ? 'text-rose-500 dark:text-rose-400 font-bold' : 'text-slate-600 dark:text-slate-300'}">
                    {r.Usage.toFixed(1)}%
                  </span>
                </div>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Section 3: Adaptive Drill-down Visual View -->
  {#if currentRange}
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-4">
      <!-- Navigation Header & Breadcrumb -->
      <div class="flex flex-wrap items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 gap-3">
        <div class="flex items-center gap-2">
          <Grid class="w-4 h-4 text-cyan-400" />
          <div class="flex items-center gap-1.5 text-xs font-bold text-slate-800 dark:text-slate-200">
            <button
              type="button"
              onclick={() => { selectedSubnetBlock = null; }}
              class="text-slate-400 hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_("report.rangeDetailHeader", { values: { range: currentRange.Range, size: currentRange.Size.toLocaleString() } })}
            </button>
            {#if selectedSubnetBlock}
              <ChevronRight class="w-3.5 h-3.5 text-slate-600" />
              <span class="text-cyan-400 font-mono">{selectedSubnetBlock} {$_("report.detailTag")}</span>
            {/if}
          </div>
        </div>

        <!-- Drill-down controls & legend -->
        <div class="flex items-center gap-4 text-[11px]">
          {#if selectedSubnetBlock}
            <button
              type="button"
              onclick={() => { selectedSubnetBlock = null; }}
              class="flex items-center gap-1 text-xs text-cyan-400 hover:text-cyan-300 font-semibold cursor-pointer rounded-lg bg-cyan-950/60 border border-cyan-800/60 px-2.5 py-1"
            >
              <CornerUpLeft class="w-3.5 h-3.5" />
              <span>{$_("report.btnBackToBlocks")}</span>
            </button>
          {/if}

          <div class="flex items-center gap-3 text-[10px] text-slate-400">
            <span class="flex items-center gap-1.5">
              <span class="h-2.5 w-2.5 rounded bg-emerald-500"></span> {$_("report.legendUsed")}
            </span>
            <span class="flex items-center gap-1.5">
              <span class="h-2.5 w-2.5 rounded bg-slate-200 dark:bg-slate-800 border border-slate-300 dark:border-slate-700"></span> {$_("report.legendFree")}
            </span>
            {#if searchQuery}
              <span class="flex items-center gap-1.5 text-amber-400">
                <span class="h-2.5 w-2.5 rounded bg-amber-400 animate-pulse"></span> {$_("report.legendSearchMatch")}
              </span>
            {/if}
          </div>
        </div>
      </div>

      <!-- CASE 1: Large Range (/16, etc.) Top-level /24 Block Heatmap -->
      {#if isLargeRange && !selectedSubnetBlock}
        <div class="space-y-2">
          <div class="flex items-center justify-between text-[11px] text-slate-400">
            <span>
              {$_("report.wideAreaHint")}
            </span>
            <span class="font-mono text-cyan-400 font-bold shrink-0">
              {currentRange.Subnets?.length || 0} {$_("report.unitBlocks")}
            </span>
          </div>

          <div class="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-8 lg:grid-cols-12 xl:grid-cols-16 gap-1.5 max-h-96 overflow-y-auto p-2.5 rounded-xl bg-slate-50/80 dark:bg-slate-950/80 border border-slate-200 dark:border-slate-800/80">
            {#each (currentRange.Subnets || []) as block}
              {@const hasUsed = block.Used > 0}
              {@const isQueryMatch = searchQuery && block.Subnet.toLowerCase().includes(searchQuery.toLowerCase())}
              <button
                type="button"
                onclick={() => { selectedSubnetBlock = block.Subnet; }}
                title={$_("report.blockTooltip", { values: { subnet: block.Subnet, used: block.Used, size: block.Size, usage: block.Usage.toFixed(1) } })}
                class="p-2 rounded-lg border text-left flex flex-col justify-between transition-all cursor-pointer hover:scale-105 hover:z-10 {isQueryMatch ? 'ring-2 ring-amber-400 border-amber-400 bg-amber-950/30' : hasUsed ? 'bg-white dark:bg-slate-900 border-cyan-200 dark:border-cyan-800/60 hover:border-cyan-400 shadow-xs' : 'bg-slate-100/60 dark:bg-slate-950/60 border-slate-200 dark:border-slate-800/80 hover:bg-slate-200 dark:hover:bg-slate-900 opacity-60'}"
              >
                <div class="text-[10px] font-mono font-bold truncate text-slate-800 dark:text-slate-200">
                  {block.Subnet.replace(/\.0\/24$/, "")}
                </div>
                <div class="flex items-center justify-between mt-1 text-[9px] font-mono">
                  <span class="{hasUsed ? 'text-emerald-400 font-bold' : 'text-slate-600'}">
                    {block.Used}
                  </span>
                  <span class="rounded px-1 py-0 text-[8px] {getUsageBadgeColor(block.Usage)}">
                    {block.Usage.toFixed(0)}%
                  </span>
                </div>
              </button>
            {/each}
          </div>
        </div>

      <!-- CASE 2: Single /24 Subnet or Drilled-Down /24 Host Grid (1..254) -->
      {:else}
        <div class="space-y-3">
          <div class="flex items-center justify-between text-[11px] text-slate-400">
            <span>
              {$_("report.subnetHostHeader", { values: { prefix: currentSubnetHosts.prefix } })}
            </span>
            <span class="text-[10px] font-mono">
              {$_("report.hostClickHint")}
            </span>
          </div>

          <div class="grid grid-cols-16 sm:grid-cols-32 gap-1 max-h-72 overflow-y-auto p-2 rounded-xl bg-slate-50/80 dark:bg-slate-950/80 border border-slate-200 dark:border-slate-800/80">
            {#each Array.from({ length: 254 }, (_, i) => i + 1) as hostNum}
              {@const host = currentSubnetHosts.hostMap.get(hostNum)}
              {@const isUsed = !!host}
              {@const hostIP = `${currentSubnetHosts.prefix}.${hostNum}`}
              {@const isQueryMatch = searchQuery && (hostIP.includes(searchQuery) || (host?.name && host.name.toLowerCase().includes(searchQuery.toLowerCase())) || (host?.mac && host.mac.toLowerCase().includes(searchQuery.toLowerCase())))}
              <button
                type="button"
                onclick={() => {
                  selectedHostInfo = isUsed
                    ? { ip: hostIP, name: host.name, mac: host.mac, vendor: host.vendor, state: host.state, isManaged: host.isManaged }
                    : { ip: hostIP, isFree: true };
                }}
                title="{hostIP} {isUsed ? `(${host.name || host.mac})` : `(${$_('report.legendFree')})`}"
                class="h-5 rounded text-[9px] flex items-center justify-center font-mono cursor-pointer transition-transform hover:scale-125 {isQueryMatch ? 'ring-2 ring-amber-400 font-bold' : ''} {isUsed ? 'bg-emerald-500 text-slate-950 font-bold shadow-xs shadow-emerald-500/50' : 'bg-slate-200 dark:bg-slate-800/60 text-slate-500 dark:text-slate-500 hover:bg-slate-300 dark:hover:bg-slate-700'}"
              >
                {hostNum}
              </button>
            {/each}
          </div>

          <!-- Selected Host Detail Modal / Panel -->
          {#if selectedHostInfo}
            <div class="rounded-xl border border-cyan-300 dark:border-cyan-800/60 bg-white/95 dark:bg-slate-950/90 p-3.5 shadow-md flex flex-wrap items-center justify-between gap-3 text-xs font-mono">
              <div class="flex items-center gap-3">
                <div class="h-3 w-3 rounded-full shrink-0 {selectedHostInfo.isFree ? 'bg-slate-700' : 'bg-emerald-400'}"></div>
                <div>
                  <span class="text-cyan-400 font-bold text-sm">{selectedHostInfo.ip}</span>
                  {#if selectedHostInfo.isFree}
                    <span class="ml-2 text-slate-400 font-sans text-xs">{$_("report.unassignedFreeIp")}</span>
                  {:else}
                    <span class="ml-2 text-slate-800 dark:text-slate-100 font-bold font-sans">{selectedHostInfo.name}</span>
                    <span class="ml-2 text-slate-500 dark:text-slate-400 text-[11px]">MAC: {selectedHostInfo.mac || "-"}</span>
                    <span class="ml-2 rounded bg-slate-100 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 px-1.5 py-0.5 text-[10px] text-slate-700 dark:text-slate-300 font-sans">
                      {selectedHostInfo.vendor || getVendor(selectedHostInfo.mac || "")}
                    </span>
                  {/if}
                </div>
              </div>
              <button
                type="button"
                onclick={() => { selectedHostInfo = null; }}
                class="rounded px-2 py-0.5 text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-200 dark:hover:bg-slate-800 text-[11px] font-sans cursor-pointer transition-colors"
              >
                {$_("report.close")}
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>
