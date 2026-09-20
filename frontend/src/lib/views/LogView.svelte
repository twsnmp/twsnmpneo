<script lang="ts">
  import { onMount } from "svelte";
  import { fetchEventLogs, queryParquetLogs, askAI, type EventLogEnt, type ParquetLogRecord } from "../api";
  import { getStateColor, getStateName } from "../common";
  import { Search, RefreshCw, FileText, Sparkles, Filter, Database, Calendar } from "@lucide/svelte";

  let activeTab = $state<"event" | "syslog" | "trap" | "netflow">("event");
  let eventLogs = $state<EventLogEnt[]>([]);
  let parquetLogs = $state<ParquetLogRecord[]>([]);
  let searchQuery = $state("");

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

  const getLevelBadge = (level: string) => {
    switch (level?.toLowerCase()) {
      case "normal":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "warn":
      case "low":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "high":
      case "error":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      case "info":
        return "bg-sky-500/10 text-sky-400 border-sky-500/30";
      default:
        return "bg-slate-800 text-slate-400 border-slate-700";
    }
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] flex-col gap-4 p-5 overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Tabs & Filter Bar (twnoaa style) -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4 shadow-lg">
    <div class="flex items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-950 p-1 text-xs">
      <button
        onclick={() => (activeTab = "event")}
        class="rounded-lg px-3.5 py-1.5 font-semibold transition-all {activeTab === 'event' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
      >
        イベントログ
      </button>
      <button
        onclick={() => (activeTab = "syslog")}
        class="rounded-lg px-3.5 py-1.5 font-semibold transition-all {activeTab === 'syslog' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
      >
        Syslog (Parquet)
      </button>
      <button
        onclick={() => (activeTab = "trap")}
        class="rounded-lg px-3.5 py-1.5 font-semibold transition-all {activeTab === 'trap' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
      >
        SNMP TRAP (Parquet)
      </button>
      <button
        onclick={() => (activeTab = "netflow")}
        class="rounded-lg px-3.5 py-1.5 font-semibold transition-all {activeTab === 'netflow' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
      >
        NetFlow (Parquet)
      </button>
    </div>

    <div class="flex items-center gap-3">
      <div class="relative w-64">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder="ログ内容・送信元で検索..."
          bind:value={searchQuery}
          onkeydown={(e) => e.key === "Enter" && loadCurrentLogs()}
          class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

      <button
        onclick={loadCurrentLogs}
        class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-all"
      >
        <RefreshCw class="h-3.5 w-3.5 text-cyan-400" />
        更新
      </button>
    </div>
  </div>

  <!-- Table Container (twnoaa style) -->
  <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
    <div class="h-full overflow-y-auto overflow-x-auto">
      {#if activeTab === "event"}
        <table class="w-full text-left text-xs">
          <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
            <tr>
              <th class="py-2.5 px-3.5 w-44">日時</th>
              <th class="py-2.5 px-3.5 w-28">レベル</th>
              <th class="py-2.5 px-3.5 w-28">種別</th>
              <th class="py-2.5 px-3.5 w-48">ノード</th>
              <th class="py-2.5 px-3.5">イベント内容</th>
              <th class="py-2.5 px-3.5 text-right w-20">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
            {#each eventLogs as el}
              <tr class="hover:bg-slate-800/40 transition-colors">
                <td class="py-2 px-3.5 text-slate-400 text-[11px]">{new Date(el.time * 1000).toLocaleString()}</td>
                <td class="py-2 px-3.5">
                  <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getLevelBadge(el.level)}">
                    <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(el.level)}"></span>
                    {el.level}
                  </span>
                </td>
                <td class="py-2 px-3.5 font-semibold text-cyan-400 font-sans">{el.type}</td>
                <td class="py-2 px-3.5 font-sans text-slate-200">{el.node_name || "-"}</td>
                <td class="py-2 px-3.5 text-slate-100 break-all font-sans">{el.event}</td>
                <td class="py-2 px-3.5 text-right font-sans">
                  <button
                    onclick={() => handleAskAI(`${el.type} (${el.level}): ${el.event} [Node: ${el.node_name || el.node_id}]`)}
                    class="inline-flex items-center gap-1 rounded-lg border border-cyan-500/30 bg-cyan-500/10 px-2 py-1 text-[10px] font-bold text-cyan-300 hover:bg-cyan-500/20 transition-all"
                  >
                    <Sparkles class="h-3 w-3" />
                    分析
                  </button>
                </td>
              </tr>
            {/each}
            {#if eventLogs.length === 0}
              <tr>
                <td colspan="6" class="py-12 text-center text-slate-500 font-sans">
                  イベントログがありません
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      {:else}
        <table class="w-full text-left text-xs">
          <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
            <tr>
              <th class="py-2.5 px-3.5 w-44">日時</th>
              <th class="py-2.5 px-3.5 w-40">送信元 (Src)</th>
              <th class="py-2.5 px-3.5">ログデータ (Parquet Record)</th>
              <th class="py-2.5 px-3.5 text-right w-20">AI</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
            {#each parquetLogs as pl}
              <tr class="hover:bg-slate-800/40 transition-colors">
                <td class="py-2 px-3.5 text-slate-400 text-[11px]">{new Date(pl.time * 1000).toLocaleString()}</td>
                <td class="py-2 px-3.5 font-semibold text-cyan-400 font-mono">{pl.src}</td>
                <td class="py-2 px-3.5 text-slate-100 break-all">{pl.log}</td>
                <td class="py-2 px-3.5 text-right font-sans">
                  <button
                    onclick={() => handleAskAI(`${pl.type} from ${pl.src}: ${pl.log}`)}
                    class="inline-flex items-center gap-1 rounded-lg border border-cyan-500/30 bg-cyan-500/10 px-2 py-1 text-[10px] font-bold text-cyan-300 hover:bg-cyan-500/20 transition-all"
                  >
                    <Sparkles class="h-3 w-3" />
                    分析
                  </button>
                </td>
              </tr>
            {/each}
            {#if parquetLogs.length === 0}
              <tr>
                <td colspan="4" class="py-12 text-center text-slate-500 font-sans">
                  該当するログは見つかりませんでした
                </td>
              </tr>
            {/if}
          </tbody>
        </table>
      {/if}
    </div>
  </div>

  <!-- AI Analysis Modal (twnoaa style) -->
  {#if showAIDialog}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <div class="flex max-h-[80vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl overflow-hidden">
        <div class="flex items-center justify-between border-b border-slate-800 pb-3">
          <div class="flex items-center gap-2 text-base font-bold text-cyan-400">
            <Sparkles class="h-5 w-5" />
            <span>AI ログ診断アシスタント</span>
          </div>
          <button onclick={() => (showAIDialog = false)} class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-white">✕</button>
        </div>

        <div class="my-3 rounded-xl border border-slate-800 bg-slate-950 p-3 font-mono text-xs text-slate-300">
          {selectedLogText}
        </div>

        <div class="flex-1 overflow-y-auto rounded-xl border border-slate-800 bg-slate-950 p-4 text-xs leading-relaxed text-slate-200 whitespace-pre-wrap">
          {#if aiLoading}
            <div class="flex items-center gap-2 text-cyan-400">
              <RefreshCw class="h-4 w-4 animate-spin" />
              <span>マルチLLM推論中... ログとトポロジーを総合解析しています</span>
            </div>
          {:else}
            {aiAnswer}
          {/if}
        </div>

        <div class="mt-4 flex justify-end border-t border-slate-800 pt-3">
          <button onclick={() => (showAIDialog = false)} class="rounded-xl bg-slate-800 hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-200 transition-colors">
            閉じる
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
