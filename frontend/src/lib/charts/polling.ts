import * as echarts from 'echarts';
import type { PollingLogEnt, PollingTimeAnalyzeResp } from '../api';
import { isDarkMode, setZoomCallback } from './utils';

export interface MetricMeta {
  mul: number;
  unit: string;
  labelJa: string;
  labelEn: string;
}

export const METRIC_PARAMS: Record<string, MetricMeta> = {
  rtt: { mul: 1e-6, unit: 'ms', labelJa: '応答時間', labelEn: 'RTT' },
  offset: { mul: 1e-6, unit: 'ms', labelJa: 'オフセット', labelEn: 'Offset' },
  stratum: { mul: 1, unit: '', labelJa: 'ストラタム', labelEn: 'Stratum' },
  ttl: { mul: 1, unit: '', labelJa: 'TTL', labelEn: 'TTL' },
  recv: { mul: 1, unit: '', labelJa: '受信数', labelEn: 'Received' },
  send: { mul: 1, unit: '', labelJa: '送信数', labelEn: 'Sent' },
  loss: { mul: 1, unit: '%', labelJa: '損失率', labelEn: 'Loss' },
  count: { mul: 1, unit: '', labelJa: '件数', labelEn: 'Count' },
  total: { mul: 1, unit: '', labelJa: '合計', labelEn: 'Total' },
  val: { mul: 1, unit: '', labelJa: '値', labelEn: 'Value' },
  value: { mul: 1, unit: '', labelJa: '値', labelEn: 'Value' },
  rate: { mul: 1, unit: '%', labelJa: '割合', labelEn: 'Rate' },
  speed: { mul: 1, unit: 'bps', labelJa: '速度', labelEn: 'Speed' },
  bps: { mul: 1, unit: 'bps', labelJa: 'bps', labelEn: 'bps' },
  pps: { mul: 1, unit: 'pps', labelJa: 'pps', labelEn: 'pps' },
  code: { mul: 1, unit: '', labelJa: 'ステータスコード', labelEn: 'Status Code' },
  size: { mul: 1, unit: 'B', labelJa: 'サイズ', labelEn: 'Size' },
  bytes: { mul: 1, unit: 'B', labelJa: 'バイト数', labelEn: 'Bytes' },
  cpu: { mul: 1, unit: '%', labelJa: 'CPU', labelEn: 'CPU' },
  mem: { mul: 1, unit: '%', labelJa: 'メモリ', labelEn: 'Memory' },
  load: { mul: 1, unit: '', labelJa: 'ロード', labelEn: 'Load' },
  temp: { mul: 1, unit: '°C', labelJa: '温度', labelEn: 'Temp' },
  score: { mul: 1, unit: '', labelJa: 'スコア', labelEn: 'Score' },
};

const SKIP_KEYS = new Set([
  'lastTime',
  'last_time',
  'state',
  'error',
  'loc',
  'info',
  'msg',
]);

export function getMetricMeta(key: string, isJa = false): { mul: number; unit: string; label: string } {
  const meta = METRIC_PARAMS[key];
  if (meta) {
    return {
      mul: meta.mul,
      unit: meta.unit,
      label: `${isJa ? meta.labelJa : meta.labelEn}${meta.unit ? ` (${meta.unit})` : ''}`,
    };
  }
  return { mul: 1, unit: '', label: key };
}

export function getMetricMul(key: string): number {
  return METRIC_PARAMS[key]?.mul ?? 1;
}

export function extractPollingMetricKeys(logs: PollingLogEnt[]): string[] {
  const keySet = new Set<string>();
  for (const l of logs || []) {
    if (!l?.Result || typeof l.Result !== 'object') continue;
    for (const [k, v] of Object.entries(l.Result)) {
      if (SKIP_KEYS.has(k) || k.startsWith('_')) continue;
      if (typeof v === 'number' && Number.isFinite(v)) {
        keySet.add(k);
      }
    }
  }
  const priority = ['rtt', 'offset', 'loss', 'val', 'value', 'count', 'total', 'rate', 'speed', 'bps', 'pps'];
  return Array.from(keySet).sort((a, b) => {
    const ia = priority.indexOf(a);
    const ib = priority.indexOf(b);
    if (ia >= 0 && ib >= 0) return ia - ib;
    if (ia >= 0) return -1;
    if (ib >= 0) return 1;
    return a.localeCompare(b);
  });
}

// Pure TS statistics helpers
export function mean(vals: number[]): number {
  if (vals.length === 0) return 0;
  let s = 0;
  for (const v of vals) s += v;
  return s / vals.length;
}

