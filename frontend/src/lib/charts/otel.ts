import * as echarts from 'echarts';
import { setZoomCallback, isDarkMode } from './utils';
import { locale } from 'svelte-i18n';
import { get } from 'svelte/store';

const getIsJa = () => (get(locale) || 'ja').startsWith('ja');

/**
 * Renders an interactive scatter plot of traces over time.
 * X-axis: Time, Y-axis: Duration (Seconds), Color: Duration gradient, Size: Span count.
 */
export function showOTelTrace(
  div: string | HTMLElement,
  traces: any[],
  zoomCallback?: (st: number, et: number) => void
): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(el, dark ? 'dark' : undefined);
  let maxDur = 0.1;

  let st = Infinity;
  let lt = 0;
  const seriesData: any[] = [];
  traces.forEach((t: any) => {
    const ts = new Date(t.Start / (1000 * 1000));
    const durSec = t.Dur || 0;
    if (durSec > maxDur) {
      maxDur = durSec;
    }
    if (t.Start && t.Start < st) st = t.Start;
    if (t.End && t.End > lt) lt = t.End;
    seriesData.push([ts, durSec, t.NumSpan || 1, t.TraceID || '', t.Services || '']);
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b' },
      formatter: (params: any) => {
        const d = params.data;
        let durStr = d[1].toFixed(3) + ' Sec';
        if (d[1] < 0.001) {
          durStr = (d[1] * 1000 * 1000).toFixed(3) + ' µs';
        } else if (d[1] < 1.0) {
          durStr = (d[1] * 1000).toFixed(3) + ' ms';
        }
        return `
          <div style="font-size: 11px; line-height: 1.5;">
            <div><strong>${isJa ? '日時:' : 'Time:'}</strong> ${echarts.time.format(d[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}.{SSS}', false)}</div>
            <div><strong>${isJa ? 'サービス:' : 'Service:'}</strong> ${d[4] || '-'}</div>
            <div><strong>${isJa ? '所要時間:' : 'Duration:'}</strong> <span style="color:#38bdf8; font-weight:bold;">${durStr}</span></div>
            <div><strong>${isJa ? 'スパン数:' : 'Spans:'}</strong> ${d[2]}</div>
            <div style="font-family:monospace; color:${dark ? '#94a3b8' : '#64748b'}; font-size:10px;">ID: ${d[3]}</div>
          </div>
        `;
      },
    },
    grid: {
      left: '65px',
      right: '85px',
      top: '40px',
      bottom: '55px',
    },
    dataZoom: [
      {
        type: 'slider',
        showDetail: false,
        bottom: 8,
        height: 16,
        borderColor: dark ? '#334155' : '#cbd5e1',
        backgroundColor: dark ? '#020617' : '#f8fafc',
        fillerColor: dark ? 'rgba(56, 189, 248, 0.2)' : 'rgba(56, 189, 248, 0.15)',
        handleStyle: { color: '#38bdf8' },
      },
      {
        type: 'inside',
      },
    ],
    visualMap: {
      min: 0,
      max: maxDur,
      dimension: 1,
      calculable: true,
      orient: 'vertical',
      right: '15px',
      top: 'middle',
      itemWidth: 12,
      itemHeight: 90,
      text: isJa ? ['遅い', '速い'] : ['Slow', 'Fast'],
      textGap: 8,
      textStyle: { color: dark ? '#94a3b8' : '#475569', fontSize: 10 },
      inRange: {
        color: [
          '#38bdf8', // Light Cyan/Blue
          '#34d399', // Emerald
          '#fbbf24', // Amber
          '#f87171', // Red
          '#e11d48', // Deep Rose
        ],
      },
    },
    xAxis: {
      type: 'time',
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 10,
        margin: 12,
        formatter: (val: any) => echarts.time.format(val, '{HH}:{mm}:{ss}', false),
      },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#f1f5f9' } },
    },
    series: [
      {
        name: 'Traces',
        type: 'scatter',
        data: seriesData,
        symbolSize: (val: any) => {
          const spans = val[2] || 1;
          return Math.min(24, Math.max(6, spans * 3));
        },
      },
    ],
  };

  chart.setOption(option);
  if (zoomCallback) {
    setZoomCallback(chart, zoomCallback, st, lt);
  }
  return chart;
}

/**
 * Renders service call dependency graph (DAG).
 */
