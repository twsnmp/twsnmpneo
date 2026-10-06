<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    Copy,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import {
    deleteDrawItem,
    copyDrawItem,
    type DrawItemEnt,
    type NodeEnt,
    type PollingEnt,
  } from "../../api";
  import DrawItemDialog from "../../components/DrawItemDialog.svelte";
  import ListPagination from "./ListPagination.svelte";
  import {
    getNodeName,
    isDrawItemOffscreen,
    getDrawItemTypeName,
    getDrawItemIcon,
    compareSortValues,
  } from "./listUtils";
  import type { SortDirection } from "./types";

  let {
    drawItems = [],
    nodes = [],
    pollings = [],
    loading = false,
    onRefresh = () => {},
  }: {
    drawItems?: DrawItemEnt[];
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    loading?: boolean;
    onRefresh?: () => void | Promise<void>;
  } = $props();

  let searchQuery = $state("");
  let typeFilter = $state("all");
  let currentPage = $state(1);
  let pageSize = $state(25);
  let sortColumn = $state<string | null>(null);
  let sortDirection = $state<SortDirection>("desc");

  // Dialog states
  let showDrawItemDialog = $state(false);
  let selectedDrawItem = $state<DrawItemEnt | null>(null);

  const filteredDrawItems = $derived(
    drawItems.filter((d) => {
      const q = searchQuery.toLowerCase();
      const text = (d.text || (d as any).Text || "").toLowerCase();
      const matchSearch = !q || text.includes(q);
      const type = d.type ?? (d as any).Type ?? 2;
      const matchType = typeFilter === "all" || String(type) === typeFilter;
      return matchSearch && matchType;
    })
  );

  const getDrawItemSortValue = (item: DrawItemEnt, column: string): unknown => {
    switch (column) {
      case "itemType": return getDrawItemTypeName(item.type ?? (item as any).Type ?? 2, $_);
      case "itemText": return item.text || (item as any).Text || "";
      case "bindInfo": return getNodeName(item.node_id || (item as any).NodeID, nodes);
      case "coords": return [item.x ?? (item as any).X ?? 0, item.y ?? (item as any).Y ?? 0];
      case "size": return [item.w ?? (item as any).W ?? 0, item.h ?? (item as any).H ?? 0];
      case "color": return item.color || (item as any).Color || "";
    }
    return "";
  };

  const sortedDrawItems = $derived.by(() => {
    if (!sortColumn) return filteredDrawItems;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filteredDrawItems].sort((a, b) => {
      return direction * compareSortValues(
        getDrawItemSortValue(a, sortColumn!),
        getDrawItemSortValue(b, sortColumn!)
      );
    });
  });

  const paginatedDrawItems = $derived.by(() => {
    if (pageSize === -1) return sortedDrawItems;
    const totalPages = Math.max(1, Math.ceil(sortedDrawItems.length / pageSize));
    const page = Math.min(currentPage, totalPages);
    const start = (page - 1) * pageSize;
    return sortedDrawItems.slice(start, start + pageSize);
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
    selectedDrawItem = null;
    showDrawItemDialog = true;
  };

  const handleEditDrawItem = (d: DrawItemEnt) => {
    selectedDrawItem = { ...d };
    showDrawItemDialog = true;
  };

  const handleCopyDrawItem = async (id: string) => {
    try {
      await copyDrawItem(id);
      onRefresh();
    } catch (e) {
      console.error("Failed to copy draw item:", e);
    }
  };

  const handleDeleteDrawItem = async (id: string) => {
    if (confirm($_('list.confirmDelete.drawItem'))) {
      await deleteDrawItem(id);
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
          placeholder={$_('list.search.drawitems')}
          bind:value={searchQuery}
          oninput={() => (currentPage = 1)}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

      <select
        bind:value={typeFilter}
        onchange={() => (currentPage = 1)}
        class="rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
      >
        <option value="all">{$_('list.filter.allItemTypes')} ({drawItems.length})</option>
        <option value="2">{$_('drawItem.types.2')}</option>
        <option value="4">{$_('drawItem.types.4')}</option>
        <option value="6">{$_('drawItem.types.6')}</option>
        <option value="7">{$_('drawItem.types.7')}</option>
        <option value="8">{$_('drawItem.types.8')}</option>
        <option value="11">{$_('drawItem.types.11')}</option>
      </select>
    </div>

    <div class="flex items-center gap-2">
      <button
        onclick={handleOpenAdd}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
      >
        <Plus class="h-4 w-4" />
        <span>{$_('list.addBtn.drawitems')}</span>
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
            {@render sortableHeader("itemType", $_('list.table.itemType'), "w-44")}
            {@render sortableHeader("itemText", $_('list.table.itemText'))}
            {@render sortableHeader("bindInfo", $_('list.table.bindInfo'))}
            {@render sortableHeader("coords", $_('list.table.coords'), "w-32")}
            {@render sortableHeader("size", $_('list.table.size'), "w-32")}
            {@render sortableHeader("color", $_('list.table.color'), "w-28")}
            <th class="py-1 px-2 text-right w-24">{$_('list.table.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
          {#each paginatedDrawItems as d}
            {@const type = d.type ?? (d as any).Type ?? 2}
            {@const IconComp = getDrawItemIcon(type)}
            {@const offscreen = isDrawItemOffscreen(d)}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors {offscreen ? 'bg-amber-500/10 dark:bg-amber-950/20' : ''}">
              <td class="py-1 px-2 font-sans font-semibold text-slate-800 dark:text-slate-200 flex items-center gap-2">
                <IconComp class="h-4 w-4 text-cyan-600 dark:text-cyan-400 shrink-0" />
                <span>{getDrawItemTypeName(type, $_)}</span>
              </td>
              <td class="py-1 px-2 text-slate-900 dark:text-slate-100 font-sans font-medium">
                {d.text || (d as any).Text || "-"}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 font-sans text-[11px]">
                {#if d.node_id}
                  <span>{$_('list.categories.nodes')}: {getNodeName(d.node_id, nodes)}</span>
                {:else}
                  <span>-</span>
                {/if}
              </td>
              <td class="py-1 px-2 text-[11px]">
                <div class="flex items-center gap-1.5 text-slate-600 dark:text-slate-300">
                  <span>({d.x ?? 0}, {d.y ?? 0})</span>
                  {#if offscreen}
                    <span class="rounded bg-amber-500/10 border border-amber-500/30 px-1 py-0.2 text-[9px] font-bold text-amber-600 dark:text-amber-400">
                      {$_('list.table.offscreen')}
                    </span>
                  {/if}
                </div>
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px]">
                {d.w ?? 0} × {d.h ?? 0}
              </td>
              <td class="py-1 px-2">
                <div class="flex items-center gap-1.5">
                  <span class="h-3 w-3 rounded-full border border-slate-300 dark:border-slate-700" style="background-color: {d.color || '#06b6d4'}"></span>
                  <span class="text-[10px] text-slate-500 dark:text-slate-400">{d.color || "#06b6d4"}</span>
                </div>
              </td>
              <td class="py-1 px-2 text-right font-sans">
                <div class="flex items-center justify-end gap-1">
                  <button
                    onclick={() => handleEditDrawItem(d)}
                    class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                    title={$_('common.edit')}
                  >
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleCopyDrawItem(d.id)}
                    class="rounded-lg p-1.5 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-950/40 hover:text-indigo-700 dark:hover:text-indigo-300 transition-colors cursor-pointer"
                    title={$_('common.copy') || 'コピー'}
                  >
                    <Copy class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleDeleteDrawItem(d.id)}
                    class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                    title={$_('common.delete')}
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredDrawItems.length === 0}
            <tr>
              <td colspan="7" class="py-12 text-center text-slate-400 dark:text-slate-500 font-sans">
                {$_('list.empty.drawitems')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <ListPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedDrawItems.length}
    />
  </div>
</div>

<!-- Modal Components -->
<DrawItemDialog
  bind:show={showDrawItemDialog}
  bind:item={selectedDrawItem}
  {nodes}
  {pollings}
  onSave={onRefresh}
/>
