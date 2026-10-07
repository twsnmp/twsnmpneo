<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    ScrollText,
    Server,
    Tag,
    Layers,
    ShieldAlert,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import {
    getStateColor,
    formatTimeStr,
    facilityNames,
    severityNames,
  } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchSyslogStats,
    resetSyslogStats,
    type SyslogStatsSummary,
    type ParquetLogRecord,
    type NodeEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";

  let {
    syslogLogs = [],
    nodes = [],
    searchQuery = "",
  }: {
    syslogLogs?: ParquetLogRecord[];
    nodes?: NodeEnt[];
    searchQuery?: string;
  } = $props();

  let backendSyslogStats = $state<SyslogStatsSummary | null>(null);

  export const refresh = async () => {
    try {
      backendSyslogStats = await fetchSyslogStats();
    } catch {
      // ignore
    }
  };

  export const handleClear = async () => {
    const ok = await showConfirm({
      title: $_('common.confirmClear') || 'レポートクリアの確認',
      message: $_("report.confirmClearReport") || "レポートデータをクリアしますか？",
      type: 'warning',
      confirmText: $_('common.clear') || 'クリア',
    });
    if (!ok) return;
    try {
      await resetSyslogStats();
      await refresh();
    } catch (e: any) {
      showAlert({
        title: $_('common.error') || 'エラー',
        message: e?.message || e,
        type: 'danger',
      });
    }
  };

  onMount(() => {
    refresh();
  });

  // Subtab navigation: host, tag, facility, level
  let subTab = $state<"host" | "tag" | "facility" | "level">("host");
  let sortColumn = $state("count");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // ECharts container & instance
  let chartElem = $state<HTMLDivElement | null>(null);
  let chartInstance: echarts.ECharts | null = null;

  // Node lookups by ID, Name, IP
  const nodeByIp = $derived(new Map<string, NodeEnt>(nodes.filter((n) => n.ip).map((n) => [n.ip, n])));
  const nodeByName = $derived(new Map<string, NodeEnt>(nodes.map((n) => [n.name.toLowerCase(), n])));

  // SLA-aligned Severity Classification (LOW is Error, WARN is Warning)
  export const isSyslogError = (severity: number, level: string) => {
    if (severity >= 0 && severity <= 3) return true; // emerg, alert, crit, err
    const l = (level || "").toLowerCase();
    return (
      l === "high" ||
      l === "low" ||
      l === "error" ||
      l === "emergency" ||
      l === "emerg" ||
      l === "alert" ||
      l === "crit" ||
      l === "down"
    );
  };

  export const isSyslogWarn = (severity: number, level: string) => {
    if (severity === 4) return true; // warning
    const l = (level || "").toLowerCase();
    return l === "warn" || l === "warning";
  };

  export const isSyslogNormal = (severity: number, level: string) => {
    return !isSyslogError(severity, level) && !isSyslogWarn(severity, level);
  };

  interface NormalizedSyslog {
    time: number;
    host: string;
    nodeName: string;
    nodeIp: string;
    tag: string;
    facility: number;
    facilityName: string;
    severity: number;
    severityName: string;
    level: string;
    message: string;
  }

  const parseJsonSafe = (str: string): any => {
    if (!str || typeof str !== "string") return {};
    try {
      return JSON.parse(str);
    } catch {
      return {};
    }
  };

  // Normalize Syslog records
  const normalizedLogs = $derived.by<NormalizedSyslog[]>(() => {
    return syslogLogs.map((r) => {
      const parsed = parseJsonSafe(r.log || (r as any).Log || "");
      const time = r.time || (r as any).Time || parsed.time || parsed.Time || Date.now() * 1e6;
      const rawSrc = r.src || (r as any).Src || parsed.src || parsed.host || parsed.srcIP || "";
      const host = parsed.hostname || parsed.Hostname || parsed.host || parsed.Host || rawSrc || "unknown";

      const matchedNode = nodeByIp.get(host) || nodeByIp.get(rawSrc) || nodeByName.get(host.toLowerCase());
      const nodeName = matchedNode?.name || host;
      const nodeIp = matchedNode?.ip || (host.match(/^\d+\.\d+\.\d+\.\d+$/) ? host : rawSrc);

      const tag = (parsed.tag || parsed.Tag || parsed.app_name || parsed.appName || "unknown").trim();

      let facility = typeof parsed.facility === "number" ? parsed.facility : (typeof parsed.Facility === "number" ? parsed.Facility : -1);
      if (facility < 0 || facility >= facilityNames.length) facility = 1; // user
      const facilityName = facilityNames[facility] || `facility-${facility}`;

      let severity = typeof parsed.severity === "number" ? parsed.severity : (typeof parsed.Severity === "number" ? parsed.Severity : -1);
      if (severity < 0 || severity >= severityNames.length) severity = 6; // info
      const severityName = severityNames[severity] || `sev-${severity}`;

      let level = (parsed.level || (parsed as any).Level || "").toLowerCase();
      if (!level) {
        if (severity <= 3) level = "high";
        else if (severity === 4) level = "warn";
        else level = "info";
      }

      let message = parsed.content || parsed.Content || parsed.message || parsed.Message || "";
      if (!message && r.log) message = r.log;

      return {
        time,
        host,
        nodeName,
        nodeIp,
        tag: tag || "unknown",
        facility,
        facilityName,
        severity,
        severityName,
        level,
        message,
      };
    });
  });

  // Filter logs by search query
  const filteredLogs = $derived.by<NormalizedSyslog[]>(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return normalizedLogs;
    return normalizedLogs.filter(
      (l) =>
        l.message.toLowerCase().includes(q) ||
        l.host.toLowerCase().includes(q) ||
        l.nodeName.toLowerCase().includes(q) ||
        l.nodeIp.toLowerCase().includes(q) ||
        l.tag.toLowerCase().includes(q) ||
        l.facilityName.toLowerCase().includes(q) ||
        l.severityName.toLowerCase().includes(q)
    );
  });

  // KPI Overview Statistics
  const kpiStats = $derived.by(() => {
    if (backendSyslogStats && backendSyslogStats.Total > 0 && !searchQuery) {
      return {
        total: backendSyslogStats.Total,
        high: backendSyslogStats.ErrorCount,
        warn: backendSyslogStats.WarnCount,
        normal: backendSyslogStats.NormalCount,
        uniqueHosts: Object.keys(backendSyslogStats.Hosts || {}).length,
        uniqueTags: Object.keys(backendSyslogStats.Tags || {}).length,
      };
    }

    const total = filteredLogs.length;
    let high = 0;
    let warn = 0;
    let normal = 0;
    const uniqueHosts = new Set<string>();
    const uniqueTags = new Set<string>();

    for (const l of filteredLogs) {
      if (isSyslogError(l.severity, l.level)) {
        high++;
      } else if (isSyslogWarn(l.severity, l.level)) {
        warn++;
      } else {
        normal++;
      }
      uniqueHosts.add(l.host);
      uniqueTags.add(l.tag);
    }

    return {
      total,
      high,
      warn,
      normal,
      uniqueHosts: uniqueHosts.size,
      uniqueTags: uniqueTags.size,
    };
  });

  // 1. Group by Host Summary
  interface HostSummary {
    host: string;
    name: string;
    ip: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    topTags: string;
    lastTime: number;
  }

  const hostSummaries = $derived.by<HostSummary[]>(() => {
    if (backendSyslogStats && backendSyslogStats.Hosts && !searchQuery) {
      const total = backendSyslogStats.Total || 1;
      return Object.values(backendSyslogStats.Hosts).map((h) => {
        const matchedNode = nodeByIp.get(h.Host) || nodeByName.get((h.Host || "").toLowerCase());
        return {
          host: h.Host,
          name: h.NodeName || matchedNode?.name || h.Host,
          ip: matchedNode?.ip || h.Host,
          count: h.Count,
          percent: Number(((h.Count / total) * 100).toFixed(1)),
          errorCount: h.ErrorCount,
          warnCount: h.WarnCount,
          normalCount: h.NormalCount,
          topTags: "-",
          lastTime: h.LastTime,
        };
      });
    }

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
        tagCounts: Map<string, number>;
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
          tagCounts: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      if (isSyslogError(l.severity, l.level)) {
        item.errorCount++;
      } else if (isSyslogWarn(l.severity, l.level)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.tagCounts.set(l.tag, (item.tagCounts.get(l.tag) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => {
      const sortedTags = Array.from(d.tagCounts.entries())
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
        topTags: sortedTags || "-",
        lastTime: d.lastTime,
      };
    });
  });

  // 2. Group by Tag / App Summary
  interface TagSummary {
    tag: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    hostCount: number;
    lastTime: number;
  }

  const tagSummaries = $derived.by<TagSummary[]>(() => {
    if (backendSyslogStats && backendSyslogStats.Tags && !searchQuery) {
      const total = backendSyslogStats.Total || 1;
      return Object.values(backendSyslogStats.Tags).map((t) => ({
        tag: t.Tag,
        count: t.Count,
        percent: Number(((t.Count / total) * 100).toFixed(1)),
        errorCount: 0,
        warnCount: 0,
        normalCount: t.Count,
        hostCount: 1,
        lastTime: t.LastTime,
      }));
    }

    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        hosts: Set<string>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.tag);
      if (!item) {
        item = {
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          hosts: new Set<string>(),
          lastTime: 0,
        };
        map.set(l.tag, item);
      }
      item.count++;
      if (isSyslogError(l.severity, l.level)) {
        item.errorCount++;
      } else if (isSyslogWarn(l.severity, l.level)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.hosts.add(l.host);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([tag, d]) => ({
      tag,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      hostCount: d.hosts.size,
      lastTime: d.lastTime,
    }));
  });

  // 3. Group by Facility Summary
  interface FacilitySummary {
    facility: number;
    facilityName: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    hostCount: number;
    lastTime: number;
  }

  const facilitySummaries = $derived.by<FacilitySummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      number,
      {
        facilityName: string;
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        hosts: Set<string>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.facility);
      if (!item) {
        item = {
          facilityName: l.facilityName,
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          hosts: new Set<string>(),
          lastTime: 0,
        };
        map.set(l.facility, item);
      }
      item.count++;
      if (isSyslogError(l.severity, l.level)) {
        item.errorCount++;
      } else if (isSyslogWarn(l.severity, l.level)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      item.hosts.add(l.host);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([facility, d]) => ({
      facility,
      facilityName: d.facilityName,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      hostCount: d.hosts.size,
      lastTime: d.lastTime,
    }));
  });

  // 4. Group by Level / Severity Summary
  interface LevelSummary {
    severity: number;
    severityName: string;
    level: string;
    count: number;
    percent: number;
    hostCount: number;
    topTags: string;
    lastTime: number;
  }

  const levelSummaries = $derived.by<LevelSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        severity: number;
        severityName: string;
        level: string;
        count: number;
        hosts: Set<string>;
        tagCounts: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      const key = l.severityName;
      let item = map.get(key);
      if (!item) {
        item = {
          severity: l.severity,
          severityName: l.severityName,
          level: l.level,
          count: 0,
          hosts: new Set<string>(),
          tagCounts: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      item.hosts.add(l.host);
      item.tagCounts.set(l.tag, (item.tagCounts.get(l.tag) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => {
      const sortedTags = Array.from(d.tagCounts.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([t, c]) => `${t} (${c})`)
        .join(", ");

      return {
        severity: d.severity,
        severityName: d.severityName,
        level: d.level,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        hostCount: d.hosts.size,
        topTags: sortedTags || "-",
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
        colKey === "tag" ||
        colKey === "facilityName" ||
        colKey === "severityName" ||
        colKey === "ip"
          ? "asc"
          : "desc";
    }
  };

  const handleSelectSubTab = (tab: "host" | "tag" | "facility" | "level") => {
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

  // Sort & paginate by tag
  const sortedTagSummaries = $derived(
    [...tagSummaries].sort((a: any, b: any) => {
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
  const paginatedTagSummaries = $derived(
    pageSize === -1
      ? sortedTagSummaries
      : sortedTagSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by facility
  const sortedFacilitySummaries = $derived(
    [...facilitySummaries].sort((a: any, b: any) => {
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
  const paginatedFacilitySummaries = $derived(
    pageSize === -1
      ? sortedFacilitySummaries
      : sortedFacilitySummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
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
      : subTab === "tag"
      ? sortedTagSummaries.length
      : subTab === "facility"
      ? sortedFacilitySummaries.length
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
    } else if (subTab === "tag") {
      const topTags = [...tagSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topTags.map((t) => ({ name: t.tag, value: t.count }));
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
    } else if (subTab === "facility") {
      const topFacs = [...facilitySummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topFacs.map((f) => ({ name: f.facilityName, value: f.count }));
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
        name: l.severityName,
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
        "Host,NodeName,IP,Count,Ratio,Error,Warn,Normal,TopTags,LastOccurrence\n" +
        sortedHostSummaries
          .map(
            (h) =>
              `"${h.host}","${h.name}","${h.ip}",${h.count},"${h.percent}%",${h.errorCount},${h.warnCount},${h.normalCount},"${h.topTags}","${h.lastTime > 0 ? formatTimeStr(h.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "tag") {
      csv =
        "Tag,Count,Ratio,Error,Warn,Normal,AffectedHosts,LastOccurrence\n" +
        sortedTagSummaries
          .map(
            (t) =>
              `"${t.tag}",${t.count},"${t.percent}%",${t.errorCount},${t.warnCount},${t.normalCount},${t.hostCount},"${t.lastTime > 0 ? formatTimeStr(t.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "facility") {
      csv =
        "Facility,Count,Ratio,Error,Warn,Normal,AffectedHosts,LastOccurrence\n" +
        sortedFacilitySummaries
          .map(
            (f) =>
              `"${f.facilityName}",${f.count},"${f.percent}%",${f.errorCount},${f.warnCount},${f.normalCount},${f.hostCount},"${f.lastTime > 0 ? formatTimeStr(f.lastTime) : ""}"`
          )
          .join("\n");
    } else {
      csv =
        "Severity,Level,Count,Ratio,AffectedHosts,TopTags,LastOccurrence\n" +
        sortedLevelSummaries
          .map(
            (l) =>
              `"${l.severityName}","${l.level}",${l.count},"${l.percent}%",${l.hostCount},"${l.topTags}","${l.lastTime > 0 ? formatTimeStr(l.lastTime) : ""}"`
          )
          .join("\n");
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_syslog_${subTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Title Header -->
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <ScrollText class="w-5 h-5 text-cyan-400" />
      {$_("report.syslogTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">{$_("report.syslogSubtitle")}</p>
  </div>

  <!-- KPI Overview with Mini ECharts (SLA-aligned: LOW is Error) -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalSyslogLogs")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {kpiStats.total.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.totalSyslogLogsSub")}
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
            {$_("report.hostDistribution")}
          {:else if subTab === "tag"}
            {$_("report.tagDistribution")}
          {:else if subTab === "facility"}
            {$_("report.facilityDistribution")}
          {:else}
            {$_("report.levelDistribution")}
          {/if}
        </span>
        <span class="text-[10px] font-mono text-slate-400">
          {#if subTab === "host"}
            {kpiStats.uniqueHosts} hosts
          {:else if subTab === "tag"}
            {kpiStats.uniqueTags} tags
          {:else if subTab === "facility"}
            {facilitySummaries.length} fac
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
      <span>{$_("report.subSyslogHost")} ({hostSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("tag")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'tag' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Tag class="w-3.5 h-3.5" />
      <span>{$_("report.subSyslogTag")} ({tagSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("facility")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'facility' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{$_("report.subSyslogFacility")} ({facilitySummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("level")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'level' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <ShieldAlert class="w-3.5 h-3.5" />
      <span>{$_("report.subSyslogLevel")} ({levelSummaries.length})</span>
    </button>
  </div>

  <!-- Tables Container with Pagination -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    {#if subTab === "host"}
      <!-- 1. GROUP BY HOST TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("name")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colSourceHost")}</span>
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
            <th class="py-2 px-3">
              <span>{$_("report.colTopTags")}</span>
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
          {#if paginatedHostSummaries.length === 0}
            <tr>
              <td colspan="9" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noSyslog")}
              </td>
            </tr>
          {:else}
            {#each paginatedHostSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px] whitespace-nowrap">
                  {row.name}
                </td>
                <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-mono text-[11px] whitespace-nowrap">
                  {row.ip}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-1 px-2">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full bg-cyan-500 rounded-full" style="width: {row.percent}%"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-1 px-2 font-bold font-mono text-rose-500 dark:text-rose-400 text-[11px]">
                  {row.errorCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-amber-500 dark:text-amber-400 text-[11px]">
                  {row.warnCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-emerald-500 dark:text-emerald-400 text-[11px]">
                  {row.normalCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-mono text-[11px]">
                  {row.topTags}
                </td>
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {:else if subTab === "tag"}
      <!-- 2. GROUP BY TAG TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("tag")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTag")}</span>
                {#if sortColumn === "tag"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
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
          {#if paginatedTagSummaries.length === 0}
            <tr>
              <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noSyslog")}
              </td>
            </tr>
          {:else}
            {#each paginatedTagSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2 font-bold text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">
                  {row.tag}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-1 px-2">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full bg-cyan-500 rounded-full" style="width: {row.percent}%"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-1 px-2 font-bold font-mono text-rose-500 dark:text-rose-400 text-[11px]">
                  {row.errorCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-amber-500 dark:text-amber-400 text-[11px]">
                  {row.warnCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-emerald-500 dark:text-emerald-400 text-[11px]">
                  {row.normalCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                  {row.hostCount}
                </td>
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {:else if subTab === "facility"}
      <!-- 3. GROUP BY FACILITY TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("facilityName")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colFacility")}</span>
                {#if sortColumn === "facilityName"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("count")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colEventCount")}</span>
                {#if sortColumn === "count"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("percent")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colRatio")}</span>
                {#if sortColumn === "percent"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("errorCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-rose-600 dark:text-rose-400 font-bold">{$_("report.colError")}</span>
                {#if sortColumn === "errorCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("warnCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colWarn")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNormal")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-1 px-2 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
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
          {#if paginatedFacilitySummaries.length === 0}
            <tr>
              <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noSyslog")}
              </td>
            </tr>
          {:else}
            {#each paginatedFacilitySummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2 font-bold text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">
                  {row.facilityName} ({row.facility})
                </td>
                <td class="py-1 px-2 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-1 px-2">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full bg-cyan-500 rounded-full" style="width: {row.percent}%"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-1 px-2 font-bold font-mono text-rose-500 dark:text-rose-400 text-[11px]">
                  {row.errorCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-amber-500 dark:text-amber-400 text-[11px]">
                  {row.warnCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 font-bold font-mono text-emerald-500 dark:text-emerald-400 text-[11px]">
                  {row.normalCount.toLocaleString()}
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                  {row.hostCount}
                </td>
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
                  {row.lastTime > 0 ? formatTimeStr(row.lastTime) : "-"}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    {:else}
      <!-- 4. GROUP BY LEVEL TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("severityName")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colLevel")}</span>
                {#if sortColumn === "severityName"}
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
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("hostCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colAffectedHosts")}</span>
                {#if sortColumn === "hostCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3">
              <span>{$_("report.colTopTags")}</span>
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
              <td colspan="6" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noSyslog")}
              </td>
            </tr>
          {:else}
            {#each paginatedLevelSummaries as row}
              <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2">
                  <span
                    class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none"
                    style="background-color: {getStateColor(row.level)}20; border-color: {getStateColor(row.level)}50; color: {getStateColor(row.level)}"
                  >
                    <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(row.level)}"></span>
                    {row.severityName} ({row.severity})
                  </span>
                </td>
                <td class="py-1 px-2 font-bold font-mono text-[11px] text-slate-900 dark:text-slate-100">
                  {row.count.toLocaleString()}
                </td>
                <td class="py-1 px-2">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                      <div class="h-full rounded-full" style="width: {row.percent}%; background-color: {getStateColor(row.level)}"></div>
                    </div>
                    <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{row.percent}%</span>
                  </div>
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                  {row.hostCount}
                </td>
                <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-mono text-[11px]">
                  {row.topTags}
                </td>
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap text-[11px]">
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
