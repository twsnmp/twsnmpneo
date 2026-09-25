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
  import { _ } from "svelte-i18n";

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

        <button
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-white transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
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
{/if}
