import * as echarts from 'echarts';
import type { MonitorDataEnt } from '../api';
import { renderBytes, renderSpeed } from '../common';
import { isDarkMode } from './utils';
import { locale } from 'svelte-i18n';
import { get } from 'svelte/store';

let resChartInstance: echarts.ECharts | null = null;
let netChartInstance: echarts.ECharts | null = null;
let forecastChartInstance: echarts.ECharts | null = null;

export const showMonitorResChart = (el: HTMLElement, monitorData: MonitorDataEnt[]): echarts.ECharts => {
  if (resChartInstance) {
    resChartInstance.dispose();
  }
  const isDark = isDarkMode();
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const memLegend = isJa ? 'Mem (ホストメモリ)' : 'Mem (Host Memory)';
  const loadLegend = isJa ? 'Load (平均負荷)' : 'Load (Avg Load)';
  const rateAxis = isJa ? '使用率 (%)' : 'Usage (%)';
  const loadAxis = isJa ? 'Load (負荷)' : 'Load';
  resChartInstance = echarts.init(el, isDark ? 'dark' : undefined);

  const cpuData: [Date, number][] = [];
  const memData: [Date, number][] = [];
  const myCpuData: [Date, number][] = [];
  const myMemData: [Date, number][] = [];
  const swapData: [Date, number][] = [];
  const diskData: [Date, number][] = [];
  const loadData: [Date, number][] = [];

  monitorData.forEach((m) => {
    const t = new Date(Math.floor(m.Time / 1e6));
    cpuData.push([t, m.CPU || 0]);
    memData.push([t, m.Mem || 0]);
    myCpuData.push([t, m.MyCPU || 0]);
    myMemData.push([t, m.MyMem || 0]);
    swapData.push([t, m.Swap || 0]);
    diskData.push([t, m.Disk || 0]);
    loadData.push([t, m.Load || 0]);
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc', fontSize: 11 },
      axisPointer: {
        type: 'cross',
        lineStyle: { color: '#38bdf8', width: 1, type: 'dashed' },
      },
      formatter: (params: any) => {
        if (!params || !params.length) return '';
        const dateStr = echarts.time.format(params[0].value[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false);
        let html = `<div class="font-bold text-slate-200 mb-1 font-mono">${dateStr}</div>`;
        for (const p of params) {
          const isLoad = p.seriesName === loadLegend;
          const valStr = isLoad ? p.value[1].toFixed(2) : p.value[1].toFixed(2) + '%';
          html += `<div class="flex items-center justify-between gap-4 text-xs">
            <span style="color:${p.color}">${p.marker} ${p.seriesName}</span>
            <span class="font-mono font-bold text-slate-100">${valStr}</span>
          </div>`;
        }
        return html;
      },
    },
    legend: {
      top: 8,
      textStyle: { color: '#94a3b8', fontSize: 11 },
      data: ['CPU', memLegend, 'My CPU (TWSNMP)', 'My Mem (TWSNMP)', 'Swap', 'Disk', loadLegend],
    },
    grid: {
      left: '4%',
      right: '4%',
      top: 48,
      bottom: 50,
      containLabel: true,
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 8,
        height: 18,
        borderColor: isDark ? '#1e293b' : '#cbd5e1',
        backgroundColor: isDark ? '#090d16' : '#f8fafc',
        fillerColor: isDark ? 'rgba(56, 189, 248, 0.15)' : 'rgba(56, 189, 248, 0.2)',
        textStyle: { color: isDark ? '#64748b' : '#64748b', fontSize: 9 },
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
        formatter: (v: any) => echarts.time.format(v, '{MM}/{dd} {HH}:{mm}', false),
      },
      splitLine: { show: false },
    },
    yAxis: [
      {
        type: 'value',
        name: rateAxis,
        min: 0,
        max: 100,
        nameTextStyle: { color: '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: { color: '#94a3b8', fontSize: 10, formatter: '{value}%' },
        splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } },
      },
      {
        type: 'value',
        name: loadAxis,
        min: 0,
        nameTextStyle: { color: '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: { color: '#94a3b8', fontSize: 10 },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: 'CPU',
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#38bdf8' }, // sky-400
        lineStyle: { width: 2 },
        data: cpuData,
      },
      {
        name: memLegend,
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#a855f7' }, // purple-500
        lineStyle: { width: 2 },
        data: memData,
      },
      {
        name: 'My CPU (TWSNMP)',
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#06b6d4' }, // cyan-500
        lineStyle: { width: 1.5, type: 'dashed' },
        data: myCpuData,
      },
      {
        name: 'My Mem (TWSNMP)',
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#ec4899' }, // pink-500
        lineStyle: { width: 1.5, type: 'dashed' },
        data: myMemData,
      },
      {
        name: 'Swap',
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#eab308' }, // yellow-500
        lineStyle: { width: 1.5 },
        data: swapData,
      },
      {
        name: 'Disk',
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#f97316' }, // orange-500
        lineStyle: { width: 2 },
        data: diskData,
      },
      {
        name: loadLegend,
        type: 'bar',
        yAxisIndex: 1,
        itemStyle: { color: 'rgba(52, 211, 153, 0.4)' }, // emerald-400 translucent
        data: loadData,
      },
    ],
  };

  resChartInstance.setOption(option);
  resChartInstance.resize();
  return resChartInstance;
};