export function showOTelDAG(div: string | HTMLElement, data: { Nodes: any[]; Links: any[] }): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const chart = echarts.init(el, isDarkMode() ? 'dark' : undefined);

  let maxNodeCount = 1;
  const nodes = (data.Nodes || []).map((n) => {
    if (n.Count > maxNodeCount) maxNodeCount = n.Count;
    return {
      name: n.Name,
      value: n.Count,
      draggable: true,
    };
  });

  let maxLinkCount = 1;
  const links = (data.Links || []).map((l) => {
    if (l.Count > maxLinkCount) maxLinkCount = l.Count;
    return {
      source: l.Src,
      target: l.Dst,
      value: l.Count,
    };
  });

  const colorPalette = ['#38bdf8', '#818cf8', '#a855f7', '#ec4899', '#f43f5e'];

  const graphNodes = nodes.map((n) => {
    const ratio = n.value / maxNodeCount;
    const size = 16 + ratio * 32;
    const colorIdx = Math.min(colorPalette.length - 1, Math.floor(ratio * (colorPalette.length - 1)));
    return {
      name: n.name,
      symbolSize: size,
      itemStyle: {
        color: colorPalette[colorIdx],
        borderColor: '#f8fafc',
        borderWidth: 1.5,
      },
      label: {
        show: true,
        position: 'right' as const,
        color: '#f1f5f9',
        fontSize: 12,
        fontWeight: 'bold' as const,
      },
      value: n.value,
    };
  });

  const graphLinks = links.map((l) => {
    const ratio = l.value / maxLinkCount;
    return {
      source: l.source,
      target: l.target,
      lineStyle: {
        width: 1.5 + ratio * 6,
        color: '#64748b',
        curveness: 0.2,
      },
      value: l.value,
    };
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      formatter: (params: any) => {
        const isJa = getIsJa();
        if (params.dataType === 'node') {
          return `<strong>${isJa ? 'サービス:' : 'Service:'}</strong> ${params.name}<br/><strong>${isJa ? 'リクエスト数:' : 'Requests:'}</strong> ${params.value}`;
        } else if (params.dataType === 'edge') {
          return `<strong>${isJa ? '呼び出し関係:' : 'Call Dependency:'}</strong> ${params.data.source} → ${params.data.target}<br/><strong>${isJa ? '回数:' : 'Count:'}</strong> ${params.data.value}`;
        }
        return '';
      },
    },
    series: [
      {
        name: 'Service DAG',
        type: 'graph',
        layout: 'force',
        edgeSymbol: ['none', 'arrow'],
        edgeSymbolSize: [4, 10],
        force: {
          repulsion: 300,
          edgeLength: [100, 200],
        },
        roam: true,
        data: graphNodes,
        links: graphLinks,
        emphasis: {
          focus: 'adjacency',
          lineStyle: {
            color: '#38bdf8',
            width: 3,
          },
        },
      },
    ],
  };

  chart.setOption(option);
  return chart;
}

/**
 * Renders span waterfall (Gantt-like timeline chart).
 */
export function showOTelTimeline(div: string | HTMLElement, trace: any): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el || !trace || !trace.Spans || trace.Spans.length === 0) return null;

  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);

  const spans = [...trace.Spans].sort((a: any, b: any) => a.Start - b.Start);
  const baseStart = spans[0].Start;

  const categories: string[] = [];
  const startOffsets: number[] = [];
  const durations: any[] = [];

  const colorList = ['#38bdf8', '#34d399', '#fbbf24', '#f87171', '#a855f7', '#6366f1', '#ec4899'];

  spans.forEach((s: any, idx: number) => {
    categories.push(s.Name || `Span-${idx}`);
    const offsetMs = (s.Start - baseStart) / (1000 * 1000);
    const durMs = (s.End - s.Start) / (1000 * 1000);
    startOffsets.push(offsetMs);
    durations.push({
      value: Math.max(0.01, durMs),
      itemStyle: {
        color: colorList[idx % colorList.length],
        borderRadius: [2, 4, 4, 2],
      },
      service: s.Service,
      spanID: s.SpanID,
    });
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b' },
      formatter: (params: any) => {
        const durItem = params.find((p: any) => p.seriesName === 'Duration');
        const startItem = params.find((p: any) => p.seriesName === 'Start');
        if (!durItem) return '';
        const offset = startItem ? startItem.value.toFixed(2) : 0;
        const dur = durItem.value.toFixed(3);
        const dataObj = durItem.data;
        const isJa = getIsJa();
        return `
          <div style="font-size:11px;">
            <div><strong>${durItem.name}</strong></div>
            <div>${isJa ? 'サービス:' : 'Service:'} ${dataObj.service || '-'}</div>
            <div>${isJa ? '開始オフセット:' : 'Start Offset:'} +${offset} ms</div>
            <div>${isJa ? '所要時間:' : 'Duration:'} <span style="color:#38bdf8; font-weight:bold;">${dur} ms</span></div>
          </div>
        `;
      },
    },
    grid: {
      left: '200px',
      right: '40px',
      top: '20px',
      bottom: '40px',
    },
    xAxis: {
      type: 'value',
      name: 'ms',
      nameTextStyle: { color: dark ? '#94a3b8' : '#64748b' },
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#e2e8f0' } },
    },
    yAxis: {
      type: 'category',
      data: categories,
      axisLabel: {
        color: dark ? '#cbd5e1' : '#334155',
        fontSize: 11,
        width: 180,
        overflow: 'truncate',
      },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
    },
    series: [
      {
        name: 'Start',
        type: 'bar',
        stack: 'time',
        itemStyle: {
          borderColor: 'transparent',
          color: 'transparent',
        },
        emphasis: {
          itemStyle: {
            borderColor: 'transparent',
            color: 'transparent',
          },
        },
        data: startOffsets,
      },
      {
        name: 'Duration',
        type: 'bar',
        stack: 'time',
        data: durations,
      },
    ],
  };

  chart.setOption(option);
  return chart;
}

