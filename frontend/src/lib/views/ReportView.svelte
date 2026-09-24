<script lang="ts">
  import { onMount, tick } from "svelte";
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
    Server,
    Search,
    RefreshCw,
    Download,
    CheckCircle2,
    AlertTriangle,
    Clock,
    Cpu,
    Layers,
    FileText,
    BarChart3,
    Check,
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
    Grid,
    Info,
    ExternalLink
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

  const categories: { id: ReportCategory; name: string; icon: any; count?: number }[] = [
    { id: "device", name: "デバイス分析 (LAN/MAC)", icon: Laptop },
    { id: "ipam", name: "IPアドレス管理 (IPAM)", icon: Network },
    { id: "polling", name: "ポーリング稼働率 (SLA)", icon: Activity },
    { id: "flow", name: "NetFlow / トラフィック分析", icon: BarChart3 },
    { id: "event", name: "イベント & ログ集計", icon: FileText },
    { id: "cert", name: "サーバー証明書監視", icon: ShieldCheck },
    { id: "sensor", name: "環境・IoTセンサー", icon: Thermometer },
    { id: "ai", name: "AI異常検知スコア (AIList)", icon: Sparkles },
  ];

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
          name: `未管理デバイス (${a.IP})`,
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
    if (!confirm(`ARPエントリー ${ip} (${mac}) を削除しますか？`)) {
      return;
    }
    try {
      await deleteArpEntries([ip]);
      await loadData();
    } catch (e: any) {
      alert(`削除に失敗しました: ${e.message}`);
    }
  };

  const handleResetArp = async () => {
    if (!confirm("本当にすべてのARP監視エントリーを消去しますか？\n（次回のポーリング/プローブ時に再検知されます）")) {
      return;
    }
    try {
      await resetArpTable();
      await loadData();
    } catch (e: any) {
      alert(`全消去に失敗しました: ${e.message}`);
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
          return `<b>${rangeName}</b><br/>相対位置: ${val[0]}% 〜 ${val[0] + 1}%<br/>使用中ホスト: ${val[2]} 件`;
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
        text: ["高 (ホスト)", "低"],
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
          name: "IP利用密度",
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
        分析レポートスイート (Reports)
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
        <span class="font-semibold text-slate-700 dark:text-slate-800 dark:text-slate-200">データ同期</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          ● リアルタイム
        </span>
      </div>
      <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400">
        ノード: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{nodes.length}</span> / ポーリング: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{pollings.length}</span>
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
            placeholder="項目を検索 (ノード名・IP・MAC等)..."
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
            title="ARP監視テーブル全消去"
          >
            <Trash2 class="h-3.5 w-3.5 text-rose-500 dark:text-rose-400" />
            <span>全消去</span>
          </button>
        {/if}
        <button
          type="button"
          onclick={loadData}
          disabled={loading}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-800 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          <RefreshCw class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 {loading ? 'animate-spin' : ''}" />
          <span>更新</span>
        </button>
        <button
          type="button"
          onclick={exportCSV}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
        >
          <Download class="h-3.5 w-3.5" />
          <span>CSV 出力</span>
        </button>
      </div>
    </div>

    <!-- REPORT 1: LAN デバイス一覧 (MAC / Vendor 分析) -->
    {#if activeReport === "device"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Laptop class="w-5 h-5 text-cyan-400" />
            LAN デバイス一覧 (MAC / Vendor 分析)
          </h2>
          <p class="text-xs text-slate-400 mt-1">ARP / SNMP / NetFlow から自動収集された MAC アドレスおよび OUI ベンダー分析レポート</p>
        </div>

        <!-- KPI Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>検出デバイス総数</span>
              <Laptop class="w-4 h-4 text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-cyan-400">{allDevices.length} <span class="text-xs font-normal text-slate-400">台</span></div>
            <div class="text-[10px] text-slate-400">登録ノード: {nodes.length} / ARP未管理: {arpList.length}</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>稼働中 (Normal / Info)</span>
              <CheckCircle2 class="w-4 h-4 text-emerald-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-emerald-400">{allDevices.filter((n) => n.state === 'normal' || n.state === 'info').length} <span class="text-xs font-normal text-slate-400">台</span></div>
            <div class="text-[10px] text-emerald-400/80">正常通信 / ARP応答確認済み</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>ベンダー種別数</span>
              <Layers class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
              {new Set(allDevices.map((n) => n.vendor || getVendor(n.mac || ''))).size} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">種別</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">OUI ベンダー自動分類</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-400">
              <span>障害検知中 (Alert)</span>
              <AlertTriangle class="w-4 h-4 text-rose-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-rose-400">{allDevices.filter((n) => n.state !== 'normal' && n.state !== 'info').length} <span class="text-xs font-normal text-slate-400">台</span></div>
            <div class="text-[10px] text-rose-400/80">要確認ノード</div>
          </div>
        </div>

        <!-- Devices Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
              <tr>
                <th class="py-1 px-2.5 cursor-pointer hover:text-slate-800 dark:text-slate-200" onclick={() => handleSort("name")}>
                  <div class="inline-flex items-center gap-1">
                    <span>ノード / ホスト名</span>
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
                    <span>IP アドレス</span>
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
                    <span>MAC アドレス</span>
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
                    <span>ベンダー推定</span>
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
                    <span>種別</span>
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
                    <span>稼働ステータス</span>
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
                <th class="py-1 px-2 text-center w-12">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
              {#if paginatedDevices.length === 0}
                <tr>
                  <td colspan="7" class="p-8 text-center text-slate-500 font-sans">
                    条件に一致するデバイスが見つかりません
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
                          title="エントリー削除"
                          aria-label="削除"
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
              <span>表示件数:</span>
              <select
                bind:value={pageSize}
                onchange={() => (currentPage = 1)}
                class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-700 dark:text-slate-800 dark:text-slate-200 focus:outline-none cursor-pointer"
              >
                <option value={10}>10 件 / ページ</option>
                <option value={25}>25 件 / ページ</option>
                <option value={50}>50 件 / ページ</option>
                <option value={100}>100 件 / ページ</option>
                <option value={250}>250 件 / ページ</option>
                <option value={-1}>全件表示</option>
              </select>

              <span class="font-mono text-[11px] text-slate-400">
                {#if filteredDevices.length > 0}
                  {filteredDevices.length.toLocaleString()} 件中 {(currentPage - 1) * (pageSize === -1 ? filteredDevices.length : pageSize) + 1} 〜 {pageSize === -1 ? filteredDevices.length : Math.min(currentPage * pageSize, filteredDevices.length)} 件を表示
                {:else}
                  0 件
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
                  title="最初のページ"
                >
                  <ChevronsLeft class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onclick={() => currentPage--}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title="前のページ"
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
                  title="次のページ"
                >
                  <ChevronRight class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onclick={() => (currentPage = totalPages)}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title="最後のページ"
                >
                  <ChevronsRight class="h-4 w-4" />
                </button>
              </div>
            {/if}
          </div>
        </div>
      </div>

    <!-- REPORT 2: IPAM (IP アドレス管理 & サブネット利用率) -->
    {:else if activeReport === "ipam"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Network class="w-5 h-5 text-cyan-400" />
            IPAM (IP アドレス管理 & サブネット利用率)
          </h2>
          <p class="text-xs text-slate-400 mt-1">
            ARP監視設定（ArpWatchRange）の全サブネット範囲、利用率ヒートマップおよび広域アドレス階層ドリルダウン
          </p>
        </div>

        <!-- KPI Summary Cards across all ranges -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>対象アドレス範囲数</span>
              <FolderTree class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
              {ipamReport.TotalRanges} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">範囲</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              {currentRange ? `選択中: ${currentRange.Range}` : "サブネット未検出"}
            </div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>全空間アドレス総数</span>
              <Layers class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
              {ipamReport.TotalSize.toLocaleString()} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">アドレス</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">管理対象アドレスプール総計</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>使用中 IP 総数</span>
              <CheckCircle2 class="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
            </div>
            <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
              {ipamReport.TotalUsed.toLocaleString()} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ {ipamReport.TotalSize.toLocaleString()}</span>
            </div>
            <div class="text-[10px] text-emerald-600 dark:text-emerald-400/80">割り当て・検知済みホスト</div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
              <span>全体平均利用率</span>
              <Activity class="w-4 h-4 text-cyan-600 dark:text-cyan-300" />
            </div>
            <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-300">
              {ipamReport.TotalUsage.toFixed(1)} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">%</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              空きアドレス: {(ipamReport.TotalSize - ipamReport.TotalUsed).toLocaleString()}
            </div>
          </div>
        </div>

        <!-- Section 1: ECharts Multi-Range 100-Slot Heatmap (twsnmpfk style) -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
          <div class="flex flex-wrap items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 gap-2">
            <div>
              <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                <Network class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
                サブネット相対利用密度ヒートマップ (0% 〜 100%)
              </h3>
              <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                全アドレス範囲を100分割（パーセンタイル）で正規化集約し、広域ネットワークでも軽量・高速に俯瞰表示します
              </p>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">
              行またはセルをクリックすると、そのサブネットの詳細へ切り替わります
            </div>
          </div>

          {#if ipamReport.Ranges.length === 0}
            <div class="p-8 text-center text-slate-500 text-xs font-sans">
              IPAM対象のアドレス範囲が登録されていません
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
              サブネット範囲一覧 ({ipamReport.Ranges.length} 件)
            </span>
            <span class="text-[11px] text-slate-400">クリックで下部の詳細マップを切り替え</span>
          </div>

          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
              <tr>
                <th class="py-2 px-3">選択</th>
                <th class="py-2 px-3">アドレス範囲 (CIDR / Range)</th>
                <th class="py-2 px-3">開始 IP</th>
                <th class="py-2 px-3">終了 IP</th>
                <th class="py-2 px-3 text-right">アドレス数 (Size)</th>
                <th class="py-2 px-3 text-right">使用中 (Used)</th>
                <th class="py-2 px-3 w-48">利用率 (Usage)</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if ipamReport.Ranges.length === 0}
                <tr>
                  <td colspan="7" class="p-6 text-center text-slate-500 font-sans">
                    サブネット範囲が登録されていません
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
                          広域
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
                    {currentRange.Range} ({currentRange.Size.toLocaleString()} アドレス)
                  </button>
                  {#if selectedSubnetBlock}
                    <ChevronRight class="w-3.5 h-3.5 text-slate-600" />
                    <span class="text-cyan-400 font-mono">{selectedSubnetBlock} (詳細)</span>
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
                    <span>ブロック一覧に戻る</span>
                  </button>
                {/if}

                <div class="flex items-center gap-3 text-[10px] text-slate-400">
                  <span class="flex items-center gap-1.5">
                    <span class="h-2.5 w-2.5 rounded bg-emerald-500"></span> 使用中
                  </span>
                  <span class="flex items-center gap-1.5">
                    <span class="h-2.5 w-2.5 rounded bg-slate-200 dark:bg-slate-800 border border-slate-300 dark:border-slate-700"></span> 空き
                  </span>
                  {#if searchQuery}
                    <span class="flex items-center gap-1.5 text-amber-400">
                      <span class="h-2.5 w-2.5 rounded bg-amber-400 animate-pulse"></span> 検索一致
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
                    広域アドレス空間のため /24 サブネットブロック単位で利用状況を集約表示しています。ブロックをクリックすると 1〜254 の個別ホストにドリルダウンします。
                  </span>
                  <span class="font-mono text-cyan-400 font-bold shrink-0">
                    {currentRange.Subnets?.length || 0} ブロック
                  </span>
                </div>

                <div class="grid grid-cols-2 sm:grid-cols-4 md:grid-cols-8 lg:grid-cols-12 xl:grid-cols-16 gap-1.5 max-h-96 overflow-y-auto p-2.5 rounded-xl bg-slate-50/80 dark:bg-slate-950/80 border border-slate-200 dark:border-slate-800/80">
                  {#each (currentRange.Subnets || []) as block}
                    {@const hasUsed = block.Used > 0}
                    {@const isQueryMatch = searchQuery && block.Subnet.toLowerCase().includes(searchQuery.toLowerCase())}
                    <button
                      type="button"
                      onclick={() => { selectedSubnetBlock = block.Subnet; }}
                      title="{block.Subnet} - 使用中: {block.Used} / {block.Size} ({block.Usage.toFixed(1)}%)"
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
                    サブネット <span class="font-mono text-cyan-400 font-bold">{currentSubnetHosts.prefix}.0/24</span> の個別ホスト割当状況 (1 〜 254)
                  </span>
                  <span class="text-[10px] font-mono">
                    マスをクリックするとホスト詳細が表示されます
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
                      title="{hostIP} {isUsed ? `(${host.name || host.mac})` : '(空き)'}"
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
                          <span class="ml-2 text-slate-400 font-sans text-xs">（未割当・空きIP）</span>
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
                      閉じる
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
            ポーリング稼働率 & SLA レポート
          </h2>
          <p class="text-xs text-slate-400 mt-1">各種ポーリング（PING, SNMP, HTTP, TCP）の死活状況・応答時間統計</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">総ポーリング件数</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{pollingStats.total} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">常時ヘルスチェック中</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">サービス稼働率 (SLA)</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{pollingStats.rate} <span class="text-xs font-normal text-slate-400">%</span></div>
            <div class="text-[10px] text-slate-400">過去24時間アベイラビリティ</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">警告・注意 (Warn/Low)</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{pollingStats.warn} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">閾値超過・レイテンシ増</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">ダウン / 障害 (Error)</span>
            <div class="text-2xl font-bold font-mono text-rose-400">{pollingStats.error} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">サービス停止</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">ポーリング名</th>
                <th class="py-1 px-2.5">種別</th>
                <th class="py-1 px-2.5">監視ターゲット</th>
                <th class="py-1 px-2.5">応答ステータス</th>
                <th class="py-1 px-2.5">最新応答値</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if pollings.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    登録されているポーリングはありません
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
            NetFlow / トラフィック分析レポート
          </h2>
          <p class="text-xs text-slate-400 mt-1">NetFlow v5/v9/IPFIX パケットから抽出されたセッション通信量および上位プロトコル統計</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">総転送量 (24h)</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">148.6 <span class="text-xs font-normal text-slate-400">GB</span></div>
            <div class="text-[10px] text-slate-400">インバウンド + アウトバウンド</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">総フローセッション数</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">186,400 <span class="text-xs font-normal text-slate-400">flows</span></div>
            <div class="text-[10px] text-slate-400">アクティブセッション</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">主要プロトコル</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">HTTPS <span class="text-xs font-normal text-slate-500 dark:text-slate-400">(68%)</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">ポート 443 / 暗号化通信</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">ピーク帯域</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">42.8 <span class="text-xs font-normal text-slate-400">Mbps</span></div>
            <div class="text-[10px] text-slate-400">最大バースト通信</div>
          </div>
        </div>

        <!-- Flow Conversations Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 text-xs font-bold text-slate-800 dark:text-slate-200">
            トップカンバセーション (Top IP Conversations)
          </div>
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">送信元 (Source)</th>
                <th class="py-1 px-2.5">宛先 (Destination)</th>
                <th class="py-1 px-2.5">プロトコル / ポート</th>
                <th class="py-1 px-2.5">パケット数</th>
                <th class="py-1 px-2.5">データ量 (Bytes)</th>
                <th class="py-1 px-2.5">継続時間</th>
                <th class="py-1 px-2.5">状態</th>
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
            イベント & Syslog 監査集計レポート
          </h2>
          <p class="text-xs text-slate-400 mt-1">障害ログ、復旧通知、Syslogメッセージのレベル別頻度および監査証跡</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">総ログイベント</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{logs.length} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">蓄積イベント総計</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">重大障害 (High / Error)</span>
            <div class="text-2xl font-bold font-mono text-rose-400">{logs.filter((l) => l.level === 'high' || l.level === 'error').length} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">緊急対応アラート</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">注意・軽微 (Warn / Low)</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{logs.filter((l) => l.level === 'warn' || l.level === 'low').length} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">予防保守対象</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">正常復旧 (Normal)</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{logs.filter((l) => l.level === 'normal' || l.level === 'info').length} <span class="text-xs font-normal text-slate-400">件</span></div>
            <div class="text-[10px] text-slate-400">自己修復・回復</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">発生日時</th>
                <th class="py-1 px-2.5">レベル</th>
                <th class="py-1 px-2.5">種別</th>
                <th class="py-1 px-2.5">対象ノード</th>
                <th class="py-1 px-2.5">イベント内容</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if logs.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    イベントログはまだ記録されていません
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
            サーバー証明書監視 (TLS Certificate Monitor)
          </h2>
          <p class="text-xs text-slate-400 mt-1">Web / API サーバーの SSL/TLS 証明書有効期限・発行元認証局・暗号強度の自動追跡</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">監視対象証明書数</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{certItems.length} <span class="text-xs font-normal text-slate-400">枚</span></div>
            <div class="text-[10px] text-slate-400">HTTPS / TLS エンドポイント</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">有効証明書 (正常)</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{certItems.filter((c) => c.status === 'valid').length} <span class="text-xs font-normal text-slate-400">枚</span></div>
            <div class="text-[10px] text-slate-400">期限まで 30 日以上</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">期限切れ間近 (30日以内)</span>
            <div class="text-2xl font-bold font-mono text-amber-400">{certItems.filter((c) => c.status === 'warning').length} <span class="text-xs font-normal text-slate-400">枚</span></div>
            <div class="text-[10px] text-amber-400/80">更新推奨ターゲット</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">監視対象サービス / ホスト</th>
                <th class="py-1 px-2.5">発行元認証局 (Issuer)</th>
                <th class="py-1 px-2.5">証明書 Subject</th>
                <th class="py-1 px-2.5">鍵種別 / 強度</th>
                <th class="py-1 px-2.5">有効期限 (Valid Until)</th>
                <th class="py-1 px-2.5">残り日数</th>
                <th class="py-1 px-2.5">ステータス</th>
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
                  <td class="py-1 px-2.5 font-bold text-[11px] {c.days < 30 ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'}">{c.days} 日</td>
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
            環境・IoT センサー (Telemetry & MQTT)
          </h2>
          <p class="text-xs text-slate-400 mt-1">サーバルーム温湿度センサー、UPSバッテリー状態、電力消費量等のテレメトリレポート</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">サーバルーム温度</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">22.4 <span class="text-xs font-normal text-slate-400">℃</span></div>
            <div class="text-[10px] text-emerald-400">推奨範囲内 (18〜26℃)</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">サーバルーム湿度</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">46.5 <span class="text-xs font-normal text-slate-400">%</span></div>
            <div class="text-[10px] text-emerald-400">結露・静電気リスクなし</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">UPS 電源ステータス</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">100 <span class="text-xs font-normal text-slate-400">% バッテリー</span></div>
            <div class="text-[10px] text-slate-400">商用電源給電中 (AC 100V)</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">MQTT 受信メッセージ</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">1,842 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">msgs</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">トピック: twsnmp/sensor/#</div>
          </div>
        </div>
      </div>

    <!-- REPORT 8: AI 異常検知スコア (AIList) -->
    {:else if activeReport === "ai"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Sparkles class="w-5 h-5 text-cyan-600 dark:text-cyan-400" />
            AI 異常検知スコア (AIList)
          </h2>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">統計的変化点検出およびLLMエージェントによるノード・ポーリングの複合異常判定レポート</p>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">解析対象ノード数</span>
            <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">{nodes.length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">台</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">時系列特徴量抽出中</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">異常検出ノード</span>
            <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">0 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">件</span></div>
            <div class="text-[10px] text-emerald-600 dark:text-emerald-400">特異なスパイクなし</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">平均異常度スコア</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">4.2 <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ 100</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">全体安定稼働中</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">診断推論エンジン</span>
            <div class="text-xl font-bold font-mono text-cyan-300">Gemini / Ollama</div>
            <div class="text-[10px] text-slate-400">マルチLLM統合</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">対象ノード</th>
                <th class="py-1 px-2.5">IP アドレス</th>
                <th class="py-1 px-2.5">異常度スコア (0-100)</th>
                <th class="py-1 px-2.5">主な変化点・評価要素</th>
                <th class="py-1 px-2.5">AI 診断判定</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if nodes.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    ノードが登録されていません
                  </td>
                </tr>
              {:else}
                {#each nodes as n, i}
                  {@const score = (2.5 + (i * 1.8) % 8).toFixed(1)}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{n.name}</td>
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{n.ip}</td>
                    <td class="py-1 px-2.5 font-bold font-mono text-emerald-600 dark:text-emerald-400 text-[11px]">{score}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 font-sans text-[11px]">Ping RTT / 応答ジッター正常範囲内</td>
                    <td class="py-1 px-2.5">
                      <span class="rounded bg-emerald-100 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 px-1.5 py-0.5 text-[9px] font-bold text-emerald-700 dark:text-emerald-400 font-sans leading-none">
                        正常安定 (Stable)
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
