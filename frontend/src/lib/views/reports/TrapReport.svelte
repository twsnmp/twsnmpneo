<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Radio,
    Server,
    BellRing,
    Layers,
    ShieldAlert,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { getStateColor, formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import type { ParquetLogRecord, NodeEnt } from "../../api";

  let {
    trapLogs = [],
    nodes = [],
    searchQuery = "",
  }: {
    trapLogs?: ParquetLogRecord[];
    nodes?: NodeEnt[];
    searchQuery?: string;
  } = $props();

  // Subtab navigation: host, type, enterprise, level
  let subTab = $state<"host" | "type" | "enterprise" | "level">("host");
  let sortColumn = $state("count");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // ECharts container & instance
  let chartElem = $state<HTMLDivElement | null>(null);
  let chartInstance: echarts.ECharts | null = null;

  // Node lookups by ID, Name, IP
  const nodeByIp = $derived(
    new Map<string, NodeEnt>(nodes.filter((n) => n.ip).map((n) => [n.ip, n]))
  );
  const nodeByName = $derived(
    new Map<string, NodeEnt>(nodes.map((n) => [n.name.toLowerCase(), n]))
  );

  // SLA-aligned Severity Classification (LOW/Error is Error, WARN is Warning, Normal/Info is Normal)
  export const isTrapError = (level: string, trapType: string, variables: string) => {
    const l = (level || "").toLowerCase();
    if (
      l === "high" ||
      l === "low" ||
      l === "error" ||
      l === "crit" ||
      l === "emergency" ||
      l === "emerg" ||
      l === "alert" ||
      l === "down"
    ) {
      return true;
    }
    const t = (trapType || "").toLowerCase();
    const v = (variables || "").toLowerCase();
    if (
      t.includes("linkdown") ||
      t.includes("loss") ||
      t.includes("fail") ||
      t.includes("error") ||
      t.includes("crit") ||
      t.includes("down")
    ) {
      return true;
    }
    if (
      v.includes("down") ||
      v.includes("error") ||
      v.includes("fail") ||
      v.includes("crit")
    ) {
      return true;
    }
    return false;
  };

  export const isTrapWarn = (level: string, trapType: string, variables: string) => {
    if (isTrapError(level, trapType, variables)) return false;
    const l = (level || "").toLowerCase();
    if (l === "warn" || l === "warning") return true;
    const t = (trapType || "").toLowerCase();
    const v = (variables || "").toLowerCase();
    if (
      t.includes("authenticationfailure") ||
      t.includes("coldstart") ||
      t.includes("warmstart") ||
      t.includes("warn")
    ) {
      return true;
    }
    if (v.includes("warn") || v.includes("warning")) return true;
    return false;
  };

  export const isTrapNormal = (level: string, trapType: string, variables: string) => {
    return !isTrapError(level, trapType, variables) && !isTrapWarn(level, trapType, variables);
  };

  interface NormalizedTrap {
    time: number;
    fromAddress: string;
    host: string;
    nodeName: string;
    nodeIp: string;
    trapType: string;
    enterprise: string;
    variables: string;
    level: string;
    levelName: string;
  }

  const parseJsonSafe = (str: string): any => {
    if (!str || typeof str !== "string") return {};
    try {
      return JSON.parse(str);
    } catch {
      return {};
    }
  };

  // Helper to extract IP and Hostname from fromAddress like "192.168.1.1(CoreSwitch)"
  const parseFromAddress = (from: string) => {
    if (!from) return { ip: "", host: "unknown", name: "" };
    const m = from.match(/^([^(]+)(?:\((.*)\))?$/);
    if (m) {
      const ip = m[1].trim();
      const name = (m[2] || "").trim();
      return { ip, host: name || ip, name };
    }
    return { ip: from, host: from, name: "" };
  };

  // Normalize Trap records
  const normalizedLogs = $derived.by<NormalizedTrap[]>(() => {
    return trapLogs.map((r) => {
      const parsed = parseJsonSafe(r.log || (r as any).Log || "");
      const time =
        r.time || (r as any).Time || parsed.Time || parsed.time || Date.now() * 1e6;
      const rawFrom =
        parsed.FromAddress ||
        parsed.fromAddress ||
        r.src ||
        (r as any).Src ||
        parsed.srcIP ||
        "";
      const { ip: parsedIp, host: parsedHost, name: parsedName } = parseFromAddress(rawFrom);

      const matchedNode =
        (parsedIp ? nodeByIp.get(parsedIp) : undefined) ||
        (parsedName ? nodeByName.get(parsedName.toLowerCase()) : undefined) ||
        (parsedHost ? nodeByName.get(parsedHost.toLowerCase()) : undefined);

      const nodeName = matchedNode?.name || parsedName || parsedHost;
      const nodeIp = matchedNode?.ip || parsedIp || "-";
      const host = nodeName || parsedHost || parsedIp || "unknown";

      const trapType = (parsed.TrapType || parsed.trapType || "unknown").trim();
      const variables = parsed.Variables || parsed.variables || "";

      let enterprise = (parsed.Enterprise || parsed.enterprise || "").trim();
      if (!enterprise && variables.includes("Enterprise=")) {
        const entMatch = variables.match(/Enterprise=([^\s]+)/);
        if (entMatch) enterprise = entMatch[1];
      }
      if (!enterprise) {
        enterprise = "SNMPv2-MIB::snmpTraps";
      }

      let level = (parsed.level || (parsed as any).Level || "").toLowerCase();
      if (!level) {
        if (isTrapError("", trapType, variables)) {
          level = "high";
        } else if (isTrapWarn("", trapType, variables)) {
          level = "warn";
        } else if (
          trapType.toLowerCase().includes("linkup") ||
          trapType.toLowerCase().includes("up")
        ) {
          level = "normal";
        } else {
          level = "info";
        }
      }

      let levelName = $_("common.info");
      if (isTrapError(level, trapType, variables)) {
        levelName = $_("report.colError");
      } else if (isTrapWarn(level, trapType, variables)) {
        levelName = $_("report.colWarn");
      } else {
        levelName = $_("report.colNormal");
      }

      return {
        time,
        fromAddress: rawFrom,
        host,
        nodeName,
        nodeIp,
        trapType,
        enterprise,
        variables,
        level,
        levelName,
      };
    });
  });

  // Filter logs by search query
  const filteredLogs = $derived.by<NormalizedTrap[]>(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return normalizedLogs;
    return normalizedLogs.filter(
      (l) =>
        l.trapType.toLowerCase().includes(q) ||
        l.enterprise.toLowerCase().includes(q) ||
        l.host.toLowerCase().includes(q) ||
        l.nodeName.toLowerCase().includes(q) ||
        l.nodeIp.toLowerCase().includes(q) ||
        l.variables.toLowerCase().includes(q) ||
        l.fromAddress.toLowerCase().includes(q) ||
        l.levelName.toLowerCase().includes(q)
    );
  });

  // KPI Overview Statistics
  const kpiStats = $derived.by(() => {
    const total = filteredLogs.length;
    let high = 0;
    let warn = 0;
    let normal = 0;
    const uniqueHosts = new Set<string>();
    const uniqueTypes = new Set<string>();
    const uniqueEnterprises = new Set<string>();

    for (const l of filteredLogs) {
      if (isTrapError(l.level, l.trapType, l.variables)) {
        high++;
      } else if (isTrapWarn(l.level, l.trapType, l.variables)) {
        warn++;
      } else {
        normal++;
      }
      uniqueHosts.add(l.host);
      uniqueTypes.add(l.trapType);
      uniqueEnterprises.add(l.enterprise);
    }

    return {
      total,
      high,
      warn,
      normal,
      uniqueHosts: uniqueHosts.size,
      uniqueTypes: uniqueTypes.size,
      uniqueEnterprises: uniqueEnterprises.size,
    };
  });

  // 1. Group by Host/Source Summary
  interface HostSummary {
    host: string;
    name: string;
    ip: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    topTrapTypes: string;
    lastTime: number;
  }

  const hostSummaries = $derived.by<HostSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        host: string;
        name: string;
        ip: string;
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        typeCounts: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      const key = l.host || "__unknown__";
      let item = map.get(key);
      if (!item) {
        item = {
          host: l.host,
          name: l.nodeName,
          ip: l.nodeIp || "-",
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          typeCounts: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      if (isTrapError(l.level, l.trapType, l.variables)) {
        item.errorCount++;
      } else if (isTrapWarn(l.level, l.trapType, l.variables)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.typeCounts.set(l.trapType, (item.typeCounts.get(l.trapType) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => {
      const sortedTypes = Array.from(d.typeCounts.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([t, c]) => `${t} (${c})`)
        .join(", ");

      return {
        host: d.host,
        name: d.name,
        ip: d.ip,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        errorCount: d.errorCount,
        warnCount: d.warnCount,
        normalCount: d.normalCount,
        topTrapTypes: sortedTypes || "-",
        lastTime: d.lastTime,
      };
    });
  });

  // 2. Group by Trap Type Summary
  interface TypeSummary {
    trapType: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    hostCount: number;
    topEnterprise: string;
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
        hosts: Set<string>;
        entCounts: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.trapType);
      if (!item) {
        item = {
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          hosts: new Set<string>(),
          entCounts: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(l.trapType, item);
      }
      item.count++;
      if (isTrapError(l.level, l.trapType, l.variables)) {
        item.errorCount++;
      } else if (isTrapWarn(l.level, l.trapType, l.variables)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.hosts.add(l.host);
      item.entCounts.set(l.enterprise, (item.entCounts.get(l.enterprise) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([trapType, d]) => {
      const topEnt =
        Array.from(d.entCounts.entries()).sort((a, b) => b[1] - a[1])[0]?.[0] || "-";
      return {
        trapType,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        errorCount: d.errorCount,
        warnCount: d.warnCount,
        normalCount: d.normalCount,
        hostCount: d.hosts.size,
        topEnterprise: topEnt,
        lastTime: d.lastTime,
      };
    });
  });

  // 3. Group by Enterprise Summary
  interface EnterpriseSummary {
    enterprise: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    typeCount: number;
    hostCount: number;
    topTrapTypes: string;
    lastTime: number;
  }

  const enterpriseSummaries = $derived.by<EnterpriseSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        types: Map<string, number>;
        hosts: Set<string>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.enterprise);
      if (!item) {
        item = {
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          types: new Map<string, number>(),
          hosts: new Set<string>(),
          lastTime: 0,
        };
        map.set(l.enterprise, item);
      }
      item.count++;
      if (isTrapError(l.level, l.trapType, l.variables)) {
        item.errorCount++;
      } else if (isTrapWarn(l.level, l.trapType, l.variables)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.types.set(l.trapType, (item.types.get(l.trapType) || 0) + 1);
      item.hosts.add(l.host);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([enterprise, d]) => {
      const sortedTypes = Array.from(d.types.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([t, c]) => `${t} (${c})`)
        .join(", ");

      return {
        enterprise,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        errorCount: d.errorCount,
        warnCount: d.warnCount,
        normalCount: d.normalCount,
        typeCount: d.types.size,
        hostCount: d.hosts.size,
        topTrapTypes: sortedTypes || "-",
        lastTime: d.lastTime,
      };
    });
  });

  // 4. Group by Level Summary
  interface LevelSummary {
    level: string;
    levelName: string;
    count: number;
    percent: number;
    hostCount: number;
    topTrapTypes: string;
    lastTime: number;
  }

  const levelSummaries = $derived.by<LevelSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        level: string;
        levelName: string;
        count: number;
        hosts: Set<string>;
        typeCounts: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let key = "normal";
      let keyName = $_("report.colNormal");
      if (isTrapError(l.level, l.trapType, l.variables)) {
        key = "high";
        keyName = $_("report.colError");
      } else if (isTrapWarn(l.level, l.trapType, l.variables)) {
        key = "warn";
        keyName = $_("report.colWarn");
      }

      let item = map.get(key);
      if (!item) {
        item = {
          level: key,
          levelName: keyName,
          count: 0,
          hosts: new Set<string>(),
          typeCounts: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      item.hosts.add(l.host);
      item.typeCounts.set(l.trapType, (item.typeCounts.get(l.trapType) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => {
      const sortedTypes = Array.from(d.typeCounts.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([t, c]) => `${t} (${c})`)
        .join(", ");

      return {
        level: d.level,
        levelName: d.levelName,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        hostCount: d.hosts.size,
        topTrapTypes: sortedTypes || "-",
        lastTime: d.lastTime,
      };
    });
  });

  // Sort handler
  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection =
        colKey === "name" ||
        colKey === "host" ||
        colKey === "trapType" ||
        colKey === "enterprise" ||
        colKey === "levelName" ||
        colKey === "ip"
          ? "asc"
          : "desc";
    }
  };

  const handleSelectSubTab = (tab: "host" | "type" | "enterprise" | "level") => {
    subTab = tab;
    currentPage = 1;
    sortColumn = "count";
    sortDirection = "desc";
  };

  // Sort & paginate by host
  const sortedHostSummaries = $derived(
    [...hostSummaries].sort((a: any, b: any) => {
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
  const paginatedHostSummaries = $derived(
    pageSize === -1
      ? sortedHostSummaries
      : sortedHostSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

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

  // Sort & paginate by enterprise
  const sortedEnterpriseSummaries = $derived(
    [...enterpriseSummaries].sort((a: any, b: any) => {
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
  const paginatedEnterpriseSummaries = $derived(
    pageSize === -1
      ? sortedEnterpriseSummaries
      : sortedEnterpriseSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
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

  const currentTotalCount = $derived(
    subTab === "host"
      ? sortedHostSummaries.length
      : subTab === "type"
      ? sortedTypeSummaries.length
      : subTab === "enterprise"
      ? sortedEnterpriseSummaries.length
      : sortedLevelSummaries.length
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

    if (subTab === "host") {
      const topHosts = [...hostSummaries].sort((a, b) => b.count - a.count).slice(0, 5).reverse();
      option = {
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        grid: { left: "3%", right: "8%", bottom: "3%", top: "5%", containLabel: true },
        xAxis: {
          type: "value",
          splitLine: { lineStyle: { color: dark ? "#334155" : "#e2e8f0" } },
          axisLabel: { color: textColor, fontSize: 9 },
        },
        yAxis: {
          type: "category",
          data: topHosts.map((h) => (h.name.length > 10 ? h.name.slice(0, 10) + "…" : h.name)),
          axisLabel: { color: textColor, fontSize: 9 },
        },
        series: [
          {
            type: "bar",
            data: topHosts.map((h) => h.count),
            itemStyle: { color: "#38bdf8", borderRadius: [0, 4, 4, 0] },
          },
        ],
      };
    } else if (subTab === "type") {
      const topTypes = [...typeSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topTypes.map((t) => ({ name: t.trapType, value: t.count }));
      option = {
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        series: [
          {
            type: "pie",
            radius: ["42%", "72%"],
            center: ["50%", "50%"],
            itemStyle: { borderRadius: 4, borderColor: dark ? "#0f172a" : "#ffffff", borderWidth: 2 },
            label: { show: false },
            emphasis: { label: { show: true, fontSize: 10, fontWeight: "bold", color: textColor } },
            data: data.length > 0 ? data : [{ name: "-", value: 0 }],
          },
        ],
      };
    } else if (subTab === "enterprise") {
      const topEnts = [...enterpriseSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topEnts.map((e) => ({
        name: e.enterprise.includes("::") ? e.enterprise.split("::")[1] : e.enterprise,
        value: e.count,
      }));
      option = {
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        series: [
          {
            type: "pie",
            radius: ["42%", "72%"],
            center: ["50%", "50%"],
            itemStyle: { borderRadius: 4, borderColor: dark ? "#0f172a" : "#ffffff", borderWidth: 2 },
            label: { show: false },
            emphasis: { label: { show: true, fontSize: 10, fontWeight: "bold", color: textColor } },
            data: data.length > 0 ? data : [{ name: "-", value: 0 }],
          },
        ],
      };
    } else {
      const data = levelSummaries.map((l) => ({
        name: l.levelName,
        value: l.count,
        itemStyle: { color: getStateColor(l.level) },
      }));
      option = {
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        series: [
          {
            type: "pie",
            radius: ["42%", "72%"],
            center: ["50%", "50%"],
            itemStyle: { borderRadius: 4, borderColor: dark ? "#0f172a" : "#ffffff", borderWidth: 2 },
            label: { show: false },
            emphasis: { label: { show: true, fontSize: 10, fontWeight: "bold", color: textColor } },
            data: data.length > 0 ? data : [{ name: "-", value: 0 }],
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
    if (subTab === "host") {
      csv =
        "Host,NodeName,IP,Count,Ratio,Error,Warn,Normal,TopTrapTypes,LastOccurrence\n" +
        sortedHostSummaries
          .map(
            (h) =>
              `"${h.host}","${h.name}","${h.ip}",${h.count},"${h.percent}%",${h.errorCount},${h.warnCount},${h.normalCount},"${h.topTrapTypes}","${h.lastTime > 0 ? formatTimeStr(h.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "type") {
      csv =
        "TrapType,Count,Ratio,Error,Warn,Normal,AffectedHosts,TopEnterprise,LastOccurrence\n" +
        sortedTypeSummaries
          .map(
            (t) =>
              `"${t.trapType}",${t.count},"${t.percent}%",${t.errorCount},${t.warnCount},${t.normalCount},${t.hostCount},"${t.topEnterprise}","${t.lastTime > 0 ? formatTimeStr(t.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "enterprise") {
      csv =
        "Enterprise,Count,Ratio,Error,Warn,Normal,TrapTypes,AffectedHosts,TopTrapTypes,LastOccurrence\n" +
        sortedEnterpriseSummaries
          .map(
            (e) =>
              `"${e.enterprise}",${e.count},"${e.percent}%",${e.errorCount},${e.warnCount},${e.normalCount},${e.typeCount},${e.hostCount},"${e.topTrapTypes}","${e.lastTime > 0 ? formatTimeStr(e.lastTime) : ""}"`
          )
          .join("\n");
    } else {
      csv =
        "Level,LevelName,Count,Ratio,AffectedHosts,TopTrapTypes,LastOccurrence\n" +
        sortedLevelSummaries
          .map(
            (l) =>
              `"${l.level}","${l.levelName}",${l.count},"${l.percent}%",${l.hostCount},"${l.topTrapTypes}","${l.lastTime > 0 ? formatTimeStr(l.lastTime) : ""}"`
          )
          .join("\n");
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_trap_${subTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Title Header -->
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Radio class="w-5 h-5 text-cyan-400" />
      {$_("report.trapTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">{$_("report.trapSubtitle")}</p>
  </div>

  <!-- KPI Overview with Mini ECharts (SLA-aligned: LOW is Error) -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalTrapLogs")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {kpiStats.total.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.totalTrapLogsSub")}
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
          {#if subTab === "host"}
            {$_("report.trapHostDistribution")}
          {:else if subTab === "type"}
            {$_("report.trapTypeDistribution")}
          {:else if subTab === "enterprise"}
            {$_("report.enterpriseDistribution")}
          {:else}
            {$_("report.levelDistribution")}
          {/if}
        </span>
        <span class="text-[10px] font-mono text-slate-400">
          {#if subTab === "host"}
            {kpiStats.uniqueHosts} hosts
          {:else if subTab === "type"}
            {kpiStats.uniqueTypes} types
          {:else if subTab === "enterprise"}
            {kpiStats.uniqueEnterprises} ents
          {:else}
            {levelSummaries.length} lvls
          {/if}
        </span>
      </div>
      <div bind:this={chartElem} class="h-24 w-full"></div>
    </div>
  </div>

  <!-- Sub-Tab Navigation -->
  <div class="flex items-center gap-1.5 border-b border-slate-200 dark:border-slate-800 pb-2">
    <button
      type="button"
      onclick={() => handleSelectSubTab("host")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'host' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Server class="w-3.5 h-3.5" />
      <span>{$_("report.subTrapHost")} ({hostSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("type")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'type' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <BellRing class="w-3.5 h-3.5" />
      <span>{$_("report.subTrapType")} ({typeSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("enterprise")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'enterprise' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{$_("report.subTrapEnterprise")} ({enterpriseSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("level")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'level' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <ShieldAlert class="w-3.5 h-3.5" />
      <span>{$_("report.subTrapLevel")} ({levelSummaries.length})</span>
    </button>
  </div>

  <!-- Tables Container with Pagination -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    {#if subTab === "host"}
      <!-- 1. GROUP BY HOST TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("name")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colSourceAddress")}</span>
                {#if sortColumn === "name"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("ip")}>
              <div class="inline-flex items-center gap-1">
                <span>IP</span>
                {#if sortColumn === "ip"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colTopTrapTypes")}</span>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("lastTime")}>
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
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
          {#each paginatedHostSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-2 px-3 font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">
                {item.name}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.ip}
              </td>
              <td class="py-2 px-3 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topTrapTypes}>
                {item.topTrapTypes}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="9" class="py-8 text-center text-slate-400">
                {$_("report.noTraps")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if subTab === "type"}
      <!-- 2. GROUP BY TRAP TYPE TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("trapType")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTrapType")}</span>
                {#if sortColumn === "trapType"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("topEnterprise")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEnterprise")}</span>
                {#if sortColumn === "topEnterprise"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("lastTime")}>
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
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
          {#each paginatedTypeSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-2 px-3 font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">
                {item.trapType}
              </td>
              <td class="py-2 px-3 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.hostCount}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topEnterprise}>
                {item.topEnterprise}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="9" class="py-8 text-center text-slate-400">
                {$_("report.noTraps")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if subTab === "enterprise"}
      <!-- 3. GROUP BY ENTERPRISE TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("enterprise")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEnterprise")}</span>
                {#if sortColumn === "enterprise"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("typeCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTrapType")}</span>
                {#if sortColumn === "typeCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colTopTrapTypes")}</span>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("lastTime")}>
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
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
          {#each paginatedEnterpriseSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-2 px-3 font-semibold text-slate-800 dark:text-slate-200 truncate max-w-sm" title={item.enterprise}>
                {item.enterprise}
              </td>
              <td class="py-2 px-3 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.typeCount}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.hostCount}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topTrapTypes}>
                {item.topTrapTypes}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="10" class="py-8 text-center text-slate-400">
                {$_("report.noTraps")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <!-- 4. GROUP BY LEVEL TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("levelName")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colStatus")}</span>
                {#if sortColumn === "levelName"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colTopTrapTypes")}</span>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("lastTime")}>
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
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60">
          {#each paginatedLevelSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-2 px-3 whitespace-nowrap">
                <span
                  class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-bold"
                  style="background-color: {getStateColor(item.level)}20; color: {getStateColor(item.level)};"
                >
                  {item.levelName}
                </span>
              </td>
              <td class="py-2 px-3 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.hostCount}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topTrapTypes}>
                {item.topTrapTypes}
              </td>
              <td class="py-2 px-3 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="py-8 text-center text-slate-400">
                {$_("report.noTraps")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}

    <!-- Pagination Footer -->
    <ReportPagination
      totalCount={currentTotalCount}
      bind:currentPage
      bind:pageSize
    />
  </div>
</div>
