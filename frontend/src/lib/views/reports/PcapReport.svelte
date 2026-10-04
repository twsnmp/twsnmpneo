<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Activity,
    Network,
    Globe,
    Shield,
    Lock,
    Trash2,
    ChevronDown,
    ChevronRight,
    MapPin,
    Map as MapIcon,
    BarChart2,
    Eye,
    X,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import { formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    deleteLogReportItem,
    type EtherTypeEnt,
    type DNSQEnt,
    type RADIUSFlowEnt,
    type TLSFlowEnt,
    type PollingEnt,
    type NodeEnt,
  } from "../../api";

  let {
    searchQuery = "",
    nodes = [],
    onRefresh = () => {},
    loading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  type TabType = "ether" | "dns" | "radius" | "tls";
  let activeTab = $state<TabType>("ether");

  let ethers = $state<EtherTypeEnt[]>([]);
  let dnsList = $state<DNSQEnt[]>([]);
  let radiusFlows = $state<RADIUSFlowEnt[]>([]);
  let tlsFlows = $state<TLSFlowEnt[]>([]);
  let internalLoading = $state(false);
  let expandedId = $state<string | null>(null);
  let showPollingModal = $state(false);
  let editingPolling = $state<PollingEnt | null>(null);

  // Detail Modal State
  let infoModalOpen = $state(false);
  let selectedItem = $state<any>(null);

  // Full Chart Modal State
  let chartModalOpen = $state(false);
  let chartCategory = $state<string>("default");
  let graphLayout = $state<"force" | "circular">("force");
  let countryMode = $state<"server" | "client">("server");
  let modalChartContainer = $state<HTMLDivElement | null>(null);
  let modalChartInstance: echarts.ECharts | null = null;

  // Inline KPI Right Chart State (like DeviceReport)
  let inlineChartElem = $state<HTMLDivElement | null>(null);
  let inlineChartInstance: echarts.ECharts | null = null;

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // DNS Type definitions and resolver
  const dnsTypeMap: Record<string, string> = {
    "1": "A",
    "28": "AAAA",
    "12": "PTR",
    "2": "NS",
    "15": "MX",
    "16": "TXT",
    "5": "CNAME",
    "6": "SOA",
    "13": "HINFO",
    "65": "HTTPS",
    "48": "DNSKEY",
    "43": "DS",
    "64": "SVCB",
    "255": "ANY",
  };

  function getDNSTypeLabel(type: string): string {
    if (!type) return "-";
    if (dnsTypeMap[type]) {
      return `${dnsTypeMap[type]} (${type})`;
    }
    return type;
  }

  // GeoIP Location parser
  function parseLocInfo(locStr?: string): { Country: string; LatLong: string; LocInfo: string } {
    if (!locStr) return { Country: "", LatLong: "", LocInfo: "" };
    const parts = locStr.split(",");
    if (parts.length < 3) {
      return { Country: locStr, LatLong: "", LocInfo: locStr === "LOCAL" ? $_("report.pcapGraphClient") || "LOCAL" : locStr };
    }
    if (parts[0] === "LOCAL") {
      return { Country: "LOCAL", LatLong: "", LocInfo: "LOCAL" };
    }
    const country = parts[0];
    const latLong = `${parts[1]},${parts[2]}`;
    const city = parts.length > 3 && parts[3] ? parts[3] : "";
    const locInfo = city ? `${city} (${country})` : country;
    return { Country: country, LatLong: latLong, LocInfo: locInfo };
  }

  function getScoreBandIndex(score: number): number {
    if (score < 33) return 0; // <=32
    if (score < 42) return 1; // 33-41
    if (score < 51) return 2; // 42-50
    if (score < 67) return 3; // 51-66
    return 4;                 // >=67
  }

  function getScoreColorHex(score: number): string {
    if (score >= 67) return "#0284c7"; // Blue
    if (score >= 51) return "#06b6d4"; // Cyan
    if (score >= 42) return "#eab308"; // Yellow
    if (score >= 33) return "#f97316"; // Orange
    return "#ef4444";                 // Red
  }

  const loadAll = async () => {
    internalLoading = true;
    try {
      const [e, d, r, t] = await Promise.all([
        fetchLogReport<EtherTypeEnt>("etherType"),
        fetchLogReport<DNSQEnt>("dnsq"),
        fetchLogReport<RADIUSFlowEnt>("radiusFlow"),
        fetchLogReport<TLSFlowEnt>("tlsFlow"),
      ]);
      ethers = e;
      dnsList = d;
      radiusFlows = r;
      tlsFlows = t;
      await tick();
      renderInlineChart();
    } catch (err) {
      console.error("Failed to load Pcap reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      ether: { kind: "etherType", name: $_("report.pcapEtherFrames") },
      dns: { kind: "dnsq", name: $_("report.pcapDnsQueries") },
      radius: { kind: "radiusFlow", name: $_("report.pcapRadiusFlows") },
      tls: { kind: "tlsFlow", name: $_("report.pcapTlsFlows") },
    };
    const target = tabKindMap[activeTab];
    if (!confirm($_("report.confirmClearItem", { values: { name: target.name } }) || `${target.name}データを全消去しますか？`)) return;
    try {
      await resetLogReport(target.kind);
      await loadAll();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  }

  const handleDeleteItem = async (e: Event, item: any) => {
    e.stopPropagation();
    if (!confirm($_("report.pcapConfirmDelete") || "選択したエントリーを削除しますか？")) return;
    const kindMap: Record<TabType, string> = {
      ether: "etherType",
      dns: "dnsq",
      radius: "radiusFlow",
      tls: "tlsFlow",
    };
    try {
      await deleteLogReportItem(kindMap[activeTab], item.ID);
      await loadAll();
      onRefresh();
      if (infoModalOpen && selectedItem?.ID === item.ID) {
        infoModalOpen = false;
      }
    } catch (err: any) {
      alert($_("report.pcapAlertDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  const openInfoModal = (e: Event, item: any) => {
    e.stopPropagation();
    selectedItem = item;
    infoModalOpen = true;
  };

  // Render inline Mini Chart on the right of KPI cards
  const renderInlineChart = () => {
    if (!inlineChartElem) return;
    if (inlineChartInstance) {
      inlineChartInstance.dispose();
    }
    const isDark = isDarkMode();
    inlineChartInstance = echarts.init(inlineChartElem, isDark ? "dark" : undefined);

    let yData: string[] = [];
    let seriesData: number[] = [];
    let chartColorStart = "#38bdf8";
    let chartColorEnd = "#0284c7";

    if (activeTab === "ether") {
      const typeMap = new Map<string, number>();
      for (const e of ethers) {
        const key = e.Name || e.Type;
        typeMap.set(key, (typeMap.get(key) || 0) + (e.Count || 0));
      }
      const topEntries = Array.from(typeMap.entries()).sort((a, b) => b[1] - a[1]).slice(0, 5).reverse();
      yData = topEntries.map(([name]) => name);
      seriesData = topEntries.map(([, count]) => count);
      chartColorStart = "#38bdf8";
      chartColorEnd = "#0284c7";
    } else if (activeTab === "dns") {
      const dnsTypeCount = new Map<string, number>();
      for (const d of dnsList) {
        const key = getDNSTypeLabel(d.Type);
        dnsTypeCount.set(key, (dnsTypeCount.get(key) || 0) + (d.Count || 0));
      }
      const topEntries = Array.from(dnsTypeCount.entries()).sort((a, b) => b[1] - a[1]).slice(0, 5).reverse();
      yData = topEntries.map(([name]) => name);
      seriesData = topEntries.map(([, count]) => count);
      chartColorStart = "#34d399";
      chartColorEnd = "#059669";
    } else if (activeTab === "radius") {
      const serverMap = new Map<string, number>();
      for (const r of radiusFlows) {
        const key = r.ServerName || r.Server;
        serverMap.set(key, (serverMap.get(key) || 0) + (r.Count || 0));
      }
      const topEntries = Array.from(serverMap.entries()).sort((a, b) => b[1] - a[1]).slice(0, 5).reverse();
      yData = topEntries.map(([name]) => name);
      seriesData = topEntries.map(([, count]) => count);
      chartColorStart = "#c084fc";
      chartColorEnd = "#9333ea";
    } else if (activeTab === "tls") {
      const verMap = new Map<string, number>();
      for (const t of tlsFlows) {
        const key = t.Version || "Unknown";
        verMap.set(key, (verMap.get(key) || 0) + (t.Count || 1));
      }
      const topEntries = Array.from(verMap.entries()).sort((a, b) => b[1] - a[1]).slice(0, 5).reverse();
      yData = topEntries.map(([name]) => name);
      seriesData = topEntries.map(([, count]) => count);
      chartColorStart = "#fbbf24";
      chartColorEnd = "#d97706";
    }

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        formatter: (params: any) => {
          if (!params || !params.length) return "";
          const item = params[0];
          return `<b>${item.name}</b>: ${item.value?.toLocaleString()}`;
        },
      },
      grid: {
        top: 6,
        bottom: 18,
        left: 95,
        right: 28,
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
          fontSize: 9,
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
            return val.length > 11 ? val.slice(0, 10) + "…" : val;
          },
        },
      },
      series: [
        {
          name: "Count",
          type: "bar",
          data: seriesData,
          itemStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
              { offset: 0, color: chartColorStart },
              { offset: 1, color: chartColorEnd },
            ]),
            borderRadius: [0, 4, 4, 0],
          },
          label: {
            show: true,
            position: "right",
            fontSize: 9,
            fontFamily: "monospace",
            color: isDark ? "#94a3b8" : "#64748b",
            formatter: (p: any) => p.value?.toLocaleString(),
          },
        },
      ],
    };

    inlineChartInstance.setOption(option);
  };

  onMount(() => {
    loadAll();
    const handleResize = () => {
      inlineChartInstance?.resize();
      modalChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      inlineChartInstance?.dispose();
      modalChartInstance?.dispose();
    };
  });

  const handleTabChange = async (tab: TabType) => {
    activeTab = tab;
    expandedId = null;
    currentPage = 1;
    sortColumn = "LastTime";
    sortDirection = "desc";
    if (chartModalOpen) {
      chartModalOpen = false;
    }
    await tick();
    renderInlineChart();
  };

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const currentList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (activeTab === "ether") {
      let res = ethers;
      if (q) {
        res = res.filter(
          (e) =>
            e.Name?.toLowerCase().includes(q) ||
            e.Type?.toLowerCase().includes(q) ||
            e.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "dns") {
      let res = dnsList;
      if (q) {
        res = res.filter(
          (d) =>
            d.Name?.toLowerCase().includes(q) ||
            d.Server?.toLowerCase().includes(q) ||
            d.Type?.toLowerCase().includes(q) ||
            d.LastClient?.toLowerCase().includes(q) ||
            d.LastMAC?.toLowerCase().includes(q) ||
            d.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "radius") {
      let res = radiusFlows;
      if (q) {
        res = res.filter(
          (r) =>
            r.Client?.toLowerCase().includes(q) ||
            r.ClientName?.toLowerCase().includes(q) ||
            r.Server?.toLowerCase().includes(q) ||
            r.ServerName?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (sortColumn === "Rate") {
          const totalA = (a.Accept || 0) + (a.Reject || 0);
          const totalB = (b.Accept || 0) + (b.Reject || 0);
          valA = totalA ? (100 * (a.Accept || 0)) / totalA : 0;
          valB = totalB ? (100 * (b.Accept || 0)) / totalB : 0;
        }
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "tls") {
      let res = tlsFlows;
      if (q) {
        res = res.filter(
          (t) =>
            t.Client?.toLowerCase().includes(q) ||
            t.ClientName?.toLowerCase().includes(q) ||
            t.Server?.toLowerCase().includes(q) ||
            t.ServerName?.toLowerCase().includes(q) ||
            t.Service?.toLowerCase().includes(q) ||
            t.Version?.toLowerCase().includes(q) ||
            t.Cipher?.toLowerCase().includes(q) ||
            t.ServerLoc?.toLowerCase().includes(q) ||
            t.ClientLoc?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }
    return [];
  });

  const paginatedList = $derived.by(() => {
    if (pageSize === -1) return currentList;
    const start = (currentPage - 1) * pageSize;
    return currentList.slice(start, start + pageSize);
  });

  // KPI Calculations
  const kpis = $derived.by(() => {
    if (activeTab === "ether") {
      const totalPackets = ethers.reduce((sum, e) => sum + (e.Count || 0), 0);
      const uniqueTypes = new Set(ethers.map((e) => e.Type)).size;
      const uniqueHosts = new Set(ethers.map((e) => e.Host)).size;
      return {
        card1Title: $_("report.pcapKpiTotalPackets"),
        card1Val: totalPackets.toLocaleString(),
        card1Sub: `${ethers.length} エントリー`,
        card2Title: $_("report.pcapKpiProtoTypes"),
        card2Val: uniqueTypes.toLocaleString(),
        card2Sub: "LLC / IPv4 / ARP / IPv6 等",
        card3Title: $_("report.pcapKpiCaptureHosts"),
        card3Val: uniqueHosts.toLocaleString(),
        card3Sub: "キャプチャノード",
      };
    } else if (activeTab === "dns") {
      const totalQueries = dnsList.reduce((sum, d) => sum + (d.Count || 0), 0);
      const uniqueNames = new Set(dnsList.map((d) => d.Name)).size;
      const changedCount = dnsList.filter((d) => (d.Change || 0) > 0).length;
      return {
        card1Title: $_("report.pcapKpiTotalQueries"),
        card1Val: totalQueries.toLocaleString(),
        card1Sub: `${dnsList.length} レコード`,
        card2Title: $_("report.pcapKpiUniqueDomains"),
        card2Val: uniqueNames.toLocaleString(),
        card2Sub: "解決ドメイン",
        card3Title: $_("report.pcapKpiChangedDns"),
        card3Val: changedCount.toLocaleString(),
        card3Sub: "IPアドレス等変更検知",
      };
    } else if (activeTab === "radius") {
      const totalFlows = radiusFlows.length;
      const totalAccept = radiusFlows.reduce((sum, r) => sum + (r.Accept || 0), 0);
      const totalReject = radiusFlows.reduce((sum, r) => sum + (r.Reject || 0), 0);
      const allAuth = totalAccept + totalReject;
      const acceptRate = allAuth ? ((100 * totalAccept) / allAuth).toFixed(1) + "%" : "100.0%";
      return {
        card1Title: $_("report.pcapKpiTotalRadius"),
        card1Val: totalFlows.toLocaleString(),
        card1Sub: "RADIUS セッションペア",
        card2Title: $_("report.pcapKpiAcceptRate"),
        card2Val: acceptRate,
        card2Sub: `Accept: ${totalAccept.toLocaleString()}`,
        card3Title: $_("report.pcapKpiRejectCount"),
        card3Val: totalReject.toLocaleString(),
        card3Sub: "拒絶・認証エラー",
      };
    } else {
      const totalFlows = tlsFlows.length;
      const modernTls = tlsFlows.filter((t) => t.Version?.includes("1.3") || t.Version?.includes("1.2")).length;
      const modernRate = totalFlows ? ((100 * modernTls) / totalFlows).toFixed(1) + "%" : "100.0%";
      const lowScore = tlsFlows.filter((t) => (t.Score || 50) < 50).length;
      return {
        card1Title: $_("report.pcapKpiTotalTls"),
        card1Val: totalFlows.toLocaleString(),
        card1Sub: "暗号化セッションフロー",
        card2Title: $_("report.pcapKpiModernTls"),
        card2Val: modernRate,
        card2Sub: `TLS 1.2/1.3: ${modernTls} 件`,
        card3Title: $_("report.pcapKpiLowScoreTls"),
        card3Val: lowScore.toLocaleString(),
        card3Sub: "古い暗号・未解決FQDN等",
      };
    }
  });

  export function handleAddNewPolling(): void {
    editingPolling = {
      id: "",
      name: "twpcap",
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "syslog",
      mode: "twpcap",
      params: "",
      filter: "",
      extractor: "",
      script: "",
      level: "off",
      poll_int: 300,
      timeout: 5,
      retry: 1,
      log_mode: 0,
      next_time: 0,
      last_time: 0,
      result: {},
      state: "unknown",
      fail_action: "",
      repair_action: "",
      ai_mode: "default",
      vector_cols: "",
      mqtt_url: "",
      mqtt_topic: "",
      mqtt_cols: "",
      fail_time: 0,
    } as PollingEnt;
    showPollingModal = true;
  }

  export function exportCSV(): void {
    let header = "";
    let rows = "";
    if (activeTab === "ether") {
      header = "Host,Type,Name,Count,FirstTime,LastTime\n";
      rows = (currentList as EtherTypeEnt[])
        .map((e) => `"${e.Host}","${e.Type}","${e.Name}","${e.Count}","${formatTimeStr(e.FirstTime)}","${formatTimeStr(e.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "dns") {
      header = "Host,Type,Server,Name,Count,Change,LastClient,LastMAC,FirstTime,LastTime\n";
      rows = (currentList as DNSQEnt[])
        .map((d) => `"${d.Host}","${d.Type}","${d.Server}","${d.Name}","${d.Count}","${d.Change}","${d.LastClient}","${d.LastMAC || ""}","${formatTimeStr(d.FirstTime)}","${formatTimeStr(d.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "radius") {
      header = "Client,ClientName,Server,ServerName,Accept,Reject,Request,Challenge,Count,Rate,Penalty,Score,FirstTime,LastTime\n";
      rows = (currentList as RADIUSFlowEnt[])
        .map((r) => {
          const tot = (r.Accept || 0) + (r.Reject || 0);
          const rate = tot ? ((100 * (r.Accept || 0)) / tot).toFixed(1) + "%" : "0.0%";
          return `"${r.Client}","${r.ClientName || ""}","${r.Server}","${r.ServerName || ""}","${r.Accept}","${r.Reject}","${r.Request}","${r.Challenge}","${r.Count}","${rate}","${r.Penalty}","${r.Score?.toFixed(1) || ""}","${formatTimeStr(r.FirstTime)}","${formatTimeStr(r.LastTime)}"`;
        })
        .join("\n");
    } else if (activeTab === "tls") {
      header = "Client,ClientName,ClientLoc,Server,ServerName,ServerLoc,Service,Version,Cipher,Count,Penalty,Score,FirstTime,LastTime\n";
      rows = (currentList as TLSFlowEnt[])
        .map((t) => `"${t.Client}","${t.ClientName || ""}","${t.ClientLoc || ""}","${t.Server}","${t.ServerName || ""}","${t.ServerLoc || ""}","${t.Service}","${t.Version}","${t.Cipher}","${t.Count}","${t.Penalty}","${t.Score?.toFixed(1) || ""}","${formatTimeStr(t.FirstTime)}","${formatTimeStr(t.LastTime)}"`)
        .join("\n");
    }

    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_pcap_${activeTab}_${Date.now()}.csv`;
    link.click();
  }

  // --- ECharts Analytics Modal ---
  const openChartModal = async (cat?: string) => {
    if (cat) chartCategory = cat;
    else {
      if (activeTab === "ether") chartCategory = "ether";
      else if (activeTab === "dns") chartCategory = "dns_name";
      else if (activeTab === "radius") chartCategory = "radius_flows";
      else if (activeTab === "tls") chartCategory = "tls_flows";
    }
    chartModalOpen = true;
    await tick();
    renderModalAnalyticsChart();
  };

  const renderModalAnalyticsChart = () => {
    if (!modalChartContainer) return;
    if (!modalChartInstance) {
      modalChartInstance = echarts.init(modalChartContainer);
    }
    const dark = isDarkMode();
    const textColor = dark ? "#94a3b8" : "#64748b";
    const lineBorder = dark ? "#334155" : "#e2e8f0";

    let option: echarts.EChartsOption = {};

    if (chartCategory === "ether") {
      const typeMap = new Map<string, number>();
      for (const e of ethers) {
        typeMap.set(e.Name || e.Type, (typeMap.get(e.Name || e.Type) || 0) + (e.Count || 0));
      }
      const data = [...typeMap.entries()].sort((a, b) => b[1] - a[1]).slice(0, 15);
      option = {
        title: { text: $_("report.pcapChartEtherType") || "Ethernet Types", textStyle: { color: dark ? "#f8fafc" : "#0f172a", fontSize: 14 } },
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        legend: { type: "scroll", orient: "vertical", right: 10, top: 40, bottom: 20, textStyle: { color: textColor } },
        series: [
          {
            type: "pie",
            radius: ["40%", "70%"],
            center: ["40%", "50%"],
            avoidLabelOverlap: true,
            itemStyle: { borderRadius: 6, borderColor: dark ? "#0f172a" : "#fff", borderWidth: 2 },
            data: data.map(([name, value]) => ({ name, value })),
          },
        ],
      };
    } else if (chartCategory === "dns_name" || chartCategory === "dns_type" || chartCategory === "dns_server") {
      const field = chartCategory === "dns_name" ? "Name" : chartCategory === "dns_type" ? "Type" : "Server";
      const countMap = new Map<string, { change: number; fixed: number }>();
      for (const d of dnsList) {
        const k = (d as any)[field] || "Unknown";
        if (!countMap.has(k)) countMap.set(k, { change: 0, fixed: 0 });
        const entry = countMap.get(k)!;
        entry.change += d.Change || 0;
        entry.fixed += Math.max(0, (d.Count || 0) - (d.Change || 0));
      }
      const sorted = [...countMap.entries()].sort((a, b) => (b[1].change + b[1].fixed) - (a[1].change + a[1].fixed)).slice(0, 20).reverse();
      option = {
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        legend: { data: [$_("report.pcapScoreChanged") || "Changed", $_("report.pcapScoreFixed") || "Fixed"], textStyle: { color: textColor } },
        grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
        xAxis: { type: "value", axisLabel: { color: textColor }, splitLine: { lineStyle: { color: lineBorder } } },
        yAxis: {
          type: "category",
          data: sorted.map(([k]) => field === "Type" ? getDNSTypeLabel(k) : k),
          axisLabel: { color: textColor, fontSize: 11 },
        },
        series: [
          {
            name: $_("report.pcapScoreChanged") || "Changed",
            type: "bar",
            stack: "total",
            itemStyle: { color: "#ef4444" },
            data: sorted.map(([, v]) => v.change),
          },
          {
            name: $_("report.pcapScoreFixed") || "Fixed",
            type: "bar",
            stack: "total",
            itemStyle: { color: "#0ea5e9" },
            data: sorted.map(([, v]) => v.fixed),
          },
        ],
      };
    } else if (chartCategory === "dns_flows") {
      const nodesMap = new Map<string, any>();
      const links: any[] = [];
      for (const d of dnsList) {
        const c = d.LastClient || "Client";
        const s = d.Server || "DNS Server";
        if (!nodesMap.has(c)) {
          nodesMap.set(c, { name: c, category: 1, symbolSize: 14, itemStyle: { color: "#0ea5e9" } });
        }
        if (!nodesMap.has(s)) {
          nodesMap.set(s, { name: s, category: 0, symbolSize: 20, itemStyle: { color: "#10b981" } });
        }
        links.push({
          source: c,
          target: s,
          value: d.Count,
          lineStyle: { color: "#64748b", curveness: 0.1 },
        });
      }
      option = {
        tooltip: { trigger: "item", formatter: "{b}" },
        legend: { data: [$_("report.pcapGraphServer") || "Server", $_("report.pcapGraphClient") || "Client"], textStyle: { color: textColor } },
        series: [
          {
            type: "graph",
            layout: graphLayout,
            categories: [{ name: $_("report.pcapGraphServer") || "Server" }, { name: $_("report.pcapGraphClient") || "Client" }],
            data: [...nodesMap.values()],
            links,
            roam: true,
            label: { show: true, position: "right", color: textColor, fontSize: 10 },
            force: { repulsion: 120, edgeLength: 80 },
          },
        ],
      };
    } else if (chartCategory === "radius_flows") {
      const nodesMap = new Map<string, any>();
      const links: any[] = [];
      for (const r of radiusFlows) {
        const c = r.ClientName && r.ClientName !== r.Client ? `${r.ClientName} (${r.Client})` : r.Client;
        const s = r.ServerName && r.ServerName !== r.Server ? `${r.ServerName} (${r.Server})` : r.Server;
        if (!nodesMap.has(c)) {
          nodesMap.set(c, { name: c, category: 1, symbolSize: 14, itemStyle: { color: "#a855f7" } });
        }
        if (!nodesMap.has(s)) {
          nodesMap.set(s, { name: s, category: 0, symbolSize: 20, itemStyle: { color: "#10b981" } });
        }
        links.push({
          source: c,
          target: s,
          value: `${r.Accept}/${r.Reject}`,
          lineStyle: { color: getScoreColorHex(r.Score || 50), width: 2, curveness: 0.15 },
        });
      }
      option = {
        tooltip: { trigger: "item", formatter: "{b}" },
        legend: { data: [$_("report.pcapGraphServer") || "Server", $_("report.pcapGraphClient") || "Client"], textStyle: { color: textColor } },
        series: [
          {
            type: "graph",
            layout: graphLayout,
            categories: [{ name: $_("report.pcapGraphServer") || "Server" }, { name: $_("report.pcapGraphClient") || "Client" }],
            data: [...nodesMap.values()],
            links,
            roam: true,
            label: { show: true, position: "right", color: textColor, fontSize: 10 },
            force: { repulsion: 140, edgeLength: 90 },
          },
        ],
      };
    } else if (chartCategory === "radius_server" || chartCategory === "radius_client" || chartCategory === "radius_pair") {
      const countMap = new Map<string, { accept: number; reject: number }>();
      for (const r of radiusFlows) {
        let k = "";
        if (chartCategory === "radius_server") k = r.ServerName || r.Server;
        else if (chartCategory === "radius_client") k = r.ClientName || r.Client;
        else k = `${r.ClientName || r.Client} -> ${r.ServerName || r.Server}`;

        if (!countMap.has(k)) countMap.set(k, { accept: 0, reject: 0 });
        const entry = countMap.get(k)!;
        entry.accept += r.Accept || 0;
        entry.reject += r.Reject || 0;
      }
      const sorted = [...countMap.entries()].sort((a, b) => (b[1].accept + b[1].reject) - (a[1].accept + a[1].reject)).slice(0, 20).reverse();
      option = {
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        legend: { data: ["Accept", "Reject"], textStyle: { color: textColor } },
        grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
        xAxis: { type: "value", axisLabel: { color: textColor }, splitLine: { lineStyle: { color: lineBorder } } },
        yAxis: { type: "category", data: sorted.map(([k]) => k), axisLabel: { color: textColor, fontSize: 11 } },
        series: [
          {
            name: "Accept",
            type: "bar",
            stack: "total",
            itemStyle: { color: "#10b981" },
            data: sorted.map(([, v]) => v.accept),
          },
          {
            name: "Reject",
            type: "bar",
            stack: "total",
            itemStyle: { color: "#ef4444" },
            data: sorted.map(([, v]) => v.reject),
          },
        ],
      };
    } else if (chartCategory === "tls_flows") {
      const nodesMap = new Map<string, any>();
      const links: any[] = [];
      for (const t of tlsFlows) {
        const c = t.ClientName && t.ClientName !== t.Client ? `${t.ClientName} (${t.Client})` : t.Client;
        const s = t.ServerName && t.ServerName !== t.Server ? `${t.ServerName} (${t.Server})` : t.Server;
        if (!nodesMap.has(c)) {
          nodesMap.set(c, { name: c, category: 1, symbolSize: 14, itemStyle: { color: "#0284c7" } });
        }
        if (!nodesMap.has(s)) {
          nodesMap.set(s, { name: s, category: 0, symbolSize: 20, itemStyle: { color: "#10b981" } });
        }
        links.push({
          source: c,
          target: s,
          value: `${t.Service} / ${t.Version} (Score: ${t.Score?.toFixed(1)})`,
          lineStyle: { color: getScoreColorHex(t.Score || 50), width: 2, curveness: 0.15 },
        });
      }
      option = {
        tooltip: { trigger: "item", formatter: "{b}" },
        legend: { data: [$_("report.pcapGraphServer") || "Server", $_("report.pcapGraphClient") || "Client"], textStyle: { color: textColor } },
        series: [
          {
            type: "graph",
            layout: graphLayout,
            categories: [{ name: $_("report.pcapGraphServer") || "Server" }, { name: $_("report.pcapGraphClient") || "Client" }],
            data: [...nodesMap.values()],
            links,
            roam: true,
            label: { show: true, position: "right", color: textColor, fontSize: 10 },
            force: { repulsion: 150, edgeLength: 100 },
          },
        ],
      };
    } else if (chartCategory === "tls_version") {
      const verMap = new Map<string, number>();
      for (const t of tlsFlows) {
        const v = t.Version || "Unknown";
        verMap.set(v, (verMap.get(v) || 0) + (t.Count || 1));
      }
      const data = [...verMap.entries()].sort((a, b) => b[1] - a[1]);
      option = {
        title: { text: $_("report.pcapChartTlsVersion") || "TLS Version Distribution", textStyle: { color: dark ? "#f8fafc" : "#0f172a", fontSize: 14 } },
        tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
        legend: { type: "scroll", orient: "vertical", right: 10, top: 40, bottom: 20, textStyle: { color: textColor } },
        series: [
          {
            type: "pie",
            radius: ["40%", "70%"],
            center: ["40%", "50%"],
            avoidLabelOverlap: true,
            itemStyle: { borderRadius: 6, borderColor: dark ? "#0f172a" : "#fff", borderWidth: 2 },
            data: data.map(([name, value]) => ({ name, value })),
          },
        ],
      };
    } else if (chartCategory === "tls_cipher") {
      const csMap = new Map<string, number[]>();
      for (const t of tlsFlows) {
        const c = t.Cipher || "Unknown";
        if (!csMap.has(c)) csMap.set(c, [0, 0, 0, 0, 0]);
        const band = getScoreBandIndex(t.Score || 50);
        csMap.get(c)![band]++;
      }
      const sorted = [...csMap.entries()].sort((a, b) => {
        const sumA = a[1].reduce((x, y) => x + y, 0);
        const sumB = b[1].reduce((x, y) => x + y, 0);
        return sumB - sumA;
      }).slice(0, 15).reverse();

      const bandNames = [
        $_("report.pcapScoreLe32") || "<=32",
        $_("report.pcapScore33_41") || "33-41",
        $_("report.pcapScore42_50") || "42-50",
        $_("report.pcapScore51_66") || "51-66",
        $_("report.pcapScoreGe67") || ">=67",
      ];
      const bandColors = ["#ef4444", "#f97316", "#eab308", "#06b6d4", "#0284c7"];

      option = {
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        legend: { data: bandNames, textStyle: { color: textColor } },
        grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
        xAxis: { type: "value", axisLabel: { color: textColor }, splitLine: { lineStyle: { color: lineBorder } } },
        yAxis: { type: "category", data: sorted.map(([k]) => k), axisLabel: { color: textColor, fontSize: 10 } },
        series: bandNames.map((name, idx) => ({
          name,
          type: "bar",
          stack: "total",
          itemStyle: { color: bandColors[idx] },
          data: sorted.map(([, v]) => v[idx]),
        })),
      };
    } else if (chartCategory === "tls_country") {
      const countryMap = new Map<string, number[]>();
      for (const t of tlsFlows) {
        const rawLoc = countryMode === "server" ? t.ServerLoc : t.ClientLoc;
        const loc = parseLocInfo(rawLoc);
        const c = loc.Country || "LOCAL";
        if (!countryMap.has(c)) countryMap.set(c, [0, 0, 0, 0, 0]);
        const band = getScoreBandIndex(t.Score || 50);
        countryMap.get(c)![band]++;
      }
      const sorted = [...countryMap.entries()].sort((a, b) => {
        const sumA = a[1].reduce((x, y) => x + y, 0);
        const sumB = b[1].reduce((x, y) => x + y, 0);
        return sumB - sumA;
      }).slice(0, 15).reverse();

      const bandNames = [
        $_("report.pcapScoreLe32") || "<=32",
        $_("report.pcapScore33_41") || "33-41",
        $_("report.pcapScore42_50") || "42-50",
        $_("report.pcapScore51_66") || "51-66",
        $_("report.pcapScoreGe67") || ">=67",
      ];
      const bandColors = ["#ef4444", "#f97316", "#eab308", "#06b6d4", "#0284c7"];

      option = {
        tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
        legend: { data: bandNames, textStyle: { color: textColor } },
        grid: { left: "3%", right: "4%", bottom: "3%", containLabel: true },
        xAxis: { type: "value", axisLabel: { color: textColor }, splitLine: { lineStyle: { color: lineBorder } } },
        yAxis: { type: "category", data: sorted.map(([k]) => k), axisLabel: { color: textColor, fontSize: 11 } },
        series: bandNames.map((name, idx) => ({
          name,
          type: "bar",
          stack: "total",
          itemStyle: { color: bandColors[idx] },
          data: sorted.map(([, v]) => v[idx]),
        })),
      };
    }

    modalChartInstance.setOption(option, true);
  };

  const handleSwitchChart = (cat: string) => {
    chartCategory = cat;
    renderModalAnalyticsChart();
  };
</script>

<div class="space-y-4">
  <!-- KPI Cards with Type-Distribution Graph (Aligned with DeviceReport layout) -->
  <div class="grid grid-cols-1 lg:grid-cols-4 gap-4">
    <div class="lg:col-span-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{kpis.card1Title}</span>
        <div class="text-2xl font-bold font-mono text-cyan-400">
          {kpis.card1Val}
        </div>
        <div class="text-[10px] text-slate-400">
          {kpis.card1Sub}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{kpis.card2Title}</span>
        <div class="text-2xl font-bold font-mono text-emerald-400">
          {kpis.card2Val}
        </div>
        <div class="text-[10px] text-slate-400">
          {kpis.card2Sub}
        </div>
      </div>

      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
        <span class="text-xs font-semibold text-slate-400">{kpis.card3Title}</span>
        <div class="text-2xl font-bold font-mono text-amber-400">
          {kpis.card3Val}
        </div>
        <div class="text-[10px] text-slate-400">
          {kpis.card3Sub}
        </div>
      </div>
    </div>

    <!-- Type Distribution Bar Chart (Right side of KPI) -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-3 shadow-sm dark:shadow-lg flex flex-col justify-between">
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
          {$_("report.pcapTypeDistribution")}
        </span>
        <span class="text-[10px] font-mono text-slate-400">Top 5</span>
      </div>
      <div bind:this={inlineChartElem} class="h-24 w-full"></div>
    </div>
  </div>

  <!-- Sub-tab Selector Pills (Item switching buttons) -->
  <div class="flex flex-wrap items-center gap-2 p-1.5 rounded-2xl bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
    <button
      type="button"
      onclick={() => handleTabChange("ether")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'ether' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Network class="w-3.5 h-3.5" />
      <span>{$_("report.pcapEtherFrames")} ({ethers.length.toLocaleString()})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("dns")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'dns' ? 'bg-emerald-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Globe class="w-3.5 h-3.5" />
      <span>{$_("report.pcapDnsQueries")} ({dnsList.length.toLocaleString()})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("radius")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'radius' ? 'bg-purple-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Shield class="w-3.5 h-3.5" />
      <span>{$_("report.pcapRadiusFlows")} ({radiusFlows.length.toLocaleString()})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("tls")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'tls' ? 'bg-amber-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Lock class="w-3.5 h-3.5" />
      <span>{$_("report.pcapTlsFlows")} ({tlsFlows.length.toLocaleString()})</span>
    </button>
  </div>

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-2">
        <Activity class="w-4 h-4 text-cyan-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {#if activeTab === "ether"}{$_("report.pcapEtherTitle")}
          {:else if activeTab === "dns"}{$_("report.pcapDnsTitle")}
          {:else if activeTab === "radius"}{$_("report.pcapRadiusTitle")}
          {:else}{$_("report.pcapTlsTitle")}{/if}
        </span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 font-mono text-slate-600 dark:text-slate-300">
          {currentList.length}
        </span>
      </div>

      <!-- Action Buttons: Charts & Stats -->
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={() => openChartModal()}
          class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-300 dark:border-cyan-800/60 bg-cyan-50/50 dark:bg-cyan-950/30 hover:bg-cyan-100 dark:hover:bg-cyan-900/50 text-cyan-700 dark:text-cyan-300 text-xs font-semibold transition-colors cursor-pointer shadow-xs"
        >
          <BarChart2 class="w-3.5 h-3.5 text-cyan-600 dark:text-cyan-400" />
          <span>{$_("report.pcapBtnCharts")}</span>
        </button>
      </div>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-sans">
        <thead class="bg-slate-50 dark:bg-slate-950/80 border-b border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400 font-semibold select-none">
          <tr>
            <th class="py-2.5 px-3 w-8"></th>
            {#if activeTab === "ether"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Type")}>{$_("report.pcapColType")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>{$_("report.pcapColProtoName")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.pcapColPackets")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>{$_("report.pcapColCaptureHost")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime")}</th>
              <th class="py-2.5 px-3 text-center w-20">{$_("report.pcapColActions")}</th>
            {:else if activeTab === "dns"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>{$_("report.pcapColQueryName")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Type")}>{$_("report.pcapColRecordType")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>{$_("report.pcapColDnsServer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.pcapColCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Change")}>{$_("report.pcapColChange")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastClient")}>{$_("report.pcapColLastClient")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime")}</th>
              <th class="py-2.5 px-3 text-center w-20">{$_("report.pcapColActions")}</th>
            {:else if activeTab === "radius"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-center" onclick={() => handleSort("Score")}>{$_("report.pcapColTrustScore")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Client")}>{$_("report.pcapColRadiusClient")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>{$_("report.pcapColRadiusServer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.pcapColCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Rate")}>{$_("report.pcapColSuccessRate")}</th>
              <th class="py-2.5 px-3 text-right font-bold text-emerald-500" onclick={() => handleSort("Accept")}>{$_("report.pcapColAccept")}</th>
              <th class="py-2.5 px-3 text-right font-bold text-rose-500" onclick={() => handleSort("Reject")}>{$_("report.pcapColReject")}</th>
              <th class="py-2.5 px-3 text-right">{$_("report.pcapColReqChallenge")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime")}</th>
              <th class="py-2.5 px-3 text-center w-24">{$_("report.pcapColActions")}</th>
            {:else if activeTab === "tls"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-center" onclick={() => handleSort("Score")}>{$_("report.pcapColTrustScore")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Client")}>{$_("report.pcapColClient")}</th>
              <th class="py-2.5 px-3">{$_("report.pcapColCountry")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>{$_("report.pcapColServer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Service")}>{$_("report.pcapColService")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Version")}>{$_("report.pcapColTlsVersion")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Cipher")}>{$_("report.pcapColCipher")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.pcapColCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.colLastTime")}</th>
              <th class="py-2.5 px-3 text-center w-24">{$_("report.pcapColActions")}</th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 font-mono">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="11" class="py-8 text-center text-slate-400 font-sans">
                {internalLoading ? $_("report.loading") : $_("report.noDataFound")}
              </td>
            </tr>
          {:else}
            {#each paginatedList as item}
              {@const isExpanded = expandedId === item.ID}
              <tr
                class="hover:bg-cyan-50/50 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/10' : ''}"
                onclick={() => (expandedId = isExpanded ? null : item.ID)}
              >
                <td class="py-1 px-2 text-center text-slate-400">
                  {#if isExpanded}
                    <ChevronDown class="w-4 h-4 text-cyan-500" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </td>

                {#if activeTab === "ether"}
                  {@const e = item as EtherTypeEnt}
                  <td class="py-1 px-2 font-mono text-cyan-600 dark:text-cyan-400 font-semibold">
                    {e.Type}
                  </td>
                  <td class="py-1 px-2 font-sans font-semibold text-slate-800 dark:text-slate-200">
                    {e.Name}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">
                    {e.Count.toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">
                    {e.Host}
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(e.LastTime)}
                  </td>
                  <td class="py-1 px-2 text-center whitespace-nowrap">
                    <div class="flex items-center justify-center gap-1">
                      <button
                        type="button"
                        onclick={(evt) => openInfoModal(evt, e)}
                        class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
                        title={$_("report.pcapDetailInfo")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={(evt) => handleDeleteItem(evt, e)}
                        class="p-1 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors cursor-pointer"
                        title={$_("report.deleteAriaLabel")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                {:else if activeTab === "dns"}
                  {@const d = item as DNSQEnt}
                  <td class="py-1 px-2 font-sans font-semibold text-slate-800 dark:text-slate-200 truncate max-w-[200px]" title={d.Name}>
                    {d.Name}
                  </td>
                  <td class="py-1 px-2 font-bold text-cyan-600 dark:text-cyan-400">
                    {getDNSTypeLabel(d.Type)}
                  </td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px] font-mono">
                    {d.Server}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">
                    {d.Count.toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-right text-slate-500 dark:text-slate-400">
                    {d.Change > 0 ? `Δ ${d.Change}` : "-"}
                  </td>
                  <td class="py-1 px-2 text-[11px] text-slate-600 dark:text-slate-400 font-mono">
                    {d.LastClient}
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(d.LastTime)}
                  </td>
                  <td class="py-1 px-2 text-center whitespace-nowrap">
                    <div class="flex items-center justify-center gap-1">
                      <button
                        type="button"
                        onclick={(evt) => openInfoModal(evt, d)}
                        class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
                        title={$_("report.pcapDetailInfo")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={(evt) => handleDeleteItem(evt, d)}
                        class="p-1 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors cursor-pointer"
                        title={$_("report.deleteAriaLabel")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                {:else if activeTab === "radius"}
                  {@const r = item as RADIUSFlowEnt}
                  {@const total = (r.Accept || 0) + (r.Reject || 0)}
                  {@const rate = total ? (100 * (r.Accept || 0)) / total : 0}
                  <td class="py-1 px-2 text-center">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {r.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {r.Score?.toFixed(1) ?? "-"} (P:{r.Penalty || 0})
                    </span>
                  </td>
                  <td class="py-1 px-2 text-slate-800 dark:text-slate-200 font-sans">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{r.Client}</span>
                    {#if r.ClientName && r.ClientName !== r.Client}
                      <span class="ml-1 text-[11px] text-slate-500">({r.ClientName})</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-slate-800 dark:text-slate-200 font-sans">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{r.Server}</span>
                    {#if r.ServerName && r.ServerName !== r.Server}
                      <span class="ml-1 text-[11px] text-slate-500">({r.ServerName})</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">
                    {r.Count.toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-right font-bold {rate >= 90 ? 'text-emerald-500' : rate >= 70 ? 'text-amber-500' : 'text-rose-500'}">
                    {rate.toFixed(1)}%
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-emerald-500">
                    {(r.Accept || 0).toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-rose-500">
                    {(r.Reject || 0).toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-right text-slate-500 dark:text-slate-400 text-[11px]">
                    {r.Request || 0} / {r.Challenge || 0}
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(r.LastTime)}
                  </td>
                  <td class="py-1 px-2 text-center whitespace-nowrap">
                    <div class="flex items-center justify-center gap-1">
                      {#if r.ClientNodeID || r.ServerNodeID}
                        <a
                          href={`#/map?node=${r.ClientNodeID || r.ServerNodeID}`}
                          class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
                          title={$_("report.pcapViewOnMap")}
                          onclick={(evt) => evt.stopPropagation()}
                        >
                          <MapIcon class="w-3.5 h-3.5" />
                        </a>
                      {/if}
                      <button
                        type="button"
                        onclick={(evt) => openInfoModal(evt, r)}
                        class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
                        title={$_("report.pcapDetailInfo")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={(evt) => handleDeleteItem(evt, r)}
                        class="p-1 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors cursor-pointer"
                        title={$_("report.deleteAriaLabel")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                {:else if activeTab === "tls"}
                  {@const t = item as TLSFlowEnt}
                  {@const sLoc = parseLocInfo(t.ServerLoc)}
                  <td class="py-1 px-2 text-center">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {t.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {t.Score?.toFixed(1) ?? "-"} (P:{t.Penalty || 0})
                    </span>
                  </td>
                  <td class="py-1 px-2 font-sans text-slate-800 dark:text-slate-200">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{t.Client}</span>
                    {#if t.ClientName && t.ClientName !== t.Client}
                      <span class="ml-1 text-[11px] text-slate-500">({t.ClientName})</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 text-[11px] text-slate-600 dark:text-slate-400">
                    {sLoc.Country || "LOCAL"}
                  </td>
                  <td class="py-1 px-2 font-sans text-slate-800 dark:text-slate-200">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{t.Server}</span>
                    {#if t.ServerName && t.ServerName !== t.Server}
                      <span class="ml-1 text-[11px] text-slate-500">({t.ServerName})</span>
                    {/if}
                  </td>
                  <td class="py-1 px-2 font-semibold text-slate-700 dark:text-slate-300">
                    {t.Service}
                  </td>
                  <td class="py-1 px-2 text-[11px] {t.Version.includes('1.3') ? 'text-emerald-500 font-bold' : t.Version.includes('1.2') ? 'text-cyan-500' : 'text-rose-500 font-bold'}">
                    {t.Version || "-"}
                  </td>
                  <td class="py-1 px-2 font-mono text-[11px] text-slate-600 dark:text-slate-400 truncate max-w-[150px]" title={t.Cipher}>
                    {t.Cipher || "-"}
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">
                    {(t.Count || 0).toLocaleString()}
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(t.LastTime)}
                  </td>
                  <td class="py-1 px-2 text-center whitespace-nowrap">
                    <div class="flex items-center justify-center gap-1">
                      {#if sLoc.LatLong}
                        <a
                          href={`https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(sLoc.LatLong)}`}
                          target="_blank"
                          rel="noreferrer"
                          class="p-1 text-slate-400 hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
                          title={$_("report.pcapViewGoogleMap")}
                          onclick={(evt) => evt.stopPropagation()}
                        >
                          <MapPin class="w-3.5 h-3.5" />
                        </a>
                      {/if}
                      {#if t.ClientNodeID || t.ServerNodeID}
                        <a
                          href={`#/map?node=${t.ClientNodeID || t.ServerNodeID}`}
                          class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
                          title={$_("report.pcapViewOnMap")}
                          onclick={(evt) => evt.stopPropagation()}
                        >
                          <MapIcon class="w-3.5 h-3.5" />
                        </a>
                      {/if}
                      <button
                        type="button"
                        onclick={(evt) => openInfoModal(evt, t)}
                        class="p-1 text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
                        title={$_("report.pcapDetailInfo")}
                      >
                        <Eye class="w-3.5 h-3.5" />
                      </button>
                      <button
                        type="button"
                        onclick={(evt) => handleDeleteItem(evt, t)}
                        class="p-1 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors cursor-pointer"
                        title={$_("report.deleteAriaLabel")}
                      >
                        <Trash2 class="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </td>
                {/if}
              </tr>

              {#if isExpanded}
                <tr class="bg-slate-50 dark:bg-slate-950/60 font-sans">
                  <td colspan="11" class="p-4 space-y-3">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapDetailId")}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px] break-all">{item.ID}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapDetailFirstSeen")}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr((item as any).FirstTime)}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapDetailLastSeen")}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr((item as any).LastTime)}</div>
                      </div>
                      {#if activeTab === "dns"}
                        {@const d = item as DNSQEnt}
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapColLastMac")}</div>
                          <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px]">{d.LastMAC || "-"}</div>
                        </div>
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapColCaptureHost")}</div>
                          <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px]">{d.Host || "-"}</div>
                        </div>
                      {:else if activeTab === "tls"}
                        {@const t = item as TLSFlowEnt}
                        {@const cLoc = parseLocInfo(t.ClientLoc)}
                        {@const sLoc = parseLocInfo(t.ServerLoc)}
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.pcapDetailGeoLoc")}</div>
                          <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px]">
                            <div>Server: {sLoc.LocInfo || sLoc.Country || "LOCAL"}</div>
                            <div>Client: {cLoc.LocInfo || cLoc.Country || "LOCAL"}</div>
                          </div>
                        </div>
                      {/if}
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={currentList.length}
    />
  </div>

  <!-- Detail Information Modal -->
  {#if infoModalOpen && selectedItem}
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-xs">
      <div class="w-full max-w-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Eye class="w-4 h-4 text-cyan-500" />
            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">
              {$_("report.pcapDetailInfo")}
            </h3>
          </div>
          <button
            type="button"
            onclick={() => (infoModalOpen = false)}
            class="p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="p-5 max-h-[70vh] overflow-y-auto space-y-4 text-xs font-sans">
          <table class="w-full text-left border-collapse">
            <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
              {#if activeTab === "ether"}
                <tr><th class="py-2 px-3 text-slate-500 font-semibold w-1/3">{$_("report.pcapColType")}</th><td class="py-2 px-3 font-mono font-bold text-cyan-600">{selectedItem.Type}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColProtoName")}</th><td class="py-2 px-3 font-semibold text-slate-800 dark:text-slate-200">{selectedItem.Name}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColPackets")}</th><td class="py-2 px-3 font-mono">{selectedItem.Count?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCaptureHost")}</th><td class="py-2 px-3 font-mono">{selectedItem.Host}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailFirstSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.FirstTime)}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailLastSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.LastTime)}</td></tr>
              {:else if activeTab === "dns"}
                <tr><th class="py-2 px-3 text-slate-500 font-semibold w-1/3">{$_("report.pcapColQueryName")}</th><td class="py-2 px-3 font-bold text-slate-800 dark:text-slate-200 break-all">{selectedItem.Name}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColRecordType")}</th><td class="py-2 px-3 font-mono font-bold text-cyan-600">{getDNSTypeLabel(selectedItem.Type)}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColDnsServer")}</th><td class="py-2 px-3 font-mono">{selectedItem.Server}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCount")}</th><td class="py-2 px-3 font-mono">{selectedItem.Count?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColChange")}</th><td class="py-2 px-3 font-mono">{selectedItem.Change}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColLastClient")}</th><td class="py-2 px-3 font-mono">{selectedItem.LastClient}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColLastMac")}</th><td class="py-2 px-3 font-mono">{selectedItem.LastMAC || "-"}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCaptureHost")}</th><td class="py-2 px-3 font-mono">{selectedItem.Host}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailFirstSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.FirstTime)}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailLastSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.LastTime)}</td></tr>
              {:else if activeTab === "radius"}
                {@const tot = (selectedItem.Accept || 0) + (selectedItem.Reject || 0)}
                {@const rate = tot ? ((100 * (selectedItem.Accept || 0)) / tot).toFixed(1) + "%" : "0.0%"}
                <tr><th class="py-2 px-3 text-slate-500 font-semibold w-1/3">{$_("report.pcapColRadiusClient")}</th><td class="py-2 px-3 font-mono font-bold text-slate-800 dark:text-slate-200">{selectedItem.Client} ({selectedItem.ClientName || "-"})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColRadiusServer")}</th><td class="py-2 px-3 font-mono font-bold text-slate-800 dark:text-slate-200">{selectedItem.Server} ({selectedItem.ServerName || "-"})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColTrustScore")}</th><td class="py-2 px-3 font-mono font-bold text-cyan-600">{selectedItem.Score?.toFixed(1)} (Penalty: {selectedItem.Penalty || 0})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCount")}</th><td class="py-2 px-3 font-mono">{selectedItem.Count?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColSuccessRate")}</th><td class="py-2 px-3 font-mono font-bold text-emerald-600">{rate}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">Request</th><td class="py-2 px-3 font-mono">{selectedItem.Request?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">Challenge</th><td class="py-2 px-3 font-mono">{selectedItem.Challenge?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">Accept</th><td class="py-2 px-3 font-mono font-bold text-emerald-500">{selectedItem.Accept?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">Reject</th><td class="py-2 px-3 font-mono font-bold text-rose-500">{selectedItem.Reject?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailFirstSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.FirstTime)}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailLastSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.LastTime)}</td></tr>
              {:else if activeTab === "tls"}
                {@const cLoc = parseLocInfo(selectedItem.ClientLoc)}
                {@const sLoc = parseLocInfo(selectedItem.ServerLoc)}
                <tr><th class="py-2 px-3 text-slate-500 font-semibold w-1/3">{$_("report.pcapColClient")}</th><td class="py-2 px-3 font-mono font-bold text-slate-800 dark:text-slate-200">{selectedItem.Client} ({selectedItem.ClientName || "-"})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColClientLoc")}</th><td class="py-2 px-3 font-mono">{cLoc.LocInfo || cLoc.Country || "LOCAL"}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColServer")}</th><td class="py-2 px-3 font-mono font-bold text-slate-800 dark:text-slate-200">{selectedItem.Server} ({selectedItem.ServerName || "-"})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColServerLoc")}</th><td class="py-2 px-3 font-mono">{sLoc.LocInfo || sLoc.Country || "LOCAL"}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColService")}</th><td class="py-2 px-3 font-semibold">{selectedItem.Service}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColTlsVersion")}</th><td class="py-2 px-3 font-mono font-bold text-cyan-600">{selectedItem.Version}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCipher")}</th><td class="py-2 px-3 font-mono break-all">{selectedItem.Cipher}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColTrustScore")}</th><td class="py-2 px-3 font-mono font-bold text-cyan-600">{selectedItem.Score?.toFixed(1)} (Penalty: {selectedItem.Penalty || 0})</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapColCount")}</th><td class="py-2 px-3 font-mono">{selectedItem.Count?.toLocaleString()}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailFirstSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.FirstTime)}</td></tr>
                <tr><th class="py-2 px-3 text-slate-500 font-semibold">{$_("report.pcapDetailLastSeen")}</th><td class="py-2 px-3 font-mono">{formatTimeStr(selectedItem.LastTime)}</td></tr>
              {/if}
            </tbody>
          </table>
        </div>

        <div class="p-4 border-t border-slate-200 dark:border-slate-800 flex justify-between items-center bg-slate-50 dark:bg-slate-950/40">
          <button
            type="button"
            onclick={(evt) => handleDeleteItem(evt, selectedItem)}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-300 dark:border-rose-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/30 text-xs font-semibold cursor-pointer"
          >
            <Trash2 class="w-3.5 h-3.5" />
            <span>{$_("report.deleteAriaLabel")}</span>
          </button>
          <button
            type="button"
            onclick={() => (infoModalOpen = false)}
            class="px-4 py-1.5 rounded-xl bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold hover:bg-slate-300 dark:hover:bg-slate-600 cursor-pointer"
          >
            {$_("report.pcapClose")}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Full Chart Analytics Modal -->
  {#if chartModalOpen}
    <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/60 backdrop-blur-xs">
      <div class="w-full max-w-4xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-xl overflow-hidden flex flex-col max-h-[90vh]">
        <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <BarChart2 class="w-4 h-4 text-cyan-500" />
            <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">
              {$_("report.pcapBtnCharts")}
            </h3>
          </div>

          <!-- Chart Sub-type Selector Buttons -->
          <div class="flex flex-wrap items-center gap-1.5 text-xs">
            {#if activeTab === "ether"}
              <button
                type="button"
                onclick={() => handleSwitchChart("ether")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'ether' ? 'bg-cyan-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartEtherType")}
              </button>
            {:else if activeTab === "dns"}
              <button
                type="button"
                onclick={() => handleSwitchChart("dns_name")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'dns_name' ? 'bg-emerald-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartDnsByName")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("dns_type")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'dns_type' ? 'bg-emerald-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartDnsByType")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("dns_server")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'dns_server' ? 'bg-emerald-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartDnsByServer")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("dns_flows")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'dns_flows' ? 'bg-emerald-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartDnsFlows")}
              </button>
            {:else if activeTab === "radius"}
              <button
                type="button"
                onclick={() => handleSwitchChart("radius_flows")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'radius_flows' ? 'bg-purple-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartRadiusFlows")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("radius_server")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'radius_server' ? 'bg-purple-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartRadiusByServer")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("radius_client")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'radius_client' ? 'bg-purple-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartRadiusByClient")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("radius_pair")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'radius_pair' ? 'bg-purple-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartRadiusByPair")}
              </button>
            {:else if activeTab === "tls"}
              <button
                type="button"
                onclick={() => handleSwitchChart("tls_flows")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'tls_flows' ? 'bg-amber-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartTlsFlows")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("tls_country")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'tls_country' ? 'bg-amber-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartTlsCountry")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("tls_version")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'tls_version' ? 'bg-amber-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartTlsVersion")}
              </button>
              <button
                type="button"
                onclick={() => handleSwitchChart("tls_cipher")}
                class="px-2.5 py-1 rounded-lg font-semibold transition-colors cursor-pointer {chartCategory === 'tls_cipher' ? 'bg-amber-600 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300'}"
              >
                {$_("report.pcapChartTlsCipher")}
              </button>
            {/if}

            <button
              type="button"
              onclick={() => (chartModalOpen = false)}
              class="ml-2 p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
            >
              <X class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Controls row for Force vs Circular or Country Server/Client -->
        {#if chartCategory.includes("flows") || chartCategory === "tls_country"}
          <div class="px-5 py-2.5 border-b border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-slate-950/20 flex items-center gap-4 text-xs">
            {#if chartCategory.includes("flows")}
              <div class="flex items-center gap-2">
                <span class="font-semibold text-slate-600 dark:text-slate-300">{$_("report.pcapGraphDisplayType")}:</span>
                <select
                  bind:value={graphLayout}
                  onchange={() => renderModalAnalyticsChart()}
                  class="bg-white dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-lg px-2.5 py-1 text-slate-800 dark:text-slate-200"
                >
                  <option value="force">{$_("report.pcapGraphForce")}</option>
                  <option value="circular">{$_("report.pcapGraphCircular")}</option>
                </select>
              </div>
            {/if}
            {#if chartCategory === "tls_country"}
              <div class="flex items-center gap-2">
                <span class="font-semibold text-slate-600 dark:text-slate-300">{$_("report.pcapGraphAggregateMode")}:</span>
                <select
                  bind:value={countryMode}
                  onchange={() => renderModalAnalyticsChart()}
                  class="bg-white dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-lg px-2.5 py-1 text-slate-800 dark:text-slate-200"
                >
                  <option value="server">{$_("report.pcapGraphServer")}</option>
                  <option value="client">{$_("report.pcapGraphClient")}</option>
                </select>
              </div>
            {/if}
          </div>
        {/if}

        <div class="p-4 flex-1 min-h-[420px]">
          <div bind:this={modalChartContainer} class="w-full h-[420px]"></div>
        </div>

        <div class="p-3 border-t border-slate-200 dark:border-slate-800 flex justify-end bg-slate-50 dark:bg-slate-950/40">
          <button
            type="button"
            onclick={() => (chartModalOpen = false)}
            class="px-4 py-1.5 rounded-xl bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 text-xs font-semibold hover:bg-slate-300 dark:hover:bg-slate-600 cursor-pointer"
          >
            {$_("report.pcapClose")}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Polling Dialog for Add/Edit -->
  {#if showPollingModal}
    <PollingDialog
      bind:show={showPollingModal}
      polling={editingPolling}
      {nodes}
      onSave={async () => {
        showPollingModal = false;
        onRefresh();
      }}
    />
  {/if}
</div>
