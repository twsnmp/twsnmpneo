<script lang="ts">
  import { _ } from "svelte-i18n";
  import { getStateColor, getStateName, formatTimeStr, getLogModeName, getLogModeBadgeClass } from "../common";
  import type { PollingEnt, NodeEnt } from "../api";
  import {
    X,
    Activity,
    Clock,
    Terminal,
    Edit,
    Server,
    CheckCircle2,
    AlertCircle,
    Info,
    Copy,
    Check,
    Layers,
    Sliders,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    polling = null,
    node = null,
    onEdit = () => {},
  } = $props<{
    show: boolean;
    polling: PollingEnt | null;
    node?: NodeEnt | null;
    onEdit?: (p: PollingEnt) => void;
  }>();

  let copied = $state(false);

  const copyResultJson = async () => {
    if (!polling?.result) return;
    try {
      await navigator.clipboard.writeText(JSON.stringify(polling.result, null, 2));
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch (e) {
      console.error("Failed to copy result:", e);
    }
  };
</script>

{#if show && polling}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") show = false;
    }}
  >
    <div
      class="flex max-h-[88vh] w-full max-w-3xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200 transition-colors"
    >
      <!-- Modal Header -->
      <div
        class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 dark:bg-cyan-500/20 border border-blue-500/20 dark:border-cyan-500/30 text-blue-600 dark:text-cyan-400"
          >
            <Activity class="h-5 w-5" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">
                {polling.name || (polling as any).Name}
              </h2>
              <span
                class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold border"
                style="background-color: {getStateColor(polling.state || (polling as any).State)}18; border-color: {getStateColor(polling.state || (polling as any).State)}50; color: {getStateColor(polling.state || (polling as any).State)}"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  style="background-color: {getStateColor(polling.state || (polling as any).State)}"
                ></span>
                {getStateName(polling.state || (polling as any).State, $_)}
              </span>
              <span
                class="rounded-md px-2 py-0.5 text-[10px] font-semibold uppercase bg-blue-50 dark:bg-cyan-500/10 text-blue-700 dark:text-cyan-300 border border-blue-200 dark:border-cyan-500/30"
              >
                {polling.type || (polling as any).Type}
              </span>
            </div>
            <p class="text-[11px] font-mono text-slate-500 dark:text-cyan-400">
              {node?.name ? `${node.name} (${node.ip})` : (node?.ip || "")}
            </p>
          </div>
        </div>

        <button
          type="button"
          aria-label={$_('common.close')}
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-5 space-y-4 text-xs min-h-0 bg-slate-50/70 dark:bg-slate-900/40">
        <!-- 1. Status Overview Card -->
        <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
            <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
              {$_('nodeDetail.colLastVal')}
            </span>
            <span class="text-base font-bold font-mono text-blue-600 dark:text-cyan-400">
              {polling.last_val !== undefined && polling.last_val !== null ? polling.last_val : "-"}
            </span>
          </div>
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
            <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
              {$_('nodeDetail.colLastTime')}
            </span>
            <span class="text-xs font-mono text-slate-700 dark:text-slate-300">
              {polling.last_time ? formatTimeStr(polling.last_time) : "-"}
            </span>
          </div>
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
            <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
              次回予定
            </span>
            <span class="text-xs font-mono text-slate-700 dark:text-slate-300">
              {polling.next_time ? formatTimeStr(polling.next_time) : "-"}
            </span>
          </div>
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-3 shadow-sm">
            <span class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider block mb-1">
              障害発生日時
            </span>
            <span class="text-xs font-mono {polling.fail_time ? 'text-rose-500 font-semibold' : 'text-slate-400'}">
              {polling.fail_time ? formatTimeStr(polling.fail_time) : "-"}
            </span>
          </div>
        </div>

        <!-- 2. Parameters & Configuration Section -->
        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden">
          <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
            <Sliders class="h-4 w-4 text-blue-500" />
            {$_('pollingDetail.params')}
          </div>
          <div class="p-4 grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <span class="text-slate-500 text-[11px] block mb-0.5">パラメータ / ターゲット:</span>
              <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all select-all">
                {polling.params || (polling as any).Params || polling.target || (polling as any).Target || "-"}
              </div>
            </div>
            <div>
              <span class="text-slate-500 text-[11px] block mb-0.5">動作モード / オプション:</span>
              <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                {polling.mode || (polling as any).Mode || "-"}
              </div>
            </div>
            <div>
              <span class="text-slate-500 text-[11px] block mb-0.5">フィルター条件:</span>
              <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                {polling.filter || (polling as any).Filter || "-"}
              </div>
            </div>
            <div>
              <span class="text-slate-500 text-[11px] block mb-0.5">抽出パターン (Extractor):</span>
              <div class="font-mono text-xs bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-200 dark:border-slate-800 text-slate-800 dark:text-slate-200 break-all">
                {polling.extractor || (polling as any).Extractor || "-"}
              </div>
            </div>
          </div>
        </div>

        <!-- 3. Evaluation Script Section -->
        {#if polling.script || (polling as any).Script}
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden">
            <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
              <Terminal class="h-4 w-4 text-emerald-500" />
              {$_('pollingDetail.script')}
            </div>
            <div class="p-3 bg-slate-950 text-slate-100 font-mono text-[11px] overflow-x-auto select-all rounded-b-xl leading-relaxed">
              <pre>{polling.script || (polling as any).Script}</pre>
            </div>
          </div>
        {/if}

        <!-- 4. Schedule & Execution Settings -->
        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden">
          <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center gap-2">
            <Clock class="h-4 w-4 text-indigo-500" />
            {$_('pollingDetail.schedule')}
          </div>
          <div class="p-4 grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
            <div>
              <span class="text-slate-500 text-[10px] block">ポーリング間隔</span>
              <span class="font-mono font-semibold">{polling.poll_int || (polling as any).PollInt || 60} 秒</span>
            </div>
            <div>
              <span class="text-slate-500 text-[10px] block">タイムアウト</span>
              <span class="font-mono font-semibold">{polling.timeout || (polling as any).Timeout || 1} 秒</span>
            </div>
            <div>
              <span class="text-slate-500 text-[10px] block">リトライ回数</span>
              <span class="font-mono font-semibold">{polling.retry || (polling as any).Retry || 1} 回</span>
            </div>
            <div>
              <span class="text-slate-500 text-[10px] block">ログ保存モード</span>
              <span class="inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-semibold border {getLogModeBadgeClass(polling.log_mode ?? (polling as any).LogMode)}">
                {getLogModeName(polling.log_mode ?? (polling as any).LogMode, $_)}
              </span>
            </div>
            {#if polling.fail_action || (polling as any).FailAction}
              <div class="col-span-2">
                <span class="text-slate-500 text-[10px] block">障害時アクション</span>
                <span class="font-mono text-[11px] text-rose-600 dark:text-rose-400 break-all">{polling.fail_action || (polling as any).FailAction}</span>
              </div>
            {/if}
            {#if polling.repair_action || (polling as any).RepairAction}
              <div class="col-span-2">
                <span class="text-slate-500 text-[10px] block">復帰時アクション</span>
                <span class="font-mono text-[11px] text-emerald-600 dark:text-emerald-400 break-all">{polling.repair_action || (polling as any).RepairAction}</span>
              </div>
            {/if}
          </div>
        </div>

        <!-- 5. Latest Result Data Section -->
        <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden">
          <div class="px-4 py-2.5 bg-slate-100/70 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 font-bold text-slate-700 dark:text-slate-200 flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Layers class="h-4 w-4 text-cyan-500" />
              {$_('pollingDetail.latestResult')}
            </div>
            {#if polling.result && Object.keys(polling.result).length > 0}
              <button
                type="button"
                onclick={copyResultJson}
                class="inline-flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 transition-colors cursor-pointer"
              >
                {#if copied}
                  <Check class="h-3.5 w-3.5 text-emerald-500" />
                  <span class="text-emerald-500">コピーしました</span>
                {:else}
                  <Copy class="h-3.5 w-3.5" />
                  <span>JSONをコピー</span>
                {/if}
              </button>
            {/if}
          </div>
          <div class="p-4">
            {#if polling.result && Object.keys(polling.result).length > 0}
              <div class="overflow-x-auto max-h-56">
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="bg-slate-50 dark:bg-slate-950/50 text-[10px] uppercase font-semibold text-slate-500 border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="py-1 px-3 w-1/3">キー (Key)</th>
                      <th class="py-1 px-3">値 (Value)</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-[11px]">
                    {#each Object.entries(polling.result) as [key, val]}
                      <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/20">
                        <td class="py-1.5 px-3 font-semibold text-slate-600 dark:text-slate-400 select-all">{key}</td>
                        <td class="py-1.5 px-3 text-slate-900 dark:text-slate-100 break-all select-all">
                          {typeof val === 'object' ? JSON.stringify(val) : String(val)}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {:else}
              <p class="text-slate-400 text-center py-3 text-xs font-sans">
                {$_('pollingDetail.noResult')}
              </p>
            {/if}
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div
        class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-2.5 shrink-0"
      >
        <button
          type="button"
          onclick={() => {
            const p = polling;
            show = false;
            if (p) onEdit(p);
          }}
          class="inline-flex items-center gap-1.5 rounded-xl border border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-cyan-400 hover:bg-blue-500/20 px-4 py-2 text-xs font-semibold shadow-sm transition-all cursor-pointer"
        >
          <Edit class="h-3.5 w-3.5" />
          {$_('common.edit')}
        </button>

        <button
          type="button"
          onclick={() => (show = false)}
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 shadow-sm hover:bg-slate-100 dark:hover:bg-slate-700 hover:text-slate-900 dark:hover:text-white transition-all cursor-pointer"
        >
          <X class="h-4 w-4" />
          {$_('common.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
