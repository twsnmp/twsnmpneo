<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    Network,
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import { deleteNetwork, type NetworkEnt } from "../../api";
  import { showConfirm } from "../../stores/modalStore";
  import NetworkDialog from "../../components/NetworkDialog.svelte";
  import ListPagination from "./ListPagination.svelte";
  import { compareSortValues } from "./listUtils";
  import type { SortDirection } from "./types";

  let {
    networks = [],
    loading = false,
    onRefresh = () => {},
  }: {
    networks?: NetworkEnt[];
    loading?: boolean;
    onRefresh?: () => void | Promise<void>;
  } = $props();

  let searchQuery = $state("");
  let currentPage = $state(1);
  let pageSize = $state(25);
  let sortColumn = $state<string | null>(null);
  let sortDirection = $state<SortDirection>("desc");

  // Dialog states
  let showNetworkDialog = $state(false);
  let selectedNetwork = $state<NetworkEnt | null>(null);

  const filteredNetworks = $derived(
    networks.filter((net) => {
      const q = searchQuery.toLowerCase();
      const name = (net.name || (net as any).Name || "").toLowerCase();
      const ip = (net.ip || (net as any).IP || "").toLowerCase();
      return !q || name.includes(q) || ip.includes(q);
    })
  );

  const getNetworkSortValue = (item: NetworkEnt, column: string): unknown => {
    switch (column) {
      case "name": return item.name || (item as any).Name || "";
      case "ip": return item.ip || (item as any).IP || "";
      case "portsCount": return (item.ports || (item as any).Ports || []).length;
      case "size": return [item.w ?? (item as any).W ?? 0, item.h ?? (item as any).H ?? 0];
      case "coords": return [item.x ?? (item as any).X ?? 0, item.y ?? (item as any).Y ?? 0];
      case "descr": return (item as any).descr || (item as any).Descr || "";
    }
    return "";
  };

  const sortedNetworks = $derived.by(() => {
    if (!sortColumn) return filteredNetworks;
    const direction = sortDirection === "asc" ? 1 : -1;
    return [...filteredNetworks].sort((a, b) => {
      return direction * compareSortValues(
        getNetworkSortValue(a, sortColumn!),
        getNetworkSortValue(b, sortColumn!)
      );
    });
  });

  const paginatedNetworks = $derived.by(() => {
    if (pageSize === -1) return sortedNetworks;
    const totalPages = Math.max(1, Math.ceil(sortedNetworks.length / pageSize));
    const page = Math.min(currentPage, totalPages);
    const start = (page - 1) * pageSize;
    return sortedNetworks.slice(start, start + pageSize);
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
    selectedNetwork = null;
    showNetworkDialog = true;
  };

  const handleEditNetwork = (net: NetworkEnt) => {
    selectedNetwork = { ...net };
    showNetworkDialog = true;
  };

  const handleDeleteNetwork = async (id: string) => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'ネットワーク削除の確認',
      message: $_('list.confirmDelete.network'),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (ok) {
      await deleteNetwork(id);
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
          placeholder={$_('list.search.networks')}
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
        <span>{$_('list.addBtn.networks')}</span>
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
            {@render sortableHeader("name", $_('list.table.netName'))}
            {@render sortableHeader("ip", $_('list.table.ip'))}
            {@render sortableHeader("portsCount", $_('list.table.portsCount'), "w-28")}
            {@render sortableHeader("size", $_('list.table.size'), "w-32")}
            {@render sortableHeader("coords", $_('list.table.coords'), "w-32")}
            {@render sortableHeader("descr", $_('list.table.descr'))}
            <th class="py-1 px-2 text-right w-24">{$_('list.table.action')}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
          {#each paginatedNetworks as net}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-bold text-slate-900 dark:text-slate-100 font-sans flex items-center gap-2">
                <Network class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 shrink-0" />
                <span>{net.name}</span>
              </td>
              <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 font-semibold">{net.ip || "-"}</td>
              <td class="py-1 px-2">
                <span class="rounded px-2 py-0.5 text-[10px] font-bold bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 border border-slate-200 dark:border-slate-700">
                  {(net.ports || []).length} {$_('network.portsUnit')}
                </span>
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px]">
                {net.w} × {net.h}
              </td>
              <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px]">
                ({net.x}, {net.y})
              </td>
              <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-sans truncate max-w-xs">{"descr" in net ? (net as any).descr || "-" : "-"}</td>
              <td class="py-0 px-1 text-right font-sans">
                <div class="flex items-center justify-end">
                  <button
                    onclick={() => handleEditNetwork(net)}
                    class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                    title={$_('common.edit')}
                  >
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button
                    onclick={() => handleDeleteNetwork(net.id)}
                    class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                    title={$_('common.delete')}
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredNetworks.length === 0}
            <tr>
              <td colspan="7" class="py-12 text-center text-slate-400 dark:text-slate-500 font-sans">
                {$_('list.empty.networks')}
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>

    <ListPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedNetworks.length}
    />
  </div>
</div>

<!-- Modal Components -->
<NetworkDialog
  bind:show={showNetworkDialog}
  bind:network={selectedNetwork}
  onSave={onRefresh}
/>
