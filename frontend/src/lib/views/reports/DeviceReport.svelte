<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Laptop,
    Trash2,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Plus,
    Layers,
    Server,
    CheckCircle2,
    AlertTriangle,
    Shield,
  } from "@lucide/svelte";
  import NodeDialog from "../../components/NodeDialog.svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { getStateColor, getStateName, formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import { getVendor, isVirtualMachine, ipToNum } from "./utils";
  import {
    deleteArpEntries,
    resetArpTable,
    type NodeEnt,
    type ArpEnt,
    type ParquetLogRecord,
  } from "../../api";

  let {
    searchQuery = "",
    nodes = [],
    arpList = [],
    arpLogs = [],
    onRefresh = () => {},
    loading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    arpList?: ArpEnt[];
    arpLogs?: ParquetLogRecord[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  // Subfilter & pagination states
  let deviceSubFilter = $state<"all" | "vm" | "managed" | "unmanaged" | "problem">("all");
  let sortColumn = $state("ip");
  let sortDirection = $state<"asc" | "desc">("asc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // Modal states
  let showAddNodeModal = $state(false);
  let nodeToEdit = $state<NodeEnt | null>(null);

  // ECharts container & instance
  let vendorChartElem = $state<HTMLDivElement | null>(null);
  let vendorChartInstance: echarts.ECharts | null = null;

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "asc";
    }
  };

  // Analyze ARP log records for IP and MAC changes
  const arpChanges = $derived.by(() => {
    const changeIP = new Map<string, number>();
    const newIP = new Map<string, number>();
    const changeMAC = new Map<string, boolean>();

    for (const logRec of arpLogs) {
      let ent: any = logRec;
      if (typeof logRec.Log === "string" && logRec.Log.startsWith("{")) {
        try {
          ent = JSON.parse(logRec.Log);
        } catch (_) {}
      }
      const logTime = ent.Time || logRec.Time || 0;
      const ip = ent.IP || logRec.Src || "";
      const state = ent.State || ent.state || "";
      if (state === "Change") {
        if (ip) changeIP.set(ip, logTime);
        if (ent.NewMAC) changeMAC.set(String(ent.NewMAC).toUpperCase(), true);
        if (ent.OldMAC) changeMAC.set(String(ent.OldMAC).toUpperCase(), true);
      } else if (state === "New") {
        if (ip && !newIP.has(ip)) newIP.set(ip, logTime);
      }
    }
    return { changeIP, newIP, changeMAC };
  });

  // Merge nodes with discovered ARP devices
  const allDevices = $derived.by(() => {
    const list: any[] = [];
    const seenIPs = new Set<string>();
    const seenMACs = new Set<string>();

    const arpByIP = new Map<string, ArpEnt>();
    const arpByMAC = new Map<string, ArpEnt>();
    for (const a of arpList) {
      if (a.IP) arpByIP.set(a.IP, a);
      if (a.MAC) arpByMAC.set(a.MAC.toUpperCase(), a);
    }

    for (const n of nodes) {
      const cleanMAC = (n.mac || "").toUpperCase();
      const matchedArp = (n.ip ? arpByIP.get(n.ip) : undefined) || (cleanMAC ? arpByMAC.get(cleanMAC) : undefined);
      list.push({
        id: n.id,
        nodeId: n.id,
        name: n.name,
        ip: n.ip,
        mac: n.mac || (matchedArp?.MAC ?? ""),
        vendor: (n as any).vendor || (n as any).Vendor || (matchedArp?.Vendor ?? getVendor(n.mac || matchedArp?.MAC || "")),
        addr_mode: (n as any).addr_mode || "IP",
        state: n.state,
        isManaged: true,
        firstTime: matchedArp?.FirstTime || 0,
        lastTime: matchedArp?.LastTime || 0,
      });
      if (n.ip) seenIPs.add(n.ip);
      if (cleanMAC) seenMACs.add(cleanMAC);
    }

    for (const a of arpList) {
      const cleanMAC = (a.MAC || "").toUpperCase();
      if (!seenIPs.has(a.IP) && (!cleanMAC || !seenMACs.has(cleanMAC))) {
        list.push({
          id: `arp-${a.IP}`,
          nodeId: a.NodeID || "",
          name: a.Vendor ? a.Vendor : $_("report.unmanagedDeviceName", { values: { ip: a.IP } }),
          ip: a.IP,
          mac: a.MAC || "",
          vendor: a.Vendor || getVendor(a.MAC || ""),
          addr_mode: "ARP",
          state: "info",
          isManaged: false,
          firstTime: a.FirstTime || 0,
          lastTime: a.LastTime || 0,
        });
        seenIPs.add(a.IP);
        if (cleanMAC) seenMACs.add(cleanMAC);
      }
    }

    // Duplicate detection across full IP/MAC dataset
    const ipCounts = new Map<string, number>();
    const macCounts = new Map<string, number>();
    for (const d of list) {
      if (d.ip && d.ip !== "0.0.0.0") {
        ipCounts.set(d.ip, (ipCounts.get(d.ip) || 0) + 1);
      }
      if (d.mac && d.mac !== "-") {
        const m = d.mac.toUpperCase();
        macCounts.set(m, (macCounts.get(m) || 0) + 1);
      }
    }

    // Determine state for each device
    for (const d of list) {
      const m = (d.mac || "").toUpperCase();
      const isDup = (d.ip && (ipCounts.get(d.ip) || 0) > 1) || Boolean(m && m !== "-" && (macCounts.get(m) || 0) > 1);
      const isDhcpErr = !d.ip || d.ip.startsWith("169.254.") || d.ip === "0.0.0.0";
      const isIpChg = arpChanges.changeIP.has(d.ip);
      const isMacChg = Boolean(m && arpChanges.changeMAC.has(m));

      let addressState: "duplicate" | "dhcpError" | "ipChanged" | "macChanged" | "normal" = "normal";
      if (isDup) {
        addressState = "duplicate";
      } else if (isDhcpErr) {
        addressState = "dhcpError";
      } else if (isIpChg) {
        addressState = "ipChanged";
      } else if (isMacChg) {
        addressState = "macChanged";
      }

      d.addressState = addressState;
      d.isDuplicate = isDup;
      d.isDhcpError = isDhcpErr;
      d.isIpChanged = isIpChg;
      d.isMacChanged = isMacChg;

      let lastChg = 0;
      if (isIpChg) {
        lastChg = arpChanges.changeIP.get(d.ip) || 0;
      } else if (arpChanges.newIP.has(d.ip)) {
        lastChg = arpChanges.newIP.get(d.ip) || 0;
      }
      d.lastChangeTime = lastChg;
    }

    return list;
  });

  const kpiCounts = $derived.by(() => {
    let normal = 0;
    let duplicate = 0;
    let changed = 0;
    let dhcpError = 0;
    for (const d of allDevices) {
      if (d.addressState === "normal") normal++;
      else if (d.addressState === "duplicate") duplicate++;
      else if (d.addressState === "ipChanged" || d.addressState === "macChanged") changed++;
      else if (d.addressState === "dhcpError") dhcpError++;
    }
    return { normal, duplicate, changed, dhcpError };
  });

  const filteredDevices = $derived(
    allDevices.filter((n) => {
      const q = searchQuery.toLowerCase();
      return (
        n.name.toLowerCase().includes(q) ||
        n.ip.toLowerCase().includes(q) ||
        (n.mac && n.mac.toLowerCase().includes(q)) ||
        (n.vendor && n.vendor.toLowerCase().includes(q))
      );
    })
  );

  const sortedDevices = $derived(
    [...filteredDevices].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];

      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";

      let comparison = 0;
      if (sortColumn === "ip") {
        const numA = ipToNum(String(valA));
        const numB = ipToNum(String(valB));
        if (numA !== -1 && numB !== -1) {
          comparison = numA - numB;
        } else {
          comparison = String(valA).localeCompare(String(valB));
        }
      } else if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }

      return sortDirection === "asc" ? comparison : -comparison;
    })
  );

  const filteredDevicesBySubtype = $derived.by(() => {
    return sortedDevices.filter((d) => {
      if (deviceSubFilter === "all") return true;
      if (deviceSubFilter === "vm") return isVirtualMachine(d);
      if (deviceSubFilter === "managed") return d.isManaged;
      if (deviceSubFilter === "unmanaged") return !d.isManaged;
      if (deviceSubFilter === "problem") return d.addressState !== "normal";
      return true;
    });
  });

  const paginatedDevices = $derived.by(() => {
    if (pageSize === -1) return filteredDevicesBySubtype;
    const start = (currentPage - 1) * pageSize;
    return filteredDevicesBySubtype.slice(start, start + pageSize);
  });

  const renderVendorChart = () => {
    if (!vendorChartElem || allDevices.length === 0) return;
    if (vendorChartInstance) {
      vendorChartInstance.dispose();
    }
    const isDark = isDarkMode();
    vendorChartInstance = echarts.init(vendorChartElem, isDark ? "dark" : undefined);

    const vendorMap = new Map<string, number>();
    for (const d of allDevices) {
      const v = d.vendor || "Unknown";
      vendorMap.set(v, (vendorMap.get(v) || 0) + 1);
    }
    const topVendors = Array.from(vendorMap.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 6)
      .reverse();

    const yData = topVendors.map(([name]) => name);
    const seriesData = topVendors.map(([, count]) => count);

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: (params: any) => {
          if (!params || !params.length) return "";
          const item = params[0];
          return `<b>${item.name}</b>: ${item.value} ${$_("report.unitDevices")}`;
        },
      },
      grid: {
        top: 8,
        bottom: 20,
        left: 110,
        right: 32,
        containLabel: false,
      },
      xAxis: {
        type: "value",
        minInterval: 1,
        splitLine: {
          lineStyle: {
            color: isDark ? "#334155" : "#e2e8f0",
            type: "dashed",
          },
        },
        axisLabel: {
          fontSize: 10,
          color: isDark ? "#94a3b8" : "#64748b",
        },
      },
      yAxis: {
        type: "category",
        data: yData,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
          fontSize: 10,
          color: isDark ? "#cbd5e1" : "#475569",
          formatter: (val: string) => {
            return val.length > 12 ? val.slice(0, 11) + "…" : val;
          },
        },
      },
      series: [
        {
          name: "Devices",
          type: "bar",
          data: seriesData,
          itemStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
              { offset: 0, color: "#38bdf8" },
              { offset: 1, color: "#0284c7" },
            ]),
            borderRadius: [0, 4, 4, 0],
          },
          label: {
            show: true,
            position: "right",
            fontSize: 10,
            fontFamily: "monospace",
            color: isDark ? "#94a3b8" : "#64748b",
          },
        },
      ],
    };

    vendorChartInstance.setOption(option);
  };

  $effect(() => {
    if (allDevices.length > 0 && vendorChartElem) {
      tick().then(() => renderVendorChart());
    }
  });

  onMount(() => {
    const handleResize = () => {
      if (vendorChartInstance) vendorChartInstance.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      if (vendorChartInstance) vendorChartInstance.dispose();
    };
  });

  const handleResetArp = async () => {
    if (!confirm($_("report.confirmResetArp"))) return;
    try {
      await resetArpTable();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  };

  const handleDeleteArp = async (ip: string, mac: string) => {
    if (!confirm($_("report.confirmDeleteArp", { values: { ip, mac } }))) return;
    try {
      await deleteArpEntries([ip]);
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  const handleOpenAddNode = (device: any) => {
    nodeToEdit = {
      id: "",
      name: device.name && !device.name.includes("未管理") ? device.name : (device.vendor ? `${device.vendor}_${device.ip}` : device.ip),
      ip: device.ip || "",
      mac: device.mac || "",
      descr: `Auto detected via ARP Watch (Vendor: ${device.vendor || "Unknown"})`,
      icon: "desktop",
      state: "normal",
      addr_mode: "ip",
      auto_ack: false,
      x: 0,
      y: 0,
    };
    showAddNodeModal = true;
  };

  export function exportCSV(): void {
    const csv = "State,IP,MAC,Node,Vendor,LastChange,FirstTime,LastTime,Managed\n" + sortedDevices.map((n) => `"${n.addressState}","${n.ip}","${n.mac || ''}","${n.name}","${n.vendor || getVendor(n.mac || '')}","${n.lastChangeTime > 0 ? formatTimeStr(n.lastChangeTime) : ''}","${n.firstTime > 0 ? formatTimeStr(n.firstTime) : ''}","${n.lastTime > 0 ? formatTimeStr(n.lastTime) : ''}","${n.isManaged ? 'yes' : 'no'}"`).join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_device_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Laptop class="w-5 h-5 text-cyan-400" />
        {$_("report.deviceTitle")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.deviceSubtitle")}</p>
    </div>
  </div>

  <!-- KPI Cards with Vendor Graph -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.totalDevices")}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {allDevices.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[10px] text-slate-400">
          {$_("report.deviceNodesSub", { values: { nodes: nodes.length, unmanaged: allDevices.filter(d => !d.isManaged).length } })}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.runningDevices")}</span>
        <div class="text-2xl font-bold font-mono text-emerald-400">
          {kpiCounts.normal} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[10px] text-slate-400">{$_("report.runningDevicesSub")}</div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{$_("report.alertDevices")}</span>
        <div class="text-2xl font-bold font-mono text-rose-400">
          {kpiCounts.duplicate + kpiCounts.changed + kpiCounts.dhcpError} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span>
        </div>
        <div class="text-[10px] text-slate-400 flex items-center gap-1.5">
          <span class="text-rose-400">{$_("report.stateDuplicate")}: {kpiCounts.duplicate}</span>
          <span>•</span>
          <span class="text-amber-400">{$_("report.stateChanged")}: {kpiCounts.changed}</span>
          <span>•</span>
          <span class="text-orange-400">DHCP: {kpiCounts.dhcpError}</span>
        </div>
      </div>
    </div>

    <!-- Vendor Bar Chart -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("report.vendorDistribution")}</span>
        <span class="text-[10px] font-mono text-slate-400">Top 6</span>
      </div>
      <div bind:this={vendorChartElem} class="h-24 w-full"></div>
    </div>
  </div>

  <!-- Subfilter buttons -->
  <div class="flex flex-wrap items-center gap-2">
    <button
      type="button"
      onclick={() => (deviceSubFilter = "all")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Layers class="w-3.5 h-3.5" />
      <span>{$_("report.subAllDevices")} ({allDevices.length})</span>
    </button>
    <button
      type="button"
      onclick={() => (deviceSubFilter = "vm")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'vm' ? 'bg-indigo-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Server class="w-3.5 h-3.5 text-indigo-300" />
      <span>{$_("report.subVmOnly")} ({allDevices.filter(d => isVirtualMachine(d)).length})</span>
    </button>
    <button
      type="button"
      onclick={() => (deviceSubFilter = "managed")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'managed' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <CheckCircle2 class="w-3.5 h-3.5 text-emerald-300" />
      <span>{$_("report.subManagedOnly")} ({allDevices.filter(d => d.isManaged).length})</span>
    </button>
    <button
      type="button"
      onclick={() => (deviceSubFilter = "unmanaged")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'unmanaged' ? 'bg-slate-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <Shield class="w-3.5 h-3.5 text-slate-300" />
      <span>{$_("report.subUnmanagedOnly")} ({allDevices.filter(d => !d.isManaged).length})</span>
    </button>
    <button
      type="button"
      onclick={() => (deviceSubFilter = "problem")}
      class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'problem' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
    >
      <AlertTriangle class="w-3.5 h-3.5 text-rose-300" />
      <span>{$_("report.subProblemOnly")} ({allDevices.filter(d => d.addressState !== 'normal').length})</span>
    </button>
  </div>

  <!-- Device Table Container -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <table class="w-full text-left text-xs border-collapse">
      <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
        <tr>
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("addressState")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colState")}</span>
              {#if sortColumn === "addressState"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("ip")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colIp")}</span>
              {#if sortColumn === "ip"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("mac")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colMac")}</span>
              {#if sortColumn === "mac"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("name")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colNodeName")}</span>
              {#if sortColumn === "name"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("vendor")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colVendor")}</span>
              {#if sortColumn === "vendor"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastChangeTime")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colLastChange")}</span>
              {#if sortColumn === "lastChangeTime"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("firstTime")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colFirstTime")}</span>
              {#if sortColumn === "firstTime"}
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
          <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("lastTime")}>
            <div class="inline-flex items-center gap-1">
              <span>{$_("report.colLastTime")}</span>
              {#if sortColumn === "lastTime"}
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
          <th class="py-1.5 px-2.5 text-right font-sans">{$_("report.colAction")}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
        {#if paginatedDevices.length === 0}
          <tr>
            <td colspan="9" class="p-6 text-center text-slate-500 font-sans">
              {$_("report.noDevices")}
            </td>
          </tr>
        {:else}
          {#each paginatedDevices as d}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 whitespace-nowrap">
                {#if d.addressState === "duplicate"}
                  <span class="rounded bg-rose-100 dark:bg-rose-500/20 border border-rose-300 dark:border-rose-500/40 px-1.5 py-0.5 text-[9px] font-bold text-rose-700 dark:text-rose-400 leading-none">
                    {$_("report.stateDuplicate")}
                  </span>
                {:else if d.addressState === "dhcpError"}
                  <span class="rounded bg-orange-100 dark:bg-orange-500/20 border border-orange-300 dark:border-orange-500/40 px-1.5 py-0.5 text-[9px] font-bold text-orange-700 dark:text-orange-400 leading-none">
                    {$_("report.stateDhcpError")}
                  </span>
                {:else if d.addressState === "ipChanged"}
                  <span class="rounded bg-amber-100 dark:bg-amber-500/20 border border-amber-300 dark:border-amber-500/40 px-1.5 py-0.5 text-[9px] font-bold text-amber-700 dark:text-amber-400 leading-none">
                    {$_("report.stateIpChanged")}
                  </span>
                {:else if d.addressState === "macChanged"}
                  <span class="rounded bg-amber-100 dark:bg-amber-500/20 border border-amber-300 dark:border-amber-500/40 px-1.5 py-0.5 text-[9px] font-bold text-amber-700 dark:text-amber-400 leading-none">
                    {$_("report.stateMacChanged")}
                  </span>
                {:else}
                  <span class="rounded bg-emerald-100 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 px-1.5 py-0.5 text-[9px] font-medium text-emerald-700 dark:text-emerald-400 leading-none">
                    {$_("report.stateNormal")}
                  </span>
                {/if}
              </td>
              <td class="py-1 px-2 font-mono text-cyan-600 dark:text-cyan-400 text-[11px] whitespace-nowrap">
                {d.ip || "-"}
              </td>
              <td class="py-1 px-2 font-mono text-[11px] text-slate-600 dark:text-slate-400 whitespace-nowrap">
                {d.mac || "-"}
              </td>
              <td class="py-1 px-2 font-sans font-medium text-slate-900 dark:text-slate-100 text-[11px] max-w-[140px] truncate" title={d.name}>
                {d.name}
              </td>
              <td class="py-1 px-2 font-sans text-slate-700 dark:text-slate-300 text-[11px] max-w-[160px] truncate" title={d.vendor}>
                {d.vendor}
              </td>
              <td class="py-1 px-2 font-mono text-[10px] text-slate-500 whitespace-nowrap">
                {d.lastChangeTime > 0 ? formatTimeStr(d.lastChangeTime) : "-"}
              </td>
              <td class="py-1 px-2 font-mono text-[10px] text-slate-500 whitespace-nowrap">
                {d.firstTime > 0 ? formatTimeStr(d.firstTime) : "-"}
              </td>
              <td class="py-1 px-2 font-mono text-[10px] text-slate-500 whitespace-nowrap">
                {d.lastTime > 0 ? formatTimeStr(d.lastTime) : "-"}
              </td>
              <td class="py-1 px-2 text-right whitespace-nowrap font-sans">
                <div class="inline-flex items-center gap-1 justify-end">
                  {#if !d.isManaged && !d.isDhcpError}
                    <button
                      type="button"
                      onclick={() => handleOpenAddNode(d)}
                      class="rounded p-1 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-50 dark:hover:bg-cyan-950/50 hover:text-cyan-700 dark:hover:text-cyan-300 transition-colors cursor-pointer"
                      title={$_("report.btnAddNode")}
                    >
                      <Plus class="h-3.5 w-3.5" />
                    </button>
                  {/if}
                  {#if !d.isManaged && d.ip}
                    <button
                      type="button"
                      onclick={() => handleDeleteArp(d.ip, d.mac)}
                      class="rounded p-1 text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                      title={$_("report.deleteEntryTitle")}
                    >
                      <Trash2 class="h-3.5 w-3.5" />
                    </button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>

    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={filteredDevicesBySubtype.length}
    />
  </div>
</div>

<NodeDialog
  bind:show={showAddNodeModal}
  node={nodeToEdit}
  onSave={() => {
    onRefresh();
  }}
/>
