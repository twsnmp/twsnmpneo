import * as echarts from 'echarts';
import { setZoomCallback } from './utils';

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
  chartInstance = echarts.init(el, 'dark');

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
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc', fontSize: 11 },
    },
    toolbox: {
      iconStyle: { borderColor: '#94a3b8' },
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
        borderColor: '#334155',
        backgroundColor: '#020617',
        fillerColor: 'rgba(31, 120, 180, 0.3)',
        handleStyle: { color: '#1f78b4' },
        textStyle: { color: '#64748b', fontSize: 9 },
      },
      {
        type: 'inside',
      },
    ],
    xAxis: {
      type: 'time',
      name: 'Time',
      nameTextStyle: { color: '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: {
        color: '#94a3b8',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(new Date(val), '{yyyy}/{MM}/{dd} {HH}:{mm}', false),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      name: 'Log count',
      nameTextStyle: { color: '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } },
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
