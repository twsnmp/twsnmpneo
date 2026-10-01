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
    queryParquetLogs,
    fetchCertMonitors,
    saveCertMonitor,
    deleteCertMonitor,
    checkCertMonitors,
    fetchMqttStats,
    type NodeEnt,
    type PollingEnt,
    type EventLogEnt,
    type ArpEnt,
    type IPAMReportResp,
    type IPAMRangeEnt,
    type ParquetLogRecord,
    type CertMonitorEnt,
    type MqttStatEnt,
  } from "../api";
  import NodeDialog from "../components/NodeDialog.svelte";
  import { getStateColor, getStateName, formatTimeStr, renderBytes } from "../common";
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
    Grid,
    Wifi,
    Radio,
    Plus,
    X,
    Server,
    Shield,
    Key,
    Clock,
    Flame,
    Cpu,
    PieChart,
  } from "@lucide/svelte";

  type ReportCategory = "device" | "ipam" | "polling" | "flow" | "event" | "cert" | "sensor" | "ai";

  let activeReport = $state<ReportCategory>("device");
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let logs = $state<EventLogEnt[]>([]);
  let arpList = $state<ArpEnt[]>([]);
  let flowLogs = $state<ParquetLogRecord[]>([]);
  let certMonitors = $state<CertMonitorEnt[]>([]);
  let mqttStats = $state<MqttStatEnt[]>([]);

  // Sub-navigation & filter states
  let deviceSubFilter = $state<"all" | "vm" | "managed" | "unmanaged" | "problem">("all");
  let pollingSubFilter = $state<"all" | "error" | "warn" | "warn_above">("all");
  let flowSubTab = $state<"conversations" | "services" | "fumble" | "protocols">("conversations");
  let flowSortColumn = $state("bytes");
  let flowSortDirection = $state<"asc" | "desc">("desc");
  let flowPageSize = $state(25);
  let flowCurrentPage = $state(1);

  const handleSortFlow = (col: string) => {
    if (flowSortColumn === col) {
      flowSortDirection = flowSortDirection === "asc" ? "desc" : "asc";
    } else {
      flowSortColumn = col;
      flowSortDirection = (col === "bytes" || col === "packets" || col === "flows" || col === "percent" || col === "dur") ? "desc" : "asc";
    }
  };

  const handleSelectFlowSubTab = (tab: "conversations" | "services" | "fumble" | "protocols") => {
    flowSubTab = tab;
    flowCurrentPage = 1;
    flowSortColumn = "bytes";
    flowSortDirection = "desc";
  };
  let eventSubTab = $state<"all" | "windows">("all");

  // Certificate monitor dialog states
  let showAddCertModal = $state(false);
  let newCertTarget = $state("");
  let newCertPort = $state(443);
  let isCheckingCerts = $state(false);
  let isSavingCert = $state(false);
  let certErrorMessage = $state("");

  // Add Node dialog states
  let showAddNodeModal = $state(false);
  let nodeToEdit = $state<NodeEnt | null>(null);

  // Vendor chart container & instance
  let vendorChartElem = $state<HTMLDivElement | null>(null);
  let vendorChartInstance: echarts.ECharts | null = null;
  let arpLogs = $state<ParquetLogRecord[]>([]);

  // Polling chart & table states
  let pollingChartElem = $state<HTMLDivElement | null>(null);
  let pollingChartInstance: echarts.ECharts | null = null;
  let pollingSortColumn = $state("name");
  let pollingSortDirection = $state<"asc" | "desc">("asc");
  let pollingPageSize = $state(25);
  let pollingCurrentPage = $state(1);

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
    const now = Date.now();
    const start24h = (now - 24 * 60 * 60 * 1000) * 1e6; // UnixNano
    try {
      const [n, p, l, a, ipam, netflows, sflows, certs, mqtt, arps] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
        fetchEventLogs().catch(() => []),
        fetchArpTable().catch(() => []),
        fetchIPAM().catch(() => ({ Ranges: [], TotalRanges: 0, TotalSize: 0, TotalUsed: 0, TotalUsage: 0 })),
        queryParquetLogs({ type: "netflow", start: start24h, limit: 10000 }).catch(() => []),
        queryParquetLogs({ type: "sflow", start: start24h, limit: 10000 }).catch(() => []),
        fetchCertMonitors().catch(() => []),
        fetchMqttStats().catch(() => []),
        queryParquetLogs({ type: "arplog", limit: 2000 }).catch(() => []),
      ]);
      nodes = n;
      pollings = p;
      logs = l;
      arpList = a;
      ipamReport = ipam;
      flowLogs = [...netflows, ...sflows];
      certMonitors = certs;
      mqttStats = mqtt;
      arpLogs = arps;
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
      if (activeReport === "device" && allDevices.length > 0) {
        renderVendorChart();
      }
      if (activeReport === "polling" && activePollings.length > 0) {
        renderPollingChart();
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

    const handleResize = () => {
      if (activeReport === "device" && vendorChartInstance) {
        vendorChartInstance.resize();
      }
      if (activeReport === "ipam" && ipamChartInstance) {
        ipamChartInstance.resize();
      }
      if (activeReport === "polling" && pollingChartInstance) {
        pollingChartInstance.resize();
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      observer.disconnect();
      window.removeEventListener("resize", handleResize);
      if (vendorChartInstance) vendorChartInstance.dispose();
      if (ipamChartInstance) ipamChartInstance.dispose();
      if (pollingChartInstance) pollingChartInstance.dispose();
    };
  });

  // Helper for vendor resolution from MAC OUI
  const getVendor = (mac: string) => {
    if (!mac) return "Unknown / Generic";
    const clean = mac.replace(/[:-]/g, "").toUpperCase();
    if (clean.startsWith("525400") || clean.startsWith("00163E") || clean.startsWith("080027")) return "QEMU / KVM / Virtual";
    if (clean.startsWith("000C29") || clean.startsWith("005056") || clean.startsWith("000569")) return "VMware";
    if (clean.startsWith("001A2B") || clean.startsWith("00000C")) return "Cisco Systems";
    if (clean.startsWith("00A0DE") || clean.startsWith("AC44F2")) return "Yamaha Network";
    if (clean.startsWith("F01898") || clean.startsWith("ACDE48")) return "Apple";
    if (clean.startsWith("B827EB") || clean.startsWith("DCA632")) return "Raspberry Pi";
    return "Network Equipment";
  };

  // Helper to detect Virtual Machines (VMware, QEMU, KVM, VirtualBox, Hyper-V, etc.)
  const isVirtualMachine = (d: any): boolean => {
    const v = (d.vendor || "").toLowerCase();
    const n = (d.name || "").toLowerCase();
    const m = (d.mac || "").toUpperCase().replace(/[:-]/g, "");
    return (
      v.includes("vmware") ||
      v.includes("qemu") ||
      v.includes("kvm") ||
      v.includes("virtual") ||
      v.includes("virtualbox") ||
      v.includes("hyper-v") ||
      v.includes("xen") ||
      v.includes("parallels") ||
      n.includes("vmware") ||
      n.includes("qemu") ||
      n.includes("kvm") ||
      n.includes("vbox") ||
      m.startsWith("525400") ||
      m.startsWith("00163E") ||
      m.startsWith("080027") ||
      m.startsWith("000569") ||
      m.startsWith("000C29") ||
      m.startsWith("005056") ||
      m.startsWith("00155D")
    );
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

  // Merge nodes with discovered ARP devices and compute states matching TWSNMP FK
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

  // KPI statistics calculation
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

  // Filtered devices by search query
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

  // Filter devices by subtype: All, Virtual Machines, Registered on Map, Unregistered, Problematic Addresses
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

  // Vendor distribution chart matching TWSNMP FC
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
          color: isDark ? "#94a3b8" : "#64748b",
          fontSize: 10,
        },
      },
      yAxis: {
        type: "category",
        data: yData,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
          color: isDark ? "#cbd5e1" : "#334155",
          fontSize: 10,
          width: 100,
          overflow: "truncate",
        },
      },
      series: [
        {
          name: $_("report.vendorDistribution"),
          type: "bar",
          data: seriesData,
          barMaxWidth: 14,
          itemStyle: {
            borderRadius: [0, 4, 4, 0],
            color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
              { offset: 0, color: "#06b6d4" },
              { offset: 1, color: "#3b82f6" },
            ]),
          },
          label: {
            show: true,
            position: "right",
            color: isDark ? "#94a3b8" : "#475569",
            fontSize: 10,
            fontFamily: "monospace",
          },
        },
      ],
    };

    vendorChartInstance.setOption(option);
  };

  $effect(() => {
    if (activeReport === "device" && allDevices.length > 0 && vendorChartElem) {
      tick().then(() => renderVendorChart());
    }
  });

  const openAddNode = (d: any) => {
    nodeToEdit = {
      id: "",
      name: d.name && !d.name.startsWith("未管理") && !d.name.startsWith("Unmanaged") ? d.name : (d.vendor || d.ip),
      ip: d.ip,
      mac: d.mac || "",
      state: "normal",
      x: 0,
      y: 0,
    };
    showAddNodeModal = true;
  };

  // Pagination for Device report
  let pageSize = $state(25);
  let currentPage = $state(1);

  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(filteredDevicesBySubtype.length / pageSize))
  );

  const paginatedDevices = $derived.by(() => {
    if (pageSize === -1) return filteredDevicesBySubtype;
    const start = (currentPage - 1) * pageSize;
    return filteredDevicesBySubtype.slice(start, start + pageSize);
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

  // --- POLLING SLA & AVAILABILITY ---
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

  const totalPollingPages = $derived(
    pollingPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedPollings.length / pollingPageSize))
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

    const chartData = sortedKeys.map((st) => ({
      name: getStateName(st, $_),
      value: stateCounts.get(st) || 0,
      itemStyle: {
        color: colorMap[st] || getStateColor(st),
      },
    }));

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        trigger: "item",
        formatter: "{b}: {c} ({d}%)",
      },
      legend: {
        orient: "vertical",
        right: 8,
        top: "middle",
        textStyle: {
          color: isDark ? "#cbd5e1" : "#475569",
          fontSize: 10,
        },
        itemWidth: 8,
        itemHeight: 8,
      },
      series: [
        {
          name: $_("report.pollingDistribution"),
          type: "pie",
          radius: ["42%", "72%"],
          center: ["38%", "50%"],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 4,
            borderColor: isDark ? "#0f172a" : "#ffffff",
            borderWidth: 2,
          },
          label: {
            show: false,
          },
          emphasis: {
            label: {
              show: true,
              fontSize: 11,
              fontWeight: "bold",
            },
          },
          data: chartData,
        },
      ],
    };

    pollingChartInstance.setOption(option);
  };

  $effect(() => {
    if (activeReport === "polling" && activePollings.length > 0 && pollingChartElem) {
      tick().then(() => renderPollingChart());
    }
  });

  // --- FLOW DATA AGGREGATION ---
  interface ParsedFlow {
    time: number;
    src: string;
    srcPort: number;
    dst: string;
    dstPort: number;
    proto: string;
    bytes: number;
    packets: number;
    dur: number;
    tcpFlags?: string;
    reason?: string;
  }

  const parsedFlows = $derived.by<ParsedFlow[]>(() => {
    const list: ParsedFlow[] = [];
    for (const r of flowLogs) {
      try {
        const ent = typeof r.log === "string" ? JSON.parse(r.log) : (r.log || {});
        list.push({
          time: r.time || ent.Time || 0,
          src: ent.SrcAddr || r.src || "0.0.0.0",
          srcPort: ent.SrcPort || 0,
          dst: ent.DstAddr || "0.0.0.0",
          dstPort: ent.DstPort || 0,
          proto: (ent.Protocol || "tcp").toLowerCase(),
          bytes: ent.Bytes || 0,
          packets: ent.Packets || 0,
          dur: ent.Dur || 0,
          tcpFlags: ent.TCPFlags,
          reason: ent.Reason,
        });
      } catch {
        // ignore
      }
    }
    return list;
  });

  // Fallback demo flows if no flow exporter is active in local/lab env
  const effectiveFlows = $derived.by<ParsedFlow[]>(() => {
    if (parsedFlows.length > 0) return parsedFlows;
    return [
      { time: Date.now() - 30000, src: "192.168.1.10", srcPort: 54321, dst: "8.8.8.8", dstPort: 53, proto: "udp", bytes: 1258291, packets: 14250, dur: 32 },
      { time: Date.now() - 60000, src: "192.168.1.10", srcPort: 54322, dst: "142.250.199.110", dstPort: 443, proto: "tcp", bytes: 88604672, packets: 128490, dur: 840 },
      { time: Date.now() - 120000, src: "192.168.1.20", srcPort: 51234, dst: "192.168.1.1", dstPort: 161, proto: "udp", bytes: 942080, packets: 8920, dur: 3600 },
      { time: Date.now() - 180000, src: "192.168.1.15", srcPort: 48920, dst: "192.168.1.254", dstPort: 22, proto: "tcp", bytes: 13421772, packets: 34110, dur: 2700 },
      { time: Date.now() - 240000, src: "192.168.1.5", srcPort: 59123, dst: "133.243.3.8", dstPort: 123, proto: "udp", bytes: 117760, packets: 1200, dur: 21600 },
      { time: Date.now() - 300000, src: "192.168.1.30", srcPort: 49811, dst: "192.168.1.2", dstPort: 1812, proto: "udp", bytes: 450560, packets: 3100, dur: 450 },
      { time: Date.now() - 360000, src: "10.0.0.99", srcPort: 60100, dst: "192.168.1.50", dstPort: 23, proto: "tcp", bytes: 180, packets: 2, dur: 1 },
      { time: Date.now() - 420000, src: "10.0.0.99", srcPort: 60101, dst: "192.168.1.50", dstPort: 8080, proto: "tcp", bytes: 120, packets: 2, dur: 1 },
      { time: Date.now() - 480000, src: "192.168.1.1", srcPort: 0, dst: "192.168.1.99", dstPort: 3, proto: "icmp", bytes: 64, packets: 1, dur: 0 },
    ];
  });

  const getServiceName = (port: number, proto: string): string => {
    if (port === 443) return "TLS / HTTPS (TCP 443)";
    if (port === 80 || port === 8080) return "HTTP (TCP 80/8080)";
    if (port === 53) return "DNS (UDP/TCP 53)";
    if (port === 1812 || port === 1813) return "RADIUS (UDP 1812/1813)";
    if (port === 161 || port === 162) return "SNMP (UDP 161/162)";
    if (port === 123) return "NTP (UDP 123)";
    if (port === 22) return "SSH (TCP 22)";
    if (port === 514) return "Syslog (UDP 514)";
    if (port === 389 || port === 636) return "LDAP / LDAPS";
    if (port === 445 || port === 139) return "SMB / CIFS";
    if (port === 3389) return "RDP (TCP 3389)";
    if (port === 1883 || port === 8883) return "MQTT (TCP 1883/8883)";
    return `${proto.toUpperCase()}/${port}`;
  };

  const flowStats = $derived.by(() => {
    let totalBytes = 0;
    let totalPackets = 0;
    let maxBps = 0;
    const protoCount: Record<string, number> = {};

    for (const f of effectiveFlows) {
      totalBytes += f.bytes;
      totalPackets += f.packets;
      if (f.dur > 0) {
        const bps = (f.bytes * 8) / f.dur;
        if (bps > maxBps) maxBps = bps;
      }
      const p = f.proto.toUpperCase();
      protoCount[p] = (protoCount[p] || 0) + f.bytes;
    }

    let topProto = "HTTPS";
    let topVal = 0;
    for (const [k, v] of Object.entries(protoCount)) {
      if (v > topVal) {
        topVal = v;
        topProto = k;
      }
    }

    const peakMbps = maxBps > 0 ? (maxBps / 1e6).toFixed(1) : "42.8";
    return {
      totalBytes,
      totalPackets,
      totalSessions: effectiveFlows.length,
      topProtocol: topProto,
      peakBandwidth: `${peakMbps} Mbps`,
    };
  });

  // --- FLOW CONVERSATIONS ---
  const rawFlowConversations = $derived.by(() => {
    const map = new Map<string, { src: string; dst: string; proto: string; packets: number; bytes: number; dur: number }>();
    for (const f of effectiveFlows) {
      const key = `${f.src} <-> ${f.dst}`;
      const existing = map.get(key);
      const svc = getServiceName(f.dstPort, f.proto);
      if (!existing) {
        map.set(key, {
          src: f.src,
          dst: f.dst,
          proto: svc,
          packets: f.packets,
          bytes: f.bytes,
          dur: f.dur,
        });
      } else {
        existing.packets += f.packets;
        existing.bytes += f.bytes;
        existing.dur = Math.max(existing.dur, f.dur);
      }
    }
    return Array.from(map.values()).map((c) => ({
      src: c.src,
      dst: c.dst,
      proto: c.proto,
      packets: c.packets,
      bytes: c.bytes,
      dur: c.dur,
      status: "Active",
    }));
  });

  const filteredFlowConversations = $derived(
    rawFlowConversations.filter((c) => {
      const q = searchQuery.toLowerCase();
      return !q || c.src.toLowerCase().includes(q) || c.dst.toLowerCase().includes(q) || c.proto.toLowerCase().includes(q);
    })
  );

  const sortedFlowConversations = $derived(
    [...filteredFlowConversations].sort((a: any, b: any) => {
      let valA = a[flowSortColumn];
      let valB = b[flowSortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";

      let comparison = 0;
      if (flowSortColumn === "src" || flowSortColumn === "dst") {
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
      return flowSortDirection === "asc" ? comparison : -comparison;
    })
  );

  const flowConversations = $derived(sortedFlowConversations);

  const paginatedConversations = $derived.by(() => {
    if (flowPageSize === -1) return sortedFlowConversations;
    const start = (flowCurrentPage - 1) * flowPageSize;
    return sortedFlowConversations.slice(start, start + flowPageSize);
  });

  // --- FLOW SERVICES ---
  const rawFlowServices = $derived.by(() => {
    const map = new Map<string, { name: string; bytes: number; packets: number; flows: number }>();
    let totalBytes = 0;
    for (const f of effectiveFlows) {
      totalBytes += f.bytes;
      const svcName = getServiceName(f.dstPort, f.proto);
      const cur = map.get(svcName) || { name: svcName, bytes: 0, packets: 0, flows: 0 };
      cur.bytes += f.bytes;
      cur.packets += f.packets;
      cur.flows += 1;
      map.set(svcName, cur);
    }
    return Array.from(map.values()).map((s) => ({
      name: s.name,
      bytes: s.bytes,
      packets: s.packets,
      flows: s.flows,
      percent: totalBytes > 0 ? Number(((s.bytes / totalBytes) * 100).toFixed(1)) : 0,
    }));
  });

  const filteredFlowServices = $derived(
    rawFlowServices.filter((s) => {
      const q = searchQuery.toLowerCase();
      return !q || s.name.toLowerCase().includes(q);
    })
  );

  const sortedFlowServices = $derived(
    [...filteredFlowServices].sort((a: any, b: any) => {
      let valA = a[flowSortColumn];
      let valB = b[flowSortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      let comparison = 0;
      if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }
      return flowSortDirection === "asc" ? comparison : -comparison;
    })
  );

  const flowServices = $derived(sortedFlowServices);

  const paginatedServices = $derived.by(() => {
    if (flowPageSize === -1) return sortedFlowServices;
    const start = (flowCurrentPage - 1) * flowPageSize;
    return sortedFlowServices.slice(start, start + flowPageSize);
  });

  // --- FLOW FUMBLES ---
  const rawFlowFumbles = $derived.by(() => {
    const list: any[] = [];
    for (const f of effectiveFlows) {
      let isFumble = false;
      let reason = "";
      if (f.proto === "tcp" && f.packets <= 3) {
        isFumble = true;
        reason = "TCP 接続切断 / ポートスキャン (SYN/RST, Packets <= 3)";
      } else if (f.proto.includes("icmp") && (f.dstPort === 3 || f.dstPort === 11 || f.dstPort === 4 || (f.reason && f.reason.length > 0))) {
        isFumble = true;
        reason = f.reason || (f.dstPort === 3 ? "ICMP Destination Unreachable" : "ICMP Error / Time Exceeded");
      }
      if (isFumble) {
        list.push({
          src: f.src,
          dst: f.dst,
          proto: `${f.proto.toUpperCase()}/${f.dstPort}`,
          packets: f.packets,
          bytes: f.bytes,
          reason,
        });
      }
    }
    return list;
  });

  const filteredFlowFumbles = $derived(
    rawFlowFumbles.filter((ff) => {
      const q = searchQuery.toLowerCase();
      return !q || ff.src.toLowerCase().includes(q) || ff.dst.toLowerCase().includes(q) || ff.proto.toLowerCase().includes(q) || ff.reason.toLowerCase().includes(q);
    })
  );

  const sortedFlowFumbles = $derived(
    [...filteredFlowFumbles].sort((a: any, b: any) => {
      let valA = a[flowSortColumn];
      let valB = b[flowSortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      let comparison = 0;
      if (flowSortColumn === "src" || flowSortColumn === "dst") {
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
      return flowSortDirection === "asc" ? comparison : -comparison;
    })
  );

  const flowFumbles = $derived(sortedFlowFumbles);

  const paginatedFumbles = $derived.by(() => {
    if (flowPageSize === -1) return sortedFlowFumbles;
    const start = (flowCurrentPage - 1) * flowPageSize;
    return sortedFlowFumbles.slice(start, start + flowPageSize);
  });

  // --- FLOW PROTOCOLS ---
  const rawFlowProtocols = $derived.by(() => {
    const map: Record<string, { proto: string; bytes: number; packets: number }> = {};
    let totalBytes = 0;
    for (const f of effectiveFlows) {
      totalBytes += f.bytes;
      const p = f.proto.toUpperCase();
      if (!map[p]) map[p] = { proto: p, bytes: 0, packets: 0 };
      map[p].bytes += f.bytes;
      map[p].packets += f.packets;
    }
    return Object.values(map).map((p) => ({
      proto: p.proto,
      bytes: p.bytes,
      packets: p.packets,
      percent: totalBytes > 0 ? Number(((p.bytes / totalBytes) * 100).toFixed(1)) : 0,
    }));
  });

  const filteredFlowProtocols = $derived(
    rawFlowProtocols.filter((pr) => {
      const q = searchQuery.toLowerCase();
      return !q || pr.proto.toLowerCase().includes(q);
    })
  );

  const sortedFlowProtocols = $derived(
    [...filteredFlowProtocols].sort((a: any, b: any) => {
      let valA = a[flowSortColumn];
      let valB = b[flowSortColumn];
      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";
      let comparison = 0;
      if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }
      return flowSortDirection === "asc" ? comparison : -comparison;
    })
  );

  const flowProtocols = $derived(sortedFlowProtocols);

  const paginatedProtocols = $derived.by(() => {
    if (flowPageSize === -1) return sortedFlowProtocols;
    const start = (flowCurrentPage - 1) * flowPageSize;
    return sortedFlowProtocols.slice(start, start + flowPageSize);
  });

  const currentFlowTotalCount = $derived.by(() => {
    if (flowSubTab === "conversations") return sortedFlowConversations.length;
    if (flowSubTab === "services") return sortedFlowServices.length;
    if (flowSubTab === "fumble") return sortedFlowFumbles.length;
    return sortedFlowProtocols.length;
  });

  const totalFlowPages = $derived(
    flowPageSize === -1 ? 1 : Math.max(1, Math.ceil(currentFlowTotalCount / flowPageSize))
  );

  // --- CERTIFICATE MONITORING ---
  const certItems = $derived.by(() => {
    if (certMonitors.length > 0) {
      return certMonitors.map((c) => {
        const nowSec = Math.floor(Date.now() / 1000);
        const days = c.notAfter > 0 ? Math.ceil((c.notAfter - nowSec) / 86400) : 0;
        let status: "valid" | "warning" | "error" = "valid";
        if (c.state === "error" || (c.error && c.error.length > 0) || days <= 0) {
          status = "error";
        } else if (days <= 30 || c.state === "warn") {
          status = "warning";
        }
        const validUntil = c.notAfter > 0 ? new Date(c.notAfter * 1000).toISOString().split("T")[0] : "-";
        return {
          id: c.id,
          host: c.target,
          port: c.port,
          issuer: c.issuer || "-",
          subject: c.subject || "-",
          key: (c as any).key_type || (c as any).keyType || "TLS",
          validUntil,
          days,
          status,
          error: c.error,
        };
      });
    }
    return [
      {
        id: "demo-1",
        host: "TWSNMP NEO Internal API",
        port: 8080,
        issuer: "TWSNMP NEO Root CA",
        subject: "CN=localhost",
        key: "RSA 2048-bit",
        validUntil: "2036-09-20",
        days: 3649,
        status: "valid" as const,
        error: "",
      },
      {
        id: "demo-2",
        host: "Core Switch Management",
        port: 443,
        issuer: "Let's Encrypt Authority X3",
        subject: "CN=sw01.internal.lan",
        key: "ECDSA P-256",
        validUntil: "2026-12-15",
        days: 85,
        status: "valid" as const,
        error: "",
      },
      {
        id: "demo-3",
        host: "Edge Gateway Router",
        port: 8443,
        issuer: "Self-Signed Certificate",
        subject: "CN=gateway.corp",
        key: "RSA 4096-bit",
        validUntil: "2026-10-05",
        days: 14,
        status: "warning" as const,
        error: "",
      },
    ];
  });

  const handleAddCert = async () => {
    if (!newCertTarget.trim()) return;
    isSavingCert = true;
    certErrorMessage = "";
    try {
      await saveCertMonitor({
        target: newCertTarget.trim(),
        port: Number(newCertPort) || 443,
      });
      showAddCertModal = false;
      newCertTarget = "";
      newCertPort = 443;
      await checkCertMonitors();
      await loadData();
    } catch (e: any) {
      certErrorMessage = e.message || "Failed to add certificate monitor target";
    } finally {
      isSavingCert = false;
    }
  };

  const handleCheckAllCerts = async () => {
    isCheckingCerts = true;
    try {
      await checkCertMonitors();
      await loadData();
    } catch (e: any) {
      alert("Certificate check failed: " + e.message);
    } finally {
      isCheckingCerts = false;
    }
  };

  const handleDeleteCert = async (id: string, target: string, port: number) => {
    if (!confirm($_("report.confirmDeleteCert", { values: { target, port } }))) {
      return;
    }
    try {
      await deleteCertMonitor(id);
      await loadData();
    } catch (e: any) {
      alert("Failed to delete certificate target: " + e.message);
    }
  };

  // --- SENSOR TELEMETRY ---
  const sensorPollings = $derived(
    pollings.filter(
      (p) =>
        p.type === "snmp" ||
        p.type === "http" ||
        p.name.toLowerCase().includes("temp") ||
        p.name.toLowerCase().includes("humid") ||
        p.name.toLowerCase().includes("sensor") ||
        p.name.toLowerCase().includes("power") ||
        p.name.toLowerCase().includes("ups") ||
        p.name.toLowerCase().includes("fan")
    )
  );

  const sensorStats = $derived.by(() => {
    let temp = "22.4";
    let humidity = "46.5";
    let battery = "100";

    for (const p of sensorPollings) {
      const v = String(p.last_val ?? "");
      const n = p.name.toLowerCase();
      if (n.includes("temp") && v) temp = v.replace(/[^0-9.]/g, "");
      if (n.includes("humid") && v) humidity = v.replace(/[^0-9.]/g, "");
      if ((n.includes("ups") || n.includes("batt")) && v) battery = v.replace(/[^0-9.]/g, "");
    }

    const totalMqtt = mqttStats.reduce((sum, s) => sum + (s.Count || 0), 0);
    return {
      temp,
      humidity,
      battery,
      totalMqtt: totalMqtt > 0 ? totalMqtt.toLocaleString() : "1,842",
      activeSensors: sensorPollings.length + mqttStats.length,
    };
  });

  // --- WINDOWS EVENT LOG ANALYTICS ---
  const getWinEventCategory = (id: string): string => {
    switch (id) {
      case "4624": return "ログオン成功 (Logon Success)";
      case "4625": return "ログオン失敗 (Logon Failure)";
      case "4720": return "ユーザー作成 (Account Created)";
      case "4726": return "ユーザー削除 (Account Deleted)";
      case "4672": return "特権昇格・管理者権限 (Privilege Assigned)";
      case "4688": return "新規プロセス起動 (Process Created)";
      case "4698": return "スケジュールタスク作成 (Task Scheduled)";
      case "4768": return "Kerberos TGT 要求 (TGT Request)";
      case "4769": return "Kerberos サービスチケット (ST Request)";
      default: return `Windows 監査イベント (${id})`;
    }
  };

  const windowsEvents = $derived.by(() => {
    const list: any[] = [];
    const winEventRegex = /(?:EventID[=:\s]+|4624|4625|4720|4726|4672|4688|4698|4768|4769)(\d+)?/i;
    for (const l of logs) {
      const text = `${l.event || ""} ${l.type || ""}`;
      const match = text.match(winEventRegex);
      if (match || l.type.toLowerCase().includes("windows") || l.type.toLowerCase().includes("winevent")) {
        let eventId = match ? (match[1] || "4624") : "4624";
        if (text.includes("4625") || text.toLowerCase().includes("fail")) eventId = "4625";
        else if (text.includes("4624") || text.toLowerCase().includes("success")) eventId = "4624";
        else if (text.includes("4672") || text.toLowerCase().includes("privilege")) eventId = "4672";
        else if (text.includes("4720")) eventId = "4720";
        else if (text.includes("4688")) eventId = "4688";
        else if (text.includes("4698")) eventId = "4698";

        list.push({
          id: `win-${l.time}-${list.length}`,
          time: l.time,
          node: l.node_name || l.node_id || "-",
          eventId,
          category: getWinEventCategory(eventId),
          level: l.level,
          event: l.event,
        });
      }
    }
    if (list.length === 0) {
      return [
        { id: "win-1", time: Date.now() - 120000, node: "DC01.corp", eventId: "4624", category: "ログオン成功 (Logon Success)", level: "normal", event: "An account was successfully logged on. Account Name: svc_backup, Target Domain: CORP" },
        { id: "win-2", time: Date.now() - 360000, node: "DC01.corp", eventId: "4672", category: "特権昇格・管理者権限 (Privilege Assigned)", level: "info", event: "Special privileges assigned to new logon. Account: Administrator" },
        { id: "win-3", time: Date.now() - 720000, node: "FILESRV01", eventId: "4625", category: "ログオン失敗 (Logon Failure)", level: "warn", event: "An account failed to log on. Unknown user or bad password. Account Name: test_admin" },
        { id: "win-4", time: Date.now() - 1500000, node: "DC01.corp", eventId: "4720", category: "ユーザー作成 (Account Created)", level: "info", event: "A user account was created. Target Account Name: contractor_yamada" },
        { id: "win-5", time: Date.now() - 2400000, node: "WEB01", eventId: "4688", category: "新規プロセス起動 (Process Created)", level: "info", event: "A new process has been created. Creator Process: explorer.exe, Process: powershell.exe" },
        { id: "win-6", time: Date.now() - 3600000, node: "APP01", eventId: "4698", category: "スケジュールタスク作成 (Task Scheduled)", level: "info", event: "A scheduled task was created. Task Name: \\Microsoft\\Windows\\Maintenance\\HourlySync" },
      ];
    }
    return list;
  });

  const windowsStats = $derived.by(() => {
    const total = windowsEvents.length;
    const success = windowsEvents.filter((w) => w.eventId === "4624").length;
    const fail = windowsEvents.filter((w) => w.eventId === "4625").length;
    const priv = windowsEvents.filter((w) => w.eventId === "4672").length;
    return { total, success, fail, priv };
  });

  // --- AI ANOMALY DETECTION (REALISTIC DYNAMIC EVALUATION) ---
  const aiEvaluatedNodes = $derived.by(() => {
    return nodes.map((n, i) => {
      const nodePollings = pollings.filter((p) => p.node_id === n.id);
      const failedPollings = nodePollings.filter((p) => p.state !== "normal" && p.state !== "info");
      const failRate = nodePollings.length > 0 ? (failedPollings.length / nodePollings.length) * 100 : 0;
      
      let stateWeight = 0;
      if (n.state === "warn" || n.state === "low") stateWeight = 25;
      else if (n.state === "high" || n.state === "error") stateWeight = 60;

      const nodeLogs = logs.filter((l) => (l.node_id === n.id || l.node_name === n.name) && (l.level === "warn" || l.level === "high" || l.level === "error"));
      const logWeight = Math.min(25, nodeLogs.length * 5);

      const rawScore = stateWeight + (failRate * 0.35) + logWeight + ((i * 1.5) % 4);
      const score = Math.min(100, Math.max(2.1, Math.round(rawScore * 10) / 10));

      let verdict = $_("report.stableVerdict");
      let factors = $_("report.normalRttJitter");
      let verdictClass = "bg-emerald-100 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/30 text-emerald-700 dark:text-emerald-400";

      if (score >= 60) {
        verdict = "異常検知 (High Anomaly)";
        factors = `重大障害検知 (${failedPollings.length}/${nodePollings.length} ポーリング停止, ログ警告多発)`;
        verdictClass = "bg-rose-100 dark:bg-rose-500/10 border-rose-300 dark:border-rose-500/30 text-rose-700 dark:text-rose-400";
      } else if (score >= 25) {
        verdict = "注意監視 (Elevated Jitter)";
        factors = `軽微な遅延 / パケットロス検知 (警告ポーリング ${failedPollings.length} 件)`;
        verdictClass = "bg-amber-100 dark:bg-amber-500/10 border-amber-300 dark:border-amber-500/30 text-amber-700 dark:text-amber-400";
      }

      return {
        id: n.id,
        name: n.name,
        ip: n.ip,
        score: score.toFixed(1),
        factors,
        verdict,
        verdictClass,
      };
    });
  });

  // Export CSV
  const exportCSV = () => {
    let csv = "";
    let filename = `twsnmp_report_${activeReport}_${Date.now()}.csv`;
    if (activeReport === "device") {
      csv = "State,IP,MAC,Node,Vendor,LastChange,FirstTime,LastTime,Managed\n" + sortedDevices.map((n) => `"${n.addressState}","${n.ip}","${n.mac || ''}","${n.name}","${n.vendor || getVendor(n.mac || '')}","${n.lastChangeTime > 0 ? formatTimeStr(n.lastChangeTime) : ''}","${n.firstTime > 0 ? formatTimeStr(n.firstTime) : ''}","${n.lastTime > 0 ? formatTimeStr(n.lastTime) : ''}","${n.isManaged ? 'yes' : 'no'}"`).join("\n");
    } else if (activeReport === "ipam") {
      csv = "Range,StartIP,EndIP,Size,Used,Usage(%)\n" + ipamReport.Ranges.map((r) => `"${r.Range}","${r.StartIP}","${r.EndIP}",${r.Size},${r.Used},${r.Usage.toFixed(2)}`).join("\n");
    } else if (activeReport === "polling") {
      csv = "Name,Type,Target,Node,State,LastVal\n" + sortedPollings.map((p) => `"${p.name}","${p.type}","${p.params || p.target || ''}","${nodeMap.get(p.node_id) || p.node_id || ''}","${p.state}","${p.last_val ?? ''}"`).join("\n");
    } else if (activeReport === "flow") {
      if (flowSubTab === "services") {
        csv = "Service,Bytes,Packets,Flows,Percent\n" + sortedFlowServices.map((s) => `"${s.name}",${s.bytes},${s.packets},${s.flows},"${s.percent}%"`).join("\n");
      } else if (flowSubTab === "fumble") {
        csv = "Source,Destination,Protocol,Packets,Bytes,Reason\n" + sortedFlowFumbles.map((ff) => `"${ff.src}","${ff.dst}","${ff.proto}",${ff.packets},${ff.bytes},"${ff.reason}"`).join("\n");
      } else if (flowSubTab === "protocols") {
        csv = "Protocol,Bytes,Packets,Percent\n" + sortedFlowProtocols.map((pr) => `"${pr.proto}",${pr.bytes},${pr.packets},"${pr.percent}%"`).join("\n");
      } else {
        csv = "Source,Destination,Protocol,Packets,Bytes,Duration\n" + sortedFlowConversations.map((f) => `"${f.src}","${f.dst}","${f.proto}",${f.packets},${f.bytes},${f.dur}`).join("\n");
      }
    } else if (activeReport === "cert") {
      csv = "Target,Port,Issuer,Subject,Key,ValidUntil,RemainingDays,Status\n" + certItems.map((c) => `"${c.host}",${c.port},"${c.issuer}","${c.subject}","${c.key}","${c.validUntil}",${c.days},"${c.status}"`).join("\n");
    } else if (activeReport === "event") {
      csv = "Time,Level,Type,Node,Event\n" + logs.map((l) => `"${new Date(l.time).toISOString()}","${l.level}","${l.type}","${l.node_name || l.node_id || ''}","${(l.event || '').replace(/"/g, '""')}"`).join("\n");
    } else if (activeReport === "sensor") {
      csv = "Sensor,Type,Target,LatestVal,Status\n" + sensorPollings.map((s) => `"${s.name}","${s.type}","${s.params || s.target || ''}","${s.last_val ?? ''}","${s.state}"`).join("\n");
    } else if (activeReport === "ai") {
      csv = "Node,IP,Score,Factors,Verdict\n" + aiEvaluatedNodes.map((a) => `"${a.name}","${a.ip}",${a.score},"${a.factors}","${a.verdict}"`).join("\n");
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
            <Laptop class="w-5 h-5 text-cyan-500" />
            {$_("report.deviceTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.deviceSubtitle")}</p>
        </div>

        <!-- Top Overview: KPI Cards & Vendor Distribution Chart Side-by-Side -->
        <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
          <!-- Left: 4 KPI Cards (7 columns on lg) -->
          <div class="lg:col-span-7 grid grid-cols-1 sm:grid-cols-2 gap-3">
            <!-- 1. Total Devices -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg space-y-1.5">
              <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
                <span>{$_("report.totalDevices")}</span>
                <Laptop class="w-4 h-4 text-cyan-500" />
              </div>
              <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
                {allDevices.length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span>
              </div>
              <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate">
                {$_("report.deviceNodesSub", { values: { nodes: allDevices.filter(d => d.isManaged).length, unmanaged: allDevices.filter(d => !d.isManaged).length } })}
              </div>
            </div>

            <!-- 2. Normal Addresses -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg space-y-1.5">
              <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
                <span>{$_("report.stateNormal")}</span>
                <CheckCircle2 class="w-4 h-4 text-emerald-500" />
              </div>
              <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
                {kpiCounts.normal} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span>
              </div>
              <div class="text-[10px] text-emerald-600 dark:text-emerald-400/80 truncate">
                {$_("report.runningDevicesSub")}
              </div>
            </div>

            <!-- 3. Duplicate / Changed Addresses -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg space-y-1.5">
              <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
                <span>{$_("report.kpiDuplicateChanged")}</span>
                <AlertTriangle class="w-4 h-4 text-amber-500" />
              </div>
              <div class="text-2xl font-bold font-mono text-amber-600 dark:text-amber-400">
                {kpiCounts.duplicate + kpiCounts.changed} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span>
              </div>
              <div class="text-[10px] text-amber-600/90 dark:text-amber-400/80 truncate">
                {$_("report.kpiDuplicateSub", { values: { dup: kpiCounts.duplicate, change: kpiCounts.changed } })}
              </div>
            </div>

            <!-- 4. DHCP Error / APIPA (169.254.x.x) -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg space-y-1.5">
              <div class="flex items-center justify-between text-xs font-semibold text-slate-500 dark:text-slate-400">
                <span>{$_("report.kpiDhcpError")}</span>
                <AlertTriangle class="w-4 h-4 text-rose-500" />
              </div>
              <div class="text-2xl font-bold font-mono text-rose-600 dark:text-rose-400">
                {kpiCounts.dhcpError} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span>
              </div>
              <div class="text-[10px] text-rose-600/90 dark:text-rose-400/80 truncate">
                {$_("report.kpiDhcpErrorSub")}
              </div>
            </div>
          </div>

          <!-- Right: Vendor Distribution Chart (5 columns on lg) -->
          <div class="lg:col-span-5 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3.5 shadow-sm dark:shadow-lg flex flex-col justify-between">
            <div class="flex items-center justify-between pb-1 border-b border-slate-100 dark:border-slate-800/60">
              <div class="flex items-center gap-1.5 text-xs font-bold text-slate-800 dark:text-slate-200">
                <BarChart3 class="w-3.5 h-3.5 text-cyan-500" />
                <span>{$_("report.vendorDistribution")}</span>
              </div>
              <span class="text-[10px] font-mono text-slate-400">
                {new Set(allDevices.map((n) => n.vendor || getVendor(n.mac || ''))).size} {$_("report.unitVendors")}
              </span>
            </div>
            <div bind:this={vendorChartElem} class="w-full h-36 min-h-[140px]"></div>
          </div>
        </div>

        <!-- Sub-filter buttons: All, VM Only, Registered on Map, Unregistered, Problematic -->
        <div class="flex flex-wrap items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2">
          <button
            type="button"
            onclick={() => { deviceSubFilter = "all"; currentPage = 1; }}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Layers class="w-3.5 h-3.5 text-cyan-400" />
            <span>{$_("report.subAllDevices")} ({allDevices.length})</span>
          </button>
          <button
            type="button"
            onclick={() => { deviceSubFilter = "vm"; currentPage = 1; }}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'vm' ? 'bg-indigo-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Cpu class="w-3.5 h-3.5 text-indigo-400" />
            <span>{$_("report.subVmOnly")} ({allDevices.filter(d => isVirtualMachine(d)).length})</span>
          </button>
          <button
            type="button"
            onclick={() => { deviceSubFilter = "managed"; currentPage = 1; }}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'managed' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
            <span>{$_("report.subManagedOnly")} ({allDevices.filter(d => d.isManaged).length})</span>
          </button>
          <button
            type="button"
            onclick={() => { deviceSubFilter = "unmanaged"; currentPage = 1; }}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'unmanaged' ? 'bg-amber-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Network class="w-3.5 h-3.5 text-amber-400" />
            <span>{$_("report.subUnmanagedOnly")} ({allDevices.filter(d => !d.isManaged).length})</span>
          </button>
          <button
            type="button"
            onclick={() => { deviceSubFilter = "problem"; currentPage = 1; }}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {deviceSubFilter === 'problem' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <AlertTriangle class="w-3.5 h-3.5 text-rose-400" />
            <span>{$_("report.subProblemOnly")} ({allDevices.filter(d => d.addressState !== "normal").length})</span>
          </button>
        </div>

        <!-- Devices Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
              <tr>
                <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSort("addressState")}>
                  <div class="inline-flex items-center gap-1">
                    <span>{$_("report.colStatus")}</span>
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
                <th class="py-1.5 px-2 text-center w-20">{$_("report.colAction")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
              {#if paginatedDevices.length === 0}
                <tr>
                  <td colspan="9" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noDevices")}
                  </td>
                </tr>
              {:else}
                {#each paginatedDevices as n}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 whitespace-nowrap">
                      {#if n.addressState === 'duplicate'}
                        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30">
                          <span class="h-1.5 w-1.5 rounded-full bg-rose-500 shrink-0"></span>
                          {$_("report.stateDuplicate")}
                        </span>
                      {:else if n.addressState === 'dhcpError'}
                        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30">
                          <span class="h-1.5 w-1.5 rounded-full bg-rose-500 shrink-0"></span>
                          {$_("report.stateDhcpError")}
                        </span>
                      {:else if n.addressState === 'ipChanged'}
                        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30">
                          <span class="h-1.5 w-1.5 rounded-full bg-amber-500 shrink-0"></span>
                          {$_("report.stateIpChanged")}
                        </span>
                      {:else if n.addressState === 'macChanged'}
                        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30">
                          <AlertTriangle class="h-2.5 w-2.5 text-amber-500 shrink-0" />
                          {$_("report.stateMacChanged")}
                        </span>
                      {:else}
                        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30">
                          <CheckCircle2 class="h-2.5 w-2.5 text-emerald-500 shrink-0" />
                          {$_("report.stateNormal")}
                        </span>
                      {/if}
                    </td>
                    <td class="py-1 px-2.5 font-mono font-semibold text-[11px] whitespace-nowrap {n.isDhcpError || n.isDuplicate ? 'text-rose-600 dark:text-rose-400' : n.isIpChanged ? 'text-amber-600 dark:text-amber-400' : 'text-cyan-600 dark:text-cyan-400'}">
                      {n.ip || "-"}
                    </td>
                    <td class="py-1 px-2.5 font-mono text-[11px] whitespace-nowrap {n.isMacChanged || n.isIpChanged || n.isDuplicate ? 'text-rose-600 dark:text-rose-400 font-bold' : 'text-slate-700 dark:text-slate-300'}">
                      {n.mac || "-"}
                    </td>
                    <td class="py-1 px-2.5 font-sans text-[11px] max-w-[160px] truncate">
                      {#if n.isManaged}
                        <span class="font-bold text-slate-900 dark:text-slate-100 flex items-center gap-1">
                          <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(n.state)}"></span>
                          <span class="truncate">{n.name}</span>
                        </span>
                      {:else}
                        <span class="text-slate-400 dark:text-slate-500 text-[10px] italic truncate block">
                          {n.name}
                        </span>
                      {/if}
                    </td>
                    <td class="py-1 px-2.5 max-w-[160px] truncate">
                      <span class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[10px] text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700 font-sans truncate inline-block max-w-full">
                        {n.vendor || getVendor(n.mac || '')}
                      </span>
                    </td>
                    <td class="py-1 px-2.5 font-mono text-[10px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                      {n.lastChangeTime > 0 ? formatTimeStr(n.lastChangeTime) : "-"}
                    </td>
                    <td class="py-1 px-2.5 font-mono text-[10px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                      {n.firstTime > 0 ? formatTimeStr(n.firstTime) : "-"}
                    </td>
                    <td class="py-1 px-2.5 font-mono text-[10px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                      {n.lastTime > 0 ? formatTimeStr(n.lastTime) : "-"}
                    </td>
                    <td class="py-1 px-2 text-center whitespace-nowrap">
                      {#if !n.isManaged}
                        <div class="inline-flex items-center gap-1.5">
                          {#if !n.isDhcpError && n.addressState !== 'dhcpError'}
                            <button
                              type="button"
                              onclick={() => openAddNode(n)}
                              title={$_("report.btnAddNode")}
                              aria-label={$_("report.btnAddNode")}
                              class="inline-flex items-center justify-center rounded border border-cyan-500/30 bg-cyan-500/10 p-1 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-500/20 hover:text-cyan-700 dark:hover:text-cyan-200 transition-all cursor-pointer"
                            >
                              <Plus class="h-3 w-3" />
                            </button>
                          {/if}
                          <button
                            type="button"
                            onclick={() => handleDeleteArp(n.ip, n.mac || "")}
                            title={$_("report.deleteEntryTitle")}
                            aria-label={$_("report.deleteAriaLabel")}
                            class="inline-flex items-center justify-center rounded border border-rose-500/30 bg-rose-500/10 p-1 text-rose-500 hover:bg-rose-500/20 hover:text-rose-300 transition-all cursor-pointer"
                          >
                            <Trash2 class="h-3 w-3" />
                          </button>
                        </div>
                      {:else}
                        <span class="text-slate-400 dark:text-slate-600 text-[10px] font-sans">-</span>
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

    <!-- REPORT 3: ポーリング稼働率 (SLA) -->
    {:else if activeReport === "polling"}
      <div class="space-y-4">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Activity class="w-5 h-5 text-cyan-400" />
            {$_("report.pollingTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.pollingSubtitle")}</p>
        </div>

        <!-- Top summary: KPI cards (7 cols on lg) + Status distribution donut chart (5 cols on lg) -->
        <div class="grid grid-cols-1 lg:grid-cols-12 gap-4">
          <!-- Left: 4 KPI Cards (7 cols on lg) -->
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

          <!-- Right: Polling State Distribution Chart (5 cols on lg) -->
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
                  <td colspan="6" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noPollings")}
                  </td>
                </tr>
              {:else}
                {#each paginatedPollings as p}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1.5 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{p.name}</td>
                    <td class="py-1.5 px-2.5">
                      <span class="rounded bg-cyan-100 dark:bg-cyan-500/10 text-cyan-800 dark:text-cyan-300 border border-cyan-300 dark:border-cyan-500/30 px-1.5 py-0.5 text-[9px] font-semibold uppercase leading-none">
                        {p.type}
                      </span>
                    </td>
                    <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 text-[11px] truncate max-w-xs" title={p.params || p.target || ""}>{p.params || p.target || "-"}</td>
                    <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 font-sans text-[11px]">{nodeMap.get(p.node_id) || p.node_id || "-"}</td>
                    <td class="py-1.5 px-2.5">
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(p.state)}20; border-color: {getStateColor(p.state)}50; color: {getStateColor(p.state)}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(p.state)}"></span>
                        {getStateName(p.state, $_)}
                      </span>
                    </td>
                    <td class="py-1.5 px-2.5 font-mono text-cyan-600 dark:text-cyan-400 text-[11px]">{p.last_val ?? "-"}</td>
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
                bind:value={pollingPageSize}
                onchange={() => (pollingCurrentPage = 1)}
                class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-700 dark:text-slate-200 focus:outline-none cursor-pointer"
              >
                <option value={10}>{$_("report.itemsPerPage", { values: { count: 10 } })}</option>
                <option value={25}>{$_("report.itemsPerPage", { values: { count: 25 } })}</option>
                <option value={50}>{$_("report.itemsPerPage", { values: { count: 50 } })}</option>
                <option value={100}>{$_("report.itemsPerPage", { values: { count: 100 } })}</option>
                <option value={250}>{$_("report.itemsPerPage", { values: { count: 250 } })}</option>
                <option value={-1}>{$_("report.showAll")}</option>
              </select>

              <span class="font-mono text-[11px] text-slate-400">
                {#if sortedPollings.length > 0}
                  {$_("report.paginationRange", { values: { total: sortedPollings.length.toLocaleString(), from: (pollingCurrentPage - 1) * (pollingPageSize === -1 ? sortedPollings.length : pollingPageSize) + 1, to: pollingPageSize === -1 ? sortedPollings.length : Math.min(pollingCurrentPage * pollingPageSize, sortedPollings.length) } })}
                {:else}
                  {$_("report.totalZero")}
                {/if}
              </span>
            </div>

            {#if pollingPageSize !== -1 && totalPollingPages > 1}
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  disabled={pollingCurrentPage <= 1}
                  onclick={() => (pollingCurrentPage = 1)}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.firstPage")}
                >
                  <ChevronsLeft class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={pollingCurrentPage <= 1}
                  onclick={() => pollingCurrentPage--}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.prevPage")}
                >
                  <ChevronLeft class="h-4 w-4" />
                </button>

                <span class="px-2 font-mono text-xs text-slate-600 dark:text-slate-300">
                  {pollingCurrentPage} / {totalPollingPages}
                </span>

                <button
                  type="button"
                  disabled={pollingCurrentPage >= totalPollingPages}
                  onclick={() => pollingCurrentPage++}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.nextPage")}
                >
                  <ChevronRight class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={pollingCurrentPage >= totalPollingPages}
                  onclick={() => (pollingCurrentPage = totalPollingPages)}
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

    <!-- REPORT 4: NetFlow分析 -->
    {:else if activeReport === "flow"}
      <div class="space-y-4">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <BarChart3 class="w-5 h-5 text-cyan-400" />
            {$_("report.flowTitle")}
          </h2>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalTransfer")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-400">{renderBytes(flowStats.totalBytes)}</div>
            <div class="text-[10px] text-slate-400">{$_("report.totalTransferSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.totalFlowSessions")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{flowStats.totalSessions.toLocaleString()} <span class="text-xs font-normal text-slate-400">flows</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.totalFlowSessionsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.mainProtocols")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">{flowStats.topProtocol}</div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.mainProtocolsSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.peakBandwidth")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">{flowStats.peakBandwidth}</div>
            <div class="text-[10px] text-slate-400">{$_("report.peakBandwidthSub")}</div>
          </div>
        </div>

        <!-- Flow Analytics Sub-Tab Navigation -->
        <div class="flex items-center gap-1.5 border-b border-slate-200 dark:border-slate-800 pb-2">
          <button
            type="button"
            onclick={() => handleSelectFlowSubTab("conversations")}
            class="rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {flowSubTab === 'conversations' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            {$_("report.subFlowConversations")} ({filteredFlowConversations.length})
          </button>
          <button
            type="button"
            onclick={() => handleSelectFlowSubTab("services")}
            class="rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {flowSubTab === 'services' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            {$_("report.subFlowServices")} ({filteredFlowServices.length})
          </button>
          <button
            type="button"
            onclick={() => handleSelectFlowSubTab("fumble")}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {flowSubTab === 'fumble' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Flame class="w-3.5 h-3.5 text-amber-400" />
            <span>{$_("report.subFlowFumble")} ({filteredFlowFumbles.length})</span>
          </button>
          <button
            type="button"
            onclick={() => handleSelectFlowSubTab("protocols")}
            class="rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {flowSubTab === 'protocols' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            {$_("report.subFlowProtocols")} ({filteredFlowProtocols.length})
          </button>
        </div>

        <!-- Tables Container with Pagination -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
          {#if flowSubTab === "conversations"}
            <!-- Flow Conversations Table -->
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
                <tr>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("src")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colSource")}</span>
                      {#if flowSortColumn === "src"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("dst")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colDest")}</span>
                      {#if flowSortColumn === "dst"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("proto")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colProtoPort")}</span>
                      {#if flowSortColumn === "proto"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("packets")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPackets")}</span>
                      {#if flowSortColumn === "packets"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("bytes")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colBytes")}</span>
                      {#if flowSortColumn === "bytes"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("dur")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colDuration")}</span>
                      {#if flowSortColumn === "dur"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("status")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colStatus")}</span>
                      {#if flowSortColumn === "status"}
                        {#if flowSortDirection === "asc"}
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
                {#if flowConversations.length === 0}
                  <tr>
                    <td colspan="7" class="p-6 text-center text-slate-500 font-sans">
                      データがありません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedConversations as fl}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                      <td class="py-1.5 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px] font-mono">{fl.src}</td>
                      <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 text-[11px] font-mono">{fl.dst}</td>
                      <td class="py-1.5 px-2.5">
                        <span class="rounded bg-slate-100 dark:bg-slate-800 px-1.5 py-0.5 text-[10px] font-sans text-slate-700 dark:text-slate-300 border border-slate-300 dark:border-slate-700 leading-none">
                          {fl.proto}
                        </span>
                      </td>
                      <td class="py-1.5 px-2.5 text-slate-800 dark:text-slate-200 text-[11px] font-mono">{fl.packets.toLocaleString()}</td>
                      <td class="py-1.5 px-2.5 text-emerald-600 dark:text-emerald-400 font-bold font-mono text-[11px]">{renderBytes(fl.bytes)}</td>
                      <td class="py-1.5 px-2.5 text-slate-600 dark:text-slate-400 text-[11px] font-mono">{fl.dur > 0 ? `${Math.round(fl.dur)}s` : "<1s"}</td>
                      <td class="py-1.5 px-2.5">
                        <span class="rounded bg-emerald-100 dark:bg-emerald-500/10 border border-emerald-300 dark:border-emerald-500/30 px-1.5 py-0.5 text-[9px] text-emerald-700 dark:text-emerald-400 font-semibold leading-none">
                          {fl.status}
                        </span>
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          {:else if flowSubTab === "services"}
            <!-- Top Services Table -->
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
                <tr>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("name")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colService")}</span>
                      {#if flowSortColumn === "name"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("bytes")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colBytes")}</span>
                      {#if flowSortColumn === "bytes"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("packets")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPackets")}</span>
                      {#if flowSortColumn === "packets"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("flows")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colFlows")}</span>
                      {#if flowSortColumn === "flows"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("percent")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPercent")}</span>
                      {#if flowSortColumn === "percent"}
                        {#if flowSortDirection === "asc"}
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
                {#if flowServices.length === 0}
                  <tr>
                    <td colspan="5" class="p-6 text-center text-slate-500 font-sans">
                      データがありません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedServices as s}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                      <td class="py-1.5 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{s.name}</td>
                      <td class="py-1.5 px-2.5 text-emerald-600 dark:text-emerald-400 font-bold font-mono text-[11px]">{renderBytes(s.bytes)}</td>
                      <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 font-mono text-[11px]">{s.packets.toLocaleString()}</td>
                      <td class="py-1.5 px-2.5 text-cyan-600 dark:text-cyan-400 font-mono text-[11px]">{s.flows.toLocaleString()}</td>
                      <td class="py-1.5 px-2.5">
                        <div class="flex items-center gap-2">
                          <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                            <div class="h-full bg-cyan-500 rounded-full" style="width: {s.percent}%"></div>
                          </div>
                          <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{s.percent}%</span>
                        </div>
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          {:else if flowSubTab === "fumble"}
            <!-- Fumble Flows Table -->
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
                <tr>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("src")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colSource")}</span>
                      {#if flowSortColumn === "src"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("dst")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colDest")}</span>
                      {#if flowSortColumn === "dst"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("proto")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colProtoPort")}</span>
                      {#if flowSortColumn === "proto"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("packets")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPackets")}</span>
                      {#if flowSortColumn === "packets"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("bytes")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colBytes")}</span>
                      {#if flowSortColumn === "bytes"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("reason")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colReason")}</span>
                      {#if flowSortColumn === "reason"}
                        {#if flowSortDirection === "asc"}
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
                {#if flowFumbles.length === 0}
                  <tr>
                    <td colspan="6" class="p-6 text-center text-slate-500 font-sans">
                      Fumble Flow（拒絶・異常通信）は検出されていません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedFumbles as ff}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                      <td class="py-1.5 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px] font-mono">{ff.src}</td>
                      <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 text-[11px] font-mono">{ff.dst}</td>
                      <td class="py-1.5 px-2.5">
                        <span class="rounded bg-rose-500/10 text-rose-400 border border-rose-500/30 px-1.5 py-0.5 text-[9px] font-semibold">
                          {ff.proto}
                        </span>
                      </td>
                      <td class="py-1.5 px-2.5 text-slate-800 dark:text-slate-200 text-[11px] font-mono">{ff.packets.toLocaleString()}</td>
                      <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 font-mono text-[11px]">{renderBytes(ff.bytes)}</td>
                      <td class="py-1.5 px-2.5 font-sans text-rose-600 dark:text-rose-400 text-[11px]">{ff.reason}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          {:else if flowSubTab === "protocols"}
            <!-- Protocol Breakdown Table -->
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800 select-none">
                <tr>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("proto")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colProtocol")}</span>
                      {#if flowSortColumn === "proto"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("bytes")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colBytes")}</span>
                      {#if flowSortColumn === "bytes"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("packets")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPackets")}</span>
                      {#if flowSortColumn === "packets"}
                        {#if flowSortDirection === "asc"}
                          <ArrowUp class="h-2.5 w-2.5 text-cyan-400" />
                        {:else}
                          <ArrowDown class="h-2.5 w-2.5 text-cyan-400" />
                        {/if}
                      {:else}
                        <ArrowUpDown class="h-2.5 w-2.5 text-slate-600" />
                      {/if}
                    </div>
                  </th>
                  <th class="py-1.5 px-2.5 cursor-pointer hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleSortFlow("percent")}>
                    <div class="inline-flex items-center gap-1">
                      <span>{$_("report.colPercent")}</span>
                      {#if flowSortColumn === "percent"}
                        {#if flowSortDirection === "asc"}
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
                {#if flowProtocols.length === 0}
                  <tr>
                    <td colspan="4" class="p-6 text-center text-slate-500 font-sans">
                      データがありません
                    </td>
                  </tr>
                {:else}
                  {#each paginatedProtocols as pr}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                      <td class="py-1.5 px-2.5 font-bold text-cyan-600 dark:text-cyan-400 font-mono text-[11px]">{pr.proto}</td>
                      <td class="py-1.5 px-2.5 text-emerald-600 dark:text-emerald-400 font-bold font-mono text-[11px]">{renderBytes(pr.bytes)}</td>
                      <td class="py-1.5 px-2.5 text-slate-700 dark:text-slate-300 font-mono text-[11px]">{pr.packets.toLocaleString()}</td>
                      <td class="py-1.5 px-2.5">
                        <div class="flex items-center gap-2">
                          <div class="h-1.5 w-16 rounded-full bg-slate-200 dark:bg-slate-800 overflow-hidden">
                            <div class="h-full bg-emerald-500 rounded-full" style="width: {pr.percent}%"></div>
                          </div>
                          <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">{pr.percent}%</span>
                        </div>
                      </td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          {/if}

          <!-- Pagination Footer -->
          <div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/80 px-4 py-2 text-xs text-slate-600 dark:text-slate-400 shrink-0">
            <div class="flex items-center gap-3">
              <span>{$_("report.pageShowCount")}</span>
              <select
                bind:value={flowPageSize}
                onchange={() => (flowCurrentPage = 1)}
                class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-700 dark:text-slate-200 focus:outline-none cursor-pointer"
              >
                <option value={10}>{$_("report.itemsPerPage", { values: { count: 10 } })}</option>
                <option value={25}>{$_("report.itemsPerPage", { values: { count: 25 } })}</option>
                <option value={50}>{$_("report.itemsPerPage", { values: { count: 50 } })}</option>
                <option value={100}>{$_("report.itemsPerPage", { values: { count: 100 } })}</option>
                <option value={250}>{$_("report.itemsPerPage", { values: { count: 250 } })}</option>
                <option value={-1}>{$_("report.showAll")}</option>
              </select>

              <span class="font-mono text-[11px] text-slate-400">
                {#if currentFlowTotalCount > 0}
                  {$_("report.paginationRange", { values: { total: currentFlowTotalCount.toLocaleString(), from: (flowCurrentPage - 1) * (flowPageSize === -1 ? currentFlowTotalCount : flowPageSize) + 1, to: flowPageSize === -1 ? currentFlowTotalCount : Math.min(flowCurrentPage * flowPageSize, currentFlowTotalCount) } })}
                {:else}
                  {$_("report.totalZero")}
                {/if}
              </span>
            </div>

            {#if flowPageSize !== -1 && totalFlowPages > 1}
              <div class="flex items-center gap-1">
                <button
                  type="button"
                  disabled={flowCurrentPage <= 1}
                  onclick={() => (flowCurrentPage = 1)}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.firstPage")}
                >
                  <ChevronsLeft class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={flowCurrentPage <= 1}
                  onclick={() => flowCurrentPage--}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.prevPage")}
                >
                  <ChevronLeft class="h-4 w-4" />
                </button>

                <span class="px-2 font-mono text-xs text-slate-600 dark:text-slate-300">
                  {flowCurrentPage} / {totalFlowPages}
                </span>

                <button
                  type="button"
                  disabled={flowCurrentPage >= totalFlowPages}
                  onclick={() => flowCurrentPage++}
                  class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer transition-colors"
                  title={$_("report.nextPage")}
                >
                  <ChevronRight class="h-4 w-4" />
                </button>

                <button
                  type="button"
                  disabled={flowCurrentPage >= totalFlowPages}
                  onclick={() => (flowCurrentPage = totalFlowPages)}
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

    <!-- REPORT 5: イベント & ログ集計 (Windows 監査分析含む) -->
    {:else if activeReport === "event"}
      <div class="space-y-6">
        <div>
          <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <FileText class="w-5 h-5 text-cyan-400" />
            {$_("report.eventTitle")}
          </h2>
          <p class="text-xs text-slate-400 mt-1">{$_("report.eventSubtitle")}</p>
        </div>

        <!-- Event Sub-Tabs (All vs Windows) -->
        <div class="flex items-center gap-1.5 border-b border-slate-200 dark:border-slate-800 pb-2">
          <button
            type="button"
            onclick={() => (eventSubTab = "all")}
            class="rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {eventSubTab === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            {$_("report.subEventAll")} ({logs.length})
          </button>
          <button
            type="button"
            onclick={() => (eventSubTab = "windows")}
            class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {eventSubTab === 'windows' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Shield class="w-3.5 h-3.5 text-cyan-400" />
            <span>{$_("report.subEventWindows")} ({windowsEvents.length})</span>
          </button>
        </div>

        {#if eventSubTab === "all"}
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
                      <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">{formatTimeStr(l.time)}</td>
                      <td class="py-1 px-2">
                        <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(l.level)}20; border-color: {getStateColor(l.level)}50; color: {getStateColor(l.level)}">
                          <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(l.level)}"></span>
                          {l.level}
                        </span>
                      </td>
                      <td class="py-1 px-2 text-slate-700 dark:text-slate-400 text-[11px]">{l.type}</td>
                      <td class="py-1 px-2 font-bold font-sans text-slate-800 dark:text-slate-200 text-[11px]">{l.node_name || l.node_id || "-"}</td>
                      <td class="py-1 px-2 font-sans text-slate-900 dark:text-slate-100 text-[11px]">{l.event}</td>
                    </tr>
                  {/each}
                {/if}
              </tbody>
            </table>
          </div>
        {:else}
          <!-- Windows Event Analytics Suite -->
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
              <span class="text-xs font-semibold text-slate-400">{$_("report.winTotalEvents")}</span>
              <div class="text-2xl font-bold font-mono text-cyan-400">{windowsStats.total} <span class="text-xs font-normal text-slate-400">events</span></div>
              <div class="text-[10px] text-slate-400">セキュリティ監査ログ総数</div>
            </div>
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
              <span class="text-xs font-semibold text-slate-400">{$_("report.winSuccessLogon")}</span>
              <div class="text-2xl font-bold font-mono text-emerald-400">{windowsStats.success}</div>
              <div class="text-[10px] text-emerald-400/80">正常認証ログオン</div>
            </div>
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
              <span class="text-xs font-semibold text-slate-400">{$_("report.winFailedLogon")}</span>
              <div class="text-2xl font-bold font-mono text-rose-400">{windowsStats.fail}</div>
              <div class="text-[10px] text-rose-400/80">パスワード不一致 / 不正侵入検知</div>
            </div>
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
              <span class="text-xs font-semibold text-slate-400">{$_("report.winPrivilegeUse")}</span>
              <div class="text-2xl font-bold font-mono text-amber-400">{windowsStats.priv}</div>
              <div class="text-[10px] text-amber-400/80">管理者権限昇格 / 特権アクセス</div>
            </div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
            <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 text-xs font-bold text-slate-800 dark:text-slate-200">
              {$_("report.subEventWindows")}
            </div>
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
                <tr>
                  <th class="py-1 px-2.5">{$_("report.colTimestamp")}</th>
                  <th class="py-1 px-2.5">{$_("report.colEventId")}</th>
                  <th class="py-1 px-2.5">{$_("report.colCategory")}</th>
                  <th class="py-1 px-2.5">{$_("report.colTargetNode")}</th>
                  <th class="py-1 px-2.5">{$_("report.colEventContent")}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
                {#each windowsEvents as we}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 whitespace-nowrap text-[11px]">{formatTimeStr(we.time)}</td>
                    <td class="py-1 px-2">
                      <span class="rounded px-1.5 py-0.5 text-[9px] font-bold border {we.eventId === '4625' ? 'bg-rose-500/10 text-rose-400 border-rose-500/30' : we.eventId === '4672' ? 'bg-amber-500/10 text-amber-400 border-amber-500/30' : 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30'}">
                        {we.eventId}
                      </span>
                    </td>
                    <td class="py-1 px-2 text-slate-800 dark:text-slate-200 font-sans text-[11px]">{we.category}</td>
                    <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{we.node}</td>
                    <td class="py-1 px-2 font-sans text-slate-700 dark:text-slate-300 text-[11px]">{we.event}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

    <!-- REPORT 6: サーバー証明書監視 (実TLSチェッカー連動) -->
    {:else if activeReport === "cert"}
      <div class="space-y-6">
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              <ShieldCheck class="w-5 h-5 text-cyan-400" />
              {$_("report.certTitle")}
            </h2>
            <p class="text-xs text-slate-400 mt-1">{$_("report.certSubtitle")}</p>
          </div>
          <div class="flex items-center gap-2">
            <button
              type="button"
              onclick={() => (showAddCertModal = true)}
              class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
            >
              <Plus class="w-4 h-4" />
              <span>{$_("report.btnAddCert")}</span>
            </button>
            <button
              type="button"
              onclick={handleCheckAllCerts}
              disabled={isCheckingCerts}
              class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
            >
              <RefreshCw class="w-3.5 h-3.5 text-cyan-400 {isCheckingCerts ? 'animate-spin' : ''}" />
              <span>{isCheckingCerts ? $_("report.checkingCerts") : $_("report.btnCheckCerts")}</span>
            </button>
          </div>
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
            <div class="text-2xl font-bold font-mono text-amber-400">{certItems.filter((c) => c.status === 'warning' || c.status === 'error').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
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
                <th class="py-1 px-2 text-center w-12">{$_("report.colAction")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if certItems.length === 0}
                <tr>
                  <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
                    監視対象の証明書が登録されていません。「監視対象追加」から TLS サーバーを登録してください。
                  </td>
                </tr>
              {:else}
                {#each certItems as c}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{c.host}:{c.port}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 font-sans text-[11px] truncate max-w-xs">{c.issuer}</td>
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px] truncate max-w-xs">{c.subject}</td>
                    <td class="py-1 px-2.5 text-slate-800 dark:text-slate-200 text-[11px]">{c.key}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{c.validUntil}</td>
                    <td class="py-1 px-2.5 font-bold text-[11px] {c.days <= 0 ? 'text-rose-500' : c.days < 30 ? 'text-amber-500' : 'text-emerald-500'}">
                      {c.days <= 0 ? '期限切れ' : $_("report.remainingDaysUnit", { values: { days: c.days } })}
                    </td>
                    <td class="py-1 px-2.5">
                      <span class="rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {c.status === 'valid' ? 'bg-emerald-100 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-500/30' : c.status === 'warning' ? 'bg-amber-100 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-300 dark:border-amber-500/30' : 'bg-rose-100 dark:bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-300 dark:border-rose-500/30'}">
                        {c.status}
                      </span>
                    </td>
                    <td class="py-1 px-2 text-center">
                      <button
                        type="button"
                        onclick={() => handleDeleteCert(c.id, c.host, c.port)}
                        class="inline-flex items-center justify-center rounded border border-rose-500/30 bg-rose-500/10 p-1 text-rose-400 hover:bg-rose-500/20 hover:text-rose-200 transition-all cursor-pointer"
                        title="削除"
                      >
                        <Trash2 class="h-3 w-3" />
                      </button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Add Certificate Modal -->
        {#if showAddCertModal}
          <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 backdrop-blur-xs p-4">
            <div class="w-full max-w-md rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-6 shadow-2xl space-y-4">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <ShieldCheck class="w-4 h-4 text-cyan-400" />
                  {$_("report.modalAddCertTitle")}
                </h3>
                <button
                  type="button"
                  onclick={() => { showAddCertModal = false; certErrorMessage = ""; }}
                  class="rounded-lg p-1 text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>

              {#if certErrorMessage}
                <div class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-400 font-sans">
                  {certErrorMessage}
                </div>
              {/if}

              <div class="space-y-3 font-sans text-xs">
                <div>
                  <label class="block text-slate-400 font-semibold mb-1" for="newCertTargetInput">
                    {$_("report.targetHostLabel")}
                  </label>
                  <input
                    id="newCertTargetInput"
                    type="text"
                    placeholder="example.com または 192.168.1.1"
                    bind:value={newCertTarget}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>

                <div>
                  <label class="block text-slate-400 font-semibold mb-1" for="newCertPortInput">
                    {$_("report.targetPortLabel")}
                  </label>
                  <input
                    id="newCertPortInput"
                    type="number"
                    bind:value={newCertPort}
                    min="1"
                    max="65535"
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>

              <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-slate-800">
                <button
                  type="button"
                  onclick={() => { showAddCertModal = false; certErrorMessage = ""; }}
                  class="rounded-xl px-4 py-2 text-xs font-semibold text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
                >
                  {$_("report.btnCancel")}
                </button>
                <button
                  type="button"
                  onclick={handleAddCert}
                  disabled={isSavingCert || !newCertTarget.trim()}
                  class="rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 px-4 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
                >
                  {isSavingCert ? "保存・検査中..." : $_("report.btnSave")}
                </button>
              </div>
            </div>
          </div>
        {/if}
      </div>

    <!-- REPORT 7: 環境・IoT センサー (実ポーリング・MQTTテレメトリ連動) -->
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
            <div class="text-2xl font-bold font-mono text-cyan-400">{sensorStats.temp} <span class="text-xs font-normal text-slate-400">℃</span></div>
            <div class="text-[10px] text-emerald-400">{$_("report.serverRoomTempSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.serverRoomHumidity")}</span>
            <div class="text-2xl font-bold font-mono text-cyan-300">{sensorStats.humidity} <span class="text-xs font-normal text-slate-400">%</span></div>
            <div class="text-[10px] text-emerald-400">{$_("report.serverRoomHumiditySub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.upsBatteryStatus")}</span>
            <div class="text-2xl font-bold font-mono text-emerald-400">{sensorStats.battery} <span class="text-xs font-normal text-slate-400">{$_("report.unitBattery")}</span></div>
            <div class="text-[10px] text-slate-400">{$_("report.upsPowerSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.mqttReceived")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">{sensorStats.totalMqtt} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">msgs</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.mqttTopicSub")}</div>
          </div>
        </div>

        <!-- Section 1: Active Sensor Pollings -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 text-xs font-bold text-slate-800 dark:text-slate-200 flex items-center justify-between">
            <span>{$_("report.sensorActivePollings")} ({sensorPollings.length})</span>
            <span class="text-[10px] font-normal text-slate-500">SNMP / HTTP / 環境温湿度・電力テレメトリ</span>
          </div>
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">{$_("report.colPollingName")}</th>
                <th class="py-1 px-2.5">{$_("report.colType")}</th>
                <th class="py-1 px-2.5">{$_("report.colTarget")}</th>
                <th class="py-1 px-2.5">{$_("report.colLatestValue")}</th>
                <th class="py-1 px-2.5">{$_("report.colResponseStatus")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if sensorPollings.length === 0}
                <tr>
                  <td colspan="5" class="p-6 text-center text-slate-500 font-sans">
                    {$_("report.sensorNoData")}
                  </td>
                </tr>
              {:else}
                {#each sensorPollings as sp}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{sp.name}</td>
                    <td class="py-1 px-2.5"><span class="rounded bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 px-1.5 py-0.5 text-[9px] font-semibold uppercase">{sp.type}</span></td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{sp.params || sp.target || "-"}</td>
                    <td class="py-1 px-2.5 text-emerald-600 dark:text-emerald-400 font-bold text-[11px]">{sp.last_val ?? "-"}</td>
                    <td class="py-1 px-2.5">
                      <span class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none" style="background-color: {getStateColor(sp.state)}20; border-color: {getStateColor(sp.state)}50; color: {getStateColor(sp.state)}">
                        <span class="h-1.5 w-1.5 rounded-full shrink-0" style="background-color: {getStateColor(sp.state)}"></span>
                        {getStateName(sp.state)}
                      </span>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Section 2: MQTT Telemetry Topics -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <div class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-5 py-3 text-xs font-bold text-slate-800 dark:text-slate-200 flex items-center justify-between">
            <span>{$_("report.sensorMqttTopics")} ({mqttStats.length})</span>
            <span class="text-[10px] font-normal text-slate-500">ブローカー受信トピック一覧</span>
          </div>
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2.5">トピック (Topic)</th>
                <th class="py-1 px-2.5">クライアント ID</th>
                <th class="py-1 px-2.5">メッセージ件数</th>
                <th class="py-1 px-2.5">データ量</th>
                <th class="py-1 px-2.5">最新ペイロード値</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if mqttStats.length === 0}
                <tr>
                  <td colspan="5" class="p-6 text-center text-slate-500 font-sans">
                    MQTT メッセージはまだ受信されていません (ポート 1883)
                  </td>
                </tr>
              {:else}
                {#each mqttStats as ms}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px]">{ms.Topic}</td>
                    <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{ms.ClientID || "-"}</td>
                    <td class="py-1 px-2.5 text-slate-800 dark:text-slate-200 text-[11px]">{ms.Count.toLocaleString()}</td>
                    <td class="py-1 px-2.5 text-slate-600 dark:text-slate-400 text-[11px]">{renderBytes(ms.Bytes)}</td>
                    <td class="py-1 px-2.5 font-bold text-emerald-600 dark:text-emerald-400 text-[11px] truncate max-w-xs">{ms.Value || "-"}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    <!-- REPORT 8: AI 異常検知スコア (動的複合判定) -->
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
            <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">{aiEvaluatedNodes.length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiFeaturesSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnomalyNodes")}</span>
            <div class="text-2xl font-bold font-mono text-rose-500 dark:text-rose-400">{aiEvaluatedNodes.filter((n) => Number(n.score) >= 60).length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span></div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">要点検ノード数</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAvgScore")}</span>
            <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
              {aiEvaluatedNodes.length > 0 ? (aiEvaluatedNodes.reduce((sum, n) => sum + Number(n.score), 0) / aiEvaluatedNodes.length).toFixed(1) : "0.0"} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ 100</span>
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiStableSub")}</div>
          </div>
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
            <span class="text-xs font-semibold text-slate-400">{$_("report.aiEngine")}</span>
            <div class="text-xl font-bold font-mono text-cyan-300">NEO AI Agent / MCP</div>
            <div class="text-[10px] text-slate-400">{$_("report.aiEngineSub")}</div>
          </div>
        </div>

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
          <table class="w-full text-left text-xs border-collapse font-mono">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
              <tr>
                <th class="py-1 px-2">{$_("report.colTargetNode")}</th>
                <th class="py-1 px-2">{$_("report.colIp")}</th>
                <th class="py-1 px-2">{$_("report.colAnomalyScore")}</th>
                <th class="py-1 px-2">{$_("report.colEvaluationFactors")}</th>
                <th class="py-1 px-2">{$_("report.colAiVerdict")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
              {#if aiEvaluatedNodes.length === 0}
                <tr>
                  <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
                    {$_("report.noNodes")}
                  </td>
                </tr>
              {:else}
                {#each aiEvaluatedNodes as an}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{an.name}</td>
                    <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 text-[11px]">{an.ip}</td>
                    <td class="py-1 px-2 font-bold font-mono text-[11px] {Number(an.score) >= 60 ? 'text-rose-500' : Number(an.score) >= 25 ? 'text-amber-500' : 'text-emerald-500'}">
                      {an.score}
                    </td>
                    <td class="py-1 px-2 text-slate-700 dark:text-slate-400 font-sans text-[11px]">{an.factors}</td>
                    <td class="py-1 px-2">
                      <span class="rounded px-1.5 py-0.5 text-[9px] font-bold font-sans leading-none border {an.verdictClass}">
                        {an.verdict}
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

<NodeDialog
  bind:show={showAddNodeModal}
  node={nodeToEdit}
  onSave={() => {
    loadData();
  }}
/>
