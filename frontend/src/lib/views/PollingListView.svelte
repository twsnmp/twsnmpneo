<script lang="ts">
  import { onMount } from "svelte";
  import { fetchPollings, deletePolling, type PollingEnt } from "../api";
  import { getStateColor, getStateName } from "../common";
  import { Search, Trash2, RefreshCw, CheckSquare } from "@lucide/svelte";

  let pollings = $state<PollingEnt[]>([]);
  let searchQuery = $state("");
  let typeFilter = $state("all");

  const loadPollings = async () => {
    try {
      pollings = await fetchPollings();
    } catch (e) {
      console.error(e);
    }
  };

  onMount(loadPollings);

  const filteredPollings = $derived(
    pollings.filter((p) => {
      const matchSearch =
        p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (p.target && p.target.toLowerCase().includes(searchQuery.toLowerCase()));
      const matchType = typeFilter === "all" || p.type === typeFilter;
      return matchSearch && matchType;
    })
  );

  const handleDelete = async (id: string) => {
    if (confirm("このポーリング設定を削除してもよろしいですか？")) {
      await deletePolling(id);
      await loadPollings();
    }
  };

  const getStatusBadge = (state: string) => {
    switch (state?.toLowerCase()) {
      case "normal":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "warn":
      case "low":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "high":
      case "error":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      default:
        return "bg-slate-800 text-slate-400 border-slate-700";
    }
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] flex-col gap-4 p-5 overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Controls bar (twnoaa style) -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4 shadow-lg">
    <div class="flex items-center gap-3">
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder="ポーリング名・ターゲットで検索..."
          bind:value={searchQuery}
          class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

      <select
        bind:value={typeFilter}
        class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
      >
        <option value="all">全プロトコル ({pollings.length})</option>
        <option value="ping">PING</option>
        <option value="http">HTTP/HTTPS</option>
        <option value="snmp">SNMP</option>
        <option value="tcp">TCP ポート</option>
        <option value="dns">DNS</option>
        <option value="ntp">NTP</option>
      </select>
    </div>

    <button
      onclick={loadPollings}
      class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-all"
    >
      <RefreshCw class="h-3.5 w-3.5 text-cyan-400" />
      更新
    </button>
  </div>

  <!-- Table container (twnoaa style) -->
  <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
    <div class="h-full overflow-y-auto overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
          <tr>
            <th class="py-2.5 px-3.5 w-32">状態</th>
            <th class="py-2.5 px-3.5">ポーリング名</th>
            <th class="py-2.5 px-3.5 w-28">種別</th>
            <th class="py-2.5 px-3.5">ターゲット</th>
            <th class="py-2.5 px-3.5 w-36">最新応答値</th>
            <th class="py-2.5 px-3.5 w-44">最終実行日時</th>
            <th class="py-2.5 px-3.5 text-right w-20">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
          {#each filteredPollings as p}
            <tr class="hover:bg-slate-800/40 transition-colors">
              <td class="py-2 px-3.5">
                <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(p.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(p.state)}"></span>
                  {getStateName(p.state)}
                </span>
              </td>
              <td class="py-2 px-3.5 font-bold text-slate-100 font-sans">{p.name}</td>
              <td class="py-2 px-3.5">
                <span class="rounded px-2 py-0.5 font-mono text-[10px] uppercase font-bold bg-cyan-500/10 text-cyan-300 border border-cyan-500/30">
                  {p.type}
                </span>
              </td>
              <td class="py-2 px-3.5 text-slate-300 font-mono">{p.target || "-"}</td>
              <td class="py-2 px-3.5 text-cyan-400 font-mono">
                {p.last_val !== undefined ? p.last_val.toFixed(2) + " ms" : "-"}
              </td>
              <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                {p.last_time ? new Date(p.last_time * 1000).toLocaleString() : "-"}
              </td>
              <td class="py-2 px-3.5 text-right font-sans">
                <button
                  onclick={() => handleDelete(p.id)}
                  class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                  title="削除"
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              </td>
            </tr>
          {/each}
          {#if filteredPollings.length === 0}
            <tr>
              <td colspan="7" class="py-12 text-center text-slate-500 font-sans">
                ポーリング項目が見つかりません
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
