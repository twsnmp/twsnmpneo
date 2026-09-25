<script lang="ts">
  import { tick } from "svelte";
  import {
    Activity,
    BarChart3,
    Calendar,
    Clock,
    Download,
    PieChart,
    RefreshCw,
    Server,
    ShieldAlert,
    Sparkles,
    X,
  } from "@lucide/svelte";
  import {
    calcEventLogDowntimeAndSLA,
    showEventLogDowntimeChart,
    showLogHeatmap,
    showEventLogStateChart,
    showEventLogNodeChart,
    type DowntimeReportResult,
  } from "../charts/eventlog";
  import { renderDuration, renderSLA, getStateColor } from "../common";
  import { askAI } from "../api";
  import { _ } from "svelte-i18n";

  let {
    show = $bindable(false),
    logs = [],
    logCategory = "event",
  }: {
    show: boolean;
    logs: any[];
    logCategory: string;
  } = $props();

  type ReportTab = "downtime" | "state" | "heatmap" | "nodes" | "ai";
  let activeTab = $state<ReportTab>("downtime");

  let downtimeStats = $state<DowntimeReportResult>({
    totalIncidents: 0,
    ongoingIncidents: 0,
    totalDowntimeSec: 0,
    maxDowntimeSec: 0,
    mttrSec: 0,
    overallSLA: 100,
    nodeStats: [],
  });

  // AI report states
  let aiSummary = $state("");
  let aiLoading = $state(false);

  const initCharts = async () => {
    await tick();
    if (!show) return;

    if (activeTab === "downtime") {
      downtimeStats = calcEventLogDowntimeAndSLA(logs);
      showEventLogDowntimeChart("reportDowntimeChart", downtimeStats.nodeStats);
    } else if (activeTab === "state") {
      showEventLogStateChart("reportStateChart", logs);
    } else if (activeTab === "heatmap") {
      showLogHeatmap("reportHeatmapChart", logs);
    } else if (activeTab === "nodes") {
      showEventLogNodeChart("reportNodesChart", logs);
    }
  };

  $effect(() => {
    if (show) {
      initCharts();
    }
  });

  const handleTabChange = (t: ReportTab) => {
    activeTab = t;
    initCharts();
    if (t === "ai" && !aiSummary) {
      runAIReport();
    }
  };

  const runAIReport = async () => {
    aiLoading = true;
    aiSummary = "";
    try {
      const stats = calcEventLogDowntimeAndSLA(logs);
      const topNodes = stats.nodeStats.slice(0, 5).map((n) =>
        $_('logReport.aiPromptNodeItem', {
          values: {
            name: n.nodeName,
            sla: renderSLA(n.sla),
            downtime: renderDuration(n.totalDowntimeSec),
            count: n.count,
          },
        })
      ).join("; ");

      const prompt =
        $_('logReport.aiPromptHeader') +
        $_('logReport.aiPromptStats') +
        $_('logReport.aiPromptLogs', { values: { count: logs.length } }) +
        $_('logReport.aiPromptSla', { values: { sla: renderSLA(stats.overallSLA) } }) +
        $_('logReport.aiPromptIncidents', { values: { total: stats.totalIncidents, ongoing: stats.ongoingIncidents } }) +
        $_('logReport.aiPromptDowntime', { values: { downtime: renderDuration(stats.totalDowntimeSec) } }) +
        $_('logReport.aiPromptMttr', { values: { mttr: renderDuration(stats.mttrSec) } }) +
        $_('logReport.aiPromptTopNodes', { values: { nodes: topNodes || $_('logReport.aiPromptNone') } });

      aiSummary = await askAI(prompt, $_('logReport.aiSystemPrompt'));
    } catch (e: any) {
      aiSummary = $_('logReport.aiError', { values: { error: e.message } });
    } finally {
      aiLoading = false;
    }
  };

  const exportReportCSV = () => {
    const stats = calcEventLogDowntimeAndSLA(logs);
    let csv = $_('logReport.csvHeader');
    csv += stats.nodeStats
      .map((n) => `"${n.nodeName}","${n.sla.toFixed(3)}%","${n.count}","${n.totalDowntimeSec}","${n.maxDowntimeSec}","${n.ongoing ? $_('logReport.ongoing') : n.currentLevel}"`)
      .join("\n");

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_log_report_${Date.now()}.csv`;
    link.click();
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 animate-in fade-in duration-150">
    <div class="flex h-[90vh] w-full max-w-5xl flex-col rounded-3xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden text-slate-800 dark:text-slate-100">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50 dark:bg-slate-950/70">
        <div class="flex items-center gap-3">
          <div class="rounded-xl bg-cyan-500/10 p-2 text-cyan-400 border border-cyan-500/20">
            <BarChart3 class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">
              {$_('logReport.title', { values: { type: logCategory.toUpperCase() } })}
            </h2>
            <p class="text-[11px] text-slate-400">
              {$_('logReport.subtitle', { values: { count: logs.length.toLocaleString() } })}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2.5">
          <button
            type="button"
            onclick={exportReportCSV}
            class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
          >
            <Download class="h-3.5 w-3.5 text-emerald-400" />
            <span>{$_('logReport.btnExportCsv')}</span>
          </button>
          <button
            type="button"
            onclick={() => (show = false)}
            class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors cursor-pointer"
          >
            <X class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- Tab Navigation -->
      <div class="flex items-center gap-1 border-b border-slate-800 bg-slate-950/40 px-6 pt-2">
        <button
          type="button"
          onclick={() => handleTabChange("downtime")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'downtime' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <ShieldAlert class="h-4 w-4" />
          <span>{$_('logReport.tabSla')}</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("state")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'state' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <PieChart class="h-4 w-4" />
          <span>{$_('logReport.tabLevel')}</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("heatmap")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'heatmap' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Calendar class="h-4 w-4" />
          <span>{$_('logReport.tabHeatmap')}</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("nodes")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'nodes' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Server class="h-4 w-4" />
          <span>{$_('logReport.tabNodes')}</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("ai")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'ai' ? 'border-pink-500 text-pink-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Sparkles class="h-4 w-4 text-pink-400" />
          <span>{$_('logReport.tabAi')}</span>
        </button>
      </div>

      <!-- Tab Content Body -->
      <div class="flex-1 overflow-y-auto p-6 min-h-0 bg-slate-900/60">
        {#if activeTab === "downtime"}
          <div class="space-y-6">
            <!-- KPI Summary Cards -->
            <div class="grid grid-cols-2 sm:grid-cols-5 gap-3.5">
              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">{$_('logReport.overallSla')}</span>
                <div class="text-xl font-mono font-black text-emerald-400">
                  {renderSLA(downtimeStats.overallSLA)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">{$_('logReport.totalIncidents')}</span>
                <div class="text-xl font-mono font-black text-rose-400">
                  {downtimeStats.totalIncidents.toLocaleString()} <span class="text-xs font-sans text-slate-400 font-normal">{$_('logReport.timesUnit')}</span>
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">{$_('logReport.totalDowntime')}</span>
                <div class="text-sm font-sans font-bold text-slate-200 truncate mt-1">
                  {renderDuration(downtimeStats.totalDowntimeSec)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">{$_('logReport.mttr')}</span>
                <div class="text-sm font-sans font-bold text-slate-200 truncate mt-1">
                  {renderDuration(downtimeStats.mttrSec)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">{$_('logReport.ongoingIncidents')}</span>
                <div class="text-xl font-mono font-black {downtimeStats.ongoingIncidents > 0 ? 'text-red-400 animate-pulse' : 'text-slate-400'}">
                  {downtimeStats.ongoingIncidents} <span class="text-xs font-sans text-slate-400 font-normal">{$_('logReport.nodesUnit')}</span>
                </div>
              </div>
            </div>

            <!-- Top Downtime Chart -->
            <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
              <h3 class="text-xs font-bold text-slate-300 mb-2">{$_('logReport.downtimeRanking')}</h3>
              <div id="reportDowntimeChart" class="h-64 w-full"></div>
            </div>

            <!-- Downtime Table -->
            <div class="rounded-2xl border border-slate-800 bg-slate-950/70 overflow-hidden shadow-sm">
              <div class="px-4 py-3 border-b border-slate-800 text-xs font-bold text-slate-300">
                {$_('logReport.nodeReportList', { values: { count: downtimeStats.nodeStats.length } })}
              </div>
              <div class="max-h-60 overflow-y-auto">
                <table class="w-full text-left text-xs">
                  <thead class="sticky top-0 bg-slate-900 border-b border-slate-800 text-[10px] uppercase text-slate-400">
                    <tr>
                      <th class="py-2.5 px-3.5">{$_('logReport.thNode')}</th>
                      <th class="py-2.5 px-3.5 text-right">{$_('logReport.thSla')}</th>
                      <th class="py-2.5 px-3.5 text-right">{$_('logReport.thCount')}</th>
                      <th class="py-2.5 px-3.5 text-right">{$_('logReport.thTotalDown')}</th>
                      <th class="py-2.5 px-3.5 text-right">{$_('logReport.thMaxDown')}</th>
                      <th class="py-2.5 px-3.5 text-center">{$_('logReport.thStatus')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
                    {#each downtimeStats.nodeStats as node}
                      <tr class="hover:bg-slate-850/50">
                        <td class="py-2 px-3.5 font-sans font-semibold text-slate-200">{node.nodeName}</td>
                        <td class="py-2 px-3.5 text-right {node.sla < 99 ? 'text-rose-400 font-bold' : 'text-emerald-400'}">
                          {renderSLA(node.sla)}
                        </td>
                        <td class="py-2 px-3.5 text-right">{node.count}</td>
                        <td class="py-2 px-3.5 text-right font-sans">{renderDuration(node.totalDowntimeSec)}</td>
                        <td class="py-2 px-3.5 text-right font-sans">{renderDuration(node.maxDowntimeSec)}</td>
                        <td class="py-2 px-3.5 text-center font-sans">
                          {#if node.ongoing}
                            <span class="rounded px-2 py-0.5 text-[10px] font-bold uppercase bg-rose-500/20 text-rose-400 border border-rose-500/30">
                              {$_('logReport.ongoing')}
                            </span>
                          {:else}
                            <span class="inline-flex items-center gap-1 text-[11px]">
                              <span class="h-2 w-2 rounded-full" style="background-color: {getStateColor(node.currentLevel)}"></span>
                              {node.currentLevel}
                            </span>
                          {/if}
                        </td>
                      </tr>
                    {/each}
                    {#if downtimeStats.nodeStats.length === 0}
                      <tr>
                        <td colspan="6" class="py-8 text-center text-slate-500 font-sans">{$_('logReport.noIncidents')}</td>
                      </tr>
                    {/if}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        {:else if activeTab === "state"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-4">{$_('logReport.levelDistribution')}</h3>
            <div id="reportStateChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "heatmap"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-2">{$_('logReport.heatmapTitle')}</h3>
            <div id="reportHeatmapChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "nodes"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-2">{$_('logReport.nodeIncidentsRanking')}</h3>
            <div id="reportNodesChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "ai"}
          <div class="space-y-4">
            <div class="flex items-center justify-between rounded-2xl border border-pink-500/20 bg-pink-500/5 p-4">
              <div class="flex items-center gap-2.5 text-xs text-pink-300">
                <Sparkles class="h-4 w-4" />
                <span>{$_('logReport.aiSectionTitle')}</span>
              </div>
              <button
                type="button"
                onclick={runAIReport}
                disabled={aiLoading}
                class="flex items-center gap-1.5 rounded-xl border border-pink-500/30 bg-pink-500/10 hover:bg-pink-500/20 px-3.5 py-1.5 text-xs font-semibold text-pink-300 transition-colors cursor-pointer"
              >
                <RefreshCw class="h-3.5 w-3.5 {aiLoading ? 'animate-spin' : ''}" />
                <span>{$_('logReport.btnReanalyze')}</span>
              </button>
            </div>

            <div class="rounded-2xl border border-slate-800 bg-slate-950/90 p-6 text-xs leading-relaxed text-slate-200 whitespace-pre-wrap font-sans">
              {#if aiLoading}
                <div class="flex items-center justify-center py-16 gap-3 text-cyan-400">
                  <RefreshCw class="h-5 w-5 animate-spin" />
                  <span class="text-sm font-semibold">{$_('logReport.aiAnalyzing')}</span>
                </div>
              {:else if aiSummary}
                {aiSummary}
              {:else}
                <div class="py-12 text-center text-slate-500">
                  {$_('logReport.aiPlaceholder')}
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
