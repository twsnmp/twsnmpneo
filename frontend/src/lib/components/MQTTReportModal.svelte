<script lang="ts">
  import { tick } from "svelte";
  import {
    showMqttClientIDChart,
    showMqttRemoteChart,
    showMqttTopicChart,
    showMqttHeatmap,
    showMqttStateChart,
    showMqttTopicTreemap,
    disposeChart,
  } from "../charts/mqtt";
  import type { MqttStatEnt } from "../api";
  import {
    X,
    Users,
    Network,
    Tag,
    Grid,
    PieChart,
    FolderTree,
    Sparkles,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";
  import { renderBytes } from "../common";
  import ReportAIExplainModal from "./ReportAIExplainModal.svelte";

  let {
    show = $bindable(false),
    stats = []
  } = $props<{
    show: boolean;
    stats: MqttStatEnt[];
  }>();

  type TabType = "client" | "remote" | "topic" | "heatmap" | "state" | "treemap";

  let activeTab = $state<TabType>("client");
  let heatmapMode = $state<"time" | "client_topic">("time");
  let chartContainer: HTMLDivElement | undefined = $state();
  let showAiModal = $state(false);

  // Derived statistics for AI domain explanation
  const totalMessages = $derived((stats || []).reduce((acc: number, s: MqttStatEnt) => acc + (s.Count || 0), 0));
  const totalBytes = $derived((stats || []).reduce((acc: number, s: MqttStatEnt) => acc + (s.Bytes || 0), 0));
  const warnOrLowStats = $derived((stats || []).filter((s: MqttStatEnt) => s.State === "warn" || s.State === "low"));
  const uniqueClients = $derived(new Set((stats || []).map((s: MqttStatEnt) => s.ClientID).filter(Boolean)).size);
  const uniqueRemotes = $derived(new Set((stats || []).map((s: MqttStatEnt) => s.Remote).filter(Boolean)).size);

  const aiSummaryText = $derived(
    `MQTTトピック総数: ${stats.length}件, 総メッセージ数: ${totalMessages.toLocaleString()}件, 総データ量: ${renderBytes(totalBytes)}, 接続クライアント数: ${uniqueClients}台 (接続元IP数: ${uniqueRemotes}), 異常状態(Warn/Low): ${warnOrLowStats.length}件, 現在表示タブ: ${activeTab}`
  );

  const aiItemsList = $derived.by(() => {
    const items: string[] = [];
    // 1. Anomalous state topics
    for (const s of warnOrLowStats.slice(0, 20)) {
      items.push(`[異常状態: ${s.State?.toUpperCase()}] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, Remote: ${s.Remote || "-"}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`);
    }
    // 2. High message volume topics
    const topByCount = [...stats].sort((a, b) => (b.Count || 0) - (a.Count || 0)).slice(0, 15);
    for (const s of topByCount) {
      if (!warnOrLowStats.includes(s)) {
        items.push(`[トラフィック上位] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, State: ${s.State}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`);
      }
    }
    // 3. Client grouping
    const clientMap = new Map<string, { count: number; bytes: number; topics: number; remotes: Set<string> }>();
    for (const s of stats) {
      const cid = s.ClientID || "(Unknown)";
      const existing = clientMap.get(cid) || { count: 0, bytes: 0, topics: 0, remotes: new Set<string>() };
      existing.count += s.Count || 0;
      existing.bytes += s.Bytes || 0;
      existing.topics += 1;
      if (s.Remote) existing.remotes.add(s.Remote);
      clientMap.set(cid, existing);
    }
    const topClients = [...clientMap.entries()].sort((a, b) => b[1].count - a[1].count).slice(0, 10);
    for (const [cid, info] of topClients) {
      items.push(`[主要クライアント] ClientID: ${cid}, Topics: ${info.topics}件, TotalMsgs: ${info.count.toLocaleString()}, TotalBytes: ${renderBytes(info.bytes)}, Remotes: ${[...info.remotes].join(", ") || "-"}`);
    }
    return items;
  });

  const switchTab = async (t: TabType) => {
    activeTab = t;
    await renderChart();
  };

  const renderChart = async () => {
    await tick();
    if (!show || !chartContainer) return;

    switch (activeTab) {
      case "client":
        showMqttClientIDChart("mqttChartContainer", stats);
        break;
      case "remote":
        showMqttRemoteChart("mqttChartContainer", stats);
        break;
      case "topic":
        showMqttTopicChart("mqttChartContainer", stats);
        break;
      case "heatmap":
        showMqttHeatmap("mqttChartContainer", stats, heatmapMode);
        break;
      case "state":
        showMqttStateChart("mqttChartContainer", stats);
        break;
      case "treemap":
        showMqttTopicTreemap("mqttChartContainer", stats);
        break;
    }
  };

  const toggleHeatmapMode = async (mode: "time" | "client_topic") => {
    heatmapMode = mode;
    await renderChart();
  };

  $effect(() => {
    if (show) {
      switchTab("client");
    } else {
      disposeChart();
    }
  });

  const handleResize = () => {
    if (show) {
      renderChart();
    }
  };
</script>

<svelte:window onresize={handleResize} />

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 animate-in fade-in duration-150">
    <div class="flex h-[88vh] w-[95vw] max-w-6xl flex-col rounded-2xl border border-slate-200 dark:border-slate-700/80 bg-white dark:bg-slate-900/95 text-slate-800 dark:text-slate-100 shadow-2xl overflow-hidden backdrop-blur-md">
      <!-- Header -->
      <div class="flex shrink-0 items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-3 bg-slate-50 dark:bg-slate-950/60">
        <div class="flex items-center gap-2">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400">
            <PieChart class="h-4 w-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-2">
              {$_('mqtt.reportTitle')}
              <span class="text-xs font-normal text-slate-500 dark:text-slate-400">({stats.length} {$_('mqtt.items')})</span>
            </h2>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={() => (showAiModal = true)}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-indigo-600/25 transition-all cursor-pointer"
            title={$_("report.btnAIExplain") || "AI分析"}
          >
            <Sparkles class="h-3.5 w-3.5 text-cyan-200" />
            <span>{$_("report.btnAIExplain") || "AI分析"}</span>
          </button>
          <button
            onclick={() => (show = false)}
            class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white transition-colors cursor-pointer"
          >
            <X class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- Tab Navigation -->
      <div class="flex shrink-0 items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-100/90 dark:bg-slate-900/80 px-6 py-2">
        <div class="flex items-center gap-1.5 overflow-x-auto">
          <button
            onclick={() => switchTab("client")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'client' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <Users class="h-3.5 w-3.5" />
            {$_('mqtt.reportByClient')}
          </button>
          <button
            onclick={() => switchTab("remote")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'remote' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <Network class="h-3.5 w-3.5" />
            {$_('mqtt.reportByRemote')}
          </button>
          <button
            onclick={() => switchTab("topic")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'topic' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <Tag class="h-3.5 w-3.5" />
            {$_('mqtt.reportByTopic')}
          </button>
          <button
            onclick={() => switchTab("heatmap")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'heatmap' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <Grid class="h-3.5 w-3.5" />
            {$_('mqtt.reportHeatmap')}
          </button>
          <button
            onclick={() => switchTab("state")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'state' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <PieChart class="h-3.5 w-3.5" />
            {$_('mqtt.reportByState')}
          </button>
          <button
            onclick={() => switchTab("treemap")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer {activeTab === 'treemap' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/20' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-200/80 dark:hover:bg-slate-800'}"
          >
            <FolderTree class="h-3.5 w-3.5" />
            {$_('mqtt.reportTopicTree')}
          </button>
        </div>

        {#if activeTab === "heatmap"}
          <div class="flex items-center gap-1 bg-white dark:bg-slate-950 p-0.5 rounded-lg border border-slate-200 dark:border-slate-800">
            <button
              onclick={() => toggleHeatmapMode("time")}
              class="px-2.5 py-1 text-[11px] font-medium rounded cursor-pointer transition-all {heatmapMode === 'time' ? 'bg-indigo-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
            >
              {$_('mqtt.reportHourly')}
            </button>
            <button
              onclick={() => toggleHeatmapMode("client_topic")}
              class="px-2.5 py-1 text-[11px] font-medium rounded cursor-pointer transition-all {heatmapMode === 'client_topic' ? 'bg-indigo-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
            >
              {$_('mqtt.reportClientTopic')}
            </button>
          </div>
        {/if}
      </div>

      <!-- Chart Content Area -->
      <div class="flex-1 overflow-hidden p-4 relative flex items-center justify-center">
        <div id="mqttChartContainer" bind:this={chartContainer} class="w-full h-full min-h-[450px]"></div>
      </div>

      <!-- Footer -->
      <div class="flex shrink-0 items-center justify-end border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-6 py-2.5">
        <button
          onclick={() => (show = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 cursor-pointer shadow-xs transition-colors"
        >
          {$_('mqtt.close')}
        </button>
      </div>
    </div>
  </div>

  {#if showAiModal}
    <ReportAIExplainModal
      bind:show={showAiModal}
      reportType="mqtt"
      title={$_("mqtt.aiReportTitle") || "MQTT トピック & 統計傾向 AI診断"}
      summary={aiSummaryText}
      items={aiItemsList}
    />
  {/if}
{/if}
