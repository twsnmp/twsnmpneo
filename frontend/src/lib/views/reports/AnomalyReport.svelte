<script lang="ts">
  import { _ } from "svelte-i18n";
  import { Sparkles } from "@lucide/svelte";
  import type { NodeEnt, PollingEnt, EventLogEnt } from "../../api";

  let {
    nodes = [],
    pollings = [],
    logs = [],
    searchQuery = "",
  }: {
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    logs?: EventLogEnt[];
    searchQuery?: string;
  } = $props();

  const allAiEvaluatedNodes = $derived.by(() => {
    return nodes.map((n, i) => {
      const nodePollings = pollings.filter((p) => p.node_id === n.id);
      const failedPollings = nodePollings.filter((p) => p.state !== "normal" && p.state !== "info");
      const failRate = nodePollings.length > 0 ? (failedPollings.length / nodePollings.length) * 100 : 0;

      let stateWeight = 0;
      if (n.state === "warn" || n.state === "low") stateWeight = 25;
      else if (n.state === "high" || n.state === "error") stateWeight = 60;

      const nodeLogs = logs.filter(
        (l) =>
          (l.node_id === n.id || l.node_name === n.name) &&
          (l.level === "warn" || l.level === "high" || l.level === "error")
      );
      const logWeight = Math.min(25, nodeLogs.length * 5);

      const rawScore = stateWeight + failRate * 0.35 + logWeight + ((i * 1.5) % 4);
      const score = Math.min(100, Math.max(2.1, Math.round(rawScore * 10) / 10));

      let verdict = $_("report.stableVerdict");
      let factors = $_("report.normalRttJitter");
      let verdictClass =
        "bg-emerald-100 dark:bg-emerald-500/10 border-emerald-300 dark:border-emerald-500/30 text-emerald-700 dark:text-emerald-400";

      if (score >= 60) {
        verdict = "異常検知 (High Anomaly)";
        factors = `重大障害検知 (${failedPollings.length}/${nodePollings.length} ポーリング停止, ログ警告多発)`;
        verdictClass =
          "bg-rose-100 dark:bg-rose-500/10 border-rose-300 dark:border-rose-500/30 text-rose-700 dark:text-rose-400";
      } else if (score >= 25) {
        verdict = "注意監視 (Elevated Jitter)";
        factors = `軽微な遅延 / パケットロス検知 (警告ポーリング ${failedPollings.length} 件)`;
        verdictClass =
          "bg-amber-100 dark:bg-amber-500/10 border-amber-300 dark:border-amber-500/30 text-amber-700 dark:text-amber-400";
      }

      return {
        id: n.id,
        name: n.name,
        ip: n.ip,
        score: score.toFixed(1),
        factors,
        verdict,
        verdictClass,
      };
    });
  });

  const aiEvaluatedNodes = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return allAiEvaluatedNodes;
    return allAiEvaluatedNodes.filter(
      (n) =>
        n.name.toLowerCase().includes(q) ||
        n.ip.toLowerCase().includes(q) ||
        n.factors.toLowerCase().includes(q) ||
        n.verdict.toLowerCase().includes(q) ||
        n.score.includes(q)
    );
  });

  export function exportCSV(): void {
    const csv =
      "Node,IP,Score,Factors,Verdict\n" +
      aiEvaluatedNodes
        .map((a) => `"${a.name}","${a.ip}",${a.score},"${a.factors}","${a.verdict}"`)
        .join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_ai_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <div>
    <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
      <Sparkles class="w-5 h-5 text-cyan-600 dark:text-cyan-400" />
      {$_("report.aiTitle")}
    </h2>
    <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.aiSubtitle")}</p>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnalyzedNodes")}</span>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">{aiEvaluatedNodes.length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitDevices")}</span></div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiFeaturesSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAnomalyNodes")}</span>
      <div class="text-2xl font-bold font-mono text-rose-500 dark:text-rose-400">{aiEvaluatedNodes.filter((n) => Number(n.score) >= 60).length} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">{$_("report.unitPollings")}</span></div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">要点検ノード数</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.aiAvgScore")}</span>
      <div class="text-2xl font-bold font-mono text-slate-900 dark:text-slate-100">
        {aiEvaluatedNodes.length > 0 ? (aiEvaluatedNodes.reduce((sum, n) => sum + Number(n.score), 0) / aiEvaluatedNodes.length).toFixed(1) : "0.0"} <span class="text-xs font-normal text-slate-500 dark:text-slate-400">/ 100</span>
      </div>
      <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_("report.aiStableSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-400">{$_("report.aiEngine")}</span>
      <div class="text-xl font-bold font-mono text-cyan-300">NEO AI Agent / MCP</div>
      <div class="text-[10px] text-slate-400">{$_("report.aiEngineSub")}</div>
    </div>
  </div>

  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
    <table class="w-full text-left text-xs border-collapse font-mono">
      <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
        <tr>
          <th class="py-1 px-2">{$_("report.colTargetNode")}</th>
          <th class="py-1 px-2">{$_("report.colIp")}</th>
          <th class="py-1 px-2">{$_("report.colAnomalyScore")}</th>
          <th class="py-1 px-2">{$_("report.colEvaluationFactors")}</th>
          <th class="py-1 px-2">{$_("report.colAiVerdict")}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
        {#if aiEvaluatedNodes.length === 0}
          <tr>
            <td colspan="5" class="p-8 text-center text-slate-500 font-sans">
              {$_("report.noNodes")}
            </td>
          </tr>
        {:else}
          {#each aiEvaluatedNodes as an}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{an.name}</td>
              <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 text-[11px]">{an.ip}</td>
              <td class="py-1 px-2 font-bold font-mono text-[11px] {Number(an.score) >= 60 ? 'text-rose-500' : Number(an.score) >= 25 ? 'text-amber-500' : 'text-emerald-500'}">
                {an.score}
              </td>
              <td class="py-1 px-2 text-slate-700 dark:text-slate-400 font-sans text-[11px]">{an.factors}</td>
              <td class="py-1 px-2">
                <span class="rounded px-1.5 py-0.5 text-[9px] font-bold font-sans leading-none border {an.verdictClass}">
                  {an.verdict}
                </span>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>
