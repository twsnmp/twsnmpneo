<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    ArrowDown,
    ArrowUp,
    ArrowUpDown,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    Sparkles,
  } from "@lucide/svelte";
  import { formatTimeStr, renderTimeMili, renderBytes, getStateColor } from "../../common";
  import { getLevelBadge, formatCounterData } from "./logUtils";
  import type { ColumnDef, LogCategory, LogItem } from "./types";

  interface Props {
    activeTab: LogCategory;
    visibleColumns: ColumnDef[];
    paginatedLogs: LogItem[];
    totalLogsCount: number;
    loading: boolean;
    sortColumn: string;
    sortDirection: "asc" | "desc";
    currentPage: number;
    pageSize: number;
    totalPages: number;
    onSort: (colKey: string) => void;
    onAskAI: (logText: string) => void;
    onPageChange: (page: number) => void;
    onPageSizeChange: (size: number) => void;
  }

  let {
    activeTab,
    visibleColumns,
    paginatedLogs,
    totalLogsCount,
    loading,
    sortColumn = $bindable("time"),
    sortDirection = $bindable("desc"),
    currentPage = $bindable(1),
    pageSize = $bindable(25),
    totalPages,
    onSort,
    onAskAI,
    onPageChange,
    onPageSizeChange,
  }: Props = $props();

  const handlePageSizeChange = (event: Event) => {
    const val = parseInt((event.target as HTMLSelectElement).value, 10);
    pageSize = val;
    currentPage = 1;
    onPageSizeChange(val);
  };
</script>

<div
  class="flex-1 overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 shadow-sm dark:shadow-lg flex flex-col min-h-0 transition-colors"
