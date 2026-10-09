<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Sparkles,
    BarChart3,
    Download,
    Trash2,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import {
    fetchAIList,
    deleteAIResult,
    type AIListEnt,
    type NodeEnt,
    type PollingEnt,
    type EventLogEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";
  import { getScoreColor, getScoreIcon, formatTimeStr } from "../../common";
  import AIReportModal from "./AIReportModal.svelte";
  import ReportPagination from "./components/ReportPagination.svelte";

  let {
    nodes = [],
    pollings = [],
    logs = [],
    searchQuery = "",
    onReload,
  }: {
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    logs?: EventLogEnt[];
    searchQuery?: string;
    onReload?: () => Promise<void> | void;
  } = $props();

  let data = $state<AIListEnt[]>([]);
  let loading = $state(false);
  let showReportModal = $state(false);
  let selectedReportId = $state("");
  let selectedReportTitle = $state("");

  // Pagination & Sorting state
  let pageSize = $state(25);
  let currentPage = $state(1);
  let sortField = $state<keyof AIListEnt>("Score");
  let sortAsc = $state(false);

  export const refresh = async () => {
    await loadData();
  };

  const loadData = async () => {
    loading = true;
    try {
      data = await fetchAIList();
    } catch (e) {
      console.error("Failed to load AI list:", e);
    } finally {
      loading = false;
    }
  };

  onMount(() => {
    loadData();
  });

  const uniqueNodes = $derived(new Set(data.map((d) => d.Node)));
  const uniqueNodeCount = $derived(uniqueNodes.size);
  const highAnomalyItems = $derived(data.filter((d) => d.Score >= 60));
  const highAnomalyCount = $derived(highAnomalyItems.length);
  const highAnomalyNodes = $derived(new Set(highAnomalyItems.map((d) => d.Node)));
  const highAnomalyNodeCount = $derived(highAnomalyNodes.size);
  const avgScore = $derived(
    data.length > 0
      ? (data.reduce((acc, d) => acc + d.Score, 0) / data.length).toFixed(2)
      : "0.00"
  );
  const maxScore = $derived(
    data.length > 0 ? Math.max(...data.map((d) => d.Score)).toFixed(2) : "0.00"
  );

  const filteredData = $derived.by(() => {
    let list = [...data];
    const q = searchQuery.trim().toLowerCase();
    if (q) {
      list = list.filter(
        (item) =>
          item.Node.toLowerCase().includes(q) ||
          item.Polling.toLowerCase().includes(q) ||
          String(item.Score).includes(q) ||
          String(item.Count).includes(q)
      );
    }

    list.sort((a, b) => {
      const valA = a[sortField];
      const valB = b[sortField];
      if (typeof valA === "number" && typeof valB === "number") {
        return sortAsc ? valA - valB : valB - valA;
      }
      return sortAsc
        ? String(valA).localeCompare(String(valB))
        : String(valB).localeCompare(String(valA));
    });

    return list;
  });

  const paginatedData = $derived.by(() => {
    if (pageSize === -1) return filteredData;
    const start = (currentPage - 1) * pageSize;
    return filteredData.slice(start, start + pageSize);
  });

  const handleSort = (field: keyof AIListEnt) => {
    if (sortField === field) {
      sortAsc = !sortAsc;
    } else {
      sortField = field;
      sortAsc = false;
    }
  };

  const handleOpenReport = (item: AIListEnt) => {
    selectedReportId = item.ID;
    selectedReportTitle = `${item.Node} - ${item.Polling}`;
    showReportModal = true;
  };

  const handleExportAIData = (item: AIListEnt) => {
    const url = `/api/ai/export/${encodeURIComponent(item.ID)}`;
    const a = document.createElement("a");
    a.href = url;
    a.download = `twsnmp_ai_data_${item.ID}_${Date.now()}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  const handleClearItem = async (item: AIListEnt) => {
    const confirmMsg = $_("report.confirmClearAIItem", { values: { polling: item.Polling, node: item.Node } }) || `ポーリング「${item.Polling}」(${item.Node}) の異常検知結果を削除しますか？`;
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || '異常検知結果削除の確認',
      message: confirmMsg,
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;

    loading = true;
    try {
      await deleteAIResult(item.ID);
      await loadData();
      await onReload?.();
    } catch (e) {
      console.error("Failed to clear AI result:", e);
      showAlert({
        title: $_('common.error') || 'エラー',
        message: $_('report.alertDeleteFailed', { values: { error: (e as any)?.message || e } }) || "削除に失敗しました",
        type: 'danger',
      });
    } finally {
      loading = false;
    }
  };

  export function exportCSV(): void {
    const headers = ["Anomaly score", "Node Name", "Polling", "Count", "Last time"];
    const rows = filteredData.map((d) => [
      d.Score.toFixed(2),
      `"${d.Node.replace(/"/g, '""')}"`,
      `"${d.Polling.replace(/"/g, '""')}"`,
      d.Count,
      `"${formatTimeStr(d.LastTime > 1e11 ? d.LastTime : d.LastTime * 1000)}"`,
    ]);
    const csvContent = [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_ai_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Top Header Title -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Sparkles class="w-5 h-5 text-cyan-500 dark:text-cyan-400" />
        {$_("report.aiTitle")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.aiSubtitle")}</p>
    </div>
  </div>

  <!-- KPI Summary Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnalyzedPollings")}</span>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
        {data.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
      </div>
      <div class="text-[10px] text-slate-400">
        {$_("report.aiTargetNodesSub", { values: { count: uniqueNodeCount } })}
      </div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnomalyPollings")}</span>
      <div class="text-2xl font-bold font-mono {highAnomalyCount > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'}">
        {highAnomalyCount} <span class="text-xs font-normal text-slate-400">{$_("report.unitPollings")}</span>
      </div>
      <div class="text-[10px] {highAnomalyCount > 0 ? 'text-rose-500/80' : 'text-emerald-500/80'}">
        {highAnomalyCount > 0 ? $_("report.aiSpikesSub", { values: { count: highAnomalyNodeCount } }) : $_("report.aiNoSpikesSub")}
      </div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAvgScore")}</span>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
        {avgScore}
      </div>
      <div class="text-[10px] text-slate-400">
        {data.length > 0 ? $_("report.aiMaxScoreSub", { values: { score: maxScore } }) : $_("report.aiStableSub")}
      </div>
    </div>
  </div>

  <!-- AI Anomaly Detection Table Container -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-sans select-none">
        <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800">
          <tr>
            <!-- Anomaly Score Sortable -->
            <th
              onclick={() => handleSort("Score")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors w-[18%]"
            >
              <div class="flex items-center gap-1">
                <span>{$_("AIList.AnomaryScore")}</span>
                {#if sortField === "Score"}
                  {#if sortAsc}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>

            <!-- Node Name Sortable -->
            <th
              onclick={() => handleSort("Node")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors w-[26%]"
            >
              <div class="flex items-center gap-1">
                <span>{$_("AIList.Node")}</span>
                {#if sortField === "Node"}
                  {#if sortAsc}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>

            <!-- Polling Sortable -->
            <th
              onclick={() => handleSort("Polling")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors w-[22%]"
            >
              <div class="flex items-center gap-1">
                <span>{$_("AIList.Polling")}</span>
                {#if sortField === "Polling"}
                  {#if sortAsc}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>

            <!-- Count Sortable -->
            <th
              onclick={() => handleSort("Count")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors w-[12%]"
            >
              <div class="flex items-center gap-1">
                <span>{$_("AIList.Count")}</span>
                {#if sortField === "Count"}
                  {#if sortAsc}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>

            <!-- Last Time Sortable -->
            <th
              onclick={() => handleSort("LastTime")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors w-[16%]"
            >
              <div class="flex items-center gap-1">
                <span>{$_("AIList.LastTime")}</span>
                {#if sortField === "LastTime"}
                  {#if sortAsc}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>

            <!-- Actions Header -->
            <th class="py-2.5 px-3 text-center w-28">
              {$_("report.colAction")}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
          {#if loading && data.length === 0}
            <tr>
              <td colspan="6" class="py-12 text-center text-slate-500 font-sans">
                <span class="mdi mdi-loading mdi-spin text-2xl text-cyan-500"></span>
                <div class="mt-2 text-xs">{$_("common.loading")}</div>
              </td>
            </tr>
          {:else if paginatedData.length === 0}
            <tr>
              <td colspan="6" class="py-12 text-center text-slate-500 font-sans">
                <span class="mdi mdi-chart-bell-curve-cumulative text-3xl text-slate-400 dark:text-slate-600"></span>
                <div class="mt-2 text-xs">
                  {$_("report.aiNoDataDesc")}
                </div>
              </td>
            </tr>
          {:else}
            {#each paginatedData as item}
              <tr class="transition-colors hover:bg-slate-50 dark:hover:bg-slate-800/40 text-slate-800 dark:text-slate-200">
                <!-- Anomaly score with colored MDI emoticon icon -->
                <td class="py-1 px-2 whitespace-nowrap">
                  <span
                    class="mdi {getScoreIcon(item.Score)} text-sm"
                    style="color: {getScoreColor(item.Score)};"
                  ></span>
                  <span class="ml-2 font-mono font-medium">
                    {item.Score.toFixed(2)}
                  </span>
                </td>

                <!-- Node Name -->
                <td class="py-1 px-2 font-sans truncate max-w-[200px]" title={item.Node}>
                  {item.Node}
                </td>

                <!-- Polling -->
                <td class="py-1 px-2 font-sans truncate max-w-[200px]" title={item.Polling}>
                  {item.Polling}
                </td>

                <!-- Count -->
                <td class="py-1 px-2 font-mono">
                  {item.Count}
                </td>

                <!-- Last time -->
                <td class="py-1 px-2 font-mono whitespace-nowrap text-slate-500 dark:text-slate-400 text-[11px]">
                  {formatTimeStr(item.LastTime > 1e11 ? item.LastTime : item.LastTime * 1000)}
                </td>

                <!-- Row Actions (Icon Buttons) -->
                <td class="py-0 px-1 text-center whitespace-nowrap">
                  <div class="inline-flex items-center gap-1.5 justify-center">
                    <button
                      type="button"
                      onclick={() => handleOpenReport(item)}
                      class="p-1 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-800 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-50 dark:hover:bg-cyan-950/40 transition-colors cursor-pointer"
                      title={$_("AIList.Report")}
                    >
                      <BarChart3 class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => handleExportAIData(item)}
                      class="p-1 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-800 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-950/40 transition-colors cursor-pointer"
                      title={$_("AIList.Csv")}
                    >
                      <Download class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => handleClearItem(item)}
                      class="p-1 rounded-lg border border-rose-200 dark:border-rose-900/60 bg-white dark:bg-slate-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 transition-colors cursor-pointer"
                      title={$_("AIList.Clear")}
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

    <!-- Pagination -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={filteredData.length}
    />
  </div>
</div>

<AIReportModal
  bind:show={showReportModal}
  id={selectedReportId}
  title={selectedReportTitle}
/>
