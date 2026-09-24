import * as echarts from 'echarts';
import { setZoomCallback, isDarkMode } from './utils';

let chartInstance: echarts.ECharts | undefined;

export const showLogCountChart = (
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

  const minuteMap = new Map<number, number>();
  let st = Infinity;
  let lt = 0;

  const sortedLogs = [...logs].sort((a, b) => {
    const ta = a.time ?? a.Time ?? 0;
    const tb = b.time ?? b.Time ?? 0;
    return ta - tb;
  });

  sortedLogs.forEach((e) => {
    const rawTime = e.time ?? e.Time ?? 0;
    if (!rawTime) return;
    const tMs = rawTime > 1e16 ? rawTime / 1e6 : (rawTime > 1e13 ? rawTime / 1e3 : (rawTime > 1e10 ? rawTime : rawTime * 1000));
    const minute = Math.floor(tMs / (60 * 1000));
    minuteMap.set(minute, (minuteMap.get(minute) || 0) + 1);
    if (st > rawTime) st = rawTime;
    if (lt < rawTime) lt = rawTime;
  });

  const data: [Date, number][] = [];
  if (minuteMap.size > 0) {
    const minutes = Array.from(minuteMap.keys()).sort((a, b) => a - b);
    const minM = minutes[0];
    const maxM = minutes[minutes.length - 1];

    for (let m = minM; m <= maxM; m++) {
      const t = new Date(m * 60 * 1000);
      data.push([t, minuteMap.get(m) || 0]);
    }
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    title: {
      show: false,
    },
    legend: {
      show: false,
    },
    grid: {
      left: 55,
      right: 25,
      top: 35,
      bottom: 45,
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
        fillerColor: dark ? 'rgba(31, 120, 180, 0.3)' : 'rgba(31, 120, 180, 0.2)',
        handleStyle: { color: '#1f78b4' },
        textStyle: { color: dark ? '#64748b' : '#94a3b8', fontSize: 9 },
      },
      {
        type: 'inside',
      },
    ],
    xAxis: {
      type: 'time',
      name: 'Time',
      nameTextStyle: { color: dark ? '#64748b' : '#94a3b8', fontSize: 10 },
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
      nameTextStyle: { color: dark ? '#64748b' : '#94a3b8', fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#f1f5f9', type: 'dashed' } },
    },
    series: [
      {
        name: 'Log count',
        type: 'bar',
        color: '#1f78b4',
        data,
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

export const resizeLogCountChart = () => {
  chartInstance?.resize();
};

export const disposeLogCountChart = () => {
  chartInstance?.dispose();
  chartInstance = undefined;
};