>
  <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
    <table class="w-full text-left text-xs">
      <thead
        class="sticky top-0 z-10 border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-[10px] font-semibold uppercase text-slate-500 dark:text-slate-400"
      >
        <tr>
          {#each visibleColumns as col}
            <th
              class="py-1 px-2 {col.width || ''} {col.align === 'center'
                ? 'text-center'
                : col.align === 'right'
                  ? 'text-right'
                  : 'text-left'} {col.sortable
                ? 'cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200'
                : ''}"
              onclick={() => col.sortable && onSort(col.key)}
            >
              <div class="inline-flex items-center gap-1">
                <span>{col.label}</span>
                {#if col.sortable}
                  {#if sortColumn === col.key}
                    {#if sortDirection === "asc"}
                      <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                    {:else}
                      <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                    {/if}
                  {:else}
                    <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                  {/if}
                {/if}
              </div>
            </th>
          {/each}
          <th class="py-1 px-2 text-center w-10">AI</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/40 font-mono text-slate-700 dark:text-slate-300">
        {#each paginatedLogs as item}
          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
            {#each visibleColumns as col}
              <td class="py-1 px-2 {col.align === 'center' ? 'text-center' : col.align === 'right' ? 'text-right' : 'text-left'}">
                {#if col.key === "time"}
                  <span class="text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap leading-tight">
                    {activeTab === "syslog" || activeTab === "netflow" || activeTab === "sflow"
                      ? renderTimeMili(item.time)
                      : formatTimeStr(item.time)}
                  </span>
                {:else if col.key === "level" || col.key === "state"}
                  <span
                    class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {getLevelBadge(item.level || item.state || 'info')}"
                  >
                    <span
                      class="h-1.5 w-1.5 rounded-full shrink-0"
                      style="background-color: {getStateColor(item.level || item.state || 'info')}"
                    ></span>
                    {item.level || item.state || 'info'}
                  </span>
                {:else if col.key === "bytes" && typeof item.bytes === "number"}
                  <span class="font-sans text-[11px] text-slate-700 dark:text-slate-300 leading-tight">
                    {renderBytes(item.bytes)}
                  </span>
                {:else if col.key === "packets" && typeof item.packets === "number"}
                  <span class="text-[11px] text-slate-700 dark:text-slate-300 leading-tight">
                    {item.packets.toLocaleString()}
                  </span>
                {:else if col.key === "dur"}
                  <span class="text-[11px] text-slate-700 dark:text-slate-300 leading-tight">
                    {typeof item.dur === 'number' ? (item.dur === 0 ? '0' : item.dur.toFixed(2)) : (item.dur || '0')}
                  </span>
                {:else if col.key === "reason"}
                  <span class="text-slate-700 dark:text-slate-300 text-[11px] font-sans leading-tight">
                    {item.reason ? item.reason : ""}
                  </span>
                {:else if col.key === "srcLoc" || col.key === "dstLoc" || col.key === "srcMac" || col.key === "dstMac" || col.key === "tcpFlags"}
                  <span class="text-slate-500 dark:text-slate-400 text-[11px] font-sans truncate leading-tight">
                    {item[col.key] || ""}
                  </span>
                {:else if col.key === "srcPort" || col.key === "dstPort"}
                  <span class="text-slate-700 dark:text-slate-300 text-[11px] font-sans leading-tight">
                    {item[col.key] || 0}
                  </span>
                {:else if col.key === "srcAddr" || col.key === "dstAddr" || col.key === "remote"}
                  <span class="font-sans text-slate-800 dark:text-slate-200 text-[11px] truncate leading-tight">
                    {item[col.key] || "-"}
                  </span>
                {:else if col.key === "counterType"}
                  <span class="font-semibold text-cyan-600 dark:text-cyan-400 text-[11px] truncate leading-tight">
                    {item.counterType || "-"}
                  </span>
                {:else if col.key === "counterData"}
                  <span class="font-sans text-slate-800 dark:text-slate-200 break-all text-[11px] leading-relaxed select-text">
                    {formatCounterData(item.counterData)}
                  </span>
                {:else if col.key === "event" || col.key === "message" || col.key === "payload" || col.key === "log"}
                  <span class="font-sans text-slate-800 dark:text-slate-100 break-all text-[11px] leading-tight line-clamp-1">
                    {item[col.key] || "-"}
                  </span>
                {:else if col.key === "node" || col.key === "host" || col.key === "src" || col.key === "ip"}
                  <span class="font-semibold text-cyan-600 dark:text-cyan-400 text-[11px] truncate leading-tight">
                    {item[col.key] || "-"}
                  </span>
                {:else}
                  <span class="text-slate-800 dark:text-slate-200 text-[11px] font-sans truncate leading-tight">
                    {item[col.key] || "-"}
                  </span>
                {/if}
              </td>
            {/each}
            <td class="py-1 px-2 text-center font-sans">
              <button
                type="button"
                onclick={() => onAskAI(item.fullText)}
                title={$_('log.aiDiagnosis')}
                aria-label={$_('log.aiDiagnosis')}
                class="inline-flex items-center justify-center rounded border border-cyan-500/30 bg-cyan-500/10 p-0.5 text-cyan-600 dark:text-cyan-300 hover:bg-cyan-500/20 hover:text-cyan-700 dark:hover:text-cyan-200 transition-all cursor-pointer"
              >
                <Sparkles class="h-3 w-3" />
              </button>
            </td>
          </tr>
        {/each}

        {#if paginatedLogs.length === 0}
          <tr>
            <td colspan={visibleColumns.length + 1} class="py-16 text-center text-slate-400 dark:text-slate-500 font-sans">
              {loading ? $_('common.loading') : $_('log.noLogs')}
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Pagination Footer -->
  <div
    class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/80 px-4 py-2.5 text-xs text-slate-500 dark:text-slate-400 shrink-0"
  >
    <div class="flex items-center gap-3">
      <span>{$_('log.itemsPerPage')}:</span>
      <select
        value={pageSize}
        onchange={handlePageSizeChange}
        class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-1 text-xs text-slate-800 dark:text-slate-200 focus:outline-none cursor-pointer"
      >
        <option value={10}>10 / {$_('log.page')}</option>
        <option value={25}>25 / {$_('log.page')}</option>
        <option value={50}>50 / {$_('log.page')}</option>
        <option value={100}>100 / {$_('log.page')}</option>
        <option value={250}>250 / {$_('log.page')}</option>
        <option value={-1}>All</option>
      </select>

      <span class="font-mono text-[11px] text-slate-500 dark:text-slate-400">
        {#if totalLogsCount > 0}
          {totalLogsCount.toLocaleString()} {$_('log.recordsUnit')} ({(currentPage - 1) * pageSize + 1} - {pageSize === -1
            ? totalLogsCount
            : Math.min(currentPage * pageSize, totalLogsCount)})
        {:else}
          0 {$_('log.recordsUnit')}
        {/if}
      </span>
    </div>

    {#if pageSize !== -1 && totalPages > 1}
      <div class="flex items-center gap-1">
        <button
          type="button"
          disabled={currentPage <= 1}
          onclick={() => onPageChange(1)}
          class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
          title={$_('log.firstPage')}
        >
          <ChevronsLeft class="h-4 w-4" />
        </button>

        <button
          type="button"
          disabled={currentPage <= 1}
          onclick={() => onPageChange(currentPage - 1)}
          class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
          title={$_('log.prevPage')}
        >
          <ChevronLeft class="h-4 w-4" />
        </button>

        <span class="px-2 font-mono text-xs text-slate-700 dark:text-slate-300">
          {currentPage} / {totalPages}
        </span>

        <button
          type="button"
          disabled={currentPage >= totalPages}
          onclick={() => onPageChange(currentPage + 1)}
          class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
          title={$_('log.nextPage')}
        >
          <ChevronRight class="h-4 w-4" />
        </button>

        <button
          type="button"
          disabled={currentPage >= totalPages}
          onclick={() => onPageChange(totalPages)}
          class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-white disabled:opacity-30 disabled:hover:bg-transparent cursor-pointer"
          title={$_('log.lastPage')}
        >
          <ChevronsRight class="h-4 w-4" />
        </button>
      </div>
    {/if}
  </div>
</div>
