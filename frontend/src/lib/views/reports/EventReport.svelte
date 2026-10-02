<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    FileText,
    Layers,
    ShieldAlert,
    Server,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { getStateColor, formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import type { EventLogEnt, NodeEnt } from "../../api";

  let {
    logs = [],
    nodes = [],
    searchQuery = "",
  }: {
    logs?: EventLogEnt[];
    nodes?: NodeEnt[];
    searchQuery?: string;
  } = $props();

  // Subtab navigation: type, level, node
  let subTab = $state<"type" | "level" | "node">("type");
  let sortColumn = $state("count");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // ECharts container & instance
  let chartElem = $state<HTMLDivElement | null>(null);
  let chartInstance: echarts.ECharts | null = null;

  // Node lookup map
  const nodeMap = $derived(new Map<string, NodeEnt>(nodes.map((n) => [n.id, n])));

  // SLA-aligned Severity Classification (LOW is Error, WARN is Warning)
  export const isEventError = (level: string) => {
    const l = (level || "").toLowerCase();
    return l === "high" || l === "low" || l === "error" || l === "emergency" || l === "down";
  };

  export const isEventWarn = (level: string) => {
    const l = (level || "").toLowerCase();
    return l === "warn";
  };

  export const isEventNormal = (level: string) => {
    return !isEventError(level) && !isEventWarn(level);
  };

  // Event Type Classification (polling, user, other)
  export const getEventTypeGroup = (type: string): "polling" | "user" | "other" => {
    const t = (type || "").toLowerCase().trim();
    if (t === "polling") return "polling";
    if (t === "user") return "user";
    return "other";
  };

  interface NormalizedLog {
    time: number;
    type: string;
    typeGroup: "polling" | "user" | "other";
    level: string;
    nodeId: string;
    nodeName: string;
    nodeIp: string;
    event: string;
  }

  // Normalize event logs
  const normalizedLogs = $derived.by<NormalizedLog[]>(() => {
    return logs.map((l) => {
      const time = l.time || l.Time || 0;
      const type = (l.type || l.Type || "system").trim() || "system";
      const level = (l.level || l.Level || "info").toLowerCase().trim() || "info";
      const nodeId = l.node_id || l.NodeID || "";
      const nodeEnt = nodeId ? nodeMap.get(nodeId) : undefined;
      const nodeName = l.node_name || l.NodeName || nodeEnt?.name || (nodeId ? nodeId : "");
      const nodeIp = nodeEnt?.ip || "";
      const event = l.event || l.Event || "";
      return {
        time,
        type,
        typeGroup: getEventTypeGroup(type),
        level,
        nodeId,
        nodeName,
        nodeIp,
        event,
      };
    });
  });

  // Filter logs by search query
  const filteredLogs = $derived.by<NormalizedLog[]>(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return normalizedLogs;
    return normalizedLogs.filter(
      (l) =>
        l.event.toLowerCase().includes(q) ||
        l.type.toLowerCase().includes(q) ||
        l.level.toLowerCase().includes(q) ||
        l.nodeName.toLowerCase().includes(q) ||
        l.nodeId.toLowerCase().includes(q) ||
        l.nodeIp.toLowerCase().includes(q)
    );
  });

  // KPI Overview Statistics (SLA-aligned: LOW is Error)
  const kpiStats = $derived.by(() => {
    const total = filteredLogs.length;
    let high = 0;
    let warn = 0;
    let normal = 0;
    const uniqueNodes = new Set<string>();
    const uniqueTypes = new Set<string>();

    for (const l of filteredLogs) {
      if (isEventError(l.level)) {
        high++;
      } else if (isEventWarn(l.level)) {
        warn++;
      } else {
        normal++;
      }
      if (l.nodeName || l.nodeId) {
        uniqueNodes.add(l.nodeName || l.nodeId);
      }
      uniqueTypes.add(l.type);
    }

    return {
      total,
      high,
      warn,
      normal,
      uniqueNodes: uniqueNodes.size,
      uniqueTypes: uniqueTypes.size,
    };
  });

  // 1. Group by Type Summary
  interface TypeSummary {
    type: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    nodeCount: number;
    lastTime: number;
  }

  const typeSummaries = $derived.by<TypeSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        nodes: Set<string>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.type);
      if (!item) {
        item = {
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          nodes: new Set<string>(),
          lastTime: 0,
        };
        map.set(l.type, item);
      }
      item.count++;
      if (isEventError(l.level)) {
        item.errorCount++;
      } else if (isEventWarn(l.level)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      if (l.nodeName || l.nodeId) {
        item.nodes.add(l.nodeName || l.nodeId);
      }
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([type, d]) => ({
      type,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      nodeCount: d.nodes.size,
      lastTime: d.lastTime,
    }));
  });

  // 2. Group by Level Summary (with polling, user, other columns)
  interface LevelSummary {
    level: string;
    count: number;
    percent: number;
    nodeCount: number;
    pollingCount: number;
    userCount: number;
    otherCount: number;
    lastTime: number;
  }

  const levelSummaries = $derived.by<LevelSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        count: number;
        nodes: Set<string>;
        pollingCount: number;
        userCount: number;
        otherCount: number;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.level);
      if (!item) {
        item = {
          count: 0,
          nodes: new Set<string>(),
          pollingCount: 0,
          userCount: 0,
          otherCount: 0,
          lastTime: 0,
        };
        map.set(l.level, item);
      }
      item.count++;
      if (l.nodeName || l.nodeId) {
        item.nodes.add(l.nodeName || l.nodeId);
      }
      if (l.typeGroup === "polling") {
        item.pollingCount++;
      } else if (l.typeGroup === "user") {
        item.userCount++;
      } else {
        item.otherCount++;
      }
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([level, d]) => ({
      level,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      nodeCount: d.nodes.size,
      pollingCount: d.pollingCount,
      userCount: d.userCount,
      otherCount: d.otherCount,
      lastTime: d.lastTime,
    }));
  });

  // 3. Group by Associated Node Summary (separate level & type columns, no latest event text)
  interface NodeSummary {
    nodeId: string;
    name: string;
    ip: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    pollingCount: number;
    userCount: number;
    otherCount: number;
    lastTime: number;
  }

  const nodeSummaries = $derived.by<NodeSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        nodeId: string;
        name: string;
        ip: string;
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        pollingCount: number;
        userCount: number;
        otherCount: number;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      const key = l.nodeId || l.nodeName || "__system__";
      let item = map.get(key);
      if (!item) {
        const name = l.nodeName || (l.nodeId ? l.nodeId : $_("report.systemUnassigned"));
        item = {
          nodeId: l.nodeId,
          name,
          ip: l.nodeIp || "-",
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          pollingCount: 0,
          userCount: 0,
          otherCount: 0,
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      if (isEventError(l.level)) {
        item.errorCount++;
      } else if (isEventWarn(l.level)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      if (l.typeGroup === "polling") {
        item.pollingCount++;
      } else if (l.typeGroup === "user") {
        item.userCount++;
      } else {
        item.otherCount++;
      }
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => ({
      nodeId: d.nodeId,
      name: d.name,
      ip: d.ip,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      pollingCount: d.pollingCount,
      userCount: d.userCount,
      otherCount: d.otherCount,
      lastTime: d.lastTime,
    }));
  });

  // Sort handler
  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection =
        colKey === "name" || colKey === "type" || colKey === "level" || colKey === "ip"
          ? "asc"
          : "desc";
    }
  };

  const handleSelectSubTab = (tab: "type" | "level" | "node") => {
    subTab = tab;
    currentPage = 1;
    sortColumn = "count";
    sortDirection = "desc";
  };

  // Sort & paginate by type
  const sortedTypeSummaries = $derived(
    [...typeSummaries].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      const comparison =
        typeof valA === "number" && typeof valB === "number"
          ? valA - valB
          : String(valA).localeCompare(String(valB));
      return sortDirection === "asc" ? comparison : -comparison;
    })
  );
  const paginatedTypeSummaries = $derived(
    pageSize === -1
      ? sortedTypeSummaries
      : sortedTypeSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by level
  const sortedLevelSummaries = $derived(
    [...levelSummaries].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      const comparison =
        typeof valA === "number" && typeof valB === "number"
          ? valA - valB
          : String(valA).localeCompare(String(valB));
      return sortDirection === "asc" ? comparison : -comparison;
    })
  );
  const paginatedLevelSummaries = $derived(
    pageSize === -1
      ? sortedLevelSummaries
      : sortedLevelSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by node
  const sortedNodeSummaries = $derived(
    [...nodeSummaries].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      const comparison =
        typeof valA === "number" && typeof valB === "number"
          ? valA - valB
          : String(valA).localeCompare(String(valB));
      return sortDirection === "asc" ? comparison : -comparison;
    })
  );
  const paginatedNodeSummaries = $derived(
    pageSize === -1
      ? sortedNodeSummaries
      : sortedNodeSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  const currentTotalCount = $derived(
    subTab === "type"
      ? sortedTypeSummaries.length
      : subTab === "level"
      ? sortedLevelSummaries.length
      : sortedNodeSummaries.length
  );

  // Chart Rendering
  const renderChart = () => {
    if (!chartElem) return;
    if (!chartInstance) {
      chartInstance = echarts.init(chartElem);
    }
    const dark = isDarkMode();
    const textColor = dark ? "#94a3b8" : "#475569";

    let option: echarts.EChartsOption = {};

    if (subTab === "type") {
      const topTypes = [...typeSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topTypes.map((t) => ({ name: t.type, value: t.count }));
      option = {
        tooltip: {
          trigger: "item",
          formatter: "{b}: {c} ({d}%)",
        },
        series: [
          {
            type: "pie",
            radius: ["42%", "72%"],
            center: ["50%", "50%"],
            avoidLabelOverlap: true,
            itemStyle: {
              borderRadius: 4,
              borderColor: dark ? "#0f172a" : "#ffffff",
              borderWidth: 2,
            },
            label: {
              show: false,
            },
            emphasis: {
              label: {
                show: true,
                fontSize: 10,
                fontWeight: "bold",
                color: textColor,
              },
            },
            data: data.length > 0 ? data : [{ name: "-", value: 0 }],
          },
        ],
      };
    } else if (subTab === "level") {
      const topLevels = [...levelSummaries].sort((a, b) => b.count - a.count);
      const data = topLevels.map((l) => ({
        name: l.level,
        value: l.count,
        itemStyle: { color: getStateColor(l.level) },
      }));
      option = {
        tooltip: {
          trigger: "item",
          formatter: "{b}: {c} ({d}%)",
        },
        series: [
          {
            type: "pie",
            radius: ["42%", "72%"],
            center: ["50%", "50%"],
            avoidLabelOverlap: true,
            itemStyle: {
              borderRadius: 4,
              borderColor: dark ? "#0f172a" : "#ffffff",
              borderWidth: 2,
            },
            label: {
              show: false,
            },
            emphasis: {
              label: {
                show: true,
                fontSize: 10,
                fontWeight: "bold",
                color: textColor,
              },
            },
            data: data.length > 0 ? data : [{ name: "-", value: 0 }],
          },
        ],
      };
    } else {
      // By Node: Top 5 horizontal bar chart
      const topNodes = [...nodeSummaries].sort((a, b) => b.count - a.count).slice(0, 5).reverse();
      option = {
        tooltip: {
          trigger: "axis",
          axisPointer: { type: "shadow" },
        },
        grid: {
          left: "3%",
          right: "8%",
          bottom: "3%",
          top: "5%",
          containLabel: true,
        },
        xAxis: {
          type: "value",
          splitLine: {
            lineStyle: { color: dark ? "#334155" : "#e2e8f0" },
          },
          axisLabel: { color: textColor, fontSize: 9 },
        },
        yAxis: {
          type: "category",
          data: topNodes.map((n) => (n.name.length > 10 ? n.name.slice(0, 10) + "…" : n.name)),
          axisLabel: { color: textColor, fontSize: 9 },
        },
        series: [
          {
            type: "bar",
            data: topNodes.map((n) => n.count),
            itemStyle: {
              color: "#06b6d4",
              borderRadius: [0, 4, 4, 0],
            },
          },
        ],
      };
    }

    chartInstance.setOption(option, true);
  };

  $effect(() => {
    if (filteredLogs && subTab && chartElem) {
      tick().then(() => renderChart());
    }
  });

  onMount(() => {
    const handleResize = () => {
      if (chartInstance) chartInstance.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      if (chartInstance) chartInstance.dispose();
    };
  });

  // CSV Export for aggregated records
  export function exportCSV(): void {
    let csv = "";
    if (subTab === "type") {
      csv =
        "Type,Count,Ratio,Error,Warn,Normal,AffectedNodes,LastOccurrence\n" +
        sortedTypeSummaries
          .map(
            (t) =>
              `"${t.type}",${t.count},"${t.percent}%",${t.errorCount},${t.warnCount},${t.normalCount},${t.nodeCount},"${t.lastTime > 0 ? formatTimeStr(t.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "level") {
      csv =
        "Level,Count,Ratio,AffectedNodes,Polling,User,Other,LastOccurrence\n" +
        sortedLevelSummaries
          .map(
            (l) =>
              `"${l.level}",${l.count},"${l.percent}%",${l.nodeCount},${l.pollingCount},${l.userCount},${l.otherCount},"${l.lastTime > 0 ? formatTimeStr(l.lastTime) : ""}"`
          )
          .join("\n");
    } else {
      csv =
        "NodeName,IP,Count,Ratio,Error,Warn,Normal,Polling,User,Other,LastOccurrence\n" +
        sortedNodeSummaries
          .map(
            (n) =>
              `"${n.name}","${n.ip}",${n.count},"${n.percent}%",${n.errorCount},${n.warnCount},${n.normalCount},${n.pollingCount},${n.userCount},${n.otherCount},"${n.lastTime > 0 ? formatTimeStr(n.lastTime) : ""}"`
          )
          .join("\n");
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_event_${subTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Title Header -->
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <FileText class="w-5 h-5 text-cyan-400" />
      {$_("report.eventTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">{$_("report.eventSubtitle")}</p>
  </div>

  <!-- KPI Overview with Mini ECharts (SLA-aligned: LOW is Error) -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalLogEvents")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {kpiStats.total.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.totalLogEventsSub")}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.highErrorEvents")}</span>
        <div class="text-2xl font-bold font-mono text-rose-400">
          {kpiStats.high.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.highErrorEventsSub")}</div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.warnLowEvents")}</span>
        <div class="text-2xl font-bold font-mono text-amber-400">
          {kpiStats.warn.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.warnLowEventsSub")}</div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.normalInfoEvents")}</span>
        <div class="text-2xl font-bold font-mono text-emerald-400">
          {kpiStats.normal.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.normalInfoEventsSub")}</div>
      </div>
    </div>

    <!-- Mini Distribution Chart Card -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
          {#if subTab === "type"}
            {$_("report.typeDistribution")}
          {:else if subTab === "level"}
            {$_("report.levelDistribution")}
          {:else}
            {$_("report.nodeDistribution")}
          {/if}
        </span>
        <span class="text-[10px] font-mono text-slate-400">
          {#if subTab === "type"}
            {kpiStats.uniqueTypes} types
          {:else if subTab === "level"}
            {levelSummaries.length} levels
          {:else}
            {kpiStats.uniqueNodes} nodes
          {/if}
        </span>
      </div>
      <div bind:this={chartElem} class="h-24 w-full"></div>
    </div>
  </div>

  <!-- Aggregation Sub-Tab Navigation -->
  <div class="flex items-center gap-1.5 border-b border-slate-200 dark:border-slate-800 pb-2">
    <button
      type="button"
      onclick={() => handleSelectSubTab("type")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'type' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{$_("report.subEventType")} ({typeSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("level")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'level' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <ShieldAlert class="w-3.5 h-3.5" />
      <span>{$_("report.subEventLevel")} ({levelSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("node")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'node' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Server class="w-3.5 h-3.5" />
      <span>{$_("report.subEventNode")} ({nodeSummaries.length})</span>
    </button>
  </div>

  <!-- Tables Container with Pagination -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    {#if subTab === "type"}
      <!-- 1. GROUP BY TYPE TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("type")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colType")}</span>
                {#if sortColumn === "type"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("nodeCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedNodes")}</span>
                {#if sortColumn === "nodeCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLastSeen")}</span>
                {#if sortColumn === "lastTime"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
          {#if paginatedTypeSummaries.length === 0}
            <tr>
              <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noLogs")}
              </td>
            </tr>
          {:else}
            {#each paginatedTypeSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-2 px-3 font-bold text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">
                  {row.type}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-2 px-3">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full bg-cyan-500 rounded-full" style="width: {row.percent}%"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-2 px-3 font-bold font-mono text-rose-500 dark:text-rose-400 text-[11px]">
                  {row.errorCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-amber-500 dark:text-amber-400 text-[11px]">
                  {row.warnCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-emerald-500 dark:text-emerald-400 text-[11px]">
                  {row.normalCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                  {row.nodeCount}
                </td>
                <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {:else if subTab === "level"}
      <!-- 2. GROUP BY LEVEL TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("level")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLevel")}</span>
                {#if sortColumn === "level"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("nodeCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedNodes")}</span>
                {#if sortColumn === "nodeCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("pollingCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-sky-600 dark:text-sky-400 font-bold">{$_("report.colPolling")}</span>
                {#if sortColumn === "pollingCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("userCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-indigo-600 dark:text-indigo-400 font-bold">{$_("report.colUserOp")}</span>
                {#if sortColumn === "userCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("otherCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-slate-600 dark:text-slate-400 font-bold">{$_("report.colOther")}</span>
                {#if sortColumn === "otherCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLastSeen")}</span>
                {#if sortColumn === "lastTime"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
          {#if paginatedLevelSummaries.length === 0}
            <tr>
              <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noLogs")}
              </td>
            </tr>
          {:else}
            {#each paginatedLevelSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-2 px-3">
                  <span
                    class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none"
                    style="background-color: {getStateColor(row.level)}20; border-color: {getStateColor(row.level)}50; color: {getStateColor(row.level)}"
                  >
                    <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(row.level)}"></span>
                    {row.level}
                  </span>
                </td>
                <td class="py-2 px-3 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-2 px-3">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full rounded-full" style="width: {row.percent}%; background-color: {getStateColor(row.level)}"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-2 px-3 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                  {row.nodeCount}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-sky-600 dark:text-sky-400 text-[11px]">
                  {row.pollingCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-indigo-600 dark:text-indigo-400 text-[11px]">
                  {row.userCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-slate-600 dark:text-slate-400 text-[11px]">
                  {row.otherCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {:else}
      <!-- 3. GROUP BY NODE TABLE (Dedicated columns, no line wrapping, no latest event) -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("name")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetNode")}</span>
                {#if sortColumn === "name"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("ip")}>
              <div class="inline-flex items-center gap-1">
                <span>IP</span>
                {#if sortColumn === "ip"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <!-- Severity Columns -->
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <!-- Type Group Columns -->
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("pollingCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-sky-600 dark:text-sky-400 font-bold">{$_("report.colPolling")}</span>
                {#if sortColumn === "pollingCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("userCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-indigo-600 dark:text-indigo-400 font-bold">{$_("report.colUserOp")}</span>
                {#if sortColumn === "userCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("otherCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-slate-600 dark:text-slate-400 font-bold">{$_("report.colOther")}</span>
                {#if sortColumn === "otherCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLastSeen")}</span>
                {#if sortColumn === "lastTime"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
          {#if paginatedNodeSummaries.length === 0}
            <tr>
              <td colspan="11" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noLogs")}
              </td>
            </tr>
          {:else}
            {#each paginatedNodeSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-2 px-3 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px] whitespace-nowrap">
                  {row.name}
                </td>
                <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 font-mono text-[11px] whitespace-nowrap">
                  {row.ip}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-2 px-3">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full bg-cyan-500 rounded-full" style="width: {row.percent}%"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <!-- Severity numbers -->
                <td class="py-2 px-3 font-bold font-mono text-rose-500 dark:text-rose-400 text-[11px]">
                  {row.errorCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-amber-500 dark:text-amber-400 text-[11px]">
                  {row.warnCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-emerald-500 dark:text-emerald-400 text-[11px]">
                  {row.normalCount.toLocaleString()}
                </td>
                <!-- Type group numbers -->
                <td class="py-2 px-3 font-bold font-mono text-sky-600 dark:text-sky-400 text-[11px]">
                  {row.pollingCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-indigo-600 dark:text-indigo-400 text-[11px]">
                  {row.userCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 font-bold font-mono text-slate-600 dark:text-slate-400 text-[11px]">
                  {row.otherCount.toLocaleString()}
                </td>
                <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {/if}

    <!-- Pagination Footer -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={currentTotalCount}
    />
  </div>
</div>
