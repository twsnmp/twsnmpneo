<script lang="ts">
  import type { EventLogEnt } from '$lib/api';
  import { diagnoseAlert } from '$lib/api';
  import { _, locale } from 'svelte-i18n';
  import { Sparkles, AlertCircle, AlertTriangle, Info, CheckCircle, Search, Loader2, Languages, Copy, Check, RefreshCw } from '@lucide/svelte';
  import AIMarkdown from './common/AIMarkdown.svelte';
  import AnimatedCatLogo from './common/AnimatedCatLogo.svelte';

  let { eventLogs = [], onRefresh }: {
    eventLogs: EventLogEnt[];
    onRefresh?: () => void;
  } = $props();

  let filterText = $state('');
  let diagnosingLog = $state<EventLogEnt | null>(null);
  let diagnosisResult = $state<string | null>(null);
  let isDiagnosing = $state(false);
  let diagError = $state<string | null>(null);
  let copied = $state(false);

  const isJaLocale = $derived($locale?.startsWith('ja') ?? true);
  let aiLanguage = $state<'ja' | 'en'>('ja');

  let filteredLogs = $derived(
    eventLogs.filter((l) => {
      if (!filterText) return true;
      const q = filterText.toLowerCase();
      return (
        l.event.toLowerCase().includes(q) ||
        (l.node_name && l.node_name.toLowerCase().includes(q)) ||
        l.level.toLowerCase().includes(q) ||
        l.type.toLowerCase().includes(q)
      );
    })
  );

  function formatTime(nanos: number): string {
    const d = new Date(Math.floor(nanos / 1_000_000));
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function handleDiagnose(log: EventLogEnt) {
    diagnosingLog = log;
    diagnosisResult = null;
    diagError = null;
    isDiagnosing = false;
    copied = false;
    aiLanguage = isJaLocale ? 'ja' : 'en';
  }

  async function runDiagnose() {
    if (!diagnosingLog || isDiagnosing) return;
    isDiagnosing = true;
    diagnosisResult = null;
    diagError = null;

    try {
      const nodeCtx = diagnosingLog.node_name ? `Node: ${diagnosingLog.node_name} (${diagnosingLog.node_id || 'N/A'})` : 'Unknown Node';
      const ans = await diagnoseAlert(diagnosingLog.event, nodeCtx, aiLanguage);
      diagnosisResult = ans;
    } catch (e: any) {
      diagError = e.message || 'Failed to diagnose alert';
    } finally {
      isDiagnosing = false;
    }
  }

  async function handleCopy() {
    if (!diagnosisResult) return;
    try {
      await navigator.clipboard.writeText(diagnosisResult);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 2000);
    } catch (e) {
      console.error('Failed to copy diagnosis:', e);
    }
  }

  function closeDiagnosis() {
    diagnosingLog = null;
    diagnosisResult = null;
    diagError = null;
    isDiagnosing = false;
  }
</script>

