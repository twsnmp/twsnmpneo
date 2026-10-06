<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    AlertTriangle,
    CheckCircle2,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import { deleteLine, type LineEnt, type NodeEnt, type NetworkEnt, type PollingEnt } from "../../api";
  import { getStateColor, getStateName } from "../../common";
  import LineDialog from "../../components/LineDialog.svelte";
  import ListPagination from "./ListPagination.svelte";
  import { getStatusBadge, getTargetLabel, isLineOrphaned, compareSortValues } from "./listUtils";
  import type { SortDirection } from "./types";

  let {
    lines = [],
    nodes = [],
    networks = [],
    pollings = [],
    loading = false,
    onRefresh = () => {},
  }: {
    lines?: LineEnt[];
    nodes?: NodeEnt[];
    networks?: NetworkEnt[];
    pollings?: PollingEnt[];
    loading?: boolean;
    onRefresh?: () => void | Promise<void>;
  } = $props();

  let searchQuery = $state("");
  let currentPage = $state(1);
  let pageSize = $state(25);
  let sortColumn = $state<string | null>(null);
  let sortDirection = $state<SortDirection>("desc");

  // Dialog states
  let showLineDialog = $state(false);
  let selectedLine = $state<LineEnt | null>(null);

  const filteredLines = $derived(
    lines.filter((l) => {
      const q = searchQuery.toLowerCase();
      const t1 = getTargetLabel(l.node_id1 || (l as any).NodeID1, nodes, networks).toLowerCase();
      const t2 = getTargetLabel(l.node_id2 || (l as any).NodeID2, nodes, networks).toLowerCase();
      const info = (l.info || (l as any).Info || "").toLowerCase();
      return !q || t1.includes(q) || t2.includes(q) || info.includes(q);
    })
  );

  const getLineSortValue = (item: LineEnt, column: string): unknown => {
    switch (column) {
      case "status": return item.state || (item as any).State || "";
      case "source1": return getTargetLabel(item.node_id1 || (item as any).NodeID1, nodes, networks);
      case "target2": return getTargetLabel(item.node_id2 || (item as any).NodeID2, nodes, networks);
      case "width": return item.width ?? (item as any).Width ?? 0;
      case "infoPort": return item.info || (item as any).Info || (item as any).port || "";
      case "health": return isLineOrphaned(item, nodes, networks) ? 0 : 1;
    }
    return "";
  };

  const sortedLines = $derived.by(() => {
    if (!sortColumn) return filteredLines;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filteredLines].sort((a, b) => {
      return direction * compareSortValues(
        getLineSortValue(a, sortColumn!),
        getLineSortValue(b, sortColumn!)
      );
    });
  });

  const paginatedLines = $derived.by(() => {
    if (pageSize === -1) return sortedLines;
    const totalPages = Math.max(1, Math.ceil(sortedLines.length / pageSize));
    const page = Math.min(currentPage, totalPages);
    const start = (page - 1) * pageSize;
    return sortedLines.slice(start, start + pageSize);
  });

  const handleSort = (column: string) => {
    if (sortColumn === column) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = column;
      sortDirection = "desc";
    }
  };

  const handleOpenAdd = () => {
    selectedLine = null;
    showLineDialog = true;
  };

  const handleEditLine = (l: LineEnt) => {
    selectedLine = { ...l };
    showLineDialog = true;
  };

  const handleDeleteLine = async (id: string) => {
    if (confirm($_('list.confirmDelete.line'))) {
      await deleteLine(id);
      onRefresh();
    }
  };
</script>

