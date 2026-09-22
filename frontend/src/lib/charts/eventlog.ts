import * as echarts from 'echarts';

export interface NodeDowntimeStat {
  nodeID: string;
  nodeName: string;
  count: number;
  totalDowntimeSec: number;
  maxDowntimeSec: number;
  sla: number; // 0 to 100
  ongoing: boolean;
  currentLevel: string;
}

export interface DowntimeReportResult {
  totalIncidents: number;
  ongoingIncidents: number;
  totalDowntimeSec: number;
  maxDowntimeSec: number;
  mttrSec: number;
  overallSLA: number;
  nodeStats: NodeDowntimeStat[];
}

export const calcEventLogDowntimeAndSLA = (logs: any[]): DowntimeReportResult => {
  if (!logs || logs.length === 0) {
    return {
      totalIncidents: 0,
      ongoingIncidents: 0,
      totalDowntimeSec: 0,
      maxDowntimeSec: 0,
      mttrSec: 0,
      overallSLA: 100,
      nodeStats: [],
    };
  }

  // Filter logs: only polling/node events
  const validLogs = logs.filter((l) => {
    const lvl = (l.level ?? l.Level ?? '').toLowerCase();
    return lvl !== 'unknown' && lvl !== '';
  });

  if (validLogs.length === 0) {
    return {
      totalIncidents: 0,
      ongoingIncidents: 0,
      totalDowntimeSec: 0,
      maxDowntimeSec: 0,
      mttrSec: 0,
      overallSLA: 100,
      nodeStats: [],
    };
  }

  const sorted = [...validLogs].sort((a, b) => {
    const ta = a.time ?? a.Time ?? 0;
    const tb = b.time ?? b.Time ?? 0;
    return ta - tb;
  });

  const getMs = (t: number) => {
    if (!t) return 0;
    return t > 1e16 ? t / 1e6 : (t > 1e13 ? t / 1e3 : (t > 1e10 ? t : t * 1000));
  };

  const minTime = getMs(sorted[0].time ?? sorted[0].Time ?? 0);
  const maxTime = getMs(sorted[sorted.length - 1].time ?? sorted[sorted.length - 1].Time ?? 0);
  let totalSpanSec = Math.max(1, Math.floor((maxTime - minTime) / 1000));

  const isFailure = (lvl: string) => lvl === 'high' || lvl === 'low' || lvl === 'warn' || lvl === 'error';
  const isRepair = (lvl: string) => lvl === 'repair' || lvl === 'normal';

  // Group logs by node
  const byNode = new Map<string, { nodeName: string; logs: any[] }>();
  sorted.forEach((l) => {
    const nid = l.node_id ?? l.NodeID ?? l.node_name ?? l.NodeName ?? 'unknown';
    const nname = l.node_name ?? l.NodeName ?? nid;
    let entry = byNode.get(nid);
    if (!entry) {
      entry = { nodeName: nname, logs: [] };
      byNode.set(nid, entry);
    }
    entry.logs.push(l);
  });

  let allTotalDowntime = 0;
  let allMaxDowntime = 0;
  let allIncidents = 0;
  let allOngoing = 0;
  const nodeStats: NodeDowntimeStat[] = [];

  byNode.forEach((entry, nid) => {
    let downStart = 0;
    let isDown = false;
    let currentLevel = 'normal';
    let nodeDowntimeSec = 0;
    let nodeMaxDowntimeSec = 0;
    let nodeIncidents = 0;
    let ongoing = false;

    entry.logs.forEach((l) => {
      const lvl = (l.level ?? l.Level ?? '').toLowerCase();
      const t = getMs(l.time ?? l.Time ?? 0);
      currentLevel = lvl;

      if (isFailure(lvl)) {
        if (!isDown) {
          isDown = true;
          downStart = t;
          nodeIncidents++;
          allIncidents++;
        }
      } else if (isRepair(lvl)) {
        if (isDown) {
          isDown = false;
          const duration = Math.max(0, Math.floor((t - downStart) / 1000));
          nodeDowntimeSec += duration;
          if (duration > nodeMaxDowntimeSec) nodeMaxDowntimeSec = duration;
        }
      }
    });

    if (isDown) {
      ongoing = true;
      allOngoing++;
      const duration = Math.max(0, Math.floor((maxTime - downStart) / 1000));
      nodeDowntimeSec += duration;
      if (duration > nodeMaxDowntimeSec) nodeMaxDowntimeSec = duration;
    }

    allTotalDowntime += nodeDowntimeSec;
    if (nodeMaxDowntimeSec > allMaxDowntime) allMaxDowntime = nodeMaxDowntimeSec;

    const sla = Math.max(0, Math.min(100, 100 - (nodeDowntimeSec * 100) / totalSpanSec));
    nodeStats.push({
      nodeID: nid,
      nodeName: entry.nodeName,
      count: nodeIncidents,
      totalDowntimeSec: nodeDowntimeSec,
      maxDowntimeSec: nodeMaxDowntimeSec,
      sla,
      ongoing,
      currentLevel,
    });
  });

  nodeStats.sort((a, b) => b.totalDowntimeSec - a.totalDowntimeSec);

  const nodeCount = Math.max(1, nodeStats.length);
  const overallSLA = Math.max(0, Math.min(100, 100 - (allTotalDowntime * 100) / (totalSpanSec * nodeCount)));
  const mttrSec = allIncidents > 0 ? Math.floor(allTotalDowntime / allIncidents) : 0;

  return {
    totalIncidents: allIncidents,
    ongoingIncidents: allOngoing,
    totalDowntimeSec: allTotalDowntime,
    maxDowntimeSec: allMaxDowntime,
    mttrSec,
    overallSLA,
    nodeStats,
  };
};

