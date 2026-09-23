import * as echarts from 'echarts';

/**
 * Renders an interactive scatter plot of traces over time.
 * X-axis: Time, Y-axis: Duration (Seconds), Color: Duration gradient, Size: Span count.
 */
export function showOTelTrace(div: string | HTMLElement, traces: any[]): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const chart = echarts.init(el, 'dark');
  let maxDur = 0.1;

  const seriesData: any[] = [];
  traces.forEach((t: any) => {
    const ts = new Date(t.Start / (1000 * 1000));
    const durSec = t.Dur || 0;
    if (durSec > maxDur) {
      maxDur = durSec;
    }
    seriesData.push([ts, durSec, t.NumSpan || 1, t.TraceID || '', t.Services || '']);
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
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
            <div><strong>日時:</strong> ${echarts.time.format(d[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}.{SSS}', false)}</div>
            <div><strong>サービス:</strong> ${d[4] || '-'}</div>
            <div><strong>所要時間:</strong> <span style="color:#38bdf8; font-weight:bold;">${durStr}</span></div>
            <div><strong>スパン数:</strong> ${d[2]}</div>
            <div style="font-family:monospace; color:#94a3b8; font-size:10px;">ID: ${d[3]}</div>
          </div>
        `;
      },
    },
    grid: {
      left: '60px',
      right: '40px',
      top: '30px',
      bottom: '60px',
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 10,
        height: 18,
        borderColor: '#334155',
        fillerColor: 'rgba(56, 189, 248, 0.2)',
        handleStyle: { color: '#38bdf8' },
        textStyle: { color: '#94a3b8', fontSize: 10 },
      },
    ],
    visualMap: {
      min: 0,
      max: maxDur,
      dimension: 1,
      calculable: true,
      orient: 'horizontal',
      right: '20px',
      top: '0px',
      textStyle: { color: '#94a3b8', fontSize: 10 },
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
        color: '#94a3b8',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(val, '{HH}:{mm}:{ss}', false),
      },
      axisLine: { lineStyle: { color: '#334155' } },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      name: 'Sec',
      nameTextStyle: { color: '#94a3b8', fontSize: 10 },
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      axisLine: { lineStyle: { color: '#334155' } },
      splitLine: { lineStyle: { color: '#1e293b' } },
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
  return chart;
}

/**
 * Renders service call dependency graph (DAG).
 */
export function showOTelDAG(div: string | HTMLElement, data: { Nodes: any[]; Links: any[] }): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el) return null;

  const chart = echarts.init(el, 'dark');

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
        if (params.dataType === 'node') {
          return `<strong>サービス:</strong> ${params.name}<br/><strong>リクエスト数:</strong> ${params.value}`;
        } else if (params.dataType === 'edge') {
          return `<strong>呼び出し関係:</strong> ${params.data.source} → ${params.data.target}<br/><strong>回数:</strong> ${params.data.value}`;
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

  const chart = echarts.init(el, 'dark');

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
      formatter: (params: any) => {
        const durItem = params.find((p: any) => p.seriesName === 'Duration');
        const startItem = params.find((p: any) => p.seriesName === 'Start');
        if (!durItem) return '';
        const offset = startItem ? startItem.value.toFixed(2) : 0;
        const dur = durItem.value.toFixed(3);
        const dataObj = durItem.data;
        return `
          <div style="font-size:11px;">
            <div><strong>${durItem.name}</strong></div>
            <div>サービス: ${dataObj.service || '-'}</div>
            <div>開始オフセット: +${offset} ms</div>
            <div>所要時間: <span style="color:#38bdf8; font-weight:bold;">${dur} ms</span></div>
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
      nameTextStyle: { color: '#94a3b8' },
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    yAxis: {
      type: 'category',
      data: categories,
      axisLabel: {
        color: '#cbd5e1',
        fontSize: 11,
        width: 180,
        overflow: 'truncate',
      },
      axisLine: { lineStyle: { color: '#334155' } },
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
 * Renders time-series chart of metric values.
 */
export function showOTelTimeChart(div: string | HTMLElement, dataPoints: any[]): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el || !dataPoints || dataPoints.length === 0) return null;

  const chart = echarts.init(el, 'dark');

  const seriesData: [Date, number][] = dataPoints.map((dp: any) => {
    const val = dp.Sum ?? dp.Gauge ?? (dp.Count ? Number(dp.Count) : 0);
    return [new Date(dp.Time / (1000 * 1000)), val];
  });

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      formatter: (params: any) => {
        if (!params || params.length === 0) return '';
        const d = params[0].data;
        return `
          <div style="font-size:11px;">
            <div>${echarts.time.format(d[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false)}</div>
            <div>値: <strong style="color:#38bdf8;">${d[1]}</strong></div>
          </div>
        `;
      },
    },
    grid: {
      left: '50px',
      right: '30px',
      top: '30px',
      bottom: '50px',
    },
    dataZoom: [
      {
        type: 'slider',
        bottom: 10,
        height: 18,
        borderColor: '#334155',
        fillerColor: 'rgba(56, 189, 248, 0.2)',
        handleStyle: { color: '#38bdf8' },
        textStyle: { color: '#94a3b8', fontSize: 10 },
      },
    ],
    xAxis: {
      type: 'time',
      axisLabel: {
        color: '#94a3b8',
        fontSize: 10,
        formatter: (val: any) => echarts.time.format(val, '{HH}:{mm}:{ss}', false),
      },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    series: [
      {
        type: 'line',
        showSymbol: seriesData.length < 50,
        smooth: true,
        data: seriesData,
        lineStyle: { color: '#38bdf8', width: 2 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(56, 189, 248, 0.3)' },
            { offset: 1, color: 'rgba(56, 189, 248, 0.0)' },
          ]),
        },
      },
    ],
  };

  chart.setOption(option);
  return chart;
}

/**
 * Renders histogram buckets bar chart.
 */
export function showOTelHistogram(div: string | HTMLElement, dp: any): echarts.ECharts | null {
  const el = typeof div === 'string' ? document.getElementById(div) : div;
  if (!el || !dp || !dp.BucketCounts || dp.BucketCounts.length === 0) return null;

  const chart = echarts.init(el, 'dark');

  const bounds = dp.ExplicitBounds || [];
  const categories: string[] = [];
  for (let i = 0; i < dp.BucketCounts.length; i++) {
    if (i === 0 && bounds.length > 0) {
      categories.push(`≤ ${bounds[0]}`);
    } else if (i === dp.BucketCounts.length - 1) {
      categories.push(`> ${bounds[bounds.length - 1] ?? 0}`);
    } else {
      categories.push(`${bounds[i - 1]} - ${bounds[i]}`);
    }
  }

  const option: echarts.EChartsOption = {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    grid: {
      left: '50px',
      right: '30px',
      top: '30px',
      bottom: '40px',
    },
    xAxis: {
      type: 'category',
      data: categories,
      axisLabel: { color: '#94a3b8', fontSize: 10, rotate: 25 },
    },
    yAxis: {
      type: 'value',
      name: '件数',
      axisLabel: { color: '#94a3b8', fontSize: 10 },
      splitLine: { lineStyle: { color: '#1e293b' } },
    },
    series: [
      {
        name: 'Counts',
        type: 'bar',
        data: dp.BucketCounts,
        itemStyle: {
          color: '#34d399',
          borderRadius: [4, 4, 0, 0],
        },
      },
    ],
  };

  chart.setOption(option);
  return chart;
}
