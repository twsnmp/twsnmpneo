<script lang="ts">
  import { onMount, tick } from "svelte";
  import { _ } from "svelte-i18n";
  import { BarChart3, RotateCcw } from "@lucide/svelte";
  import { showLogLevelChart, resizeLogLevelChart, disposeLogLevelChart } from "../../charts/loglevel";
  import { showLogCountChart, resizeLogCountChart, disposeLogCountChart } from "../../charts/logcount";
  import type { LogCategory, LogItem } from "./types";

  interface Props {
    activeTab: LogCategory;
    logs: LogItem[];
    chartZoomRange: { st: number; et: number } | null;
  }

  let {
    activeTab,
    logs,
    chartZoomRange = $bindable(null),
  }: Props = $props();

  let showChart = $state(true);
  let chartContainer: HTMLDivElement | null = $state(null);

  export const renderChart = async () => {
    await tick();
    if (!showChart || !chartContainer) return;

    if (activeTab === "event" || activeTab === "syslog") {
      showLogLevelChart(chartContainer, logs, (st, et) => {
        if (st && et) {
          chartZoomRange = { st, et };
        } else {
          chartZoomRange = null;
        }
      });
    } else {
      showLogCountChart(chartContainer, logs, (st, et) => {
        if (st && et) {
          chartZoomRange = { st, et };
        } else {
          chartZoomRange = null;
        }
      });
    }
  };

  const handleToggleChart = async (show: boolean) => {
    showChart = show;
    if (show) {
      await renderChart();
    }
  };

  onMount(() => {
    renderChart();
    const handleResize = () => {
      resizeLogLevelChart();
      resizeLogCountChart();
    };
    window.addEventListener("resize", handleResize);
    return () => {
      window.removeEventListener("resize", handleResize);
      disposeLogLevelChart();
      disposeLogCountChart();
    };
  });

  $effect(() => {
    activeTab;
    logs;
    if (showChart) {
      renderChart();
    }
  });
</script>

{#if showChart}
  <div
    class="relative rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 p-3 shadow-sm dark:shadow-lg shrink-0 transition-all"
  >
    <div class="flex items-center justify-between mb-1 px-1">
      <div class="flex items-center gap-2">
        <span class="text-[11px] font-bold text-slate-700 dark:text-slate-300">{$_('log.chartTitle')}</span>
        {#if chartZoomRange}
          <span
            class="inline-flex items-center gap-1 rounded-full bg-cyan-100 dark:bg-cyan-950/80 border border-cyan-300 dark:border-cyan-800 px-2 py-0.5 text-[10px] text-cyan-700 dark:text-cyan-300 font-mono"
          >
            {$_('log.filterPeriod')}
          </span>
          <button
            type="button"
            onclick={() => (chartZoomRange = null)}
            class="flex items-center gap-1 rounded-md bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-2 py-0.5 text-[10px] text-slate-700 dark:text-slate-300 cursor-pointer"
          >
            <RotateCcw class="h-3 w-3" />
            <span>{$_('common.clear')}</span>
          </button>
        {/if}
      </div>
      <button
        type="button"
        onclick={() => handleToggleChart(false)}
        aria-label="Collapse chart"
        class="text-[10px] text-slate-500 dark:text-slate-400 hover:text-slate-800 dark:hover:text-slate-200 cursor-pointer"
      >
        ▲
      </button>
    </div>
    <div bind:this={chartContainer} class="h-64 min-h-[250px] w-full"></div>
  </div>
{:else}
  <div class="flex justify-end shrink-0">
    <button
      type="button"
      onclick={() => handleToggleChart(true)}
      class="flex items-center gap-1 text-[11px] text-cyan-600 dark:text-cyan-400 hover:text-cyan-500 dark:hover:text-cyan-300 cursor-pointer"
    >
      <BarChart3 class="h-3.5 w-3.5" />
      <span>{$_('log.chartToggle')} ▼</span>
    </button>
  </div>
{/if}
