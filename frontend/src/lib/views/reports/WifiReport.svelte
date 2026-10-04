<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Wifi,
    Radio,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Layers,
    ChevronDown,
    ChevronRight,
    Signal,
    Building2,
    Calendar,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import { formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    type WifiAPEnt,
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

  let apList = $state<WifiAPEnt[]>([]);
  let internalLoading = $state(false);
  let expandedId = $state<string | null>(null);
  let showPollingModal = $state(false);
  let editingPolling = $state<PollingEnt | null>(null);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // ECharts container & instance for Channel distribution
  let chartElem = $state<HTMLDivElement | null>(null);
  let chartInstance: echarts.ECharts | null = null;

  const loadAPData = async () => {
    internalLoading = true;
    try {
      apList = await fetchLogReport<WifiAPEnt>("wifiAP");
      await tick();
      renderChart();
    } catch (err) {
      console.error("Failed to load WifiAP data:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAPData();
  };

  onMount(() => {
    loadAPData();
    const handleResize = () => chartInstance?.resize();
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      chartInstance?.dispose();
    };
  });

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const getLatestRSSI = (ap: WifiAPEnt): number => {
    if (ap.RSSI && ap.RSSI.length > 0) {
      return ap.RSSI[ap.RSSI.length - 1].Value;
    }
    return -100;
  };

  const filteredAPs = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = apList;
    if (q) {
      res = res.filter(
        (ap) =>
          ap.SSID?.toLowerCase().includes(q) ||
          ap.BSSID?.toLowerCase().includes(q) ||
          ap.Vendor?.toLowerCase().includes(q) ||
          ap.Host?.toLowerCase().includes(q) ||
          ap.Channel?.toLowerCase().includes(q) ||
          ap.Info?.toLowerCase().includes(q)
      );
    }

    return [...res].sort((a, b) => {
      let valA: any = (a as any)[sortColumn];
      let valB: any = (b as any)[sortColumn];

      if (sortColumn === "RSSI") {
        valA = getLatestRSSI(a);
        valB = getLatestRSSI(b);
      }

      if (typeof valA === "string") {
        const cmp = valA.localeCompare(valB || "");
        return sortDirection === "asc" ? cmp : -cmp;
      }
      valA = Number(valA || 0);
      valB = Number(valB || 0);
      return sortDirection === "asc" ? valA - valB : valB - valA;
    });
  });

  const paginatedAPs = $derived.by(() => {
    if (pageSize === -1) return filteredAPs;
    const start = (currentPage - 1) * pageSize;
    return filteredAPs.slice(start, start + pageSize);
  });

  const stats = $derived.by(() => {
    const total = apList.length;
    const uniqueSSIDs = new Set(apList.map((a) => a.SSID).filter(Boolean)).size;
    const uniqueHosts = new Set(apList.map((a) => a.Host).filter(Boolean)).size;
    const channels = new Map<string, number>();
    for (const a of apList) {
      if (a.Channel) {
        channels.set(a.Channel, (channels.get(a.Channel) || 0) + 1);
      }
    }
    let topChannel = "-";
    let maxC = 0;
    for (const [ch, count] of channels.entries()) {
      if (count > maxC) {
        maxC = count;
        topChannel = `Ch ${ch} (${count})`;
      }
    }

    return { total, uniqueSSIDs, uniqueHosts, topChannel };
  });

  const renderChart = () => {
    if (!chartElem) return;
    if (!chartInstance) {
      chartInstance = echarts.init(chartElem);
    }

    const channelMap = new Map<string, number>();
    for (const ap of apList) {
      const ch = ap.Channel ? `Ch ${ap.Channel}` : "Unknown";
      channelMap.set(ch, (channelMap.get(ch) || 0) + 1);
    }

    const sortedChannels = [...channelMap.entries()].sort((a, b) => b[1] - a[1]).slice(0, 10);
    const dark = isDarkMode();

    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: {
        trigger: "axis",
        axisPointer: { type: "shadow" },
        backgroundColor: dark ? "rgba(15, 23, 42, 0.95)" : "rgba(255, 255, 255, 0.95)",
        borderColor: dark ? "#334155" : "#e2e8f0",
        textStyle: { color: dark ? "#f8fafc" : "#0f172a", fontSize: 12 },
      },
      grid: { top: "10%", left: "3%", right: "4%", bottom: "5%", containLabel: true },
      xAxis: {
        type: "category",
        data: sortedChannels.map((c) => c[0]),
        axisLabel: { color: dark ? "#94a3b8" : "#64748b", fontSize: 11 },
        axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
      },
      yAxis: {
        type: "value",
        axisLabel: { color: dark ? "#94a3b8" : "#64748b", fontSize: 11 },
        splitLine: { lineStyle: { color: dark ? "#1e293b" : "#f1f5f9" } },
      },
      series: [
        {
          name: $_("report.apCount") || "AP Count",
          type: "bar",
          data: sortedChannels.map((c) => c[1]),
          itemStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: "#06b6d4" },
              { offset: 1, color: "#0891b2" },
            ]),
            borderRadius: [4, 4, 0, 0],
          },
          barMaxWidth: 32,
        },
      ],
    };
    chartInstance.setOption(option);
  };

  export async function handleClear(): Promise<void> {
    if (!confirm($_("report.confirmClearWifi") || "Wi-Fi APレポートデータを全消去しますか？")) return;
    try {
      await resetLogReport("wifiAP");
      await loadAPData();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  }

  export function handleAddNewPolling(): void {
    editingPolling = {
      id: "",
      name: "twWifiScan",
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "syslog",
      mode: "twwifiscan",
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
    const header = "Host,BSSID,SSID,Channel,Vendor,LatestRSSI,Count,Change,Info,FirstTime,LastTime\n";
    const rows = filteredAPs
      .map((ap) => {
        const rssi = getLatestRSSI(ap);
        return `"${ap.Host}","${ap.BSSID}","${ap.SSID || ""}","${ap.Channel || ""}","${ap.Vendor || ""}","${rssi}","${ap.Count}","${ap.Change}","${ap.Info || ""}","${formatTimeStr(ap.FirstTime)}","${formatTimeStr(ap.LastTime)}"`;
      })
      .join("\n");
    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_wifi_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <!-- Title Header -->
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Wifi class="w-5 h-5 text-cyan-500 dark:text-cyan-400" />
      {$_("report.wifiTitle") || $_("report.tabWifi")}
    </h2>
    <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">
      {$_("report.wifiSubtitle")}
    </p>
  </div>

  <!-- Summary Cards & Channel Chart -->
  <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/50 border border-cyan-200 dark:border-cyan-800/60 text-cyan-600 dark:text-cyan-400">
        <Wifi class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          {$_("report.wifiTotalAPs") || "検出Wi-Fi AP総数"}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.total.toLocaleString()}
        </div>
      </div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/50 border border-emerald-200 dark:border-emerald-800/60 text-emerald-600 dark:text-emerald-400">
        <Radio class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          {$_("report.wifiUniqueSSIDs") || "ユニークSSID数"}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.uniqueSSIDs.toLocaleString()}
        </div>
      </div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-purple-50 dark:bg-purple-950/50 border border-purple-200 dark:border-purple-800/60 text-purple-600 dark:text-purple-400">
        <Layers class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          {$_("report.wifiTopChannel") || "最多利用チャネル"}
        </div>
        <div class="text-base font-bold font-mono text-slate-800 dark:text-slate-100 truncate">
          {stats.topChannel}
        </div>
      </div>
    </div>

    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center gap-3">
      <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/50 border border-amber-200 dark:border-amber-800/60 text-amber-600 dark:text-amber-400">
        <Building2 class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          {$_("report.wifiScanners") || "スキャナーノード数"}
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {stats.uniqueHosts.toLocaleString()}
        </div>
      </div>
    </div>
  </div>

  <!-- Channel Chart Card -->
  {#if apList.length > 0}
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm">
      <div class="text-xs font-bold text-slate-700 dark:text-slate-300 mb-2 flex items-center gap-2">
        <Radio class="w-4 h-4 text-cyan-500" />
        {$_("report.wifiChannelDistribution") || "チャネル別AP分布 (Top 10)"}
      </div>
      <div bind:this={chartElem} class="h-44 w-full"></div>
    </div>
  {/if}

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Wifi class="w-4 h-4 text-cyan-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {$_("report.wifiAPList") || "Wi-Fi アクセスポイント一覧"}
        </span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 font-mono text-slate-600 dark:text-slate-300">
          {filteredAPs.length}
        </span>
      </div>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-sans">
        <thead class="bg-slate-50 dark:bg-slate-950/80 border-b border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400 font-semibold select-none">
          <tr>
            <th class="py-2.5 px-3 w-8"></th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("SSID")}>
              <div class="flex items-center gap-1">
                <span>SSID</span>
                {#if sortColumn === "SSID"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("BSSID")}>
              <div class="flex items-center gap-1">
                <span>BSSID (MAC)</span>
                {#if sortColumn === "BSSID"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Channel")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.channel") || "チャネル"}</span>
                {#if sortColumn === "Channel"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Vendor")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.vendor") || "ベンダー"}</span>
                {#if sortColumn === "Vendor"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("RSSI")}>
              <div class="flex items-center justify-end gap-1">
                <span>RSSI (dBm)</span>
                {#if sortColumn === "RSSI"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>
              <div class="flex items-center justify-end gap-1">
                <span>{$_("report.count") || "回数"}</span>
                {#if sortColumn === "Count"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.scannerHost") || "スキャナー"}</span>
                {#if sortColumn === "Host"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.lastSeen") || "最終確認"}</span>
                {#if sortColumn === "LastTime"}
                  {#if sortDirection === "asc"}<ArrowUp class="w-3 h-3 text-cyan-500" />{:else}<ArrowDown class="w-3 h-3 text-cyan-500" />{/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 font-mono">
          {#if paginatedAPs.length === 0}
            <tr>
              <td colspan="9" class="py-8 text-center text-slate-400 font-sans">
                {internalLoading ? $_("report.loading") : $_("report.noDataFound")}
              </td>
            </tr>
          {:else}
            {#each paginatedAPs as ap}
              {@const rssi = getLatestRSSI(ap)}
              {@const isExpanded = expandedId === ap.ID}
              <tr
                class="hover:bg-cyan-50/50 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/10' : ''}"
                onclick={() => (expandedId = isExpanded ? null : ap.ID)}
              >
                <td class="py-1 px-2 text-center text-slate-400">
                  {#if isExpanded}
                    <ChevronDown class="w-4 h-4 text-cyan-500" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </td>
                <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">
                  {ap.SSID || "(Hidden SSID)"}
                </td>
                <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-mono text-[11px]">
                  {ap.BSSID}
                </td>
                <td class="py-1 px-2 text-slate-700 dark:text-slate-300">
                  {ap.Channel ? `Ch ${ap.Channel}` : "-"}
                </td>
                <td class="py-1 px-2 font-sans text-slate-600 dark:text-slate-400 text-[11px] truncate max-w-[140px]" title={ap.Vendor}>
                  {ap.Vendor || "-"}
                </td>
                <td class="py-1 px-2 text-right">
                  <span class="inline-flex items-center gap-1 font-bold {rssi > -60 ? 'text-emerald-500' : rssi > -75 ? 'text-amber-500' : 'text-rose-500'}">
                    <Signal class="w-3 h-3" />
                    {rssi} dBm
                  </span>
                </td>
                <td class="py-1 px-2 text-right text-slate-700 dark:text-slate-300 font-bold">
                  {ap.Count.toLocaleString()}
                  {#if ap.Change > 0}
                    <span class="ml-1 text-[10px] text-amber-500" title={$_("report.changeCount") || "属性変更回数"}>
                      (Δ{ap.Change})
                    </span>
                  {/if}
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px] truncate max-w-[120px]" title={ap.Host}>
                  {ap.Host}
                </td>
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {formatTimeStr(ap.LastTime)}
                </td>
              </tr>

              {#if isExpanded}
                <tr class="bg-slate-50 dark:bg-slate-950/60 font-sans">
                  <td colspan="9" class="p-4 space-y-3">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.wifiSecurityDetails") || "セキュリティ & 詳細情報"}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200 break-words">{ap.Info || $_("report.none") || "なし"}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.firstSeen") || "初回検知日時"}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr(ap.FirstTime)}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.wifiRssiHistory") || "RSSI 履歴件数"}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{ap.RSSI?.length || 0} {$_("report.samples") || "サンプル"}</div>
                      </div>
                    </div>

                    <!-- Mini RSSI sparkline visual -->
                    {#if ap.RSSI && ap.RSSI.length > 1}
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-2">
                        <div class="text-[11px] font-bold text-slate-400 uppercase flex items-center gap-1.5">
                          <Signal class="w-3.5 h-3.5 text-cyan-500" />
                          {$_("report.wifiRssiTrend", { values: { count: ap.RSSI.length } }) || `RSSI 電波強度推移 (直近 ${ap.RSSI.length} 件)`}
                        </div>
                        <div class="flex items-end gap-1 h-12 pt-2 px-1">
                          {#each ap.RSSI.slice(-40) as r}
                            {@const normHeight = Math.max(10, Math.min(100, (r.Value + 100) * 1.5))}
                            <div
                              class="flex-1 rounded-t transition-all {r.Value > -60 ? 'bg-emerald-500' : r.Value > -75 ? 'bg-amber-500' : 'bg-rose-500'}"
                              style="height: {normHeight}%;"
                              title="{r.Value} dBm ({formatTimeStr(r.Time)})"
                            ></div>
                          {/each}
                        </div>
                      </div>
                    {/if}
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
      totalCount={filteredAPs.length}
    />
  </div>

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
