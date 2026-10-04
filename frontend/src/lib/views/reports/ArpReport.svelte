<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Eye,
    Server,
    Layers,
    ShieldAlert,
    Cpu,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { getStateColor, formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import { getVendor } from "./utils";
  import type { ParquetLogRecord, NodeEnt } from "../../api";

  let {
    arpLogs = [],
    nodes = [],
    searchQuery = "",
  }: {
    arpLogs?: ParquetLogRecord[];
    nodes?: NodeEnt[];
    searchQuery?: string;
  } = $props();

  // Subtab navigation: ip, vendor, state, level
  let subTab = $state<"ip" | "vendor" | "state" | "level">("ip");
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

  // SLA-aligned Severity Classification
  // Delete / Conflict / Spoof / Down -> Error (障害)
  // Change -> Warning (注意 / MAC変更検知)
  // New / Normal / Info -> Normal (正常 / 新規検知)
  export const isArpError = (state: string) => {
    const s = (state || "").toLowerCase();
    return (
      s === "delete" ||
      s === "down" ||
      s === "error" ||
      s === "fail" ||
      s === "conflict" ||
      s === "spoof" ||
      s === "alert" ||
      s === "high"
    );
  };

  export const isArpWarn = (state: string) => {
    if (isArpError(state)) return false;
    const s = (state || "").toLowerCase();
    return s === "change" || s === "warn" || s === "warning" || s === "update";
  };

  export const isArpNormal = (state: string) => {
    return !isArpError(state) && !isArpWarn(state);
  };

  interface NormalizedArp {
    time: number;
    ip: string;
    nodeName: string;
    state: string;
    stateName: string;
    newMac: string;
    newVendor: string;
    oldMac: string;
    oldVendor: string;
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

  // Normalize ARP records
  const normalizedLogs = $derived.by<NormalizedArp[]>(() => {
    return arpLogs.map((r) => {
      const rawLog = r.log || (r as any).Log || "";
      const parsed = parseJsonSafe(rawLog);
      const time =
        r.time || (r as any).Time || parsed.Time || parsed.time || Date.now() * 1e6;

      let state = parsed.State || parsed.state || "";
      let ip = parsed.IP || parsed.ip || r.src || (r as any).Src || "";
      let node = parsed.Node || parsed.node || "";
      let newMac = parsed.NewMAC || parsed.newMAC || "";
      let newVendor = parsed.NewVendor || parsed.newVendor || "";
      let oldMac = parsed.OldMAC || parsed.oldMAC || "";
      let oldVendor = parsed.OldVendor || parsed.oldVendor || "";

      // CSV fallback
      if (!newMac && typeof rawLog === "string" && rawLog.includes(",")) {
        const parts = rawLog.split(",");
        if (parts.length >= 3) {
          if (!state) state = parts[0];
          if (!ip) ip = parts[1];
          newMac = parts[2];
          if (parts.length > 3) oldMac = parts[3];
        }
      }

      if (!newVendor && newMac) {
        newVendor = getVendor(newMac);
      }
      if (!oldVendor && oldMac) {
        oldVendor = getVendor(oldMac);
      }
      if (!newVendor) {
        newVendor = "Unknown / Generic";
      }

      const matchedNode =
        (ip ? nodeByIp.get(ip) : undefined) ||
        (node ? nodeByName.get(node.toLowerCase()) : undefined);
      const nodeName = matchedNode?.name || node || "-";

      let level = "normal";
      let levelName = $_("report.colNormal");
      if (isArpError(state)) {
        level = "high";
        levelName = $_("report.colError");
      } else if (isArpWarn(state)) {
        level = "warn";
        levelName = $_("report.colWarn");
      }

      let stateName = state || "Unknown";
      const sLower = state.toLowerCase();
      if (sLower === "new") {
        stateName = $_("report.arpStateNew");
      } else if (sLower === "change") {
        stateName = $_("report.arpStateChange");
      } else if (sLower === "delete") {
        stateName = $_("report.arpStateDelete");
      } else if (!state) {
        stateName = $_("report.arpStateOther");
      }

      return {
        time,
        ip: ip || "unknown",
        nodeName,
        state: state || "New",
        stateName,
        newMac,
        newVendor,
        oldMac,
        oldVendor,
        level,
        levelName,
      };
    });
  });

  // Filter logs by search query
  const filteredLogs = $derived.by<NormalizedArp[]>(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return normalizedLogs;
    return normalizedLogs.filter(
      (l) =>
        l.ip.toLowerCase().includes(q) ||
        l.nodeName.toLowerCase().includes(q) ||
        l.state.toLowerCase().includes(q) ||
        l.stateName.toLowerCase().includes(q) ||
        l.newMac.toLowerCase().includes(q) ||
        l.newVendor.toLowerCase().includes(q) ||
        l.oldMac.toLowerCase().includes(q) ||
        l.oldVendor.toLowerCase().includes(q) ||
        l.levelName.toLowerCase().includes(q)
    );
  });

  // KPI Overview Statistics
  const kpiStats = $derived.by(() => {
    const total = filteredLogs.length;
    let high = 0;
    let warn = 0;
    let normal = 0;
    const uniqueIps = new Set<string>();
    const uniqueMacs = new Set<string>();
    const uniqueVendors = new Set<string>();

    for (const l of filteredLogs) {
      if (isArpError(l.state)) {
        high++;
      } else if (isArpWarn(l.state)) {
        warn++;
      } else {
        normal++;
      }
      if (l.ip && l.ip !== "unknown") uniqueIps.add(l.ip);
      if (l.newMac) uniqueMacs.add(l.newMac);
      if (l.newVendor) uniqueVendors.add(l.newVendor);
    }

    return {
      total,
      high,
      warn,
      normal,
      uniqueIps: uniqueIps.size,
      uniqueMacs: uniqueMacs.size,
      uniqueVendors: uniqueVendors.size,
    };
  });

  // 1. Group by IP Summary
  interface IpSummary {
    ip: string;
    name: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    newMac: string;
    vendor: string;
    lastTime: number;
  }

  const ipSummaries = $derived.by<IpSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        ip: string;
        name: string;
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        newMac: string;
        vendor: string;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      const key = l.ip || "__unknown__";
      let item = map.get(key);
      if (!item) {
        item = {
          ip: l.ip,
          name: l.nodeName,
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          newMac: l.newMac,
          vendor: l.newVendor,
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      if (isArpError(l.state)) {
        item.errorCount++;
      } else if (isArpWarn(l.state)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
        if (l.newMac) item.newMac = l.newMac;
        if (l.newVendor) item.vendor = l.newVendor;
      }
    }

    return Array.from(map.values()).map((d) => ({
      ip: d.ip,
      name: d.name,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      newMac: d.newMac || "-",
      vendor: d.vendor || "-",
      lastTime: d.lastTime,
    }));
  });

  // 2. Group by Vendor Summary
  interface VendorSummary {
    vendor: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    ipCount: number;
    macCount: number;
    lastTime: number;
  }

  const vendorSummaries = $derived.by<VendorSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        ips: Set<string>;
        macs: Set<string>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      const v = l.newVendor || "Unknown / Generic";
      let item = map.get(v);
      if (!item) {
        item = {
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          ips: new Set<string>(),
          macs: new Set<string>(),
          lastTime: 0,
        };
        map.set(v, item);
      }
      item.count++;
      if (isArpError(l.state)) {
        item.errorCount++;
      } else if (isArpWarn(l.state)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      if (l.ip && l.ip !== "unknown") item.ips.add(l.ip);
      if (l.newMac) item.macs.add(l.newMac);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([vendor, d]) => ({
      vendor,
      count: d.count,
      percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
      errorCount: d.errorCount,
      warnCount: d.warnCount,
      normalCount: d.normalCount,
      ipCount: d.ips.size,
      macCount: d.macs.size,
      lastTime: d.lastTime,
    }));
  });

  // 3. Group by State Summary
  interface StateSummary {
    state: string;
    stateName: string;
    count: number;
    percent: number;
    errorCount: number;
    warnCount: number;
    normalCount: number;
    ipCount: number;
    macCount: number;
    topVendors: string;
    lastTime: number;
  }

  const stateSummaries = $derived.by<StateSummary[]>(() => {
    const total = filteredLogs.length;
    const map = new Map<
      string,
      {
        stateName: string;
        count: number;
        errorCount: number;
        warnCount: number;
        normalCount: number;
        ips: Set<string>;
        macs: Set<string>;
        vendors: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let item = map.get(l.state);
      if (!item) {
        item = {
          stateName: l.stateName,
          count: 0,
          errorCount: 0,
          warnCount: 0,
          normalCount: 0,
          ips: new Set<string>(),
          macs: new Set<string>(),
          vendors: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(l.state, item);
      }
      item.count++;
      if (isArpError(l.state)) {
        item.errorCount++;
      } else if (isArpWarn(l.state)) {
        item.warnCount++;
      } else {
        item.normalCount++;
      }
      if (l.ip && l.ip !== "unknown") item.ips.add(l.ip);
      if (l.newMac) item.macs.add(l.newMac);
      if (l.newVendor) {
        item.vendors.set(l.newVendor, (item.vendors.get(l.newVendor) || 0) + 1);
      }
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.entries()).map(([state, d]) => {
      const sortedVendors = Array.from(d.vendors.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([v, c]) => `${v} (${c})`)
        .join(", ");

      return {
        state,
        stateName: d.stateName,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        errorCount: d.errorCount,
        warnCount: d.warnCount,
        normalCount: d.normalCount,
        ipCount: d.ips.size,
        macCount: d.macs.size,
        topVendors: sortedVendors || "-",
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
    ipCount: number;
    topStates: string;
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
        ips: Set<string>;
        states: Map<string, number>;
        lastTime: number;
      }
    >();

    for (const l of filteredLogs) {
      let key = "normal";
      let keyName = $_("report.colNormal");
      if (isArpError(l.state)) {
        key = "high";
        keyName = $_("report.colError");
      } else if (isArpWarn(l.state)) {
        key = "warn";
        keyName = $_("report.colWarn");
      }

      let item = map.get(key);
      if (!item) {
        item = {
          level: key,
          levelName: keyName,
          count: 0,
          ips: new Set<string>(),
          states: new Map<string, number>(),
          lastTime: 0,
        };
        map.set(key, item);
      }
      item.count++;
      if (l.ip && l.ip !== "unknown") item.ips.add(l.ip);
      item.states.set(l.stateName, (item.states.get(l.stateName) || 0) + 1);
      if (l.time >= item.lastTime) {
        item.lastTime = l.time;
      }
    }

    return Array.from(map.values()).map((d) => {
      const sortedStates = Array.from(d.states.entries())
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([s, c]) => `${s} (${c})`)
        .join(", ");

      return {
        level: d.level,
        levelName: d.levelName,
        count: d.count,
        percent: total > 0 ? Number(((d.count / total) * 100).toFixed(1)) : 0,
        ipCount: d.ips.size,
        topStates: sortedStates || "-",
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
        colKey === "ip" ||
        colKey === "vendor" ||
        colKey === "stateName" ||
        colKey === "levelName"
          ? "asc"
          : "desc";
    }
  };

  const handleSelectSubTab = (tab: "ip" | "vendor" | "state" | "level") => {
    subTab = tab;
    currentPage = 1;
    sortColumn = "count";
    sortDirection = "desc";
  };

  // Sort & paginate by IP
  const sortedIpSummaries = $derived(
    [...ipSummaries].sort((a: any, b: any) => {
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
  const paginatedIpSummaries = $derived(
    pageSize === -1
      ? sortedIpSummaries
      : sortedIpSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by Vendor
  const sortedVendorSummaries = $derived(
    [...vendorSummaries].sort((a: any, b: any) => {
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
  const paginatedVendorSummaries = $derived(
    pageSize === -1
      ? sortedVendorSummaries
      : sortedVendorSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by State
  const sortedStateSummaries = $derived(
    [...stateSummaries].sort((a: any, b: any) => {
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
  const paginatedStateSummaries = $derived(
    pageSize === -1
      ? sortedStateSummaries
      : sortedStateSummaries.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  // Sort & paginate by Level
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
    subTab === "ip"
      ? sortedIpSummaries.length
      : subTab === "vendor"
      ? sortedVendorSummaries.length
      : subTab === "state"
      ? sortedStateSummaries.length
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

    if (subTab === "ip") {
      const topIps = [...ipSummaries].sort((a, b) => b.count - a.count).slice(0, 5).reverse();
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
          data: topIps.map((h) => (h.name && h.name !== "-" ? h.name : h.ip)),
          axisLabel: { color: textColor, fontSize: 9 },
        },
        series: [
          {
            type: "bar",
            data: topIps.map((h) => h.count),
            itemStyle: { color: "#38bdf8", borderRadius: [0, 4, 4, 0] },
          },
        ],
      };
    } else if (subTab === "vendor") {
      const topVendors = [...vendorSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topVendors.map((t) => ({ name: t.vendor, value: t.count }));
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
    } else if (subTab === "state") {
      const topStates = [...stateSummaries].sort((a, b) => b.count - a.count).slice(0, 6);
      const data = topStates.map((s) => ({
        name: s.stateName,
        value: s.count,
        itemStyle: {
          color: isArpError(s.state) ? "#f43f5e" : isArpWarn(s.state) ? "#f59e0b" : "#10b981",
        },
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
    if (subTab === "ip") {
      csv =
        "IP,NodeName,Count,Ratio,Error,Change,New,CurrentMAC,Vendor,LastOccurrence\n" +
        sortedIpSummaries
          .map(
            (h) =>
              `"${h.ip}","${h.name}",${h.count},"${h.percent}%",${h.errorCount},${h.warnCount},${h.normalCount},"${h.newMac}","${h.vendor}","${h.lastTime > 0 ? formatTimeStr(h.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "vendor") {
      csv =
        "Vendor,Count,Ratio,Error,Change,New,TargetIPs,TargetMACs,LastOccurrence\n" +
        sortedVendorSummaries
          .map(
            (v) =>
              `"${v.vendor}",${v.count},"${v.percent}%",${v.errorCount},${v.warnCount},${v.normalCount},${v.ipCount},${v.macCount},"${v.lastTime > 0 ? formatTimeStr(v.lastTime) : ""}"`
          )
          .join("\n");
    } else if (subTab === "state") {
      csv =
        "State,StateName,Count,Ratio,Error,Warn,Normal,TargetIPs,TargetMACs,TopVendors,LastOccurrence\n" +
        sortedStateSummaries
          .map(
            (s) =>
              `"${s.state}","${s.stateName}",${s.count},"${s.percent}%",${s.errorCount},${s.warnCount},${s.normalCount},${s.ipCount},${s.macCount},"${s.topVendors}","${s.lastTime > 0 ? formatTimeStr(s.lastTime) : ""}"`
          )
          .join("\n");
    } else {
      csv =
        "Level,LevelName,Count,Ratio,TargetIPs,TopStates,LastOccurrence\n" +
        sortedLevelSummaries
          .map(
            (l) =>
              `"${l.level}","${l.levelName}",${l.count},"${l.percent}%",${l.ipCount},"${l.topStates}","${l.lastTime > 0 ? formatTimeStr(l.lastTime) : ""}"`
          )
          .join("\n");
    }

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_arp_${subTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Title Header -->
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Eye class="w-5 h-5 text-cyan-400" />
      {$_("report.arpTitle")}
    </h2>
    <p class="text-xs text-slate-400 mt-1">{$_("report.arpSubtitle")}</p>
  </div>

  <!-- KPI Overview with Mini ECharts (SLA-aligned: LOW is Error) -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalArpLogs")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {kpiStats.total.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.totalArpLogsSub")}
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
        <span class="text-xs font-semibold text-slate-400">{$_("report.colMacChange")}</span>
        <div class="text-2xl font-bold font-mono text-amber-400">
          {kpiStats.warn.toLocaleString()}
          <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.warnLowEventsSub")}</div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.colNewDevice")}</span>
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
          {#if subTab === "ip"}
            {$_("report.arpIpDistribution")}
          {:else if subTab === "vendor"}
            {$_("report.arpVendorDistribution")}
          {:else if subTab === "state"}
            {$_("report.arpStateDistribution")}
          {:else}
            {$_("report.levelDistribution")}
          {/if}
        </span>
        <span class="text-[10px] font-mono text-slate-400">
          {#if subTab === "ip"}
            {kpiStats.uniqueIps} IPs
          {:else if subTab === "vendor"}
            {kpiStats.uniqueVendors} vendors
          {:else if subTab === "state"}
            {stateSummaries.length} states
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
      onclick={() => handleSelectSubTab("ip")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'ip' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Server class="w-3.5 h-3.5" />
      <span>{$_("report.subArpIp")} ({ipSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("vendor")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'vendor' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Cpu class="w-3.5 h-3.5" />
      <span>{$_("report.subArpVendor")} ({vendorSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("state")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'state' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{$_("report.subArpState")} ({stateSummaries.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleSelectSubTab("level")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {subTab === 'level' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <ShieldAlert class="w-3.5 h-3.5" />
      <span>{$_("report.subArpLevel")} ({levelSummaries.length})</span>
    </button>
  </div>

  <!-- Tables Container with Pagination -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    {#if subTab === "ip"}
      <!-- 1. GROUP BY IP TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
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
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("name")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colNodeName")}</span>
                {#if sortColumn === "name"}
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
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colMacChange")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNewDevice")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colMac")}</span>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colVendor")}</span>
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
          {#each paginatedIpSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">
                {item.ip}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.name}
              </td>
              <td class="py-1 px-2 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.newMac}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.vendor}>
                {item.vendor}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="10" class="py-8 text-center text-slate-400">
                {$_("report.noArpLogs")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if subTab === "vendor"}
      <!-- 2. GROUP BY VENDOR TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("vendor")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colVendor")}</span>
                {#if sortColumn === "vendor"}
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
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colMacChange")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNewDevice")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("ipCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetIps")}</span>
                {#if sortColumn === "ipCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("macCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetMacs")}</span>
                {#if sortColumn === "macCount"}
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
          {#each paginatedVendorSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-200 truncate max-w-sm" title={item.vendor}>
                {item.vendor}
              </td>
              <td class="py-1 px-2 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.ipCount}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.macCount}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="9" class="py-8 text-center text-slate-400">
                {$_("report.noArpLogs")}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else if subTab === "state"}
      <!-- 3. GROUP BY STATE TABLE -->
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("stateName")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colStatus")}</span>
                {#if sortColumn === "stateName"}
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
                <span class="text-amber-600 dark:text-amber-400 font-bold">{$_("report.colMacChange")}</span>
                {#if sortColumn === "warnCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("normalCount")}>
              <div class="inline-flex items-center gap-1">
                <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_("report.colNewDevice")}</span>
                {#if sortColumn === "normalCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("ipCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetIps")}</span>
                {#if sortColumn === "ipCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("macCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetMacs")}</span>
                {#if sortColumn === "macCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colTopVendors")}</span>
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
          {#each paginatedStateSummaries as item}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-200 whitespace-nowrap">
                {item.stateName}
              </td>
              <td class="py-1 px-2 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.errorCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-rose-500/10 text-rose-500">
                    {item.errorCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.warnCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-amber-500/10 text-amber-500">
                    {item.warnCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                {#if item.normalCount > 0}
                  <span class="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-semibold bg-emerald-500/10 text-emerald-500">
                    {item.normalCount.toLocaleString()}
                  </span>
                {:else}
                  <span class="text-slate-400 dark:text-slate-600">0</span>
                {/if}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.ipCount}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.macCount}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topVendors}>
                {item.topVendors}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="10" class="py-8 text-center text-slate-400">
                {$_("report.noArpLogs")}
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
            <th class="py-2 px-3 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200 whitespace-nowrap" onclick={() => handleSort("ipCount")}>
              <div class="inline-flex items-center gap-1">
                <span>{$_("report.colTargetIps")}</span>
                {#if sortColumn === "ipCount"}
                  {#if sortDirection === "asc"}<ArrowUp class="h-2.5 w-2.5 text-cyan-400" />{:else}<ArrowDown class="h-2.5 w-2.5 text-cyan-400" />{/if}
                {:else}
                  <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                {/if}
              </div>
            </th>
            <th class="py-2 px-3 text-slate-600 dark:text-slate-400 whitespace-nowrap">
              <span>{$_("report.colTopStates")}</span>
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
              <td class="py-1 px-2 whitespace-nowrap">
                <span
                  class="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-bold"
                  style="background-color: {getStateColor(item.level)}20; color: {getStateColor(item.level)};"
                >
                  {item.levelName}
                </span>
              </td>
              <td class="py-1 px-2 font-bold text-cyan-500 whitespace-nowrap">
                {item.count.toLocaleString()}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                <span class="inline-block w-12 text-right">{item.percent}%</span>
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.ipCount}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 truncate max-w-xs" title={item.topStates}>
                {item.topStates}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 whitespace-nowrap">
                {item.lastTime > 0 ? formatTimeStr(item.lastTime) : "-"}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="py-8 text-center text-slate-400">
                {$_("report.noArpLogs")}
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