export const showMonitorNetChart = (el: HTMLElement, monitorData: MonitorDataEnt[]): echarts.ECharts => {
  if (netChartInstance) {
    netChartInstance.dispose();
  }
  const isDark = isDarkMode();
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const speedLegend = isJa ? 'トラフィック速度 (Speed)' : 'Traffic Speed';
  const connLegend = isJa ? 'TCP 接続数' : 'TCP Connections';
  const speedAxis = isJa ? '速度 (bps)' : 'Speed (bps)';
  const connAxis = isJa ? '接続数 (Conn)' : 'Connections';

  netChartInstance = echarts.init(el, isDark ? 'dark' : undefined);

  const speedData: [Date, number][] = [];
  const connData: [Date, number][] = [];

  monitorData.forEach((m) => {
    const t = new Date(Math.floor(m.Time / 1e6));
    speedData.push([t, m.Net || 0]);
    connData.push([t, m.Conn || 0]);
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: '#0f172a',
      borderColor: '#334155',
      textStyle: { color: '#f8fafc', fontSize: 11 },
      axisPointer: {
        type: 'cross',
        lineStyle: { color: '#10b981', width: 1, type: 'dashed' },
      },
      formatter: (params: any) => {
        if (!params || !params.length) return '';
        const dateStr = echarts.time.format(params[0].value[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false);
        let html = `<div class="font-bold text-slate-200 mb-1 font-mono">${dateStr}</div>`;
        for (const p of params) {
          const isConn = p.seriesName === connLegend;
          const valStr = isConn ? `${p.value[1]} conn` : renderSpeed(p.value[1]);
          html += `<div class="flex items-center justify-between gap-4 text-xs">
            <span style="color:${p.color}">${p.marker} ${p.seriesName}</span>
            <span class="font-mono font-bold text-slate-100">${valStr}</span>
          </div>`;
        }
        return html;
      },
    },
    legend: {
      top: 8,
      textStyle: { color: '#94a3b8', fontSize: 11 },
      data: [speedLegend, connLegend],
    },
    grid: {
      left: '4%',
      right: '4%',
      top: 48,
      bottom: 50,
      containLabel: true,
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 8,
        height: 18,
        borderColor: isDark ? '#1e293b' : '#cbd5e1',
        backgroundColor: isDark ? '#090d16' : '#f8fafc',
        fillerColor: isDark ? 'rgba(16, 185, 129, 0.15)' : 'rgba(16, 185, 129, 0.2)',
        textStyle: { color: isDark ? '#64748b' : '#64748b', fontSize: 9 },
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
        formatter: (v: any) => echarts.time.format(v, '{MM}/{dd} {HH}:{mm}', false),
      },
      splitLine: { show: false },
    },
    yAxis: [
      {
        type: 'value',
        name: speedAxis,
        min: 0,
        nameTextStyle: { color: '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: {
          color: '#94a3b8',
          fontSize: 10,
          formatter: (v: any) => renderSpeed(v),
        },
        splitLine: { lineStyle: { color: '#1e293b', type: 'dashed' } },
      },
      {
        type: 'value',
        name: connAxis,
        min: 0,
        nameTextStyle: { color: '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: { color: '#94a3b8', fontSize: 10 },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: speedLegend,
        type: 'line',
        showSymbol: false,
        smooth: true,
        itemStyle: { color: '#10b981' }, // emerald-500
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(16, 185, 129, 0.35)' },
            { offset: 1, color: 'rgba(16, 185, 129, 0.0)' },
          ]),
        },
        lineStyle: { width: 2 },
        data: speedData,
      },
      {
        name: connLegend,
        type: 'bar',
        yAxisIndex: 1,
        itemStyle: { color: 'rgba(56, 189, 248, 0.45)' }, // sky-400
        data: connData,
      },
    ],
  };

  netChartInstance.setOption(option);
  netChartInstance.resize();
  return netChartInstance;
};

