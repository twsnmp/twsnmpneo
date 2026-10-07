<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Laptop,
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    Box,
    Layers,
    CheckCircle2,
    AlertTriangle,
    AlertCircle,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import { deleteNode, type NodeEnt, type PollingEnt } from "../../api";
  import { showConfirm } from "../../stores/modalStore";
  import { getStateColor, getStateName, isImageIcon, getIconImage, getIconCode } from "../../common";
  import { getVendor } from "../reports/utils";
  import NodeDialog from "../../components/NodeDialog.svelte";
  import NodeDetailModal from "../../components/NodeDetailModal.svelte";
  import ListPagination from "./ListPagination.svelte";
  import { getStatusBadge, compareSortValues } from "./listUtils";
  import type { SortDirection } from "./types";

  let {
    nodes = [],
    pollings = $bindable([]),
    loading = false,
    onRefresh = () => {},
  }: {
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    loading?: boolean;
    onRefresh?: () => void | Promise<void>;
  } = $props();

  let searchQuery = $state("");
  let statusFilter = $state("all");
  let currentPage = $state(1);
  let pageSize = $state(25);
  let sortColumn = $state<string | null>(null);
  let sortDirection = $state<SortDirection>("desc");

  // Dialog states
  let showNodeDialog = $state(false);
  let selectedNode = $state<NodeEnt | null>(null);
  let showDetailModal = $state(false);
  let detailNode = $state<NodeEnt | null>(null);

  const isNodeMatchStatus = (n: NodeEnt, filter: string) => {
    const st = (n.state || (n as any).State || "normal").toLowerCase();
    if (filter === "all") return true;
    if (filter === "normal") return st === "normal" || st === "repair" || st === "info" || st === "up";
    if (filter === "warn") return st === "warn" || st === "warning";
    if (filter === "error" || filter === "problem") return st === "high" || st === "low" || st === "error" || st === "down";
    return st === filter;
  };

  const nodeStatusCounts = $derived.by(() => {
    let normal = 0;
    let warn = 0;
    let error = 0;
    for (const n of nodes) {
      const st = (n.state || (n as any).State || "normal").toLowerCase();
      if (st === "warn" || st === "warning") {
        warn++;
      } else if (st === "high" || st === "low" || st === "error" || st === "down") {
        error++;
      } else {
        normal++;
      }
    }
    return { all: nodes.length, normal, warn, error };
  });

  const filteredNodes = $derived(
    nodes.filter((n) => {
      if (!n) return false;
      const q = searchQuery.toLowerCase();
      const name = (n.name || (n as any).Name || "").toLowerCase();
      const ip = (n.ip || (n as any).IP || "").toLowerCase();
      const mac = (n.mac || (n as any).MAC || "").toLowerCase();
      const vendor = (n.vendor || (n as any).Vendor || (n.mac ? getVendor(n.mac) : "") || "").toLowerCase();
      const descr = (n.descr || (n as any).Descr || "").toLowerCase();
      const matchSearch = !q || name.includes(q) || ip.includes(q) || mac.includes(q) || vendor.includes(q) || descr.includes(q);
      const matchStatus = isNodeMatchStatus(n, statusFilter);
      return matchSearch && matchStatus;
    })
  );

  const getNodeSortValue = (item: NodeEnt, column: string): unknown => {
    switch (column) {
      case "status": return item.state || (item as any).State || "";
      case "name": return item.name || (item as any).Name || "";
      case "ip": return item.ip || (item as any).IP || "";
      case "mac": return item.mac || (item as any).MAC || "";
      case "vendor": return item.vendor || (item as any).Vendor || (item.mac ? getVendor(item.mac) : "") || "";
      case "descr": return item.descr || (item as any).Descr || "";
    }
    return "";
  };

  const sortedNodes = $derived.by(() => {
    if (!sortColumn) return filteredNodes;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filteredNodes].sort((a, b) => {
      return direction * compareSortValues(
        getNodeSortValue(a, sortColumn!),
        getNodeSortValue(b, sortColumn!)
      );
    });
  });

  const paginatedNodes = $derived.by(() => {
    if (pageSize === -1) return sortedNodes;
    const totalPages = Math.max(1, Math.ceil(sortedNodes.length / pageSize));
    const page = Math.min(currentPage, totalPages);
    const start = (page - 1) * pageSize;
    return sortedNodes.slice(start, start + pageSize);
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
    selectedNode = {
      id: "",
      name: "",
      ip: "192.168.1.10",
      mac: "",
      descr: "",
      icon: "desktop",
      state: "normal",
      x: 320,
      y: 200,
    };
    showNodeDialog = true;
  };

  const handleEditNode = (n: NodeEnt) => {
    selectedNode = { ...n };
    showNodeDialog = true;
  };

  const handleDetailNode = (n: NodeEnt) => {
    detailNode = n;
    showDetailModal = true;
  };

  const handleDeleteNode = async (id: string) => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'ノード削除の確認',
      message: $_('list.confirmDelete.node'),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (ok) {
      await deleteNode(id);
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
          placeholder={$_('list.search.nodes')}
          bind:value={searchQuery}
          oninput={() => (currentPage = 1)}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

      <!-- Status Filters -->
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onclick={() => { statusFilter = "all"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'all' ? 'bg-cyan-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <Layers class="w-3.5 h-3.5" />
          <span>{$_('list.filter.allStatus')} ({nodeStatusCounts.all})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "normal"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'normal' ? 'bg-emerald-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <CheckCircle2 class="w-3.5 h-3.5 {statusFilter === 'normal' ? 'text-white' : 'text-emerald-500'}" />
          <span>{$_('list.filter.normalRepair')} ({nodeStatusCounts.normal})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "warn"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'warn' ? 'bg-amber-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <AlertTriangle class="w-3.5 h-3.5 {statusFilter === 'warn' ? 'text-white' : 'text-amber-500'}" />
          <span>{$_('list.filter.warn')} ({nodeStatusCounts.warn})</span>
        </button>
        <button
          type="button"
          onclick={() => { statusFilter = "error"; currentPage = 1; }}
          class="flex items-center gap-1.5 rounded-xl px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {statusFilter === 'error' ? 'bg-rose-600 text-white shadow-xs' : 'bg-slate-100 dark:bg-slate-800/80 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <AlertCircle class="w-3.5 h-3.5 {statusFilter === 'error' ? 'text-white' : 'text-rose-500'}" />
          <span>{$_('list.filter.faultHeavyLight')} ({nodeStatusCounts.error})</span>
        </button>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <button
        onclick={handleOpenAdd}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
      >
        <Plus class="h-4 w-4" />
        <span>{$_('list.addBtn.nodes')}</span>
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
            {@render sortableHeader("name", $_('list.table.nodeName'))}
            {@render sortableHeader("ip", $_('list.table.ip'))}
            {@render sortableHeader("mac", $_('list.table.mac'))}
            {@render sortableHeader("vendor", $_('list.table.vendor') || 'ベンダー')}
            {@render sortableHeader("descr", $_('list.table.descr'))}
            <th class="py-1 px-2 text-right w-28">{$_('list.table.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
          {#each paginatedNodes as n}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2">
                <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(n.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(n.state)}"></span>
                  {getStateName(n.state, $_)}
                </span>
              </td>
              <td class="py-1 px-2 font-bold text-slate-900 dark:text-slate-100 font-sans flex items-center gap-2">
                <span class="text-base text-cyan-500 dark:text-cyan-400 shrink-0 leading-none" style="font-family: 'Material Design Icons'">
                  {getIconCode(n.icon)}
                </span>
                <span>{n.name}</span>
              </td>
              <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-semibold">{n.ip}</td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{n.mac || "-"}</td>
              <td class="py-1 px-2 font-sans text-slate-700 dark:text-slate-300 text-[11px] max-w-[160px] truncate" title={n.vendor || (n as any).Vendor || (n.mac ? getVendor(n.mac) : "") || "-"}>
                {n.vendor || (n as any).Vendor || (n.mac ? getVendor(n.mac) : "") || "-"}
              </td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-300 font-sans truncate max-w-xs">{n.descr || "-"}</td>
              <td class="py-0 px-1 text-right font-sans">
                <div class="flex items-center justify-end">
                  <button
                    onclick={() => handleDetailNode(n)}
                    class="rounded-lg p-1.5 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-50 dark:hover:bg-cyan-950/40 hover:text-cyan-700 dark:hover:text-cyan-300 transition-colors cursor-pointer"
                    title={$_('map.context.vpanelDetail')}
                  >
                    <Box class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleEditNode(n)}
                    class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                    title={$_('common.edit')}
                  >
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleDeleteNode(n.id)}
                    class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                    title={$_('common.delete')}
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredNodes.length === 0}
            <tr>
              <td colspan="7" class="py-12 text-center text-slate-400 dark:text-slate-500 font-sans">
                {$_('list.empty.nodes')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <ListPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedNodes.length}
    />
  </div>
</div>

<!-- Modal Components -->
<NodeDialog
  bind:show={showNodeDialog}
  bind:node={selectedNode}
  onSave={onRefresh}
/>

{#if detailNode}
  <NodeDetailModal
    bind:show={showDetailModal}
    node={detailNode}
    bind:pollings={pollings}
    onPollingChanged={onRefresh}
  />
{/if}
