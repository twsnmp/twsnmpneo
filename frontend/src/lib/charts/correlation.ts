import * as echarts from 'echarts';
import { isDarkMode } from './utils';
import type { CorrelationDiscoverItem, CorrelationPairRes } from '../api';

export interface CorrelationChartLabels {
  normAxisLabel?: string;
  regressionLine?: string;
  correlation?: string;
  lagSecondsAxis?: string;
  lagScoreAxis?: string;
  positiveEdge?: string;
  negativeEdge?: string;
  samples?: string;
  lagLabel?: string;
}

const getThemeColors = () => {
  const dark = isDarkMode();
  return {
    dark,
    textColor: dark ? '#cbd5e1' : '#334155',
    subTextColor: dark ? '#94a3b8' : '#64748b',
    lineColor: dark ? '#334155' : '#e2e8f0',
    tooltipBg: dark ? 'rgba(15, 23, 42, 0.92)' : 'rgba(255, 255, 255, 0.96)',
    tooltipBorder: dark ? '#334155' : '#cbd5e1',
  };
};

const disposeChart = (dom: HTMLElement) => {
  const existing = echarts.getInstanceByDom(dom);
  if (existing) {
    existing.dispose();
  }
};

const createChart = (dom: HTMLElement): echarts.ECharts => {
  disposeChart(dom);
  const { dark } = getThemeColors();
  return echarts.init(dom, dark ? 'dark' : undefined);
};

export const showCorrelationOverlayChart = (
  dom: HTMLElement,
  res: CorrelationPairRes,
  mode: 'dual' | 'norm' = 'dual',
  labels: CorrelationChartLabels = {},
): echarts.ECharts => {
  const chart = createChart(dom);
  const { textColor, lineColor, tooltipBg, tooltipBorder } = getThemeColors();
  const points = mode === 'norm' ? res.NormSeries || [] : res.Series || [];

  const label1 = `${res.NodeName1 || ''} / ${res.PollingName1} (${res.BestVar1})`;
  const label2 = `${res.NodeName2 || ''} / ${res.PollingName2} (${res.BestVar2})`;

  const data1 = points.map((p) => [p.Time * 1000, p.Value1]);
  const data2 = points.map((p) => [p.Time * 1000, p.Value2]);

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: textColor, fontSize: 12 },
      axisPointer: { type: 'cross' },
    },
    legend: {
      data: [label1, label2],
      textStyle: { color: textColor, fontSize: 11 },
      top: 4,
    },
    grid: {
      left: '7%',
      right: mode === 'norm' ? '4%' : '7%',
      top: 44,
      bottom: 58,
      containLabel: true,
    },
    dataZoom: [
      { type: 'inside', xAxisIndex: 0 },
      { type: 'slider', xAxisIndex: 0, height: 18, bottom: 8 },
    ],
    xAxis: {
      type: 'time',
      axisLabel: { color: textColor, fontSize: 11 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis:
      mode === 'norm'
        ? [
            {
              type: 'value',
              name: labels.normAxisLabel || 'Normalized (0-1)',
              min: 0,
              max: 1,
              nameTextStyle: { color: textColor, fontSize: 11 },
              axisLabel: { color: textColor, fontSize: 11 },
              splitLine: { lineStyle: { color: lineColor, type: 'dashed' } },
            },
          ]
        : [
            {
              type: 'value',
              name: res.BestVar1,
              nameTextStyle: { color: '#3b82f6', fontSize: 11 },
              axisLabel: { color: '#3b82f6', fontSize: 11 },
              splitLine: { lineStyle: { color: lineColor, type: 'dashed' } },
            },
            {
              type: 'value',
              name: res.BestVar2,
              nameTextStyle: { color: '#f97316', fontSize: 11 },
              axisLabel: { color: '#f97316', fontSize: 11 },
              splitLine: { show: false },
            },
          ],
    series: [
      {
        name: label1,
        type: 'line',
        yAxisIndex: 0,
        showSymbol: points.length < 80,
        symbolSize: 4,
        smooth: 0.15,
        lineStyle: { width: 2, color: '#3b82f6' },
        itemStyle: { color: '#3b82f6' },
        data: data1,
      },
      {
        name: label2,
        type: 'line',
        yAxisIndex: mode === 'norm' ? 0 : 1,
        showSymbol: points.length < 80,
        symbolSize: 4,
        smooth: 0.15,
        lineStyle: { width: 2, color: '#f97316' },
        itemStyle: { color: '#f97316' },
        data: data2,
      },
    ],
  };

  chart.setOption(option);
  return chart;
};

