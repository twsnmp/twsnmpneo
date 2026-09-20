<script lang="ts">
  import { onMount } from "svelte";
  import { fetchPollings, deletePolling, type PollingEnt } from "../api";
  import { getStateColor, getStateName } from "../common";
  import { Search, Plus, Trash2, RefreshCw, Activity, CheckCircle2 } from "@lucide/svelte";

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
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-4 p-6 overflow-hidden bg-background">
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border bg-card p-4 shadow-sm">
    <div class="flex items-center gap-3">
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          placeholder="ポーリング名・ターゲットで検索..."
          bind:value={searchQuery}
          class="w-full rounded-lg border border-border bg-background py-1.5 pl-9 pr-3 text-xs focus:border-primary focus:outline-none"
        />
      </div>

      <select
        bind:value={typeFilter}
        class="rounded-lg border border-border bg-background px-3 py-1.5 text-xs focus:border-primary focus:outline-none"
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

    <button onclick={loadPollings} class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs hover:bg-muted font-medium">
      <RefreshCw class="h-3.5 w-3.5" />
      更新
    </button>
  </div>

  <div class="flex-1 overflow-hidden rounded-xl border border-border bg-card shadow-sm">
    <div class="h-full overflow-y-auto">
      <table class="w-full text-left text-xs border-collapse">
        <thead class="sticky top-0 z-10 border-b border-border bg-muted/80 backdrop-blur-sm text-muted-foreground">
          <tr>
            <th class="p-3.5">状態</th>
            <th class="p-3.5">ポーリング名</th>
            <th class="p-3.5">種別</th>
            <th class="p-3.5">ターゲット</th>
            <th class="p-3.5">最新応答値</th>
            <th class="p-3.5">最終実行日時</th>
            <th class="p-3.5 text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          {#each filteredPollings as p}
            <tr class="hover:bg-muted/40 transition-colors">
              <td class="p-3.5">
                <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-medium" style="background-color: {getStateColor(p.state)}20; color: {getStateColor(p.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(p.state)}"></span>
                  {getStateName(p.state)}
                </span>
              </td>
              <td class="p-3.5 font-semibold text-foreground">{p.name}</td>
              <td class="p-3.5">
                <span class="rounded bg-muted px-2 py-0.5 font-mono text-[10px] uppercase font-bold text-primary">
                  {p.type}
                </span>
              </td>
              <td class="p-3.5 font-mono text-muted-foreground">{p.target || "-"}</td>
              <td class="p-3.5 font-mono text-foreground font-semibold">{p.last_val !== undefined ? `${p.last_val} ms` : "-"}</td>
              <td class="p-3.5 text-muted-foreground">{p.last_time ? new Date(p.last_time * 1000).toLocaleString() : "-"}</td>
              <td class="p-3.5 text-right">
                <button onclick={() => handleDelete(p.id)} class="rounded p-1 text-destructive hover:bg-destructive/10" title="削除">
                  <Trash2 class="h-4 w-4" />
                </button>
              </td>
            </tr>
          {/each}
          {#if filteredPollings.length === 0}
            <tr>
              <td colspan="7" class="p-8 text-center text-muted-foreground">
                ポーリング設定が見つかりません
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>
