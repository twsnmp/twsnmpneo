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

  if (chartInstance) {
    chartInstance.dispose();
  }
  chartInstance = echarts.init(el, 'dark');

  const data: [Date, number][] = [];
  let count = 0;
  let ctm: number | undefined;
  let st = Infinity;
  let lt = 0;

  const sortedLogs = [...logs].sort((a, b) => {
    const ta = a.time ?? a.Time ?? 0;
    const tb = b.time ?? b.Time ?? 0;
    return ta - tb;
  });

  const addChartData = (currentMinute: number, nextMinute: number) => {
    let t = new Date(currentMinute * 60 * 1000);
    data.push([t, count]);
    currentMinute++;
    for (; currentMinute < nextMinute; currentMinute++) {
      t = new Date(currentMinute * 60 * 1000);
      data.push([t, 0]);
    }
    return currentMinute;
  };

  sortedLogs.forEach((e) => {
    const rawTime = e.time ?? e.Time ?? 0;
    if (!rawTime) return;
    const tMs = rawTime > 1e16 ? rawTime / 1e6 : (rawTime > 1e13 ? rawTime / 1e3 : (rawTime > 1e10 ? rawTime : rawTime * 1000));
    const newCtm = Math.floor(tMs / (60 * 1000));

    if (ctm === undefined) {
      ctm = newCtm;
    }
    if (ctm !== newCtm) {
      ctm = addChartData(ctm, newCtm);
      count = 0;
    }
    count++;
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
        fillerColor: 'rgba(6, 182, 212, 0.2)',
        handleStyle: { color: '#06b6d4' },
        textStyle: { color: '#64748b', fontSize: 9 },
      },
      {
        type: 'inside',
      },
    ],
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: {
        color: '#94a3b8',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(new Date(val), '{MM}/{dd} {HH}:{mm}', false),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      name: '件数',
      nameTextStyle: { color: '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } },
    },
    series: [
      {
        name: '受信件数',
        type: 'bar',
        color: '#06b6d4',
        data,
      },
    ],
  };

  chartInstance.setOption(option);
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