export const showCorrelationScatterChart = (
  dom: HTMLElement,
  res: CorrelationPairRes,
  labels: CorrelationChartLabels = {},
): echarts.ECharts => {
  const chart = createChart(dom);
  const { textColor, lineColor, tooltipBg, tooltipBorder } = getThemeColors();
  const points = res.Series || [];

  const scatterData = points.map((p) => [p.Value1, p.Value2]);
  let minX = Infinity;
  let maxX = -Infinity;
  for (const p of points) {
    if (p.Value1 < minX) minX = p.Value1;
    if (p.Value1 > maxX) maxX = p.Value1;
  }

  const regData: number[][] = [];
  if (Number.isFinite(minX) && Number.isFinite(maxX) && maxX >= minX) {
    regData.push([minX, res.Slope * minX + res.Intercept]);
    regData.push([maxX, res.Slope * maxX + res.Intercept]);
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: textColor, fontSize: 12 },
      formatter: (params: any) => {
        if (params.seriesType === 'line') return '';
        const val = params.value as number[];
        return `${res.BestVar1}: <b>${val[0].toFixed(3)}</b><br/>${res.BestVar2}: <b>${val[1].toFixed(3)}</b>`;
      },
    },
    grid: {
      left: '8%',
      right: '6%',
      top: 32,
      bottom: 42,
      containLabel: true,
    },
    xAxis: {
      type: 'value',
      name: `${res.PollingName1} (${res.BestVar1})`,
      nameLocation: 'middle',
      nameGap: 26,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 11 },
      splitLine: { lineStyle: { color: lineColor, type: 'dashed' } },
    },
    yAxis: {
      type: 'value',
      name: `${res.PollingName2} (${res.BestVar2})`,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 11 },
      splitLine: { lineStyle: { color: lineColor, type: 'dashed' } },
    },
    series: [
      {
        type: 'scatter',
        symbolSize: 7,
        itemStyle: { color: '#3b82f6', opacity: 0.72 },
        data: scatterData,
      },
      {
        name: labels.regressionLine || 'Regression',
        type: 'line',
        showSymbol: false,
        lineStyle: { color: '#ef4444', width: 2, type: 'dashed' },
        data: regData,
      },
    ],
  };

  chart.setOption(option);
  return chart;
};

export const showCorrelationLagChart = (
  dom: HTMLElement,
  res: CorrelationPairRes,
  labels: CorrelationChartLabels = {},
): echarts.ECharts => {
  const chart = createChart(dom);
  const { textColor, lineColor, tooltipBg, tooltipBorder } = getThemeColors();
  const profile = res.LagProfile || [];

  const xLabels = profile.map((p) => `${p.LagSeconds >= 0 ? '+' : ''}${p.LagSeconds}s`);
  const barData = profile.map((p) => {
    const isBest = p.LagBuckets === res.BestLagBuckets;
    return {
      value: Number(p.Score.toFixed(4)),
      itemStyle: {
        color: isBest ? '#f59e0b' : p.Score >= 0 ? '#3b82f6' : '#ef4444',
        borderWidth: isBest ? 2 : 0,
        borderColor: isBest ? '#fff' : undefined,
      },
    };
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: textColor, fontSize: 12 },
      axisPointer: { type: 'shadow' },
    },
    grid: {
      left: '7%',
      right: '4%',
      top: 28,
      bottom: 42,
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      name: labels.lagSecondsAxis || 'Lag (s)',
      nameLocation: 'middle',
      nameGap: 24,
      nameTextStyle: { color: textColor, fontSize: 11 },
      data: xLabels,
      axisLabel: { color: textColor, fontSize: 11 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis: {
      type: 'value',
      name: labels.lagScoreAxis || 'r',
      min: -1,
      max: 1,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 11 },
      splitLine: { lineStyle: { color: lineColor, type: 'dashed' } },
    },
    series: [
      {
        name: labels.correlation || 'Correlation',
        type: 'bar',
        data: barData,
      },
    ],
  };

  chart.setOption(option);
  return chart;
};