{#snippet sortableHeader(column: string, label: string, classes = "")}
  <th
    class="py-2.5 px-3.5 {classes}"
    aria-sort={sortColumn === column
      ? sortDirection === "asc" ? "ascending" : "descending"
      : "none"}
  >
    <button
      type="button"
      class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
      onclick={() => handleSort(column)}
    >
      <span>{label}</span>
      {#if sortColumn === column}
        {#if sortDirection === "asc"}
          <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
        {:else}
          <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
        {/if}
      {:else}
        <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
      {/if}
    </button>
  </th>
{/snippet}

<div class="flex-1 overflow-hidden flex flex-col gap-4 min-w-0">
  <!-- Top Action Bar -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg shrink-0 transition-colors">
    <div class="flex flex-wrap items-center gap-3">
      <!-- Search -->
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder={$_('list.search.lines')}
          bind:value={searchQuery}
          oninput={() => (currentPage = 1)}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>
    </div>

    <div class="flex items-center gap-2">
      <button
        onclick={handleOpenAdd}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
      >
        <Plus class="h-4 w-4" />
        <span>{$_('list.addBtn.lines')}</span>
      </button>

      <button
        onclick={onRefresh}
        disabled={loading}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-all cursor-pointer"
      >
        <RefreshCw class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 {loading ? 'animate-spin' : ''}" />
        <span>{$_('common.refresh')}</span>
      </button>
    </div>
  </div>

  <!-- Table Container -->
  <div class="flex-1 overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 shadow-sm dark:shadow-lg flex flex-col min-h-0 transition-colors">
    <div class="flex-1 min-h-0 overflow-y-auto overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="sticky top-0 z-10 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-[10px] font-semibold uppercase text-slate-500 dark:text-slate-400">
          <tr>
            {@render sortableHeader("status", $_('list.table.status'), "w-28")}
            {@render sortableHeader("source1", $_('list.table.source1'))}
            {@render sortableHeader("target2", $_('list.table.target2'))}
            {@render sortableHeader("width", $_('list.table.width'), "w-24")}
            {@render sortableHeader("infoPort", $_('list.table.infoPort'))}
            {@render sortableHeader("health", $_('list.table.health'), "w-48")}
            <th class="py-1 px-2 text-right w-24">{$_('list.table.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
          {#each paginatedLines as l}
            {@const orphaned = isLineOrphaned(l, nodes, networks)}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors {orphaned ? 'bg-amber-500/10 dark:bg-amber-950/20' : ''}">
              <td class="py-1 px-2">
                <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(l.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(l.state)}"></span>
                  {getStateName(l.state, $_)}
                </span>
              </td>
              <td class="py-1 px-2 font-sans font-medium text-slate-800 dark:text-slate-200">
                {getTargetLabel(l.node_id1 || (l as any).NodeID1, nodes, networks)}
              </td>
              <td class="py-1 px-2 font-sans font-medium text-slate-800 dark:text-slate-200">
                {getTargetLabel(l.node_id2 || (l as any).NodeID2, nodes, networks)}
              </td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{l.width} px</td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-sans">
                {l.info || (l as any).port || "-"}
              </td>
              <td class="py-1 px-2 font-sans">
                {#if orphaned}
                  <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30">
                    <AlertTriangle class="h-3 w-3" />
                    {$_('list.table.orphaned')}
                  </span>
                {:else}
                  <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                    <CheckCircle2 class="h-3 w-3" />
                    {$_('list.table.healthy')}
                  </span>
                {/if}
              </td>
              <td class="py-0 px-1 text-right font-sans">
                <div class="flex items-center justify-end">
                  <button
                    onclick={() => handleEditLine(l)}
                    class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                    title={$_('common.edit')}
                  >
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleDeleteLine(l.id)}
                    class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                    title={$_('common.delete')}
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredLines.length === 0}
            <tr>
              <td colspan="7" class="py-12 text-center text-slate-400 dark:text-slate-500 font-sans">
                {$_('list.empty.lines')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <ListPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedLines.length}
    />
  </div>
</div>

<!-- Modal Components -->
<LineDialog
  bind:show={showLineDialog}
  bind:line={selectedLine}
  {nodes}
  {networks}
  {pollings}
  onSave={onRefresh}
  onDelete={onRefresh}
/>
