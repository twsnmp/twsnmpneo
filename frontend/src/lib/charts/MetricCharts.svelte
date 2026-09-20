<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import * as echarts from 'echarts';
  import type { NodeEnt, EventLogEnt } from '$lib/api';

  let { nodes = [], eventLogs = [] }: {
    nodes: NodeEnt[];
    eventLogs: EventLogEnt[];
  } = $props();

  let stateChartEl: HTMLDivElement;
  let logChartEl: HTMLDivElement;

  let stateChart: echarts.ECharts | null = null;
  let logChart: echarts.ECharts | null = null;

  function updateCharts() {
    if (!stateChart || !logChart) return;

    // 1. Calculate node state counts
    const stateCounts: Record<string, number> = {
      normal: 0,
      warn: 0,
      error: 0,
      unknown: 0,
    };
    for (const n of nodes) {
      const s = (n.state || 'unknown').toLowerCase();
      if (s in stateCounts) {
        stateCounts[s]++;
      } else {
        stateCounts.unknown++;
      }
    }

    stateChart.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)',
      },
      legend: {
        bottom: '0%',
        textStyle: { color: '#94a3b8' },
      },
      series: [
        {
          name: 'Node Status',
          type: 'pie',
          radius: ['45%', '70%'],
          avoidLabelOverlap: false,
          itemStyle: {
            borderRadius: 6,
            borderColor: '#0f172a',
            borderWidth: 2,
          },
          label: { show: false },
          data: [
            { value: stateCounts.normal, name: 'Normal', itemStyle: { color: '#10b981' } },
            { value: stateCounts.warn, name: 'Warning', itemStyle: { color: '#f59e0b' } },
            { value: stateCounts.error, name: 'Error', itemStyle: { color: '#ef4444' } },
            { value: stateCounts.unknown, name: 'Unknown', itemStyle: { color: '#64748b' } },
          ],
        },
      ],
    });

    // 2. Calculate event logs by level
    const levelCounts: Record<string, number> = {
      info: 0,
      warn: 0,
      high: 0,
      error: 0,
    };
    for (const l of eventLogs) {
      const lev = (l.level || 'info').toLowerCase();
      if (lev in levelCounts) {
        levelCounts[lev]++;
      } else {
        levelCounts.info++;
      }
    }

    logChart.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
      },
      grid: {
        top: '12%',
        left: '8%',
        right: '8%',
        bottom: '15%',
      },
      xAxis: {
        type: 'category',
        data: ['Info', 'Warning', 'High', 'Error'],
        axisLine: { lineStyle: { color: '#475569' } },
        axisLabel: { color: '#94a3b8' },
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: '#1e293b' } },
        axisLabel: { color: '#94a3b8' },
      },
      series: [
        {
          name: 'Count',
          type: 'bar',
          barWidth: '40%',
          data: [
            { value: levelCounts.info, itemStyle: { color: '#3b82f6', borderRadius: [4, 4, 0, 0] } },
            { value: levelCounts.warn, itemStyle: { color: '#f59e0b', borderRadius: [4, 4, 0, 0] } },
            { value: levelCounts.high, itemStyle: { color: '#f97316', borderRadius: [4, 4, 0, 0] } },
            { value: levelCounts.error, itemStyle: { color: '#ef4444', borderRadius: [4, 4, 0, 0] } },
          ],
        },
      ],
    });
  }

  $effect(() => {
    // Re-render charts when nodes or eventLogs change
    if (nodes || eventLogs) {
      updateCharts();
    }
  });

  onMount(() => {
    stateChart = echarts.init(stateChartEl);
    logChart = echarts.init(logChartEl);
    updateCharts();

    const resizeHandler = () => {
      stateChart?.resize();
      logChart?.resize();
    };
    window.addEventListener('resize', resizeHandler);

    return () => {
      window.removeEventListener('resize', resizeHandler);
      stateChart?.dispose();
      logChart?.dispose();
    };
  });
</script>

<div class="grid grid-cols-1 lg:grid-cols-2 gap-4 w-full">
  <div class="bg-slate-800/80 border border-slate-700/70 rounded-xl p-5 shadow-sm">
    <h3 class="text-sm font-semibold text-slate-200 mb-2">Node State Breakdown</h3>
    <div bind:this={stateChartEl} class="w-full h-56"></div>
  </div>

  <div class="bg-slate-800/80 border border-slate-700/70 rounded-xl p-5 shadow-sm">
    <h3 class="text-sm font-semibold text-slate-200 mb-2">Event Severity Distribution</h3>
    <div bind:this={logChartEl} class="w-full h-56"></div>
  </div>
</div>
