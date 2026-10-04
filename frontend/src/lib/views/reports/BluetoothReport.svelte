<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import * as echarts from "echarts";
  import {
    Bluetooth,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Signal,
    ChevronDown,
    ChevronRight,
    LineChart,
    Eye,
    Pencil,
    Download,
    X,
    Building2,
    Calendar,
    Radio,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import { formatTimeStr } from "../../common";
  import { isDarkMode } from "../../charts/utils";
  import {
    fetchLogReport,
    resetLogReport,
    deleteLogReportItem,
    updateLogReportName,
    type BlueDeviceEnt,
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

  let devices = $state<BlueDeviceEnt[]>([]);
  let internalLoading = $state(false);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  // Modals
  let detailModalOpen = $state(false);
  let detailDevice = $state<BlueDeviceEnt | null>(null);
  let detailChartElem = $state<HTMLDivElement | null>(null);
  let detailChartInstance: echarts.ECharts | null = null;

  let editNameModalOpen = $state(false);
  let editingDevice = $state<BlueDeviceEnt | null>(null);
  let editingNameValue = $state("");

  let deleteModalOpen = $state(false);
  let deletingDevice = $state<BlueDeviceEnt | null>(null);

  const loadAll = async () => {
    internalLoading = true;
    try {
      devices = await fetchLogReport<BlueDeviceEnt>("blueDevice");
    } catch (err) {
      console.error("Failed to load Bluetooth reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    if (
      !confirm(
        $_("report.confirmClearItem", { values: { name: "Bluetoothデバイス" } }) ||
          "Bluetoothデバイスデータを全消去しますか？"
      )
    ) {
      return;
    }
    try {
      await resetLogReport("blueDevice");
      await loadAll();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  }

  onMount(() => {
    loadAll();

    const handleResize = () => {
      detailChartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      detailChartInstance?.dispose();
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

  const getLatestRSSI = (d: BlueDeviceEnt): number => {
    if (d.RSSI && d.RSSI.length > 0) {
      return d.RSSI[d.RSSI.length - 1].Value;
    }
    return -100;
  };

  const getRSSIBadge = (rssi: number) => {
    if (rssi >= -60) {
      return { label: `${rssi} dBm`, color: "text-emerald-500 bg-emerald-500/10 border-emerald-500/20" };
    }
    if (rssi >= -75) {
      return { label: `${rssi} dBm`, color: "text-teal-500 bg-teal-500/10 border-teal-500/20" };
    }
    if (rssi >= -85) {
      return { label: `${rssi} dBm`, color: "text-amber-500 bg-amber-500/10 border-amber-500/20" };
    }
    return { label: `${rssi} dBm`, color: "text-rose-500 bg-rose-500/10 border-rose-500/20" };
  };

  const filteredList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    let res = devices;
    if (q) {
      res = res.filter(
        (d) =>
          d.Name?.toLowerCase().includes(q) ||
          d.Address?.toLowerCase().includes(q) ||
          d.Vendor?.toLowerCase().includes(q) ||
          d.Host?.toLowerCase().includes(q) ||
          d.AddressType?.toLowerCase().includes(q) ||
          d.Info?.toLowerCase().includes(q)
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

  const paginatedList = $derived.by(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredList.slice(start, start + pageSize);
  });

  const openEditNameModal = (d: BlueDeviceEnt) => {
    editingDevice = d;
    editingNameValue = d.Name || "";
    editNameModalOpen = true;
  };

  const handleSaveName = async () => {
    if (!editingDevice) return;
    try {
      await updateLogReportName("blueDevice", editingDevice.ID, editingNameValue);
      editNameModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert("名前の変更に失敗しました: " + (err.message || err));
    }
  };

  const openDeleteModal = (d: BlueDeviceEnt) => {
    deletingDevice = d;
    deleteModalOpen = true;
  };

  const handleConfirmDelete = async () => {
    if (!deletingDevice) return;
    try {
      await deleteLogReportItem("blueDevice", deletingDevice.ID);
      deleteModalOpen = false;
      await loadAll();
    } catch (err: any) {
      alert("削除に失敗しました: " + (err.message || err));
    }
  };

  const openDetailModal = async (d: BlueDeviceEnt) => {
    detailDevice = d;
    detailModalOpen = true;
    await tick();
    renderDetailChart();
  };

  const renderDetailChart = () => {
    if (!detailChartElem || !detailDevice) return;
    if (detailChartInstance) {
      detailChartInstance.dispose();
    }
    detailChartInstance = echarts.init(detailChartElem);
    const dark = isDarkMode();
    const textColor = dark ? "#cbd5e1" : "#475569";
    const splitLineColor = dark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";

    const rssiData = (detailDevice.RSSI || []).map((r) => [
      new Date(r.Time * 1e6),
      r.Value,
    ]);

    detailChartInstance.setOption({
      tooltip: { trigger: "axis" },
      legend: { data: ["信号強度 (RSSI dBm)"], textStyle: { color: textColor } },
      grid: { left: "4%", right: "4%", bottom: "12%", top: "18%", containLabel: true },
      xAxis: {
        type: "time",
        axisLabel: { color: textColor, fontSize: 11 },
        axisLine: { lineStyle: { color: splitLineColor } },
      },
      yAxis: {
        type: "value",
        name: "dBm",
        max: -30,
        min: -100,
        axisLabel: { color: textColor },
        splitLine: { lineStyle: { color: splitLineColor } },
      },
      series: [
        {
          name: "信号強度 (RSSI dBm)",
          type: "line",
          smooth: true,
          showSymbol: false,
          data: rssiData,
          itemStyle: { color: "#3b82f6" },
          areaStyle: { opacity: 0.15 },
        },
      ],
    });
  };

  export function exportCSV(): void {
    let csv = "アドレス,アドレス種別,名前,ベンダー,送信元ホスト,信号レベル(dBm),受信回数,追加情報,初回日時,最終日時\n";
    filteredList.forEach((d) => {
      const lastR = getLatestRSSI(d);
      csv += `"${d.Address}","${d.AddressType || ""}","${d.Name || ""}","${d.Vendor || ""}","${d.Host || ""}",${lastR},${d.Count || 0},"${(d.Info || "").replace(/"/g, '""')}","${formatTimeStr(d.FirstTime * 1e6)}","${formatTimeStr(d.LastTime * 1e6)}"\n`;
    });

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_bluetooth_report_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Title & Actions -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Bluetooth class="w-5 h-5 text-blue-500" />
        {$_("report.tabBluetooth")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
        twBlueScan から受信した Bluetooth (BLE) デバイスの検出・信号強度レポート
      </p>
    </div>

    <div class="flex items-center gap-2">
      <button
        type="button"
        onclick={exportCSV}
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium border border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-200 transition-colors"
      >
        <Download class="w-3.5 h-3.5" />
        {$_("report.exportCsv")}
      </button>
    </div>
  </div>

  <!-- Table Card -->
  <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 shadow-sm overflow-hidden flex flex-col">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
        <thead class="bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 font-medium uppercase tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("RSSI")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorSignal")}</span>
                {#if sortColumn === "RSSI"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Address")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorAddress")}</span>
                {#if sortColumn === "Address"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Name")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorName")}</span>
                {#if sortColumn === "Name"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Vendor")}>
              <div class="flex items-center gap-1">
                <span>ベンダー</span>
                {#if sortColumn === "Vendor"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Host")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorHost")}</span>
                {#if sortColumn === "Host"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("AddressType")}>
              <div class="flex items-center gap-1">
                <span>アドレス種別</span>
                {#if sortColumn === "AddressType"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5">追加情報</th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("Count")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorCount")}</span>
                {#if sortColumn === "Count"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 cursor-pointer hover:text-slate-900 dark:hover:text-white" onclick={() => handleSort("LastTime")}>
              <div class="flex items-center gap-1">
                <span>{$_("report.sensorLast")}</span>
                {#if sortColumn === "LastTime"}
                  {sortDirection === "asc" ? "▲" : "▼"}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <th class="px-3 py-2.5 text-right">{$_("common.action")}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="10" class="text-center py-12 text-slate-400">
                <div class="flex flex-col items-center justify-center gap-2">
                  <Bluetooth class="w-8 h-8 opacity-20 text-blue-500" />
                  <p class="text-sm">Bluetoothデバイスはまだ検出されていません</p>
                </div>
              </td>
            </tr>
          {:else}
            {#each paginatedList as item}
              {@const lastRSSI = getLatestRSSI(item)}
              {@const rssiBadge = getRSSIBadge(lastRSSI)}
              <tr class="hover:bg-slate-50/70 dark:hover:bg-slate-800/40 transition-colors group">
                <td class="px-3 py-2.5 whitespace-nowrap">
                  <span class={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-medium border ${rssiBadge.color}`}>
                    <Signal class="w-3 h-3" />
                    {rssiBadge.label}
                  </span>
                </td>
                <td class="px-3 py-2.5 font-mono text-[11px] font-medium text-slate-900 dark:text-slate-100 whitespace-nowrap">
                  <button
                    type="button"
                    onclick={() => openDetailModal(item)}
                    class="hover:underline hover:text-blue-500 text-left"
                  >
                    {item.Address}
                  </button>
                </td>
                <td class="px-3 py-2.5 font-medium text-slate-800 dark:text-slate-200 max-w-[160px] truncate">
                  <div class="flex items-center gap-1.5">
                    <span class="truncate">{item.Name || "-"}</span>
                    <button
                      type="button"
                      onclick={() => openEditNameModal(item)}
                      class="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-blue-500 transition-opacity p-0.5"
                      title="名前を変更"
                    >
                      <Pencil class="w-3 h-3" />
                    </button>
                  </div>
                </td>
                <td class="px-3 py-2.5 text-slate-600 dark:text-slate-400 max-w-[140px] truncate">
                  {item.Vendor || "-"}
                </td>
                <td class="px-3 py-2.5 text-slate-600 dark:text-slate-400 whitespace-nowrap">
                  {item.Host || "-"}
                </td>
                <td class="px-3 py-2.5 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {item.AddressType || "-"}
                </td>
                <td class="px-3 py-2.5 text-slate-500 dark:text-slate-400 text-[11px] max-w-[200px] truncate" title={item.Info || ""}>
                  {item.Info || "-"}
                </td>
                <td class="px-3 py-2.5 text-xs text-slate-600 dark:text-slate-400 whitespace-nowrap">
                  {item.Count || 0}
                </td>
                <td class="px-3 py-2.5 text-[11px] text-slate-500 dark:text-slate-400 whitespace-nowrap">
                  {formatTimeStr(item.LastTime * 1e6)}
                </td>
                <td class="px-3 py-2.5 text-right whitespace-nowrap">
                  <div class="flex items-center justify-end gap-1">
                    <button
                      type="button"
                      onclick={() => openDetailModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-blue-500 transition-colors"
                      title="詳細と電波履歴"
                    >
                      <Eye class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => openEditNameModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-amber-500 transition-colors"
                      title="名前変更"
                    >
                      <Pencil class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => openDeleteModal(item)}
                      class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-500 hover:text-rose-500 transition-colors"
                      title="削除"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    {#if filteredList.length > 0}
      <div class="p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/30">
        <ReportPagination
          totalCount={filteredList.length}
          bind:pageSize
          bind:currentPage
        />
      </div>
    {/if}
  </div>
</div>

<!-- Bluetooth Device Detail Modal -->
{#if detailModalOpen && detailDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-3xl max-h-[90vh] flex flex-col overflow-hidden">
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-500">
            <Bluetooth class="w-5 h-5" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
              Bluetoothデバイス情報 - {detailDevice.Name || detailDevice.Address}
            </h3>
            <p class="text-xs text-slate-500 font-mono mt-0.5">
              {detailDevice.Address} ({detailDevice.AddressType || "Public"})
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="p-1.5 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="p-6 overflow-y-auto space-y-6 flex-1 text-xs">
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-slate-50 dark:bg-slate-800/40 p-4 rounded-xl border border-slate-200 dark:border-slate-800">
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">アドレス</div>
            <div class="font-mono font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailDevice.Address}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">名前</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailDevice.Name || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">ベンダー</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{detailDevice.Vendor || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">信号強度</div>
            <div class="font-medium text-slate-800 dark:text-slate-200 mt-0.5">{getLatestRSSI(detailDevice)} dBm</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">送信元ホスト</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{detailDevice.Host || "-"}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">受信回数</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{detailDevice.Count || 0}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">初回検出</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{formatTimeStr(detailDevice.FirstTime * 1e6)}</div>
          </div>
          <div>
            <div class="text-[10px] text-slate-400 uppercase font-semibold">最終検出</div>
            <div class="text-slate-600 dark:text-slate-400 mt-0.5">{formatTimeStr(detailDevice.LastTime * 1e6)}</div>
          </div>
        </div>

        {#if detailDevice.Info}
          <div class="p-3 bg-slate-50 dark:bg-slate-800/40 rounded-xl border border-slate-200 dark:border-slate-800">
            <div class="text-[10px] text-slate-400 uppercase font-semibold mb-1">追加情報</div>
            <div class="font-mono text-slate-700 dark:text-slate-300 break-all">{detailDevice.Info}</div>
          </div>
        {/if}

        <div>
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 mb-2 flex items-center gap-2">
            <LineChart class="w-4 h-4 text-blue-500" />
            信号強度 (RSSI) 推移
          </h4>
          <div
            bind:this={detailChartElem}
            class="w-full h-56 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40"
          ></div>
        </div>
      </div>

      <div class="px-6 py-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/30 flex justify-end">
        <button
          type="button"
          onclick={() => (detailModalOpen = false)}
          class="px-4 py-2 rounded-lg text-xs font-semibold bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-800 dark:text-slate-100 transition-colors"
        >
          {$_("common.close")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Name Modal -->
{#if editNameModalOpen && editingDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Pencil class="w-4 h-4 text-blue-500" />
          名前変更
        </h3>
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <div class="space-y-2">
        <label for="ble-device-name-input" class="block text-xs font-semibold text-slate-600 dark:text-slate-300">
          デバイス名
        </label>
        <input
          id="ble-device-name-input"
          type="text"
          bind:value={editingNameValue}
          placeholder="例: BLEビーコン A"
          class="w-full px-3 py-2 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-xs text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500/50"
        />
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (editNameModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleSaveName}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-blue-600 hover:bg-blue-500 text-white shadow-sm transition-colors"
        >
          {$_("common.save")}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if deleteModalOpen && deletingDevice}
  <div class="fixed inset-0 z-50 bg-slate-950/60 backdrop-blur-sm flex items-center justify-center p-4">
    <div class="bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-2xl w-full max-w-md p-6 space-y-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-500 shrink-0">
          <Trash2 class="w-5 h-5" />
        </div>
        <div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            Bluetoothデバイス削除
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
            選択したBluetoothデバイスを削除しますか？
          </p>
        </div>
      </div>

      <div class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-lg text-xs space-y-1 font-mono text-slate-700 dark:text-slate-300">
        <div>アドレス: <span class="font-bold">{deletingDevice.Address}</span></div>
        {#if deletingDevice.Name}
          <div>名前: <span class="font-bold">{deletingDevice.Name}</span></div>
        {/if}
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (deleteModalOpen = false)}
          class="px-3 py-1.5 rounded-lg text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={handleConfirmDelete}
          class="px-4 py-1.5 rounded-lg text-xs font-semibold bg-rose-600 hover:bg-rose-500 text-white shadow-sm transition-colors"
        >
          {$_("common.delete")}
        </button>
      </div>
    </div>
  </div>
{/if}
