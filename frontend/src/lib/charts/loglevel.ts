import * as echarts from 'echarts';
import { setZoomCallback, isDarkMode } from './utils';

let chartInstance: echarts.ECharts | undefined;

export const showLogLevelChart = (
  dom: HTMLElement | string,
  logs: any[],
  zoomCallback?: (st: number, et: number) => void
): echarts.ECharts | undefined => {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return undefined;

  const existing = echarts.getInstanceByDom(el);
  if (existing) {
    existing.dispose();
  }
  const dark = isDarkMode();
  chartInstance = echarts.init(el, dark ? 'dark' : undefined);

  const data: Record<string, [Date, number][]> = {
    high: [],
    low: [],
    warn: [],
    other: [],
  };

  const count: Record<string, number> = {
    high: 0,
    low: 0,
    warn: 0,
    other: 0,
  };

  const addChartData = (ctm: number, newCtm: number) => {
    let t = new Date(ctm * 60 * 1000);
    for (const k of ['high', 'low', 'warn', 'other']) {
      data[k].push([t, count[k]]);
    }
    ctm++;
    for (; ctm < newCtm; ctm++) {
      t = new Date(ctm * 60 * 1000);
      for (const k of ['high', 'low', 'warn', 'other']) {
        data[k].push([t, 0]);
      }
    }
    return ctm;
  };

  const sortedLogs = [...logs].sort((a, b) => {
    const ta = a.time ?? a.Time ?? 0;
    const tb = b.time ?? b.Time ?? 0;
    return ta - tb;
  });

  let ctm: number | undefined;
  let st = Infinity;
  let lt = 0;

  sortedLogs.forEach((e) => {
    const rawTime = e.time ?? e.Time ?? 0;
    if (!rawTime) return;
    const tMs = rawTime > 1e16 ? rawTime / 1e6 : (rawTime > 1e13 ? rawTime / 1e3 : (rawTime > 1e10 ? rawTime : rawTime * 1000));
    const lvlKey = (e.level ?? e.Level ?? '').toLowerCase();
    const lvl = (lvlKey === 'high' || lvlKey === 'low' || lvlKey === 'warn') ? lvlKey : 'other';
    const newCtm = Math.floor(tMs / (60 * 1000));

    if (ctm === undefined) {
      ctm = newCtm;
    }
    if (ctm !== newCtm) {
      ctm = addChartData(ctm, newCtm);
      for (const k in count) count[k] = 0;
    }
    count[lvl]++;
    if (st > rawTime) st = rawTime;
    if (lt < rawTime) lt = rawTime;
  });

  if (ctm !== undefined) {
    addChartData(ctm, ctm + 1);
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    grid: {
      left: 55,
      right: 35,
      top: 40,
      bottom: 50,
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#0f172a', fontSize: 11 },
    },
    toolbox: {
      iconStyle: { borderColor: dark ? '#94a3b8' : '#64748b' },
      feature: {
        dataZoom: { yAxisIndex: 'none' },
        restore: {},
      },
      right: 20,
      top: 5,
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 5,
        height: 16,
        borderColor: dark ? '#334155' : '#e2e8f0',
        backgroundColor: dark ? '#020617' : '#f8fafc',
        fillerColor: dark ? 'rgba(6, 182, 212, 0.2)' : 'rgba(6, 182, 212, 0.15)',
        handleStyle: { color: '#06b6d4' },
        textStyle: { color: dark ? '#64748b' : '#94a3b8', fontSize: 9 },
      },
      {
        type: 'inside',
      },
    ],
    legend: {
      top: 10,
      textStyle: { color: dark ? '#cbd5e1' : '#475569', fontSize: 11 },
      data: ['High', 'Low', 'Warn', 'Other'],
    },
    xAxis: {
      type: 'time',
      name: 'Time',
      nameTextStyle: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(new Date(val), '{yyyy}/{MM}/{dd} {HH}:{mm}', false),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      name: 'Log count',
      nameTextStyle: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#f1f5f9', type: 'dashed' } },
    },
    series: [
      {
        name: 'High',
        type: 'bar',
        stack: 'count',
        color: '#e31a1c',
        data: data.high,
      },
      {
        name: 'Low',
        type: 'bar',
        stack: 'count',
        color: '#fb9a99',
        data: data.low,
      },
      {
        name: 'Warn',
        type: 'bar',
        stack: 'count',
        color: '#dfdf22',
        data: data.warn,
      },
      {
        name: 'Other',
        type: 'bar',
        stack: 'count',
        color: '#1f78b4',
        data: data.other,
      },
    ],
  };

  chartInstance.setOption(option, true);
  chartInstance.resize();

  if (zoomCallback) {
    setZoomCallback(chartInstance, zoomCallback, st, lt);
  }

  return chartInstance;
};

export const resizeLogLevelChart = () => {
  chartInstance?.resize();
};

export const disposeLogLevelChart = () => {
  chartInstance?.dispose();
  chartInstance = undefined;
};
