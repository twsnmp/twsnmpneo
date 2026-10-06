<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Laptop,
    CheckSquare,
    Network,
    GitCommitHorizontal,
    Boxes,
    AlertTriangle,
  } from "@lucide/svelte";
  import {
    fetchNodes,
    fetchPollings,
    fetchNetworks,
    fetchLines,
    fetchDrawItems,
    type NodeEnt,
    type PollingEnt,
    type NetworkEnt,
    type LineEnt,
    type DrawItemEnt,
  } from "../api";
  import NodeList from "./list/NodeList.svelte";
  import PollingList from "./list/PollingList.svelte";
  import NetworkList from "./list/NetworkList.svelte";
  import LineList from "./list/LineList.svelte";
  import DrawItemList from "./list/DrawItemList.svelte";
  import { isLineOrphaned, isDrawItemOffscreen } from "./list/listUtils";
  import type { ListCategory } from "./list/types";

  let activeCategory = $state<ListCategory>("nodes");
  let loading = $state(false);

  // Entities
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let networks = $state<NetworkEnt[]>([]);
  let lines = $state<LineEnt[]>([]);
  let drawItems = $state<DrawItemEnt[]>([]);

  // Categories definition
  const categories = $derived([
    { id: "nodes" as const, name: $_('list.categories.nodes'), icon: Laptop },
    { id: "pollings" as const, name: $_('list.categories.pollings'), icon: CheckSquare },
    { id: "networks" as const, name: $_('list.categories.networks'), icon: Network },
    { id: "lines" as const, name: $_('list.categories.lines'), icon: GitCommitHorizontal },
    { id: "drawitems" as const, name: $_('list.categories.drawitems'), icon: Boxes },
  ]);

  const loadAll = async () => {
    loading = true;
    try {
      const [n, p, net, l, d] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
        fetchNetworks().catch(() => []),
        fetchLines().catch(() => []),
        fetchDrawItems().catch(() => []),
      ]);
      nodes = n;
      pollings = p;
      networks = net;
      lines = l;
      drawItems = d;
    } catch (e) {
      console.error("Failed to load inventory data:", e);
    } finally {
      loading = false;
    }
  };

  onMount(loadAll);

  // Orphan & offscreen counts for summary card
  const orphanLinesCount = $derived(
    lines.filter((l) => isLineOrphaned(l, nodes, networks)).length
  );
  const offscreenDrawItemsCount = $derived(
    drawItems.filter(isDrawItemOffscreen).length
  );
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-100 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans transition-colors">
  <!-- Left Sidebar (Matching ReportView / LogView layout) -->
  <div class="w-64 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/70 p-3 space-y-1.5 shrink-0 flex flex-col justify-between transition-colors">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
        {$_('list.itemsTitle')}
      </div>

      {#each categories as cat}
        {@const count =
          cat.id === "nodes" ? nodes.length :
          cat.id === "pollings" ? pollings.length :
          cat.id === "networks" ? networks.length :
          cat.id === "lines" ? lines.length :
          drawItems.length}
        <button
          type="button"
          onclick={() => {
            activeCategory = cat.id;
          }}
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeCategory === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <div class="flex items-center gap-2.5 truncate">
            <cat.icon class="h-4 w-4 shrink-0 {activeCategory === cat.id ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{cat.name}</span>
          </div>
          <span class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeCategory === cat.id ? 'bg-white/20 text-white' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-transparent'}">
            {count}
          </span>
        </button>
      {/each}
    </div>

    <!-- Live Status & Orphan Inspection Card -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-2 transition-colors">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-700 dark:text-slate-200">{$_('list.sync')}</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          ● {$_('list.realtime')}
        </span>
      </div>

      {#if orphanLinesCount > 0 || offscreenDrawItemsCount > 0}
        <div class="rounded-xl border border-amber-500/30 bg-amber-500/10 p-2 text-[10px] text-amber-700 dark:text-amber-300 space-y-1">
          <div class="font-bold flex items-center gap-1">
            <AlertTriangle class="h-3 w-3 text-amber-500 dark:text-amber-400" />
            <span>{$_('list.orphanAlert')}</span>
          </div>
          {#if orphanLinesCount > 0}
            <div>• {$_('list.orphanLines')}: <span class="font-bold">{orphanLinesCount}</span></div>
          {/if}
          {#if offscreenDrawItemsCount > 0}
            <div>• {$_('list.offscreenItems')}: <span class="font-bold">{offscreenDrawItemsCount}</span></div>
          {/if}
        </div>
      {/if}

      <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400 pt-1 border-t border-slate-200 dark:border-slate-800/80">
        {$_('list.totalItems')}: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{nodes.length + pollings.length + networks.length + lines.length + drawItems.length}</span>
      </div>
    </div>
  </div>

  <!-- Main View Canvas -->
  <div class="flex-1 overflow-hidden flex flex-col p-5 gap-4 min-w-0">
    {#if activeCategory === "nodes"}
      <NodeList
        {nodes}
        bind:pollings
        {loading}
        onRefresh={loadAll}
      />
    {:else if activeCategory === "pollings"}
      <PollingList
        {pollings}
        {nodes}
        {loading}
        onRefresh={loadAll}
      />
    {:else if activeCategory === "networks"}
      <NetworkList
        {networks}
        {loading}
        onRefresh={loadAll}
      />
    {:else if activeCategory === "lines"}
      <LineList
        {lines}
        {nodes}
        {networks}
        {pollings}
        {loading}
        onRefresh={loadAll}
      />
    {:else if activeCategory === "drawitems"}
      <DrawItemList
        {drawItems}
        {nodes}
        {pollings}
        {loading}
        onRefresh={loadAll}
      />
    {/if}
  </div>
</div>