/**
 * Renders donut chart for OTel metric types distribution.
 */
export function showOTelMetricTypePie(div: string | HTMLElement, metrics: any[]): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);
  const typeMap: Record<string, number> = {};
  for (const m of metrics || []) {
    const t = m.Type || 'Unknown';
    typeMap[t] = (typeMap[t] || 0) + 1;
  }

  const data = Object.entries(typeMap).map(([name, value]) => ({ name, value }));
  const colorPalette = ['#38bdf8', '#34d399', '#fbbf24', '#a855f7', '#f87171', '#6366f1', '#ec4899'];

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b', fontSize: 11 },
      formatter: '{b}: <strong>{c}</strong> ({d}%)',
    },
    legend: {
      orient: 'vertical',
      right: '2%',
      top: 'middle',
      textStyle: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      itemWidth: 10,
      itemHeight: 10,
    },
    series: [
      {
        name: 'Metric Types',
        type: 'pie',
        radius: ['45%', '72%'],
        center: ['36%', '50%'],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 4,
          borderColor: dark ? '#0f172a' : '#ffffff',
          borderWidth: 2,
        },
        label: {
          show: false,
          position: 'center',
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 12,
            fontWeight: 'bold',
            color: dark ? '#f8fafc' : '#0f172a',
          },
        },
        data: data.length > 0 ? data : [{ name: getIsJa() ? 'データなし' : 'No Data', value: 0 }],
        color: colorPalette,
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
}

/**
 * Renders horizontal bar chart for top services emitting metrics.
 */
export function showOTelServiceMetricBar(div: string | HTMLElement, metrics: any[]): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);
  const svcMap: Record<string, number> = {};
  for (const m of metrics || []) {
    const s = m.Service || 'unknown';
    svcMap[s] = (svcMap[s] || 0) + 1;
  }

  // Allow up to top 10 services with expanded vertical space
  const sorted = Object.entries(svcMap).sort((a, b) => b[1] - a[1]).slice(0, 10).reverse();
  const categories = sorted.map((s) => s[0]);
  const counts = sorted.map((s) => s[1]);

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b', fontSize: 11 },
      formatter: (params: any) => {
        if (!params || params.length === 0) return '';
        return `${params[0].name}: <strong>${params[0].value}</strong> ${getIsJa() ? '系列' : 'Series'}`;
      },
    },
    grid: {
      left: '10px',
      right: '40px',
      top: '15px',
      bottom: '10px',
      containLabel: true,
    },
    xAxis: {
      type: 'value',
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 9 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#f1f5f9' } },
    },
    yAxis: {
      type: 'category',
      data: categories,
      axisLabel: {
        color: dark ? '#cbd5e1' : '#334155',
        fontSize: 11,
        width: 120,
        overflow: 'truncate',
      },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
    },
    series: [
      {
        name: 'Metrics',
        type: 'bar',
        data: counts,
        barMaxWidth: 22,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
            { offset: 0, color: '#06b6d4' },
            { offset: 1, color: '#3b82f6' },
          ]),
          borderRadius: [0, 4, 4, 0],
        },
        label: {
          show: true,
          position: 'right',
          color: dark ? '#cbd5e1' : '#334155',
          fontSize: 10,
        },
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
}

/**
 * Renders time-series chart of metric values.
 */
