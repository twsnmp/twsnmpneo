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
  } from "@lucide/svelte";

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
    <div class="flex h-[88vh] w-[95vw] max-w-6xl flex-col rounded-2xl border border-slate-700/80 bg-slate-900/95 text-slate-100 shadow-2xl overflow-hidden backdrop-blur-md">
      <!-- Header -->
      <div class="flex shrink-0 items-center justify-between border-b border-slate-800 px-6 py-3 bg-slate-950/60">
        <div class="flex items-center gap-2">
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-400">
            <PieChart class="h-4 w-4" />
          </div>
          <div>
            <h2 class="text-sm font-bold tracking-tight text-white flex items-center gap-2">
              MQTT 受信統計レポート
              <span class="text-xs font-normal text-slate-400">({stats.length} 件)</span>
            </h2>
          </div>
        </div>

        <button
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Tab Navigation -->
      <div class="flex shrink-0 items-center justify-between border-b border-slate-800/80 bg-slate-900/80 px-6 py-2">
        <div class="flex items-center gap-1.5 overflow-x-auto">
          <button
            onclick={() => switchTab("client")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'client' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <Users class="h-3.5 w-3.5" />
            クライアント別
          </button>
          <button
            onclick={() => switchTab("remote")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'remote' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <Network class="h-3.5 w-3.5" />
            送信元別
          </button>
          <button
            onclick={() => switchTab("topic")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'topic' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <Tag class="h-3.5 w-3.5" />
            トピック別
          </button>
          <button
            onclick={() => switchTab("heatmap")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'heatmap' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <Grid class="h-3.5 w-3.5" />
            ヒートマップ
          </button>
          <button
            onclick={() => switchTab("state")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'state' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <PieChart class="h-3.5 w-3.5" />
            状態別
          </button>
          <button
            onclick={() => switchTab("treemap")}
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all {activeTab === 'treemap' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'}"
          >
            <FolderTree class="h-3.5 w-3.5" />
            トピックツリー
          </button>
        </div>

        {#if activeTab === "heatmap"}
          <div class="flex items-center gap-1 bg-slate-950 p-0.5 rounded-lg border border-slate-800">
            <button
              onclick={() => toggleHeatmapMode("time")}
              class="px-2.5 py-1 text-[11px] font-medium rounded {heatmapMode === 'time' ? 'bg-indigo-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              時間帯別
            </button>
            <button
              onclick={() => toggleHeatmapMode("client_topic")}
              class="px-2.5 py-1 text-[11px] font-medium rounded {heatmapMode === 'client_topic' ? 'bg-indigo-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              クライアント×トピック
            </button>
          </div>
        {/if}
      </div>

      <!-- Chart Content Area -->
      <div class="flex-1 overflow-hidden p-4 relative flex items-center justify-center">
        <div id="mqttChartContainer" bind:this={chartContainer} class="w-full h-full min-h-[450px]"></div>
      </div>

      <!-- Footer -->
      <div class="flex shrink-0 items-center justify-end border-t border-slate-800 bg-slate-950/60 px-6 py-2.5">
        <button
          onclick={() => (show = false)}
          class="rounded-xl border border-slate-700 bg-slate-800 px-4 py-1.5 text-xs font-semibold text-slate-300 hover:bg-slate-700 hover:text-white transition-colors"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}