export const showCorrelationMatrixChart = (
  dom: HTMLElement,
  res: CorrelationPairRes,
  onSelectVars?: (v1: string, v2: string) => void,
): echarts.ECharts => {
  const chart = createChart(dom);
  const { dark, textColor, tooltipBg, tooltipBorder } = getThemeColors();

  const vars1 = res.UsedVars1 || [];
  const vars2 = res.UsedVars2 || [];
  const matrix = res.VarMatrix || [];

  const data: [number, number, number][] = [];
  for (const cell of matrix) {
    const xIdx = vars1.indexOf(cell.Var1);
    const yIdx = vars2.indexOf(cell.Var2);
    if (xIdx >= 0 && yIdx >= 0) {
      data.push([xIdx, yIdx, Number(cell.Score.toFixed(3))]);
    }
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      position: 'top',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: textColor, fontSize: 12 },
      formatter: (params: any) => {
        const v = params.value as [number, number, number];
        return `${vars1[v[0]]} × ${vars2[v[1]]}<br/>Score: <b>${v[2]}</b>`;
      },
    },
    grid: {
      top: 16,
      bottom: 52,
      left: 16,
      right: 60,
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: vars1,
      axisLabel: { color: textColor, fontSize: 11, rotate: vars1.length > 5 ? 25 : 0 },
      splitArea: { show: true },
    },
    yAxis: {
      type: 'category',
      data: vars2,
      axisLabel: { color: textColor, fontSize: 11 },
      splitArea: { show: true },
    },
    visualMap: {
      min: -1,
      max: 1,
      calculable: true,
      orient: 'vertical',
      right: 4,
      top: 'center',
      textStyle: { color: textColor, fontSize: 10 },
      inRange: {
        color: ['#dc2626', '#f87171', dark ? '#1e293b' : '#f1f5f9', '#60a5fa', '#2563eb'],
      },
    },
    series: [
      {
        type: 'heatmap',
        data,
        label: {
          show: vars1.length * vars2.length <= 49,
          color: textColor,
          fontSize: 11,
        },
        emphasis: {
          itemStyle: {
            shadowBlur: 8,
            shadowColor: 'rgba(0, 0, 0, 0.4)',
          },
        },
      },
    ],
  };

  chart.setOption(option);
  if (onSelectVars) {
    chart.on('click', (params: any) => {
      const val = params.value as [number, number, number] | undefined;
      if (val && vars1[val[0]] && vars2[val[1]]) {
        onSelectVars(vars1[val[0]], vars2[val[1]]);
      }
    });
  }
  return chart;
};

