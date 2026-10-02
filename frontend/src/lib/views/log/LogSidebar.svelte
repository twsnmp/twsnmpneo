<script lang="ts">
  import { _ } from "svelte-i18n";
  import { FileText, Server, AlertTriangle, BarChart3, Activity } from "@lucide/svelte";
  import type { LogCategory } from "./types";

  interface Props {
    activeTab: LogCategory;
    logCounts: Record<string, number>;
    currentTabCount: number;
    hitCount: number;
    fetchLimit: number;
    loading?: boolean;
    onTabSelect: (tab: LogCategory) => void;
    onLimitChange: () => void;
  }

  let {
    activeTab = $bindable(),
    logCounts = {},
    currentTabCount = 0,
    hitCount = 0,
    fetchLimit = $bindable(),
    loading = false,
    onTabSelect,
    onLimitChange,
  }: Props = $props();

  const categories = $derived<{ id: LogCategory; name: string; icon: any }[]>([
    { id: "event", name: $_('log.categories.event'), icon: FileText },
    { id: "syslog", name: $_('log.categories.syslog'), icon: Server },
    { id: "trap", name: $_('log.categories.trap'), icon: AlertTriangle },
    { id: "netflow", name: $_('log.categories.netflow'), icon: BarChart3 },
    { id: "sflow", name: $_('log.categories.sflow'), icon: Activity },
    { id: "arp", name: $_('log.categories.arp'), icon: Server },
  ]);
</script>

<div
  class="w-60 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/80 p-3 space-y-1.5 shrink-0 flex flex-col justify-between transition-colors"
>
  <div class="space-y-1">
    <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
      {$_('log.typeHeader')}
    </div>

    {#each categories as cat}
      {@const count = logCounts[cat.id] ?? (activeTab === cat.id ? currentTabCount : 0)}
      <button
        type="button"
        onclick={() => onTabSelect(cat.id)}
        class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === cat.id
          ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30'
          : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
      >
        <div class="flex items-center gap-2.5 truncate">
          <cat.icon class="h-4 w-4 shrink-0 {activeTab === cat.id ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
          <span class="truncate">{cat.name}</span>
        </div>
        <span
          class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeTab === cat.id
            ? 'bg-white/20 text-white'
            : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-transparent'}"
        >
          {#if loading && activeTab === cat.id}
            <span class="animate-pulse">...</span>
          {:else}
            {count.toLocaleString()}
          {/if}
        </span>
      </button>
    {/each}
  </div>

  <!-- Live Status & Stats Card -->
  <div
    class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-2 transition-colors"
  >
    <div class="flex items-center justify-between">
      <span class="font-semibold text-slate-700 dark:text-slate-200">{$_('log.fetchLimit')}</span>
      <select
        bind:value={fetchLimit}
        onchange={onLimitChange}
        class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2 py-0.5 text-[10px] text-cyan-600 dark:text-cyan-400 font-mono focus:outline-none cursor-pointer"
      >
        <option value={1000}>1,000 {$_('log.recordsUnit')}</option>
        <option value={5000}>5,000 {$_('log.recordsUnit')}</option>
        <option value={10000}>10,000 {$_('log.recordsUnit')}</option>
        <option value={20000}>20,000 {$_('log.recordsUnit')}</option>
      </select>
    </div>
    <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400 pt-1 border-t border-slate-200 dark:border-slate-800/80 flex items-center justify-between">
      <span>{$_('log.hitCount')}</span>
      {#if loading}
        <span class="text-cyan-600 dark:text-cyan-400 font-bold animate-pulse flex items-center gap-1">
          <span class="inline-block h-1.5 w-1.5 rounded-full bg-cyan-500 animate-ping"></span>
          <span>{$_('log.searching')}</span>
        </span>
      {:else}
        <span class="text-cyan-600 dark:text-cyan-400 font-bold">{hitCount.toLocaleString()} {$_('log.recordsUnit')}</span>
      {/if}
    </div>
  </div>
</div>