export function showOTelTimeChart(
  div: string | HTMLElement,
  dataPoints: any[],
  filterAttr?: string
): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el || !dataPoints || dataPoints.length === 0) return null;

  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);

  const points = filterAttr && filterAttr !== 'all'
    ? dataPoints.filter((dp) => (dp.Attributes?.join(' ') || '') === filterAttr)
    : dataPoints;

  const groupMap = new Map<string, any[]>();
  for (const dp of points) {
    const key = dp.Attributes && dp.Attributes.length > 0 ? dp.Attributes.join(' ') : 'Default';
    if (!groupMap.has(key)) {
      groupMap.set(key, []);
    }
    groupMap.get(key)!.push(dp);
  }

  const colorPalette = ['#38bdf8', '#34d399', '#fbbf24', '#f87171', '#a855f7', '#6366f1', '#ec4899'];
  const series: any[] = [];
  const legendNames: string[] = [];

  let colorIdx = 0;
  groupMap.forEach((pts, key) => {
    legendNames.push(key);
    const sData = pts.map((dp: any) => {
      const val = dp.Sum ?? dp.Gauge ?? (dp.Count ? Number(dp.Count) : 0);
      return [new Date(dp.Time / (1000 * 1000)), val];
    });

    const c = colorPalette[colorIdx % colorPalette.length];
    series.push({
      name: key,
      type: 'line',
      showSymbol: sData.length < 50,
      smooth: true,
      data: sData,
      lineStyle: { color: c, width: 2 },
      itemStyle: { color: c },
      areaStyle: groupMap.size === 1 ? {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: 'rgba(56, 189, 248, 0.3)' },
          { offset: 1, color: 'rgba(56, 189, 248, 0.0)' },
        ]),
      } : undefined,
    });
    colorIdx++;
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b' },
      formatter: (params: any) => {
        if (!params || params.length === 0) return '';
        const timeStr = echarts.time.format(params[0].data[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false);
        let html = `<div style="font-size:11px;"><div><strong>${timeStr}</strong></div>`;
        for (const p of params) {
          html += `<div><span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color};margin-right:4px;"></span>${p.seriesName}: <strong>${p.data[1]}</strong></div>`;
        }
        html += `</div>`;
        return html;
      },
    },
    legend: groupMap.size > 1 ? {
      data: legendNames,
      top: 5,
      textStyle: { color: dark ? '#94a3b8' : '#475569', fontSize: 10 },
      type: 'scroll',
    } : undefined,
    grid: {
      left: '50px',
      right: '30px',
      top: groupMap.size > 1 ? '40px' : '25px',
      bottom: '50px',
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 10,
        height: 18,
        borderColor: dark ? '#334155' : '#cbd5e1',
        backgroundColor: dark ? '#020617' : '#f8fafc',
        fillerColor: dark ? 'rgba(56, 189, 248, 0.2)' : 'rgba(56, 189, 248, 0.15)',
        handleStyle: { color: '#38bdf8' },
        textStyle: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      },
    ],
    xAxis: {
      type: 'time',
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(val, '{HH}:{mm}:{ss}', false),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#e2e8f0' } },
    },
    series: series,
  };

  chart.setOption(option);
  chart.resize();
  return chart;
}

/**
 * Renders histogram buckets bar chart.
 */
export function showOTelHistogram(div: string | HTMLElement, dp: any): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el || !dp) return null;

  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);

  const bounds = dp.ExplicitBounds || [];
  const counts = dp.BucketCounts || [];
  const categories: string[] = [];

  for (let i = 0; i < counts.length; i++) {
    if (i === 0 && bounds.length > 0) {
      categories.push(`≤ ${bounds[0]}`);
    } else if (i === counts.length - 1) {
      categories.push(`> ${bounds[bounds.length - 1] ?? 0}`);
    } else {
      categories.push(`${bounds[i - 1]} ~ ${bounds[i]}`);
    }
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: dark ? '#0f172a' : '#ffffff',
      borderColor: dark ? '#334155' : '#cbd5e1',
      textStyle: { color: dark ? '#f8fafc' : '#1e293b' },
      formatter: (params: any) => {
        if (!params || params.length === 0) return '';
        const p = params[0];
        const isJa = getIsJa();
        return `
          <div style="font-size:11px;">
            <div>${isJa ? 'バケット範囲:' : 'Bucket Range:'} <strong>${p.name}</strong></div>
            <div>${isJa ? '度数 (件数):' : 'Frequency (Count):'} <strong style="color:#34d399;">${p.value}</strong></div>
          </div>
        `;
      },
    },
    grid: {
      left: '60px',
      right: '30px',
      top: '30px',
      bottom: '60px',
    },
    xAxis: {
      type: 'category',
      data: categories,
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 10,
        rotate: 30,
        interval: 0,
      },
      axisLine: { lineStyle: { color: dark ? '#334155' : '#cbd5e1' } },
    },
    yAxis: {
      type: 'value',
      name: getIsJa() ? '度数 (件数)' : 'Frequency (Count)',
      nameTextStyle: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      axisLabel: { color: dark ? '#94a3b8' : '#64748b', fontSize: 10 },
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#e2e8f0' } },
    },
    series: [
      {
        name: 'Counts',
        type: 'bar',
        data: counts,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#34d399' },
            { offset: 1, color: '#059669' },
          ]),
          borderRadius: [4, 4, 0, 0],
        },
        label: {
          show: true,
          position: 'top',
          color: '#a7f3d0',
          fontSize: 10,
        },
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
}

