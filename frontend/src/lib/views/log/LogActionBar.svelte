<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Search,
    X,
    Filter,
    Columns,
    ChevronDown,
    Check,
    BarChart3,
    Trash2,
    Download,
    FileText,
    RefreshCw,
  } from "@lucide/svelte";
  import type { ColumnDef, FilterState, LogCategory } from "./types";

  interface Props {
    activeTab: LogCategory;
    searchQuery: string;
    filterState: FilterState;
    currentColumns: ColumnDef[];
    columnVisibility: Record<string, boolean>;
    sflowCounter: boolean;
    loading: boolean;
    onOpenFilter: () => void;
    onClearFilter?: () => void;
    onToggleColumn: (key: string) => void;
    onSflowCounterToggle: () => void;
    onOpenReport: () => void;
    onDeleteAll: () => void;
    onExportCSV: () => void;
    onExportExcel: () => void;
    onRefresh: () => void;
  }

  let {
    activeTab,
    searchQuery = $bindable(""),
    filterState,
    currentColumns,
    columnVisibility,
    sflowCounter = $bindable(false),
    loading = false,
    onOpenFilter,
    onClearFilter,
    onToggleColumn,
    onSflowCounterToggle,
    onOpenReport,
    onDeleteAll,
    onExportCSV,
    onExportExcel,
    onRefresh,
  }: Props = $props();

  let showColumnMenu = $state(false);

  const hasActiveFilters = $derived(
    Boolean(
      filterState.start ||
      filterState.end ||
      (filterState.level && filterState.level !== "all") ||
      filterState.type ||
      filterState.source ||
      filterState.keyword ||
      filterState.srcPort ||
      filterState.dstAddr ||
      filterState.dstPort ||
      filterState.protocol ||
      filterState.tcpFlags ||
      filterState.mac ||
      filterState.state
    )
  );

  const currentTabKey = $derived(
    activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab
  );
</script>

<div
  class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 p-3 shadow-sm dark:shadow-md shrink-0 transition-colors"
>
  <!-- Search & Filters -->
  <div class="flex items-center gap-2.5">
    <!-- Quick Text Search Input -->
    <div class="relative w-64">
      <Search class="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
      <input
        type="text"
        placeholder={$_('log.searchPlaceholder')}
        bind:value={searchQuery}
        class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-8 pr-7 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
      />
      {#if searchQuery}
        <button
          type="button"
          onclick={() => (searchQuery = "")}
          class="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-700 dark:hover:text-white cursor-pointer"
        >
          <X class="h-3.5 w-3.5" />
        </button>
      {/if}
    </div>

    <!-- Filter Modal Button & Active Indicator -->
    <div class="flex items-center gap-1">
      <button
        type="button"
        onclick={onOpenFilter}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer {hasActiveFilters ? 'border-cyan-500/50 bg-cyan-50/50 dark:bg-cyan-950/30 text-cyan-700 dark:text-cyan-300' : ''}"
      >
        <Filter class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
        <span>{$_('log.filterBtn')}</span>
        {#if hasActiveFilters}
          <span class="flex h-2 w-2 relative">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-cyan-500"></span>
          </span>
        {/if}
      </button>

      {#if hasActiveFilters && onClearFilter}
        <button
          type="button"
          title={$_('log.filterModal.clearActive')}
          onclick={onClearFilter}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-rose-100 dark:bg-slate-800 dark:hover:bg-rose-950/40 p-1.5 text-slate-400 hover:text-rose-600 dark:hover:text-rose-400 transition-colors cursor-pointer"
        >
          <X class="h-3.5 w-3.5" />
        </button>
      {/if}
    </div>

    <!-- Column Visibility Toggle Dropdown -->
    <div class="relative">
      <button
        type="button"
        onclick={() => (showColumnMenu = !showColumnMenu)}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
      >
        <Columns class="h-3.5 w-3.5 text-slate-400" />
        <span>{$_('log.columnsBtn')}</span>
        <ChevronDown class="h-3 w-3 text-slate-400" />
      </button>

      {#if showColumnMenu}
        <div
          class="absolute left-0 mt-2 z-30 w-48 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 p-2 shadow-xl space-y-1"
        >
          <div class="text-[10px] font-bold text-slate-500 dark:text-slate-400 px-2 py-1 uppercase">
            {$_('log.columnsBtn')}
          </div>
          {#each currentColumns as col}
            {@const isVis = columnVisibility[`${currentTabKey}_${col.key}`] !== false}
            <button
              type="button"
              onclick={() => onToggleColumn(col.key)}
              class="flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-xs text-left transition-colors cursor-pointer {isVis
                ? 'text-cyan-700 dark:text-cyan-300 bg-cyan-50 dark:bg-cyan-950/40'
                : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900'}"
            >
              <span>{col.label}</span>
              {#if isVis}
                <Check class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  <!-- Action Buttons -->
  <div class="flex items-center gap-2">
    {#if activeTab === "sflow"}
      <label
        class="flex items-center gap-2 cursor-pointer select-none bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700/80 px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 transition-colors"
      >
        <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
          {sflowCounter ? $_('log.sflowCounter') : $_('log.sflowFlow')}
        </span>
        <input
          type="checkbox"
          bind:checked={sflowCounter}
          onchange={onSflowCounterToggle}
          class="sr-only peer"
        />
        <div
          class="relative w-8 h-4 bg-slate-300 dark:bg-slate-600 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-3 after:w-3 after:transition-all peer-checked:bg-cyan-500"
        ></div>
      </label>
    {/if}

    <button
      type="button"
      onclick={onOpenReport}
      class="flex items-center gap-1.5 rounded-xl border border-emerald-500/40 bg-emerald-500/10 hover:bg-emerald-500/20 px-3.5 py-1.5 text-xs font-bold text-emerald-600 dark:text-emerald-300 transition-all cursor-pointer"
    >
      <BarChart3 class="h-3.5 w-3.5" />
      <span>{$_('log.reportBtn')}</span>
    </button>

    <button
      type="button"
      onclick={onDeleteAll}
      title={$_('log.deleteBtn')}
      class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-1.5 text-xs font-semibold text-rose-600 dark:text-rose-300 transition-colors cursor-pointer"
    >
      <Trash2 class="h-3.5 w-3.5" />
      <span>{$_('log.deleteBtn')}</span>
    </button>

    <button
      type="button"
      onclick={onExportCSV}
      class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
    >
      <Download class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
      <span>CSV</span>
    </button>

    <button
      type="button"
      onclick={onExportExcel}
      class="flex items-center gap-1.5 rounded-xl border border-emerald-500/30 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/40 px-3 py-1.5 text-xs font-semibold text-emerald-700 dark:text-emerald-300 transition-colors cursor-pointer"
    >
      <FileText class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
      <span>Excel</span>
    </button>

    <button
      type="button"
      onclick={onRefresh}
      disabled={loading}
      class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer {loading ? 'opacity-80 cursor-wait' : ''}"
    >
      <RefreshCw class="h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
      <span>{loading ? $_('log.searching') : $_('common.refresh')}</span>
    </button>
  </div>
</div>
