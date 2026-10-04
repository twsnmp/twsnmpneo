<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Activity,
    CheckCircle2,
    AlertTriangle,
    PieChart,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { getStateColor, getStateName } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import type { PollingEnt, NodeEnt } from "../../api";

  let {
    searchQuery = "",
    pollings = [],
    nodes = [],
  }: {
    searchQuery?: string;
    pollings?: PollingEnt[];
    nodes?: NodeEnt[];
  } = $props();

  let pollingSubFilter = $state<"all" | "error" | "warn" | "warn_above">("all");
  let pollingSortColumn = $state("name");
  let pollingSortDirection = $state<"asc" | "desc">("asc");
  let pollingPageSize = $state(25);
  let pollingCurrentPage = $state(1);

  let pollingChartElem = $state<HTMLDivElement | null>(null);
  let pollingChartInstance: echarts.ECharts | null = null;

  const isPollingActive = (p: PollingEnt) => {
    const lvl = (p.level || "").toLowerCase();
    const st = (p.state || "").toLowerCase();
    return lvl !== "off" && st !== "off" && lvl !== "stop" && st !== "stop";
  };

  const activePollings = $derived(pollings.filter(isPollingActive));

  const isPollingError = (st: string) => {
    const s = (st || "").toLowerCase();
    return s === "high" || s === "low" || s === "error" || s === "down";
  };

  const isPollingWarn = (st: string) => {
    return (st || "").toLowerCase() === "warn";
  };

  const pollingStats = $derived.by(() => {
    const total = activePollings.length;
    const normal = activePollings.filter((p) => p.state === "normal" || p.state === "up").length;
    const warn = activePollings.filter((p) => isPollingWarn(p.state || "")).length;
    const error = activePollings.filter((p) => isPollingError(p.state || "")).length;
    const rate = total > 0 ? ((normal / total) * 100).toFixed(1) : "100.0";
    return { total, normal, warn, error, rate };
  });

  const nodeMap = $derived(new Map(nodes.map((n) => [n.id, n.name])));

  const filteredPollings = $derived(
    activePollings.filter((p) => {
      const q = searchQuery.toLowerCase();
      const nodeName = (nodeMap.get(p.node_id) || "").toLowerCase();
      const matchSearch =
        !q ||
        (p.name || "").toLowerCase().includes(q) ||
        (p.type || "").toLowerCase().includes(q) ||
        (p.params || p.target || "").toLowerCase().includes(q) ||
        nodeName.includes(q);

      if (!matchSearch) return false;

      if (pollingSubFilter === "error") {
        return isPollingError(p.state || "");
      }
      if (pollingSubFilter === "warn") {
        return isPollingWarn(p.state || "");
      }
      if (pollingSubFilter === "warn_above") {
        return isPollingWarn(p.state || "") || isPollingError(p.state || "");
      }
      return true;
    })
  );

  const handleSortPolling = (colKey: string) => {
    if (pollingSortColumn === colKey) {
      pollingSortDirection = pollingSortDirection === "asc" ? "desc" : "asc";
    } else {
      pollingSortColumn = colKey;
      pollingSortDirection = "asc";
    }
  };

  const sortedPollings = $derived(
    [...filteredPollings].sort((a: PollingEnt, b: PollingEnt) => {
      let valA: any = "";
      let valB: any = "";

      if (pollingSortColumn === "name") {
        valA = a.name || "";
        valB = b.name || "";
      } else if (pollingSortColumn === "type") {
        valA = a.type || "";
        valB = b.type || "";
      } else if (pollingSortColumn === "target") {
        valA = a.params || a.target || "";
        valB = b.params || b.target || "";
      } else if (pollingSortColumn === "node") {
        valA = nodeMap.get(a.node_id) || "";
        valB = nodeMap.get(b.node_id) || "";
      } else if (pollingSortColumn === "state") {
        valA = a.state || "";
        valB = b.state || "";
      } else if (pollingSortColumn === "last_val") {
        valA = a.last_val ?? "";
        valB = b.last_val ?? "";
      }

      let comparison = 0;
      if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }
      return pollingSortDirection === "asc" ? comparison : -comparison;
    })
  );

  const paginatedPollings = $derived.by(() => {
    if (pollingPageSize === -1) return sortedPollings;
    const start = (pollingCurrentPage - 1) * pollingPageSize;
    return sortedPollings.slice(start, start + pollingPageSize);
  });

  const renderPollingChart = () => {
    if (!pollingChartElem || activePollings.length === 0) return;
    if (pollingChartInstance) {
      pollingChartInstance.dispose();
    }
    const isDark = isDarkMode();
    pollingChartInstance = echarts.init(pollingChartElem, isDark ? "dark" : undefined);

    const stateCounts = new Map<string, number>();
    for (const p of activePollings) {
      const st = (p.state || "unknown").toLowerCase();
      stateCounts.set(st, (stateCounts.get(st) || 0) + 1);
    }

    const order = ["normal", "up", "warn", "low", "high", "error", "down", "info", "repair", "unknown"];
    const colorMap: Record<string, string> = {
      normal: "#10b981",
      up: "#10b981",
      warn: "#f59e0b",
      low: "#fb9a99",
      high: "#ef4444",
      error: "#ef4444",
      down: "#ef4444",
      info: "#06b6d4",
      repair: "#3b82f6",
      unknown: "#94a3b8",
    };

    const sortedKeys = Array.from(stateCounts.keys()).sort((a, b) => {
      let idxA = order.indexOf(a);
      let idxB = order.indexOf(b);
      if (idxA === -1) idxA = 99;
      if (idxB === -1) idxB = 99;
      return idxA - idxB;
    });

    const data = sortedKeys.map((st) => ({
      name: getStateName(st),
      value: stateCounts.get(st) || 0,
      itemStyle: { color: colorMap[st] || "#94a3b8" },
    }));

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        trigger: "item",
        formatter: "{b}: <b>{c}</b> ({d}%)",
      },
      legend: {
        orient: "vertical",
        right: 4,
        top: "middle",
        textStyle: {
          color: isDark ? "#cbd5e1" : "#475569",
          fontSize: 10,
        },
        itemWidth: 10,
        itemHeight: 10,
      },
      series: [
        {
          name: "Polling State",
          type: "pie",
          radius: ["45%", "75%"],
          center: ["38%", "50%"],
          avoidLabelOverlap: false,
          label: {
            show: false,
          },
          labelLine: {
            show: false,
          },
          data,
        },
      ],
    };

    pollingChartInstance.setOption(option);
  };

  $effect(() => {
    if (activePollings.length > 0 && pollingChartElem) {
      tick().then(() => renderPollingChart());
    }
  });

  onMount(() => {
    const handleResize = () => {
      if (pollingChartInstance) pollingChartInstance.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      if (pollingChartInstance) pollingChartInstance.dispose();
    };
  });

  export function exportCSV(): void {
    const csv = "Name,Type,Target,Node,State,LastVal\n" + sortedPollings.map((p) => `"${p.name}","${p.type}","${p.params || p.target || ''}","${nodeMap.get(p.node_id) || p.node_id || ''}","${p.state}","${p.last_val ?? ''}"`).join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_polling_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Activity class="w-5 h-5 text-cyan-400" />
      {$_("report.pollingTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">{$_("report.pollingSubtitle")}</p>
  </div>

  <!-- Top summary: KPI cards + Status distribution donut chart -->
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
    <!-- Left: 4 KPI Cards -->
    <div class="lg:col-span-7 grid grid-cols-1 sm:grid-cols-2 gap-3">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
        <div class="flex items-center justify-between">
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.totalPollings")}</span>
          <Activity class="w-4 h-4 text-cyan-500" />
        </div>
        <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
          {pollingStats.total} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400 truncate">
          {$_("report.pollingMonitoringSub")}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
        <div class="flex items-center justify-between">
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.serviceSla")}</span>
          <CheckCircle2 class="w-4 h-4 text-emerald-500" />
        </div>
        <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
          {pollingStats.rate} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">%</span>
        </div>
        <div class="text-[10px] text-slate-400 truncate">
          {$_("report.serviceSlaSub")}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
        <div class="flex items-center justify-between">
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.warnPollings")}</span>
          <AlertTriangle class="w-4 h-4 text-amber-500" />
        </div>
        <div class="text-2xl font-bold font-mono text-amber-600 dark:text-amber-400">
          {pollingStats.warn} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400 truncate">
          {$_("report.warnPollingsSub")}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
        <div class="flex items-center justify-between">
          <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.errorPollings")}</span>
          <AlertTriangle class="w-4 h-4 text-rose-500" />
        </div>
        <div class="text-2xl font-bold font-mono text-rose-600 dark:text-rose-400">
          {pollingStats.error} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400 truncate">
          {$_("report.errorPollingsSub")}
        </div>
      </div>
    </div>

    <!-- Right: Polling State Distribution Chart -->
    <div class="lg:col-span-5 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
      <div class="flex items-center justify-between pb-1 border-b border-slate-100 dark:border-slate-800/60">
        <div class="flex items-center gap-1.5 text-xs font-bold text-slate-800 dark:text-slate-200">
          <PieChart class="w-3.5 h-3.5 text-cyan-500" />
          <span>{$_("report.pollingDistribution")}</span>
        </div>
        <span class="text-[10px] font-mono text-slate-400">
          {activePollings.length} {$_("report.unitPollings")}
        </span>
      </div>
      <div bind:this={pollingChartElem} class="w-full h-36 min-h-[140px]"></div>
    </div>
  </div>

  <!-- Sub-filter buttons: All, Error, Caution, Caution or higher -->
  <div class="flex flex-wrap items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2">
    <button
      type="button"
      onclick={() => { pollingSubFilter = "all"; pollingCurrentPage = 1; }}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {pollingSubFilter === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Activity class="w-3.5 h-3.5 text-cyan-400" />
      <span>{$_("report.subAllPollings")} ({activePollings.length})</span>
    </button>
    <button
      type="button"
      onclick={() => { pollingSubFilter = "error"; pollingCurrentPage = 1; }}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {pollingSubFilter === 'error' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <AlertTriangle class="w-3.5 h-3.5 text-rose-400" />
      <span>{$_("report.subErrorPollings")} ({activePollings.filter(p => isPollingError(p.state || '')).length})</span>
    </button>
    <button
      type="button"
      onclick={() => { pollingSubFilter = "warn"; pollingCurrentPage = 1; }}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {pollingSubFilter === 'warn' ? 'bg-amber-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <AlertTriangle class="w-3.5 h-3.5 text-amber-400" />
      <span>{$_("report.subWarnOnlyPollings")} ({activePollings.filter(p => isPollingWarn(p.state || '')).length})</span>
    </button>
    <button
      type="button"
      onclick={() => { pollingSubFilter = "warn_above"; pollingCurrentPage = 1; }}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {pollingSubFilter === 'warn_above' ? 'bg-purple-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <AlertTriangle class="w-3.5 h-3.5 text-purple-400" />
      <span>{$_("report.subWarnPollings")} ({activePollings.filter(p => isPollingWarn(p.state || '') || isPollingError(p.state || '')).length})</span>
    </button>
  </div>

  <!-- Polling Table -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <table class="w-full text-left text-xs border-collapse font-mono">
      <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
        <tr>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("name")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colPollingName")}</span>
              {#if pollingSortColumn === "name"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("type")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colType")}</span>
              {#if pollingSortColumn === "type"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("target")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colTarget")}</span>
              {#if pollingSortColumn === "target"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("node")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colTargetNode")}</span>
              {#if pollingSortColumn === "node"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("state")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colResponseStatus")}</span>
              {#if pollingSortColumn === "state"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortPolling("last_val")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colLatestValue")}</span>
              {#if pollingSortColumn === "last_val"}
                {#if pollingSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
              {/if}
            </div>
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
        {#if sortedPollings.length === 0}
          <tr>
            <td colspan="6" class="p-6 text-center text-slate-500 font-sans">
              {$_("report.noPollings")}
            </td>
          </tr>
        {:else}
          {#each paginatedPollings as p}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{p.name}</td>
              <td class="py-1 px-2">
                <span class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[10px] font-sans text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 leading-none">
                  {p.type}
                </span>
              </td>
              <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px] truncate max-w-[180px]" title={p.params || p.target || ""}>
                {p.params || p.target || "-"}
              </td>
              <td class="py-1 px-2 font-sans text-slate-700 dark:text-slate-300 text-[11px]">
                {nodeMap.get(p.node_id) || p.node_id || "-"}
              </td>
              <td class="py-1 px-2">
                <span
                  class="rounded px-2 py-0.5 text-[10px] font-sans font-semibold leading-none border"
                  style="background-color: {getStateColor(p.state)}15; color: {getStateColor(p.state)}; border-color: {getStateColor(p.state)}30;"
                >
                  {getStateName(p.state)}
                </span>
              </td>
              <td class="py-1 px-2 font-mono text-cyan-600 dark:text-cyan-400 text-[11px]">
                {p.last_val !== undefined && p.last_val !== null ? p.last_val : "-"}
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>

    <ReportPagination
      bind:pageSize={pollingPageSize}
      bind:currentPage={pollingCurrentPage}
      totalCount={sortedPollings.length}
    />
  </div>
</div>