/**
 * Renders an interactive stacked histogram of OpenTelemetry logs over time.
 * Series: ERROR, WARN, INFO, DEBUG
 */
export function showOTelLogChart(
  div: string | HTMLElement,
  logs: any[],
  zoomCallback?: (st: number, et: number) => void
): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const existing = echarts.getInstanceByDom(el);
  if (existing) {
    existing.dispose();
  }
  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);

  const data: Record<string, [Date, number][]> = {
    ERROR: [],
    WARN: [],
    INFO: [],
    DEBUG: [],
  };

  const count: Record<string, number> = {
    ERROR: 0,
    WARN: 0,
    INFO: 0,
    DEBUG: 0,
  };

  const addChartData = (ctm: number, newCtm: number) => {
    let t = new Date(ctm * 60 * 1000);
    for (const k of ['ERROR', 'WARN', 'INFO', 'DEBUG']) {
      data[k].push([t, count[k]]);
    }
    ctm++;
    for (; ctm < newCtm; ctm++) {
      t = new Date(ctm * 60 * 1000);
      for (const k of ['ERROR', 'WARN', 'INFO', 'DEBUG']) {
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

    let lvlKey = (e.level || e.severityText || '').toUpperCase();
    if (lvlKey.includes('ERR') || lvlKey.includes('FATAL') || lvlKey.includes('CRIT') || (typeof e.severity === 'number' && e.severity <= 3)) {
      lvlKey = 'ERROR';
    } else if (lvlKey.includes('WARN') || (typeof e.severity === 'number' && e.severity === 4)) {
      lvlKey = 'WARN';
    } else if (lvlKey.includes('INFO') || (typeof e.severity === 'number' && (e.severity === 5 || e.severity === 6))) {
      lvlKey = 'INFO';
    } else {
      lvlKey = 'DEBUG';
    }

    const newCtm = Math.floor(tMs / (60 * 1000));
    if (ctm === undefined) {
      ctm = newCtm;
    }
    if (ctm !== newCtm) {
      ctm = addChartData(ctm, newCtm);
      for (const k in count) count[k] = 0;
    }
    count[lvlKey]++;
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
      textStyle: { color: dark ? '#f8fafc' : '#1e293b', fontSize: 11 },
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
        borderColor: dark ? '#334155' : '#cbd5e1',
        backgroundColor: dark ? '#020617' : '#f8fafc',
        fillerColor: dark ? 'rgba(6, 182, 212, 0.2)' : 'rgba(6, 182, 212, 0.15)',
        handleStyle: { color: '#06b6d4' },
        textStyle: { color: dark ? '#64748b' : '#64748b', fontSize: 9 },
      },
      {
        type: 'inside',
      },
    ],
    legend: {
      top: 10,
      textStyle: { color: dark ? '#cbd5e1' : '#334155', fontSize: 11 },
      data: ['ERROR', 'WARN', 'INFO', 'DEBUG'],
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
      splitLine: { lineStyle: { color: dark ? '#1e293b' : '#e2e8f0', type: 'dashed' } },
    },
    series: [
      {
        name: 'ERROR',
        type: 'bar',
        stack: 'count',
        color: '#f43f5e',
        data: data.ERROR,
      },
      {
        name: 'WARN',
        type: 'bar',
        stack: 'count',
        color: '#eab308',
        data: data.WARN,
      },
      {
        name: 'INFO',
        type: 'bar',
        stack: 'count',
        color: '#06b6d4',
        data: data.INFO,
      },
      {
        name: 'DEBUG',
        type: 'bar',
        stack: 'count',
        color: '#64748b',
        data: data.DEBUG,
      },
    ],
  };

  chart.setOption(option, true);
  chart.resize();
  return chart;
}