export function median(vals: number[]): number {
  if (vals.length === 0) return 0;
  const sorted = [...vals].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 !== 0 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

export function sampleVariance(vals: number[]): number {
  if (vals.length < 2) return 0;
  const m = mean(vals);
  let ss = 0;
  for (const v of vals) {
    const d = v - m;
    ss += d * d;
  }
  return ss / (vals.length - 1);
}

export function stddev(vals: number[]): number {
  return Math.sqrt(sampleVariance(vals));
}

// Acklam inverse standard normal CDF approximation
export function probit(p: number): number {
  if (p <= 0) return -Infinity;
  if (p >= 1) return Infinity;
  if (p === 0.5) return 0;

  const a = [
    -3.969683028665376e1,
    2.209460984245205e2,
    -2.759285104469687e2,
    1.38357751867269e2,
    -3.066479806614716e1,
    2.506628277459239,
  ];
  const b = [
    -5.447609879822406e1,
    1.615858368580409e2,
    -1.556989798598866e2,
    6.680131188771972e1,
    -1.328068155288572e1,
  ];
  const c = [
    -7.784894002430293e-3,
    -3.223964580411365e-1,
    -2.400758277161838,
    -2.549732539343734,
    4.374664141464968,
    2.938163982698783,
  ];
  const d = [
    7.784695709041462e-3,
    3.224671290700398e-1,
    2.445134137142996,
    3.754408661907416,
  ];

  const pLow = 0.02425;
  const pHigh = 1 - pLow;

  let q: number;
  let r: number;
  if (p < pLow) {
    q = Math.sqrt(-2 * Math.log(p));
    return (
      (((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) /
      ((((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1)
    );
  }
  if (p <= pHigh) {
    q = p - 0.5;
    r = q * q;
    return (
      ((((((a[0] * r + a[1]) * r + a[2]) * r + a[3]) * r + a[4]) * r + a[5]) * q) /
      (((((b[0] * r + b[1]) * r + b[2]) * r + b[3]) * r + b[4]) * r + 1)
    );
  }
  q = Math.sqrt(-2 * Math.log(1 - p));
  return -(
    (((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) /
    ((((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1)
  );
}

export function linearRegression(points: [number, number][]): { slope: number; intercept: number } {
  const n = points.length;
  if (n === 0) return { slope: 0, intercept: 0 };
  if (n === 1) return { slope: 0, intercept: points[0][1] };

  let sumX = 0;
  let sumY = 0;
  let sumXY = 0;
  let sumXX = 0;
  for (const [x, y] of points) {
    sumX += x;
    sumY += y;
    sumXY += x * y;
    sumXX += x * x;
  }
  const meanX = sumX / n;
  const meanY = sumY / n;
  const denom = sumXX - sumX * meanX;
  if (Math.abs(denom) < 1e-12) {
    return { slope: 0, intercept: meanY };
  }
  const slope = (sumXY - sumX * meanY) / denom;
  const intercept = meanY - slope * meanX;
  return { slope, intercept };
}

export interface HistogramBin {
  label: string;
  min: number;
  max: number;
  count: number;
}

export interface NumericStatsSummary {
  count: number;
  mean: number;
  median: number;
  min: number;
  max: number;
  variance: number;
  stddev: number;
}

export function computeNumericStats(logs: PollingLogEnt[], key: string): {
  values: number[];
  stats: NumericStatsSummary;
} {
  const mul = getMetricMul(key);
  const values: number[] = [];
  for (const l of logs || []) {
    const raw = l?.Result?.[key];
    if (typeof raw === 'number' && Number.isFinite(raw)) {
      values.push(raw * mul);
    }
  }
  if (values.length === 0) {
    return {
      values: [],
      stats: { count: 0, mean: 0, median: 0, min: 0, max: 0, variance: 0, stddev: 0 },
    };
  }
  let minV = values[0];
  let maxV = values[0];
  for (const v of values) {
    if (v < minV) minV = v;
    if (v > maxV) maxV = v;
  }
  const vVar = sampleVariance(values);
  return {
    values,
    stats: {
      count: values.length,
      mean: mean(values),
      median: median(values),
      min: minV,
      max: maxV,
      variance: vVar,
      stddev: Math.sqrt(vVar),
    },
  };
}

export function computeHistogram(values: number[], requestedBins?: number): HistogramBin[] {
  if (values.length === 0) return [];
  let minV = values[0];
  let maxV = values[0];
  for (const v of values) {
    if (v < minV) minV = v;
    if (v > maxV) maxV = v;
  }
  if (maxV - minV <= 1e-12) {
    return [{ label: minV.toFixed(2), min: minV, max: maxV, count: values.length }];
  }
  const k =
    requestedBins && requestedBins > 0
      ? requestedBins
      : Math.min(24, Math.max(5, Math.ceil(Math.log2(values.length)) + 1));
  const width = (maxV - minV) / k;
  const counts = new Array<number>(k).fill(0);
  for (const v of values) {
    let idx = Math.floor((v - minV) / width);
    if (idx >= k) idx = k - 1;
    if (idx < 0) idx = 0;
    counts[idx]++;
  }
  const bins: HistogramBin[] = [];
  for (let i = 0; i < k; i++) {
    const bMin = minV + i * width;
    const bMax = bMin + width;
    const digits = width < 1 ? 2 : width < 10 ? 1 : 0;
    bins.push({
      label: `${bMin.toFixed(digits)}-${bMax.toFixed(digits)}`,
      min: bMin,
      max: bMax,
      count: counts[i],
    });
  }
  return bins;
}

export interface DowntimeEvent {
  startNs: number;
  endNs: number;
  durationSec: number;
  state: string;
}

export interface SinglePollingDowntimeResult {
  slaPercent: number;
  totalSpanSec: number;
  totalDowntimeSec: number;
  totalUptimeSec: number;
  mtbfSec: number;
  mttrSec: number;
  outageCount: number;
  normalCount: number;
  warnCount: number;
  lowCount: number;
  highCount: number;
  events: DowntimeEvent[];
}

function isOkState(state: string): boolean {
  const s = (state || '').toLowerCase();
  return s === 'normal' || s === 'repair';
}

export function calcSinglePollingDowntimeAndSLA(
  logs: PollingLogEnt[],
  pollIntSec = 60
): SinglePollingDowntimeResult {
  const sorted = [...(logs || [])].sort((a, b) => a.Time - b.Time);
  const stepSec = Math.max(pollIntSec || 60, 1);

  let normalCount = 0;
  let warnCount = 0;
  let lowCount = 0;
  let highCount = 0;

  for (const l of sorted) {
    const st = (l.State || '').toLowerCase();
    if (st === 'normal' || st === 'repair') normalCount++;
    else if (st === 'warn') warnCount++;
    else if (st === 'low') lowCount++;
    else if (st === 'high') highCount++;
  }

  if (sorted.length === 0) {
    return {
      slaPercent: 100,
      totalSpanSec: 0,
      totalDowntimeSec: 0,
      totalUptimeSec: 0,
      mtbfSec: 0,
      mttrSec: 0,
      outageCount: 0,
      normalCount,
      warnCount,
      lowCount,
      highCount,
      events: [],
    };
  }

  const events: DowntimeEvent[] = [];
  let totalSpanSec = 0;
  let totalDowntimeSec = 0;

  let activeDownStartNs = 0;
  let activeDownState = '';

  for (let i = 0; i < sorted.length; i++) {
    const cur = sorted[i];
    const ok = isOkState(cur.State);

    if (!ok) {
      if (activeDownStartNs === 0) {
        activeDownStartNs = cur.Time;
        activeDownState = (cur.State || 'high').toLowerCase();
      } else if ((cur.State || '').toLowerCase() === 'high') {
        activeDownState = 'high';
      }
    } else if (activeDownStartNs > 0) {
      const dur = Math.max((cur.Time - activeDownStartNs) / 1e9, stepSec);
      events.push({
        startNs: activeDownStartNs,
        endNs: cur.Time,
        durationSec: dur,
        state: activeDownState,
      });
      activeDownStartNs = 0;
      activeDownState = '';
    }

    if (i < sorted.length - 1) {
      const dtSec = Math.max((sorted[i + 1].Time - cur.Time) / 1e9, 0);
      totalSpanSec += dtSec;
      if (!ok) {
        totalDowntimeSec += dtSec;
      }
    }
  }

  if (sorted.length === 1) {
    totalSpanSec = stepSec;
    if (!isOkState(sorted[0].State)) {
      totalDowntimeSec = stepSec;
    }
  } else {
    // Include trailing interval for the last sample
    totalSpanSec += stepSec;
    if (!isOkState(sorted[sorted.length - 1].State)) {
      totalDowntimeSec += stepSec;
    }
  }

  if (activeDownStartNs > 0) {
    const lastNs = sorted[sorted.length - 1].Time;
    const endNs = lastNs + stepSec * 1e9;
    const dur = Math.max((endNs - activeDownStartNs) / 1e9, stepSec);
    events.push({
      startNs: activeDownStartNs,
      endNs,
      durationSec: dur,
      state: activeDownState,
    });
  }

  const totalUptimeSec = Math.max(totalSpanSec - totalDowntimeSec, 0);
  const slaPercent = totalSpanSec > 0 ? (totalUptimeSec / totalSpanSec) * 100 : 100;
  const outageCount = events.length;
  const mttrSec = outageCount > 0 ? totalDowntimeSec / outageCount : 0;
  const mtbfSec = outageCount > 0 ? totalUptimeSec / outageCount : totalUptimeSec;

  return {
    slaPercent,
    totalSpanSec,
    totalDowntimeSec,
    totalUptimeSec,
    mtbfSec,
    mttrSec,
    outageCount,
    normalCount,
    warnCount,
    lowCount,
    highCount,
    events,
  };
}

function initChart(dom: HTMLElement | string): {
  chart: echarts.ECharts;
  dark: boolean;
  textColor: string;
  lineColor: string;
} | null {
  const el = typeof dom === 'string' ? document.getElementById(dom) : dom;
  if (!el) return null;
  const existing = echarts.getInstanceByDom(el);
  if (existing) existing.dispose();
  const dark = isDarkMode();
  const chart = echarts.init(el, dark ? 'dark' : undefined);
  return {
    chart,
    dark,
    textColor: dark ? '#cbd5e1' : '#475569',
    lineColor: dark ? '#334155' : '#e2e8f0',
  };
}

export function showSinglePollingDowntimeChart(
  dom: HTMLElement | string,
  dt: SinglePollingDowntimeResult,
  labels: { durationSec: string; time: string; noDowntime: string }
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;

  const colorMap: Record<string, string> = {
    high: '#ef4444',
    low: '#f97316',
    warn: '#f59e0b',
  };

  const seriesData = dt.events.map((ev) => ({
    value: [new Date(ev.startNs / 1e6), Number(ev.durationSec.toFixed(1))],
    itemStyle: { color: colorMap[ev.state] || '#ef4444' },
    state: ev.state,
    endMs: ev.endNs / 1e6,
  }));

  chart.setOption({
    backgroundColor: 'transparent',
    title:
      dt.events.length === 0
        ? {
            text: labels.noDowntime,
            left: 'center',
            top: 'middle',
            textStyle: { color: '#10b981', fontSize: 14, fontWeight: 600 },
          }
        : { show: false },
    grid: { left: '7%', right: '5%', top: 28, bottom: 60 },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 12, height: 18 }],
    tooltip: {
      trigger: 'axis',
      formatter(params: any) {
        const p = Array.isArray(params) ? params[0] : params;
        if (!p?.data) return '';
        const startStr = echarts.time.format(p.data.value[0], '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false);
        const endStr = echarts.time.format(new Date(p.data.endMs), '{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}', false);
        return `${startStr} - ${endStr}<br/>${p.data.state.toUpperCase()}: <b>${p.data.value[1]} s</b>`;
      },
    },
    xAxis: {
      type: 'time',
      name: labels.time,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis: {
      type: 'value',
      name: labels.durationSec,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      {
        name: labels.durationSec,
        type: 'bar',
        barMaxWidth: 28,
        data: seriesData,
      },
    ],
  });
  return chart;
}

export function showPollingLogStateChart(
  dom: HTMLElement | string,
  logs: PollingLogEnt[],
  labels: { normal: string; warn: string; low: string; high: string; count: string },
  zoomCallback?: (st: number, et: number) => void
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;

  const sorted = [...(logs || [])].sort((a, b) => a.Time - b.Time);
  if (sorted.length === 0) {
    chart.setOption({ backgroundColor: 'transparent', title: { show: false }, xAxis: { type: 'time' }, yAxis: { type: 'value' }, series: [] });
    return chart;
  }

  const stNs = sorted[0].Time;
  const ltNs = sorted[sorted.length - 1].Time;
  const spanSec = Math.max((ltNs - stNs) / 1e9, 60);

  // Adaptive bucket size keeps buckets bounded (<= ~400 points even over multi-month "All Time")
  let bucketSec = 60;
  if (spanSec > 90 * 86400) bucketSec = 86400;
  else if (spanSec > 14 * 86400) bucketSec = 21600;
  else if (spanSec > 48 * 3600) bucketSec = 3600;
  else if (spanSec > 6 * 3600) bucketSec = 600;

  const bucketNs = bucketSec * 1e9;
  const buckets = new Map<number, { normal: number; warn: number; low: number; high: number }>();

  for (const l of sorted) {
    const key = Math.floor(l.Time / bucketNs) * bucketNs;
    let b = buckets.get(key);
    if (!b) {
      b = { normal: 0, warn: 0, low: 0, high: 0 };
      buckets.set(key, b);
    }
    const s = (l.State || '').toLowerCase();
    if (s === 'high') b.high++;
    else if (s === 'low') b.low++;
    else if (s === 'warn') b.warn++;
    else b.normal++;
  }

  const keys = Array.from(buckets.keys()).sort((a, b) => a - b);
  const normalData: [Date, number][] = [];
  const warnData: [Date, number][] = [];
  const lowData: [Date, number][] = [];
  const highData: [Date, number][] = [];

  for (const k of keys) {
    const b = buckets.get(k)!;
    const t = new Date(k / 1e6);
    normalData.push([t, b.normal]);
    warnData.push([t, b.warn]);
    lowData.push([t, b.low]);
    highData.push([t, b.high]);
  }

  chart.setOption({
    backgroundColor: 'transparent',
    color: ['#10b981', '#f59e0b', '#f97316', '#ef4444'],
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
    },
    legend: {
      top: 4,
      textStyle: { color: textColor, fontSize: 11 },
      data: [labels.normal, labels.warn, labels.low, labels.high],
    },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 10, height: 16 }],
    grid: { left: '6%', right: '4%', top: 36, bottom: 52 },
    xAxis: {
      type: 'time',
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis: {
      type: 'value',
      name: labels.count,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      { name: labels.normal, type: 'bar', stack: 'state', data: normalData },
      { name: labels.warn, type: 'bar', stack: 'state', data: warnData },
      { name: labels.low, type: 'bar', stack: 'state', data: lowData },
      { name: labels.high, type: 'bar', stack: 'state', data: highData },
    ],
  });

  if (zoomCallback) {
    setZoomCallback(chart, zoomCallback, stNs, ltNs);
  }
  return chart;
}

export function showPollingTimeChart(
  dom: HTMLElement | string,
  logs: PollingLogEnt[],
  key: string,
  mode: 'raw' | 'hourly',
  showForecast: boolean,
  labels: { mean: string; max: string; min: string; median: string; variance: string; forecast: string },
  isJa = false
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;

  const { mul, label: axisLabel } = getMetricMeta(key, isJa);
  const sorted = [...(logs || [])].sort((a, b) => a.Time - b.Time);
  const points: [number, number][] = [];
  for (const l of sorted) {
    const v = l?.Result?.[key];
    if (typeof v === 'number' && Number.isFinite(v)) {
      points.push([l.Time / 1e6, v * mul]);
    }
  }

  const series: any[] = [];
  const legendData: string[] = [];
  let yAxes: any = {
    type: 'value',
    name: axisLabel,
    nameTextStyle: { color: textColor, fontSize: 11 },
    axisLabel: { color: textColor, fontSize: 10 },
    axisLine: { lineStyle: { color: lineColor } },
    splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
  };

  if (mode === 'raw') {
    legendData.push(axisLabel);
    series.push({
      name: axisLabel,
      type: 'line',
      showSymbol: points.length <= 120,
      symbolSize: 4,
      smooth: false,
      color: '#38bdf8',
      areaStyle: { color: 'rgba(56, 189, 248, 0.12)' },
      data: points.map(([tMs, val]) => [new Date(tMs), Number(val.toFixed(4))]),
    });

    if (showForecast && points.length >= 2) {
      const t0 = points[0][0];
      const tEnd = points[points.length - 1][0];
      const reg = linearRegression(points.map(([tMs, v]) => [(tMs - t0) / 3600000, v]));
      const spanMs = Math.max(tEnd - t0, 3600000);
      const extMs = Math.min(spanMs * 0.25, 24 * 3600000);
      const fPoints: [Date, number][] = [
        [new Date(t0), Number(reg.intercept.toFixed(4))],
        [new Date(tEnd), Number((reg.intercept + reg.slope * ((tEnd - t0) / 3600000)).toFixed(4))],
        [new Date(tEnd + extMs), Number((reg.intercept + reg.slope * ((tEnd + extMs - t0) / 3600000)).toFixed(4))],
      ];
      legendData.push(labels.forecast);
      series.push({
        name: labels.forecast,
        type: 'line',
        symbol: 'none',
        lineStyle: { color: '#f43f5e', type: 'dashed', width: 2 },
        color: '#f43f5e',
        data: fPoints,
      });
    }
  } else {
    // Hourly aggregation WITHOUT zero-padding empty buckets
    const hourBuckets = new Map<number, number[]>();
    for (const [tMs, val] of points) {
      const hMs = Math.floor(tMs / 3600000) * 3600000;
      let arr = hourBuckets.get(hMs);
      if (!arr) {
        arr = [];
        hourBuckets.set(hMs, arr);
      }
      arr.push(val);
    }

    const hourKeys = Array.from(hourBuckets.keys()).sort((a, b) => a - b);
    const meanData: [Date, number][] = [];
    const maxData: [Date, number][] = [];
    const minData: [Date, number][] = [];
    const medianData: [Date, number][] = [];
    const varData: [Date, number][] = [];
    const meanPts: [number, number][] = [];

    for (const hMs of hourKeys) {
      const arr = hourBuckets.get(hMs)!;
      const d = new Date(hMs);
      const mVal = mean(arr);
      meanData.push([d, Number(mVal.toFixed(4))]);
      meanPts.push([hMs, mVal]);
      maxData.push([d, Number(Math.max(...arr).toFixed(4))]);
      minData.push([d, Number(Math.min(...arr).toFixed(4))]);
      medianData.push([d, Number(median(arr).toFixed(4))]);
      varData.push([d, Number(sampleVariance(arr).toFixed(4))]);
    }

    legendData.push(labels.mean, labels.max, labels.min, labels.median, labels.variance);
    yAxes = [
      {
        type: 'value',
        name: axisLabel,
        nameTextStyle: { color: textColor, fontSize: 11 },
        axisLabel: { color: textColor, fontSize: 10 },
        axisLine: { lineStyle: { color: lineColor } },
        splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
      },
      {
        type: 'value',
        name: labels.variance,
        nameTextStyle: { color: textColor, fontSize: 11 },
        axisLabel: { color: textColor, fontSize: 10 },
        axisLine: { lineStyle: { color: lineColor } },
        splitLine: { show: false },
      },
    ];

    series.push(
      { name: labels.mean, type: 'line', smooth: false, color: '#38bdf8', showSymbol: false, data: meanData },
      { name: labels.max, type: 'line', smooth: false, color: '#f97316', showSymbol: false, lineStyle: { type: 'dashed', width: 1.5 }, data: maxData },
      { name: labels.min, type: 'line', smooth: false, color: '#10b981', showSymbol: false, lineStyle: { type: 'dashed', width: 1.5 }, data: minData },
      { name: labels.median, type: 'line', smooth: false, color: '#a855f7', showSymbol: false, lineStyle: { type: 'dotted', width: 1.5 }, data: medianData },
      { name: labels.variance, type: 'line', yAxisIndex: 1, smooth: false, color: '#eab308', showSymbol: false, lineStyle: { width: 1.2, opacity: 0.8 }, data: varData }
    );

    if (showForecast && meanPts.length >= 2) {
      const t0 = meanPts[0][0];
      const tEnd = meanPts[meanPts.length - 1][0];
      const reg = linearRegression(meanPts.map(([tMs, v]) => [(tMs - t0) / 3600000, v]));
      const spanMs = Math.max(tEnd - t0, 3600000);
      const extMs = Math.min(spanMs * 0.25, 24 * 3600000);
      legendData.push(labels.forecast);
      series.push({
        name: labels.forecast,
        type: 'line',
        symbol: 'none',
        color: '#f43f5e',
        lineStyle: { color: '#f43f5e', type: 'dashed', width: 2 },
        data: [
          [new Date(t0), Number(reg.intercept.toFixed(4))],
          [new Date(tEnd), Number((reg.intercept + reg.slope * ((tEnd - t0) / 3600000)).toFixed(4))],
          [new Date(tEnd + extMs), Number((reg.intercept + reg.slope * ((tEnd + extMs - t0) / 3600000)).toFixed(4))],
        ],
      });
    }
  }

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: {
      top: 4,
      textStyle: { color: textColor, fontSize: 11 },
      data: legendData,
    },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 10, height: 16 }],
    grid: { left: '7%', right: mode === 'hourly' ? '8%' : '5%', top: 40, bottom: 54 },
    xAxis: {
      type: 'time',
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
    },
    yAxis: yAxes,
    series,
  });

  return chart;
}

export function showPollingHistogram(
  dom: HTMLElement | string,
  logs: PollingLogEnt[],
  key: string,
  labels: { count: string; mean: string },
  isJa = false
): { chart?: echarts.ECharts; stats: NumericStatsSummary } {
  const { values, stats } = computeNumericStats(logs, key);
  const ctx = initChart(dom);
  if (!ctx) return { stats };
  const { chart, textColor, lineColor } = ctx;
  const { label: metricLabel } = getMetricMeta(key, isJa);
  const bins = computeHistogram(values);

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
    },
    grid: { left: '7%', right: '5%', top: 32, bottom: 60 },
    xAxis: {
      type: 'category',
      name: metricLabel,
      nameLocation: 'middle',
      nameGap: 36,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10, rotate: bins.length > 10 ? 25 : 0 },
      axisLine: { lineStyle: { color: lineColor } },
      data: bins.map((b) => b.label),
    },
    yAxis: {
      type: 'value',
      name: labels.count,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      {
        name: labels.count,
        type: 'bar',
        color: '#06b6d4',
        barWidth: '82%',
        data: bins.map((b) => b.count),
      },
    ],
  });

  return { chart, stats };
}

export function showPollingQQPlot(
  dom: HTMLElement | string,
  logs: PollingLogEnt[],
  key: string,
  labels: { theoreticalQuantile: string; sampleQuantile: string; referenceLine: string },
  isJa = false
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;
  const { values, stats } = computeNumericStats(logs, key);
  const { label: metricLabel } = getMetricMeta(key, isJa);

  if (values.length === 0) {
    chart.setOption({ backgroundColor: 'transparent', xAxis: { type: 'value' }, yAxis: { type: 'value' }, series: [] });
    return chart;
  }

  const sorted = [...values].sort((a, b) => a - b);
  const n = sorted.length;
  const step = n > 1000 ? Math.ceil(n / 1000) : 1;
  const scatterData: [number, number][] = [];

  for (let i = 0; i < n; i += step) {
    const p = (i + 0.5) / n;
    const z = probit(p);
    scatterData.push([Number(z.toFixed(4)), Number(sorted[i].toFixed(4))]);
  }
  if ((n - 1) % step !== 0 && n > 1) {
    const p = (n - 0.5) / n;
    scatterData.push([Number(probit(p).toFixed(4)), Number(sorted[n - 1].toFixed(4))]);
  }

  const zMin = scatterData[0][0];
  const zMax = scatterData[scatterData.length - 1][0];
  const refLine: [number, number][] = [
    [zMin, Number((stats.mean + stats.stddev * zMin).toFixed(4))],
    [zMax, Number((stats.mean + stats.stddev * zMax).toFixed(4))],
  ];

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      formatter(params: any) {
        if (!Array.isArray(params?.data)) return '';
        return `${labels.theoreticalQuantile}: ${params.data[0]}<br/>${metricLabel}: ${params.data[1]}`;
      },
    },
    legend: {
      top: 4,
      textStyle: { color: textColor, fontSize: 11 },
      data: [metricLabel, labels.referenceLine],
    },
    grid: { left: '8%', right: '5%', top: 36, bottom: 52 },
    xAxis: {
      type: 'value',
      name: labels.theoreticalQuantile,
      nameLocation: 'middle',
      nameGap: 30,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.25 } },
    },
    yAxis: {
      type: 'value',
      name: `${labels.sampleQuantile} (${metricLabel})`,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      {
        name: metricLabel,
        type: 'scatter',
        symbolSize: 5,
        color: '#38bdf8',
        data: scatterData,
      },
      {
        name: labels.referenceLine,
        type: 'line',
        symbol: 'none',
        color: '#f43f5e',
        lineStyle: { color: '#f43f5e', type: 'dashed', width: 2 },
        data: refLine,
      },
    ],
  });
  return chart;
}