<div class="bg-slate-800/80 border border-slate-700/70 rounded-xl p-5 shadow-sm space-y-4">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
    <div class="flex items-center space-x-2">
      <AlertTriangle class="w-5 h-5 text-amber-400" />
      <h3 class="text-base font-semibold text-slate-100">Event & Alert Log</h3>
      <span class="text-xs bg-slate-700 text-slate-300 px-2 py-0.5 rounded-full font-mono">
        {filteredLogs.length}
      </span>
    </div>

    <div class="relative w-full sm:w-64">
      <Search class="w-4 h-4 absolute left-3 top-2.5 text-slate-400" />
      <input
        type="text"
        bind:value={filterText}
        placeholder="Filter logs or nodes..."
        class="w-full pl-9 pr-3 py-1.5 bg-slate-900 border border-slate-700 rounded-lg text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
      />
    </div>
  </div>

  <div class="overflow-x-auto rounded-lg border border-slate-700/60 max-h-[420px] overflow-y-auto">
    <table class="w-full text-left text-sm text-slate-300">
      <thead class="bg-slate-900/90 text-xs uppercase text-slate-400 sticky top-0 backdrop-blur z-10">
        <tr>
          <th class="px-4 py-3">Time</th>
          <th class="px-4 py-3">Severity</th>
          <th class="px-4 py-3">Type</th>
          <th class="px-4 py-3">Target Node</th>
          <th class="px-4 py-3">Event Message</th>
          <th class="px-4 py-3 text-right">Action</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-700/50 bg-slate-800/40">
        {#if filteredLogs.length === 0}
          <tr>
            <td colspan="6" class="px-4 py-8 text-center text-slate-500">
              No event logs found
            </td>
          </tr>
        {:else}
          {#each filteredLogs as log}
            <tr class="hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 text-xs text-slate-400 font-mono whitespace-nowrap">
                {formatTime(log.time)}
              </td>
              <td class="px-4 py-2.5 whitespace-nowrap">
                {#if log.level === 'error'}
                  <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded bg-red-950/80 text-red-400 border border-red-800/50">
                    <AlertCircle class="w-3 h-3" /> Error
                  </span>
                {:else if log.level === 'warn' || log.level === 'high'}
                  <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded bg-amber-950/80 text-amber-400 border border-amber-800/50">
                    <AlertTriangle class="w-3 h-3" /> {log.level}
                  </span>
                {:else}
                  <span class="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded bg-blue-950/80 text-blue-400 border border-blue-800/50">
                    <Info class="w-3 h-3" /> {log.level || 'info'}
                  </span>
                {/if}
              </td>
              <td class="px-4 py-2.5 text-xs text-slate-400 font-mono whitespace-nowrap">
                {log.type}
              </td>
              <td class="px-4 py-2.5 text-xs font-medium text-slate-200 whitespace-nowrap">
                {log.node_name || log.node_id || '-'}
              </td>
              <td class="px-4 py-2.5 text-sm text-slate-200">
                {log.event}
              </td>
              <td class="px-4 py-2.5 text-right whitespace-nowrap">
                <button
                  onclick={() => handleDiagnose(log)}
                  class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium bg-purple-600/20 text-purple-300 hover:bg-purple-600/30 border border-purple-500/40 rounded transition-colors"
                >
                  <Sparkles class="w-3.5 h-3.5 text-purple-400" />
                  AI Diagnose
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

<!-- AI Diagnosis Modal -->
{#if diagnosingLog}
  <div class="fixed inset-0 bg-black/75 backdrop-blur-sm z-50 flex items-center justify-center p-3 sm:p-6 animate-in fade-in duration-150">
    <div class="bg-slate-800 border border-slate-700 rounded-2xl shadow-2xl max-w-4xl w-full h-[90vh] max-h-[90vh] flex flex-col overflow-hidden text-slate-100">
      <!-- Header -->
      <div class="shrink-0 flex items-center justify-between border-b border-slate-700 px-6 py-4 bg-slate-900/60">
        <div class="flex items-center gap-2.5 text-base font-bold text-purple-400">
          <Sparkles class="w-5 h-5" />
          <span>{$_('alert.aiDiagnoseTitle') || 'AI アラート診断'}</span>
        </div>
        <button
          onclick={closeDiagnosis}
          class="rounded-lg p-1.5 text-slate-400 hover:text-white hover:bg-slate-700 text-lg font-bold transition-colors cursor-pointer"
        >
          &times;
        </button>
      </div>

      <!-- Target Event Details Preview -->
      <div class="shrink-0 px-6 pt-3 pb-1">
        <div class="bg-slate-900/80 p-3 rounded-xl border border-slate-700 text-xs space-y-1 shadow-inner">
          <div class="text-[11px] font-semibold text-slate-400">Target Event:</div>
          <div class="text-sm font-medium text-amber-300 font-mono break-all">{diagnosingLog.event}</div>
          <div class="text-slate-400 font-mono">Node: {diagnosingLog.node_name || 'N/A'} ({diagnosingLog.node_id || '-'}) | Level: <span class="capitalize font-semibold text-slate-200">{diagnosingLog.level}</span></div>
        </div>
      </div>

      <!-- Condition Setting & Action Bar -->
      <div class="shrink-0 px-6 py-2.5 flex flex-wrap items-center justify-between gap-3 border-y border-slate-700/80 bg-slate-900/40">
        <div class="flex items-center gap-3">
          {#if isJaLocale}
            <!-- Language selection (when UI is Japanese) -->
            <div class="flex items-center gap-2 text-xs">
              <span class="flex items-center gap-1 font-semibold text-slate-300">
                <Languages class="h-3.5 w-3.5 text-purple-400" />
                <span>{$_('log.aiQueryLanguage') || '回答言語'}:</span>
              </span>
              <div class="inline-flex rounded-lg border border-slate-700 p-0.5 bg-slate-800 shadow-xs">
                <button
                  type="button"
                  disabled={isDiagnosing}
                  onclick={() => (aiLanguage = "ja")}
                  class="px-2.5 py-1 text-xs font-medium rounded-md transition-colors cursor-pointer {aiLanguage === 'ja' ? 'bg-purple-600 text-white shadow-xs font-semibold' : 'text-slate-300 hover:text-white'}"
                >
                  {$_('log.aiLangJa') || '日本語'}
                </button>
                <button
                  type="button"
                  disabled={isDiagnosing}
                  onclick={() => (aiLanguage = "en")}
                  class="px-2.5 py-1 text-xs font-medium rounded-md transition-colors cursor-pointer {aiLanguage === 'en' ? 'bg-purple-600 text-white shadow-xs font-semibold' : 'text-slate-300 hover:text-white'}"
                >
                  {$_('log.aiLangEn') || '英語 (English)'}
                </button>
              </div>
            </div>
          {/if}
        </div>

        <!-- Query Execution Button -->
        <button
          type="button"
          onclick={runDiagnose}
          disabled={isDiagnosing}
          class="flex items-center gap-2 rounded-xl bg-gradient-to-r from-purple-600 to-indigo-600 hover:from-purple-500 hover:to-indigo-500 disabled:opacity-50 text-white px-4 py-2 text-xs font-bold transition-all shadow-md shadow-purple-600/20 cursor-pointer active:scale-95"
        >
          <Sparkles class="h-4 w-4" />
          <span>{diagnosisResult ? ($_('alert.aiReanalyze') || '再診断を実行') : ($_('alert.aiDiagnoseStart') || 'AIアラート診断を実行')}</span>
        </button>
      </div>

      <!-- AI Response Area -->
      <div class="flex-1 min-h-0 overflow-y-auto mx-6 my-3 rounded-xl border border-slate-700 bg-slate-900/50 p-5 text-sm leading-relaxed text-slate-200 shadow-inner">
        {#if isDiagnosing}
          <!-- Animated Cat Logo WAIT Screen -->
          <div class="flex flex-col items-center justify-center py-16 space-y-4">
            <AnimatedCatLogo mode="loading" size="lg" />
            <div class="text-center space-y-1.5 max-w-md">
              <p class="text-sm font-bold text-purple-400 animate-pulse">
                {aiLanguage === 'ja' ? 'マルチLLM推論中... アラート文脈とトポロジーを分析しています' : 'Analyzing alert context and inferring root cause with multi-LLM...'}
              </p>
              <p class="text-xs text-slate-400">
                {$_('log.aiWaiting') || 'しばらくお待ちください...'}
              </p>
            </div>
          </div>
        {:else if diagError}
          <div class="p-4 bg-red-950/50 border border-red-800/60 text-red-300 rounded-xl text-xs space-y-3">
            <div class="flex items-center gap-2 font-semibold">
              <AlertCircle class="h-4 w-4 shrink-0 text-red-400" />
              <span>{$_('common.error')}: {diagError}</span>
            </div>
            <button
              type="button"
              onclick={runDiagnose}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 text-white text-xs font-medium cursor-pointer shadow-xs"
            >
              <RefreshCw class="h-3.5 w-3.5" />
              <span>{$_('common.retry') || '再試行'}</span>
            </button>
          </div>
        {:else if diagnosisResult}
          <div class="p-2">
            <AIMarkdown content={diagnosisResult} />
          </div>
        {:else}
          <!-- Idle Guidance -->
          <div class="flex flex-col items-center justify-center py-20 text-center space-y-3">
            <div class="p-3 rounded-2xl bg-purple-950/40 text-purple-400 border border-purple-800/50">
              <Sparkles class="h-8 w-8" />
            </div>
            <div class="space-y-1 max-w-md">
              <h4 class="text-sm font-bold text-slate-200">
                {$_('alert.aiDiagnoseTitle') || 'AI アラート診断'}
              </h4>
              <p class="text-xs text-slate-400 leading-relaxed">
                {$_('alert.aiDiagnosePlaceholder') || '言語等の条件を確認し、「AIアラート診断を実行」を押してください。'}
              </p>
            </div>
          </div>
        {/if}
      </div>

      <!-- Footer Actions -->
      <div class="shrink-0 flex items-center justify-between border-t border-slate-700 px-6 py-3.5 bg-slate-900/60">
        <div>
          {#if diagnosisResult && !isDiagnosing}
            <button
              type="button"
              onclick={handleCopy}
              class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer shadow-xs"
            >
              {#if copied}
                <Check class="h-4 w-4 text-emerald-400" />
                <span class="text-emerald-400 font-medium">{$_('common.copied')}</span>
              {:else}
                <Copy class="h-4 w-4" />
                <span>{$_('common.copy')}</span>
              {/if}
            </button>
          {/if}
        </div>
        <button
          onclick={closeDiagnosis}
          class="rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-5 py-2 text-xs font-semibold text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_('common.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