export const showEventLogDowntimeChart = (dom: HTMLElement | string, stats: NodeDowntimeStat[]): echarts.ECharts | undefined => {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return undefined;

  const chart = echarts.init(el, 'dark');
  const top15 = [...stats].slice(0, 15).reverse();
  const yData = top15.map((s) => s.nodeName);
  const seriesData = top15.map((s) => Math.round(s.totalDowntimeSec / 60)); // minutes

  chart.setOption({
    backgroundColor: 'transparent',
    grid: { left: 140, right: 30, top: 20, bottom: 25 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc' },
      formatter: (params: any) => {
        const item = params[0];
        return `${item.name}<br/>総停止時間: ${item.value} 分`;
      },
    },
    xAxis: {
      type: 'value',
      name: '停止時間 (分)',
      nameTextStyle: { color: '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      splitLine: { lineStyle: { color: '#1e293b' } },
      axisLabel: { color: '#94a3b8' },
    },
    yAxis: {
      type: 'category',
      data: yData,
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#cbd5e1', fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: seriesData,
        color: '#f87171',
        itemStyle: { borderRadius: [0, 4, 4, 0] },
      },
    ],
  });
  chart.resize();
  return chart;
};

export const showLogHeatmap = (dom: HTMLElement | string, logs: any[]): echarts.ECharts | undefined => {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return undefined;

  const chart = echarts.init(el, 'dark');
  const days = ['日', '月', '火', '水', '木', '金', '土'];
  const hours = Array.from({ length: 24 }, (_, i) => `${i}時`);

  const matrix: number[][] = [];
  for (let d = 0; d < 7; d++) {
    for (let h = 0; h < 24; h++) {
      matrix.push([h, d, 0]);
    }
  }

  let maxVal = 1;
  logs.forEach((l) => {
    const rawTime = l.time ?? l.Time ?? 0;
    if (!rawTime) return;
    const tMs = rawTime > 1e16 ? rawTime / 1e6 : (rawTime > 1e13 ? rawTime / 1e3 : (rawTime > 1e10 ? rawTime : rawTime * 1000));
    const dt = new Date(tMs);
    const day = dt.getDay();
    const hour = dt.getHours();
    const idx = day * 24 + hour;
    if (matrix[idx]) {
      matrix[idx][2]++;
      if (matrix[idx][2] > maxVal) maxVal = matrix[idx][2];
    }
  });

  chart.setOption({
    backgroundColor: 'transparent',
    grid: { left: 45, right: 30, top: 20, bottom: 40 },
    tooltip: {
      position: 'top',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc' },
      formatter: (p: any) => `${days[p.data[1]]}曜日 ${p.data[0]}時: ${p.data[2]} 件`,
    },
    xAxis: {
      type: 'category',
      data: hours,
      splitArea: { show: true },
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#94a3b8', fontSize: 10 },
    },
    yAxis: {
      type: 'category',
      data: days,
      splitArea: { show: true },
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#cbd5e1' },
    },
    visualMap: {
      min: 0,
      max: maxVal,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 0,
      inRange: {
        color: ['#0f172a', '#0369a1', '#06b6d4', '#eab308', '#ef4444'],
      },
      textStyle: { color: '#94a3b8', fontSize: 10 },
    },
    series: [
      {
        type: 'heatmap',
        data: matrix,
        label: { show: false },
        emphasis: {
          itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0, 0, 0, 0.5)' },
        },
      },
    ],
  });
  chart.resize();
  return chart;
};