export const showCorrelationNetworkChart = (
  dom: HTMLElement,
  items: CorrelationDiscoverItem[],
  onSelectPair?: (item: CorrelationDiscoverItem) => void,
  labels: CorrelationChartLabels = {},
): echarts.ECharts => {
  const chart = createChart(dom);
  const { dark, textColor, tooltipBg, tooltipBorder } = getThemeColors();

  const nodeColorPalette = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#8b5cf6',
    '#ec4899',
    '#06b6d4',
    '#f97316',
    '#84cc16',
  ];

  const categoryMap = new Map<string, number>();
  const categories: { name: string; itemStyle: { color: string } }[] = [];
  const nodeDegree = new Map<string, number>();
  const nodeMeta = new Map<string, { id: string; name: string; nodeName: string; catIdx: number }>();

  const ensureNode = (pid: string, pname: string, nname: string) => {
    nodeDegree.set(pid, (nodeDegree.get(pid) || 0) + 1);
    if (!nodeMeta.has(pid)) {
      const catKey = nname || 'Unknown';
      let catIdx = categoryMap.get(catKey);
      if (catIdx === undefined) {
        catIdx = categories.length;
        categoryMap.set(catKey, catIdx);
        categories.push({
          name: catKey,
          itemStyle: { color: nodeColorPalette[catIdx % nodeColorPalette.length] },
        });
      }
      nodeMeta.set(pid, { id: pid, name: pname, nodeName: catKey, catIdx });
    }
  };

  const links = items.map((item, idx) => {
    ensureNode(item.PollingID1, item.PollingName1, item.NodeName1);
    ensureNode(item.PollingID2, item.PollingName2, item.NodeName2);
    const positive = item.Score >= 0;
    const width = Math.max(1.5, Math.min(6, item.AbsScore * 6));
    return {
      source: item.PollingID1,
      target: item.PollingID2,
      value: item.Score,
      itemIndex: idx,
      lineStyle: {
        color: positive ? '#3b82f6' : '#ef4444',
        width,
        type: positive ? ('solid' as const) : ('dashed' as const),
        opacity: item.SameNode ? 0.55 : 0.9,
        curveness: 0.12,
      },
    };
  });

  const nodes = Array.from(nodeMeta.values()).map((meta) => {
    const deg = nodeDegree.get(meta.id) || 1;
    return {
      id: meta.id,
      name: `${meta.nodeName}\n${meta.name}`,
      category: meta.catIdx,
      symbolSize: Math.min(44, 18 + deg * 5),
      label: {
        show: true,
        color: textColor,
        fontSize: 10,
      },
    };
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: tooltipBg,
      borderColor: tooltipBorder,
      textStyle: { color: textColor, fontSize: 12 },
      formatter: (params: any) => {
        if (params.dataType === 'edge') {
          const item = items[params.data.itemIndex];
          if (!item) return '';
          const lagStr = item.BestLagSec !== 0 ? `<br/>${labels.lagLabel || 'Lag'}: ${item.BestLagSec}s` : '';
          return (
            `<b>${item.NodeName1} / ${item.PollingName1}</b> (${item.BestVar1})<br/>` +
            `↔ <b>${item.NodeName2} / ${item.PollingName2}</b> (${item.BestVar2})<br/>` +
            `Score: <b>${item.Score.toFixed(4)}</b> | p=${item.PValue.toExponential(2)} | n=${item.Samples}${lagStr}`
          );
        }
        return params.data.name.replace('\n', ' / ');
      },
    },
    legend:
      categories.length <= 12
        ? [
            {
              data: categories.map((c) => c.name),
              textStyle: { color: textColor, fontSize: 11 },
              top: 4,
              type: 'scroll',
            },
          ]
        : undefined,
    series: [
      {
        type: 'graph',
        layout: 'force',
        roam: true,
        draggable: true,
        categories,
        data: nodes,
        links,
        edgeSymbol: ['none', 'none'],
        force: {
          repulsion: 260,
          edgeLength: [80, 180],
          gravity: 0.08,
        },
        emphasis: {
          focus: 'adjacency',
          lineStyle: { width: 5 },
        },
        itemStyle: {
          borderColor: dark ? '#0f172a' : '#ffffff',
          borderWidth: 1.5,
        },
      },
    ],
  };

  chart.setOption(option);
  if (onSelectPair) {
    chart.on('click', (params: any) => {
      if (params.dataType === 'edge' && params.data?.itemIndex !== undefined) {
        const item = items[params.data.itemIndex];
        if (item) onSelectPair(item);
      }
    });
  }
  return chart;
};
