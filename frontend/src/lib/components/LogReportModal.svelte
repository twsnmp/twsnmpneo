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
      const topNodes = stats.nodeStats.slice(0, 5).map((n) => `${n.nodeName}: SLA ${renderSLA(n.sla)}, 停止時間 ${renderDuration(n.totalDowntimeSec)} (${n.count}回)`).join("; ");
      const prompt = `以下のネットワーク監視ログ集計データを解析し、システム全体の健全性評価、主なボトルネック、および具体的な推奨改善アクションを専門家として簡潔にレポートしてください。\n\n` +
        `【集計データ】\n` +
        `- ログ対象件数: ${logs.length} 件\n` +
        `- 全体稼働率 (SLA): ${renderSLA(stats.overallSLA)}\n` +
        `- 障害発生件数: ${stats.totalIncidents} 件 (現在障害中: ${stats.ongoingIncidents} 件)\n` +
        `- 総ダウンタイム: ${renderDuration(stats.totalDowntimeSec)}\n` +
        `- 平均復旧時間 (MTTR): ${renderDuration(stats.mttrSec)}\n` +
        `- 障害上位ノード: ${topNodes || "なし"}`;

      aiSummary = await askAI(prompt, "あなたは高可用性ネットワーク運用のシニアインフラエンジニアです。");
    } catch (e: any) {
      aiSummary = `AIレポート生成エラー: ${e.message}`;
    } finally {
      aiLoading = false;
    }
  };

  const exportReportCSV = () => {
    const stats = calcEventLogDowntimeAndSLA(logs);
    let csv = "ノード名,稼働率(SLA),障害回数,総停止時間(秒),最大停止時間(秒),現在の状態\n";
    csv += stats.nodeStats
      .map((n) => `"${n.nodeName}","${n.sla.toFixed(3)}%","${n.count}","${n.totalDowntimeSec}","${n.maxDowntimeSec}","${n.ongoing ? '障害発生中' : n.currentLevel}"`)
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
    <div class="flex h-[90vh] w-full max-w-5xl flex-col rounded-3xl border border-slate-800 bg-slate-900 shadow-2xl overflow-hidden text-slate-100">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-800 px-6 py-4 bg-slate-950/70">
        <div class="flex items-center gap-3">
          <div class="rounded-xl bg-cyan-500/10 p-2 text-cyan-400 border border-cyan-500/20">
            <BarChart3 class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">
              ログ総合アナリティクスレポート ({logCategory.toUpperCase()})
            </h2>
            <p class="text-[11px] text-slate-400">
              対象レコード: <span class="font-mono text-cyan-400 font-bold">{logs.length.toLocaleString()}</span> 件 の統計分析
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
            <span>CSVエクスポート</span>
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
          <span>稼働率・障害 (SLA)</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("state")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'state' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <PieChart class="h-4 w-4" />
          <span>重要度分布</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("heatmap")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'heatmap' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Calendar class="h-4 w-4" />
          <span>時間帯ヒートマップ</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("nodes")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'nodes' ? 'border-cyan-500 text-cyan-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Server class="h-4 w-4" />
          <span>ノード別障害件数</span>
        </button>

        <button
          type="button"
          onclick={() => handleTabChange("ai")}
          class="flex items-center gap-2 border-b-2 px-4 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'ai' ? 'border-pink-500 text-pink-400 font-bold' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <Sparkles class="h-4 w-4 text-pink-400" />
          <span>AI 総合分析要約</span>
        </button>
      </div>

      <!-- Tab Content Body -->
      <div class="flex-1 overflow-y-auto p-6 min-h-0 bg-slate-900/60">
        {#if activeTab === "downtime"}
          <div class="space-y-6">
            <!-- KPI Summary Cards -->
            <div class="grid grid-cols-2 sm:grid-cols-5 gap-3.5">
              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">全体稼働率 (SLA)</span>
                <div class="text-xl font-mono font-black text-emerald-400">
                  {renderSLA(downtimeStats.overallSLA)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">総障害件数</span>
                <div class="text-xl font-mono font-black text-rose-400">
                  {downtimeStats.totalIncidents.toLocaleString()} <span class="text-xs font-sans text-slate-400 font-normal">回</span>
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">総停止時間</span>
                <div class="text-sm font-sans font-bold text-slate-200 truncate mt-1">
                  {renderDuration(downtimeStats.totalDowntimeSec)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">平均復旧時間 (MTTR)</span>
                <div class="text-sm font-sans font-bold text-slate-200 truncate mt-1">
                  {renderDuration(downtimeStats.mttrSec)}
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
                <span class="text-[11px] font-medium text-slate-400 block mb-1">現在障害中ノード</span>
                <div class="text-xl font-mono font-black {downtimeStats.ongoingIncidents > 0 ? 'text-red-400 animate-pulse' : 'text-slate-400'}">
                  {downtimeStats.ongoingIncidents} <span class="text-xs font-sans text-slate-400 font-normal">台</span>
                </div>
              </div>
            </div>

            <!-- Top Downtime Chart -->
            <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-4 shadow-sm">
              <h3 class="text-xs font-bold text-slate-300 mb-2">ノード別 累積停止時間ランキング (上位15)</h3>
              <div id="reportDowntimeChart" class="h-64 w-full"></div>
            </div>

            <!-- Downtime Table -->
            <div class="rounded-2xl border border-slate-800 bg-slate-950/70 overflow-hidden shadow-sm">
              <div class="px-4 py-3 border-b border-slate-800 text-xs font-bold text-slate-300">
                ノード別稼働実績一覧 ({downtimeStats.nodeStats.length} ノード)
              </div>
              <div class="max-h-60 overflow-y-auto">
                <table class="w-full text-left text-xs">
                  <thead class="sticky top-0 bg-slate-900 border-b border-slate-800 text-[10px] uppercase text-slate-400">
                    <tr>
                      <th class="py-2.5 px-3.5">ノード名</th>
                      <th class="py-2.5 px-3.5 text-right">稼働率 (SLA)</th>
                      <th class="py-2.5 px-3.5 text-right">障害回数</th>
                      <th class="py-2.5 px-3.5 text-right">総停止時間</th>
                      <th class="py-2.5 px-3.5 text-right">最大停止時間</th>
                      <th class="py-2.5 px-3.5 text-center">現在の状態</th>
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
                              障害発生中
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
                        <td colspan="6" class="py-8 text-center text-slate-500 font-sans">障害履歴はありません</td>
                      </tr>
                    {/if}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        {:else if activeTab === "state"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-4">ログ重要度レベル分布</h3>
            <div id="reportStateChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "heatmap"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-2">曜日・時間帯別 ログ発生ヒートマップ (24時間 × 7日間)</h3>
            <div id="reportHeatmapChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "nodes"}
          <div class="rounded-2xl border border-slate-800 bg-slate-950/70 p-6">
            <h3 class="text-xs font-bold text-slate-300 mb-2">ノード別 発生イベント件数ランキング (上位15)</h3>
            <div id="reportNodesChart" class="h-96 w-full"></div>
          </div>
        {:else if activeTab === "ai"}
          <div class="space-y-4">
            <div class="flex items-center justify-between rounded-2xl border border-pink-500/20 bg-pink-500/5 p-4">
              <div class="flex items-center gap-2.5 text-xs text-pink-300">
                <Sparkles class="h-4 w-4" />
                <span>AIによる統計解析と推奨運用アクション</span>
              </div>
              <button
                type="button"
                onclick={runAIReport}
                disabled={aiLoading}
                class="flex items-center gap-1.5 rounded-xl border border-pink-500/30 bg-pink-500/10 hover:bg-pink-500/20 px-3.5 py-1.5 text-xs font-semibold text-pink-300 transition-colors cursor-pointer"
              >
                <RefreshCw class="h-3.5 w-3.5 {aiLoading ? 'animate-spin' : ''}" />
                <span>再分析</span>
              </button>
            </div>

            <div class="rounded-2xl border border-slate-800 bg-slate-950/90 p-6 text-xs leading-relaxed text-slate-200 whitespace-pre-wrap font-sans">
              {#if aiLoading}
                <div class="flex items-center justify-center py-16 gap-3 text-cyan-400">
                  <RefreshCw class="h-5 w-5 animate-spin" />
                  <span class="text-sm font-semibold">マルチLLM推論中... ログ統計データを分析しています</span>
                </div>
              {:else if aiSummary}
                {aiSummary}
              {:else}
                <div class="py-12 text-center text-slate-500">
                  「再分析」ボタンを押してAIレポートを生成してください
                </div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
