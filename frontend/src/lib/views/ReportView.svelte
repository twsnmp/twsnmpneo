<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _, locale } from "svelte-i18n";
  import { get } from "svelte/store";
  import * as echarts from "echarts";
  import {
    fetchNodes,
    fetchPollings,
    fetchEventLogs,
    fetchArpTable,
    fetchIPAM,
    deleteArpEntries,
    resetArpTable,
    type NodeEnt,
    type PollingEnt,
    type EventLogEnt,
    type ArpEnt,
    type IPAMReportResp,
    type IPAMRangeEnt
  } from "../api";
  import { getStateColor, getStateName, formatTimeStr } from "../common";
  import { isDarkMode } from "../charts/utils";
  import {
    Laptop,
    Network,
    Activity,
    ShieldCheck,
    Thermometer,
    Sparkles,
    Search,
    RefreshCw,
    Download,
    CheckCircle2,
    AlertTriangle,
    Layers,
    FileText,
    BarChart3,
    Trash2,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    CornerUpLeft,
    FolderTree,
    Grid
  } from "@lucide/svelte";

  type ReportCategory = "device" | "ipam" | "polling" | "flow" | "event" | "cert" | "sensor" | "ai";

  let activeReport = $state<ReportCategory>("device");
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let logs = $state<EventLogEnt[]>([]);
  let arpList = $state<ArpEnt[]>([]);
  let ipamReport = $state<IPAMReportResp>({
    Ranges: [],
    TotalRanges: 0,
    TotalSize: 0,
    TotalUsed: 0,
    TotalUsage: 0,
  });
  let selectedRangeIndex = $state<number>(0);
  let selectedSubnetBlock = $state<string | null>(null);
  let selectedHostInfo = $state<any | null>(null);
  let loading = $state(false);
  let searchQuery = $state("");

  const categories = $derived<{ id: ReportCategory; name: string; icon: any; count?: number }[]>([
    { id: "device", name: $_("report.tabDevice"), icon: Laptop },
    { id: "ipam", name: $_("report.tabIpam"), icon: Network },
    { id: "polling", name: $_("report.tabPolling"), icon: Activity },
    { id: "flow", name: $_("report.tabFlow"), icon: BarChart3 },
    { id: "event", name: $_("report.tabEvent"), icon: FileText },
    { id: "cert", name: $_("report.tabCert"), icon: ShieldCheck },
    { id: "sensor", name: $_("report.tabSensor"), icon: Thermometer },
    { id: "ai", name: $_("report.tabAi"), icon: Sparkles },
  ]);

  const loadData = async () => {
    loading = true;
    try {
      const [n, p, l, a, ipam] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
        fetchEventLogs().catch(() => []),
        fetchArpTable().catch(() => []),
        fetchIPAM().catch(() => ({ Ranges: [], TotalRanges: 0, TotalSize: 0, TotalUsed: 0, TotalUsage: 0 })),
      ]);
      nodes = n;
      pollings = p;
      logs = l;
      arpList = a;
      ipamReport = ipam;
      if (selectedRangeIndex >= ipam.Ranges.length) {
        selectedRangeIndex = 0;
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  onMount(() => {
    loadData();

    const observer = new MutationObserver(() => {
      if (activeReport === "ipam" && ipamReport.Ranges.length > 0) {
        renderIPAMHeatmap();
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

    return () => {
      observer.disconnect();
    };
  });

  // Helper for vendor resolution from MAC OUI
  const getVendor = (mac: string) => {
    if (!mac) return "Unknown / Generic";
    const clean = mac.replace(/[:-]/g, "").toUpperCase();
    if (clean.startsWith("525400") || clean.startsWith("00163E") || clean.startsWith("080027")) return "QEMU / KVM / Virtual";
    if (clean.startsWith("000C29") || clean.startsWith("005056")) return "VMware";
    if (clean.startsWith("001A2B") || clean.startsWith("00000C")) return "Cisco Systems";
    if (clean.startsWith("00A0DE") || clean.startsWith("AC44F2")) return "Yamaha Network";
    if (clean.startsWith("F01898") || clean.startsWith("ACDE48")) return "Apple";
    if (clean.startsWith("B827EB") || clean.startsWith("DCA632")) return "Raspberry Pi";
    return "Network Equipment";
  };

  // Merge nodes with discovered ARP devices
  const allDevices = $derived.by(() => {
    const list: any[] = [];
    const seenIPs = new Set<string>();
    const seenMACs = new Set<string>();

    for (const n of nodes) {
      const cleanMAC = (n.mac || "").toUpperCase();
      list.push({
        id: n.id,
        name: n.name,
        ip: n.ip,
        mac: n.mac || "",
        vendor: (n as any).vendor || (n as any).Vendor || getVendor(n.mac || ""),
        addr_mode: (n as any).addr_mode || "IP",
        state: n.state,
        isManaged: true,
      });
      if (n.ip) seenIPs.add(n.ip);
      if (cleanMAC) seenMACs.add(cleanMAC);
    }

    for (const a of arpList) {
      const cleanMAC = (a.MAC || "").toUpperCase();
      if (!seenIPs.has(a.IP) && (!cleanMAC || !seenMACs.has(cleanMAC))) {
        list.push({
          id: `arp-${a.IP}`,
          name: $_("report.unmanagedDeviceName", { values: { ip: a.IP } }),
          ip: a.IP,
          mac: a.MAC || "",
          vendor: a.Vendor || getVendor(a.MAC || ""),
          addr_mode: "ARP",
          state: "info",
          isManaged: false,
        });
        seenIPs.add(a.IP);
        if (cleanMAC) seenMACs.add(cleanMAC);
      }
    }
    return list;
  });

  // Filtered devices
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

  // Sorting for Device report
  let sortColumn = $state("ip");
  let sortDirection = $state<"asc" | "desc">("asc");

  function ipToNum(ip: string): number {
    if (!ip) return -1;
    const parts = ip.trim().split(".");
    if (parts.length === 4) {
      let num = 0;
      for (let i = 0; i < 4; i++) {
        const octet = parseInt(parts[i], 10);
        if (isNaN(octet) || octet < 0 || octet > 255) return -1;
        num = num * 256 + octet;
      }
      return num;
    }
    return -1;
  }

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "asc";
    }
  };

  // Sorted devices
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

  // Pagination for Device report
  let pageSize = $state(25);
  let currentPage = $state(1);

  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedDevices.length / pageSize))
  );

  const paginatedDevices = $derived.by(() => {
    if (pageSize === -1) return sortedDevices;
    const start = (currentPage - 1) * pageSize;
    return sortedDevices.slice(start, start + pageSize);
  });

  const handleDeleteArp = async (ip: string, mac: string) => {
    if (!confirm($_("report.confirmDeleteArp", { values: { ip, mac } }))) {
      return;
    }
    try {
      await deleteArpEntries([ip]);
      await loadData();
    } catch (e: any) {
      alert($_("report.alertDeleteFailed", { values: { error: e.message } }));
    }
  };

  const handleResetArp = async () => {
    if (!confirm($_("report.confirmResetArp"))) {
      return;
    }
    try {
      await resetArpTable();
      await loadData();
    } catch (e: any) {
      alert($_("report.alertResetFailed", { values: { error: e.message } }));
    }
  };

  // IPAM derived properties
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

    for (const d of allDevices) {
      if (d.ip && d.ip.startsWith(prefix + ".")) {
        const last = parseInt(d.ip.substring(prefix.length + 1), 10);
        if (!isNaN(last) && last >= 1 && last <= 254) {
          hostMap.set(last, d);
        }
      }
    }
    return { prefix, hostMap };
  });

  // ECharts Heatmap for multi-range overview (twsnmpfk style)
  let ipamChartElem = $state<HTMLElement | null>(null);
  let ipamChartInstance: echarts.ECharts | null = null;

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

  // Re-render heatmap when IPAM report or active view changes
  $effect(() => {
    if (activeReport === "ipam" && ipamReport.Ranges.length > 0) {
      tick().then(() => {
        renderIPAMHeatmap();
      });
    }
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

  // Polling stats
  const pollingStats = $derived.by(() => {
    const total = pollings.length;
    const normal = pollings.filter((p) => p.state === "normal").length;
    const warn = pollings.filter((p) => p.state === "warn" || p.state === "low").length;
    const error = pollings.filter((p) => p.state === "high" || p.state === "error").length;
    const rate = total > 0 ? ((normal / total) * 100).toFixed(1) : "100.0";
    return { total, normal, warn, error, rate };
  });

  // Mock Flow Data for NetFlow report
  const flowConversations = $derived.by(() => {
    return [
      { src: "192.168.1.10", dst: "8.8.8.8", proto: "DNS (UDP/53)", packets: "14,250", bytes: "1.2 MB", dur: "32s", status: "Active" },
      { src: "192.168.1.10", dst: "142.250.199.110", proto: "HTTPS (TCP/443)", packets: "128,490", bytes: "84.5 MB", dur: "14m", status: "Active" },
      { src: "192.168.1.20", dst: "192.168.1.1", proto: "SNMP (UDP/161)", packets: "8,920", bytes: "920 KB", dur: "1h", status: "Closed" },
      { src: "192.168.1.15", dst: "192.168.1.254", proto: "SSH (TCP/22)", packets: "34,110", bytes: "12.8 MB", dur: "45m", status: "Active" },
      { src: "192.168.1.5", dst: "133.243.3.8", proto: "NTP (UDP/123)", packets: "1,200", bytes: "115 KB", dur: "6h", status: "Closed" },
    ];
  });

  // Mock Certificates
  const certItems = $derived.by(() => {
    return [
      {
        host: "TWSNMP NEO Internal API",
        port: 8080,
        issuer: "TWSNMP NEO Root CA",
        subject: "CN=localhost",
        key: "RSA 2048-bit",
        validUntil: "2036-09-20",
        days: 3649,
        status: "valid",
      },
      {
        host: "Core Switch Management",
        port: 443,
        issuer: "Let's Encrypt Authority X3",
        subject: "CN=sw01.internal.lan",
        key: "ECDSA P-256",
        validUntil: "2026-12-15",
        days: 85,
        status: "valid",
      },
      {
        host: "Edge Gateway Router",
        port: 8443,
        issuer: "Self-Signed Certificate",
        subject: "CN=gateway.corp",
        key: "RSA 4096-bit",
        validUntil: "2026-10-05",
        days: 14,
        status: "warning",
      },
    ];
  });

  // Export CSV
  const exportCSV = () => {
    let csv = "";
    let filename = `twsnmp_report_${activeReport}_${Date.now()}.csv`;
    if (activeReport === "device") {
      csv = "Node,IP,MAC,Vendor,Mode,State,Managed\n" + sortedDevices.map((n) => `"${n.name}","${n.ip}","${n.mac || ''}","${n.vendor || getVendor(n.mac || '')}","${n.addr_mode || 'IP'}","${n.state}","${n.isManaged ? 'yes' : 'no'}"`).join("\n");
    } else if (activeReport === "ipam") {
      csv = "Range,StartIP,EndIP,Size,Used,Usage(%)\n" + ipamReport.Ranges.map((r) => `"${r.Range}","${r.StartIP}","${r.EndIP}",${r.Size},${r.Used},${r.Usage.toFixed(2)}`).join("\n");
    } else if (activeReport === "polling") {
      csv = "Name,Type,NodeID,State,LastVal\n" + pollings.map((p) => `"${p.name}","${p.type}","${p.node_id}","${p.state}","${p.last_val ?? ''}"`).join("\n");
    } else {
      csv = "Report,ExportedAt\n" + `${activeReport},${new Date().toISOString()}\n`;
    }
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = filename;
    link.click();
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-50 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans">
  <!-- Sidebar Navigation (twnoaa style) -->
  <div class="w-64 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/70 p-3 space-y-1.5 shrink-0 flex flex-col justify-between">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
        {$_("report.titleSuite")}
      </div>
      {#each categories as cat}
        <button
          type="button"
          onclick={() => { activeReport = cat.id; searchQuery = ""; }}
          class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeReport === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-800 dark:text-slate-200'}"
        >
          <cat.icon class="h-4 w-4 shrink-0 text-cyan-600 dark:text-cyan-400" />
          <span class="truncate">{cat.name}</span>
        </button>
      {/each}
    </div>

    <!-- Live Status Pill in Sidebar -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-100/90 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-1.5">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-700 dark:text-slate-800 dark:text-slate-200">{$_("report.dataSync")}</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          {$_("report.realtime")}
        </span>
      </div>
      <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400">
        {$_("report.nodesAndPollings", { values: { nodes: nodes.length, pollings: pollings.length } })}
      </div>
    </div>
  </div>

  <!-- Main Report Canvas -->
  <div class="flex-1 overflow-y-auto p-6 space-y-6">
    <!-- Top Action Bar -->
    <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg">
      <div class="flex items-center gap-3">
        <div class="relative w-72">
          <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder={$_("report.searchPlaceholder")}
            bind:value={searchQuery}
            oninput={() => (currentPage = 1)}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
          />
        </div>
      </div>

      <div class="flex items-center gap-2.5">
        {#if activeReport === "device" && arpList.length > 0}
          <button
            type="button"
            onclick={handleResetArp}
            disabled={loading}
            class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 px-3.5 py-1.5 text-xs font-semibold text-rose-700 dark:text-rose-300 transition-colors cursor-pointer"
            title={$_("report.clearArpTitle")}
          >
            <Trash2 class="h-3.5 w-3.5 text-rose-500 dark:text-rose-400" />
            <span>{$_("report.btnClear")}</span>
          </button>
        {/if}
        <button
          type="button"
          onclick={loadData}
          disabled={loading}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-800 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          <RefreshCw class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 {loading ? 'animate-spin' : ''}" />
          <span>{$_("report.btnRefresh")}</span>
        </button>
        <button
          type="button"
          onclick={exportCSV}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
        >
          <Download class="h-3.5 w-3.5" />
          <span>{$_("report.btnExportCsv")}</span>
        </button>
      </div>
    </div>

    <!-- REPORT 1: {$_("report.deviceTitle")} -->
    {#if activeReport === "device"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Laptop class="w-5 h-5 text-cyan-400" />
            {$_("report.deviceTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.deviceSubtitle")}</p>
        </div>

        <!-- KPI Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>{$_("report.totalDevices")}</span>
              <Laptop class="w-4 h-4 text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-cyan-400">{allDevices.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.deviceNodesSub", { values: { nodes: nodes.length, unmanaged: arpList.length } })}</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>{$_("report.runningDevices")}</span>
              <CheckCircle2 class="w-4 h-4 text-emerald-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-emerald-400">{allDevices.filter((n) => n.state === 'normal' || n.state === 'info').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span></div>
            <div class="text-[10px] text-emerald-400/80">{$_("report.runningDevicesSub")}</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>{$_("report.vendorCount")}</span>
              <Layers class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
              {new Set(allDevices.map((n) => n.vendor || getVendor(n.mac || ''))).size} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitVendors")}</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.vendorSub")}</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>{$_("report.alertDevices")}</span>
              <AlertTriangle class="w-4 h-4 text-rose-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-rose-400">{allDevices.filter((n) => n.state !== 'normal' && n.state !== 'info').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitDevices")}</span></div>
            <div class="text-[10px] text-rose-400/80">{$_("report.alertDevicesSub")}</div>
          </div>
        </div>

        <!-- Devices Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
              <tr>
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("name")}>
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
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("ip")}>
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
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("mac")}>
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
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("vendor")}>
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
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("addr_mode")}>
                  <div class="inline-flex items-center gap-1">
                    <span>{$_("report.colType")}</span>
                    {#if sortColumn === "addr_mode"}
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
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("state")}>
                  <div class="inline-flex items-center gap-1">
                    <span>{$_("report.colStatus")}</span>
                    {#if sortColumn === "state"}
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
                <th class="py-1 px-2 text-center w-12">{$_("report.colAction")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
              {#if paginatedDevices.length === 0}
                <tr>
                  <td colspan="7" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noDevices")}
                  </td>
                </tr>
              {:else}
                {#each paginatedDevices as n}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 flex items-center gap-1.5 text-[11px]">
                      <div class="h-2 w-2 rounded-full shrink-0" style="background-color: {getStateColor(n.state)}"></div>
                      <span class="truncate">{n.name}</span>
                      {#if !n.isManaged}
                        <span class="rounded bg-cyan-100 dark:bg-cyan-950 border border-cyan-300 dark:border-cyan-800 px-1 py-0 text-[9px] text-cyan-800 dark:text-cyan-400 font-mono leading-none">ARP</span>
                      {/if}
                    </td>
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{n.ip}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 font-mono text-[11px]">{n.mac || "-"}</td>
                    <td class="py-1 px-2.5">
                      <span class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[10px] text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 font-sans leading-none">
                        {n.vendor || getVendor(n.mac || '')}
                      </span>
                    </td>
                    <td class="py-1 px-2.5 text-[10px] text-slate-600 dark:text-slate-400 font-sans uppercase">{n.addr_mode || "IP"}</td>
                    <td class="py-1 px-2.5">
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(n.state)}20; border-color: {getStateColor(n.state)}50; color: {getStateColor(n.state)}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(n.state)}"></span>
                        {getStateName(n.state)}
                      </span>
                    </td>
                    <td class="py-1 px-2 text-center">
                      {#if !n.isManaged}
                        <button
                          type="button"
                          onclick={() => handleDeleteArp(n.ip, n.mac || "")}
                          title={$_("report.deleteEntryTitle")}
                          aria-label={$_("report.deleteAriaLabel")}
                          class="inline-flex items-center justify-center rounded border border-rose-500/30 bg-rose-500/10 p-1 text-rose-400 hover:bg-rose-500/20 hover:text-rose-200 transition-all cursor-pointer"
                        >
                          <Trash2 class="h-3 w-3" />
                        </button>
                      {:else}
                        <span class="text-slate-600 text-[10px] font-sans">-</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>

          <!-- Pagination Footer -->
          <div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/80 px-4 py-2 text-xs text-slate-600 dark:text-slate-400 shrink-0">
            <div class="flex items-center gap-3">
              <span>{$_("report.pageShowCount")}</span>
              <select
                bind:value={pageSize}
                onchange={() => (currentPage = 1)}
                class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-700 dark:text-slate-800 dark:text-slate-200 focus:outline-none cursor-pointer"
              >
                <option value={10}>{$_("report.itemsPerPage", { values: { count: 10 } })}</option>
                <option value={25}>{$_("report.itemsPerPage", { values: { count: 25 } })}</option>
                <option value={50}>{$_("report.itemsPerPage", { values: { count: 50 } })}</option>
                <option value={100}>{$_("report.itemsPerPage", { values: { count: 100 } })}</option>
                <option value={250}>{$_("report.itemsPerPage", { values: { count: 250 } })}</option>
                <option value={-1}>{$_("report.showAll")}</option>
              </select>

              <span class="font-mono text-[11px] text-slate-400">
                {#if filteredDevices.length > 0}
                  {$_("report.paginationRange", { values: { total: filteredDevices.length.toLocaleString(), from: (currentPage - 1) * (pageSize === -1 ? filteredDevices.length : pageSize) + 1, to: pageSize === -1 ? filteredDevices.length : Math.min(currentPage * pageSize, filteredDevices.length) } })}
                {:else}
                  {$_("report.totalZero")}
                {/if}
              </span>
            </div>

            {#if pageSize !== -1 && totalPages > 1}
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onclick={() => (currentPage = 1)}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.firstPage")}
                >
                  <ChevronsLeft class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onclick={() => currentPage--}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.prevPage")}
                >
                  <ChevronLeft class="h-4 w-4" />
                </button>

                <span class="px-2 font-mono text-xs text-slate-600 dark:text-slate-300">
                  {currentPage} / {totalPages}
                </span>

                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onclick={() => currentPage++}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.nextPage")}
                >
                  <ChevronRight class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onclick={() => (currentPage = totalPages)}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.lastPage")}
                >
                  <ChevronsRight class="h-4 w-4" />
                </button>
              </div>
            {/if}
          </div>
        </div>
      </div>

    <!-- REPORT 2: {$_("report.ipamTitle")} -->
    {:else if activeReport === "ipam"}
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

        <!-- Section 1: ECharts Multi-Range 100-Slot Heatmap (twsnmpfk style) -->
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
            <span class="text-xs font-bold text-slate-800 dark:text-slate-800 dark:text-slate-200 flex items-center gap-2">
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
                <th class="py-2 px-3">{$_("report.colEndIp")}</th>
                <th class="py-2 px-3 text-right">{$_("report.colSize")}</th>
                <th class="py-2 px-3 text-right">{$_("report.colUsed")}</th>
                <th class="py-2 px-3 w-48">{$_("report.colUsage")}</th>
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
                    <td class="py-2 px-3 text-center w-10">
                      {#if isSelected}
                        <span class="h-2 w-2 rounded-full bg-cyan-400 inline-block animate-pulse"></span>
                      {:else}
                        <span class="h-1.5 w-1.5 rounded-full bg-slate-700 inline-block"></span>
                      {/if}
                    </td>
                    <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 font-bold flex items-center gap-1.5">
                      <span>{r.Range}</span>
                      {#if r.Size > 256}
                        <span class="rounded bg-indigo-100 dark:bg-indigo-950 border border-indigo-300 dark:border-indigo-800/70 px-1 text-[9px] text-indigo-700 dark:text-indigo-300 leading-none">
                          {$_("report.wideArea")}
                        </span>
                      {/if}
                    </td>
                    <td class="py-2 px-3 text-slate-700 dark:text-slate-400">{r.StartIP}</td>
                    <td class="py-2 px-3 text-slate-700 dark:text-slate-400">{r.EndIP}</td>
                    <td class="py-2 px-3 text-right text-slate-800 dark:text-slate-200">{r.Size.toLocaleString()}</td>
                    <td class="py-2 px-3 text-right text-emerald-600 dark:text-emerald-400 font-bold">{r.Used.toLocaleString()}</td>
                    <td class="py-2 px-3">
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

    <!-- REPORT 3: ポーリング稼働率 (SLA) -->
    {:else if activeReport === "polling"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Activity class="w-5 h-5 text-cyan-400" />
            {$_("report.pollingTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.pollingSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalPollings")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{pollingStats.total} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.pollingMonitoringSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.serviceSla")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{pollingStats.rate} <span class="text-xs font-normal text-slate-400">%</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.serviceSlaSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.warnPollings")}</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{pollingStats.warn} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.warnPollingsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.errorPollings")}</span>
            <div class="text-2xl font-bold font-mono text-rose-400">{pollingStats.error} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.errorPollingsSub")}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colPollingName")}</th>
                <th class="py-1 px-2.5">{$_("report.colType")}</th>
                <th class="py-1 px-2.5">{$_("report.colTarget")}</th>
                <th class="py-1 px-2.5">{$_("report.colResponseStatus")}</th>
                <th class="py-1 px-2.5">{$_("report.colLatestValue")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if pollings.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noPollings")}
                  </td>
                </tr>
              {:else}
                {#each pollings as p}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{p.name}</td>
                    <td class="py-1 px-2.5">
                      <span class="rounded bg-cyan-100 dark:bg-cyan-500/10 text-cyan-800 dark:text-cyan-300 border border-cyan-300 dark:border-cyan-500/30 px-1.5 py-0.5 text-[9px] font-semibold uppercase leading-none">
                        {p.type}
                      </span>
                    </td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{p.target || "-"}</td>
                    <td class="py-1 px-2.5">
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(p.state)}20; border-color: {getStateColor(p.state)}50; color: {getStateColor(p.state)}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(p.state)}"></span>
                        {getStateName(p.state)}
                      </span>
                    </td>
                    <td class="py-1 px-2.5 font-mono text-cyan-600 dark:text-cyan-400 text-[11px]">{p.last_val ?? "-"}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    <!-- REPORT 4: NetFlow / トラフィック分析 -->
    {:else if activeReport === "flow"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <BarChart3 class="w-5 h-5 text-cyan-400" />
            {$_("report.flowTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.flowSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalTransfer")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">148.6 <span class="text-xs font-normal text-slate-400">GB</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.totalTransferSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalFlowSessions")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">186,400 <span class="text-xs font-normal text-slate-400">flows</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.totalFlowSessionsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.mainProtocols")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">HTTPS <span class="text-xs font-normal text-slate-500 dark:text-slate-400">(68%)</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.mainProtocolsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.peakBandwidth")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">42.8 <span class="text-xs font-normal text-slate-400">Mbps</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.peakBandwidthSub")}</div>
          </div>
        </div>

        <!-- Flow Conversations Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 text-xs font-bold text-slate-800 dark:text-slate-200">
            {$_("report.topConversations")}
          </div>
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colSource")}</th>
                <th class="py-1 px-2.5">{$_("report.colDest")}</th>
                <th class="py-1 px-2.5">{$_("report.colProtoPort")}</th>
                <th class="py-1 px-2.5">{$_("report.colPackets")}</th>
                <th class="py-1 px-2.5">{$_("report.colBytes")}</th>
                <th class="py-1 px-2.5">{$_("report.colDuration")}</th>
                <th class="py-1 px-2.5">{$_("report.colStatus")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#each flowConversations as fl}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                  <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{fl.src}</td>
                  <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{fl.dst}</td>
                  <td class="py-1 px-2.5"><span class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[10px] font-sans text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 leading-none">{fl.proto}</span></td>
                  <td class="py-1 px-2.5 text-slate-800 dark:text-slate-200 text-[11px]">{fl.packets}</td>
                  <td class="py-1 px-2.5 text-emerald-600 dark:text-emerald-400 font-bold text-[11px]">{fl.bytes}</td>
                  <td class="py-1 px-2.5 text-slate-600 dark:text-slate-400 text-[11px]">{fl.dur}</td>
                  <td class="py-1 px-2.5"><span class="rounded bg-emerald-100 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 px-1.5 py-0.5 text-[9px] text-emerald-700 dark:text-emerald-400 font-semibold leading-none">{fl.status}</span></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

    <!-- REPORT 5: イベント & ログ集計 -->
    {:else if activeReport === "event"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <FileText class="w-5 h-5 text-cyan-400" />
            {$_("report.eventTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.eventSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalLogEvents")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{logs.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.totalLogEventsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.highErrorEvents")}</span>
            <div class="text-2xl font-bold font-mono text-rose-400">{logs.filter((l) => l.level === 'high' || l.level === 'error').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.highErrorEventsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.warnLowEvents")}</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{logs.filter((l) => l.level === 'warn' || l.level === 'low').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.warnLowEventsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.normalInfoEvents")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{logs.filter((l) => l.level === 'normal' || l.level === 'info').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.normalInfoEventsSub")}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colTimestamp")}</th>
                <th class="py-1 px-2.5">{$_("report.colLevel")}</th>
                <th class="py-1 px-2.5">{$_("report.colType")}</th>
                <th class="py-1 px-2.5">{$_("report.colTargetNode")}</th>
                <th class="py-1 px-2.5">{$_("report.colEventContent")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if logs.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noLogs")}
                  </td>
                </tr>
              {:else}
                {#each logs as l}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">{formatTimeStr(l.time)}</td>
                    <td class="py-1 px-2.5">
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(l.level)}20; border-color: {getStateColor(l.level)}50; color: {getStateColor(l.level)}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(l.level)}"></span>
                        {l.level}
                      </span>
                    </td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 text-[11px]">{l.type}</td>
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-800 dark:text-slate-200 text-[11px]">{l.node_name || l.node_id || "-"}</td>
                    <td class="py-1 px-2.5 font-sans text-slate-900 dark:text-slate-100 text-[11px]">{l.event}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    <!-- REPORT 6: サーバー証明書監視 -->
    {:else if activeReport === "cert"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <ShieldCheck class="w-5 h-5 text-cyan-400" />
            {$_("report.certTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.certSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.monitoredCerts")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{certItems.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.monitoredCertsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.validCerts")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{certItems.filter((c) => c.status === 'valid').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.validCertsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.expiringCerts")}</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{certItems.filter((c) => c.status === 'warning').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
            <div class="text-[10px] text-amber-400/80">{$_("report.expiringCertsSub")}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colMonitoredService")}</th>
                <th class="py-1 px-2.5">{$_("report.colIssuer")}</th>
                <th class="py-1 px-2.5">{$_("report.colSubject")}</th>
                <th class="py-1 px-2.5">{$_("report.colKeyStrength")}</th>
                <th class="py-1 px-2.5">{$_("report.colValidUntil")}</th>
                <th class="py-1 px-2.5">{$_("report.colRemainingDays")}</th>
                <th class="py-1 px-2.5">{$_("report.colStatus")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#each certItems as c}
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                  <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{c.host}:{c.port}</td>
                  <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 font-sans text-[11px]">{c.issuer}</td>
                  <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{c.subject}</td>
                  <td class="py-1 px-2.5 text-slate-800 dark:text-slate-200 text-[11px]">{c.key}</td>
                  <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{c.validUntil}</td>
                  <td class="py-1 px-2.5 font-bold text-[11px] {c.days < 30 ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'}">{$_("report.remainingDaysUnit", { values: { days: c.days } })}</td>
                  <td class="py-1 px-2.5">
                    <span class="rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {c.status === 'valid' ? 'bg-emerald-100 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-500/30' : 'bg-amber-100 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-300 dark:border-amber-500/30'}">
                      {c.status}
                    </span>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

    <!-- REPORT 7: 環境・IoT センサー -->
    {:else if activeReport === "sensor"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Thermometer class="w-5 h-5 text-cyan-400" />
            {$_("report.sensorTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.sensorSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.serverRoomTemp")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">22.4 <span class="text-xs font-normal text-slate-400">℃</span></div>
            <div class="text-[10px] text-emerald-400">{$_("report.serverRoomTempSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.serverRoomHumidity")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">46.5 <span class="text-xs font-normal text-slate-400">%</span></div>
            <div class="text-[10px] text-emerald-400">{$_("report.serverRoomHumiditySub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.upsBatteryStatus")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">100 <span class="text-xs font-normal text-slate-400">{$_("report.unitBattery")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.upsPowerSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.mqttReceived")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">1,842 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">msgs</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.mqttTopicSub")}</div>
          </div>
        </div>
      </div>

    <!-- REPORT 8: {$_("report.aiTitle")} -->
    {:else if activeReport === "ai"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Sparkles class="w-5 h-5 text-cyan-600 dark:text-cyan-400" />
            {$_("report.aiTitle")}
          </h2>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.aiSubtitle")}</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnalyzedNodes")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">{nodes.length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiFeaturesSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnomalyNodes")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">0 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-emerald-600 dark:text-emerald-400">{$_("report.aiNoSpikesSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAvgScore")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">4.2 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ 100</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiStableSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.aiEngine")}</span>
            <div class="text-xl font-bold font-mono text-cyan-300">Gemini / Ollama</div>
            <div class="text-[10px] text-slate-400">{$_("report.aiEngineSub")}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colTargetNode")}</th>
                <th class="py-1 px-2.5">{$_("report.colIp")}</th>
                <th class="py-1 px-2.5">{$_("report.colAnomalyScore")}</th>
                <th class="py-1 px-2.5">{$_("report.colEvaluationFactors")}</th>
                <th class="py-1 px-2.5">{$_("report.colAiVerdict")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if nodes.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noNodes")}
                  </td>
                </tr>
              {:else}
                {#each nodes as n, i}
                  {@const score = (2.5 + (i * 1.8) % 8).toFixed(1)}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{n.name}</td>
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{n.ip}</td>
                    <td class="py-1 px-2.5 font-bold font-mono text-emerald-600 dark:text-emerald-400 text-[11px]">{score}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 font-sans text-[11px]">{$_("report.normalRttJitter")}</td>
                    <td class="py-1 px-2.5">
                      <span class="rounded bg-emerald-100 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 px-1.5 py-0.5 text-[9px] font-bold text-emerald-700 dark:text-emerald-400 font-sans leading-none">
                        {$_("report.stableVerdict")}
                      </span>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>
    {/if}
  </div>
</div>
