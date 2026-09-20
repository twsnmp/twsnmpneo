<script lang="ts">
  import type { EventLogEnt } from '$lib/api';
  import { diagnoseAlert } from '$lib/api';
  import { Sparkles, AlertCircle, AlertTriangle, Info, CheckCircle, Search, Loader2 } from '@lucide/svelte';

  let { eventLogs = [], onRefresh }: {
    eventLogs: EventLogEnt[];
    onRefresh?: () => void;
  } = $props();

  let filterText = $state('');
  let diagnosingLog = $state<EventLogEnt | null>(null);
  let diagnosisResult = $state<string | null>(null);
  let isDiagnosing = $state(false);
  let diagError = $state<string | null>(null);

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

  async function handleDiagnose(log: EventLogEnt) {
    diagnosingLog = log;
    diagnosisResult = null;
    diagError = null;
    isDiagnosing = true;

    try {
      const nodeCtx = log.node_name ? `Node: ${log.node_name} (${log.node_id || 'N/A'})` : 'Unknown Node';
      const ans = await diagnoseAlert(log.event, nodeCtx);
      diagnosisResult = ans;
    } catch (e: any) {
      diagError = e.message || 'Failed to diagnose alert';
    } finally {
      isDiagnosing = false;
    }
  }

  function closeDiagnosis() {
    diagnosingLog = null;
    diagnosisResult = null;
    diagError = null;
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
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="bg-slate-800 border border-slate-700 rounded-xl shadow-2xl max-w-xl w-full p-6 space-y-4">
      <div class="flex items-center justify-between border-b border-slate-700 pb-3">
        <div class="flex items-center gap-2">
          <Sparkles class="w-5 h-5 text-purple-400" />
          <h3 class="text-base font-semibold text-white">AI Alert Diagnosis</h3>
        </div>
        <button
          onclick={closeDiagnosis}
          class="text-slate-400 hover:text-slate-200 text-lg font-bold"
        >
          &times;
        </button>
      </div>

      <div class="bg-slate-900/60 p-3 rounded-lg border border-slate-700/60 text-xs space-y-1">
        <div class="text-slate-400">Target Event:</div>
        <div class="text-sm font-medium text-amber-300">{diagnosingLog.event}</div>
        <div class="text-slate-500 font-mono">Node: {diagnosingLog.node_name || 'N/A'} ({diagnosingLog.node_id || '-'})</div>
      </div>

      <div class="min-h-[140px] max-h-[300px] overflow-y-auto">
        {#if isDiagnosing}
          <div class="flex flex-col items-center justify-center py-8 text-slate-400 space-y-2">
            <Loader2 class="w-6 h-6 animate-spin text-purple-400" />
            <p class="text-sm">Analyzing alert context and inferring root cause...</p>
          </div>
        {:else if diagError}
          <div class="p-3 bg-red-950/50 border border-red-800/60 text-red-300 rounded-lg text-sm">
            {diagError}
          </div>
        {:else if diagnosisResult}
          <div class="prose prose-invert prose-sm text-slate-200 whitespace-pre-wrap leading-relaxed">
            {diagnosisResult}
          </div>
        {/if}
      </div>

      <div class="flex justify-end pt-2 border-t border-slate-700">
        <button
          onclick={closeDiagnosis}
          class="px-4 py-1.5 bg-slate-700 hover:bg-slate-600 text-sm font-medium rounded-lg text-white transition-colors"
        >
          Close
        </button>
      </div>
    </div>
  </div>
{/if}
