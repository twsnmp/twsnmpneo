import * as echarts from 'echarts';
import { isDarkMode } from './utils';

export interface AIChartLabels {
  heatmapDate: string;
  heatmapHour: string;
  anomalyScore: string;
  normal: string;
  warn: string;
  anomaly: string;
}

function initThemedChart(dom: HTMLElement | string): {
  chart: echarts.ECharts;
  dark: boolean;
  textColor: string;
  lineColor: string;
} | null {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return null;
  const existing = echarts.getInstanceByDom(el);
  if (existing) {
    existing.dispose();
  }
  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);
  return {
    chart,
    dark,
    textColor: dark ? '#cbd5e1' : '#475569',
    lineColor: dark ? '#334155' : '#e2e8f0',
  };
}

export function renderAIHeatmap(
  dom: HTMLElement | string,
  scores: [number, number][],
  labels: Pick<AIChartLabels, 'heatmapDate' | 'heatmapHour'>
): echarts.ECharts | undefined {
  const ctx = initThemedChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;
  const hours = Array.from({ length: 24 }, (_, i) => String(i));
  const option: any = {
    backgroundColor: 'transparent',
    title: { show: false },
    grid: {
      left: '8%',
      right: '5%',
      top: 30,
      bottom: 70,
    },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 15, height: 18 }],
    tooltip: {
      trigger: 'item',
      formatter(params: any) {
        return `${params.name} ${params.data[1]}:00 : ${Number(params.data[2]).toFixed(2)}`;
      },
    },
    xAxis: {
      type: 'category',
      name: labels.heatmapDate || 'Date',
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      data: [] as string[],
    },
    yAxis: {
      type: 'category',
      name: labels.heatmapHour || 'Hour',
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      data: hours,
    },
    visualMap: {
      min: 40,
      max: 80,
      textStyle: { color: textColor, fontSize: 10 },
      calculable: true,
      realtime: false,
      inRange: {
        color: [
          '#313695',
          '#4575b4',
          '#74add1',
          '#abd9e9',
          '#e0f3f8',
          '#ffffbf',
          '#fee090',
          '#fdae61',
          '#f46d43',
          '#d73027',
          '#a50026',
        ],
      },
    },
    series: [
      {
        name: 'Score',
        type: 'heatmap',
        data: [] as [number, number, number][],
        emphasis: {
          itemStyle: {
            borderColor: '#fff',
            borderWidth: 1,
          },
        },
      },
    ],
  };

  let nD = 0;
  let x = -1;
  (scores || []).forEach((e) => {
    const t = new Date(e[0] * 1000);
    if (nD !== t.getDate()) {
      const dStr = `${t.getFullYear()}/${String(t.getMonth() + 1).padStart(2, '0')}/${String(t.getDate()).padStart(2, '0')}`;
      option.xAxis.data.push(dStr);
      nD = t.getDate();
      x++;
    }
    option.series[0].data.push([x, t.getHours(), e[1]]);
  });

  chart.setOption(option);
  return chart;
}

export function renderAIPieChart(
  dom: HTMLElement | string,
  scores: [number, number][],
  labels: Pick<AIChartLabels, 'anomalyScore' | 'normal' | 'warn' | 'anomaly'>
): echarts.ECharts | undefined {
  const ctx = initThemedChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor } = ctx;

  let normal = 0;
  let warn = 0;
  let anomaly = 0;
  (scores || []).forEach((e) => {
    const s = e[1];
    if (s > 66.0) anomaly++;
    else if (s > 50.0) warn++;
    else normal++;
  });

  const normalLabel = labels.normal || 'Normal';
  const warnLabel = labels.warn || 'Warning';
  const anomalyLabel = labels.anomaly || 'Anomaly';
  const scoreLabel = labels.anomalyScore || 'Anomaly Score';

  chart.setOption({
    backgroundColor: 'transparent',
    title: { show: false },
    color: ['#10b981', '#f59e0b', '#ef4444'],
    tooltip: {
      trigger: 'item',
      formatter: '{a} <br/>{b} : {c} ({d}%)',
    },
    legend: {
      top: 20,
      textStyle: { color: textColor, fontSize: 12 },
      data: [normalLabel, warnLabel, anomalyLabel],
    },
    series: [
      {
        name: scoreLabel,
        type: 'pie',
        radius: ['40%', '70%'],
        center: ['50%', '55%'],
        label: { color: textColor, fontSize: 11 },
        data: [
          { name: normalLabel, value: normal },
          { name: warnLabel, value: warn },
          { name: anomalyLabel, value: anomaly },
        ],
      },
    ],
  });
  return chart;
}

export function renderAITimeChart(
  dom: HTMLElement | string,
  scores: [number, number][],
  labels: Pick<AIChartLabels, 'anomalyScore' | 'normal' | 'anomaly'>
): echarts.ECharts | undefined {
  const ctx = initThemedChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;
  const scoreLabel = labels.anomalyScore || 'Anomaly Score';

  chart.setOption({
    backgroundColor: 'transparent',
    title: { show: false },
    grid: {
      left: '8%',
      right: '5%',
      top: 30,
      bottom: 70,
    },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 15, height: 18 }],
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' },
    },
    xAxis: {
      type: 'time',
      nameTextStyle: { color: textColor },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis: {
      type: 'value',
      name: scoreLabel,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      {
        name: scoreLabel,
        type: 'line',
        smooth: true,
        color: '#38bdf8',
        showSymbol: false,
        data: (scores || []).map((e) => [new Date(e[0] * 1000), e[1]]),
        markLine: {
          silent: true,
          data: [
            { yAxis: 50, lineStyle: { color: '#10b981', type: 'dashed' }, label: { formatter: `50 (${labels.normal || 'Normal'})` } },
            { yAxis: 66, lineStyle: { color: '#ef4444', type: 'dashed' }, label: { formatter: `66 (${labels.anomaly || 'Anomaly'})` } },
          ],
        },
      },
    ],
  });
  return chart;
}