export function showPollingSTLChart(
  dom: HTMLElement | string,
  analyze: PollingTimeAnalyzeResp,
  key: string,
  resolution: 'H' | 'PX2',
  labels: { observed: string; trend: string; seasonal: string; residual: string },
  isJa = false
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;

  const times = resolution === 'H' ? analyze.TimeH : analyze.TimePX2;
  const stl = resolution === 'H' ? analyze.StlMapH?.[key] : analyze.StlMapPX2?.[key];
  const { mul, unit } = getMetricMeta(key, isJa);

  if (!stl || !times || times.length === 0) {
    chart.setOption({ backgroundColor: 'transparent', series: [] });
    return chart;
  }

  const dates = times.map((sec) => echarts.time.format(new Date(sec * 1000), '{MM}/{dd} {HH}:{mm}', false));
  const observed = times.map((_, i) =>
    Number((((stl.Trend[i] ?? 0) + (stl.Seasonal[i] ?? 0) + (stl.Resid[i] ?? 0)) * mul).toFixed(4))
  );
  const trend = stl.Trend.map((v) => Number((v * mul).toFixed(4)));
  const seasonal = stl.Seasonal.map((v) => Number((v * mul).toFixed(4)));
  const resid = stl.Resid.map((v) => Number((v * mul).toFixed(4)));

  const unitSuffix = unit ? ` (${unit})` : '';
  const panels = [
    { title: `${labels.observed}${unitSuffix}`, data: observed, color: '#38bdf8', type: 'line' as const },
    { title: `${labels.trend}${unitSuffix}`, data: trend, color: '#10b981', type: 'line' as const },
    { title: `${labels.seasonal}${unitSuffix}`, data: seasonal, color: '#a855f7', type: 'line' as const },
    { title: `${labels.residual}${unitSuffix}`, data: resid, color: '#f97316', type: 'bar' as const },
  ];

  const grids = [
    { left: '8%', right: '4%', top: '6%', height: '16%' },
    { left: '8%', right: '4%', top: '28%', height: '16%' },
    { left: '8%', right: '4%', top: '50%', height: '16%' },
    { left: '8%', right: '4%', top: '72%', height: '16%' },
  ];

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', axisPointer: { type: 'line' } },
    axisPointer: { link: [{ xAxisIndex: 'all' }] },
    dataZoom: [{ type: 'slider', xAxisIndex: [0, 1, 2, 3], bottom: 6, height: 16 }],
    grid: grids,
    xAxis: panels.map((_, idx) => ({
      type: 'category',
      gridIndex: idx,
      data: dates,
      axisLabel: { show: idx === 3, color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
    })),
    yAxis: panels.map((p, idx) => ({
      type: 'value',
      gridIndex: idx,
      name: p.title,
      nameGap: 6,
      nameTextStyle: { color: textColor, fontSize: 10, align: 'left' },
      axisLabel: { color: textColor, fontSize: 9 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.25 } },
    })),
    series: panels.map((p, idx) => ({
      name: p.title,
      type: p.type,
      xAxisIndex: idx,
      yAxisIndex: idx,
      showSymbol: false,
      color: p.color,
      data: p.data,
    })),
  });

  return chart;
}