export const showEventLogStateChart = (dom: HTMLElement | string, logs: any[]): echarts.ECharts | undefined => {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return undefined;

  const chart = echarts.init(el, 'dark');
  const counts: Record<string, number> = {
    high: 0,
    low: 0,
    warn: 0,
    normal: 0,
    info: 0,
    other: 0,
  };

  logs.forEach((l) => {
    const lvl = (l.level ?? l.Level ?? '').toLowerCase();
    if (counts[lvl] !== undefined) {
      counts[lvl]++;
    } else {
      counts.other++;
    }
  });

  const pieData = [
    { name: '重度 (High)', value: counts.high, itemStyle: { color: '#ef4444' } },
    { name: '軽度 (Low)', value: counts.low, itemStyle: { color: '#f87171' } },
    { name: '注意 (Warn)', value: counts.warn, itemStyle: { color: '#eab308' } },
    { name: '正常 (Normal)', value: counts.normal, itemStyle: { color: '#10b981' } },
    { name: '情報 (Info)', value: counts.info, itemStyle: { color: '#06b6d4' } },
  ].filter((p) => p.value > 0);

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc' },
      formatter: '{b}: {c} 件 ({d}%)',
    },
    legend: {
      orient: 'vertical',
      right: 20,
      top: 'center',
      textStyle: { color: '#cbd5e1', fontSize: 11 },
    },
    series: [
      {
        type: 'pie',
        radius: ['45%', '70%'],
        avoidLabelOverlap: false,
        itemStyle: { borderRadius: 6, borderColor: '#0b1329', borderWidth: 2 },
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 13, fontWeight: 'bold', color: '#f8fafc' },
        },
        data: pieData,
      },
    ],
  });
  chart.resize();
  return chart;
};

export const showEventLogNodeChart = (dom: HTMLElement | string, logs: any[]): echarts.ECharts | undefined => {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return undefined;

  const chart = echarts.init(el, 'dark');
  const nodeCountMap = new Map<string, number>();

  logs.forEach((l) => {
    const node = l.node_name ?? l.NodeName ?? l.node_id ?? l.NodeID ?? 'unknown';
    nodeCountMap.set(node, (nodeCountMap.get(node) ?? 0) + 1);
  });

  const sorted = Array.from(nodeCountMap.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 15)
    .reverse();

  chart.setOption({
    backgroundColor: 'transparent',
    grid: { left: 140, right: 30, top: 20, bottom: 25 },
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc' },
    },
    xAxis: {
      type: 'value',
      name: 'イベント数',
      nameTextStyle: { color: '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      splitLine: { lineStyle: { color: '#1e293b' } },
      axisLabel: { color: '#94a3b8' },
    },
    yAxis: {
      type: 'category',
      data: sorted.map((s) => s[0]),
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#cbd5e1', fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: sorted.map((s) => s[1]),
        color: '#06b6d4',
        itemStyle: { borderRadius: [0, 4, 4, 0] },
      },
    ],
  });
  chart.resize();
  return chart;
};
