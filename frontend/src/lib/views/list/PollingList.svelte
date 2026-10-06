<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    Eye,
    Layers,
    CheckCircle2,
    AlertTriangle,
    AlertCircle,
    FileText,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import { deletePolling, type PollingEnt, type NodeEnt } from "../../api";
  import {
    getStateColor,
    getStateName,
    formatTimeStr,
    getLogModeName,
    getLogModeBadgeClass,
  } from "../../common";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import PollingDetailModal from "../../components/PollingDetailModal.svelte";
  import PollingTemplateDialog from "../../components/PollingTemplateDialog.svelte";
  import ListPagination from "./ListPagination.svelte";
  import { getStatusBadge, getNodeName, compareSortValues } from "./listUtils";
  import type { SortDirection } from "./types";

  let {
    pollings = [],
    nodes = [],
    loading = false,
    onRefresh = () => {},
  }: {
    pollings?: PollingEnt[];
    nodes?: NodeEnt[];
    loading?: boolean;
    onRefresh?: () => void | Promise<void>;
  } = $props();

  let searchQuery = $state("");
  let statusFilter = $state("all");
  let typeFilter = $state("all");
  let currentPage = $state(1);
  let pageSize = $state(25);
  let sortColumn = $state<string | null>(null);
  let sortDirection = $state<SortDirection>("desc");

  // Dialog states
  let showPollingDialog = $state(false);
  let showTemplateDialog = $state(false);
  let showPollingDetailModal = $state(false);
  let selectedPolling = $state<PollingEnt | null>(null);
  let detailPolling = $state<PollingEnt | null>(null);
  let detailPollingNode = $state<NodeEnt | null>(null);

  const isPollingMatchStatus = (p: PollingEnt, filter: string) => {
    const st = (p.state || (p as any).State || "normal").toLowerCase();
    const logMode = p.log_mode ?? (p as any).LogMode ?? 0;
    if (filter === "all") return true;
    if (filter === "normal") return st === "normal" || st === "repair" || st === "info" || st === "up";
    if (filter === "warn") return st === "warn" || st === "warning";
    if (filter === "error" || filter === "problem") return st === "high" || st === "low" || st === "error" || st === "down";
    if (filter === "logging" || filter === "hasLog") return logMode !== 0;
    return st === filter;
  };

  const pollingStatusCounts = $derived.by(() => {
    let normal = 0;
    let warn = 0;
    let error = 0;
    let logging = 0;
    for (const p of pollings) {
      const st = (p.state || (p as any).State || "normal").toLowerCase();
      const logMode = p.log_mode ?? (p as any).LogMode ?? 0;
      if (st === "warn" || st === "warning") {
        warn++;
      } else if (st === "high" || st === "low" || st === "error" || st === "down") {
        error++;
      } else {
        normal++;
      }
      if (logMode !== 0) {
        logging++;
      }
    }
    return { all: pollings.length, normal, warn, error, logging };
  });

  const availablePollingTypes = $derived.by(() => {
    const typeSet = new Set<string>();
    for (const p of pollings) {
      const t = (p.type || (p as any).Type || "").trim();
      if (t) typeSet.add(t.toLowerCase());
    }
    const types = Array.from(typeSet).sort();
    return types.map((t) => ({ value: t, label: t.toUpperCase() }));
  });

  const filteredPollings = $derived(
    pollings.filter((p) => {
      const q = searchQuery.toLowerCase();
      const name = (p.name || (p as any).Name || "").toLowerCase();
      const target = (p.params || p.target || p.Params || (p as any).Target || "").toLowerCase();
      const type = (p.type || (p as any).Type || "").toLowerCase();
      const mode = (p.mode || (p as any).Mode || "").toLowerCase();
      const nodeName = getNodeName(p.node_id || (p as any).NodeID, nodes).toLowerCase();
      const matchSearch = !q || name.includes(q) || target.includes(q) || nodeName.includes(q) || type.includes(q) || mode.includes(q);
      const matchType = typeFilter === "all" || type === typeFilter.toLowerCase();
      const matchStatus = isPollingMatchStatus(p, statusFilter);
      return matchSearch && matchType && matchStatus;
    })
  );

  const getPollingSortValue = (item: PollingEnt, column: string): unknown => {
    switch (column) {
      case "status": return item.state || (item as any).State || "";
      case "name": return item.name || (item as any).Name || "";
      case "type": return item.type || (item as any).Type || "";
      case "mode": return item.mode || (item as any).Mode || "";
      case "logMode": return item.log_mode ?? (item as any).LogMode ?? 0;
      case "targetNode": return getNodeName(item.node_id || (item as any).NodeID, nodes);
      case "lastVal": return item.last_val ?? "";
      case "lastTime": return item.last_time ?? "";
    }
    return "";
  };

  const sortedPollings = $derived.by(() => {
    if (!sortColumn) return filteredPollings;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filteredPollings].sort((a, b) => {
      return direction * compareSortValues(
        getPollingSortValue(a, sortColumn!),
        getPollingSortValue(b, sortColumn!)
      );
    });
  });

  const paginatedPollings = $derived.by(() => {
    if (pageSize === -1) return sortedPollings;
    const totalPages = Math.max(1, Math.ceil(sortedPollings.length / pageSize));
    const page = Math.min(currentPage, totalPages);
    const start = (page - 1) * pageSize;
    return sortedPollings.slice(start, start + pageSize);
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
    selectedPolling = null;
    showTemplateDialog = true;
  };

  const handleViewPolling = (p: PollingEnt) => {
    detailPolling = p;
    detailPollingNode = nodes.find((n) => n.id === (p.node_id || (p as any).NodeID)) || null;
    showPollingDetailModal = true;
  };

  const handleEditPolling = (p: PollingEnt) => {
    selectedPolling = { ...p };
    showPollingDialog = true;
  };

  const handleSelectTemplate = (_template: any, prefilled?: Partial<PollingEnt>) => {
    selectedPolling = prefilled ? ({ ...prefilled } as PollingEnt) : null;
    showPollingDialog = true;
  };

  const handleDeletePolling = async (id: string) => {
    if (confirm($_('list.confirmDelete.polling'))) {
      await deletePolling(id);
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
          placeholder={$_('list.search.pollings')}
          bind:value={searchQuery}
          oninput={() => (currentPage = 1)}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

      <!-- Status & Type Filters -->
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onclick={() => { statusFilter = "all"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <Layers class="w-3.5 h-3.5" />
          <span>{$_('list.filter.allStatus')} ({pollingStatusCounts.all})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "normal"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'normal' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <CheckCircle2 class="w-3.5 h-3.5 {statusFilter === 'normal' ? 'text-white' : 'text-emerald-500'}" />
          <span>{$_('list.filter.normalRepair')} ({pollingStatusCounts.normal})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "warn"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'warn' ? 'bg-amber-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <AlertTriangle class="w-3.5 h-3.5 {statusFilter === 'warn' ? 'text-white' : 'text-amber-500'}" />
          <span>{$_('list.filter.warn')} ({pollingStatusCounts.warn})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "error"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'error' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <AlertCircle class="w-3.5 h-3.5 {statusFilter === 'error' ? 'text-white' : 'text-rose-500'}" />
          <span>{$_('list.filter.faultHeavyLight')} ({pollingStatusCounts.error})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "logging"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'logging' ? 'bg-indigo-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <FileText class="w-3.5 h-3.5 {statusFilter === 'logging' ? 'text-white' : 'text-indigo-500 dark:text-indigo-400'}" />
          <span>{$_('list.filter.loggingOnly')} ({pollingStatusCounts.logging})</span>
        </button>
      </div>

      <select
        bind:value={typeFilter}
        onchange={() => (currentPage = 1)}
        class="rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
      >
        <option value="all">{$_('list.filter.allPollingTypes')} ({pollings.length})</option>
        {#each availablePollingTypes as pt}
          <option value={pt.value}>{pt.label} ({pollings.filter((p) => (p.type || (p as any).Type || '').toLowerCase() === pt.value.toLowerCase()).length})</option>
        {/each}
      </select>
    </div>

    <div class="flex items-center gap-2">
      <button
        onclick={handleOpenAdd}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
      >
        <Plus class="h-4 w-4" />
        <span>{$_('list.addBtn.pollings')}</span>
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
            {@render sortableHeader("name", $_('list.table.pollingName'))}
            {@render sortableHeader("type", $_('list.table.type'), "w-24")}
            {@render sortableHeader("mode", $_('list.table.mode') || 'モード', "w-28")}
            {@render sortableHeader("logMode", $_('list.table.logMode'), "w-28")}
            {@render sortableHeader("targetNode", $_('list.table.targetNode'))}
            {@render sortableHeader("lastVal", $_('list.table.lastVal'), "w-32")}
            {@render sortableHeader("lastTime", $_('list.table.lastTime'), "w-40")}
            <th class="py-1 px-2 text-right w-24">{$_('list.table.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
          {#each paginatedPollings as p}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2">
                <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(p.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(p.state)}"></span>
                  {getStateName(p.state, $_)}
                </span>
              </td>
              <td class="py-1 px-2 font-bold text-slate-900 dark:text-slate-100 font-sans">{p.name}</td>
              <td class="py-1 px-2">
                <span class="rounded px-2 py-0.5 font-mono text-[10px] uppercase font-bold bg-cyan-500/10 text-cyan-600 dark:text-cyan-300 border border-cyan-500/30">
                  {p.type}
                </span>
              </td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-mono text-[11px] truncate max-w-[120px]">
                {p.mode || (p as any).Mode || "-"}
              </td>
              <td class="py-1 px-2 whitespace-nowrap">
                <span class="inline-flex items-center rounded-md px-2 py-0.5 text-[10px] font-semibold border {getLogModeBadgeClass(p.log_mode ?? (p as any).LogMode)}">
                  {getLogModeName(p.log_mode ?? (p as any).LogMode, $_)}
                </span>
              </td>
              <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-sans font-medium">{getNodeName(p.node_id || (p as any).NodeID, nodes)}</td>
              <td class="py-1 px-2 text-emerald-600 dark:text-emerald-400 font-semibold">
                {#if p.last_val !== undefined}
                  {['ping', 'tcp', 'http', 'https', 'dns', 'ntp'].includes((p.type || '').toLowerCase()) ? p.last_val.toFixed(2) + " ms" : p.last_val.toFixed(2)}
                {:else}
                  -
                {/if}
              </td>
              <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">
                {formatTimeStr(p.last_time)}
              </td>
              <td class="py-0 px-1 text-right font-sans">
                <div class="flex items-center justify-end">
                  <button
                    onclick={() => handleViewPolling(p)}
                    class="rounded-lg p-1.5 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-50 dark:hover:bg-cyan-950/40 hover:text-cyan-700 dark:hover:text-cyan-300 transition-colors cursor-pointer"
                    title={$_('common.view')}
                    aria-label={$_('common.view')}
                  >
                    <Eye class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleEditPolling(p)}
                    class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                    title={$_('common.edit')}
                    aria-label={$_('common.edit')}
                  >
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleDeletePolling(p.id)}
                    class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                    title={$_('common.delete')}
                    aria-label={$_('common.delete')}
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredPollings.length === 0}
            <tr>
              <td colspan="9" class="py-12 text-center text-slate-400 dark:text-slate-500 font-sans">
                {$_('list.empty.pollings')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <ListPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedPollings.length}
    />
  </div>
</div>

<!-- Modal Components -->
<PollingTemplateDialog
  bind:show={showTemplateDialog}
  {nodes}
  onSelect={handleSelectTemplate}
  onCreated={onRefresh}
/>

<PollingDialog
  bind:show={showPollingDialog}
  bind:polling={selectedPolling}
  {nodes}
  onSave={onRefresh}
/>

<PollingDetailModal
  bind:show={showPollingDetailModal}
  polling={detailPolling}
  node={detailPollingNode}
  onEdit={(p) => {
    handleEditPolling(p);
  }}
/>