function formatPeriodFromHz(freqHz: number): string {
  if (!freqHz || freqHz <= 0) return '-';
  const sec = 1 / freqHz;
  if (sec >= 86400) return `${(sec / 86400).toFixed(2)} d`;
  if (sec >= 3600) return `${(sec / 3600).toFixed(2)} h`;
  if (sec >= 60) return `${(sec / 60).toFixed(1)} min`;
  return `${sec.toFixed(1)} s`;
}

export function showPollingFFTChart(
  dom: HTMLElement | string,
  analyze: PollingTimeAnalyzeResp,
  key: string,
  resolution: 'H' | 'PX2',
  labels: { frequency: string; amplitude: string; period: string },
  isJa = false
): echarts.ECharts | undefined {
  const ctx = initChart(dom);
  if (!ctx) return undefined;
  const { chart, textColor, lineColor } = ctx;

  const rawPts = resolution === 'H' ? analyze.FFTH?.[key] : analyze.FFTPX2?.[key];
  const { mul, unit } = getMetricMeta(key, isJa);

  if (!rawPts || rawPts.length === 0) {
    chart.setOption({ backgroundColor: 'transparent', series: [] });
    return chart;
  }

  const pts = rawPts.map(([freq, amp]) => ({
    freq,
    amp: Number((amp * mul).toFixed(5)),
    period: formatPeriodFromHz(freq),
  }));

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      formatter(params: any) {
        const p = Array.isArray(params) ? params[0] : params;
        if (!p) return '';
        const item = pts[p.dataIndex];
        if (!item) return '';
        return `${labels.frequency}: ${item.freq.toExponential(3)} Hz<br/>${labels.period}: <b>${item.period}</b><br/>${labels.amplitude}: <b>${item.amp}${unit ? ' ' + unit : ''}</b>`;
      },
    },
    toolbox: {
      iconStyle: { color: textColor },
      feature: { dataZoom: {} },
    },
    dataZoom: [{ bottom: 10, height: 16 }],
    grid: { left: '8%', right: '5%', top: 32, bottom: 60 },
    xAxis: {
      type: 'category',
      name: `${labels.frequency} (Hz)`,
      nameLocation: 'middle',
      nameGap: 32,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      data: pts.map((p) => p.freq.toExponential(2)),
    },
    yAxis: {
      type: 'value',
      name: `${labels.amplitude}${unit ? ` (${unit})` : ''}`,
      nameTextStyle: { color: textColor, fontSize: 11 },
      axisLabel: { color: textColor, fontSize: 10 },
      axisLine: { lineStyle: { color: lineColor } },
      splitLine: { lineStyle: { color: lineColor, opacity: 0.3 } },
    },
    series: [
      {
        name: labels.amplitude,
        type: 'bar',
        color: '#8b5cf6',
        data: pts.map((p) => p.amp),
      },
    ],
  });

  return chart;
}
