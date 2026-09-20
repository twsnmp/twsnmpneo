<script lang="ts">
  import { onMount } from "svelte";
  import { fetchEventLogs, queryParquetLogs, askAI, type EventLogEnt, type ParquetLogRecord } from "../api";
  import { getStateColor, getStateName } from "../common";
  import { Search, RefreshCw, FileText, Sparkles, Filter, Database, Calendar } from "@lucide/svelte";

  let activeTab = $state<"event" | "syslog" | "trap" | "netflow" | "arp">("event");
  let eventLogs = $state<EventLogEnt[]>([]);
  let parquetLogs = $state<ParquetLogRecord[]>([]);
  let searchQuery = $state("");
  let levelFilter = $state("all");

  // AI Dialog state
  let showAIDialog = $state(false);
  let selectedLogText = $state("");
  let aiAnswer = $state("");
  let aiLoading = $state(false);

  const loadCurrentLogs = async () => {
    if (activeTab === "event") {
      try {
        eventLogs = await fetchEventLogs();
      } catch (e) {
        console.error(e);
      }
    } else {
      try {
        parquetLogs = await queryParquetLogs(activeTab, searchQuery);
      } catch (e) {
        console.error(e);
      }
    }
  };

  onMount(loadCurrentLogs);

  $effect(() => {
    activeTab;
    loadCurrentLogs();
  });

  const handleAskAI = async (logText: string) => {
    selectedLogText = logText;
    showAIDialog = true;
    aiLoading = true;
    aiAnswer = "";
    try {
      aiAnswer = await askAI(`以下のネットワークログを解析し、根本原因と推奨対策を説明してください。\n\nログ:\n${logText}`);
    } catch (e: any) {
      aiAnswer = `AI解析エラー: ${e.message}`;
    } finally {
      aiLoading = false;
    }
  };
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-4 p-6 overflow-hidden bg-background">
  <!-- Tabs & Filter bar -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border bg-card p-4 shadow-sm">
    <div class="flex items-center gap-2 rounded-lg border border-border bg-background p-1 text-xs">
      <button
        onclick={() => (activeTab = "event")}
        class="rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'event' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
      >
        イベントログ
      </button>
      <button
        onclick={() => (activeTab = "syslog")}
        class="rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'syslog' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
      >
        Syslog (Parquet)
      </button>
      <button
        onclick={() => (activeTab = "trap")}
        class="rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'trap' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
      >
        SNMP TRAP (Parquet)
      </button>
      <button
        onclick={() => (activeTab = "netflow")}
        class="rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'netflow' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
      >
        NetFlow (Parquet)
      </button>
    </div>

    <div class="flex items-center gap-3">
      <div class="relative w-64">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          placeholder="ログ内容・送信元で検索..."
          bind:value={searchQuery}
          onkeydown={(e) => e.key === "Enter" && loadCurrentLogs()}
          class="w-full rounded-lg border border-border bg-background py-1.5 pl-9 pr-3 text-xs focus:border-primary focus:outline-none"
        />
      </div>

      <button onclick={loadCurrentLogs} class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs hover:bg-muted font-medium">
        <RefreshCw class="h-3.5 w-3.5" />
        更新
      </button>
    </div>
  </div>

  <!-- Table View -->
  <div class="flex-1 overflow-hidden rounded-xl border border-border bg-card shadow-sm">
    <div class="h-full overflow-y-auto">
      {#if activeTab === "event"}
        <table class="w-full text-left text-xs border-collapse font-mono">
          <thead class="sticky top-0 z-10 border-b border-border bg-muted/80 backdrop-blur-sm text-muted-foreground font-sans">
            <tr>
              <th class="p-3">日時</th>
              <th class="p-3">レベル</th>
              <th class="p-3">種別</th>
              <th class="p-3">ノード</th>
              <th class="p-3">イベント内容</th>
              <th class="p-3 text-right">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each eventLogs as el}
              <tr class="hover:bg-muted/40 transition-colors">
                <td class="p-3 text-muted-foreground">{new Date(el.time * 1000).toLocaleString()}</td>
                <td class="p-3">
                  <span class="rounded px-2 py-0.5 text-[10px] uppercase font-bold" style="background-color: {getStateColor(el.level)}20; color: {getStateColor(el.level)}">
                    {el.level}
                  </span>
                </td>
                <td class="p-3 font-semibold text-primary">{el.type}</td>
                <td class="p-3 font-sans text-foreground">{el.node_name || "-"}</td>
                <td class="p-3 text-foreground break-all">{el.event}</td>
                <td class="p-3 text-right">
                  <button
                    onclick={() => handleAskAI(`${el.type} (${el.level}): ${el.event} [Node: ${el.node_name || el.node_id}]`)}
                    class="flex items-center gap-1 rounded border border-primary/40 bg-primary/10 px-2 py-1 text-[10px] font-sans font-medium text-primary hover:bg-primary/20"
                  >
                    <Sparkles class="h-3 w-3" />
                    分析
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <table class="w-full text-left text-xs border-collapse font-mono">
          <thead class="sticky top-0 z-10 border-b border-border bg-muted/80 backdrop-blur-sm text-muted-foreground font-sans">
            <tr>
              <th class="p-3">日時</th>
              <th class="p-3">送信元 (Src)</th>
              <th class="p-3">ログデータ (Columnar Record)</th>
              <th class="p-3 text-right">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            {#each parquetLogs as pl}
              <tr class="hover:bg-muted/40 transition-colors">
                <td class="p-3 text-muted-foreground">{new Date(pl.time * 1000).toLocaleString()}</td>
                <td class="p-3 font-semibold text-primary">{pl.src}</td>
                <td class="p-3 text-foreground break-all">{pl.log}</td>
                <td class="p-3 text-right">
                  <button
                    onclick={() => handleAskAI(`${pl.type} from ${pl.src}: ${pl.log}`)}
                    class="flex items-center gap-1 rounded border border-primary/40 bg-primary/10 px-2 py-1 text-[10px] font-sans font-medium text-primary hover:bg-primary/20"
                  >
                    <Sparkles class="h-3 w-3" />
                    分析
                  </button>
                </td>
              </tr>
            {/each}
            {#if parquetLogs.length === 0}
              <tr>
                <td colspan="4" class="p-8 text-center text-muted-foreground font-sans">
                  該当するログは見つかりませんでした
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      {/if}
    </div>
  </div>

  <!-- AI Analysis Modal -->
  {#if showAIDialog}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <div class="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-xl border border-border bg-card p-6 shadow-2xl overflow-hidden">
        <div class="flex items-center justify-between border-b border-border pb-3">
          <div class="flex items-center gap-2 text-base font-bold text-primary">
            <Sparkles class="h-5 w-5" />
            <span>AI ログ診断アシスタント</span>
          </div>
          <button onclick={() => (showAIDialog = false)} class="rounded p-1 text-muted-foreground hover:bg-muted">✕</button>
        </div>

        <div class="my-3 rounded-lg border border-border bg-muted/40 p-3 font-mono text-xs text-foreground">
          {selectedLogText}
        </div>

        <div class="flex-1 overflow-y-auto rounded-lg border border-border/80 bg-background p-4 text-xs leading-relaxed text-foreground whitespace-pre-wrap">
          {#if aiLoading}
            <div class="flex items-center gap-2 text-muted-foreground">
              <RefreshCw class="h-4 w-4 animate-spin text-primary" />
              <span>マルチLLM推論中... ログとトポロジーを総合解析しています</span>
            </div>
          {:else}
            {aiAnswer}
          {/if}
        </div>

        <div class="mt-4 flex justify-end border-t border-border pt-3">
          <button onclick={() => (showAIDialog = false)} class="rounded-lg bg-muted px-4 py-1.5 text-xs font-semibold hover:bg-muted/80">
            閉じる
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