// Linear regression calculator
function linearRegression(points: [number, number][]): { slope: number; intercept: number } {
  const n = points.length;
  if (n < 2) return { slope: 0, intercept: points[0]?.[1] ?? 0 };
  let sumX = 0, sumY = 0, sumXY = 0, sumXX = 0;
  for (const [x, y] of points) {
    sumX += x;
    sumY += y;
    sumXY += x * y;
    sumXX += x * x;
  }
  const denom = n * sumXX - sumX * sumX;
  if (denom === 0) return { slope: 0, intercept: sumY / n };
  const slope = (n * sumXY - sumX * sumY) / denom;
  const intercept = (sumY - slope * sumX) / n;
  return { slope, intercept };
}

export const showMonitorForecastChart = (el: HTMLElement, monitorData: MonitorDataEnt[]): echarts.ECharts => {
  if (forecastChartInstance) {
    forecastChartInstance.dispose();
  }
  const isDark = isDarkMode();
  const isJa = (get(locale) || 'ja').startsWith('ja');
  const titleText = isJa ? 'ディスク容量 & DBサイズ将来予測 (1年推移シミュレーション)' : 'Disk & DB Size Future Capacity Projection (1-Year Simulation)';
  const predSuffix = isJa ? '(予測)' : '(Projected)';
  const diskPredLegend = isJa ? 'ディスク使用率予測 (%)' : 'Projected Disk Usage (%)';
  const dbPredLegend = isJa ? 'DBサイズ予測 (Bytes)' : 'Projected DB Size (Bytes)';
  const diskAxis = isJa ? 'ディスク使用率 (%)' : 'Disk Usage (%)';
  const dbAxis = isJa ? 'DBサイズ (Bytes)' : 'DB Size (Bytes)';
  const warnThresh = isJa ? '警告閾値 90%' : 'Warning 90%';
  const dangerThresh = isJa ? '危険閾値 95%' : 'Critical 95%';

  forecastChartInstance = echarts.init(el, isDark ? 'dark' : undefined);

  const diskHistory: [number, number][] = [];
  const dbHistory: [number, number][] = [];

  monitorData.forEach((m) => {
    const ms = Math.floor(m.Time / 1e6);
    diskHistory.push([ms, m.Disk || 0]);
    dbHistory.push([ms, m.DBSize || 0]);
  });

  const regDisk = linearRegression(diskHistory);
  const regDB = linearRegression(dbHistory);

  const forecastDisk: [Date, number][] = [];
  const forecastDB: [Date, number][] = [];

  const nowMs = Date.now();
  const dayMs = 24 * 3600 * 1000;

  // Forecast for next 365 days (every 7 days)
  for (let d = 0; d <= 365; d += 7) {
    const tMs = nowMs + d * dayMs;
    const t = new Date(tMs);
    const yDisk = Math.max(0, Math.min(100, regDisk.intercept + regDisk.slope * tMs));
    const yDB = Math.max(0, regDB.intercept + regDB.slope * tMs);
    forecastDisk.push([t, yDisk]);
    forecastDB.push([t, yDB]);
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    title: {
      text: titleText,
      left: 'center',
      top: 10,
      textStyle: { color: isDark ? '#f8fafc' : '#0f172a', fontSize: 13, fontWeight: 'bold' },
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#0f172a' : '#ffffff',
      borderColor: isDark ? '#334155' : '#cbd5e1',
      textStyle: { color: isDark ? '#f8fafc' : '#0f172a', fontSize: 11 },
      axisPointer: { type: 'cross' },
      formatter: (params: any) => {
        if (!params || !params.length) return '';
        const dateStr = echarts.time.format(params[0].value[0], '{yyyy}/{MM}/{dd}', false);
        let html = `<div class="font-bold ${isDark ? 'text-slate-200' : 'text-slate-800'} mb-1 font-mono">${dateStr} ${predSuffix}</div>`;
        for (const p of params) {
          const isDisk = p.seriesName === diskPredLegend;
          const valStr = isDisk ? p.value[1].toFixed(2) + '%' : renderBytes(p.value[1]);
          html += `<div class="flex items-center justify-between gap-4 text-xs">
            <span style="color:${p.color}">${p.marker} ${p.seriesName}</span>
            <span class="font-mono font-bold ${isDark ? 'text-slate-100' : 'text-slate-900'}">${valStr}</span>
          </div>`;
        }
        return html;
      },
    },
    legend: {
      top: 38,
      textStyle: { color: isDark ? '#94a3b8' : '#475569', fontSize: 11 },
      data: [diskPredLegend, dbPredLegend],
    },
    grid: {
      left: '5%',
      right: '6%',
      top: 80,
      bottom: 60,
      containLabel: true,
    },
    xAxis: {
      type: 'time',
      axisLine: { lineStyle: { color: isDark ? '#334155' : '#cbd5e1' } },
      axisLabel: {
        color: isDark ? '#94a3b8' : '#64748b',
        fontSize: 10,
        formatter: (v: any) => echarts.time.format(v, '{yyyy}/{MM}/{dd}', false),
      },
      splitLine: { show: false },
    },
    yAxis: [
      {
        type: 'value',
        name: diskAxis,
        min: 0,
        max: 100,
        nameTextStyle: { color: isDark ? '#94a3b8' : '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: isDark ? '#334155' : '#cbd5e1' } },
        axisLabel: { color: isDark ? '#94a3b8' : '#64748b', fontSize: 10, formatter: '{value}%' },
        splitLine: { lineStyle: { color: isDark ? '#1e293b' : '#e2e8f0', type: 'dashed' } },
      },
      {
        type: 'value',
        name: dbAxis,
        min: 0,
        nameTextStyle: { color: '#64748b', fontSize: 10 },
        axisLine: { lineStyle: { color: '#334155' } },
        axisLabel: {
          color: '#94a3b8',
          fontSize: 10,
          formatter: (v: any) => renderBytes(v),
        },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: diskPredLegend,
        type: 'line',
        itemStyle: { color: '#f59e0b' }, // amber-500
        lineStyle: { width: 2.5 },
        markLine: {
          silent: true,
          data: [
            {
              yAxis: 90,
              lineStyle: { color: '#f59e0b', type: 'dashed' },
              label: { formatter: warnThresh, color: '#f59e0b', position: 'insideEndTop' },
            },
            {
              yAxis: 95,
              lineStyle: { color: '#ef4444', type: 'dashed' },
              label: { formatter: dangerThresh, color: '#ef4444', position: 'insideEndTop' },
            },
          ],
        },
        data: forecastDisk,
      },
      {
        name: dbPredLegend,
        type: 'line',
        yAxisIndex: 1,
        itemStyle: { color: '#38bdf8' }, // sky-400
        lineStyle: { width: 2.5 },
        data: forecastDB,
      },
    ],
  };

  forecastChartInstance.setOption(option);
  forecastChartInstance.resize();
  return forecastChartInstance;
};

export const resizeMonitorChart = (showForecast = false) => {
  if (showForecast && forecastChartInstance) {
    forecastChartInstance.resize();
  }
  if (resChartInstance) {
    resChartInstance.resize();
  }
  if (netChartInstance) {
    netChartInstance.resize();
  }
};

export const disposeMonitorCharts = () => {
  if (resChartInstance) {
    resChartInstance.dispose();
    resChartInstance = null;
  }
  if (netChartInstance) {
    netChartInstance.dispose();
    netChartInstance = null;
  }
  if (forecastChartInstance) {
    forecastChartInstance.dispose();
    forecastChartInstance = null;
  }
};
