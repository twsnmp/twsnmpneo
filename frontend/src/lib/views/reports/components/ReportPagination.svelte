<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
  } from "@lucide/svelte";

  let {
    pageSize = $bindable(25),
    currentPage = $bindable(1),
    totalCount = 0,
    pageSizes = [10, 25, 50, 100, 250, -1],
  }: {
    pageSize?: number;
    currentPage?: number;
    totalCount?: number;
    pageSizes?: number[];
  } = $props();

  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(totalCount / pageSize))
  );

  const rangeFrom = $derived(
    (currentPage - 1) * (pageSize === -1 ? totalCount : pageSize) + 1
  );

  const rangeTo = $derived(
    pageSize === -1 ? totalCount : Math.min(currentPage * pageSize, totalCount)
  );

  const handlePageSizeChange = () => {
    currentPage = 1;
  };
</script>

<div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/80 px-4 py-2 text-xs text-slate-600 dark:text-slate-400 shrink-0">
  <div class="flex items-center gap-3">
    <span>{$_("report.pageShowCount")}</span>
    <select
      bind:value={pageSize}
      onchange={handlePageSizeChange}
      class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-700 dark:text-slate-200 focus:outline-none cursor-pointer"
    >
      {#each pageSizes as size}
        {#if size === -1}
          <option value={-1}>{$_("report.showAll")}</option>
        {:else}
          <option value={size}>{$_("report.itemsPerPage", { values: { count: size } })}</option>
        {/if}
      {/each}
    </select>

    <span class="font-mono text-[11px] text-slate-400">
      {#if totalCount > 0}
        {$_("report.paginationRange", { values: { total: totalCount.toLocaleString(), from: rangeFrom, to: rangeTo } })}
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
