import * as echarts from "echarts";

let chart: any;

export const getPingChartOption = () => {
  return {
    title: { show: false },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
    },
    grid: {
      left: "5%",
      right: "5%",
      top: 40,
      bottom: 50,
    },
    legend: {
      top: 10,
      data: ["応答時間 (秒)", "送信TTL", "受信TTL"],
      textStyle: { color: "#94a3b8", fontSize: 11 },
    },
    xAxis: {
      type: "time",
      axisLabel: {
        color: "#94a3b8",
        fontSize: 9,
        formatter(value: any) {
          const date = new Date(value);
          return echarts.time.format(date, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}", false);
        },
      },
      axisLine: { lineStyle: { color: "#475569" } },
      splitLine: { show: false },
    },
    yAxis: [
      {
        type: "value",
        name: "応答時間 (秒)",
        nameTextStyle: { color: "#94a3b8", fontSize: 10 },
        axisLabel: { color: "#94a3b8", fontSize: 9 },
        axisLine: { lineStyle: { color: "#475569" } },
        splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.08)" } },
      },
      {
        type: "value",
        name: "TTL",
        nameTextStyle: { color: "#94a3b8", fontSize: 10 },
        axisLabel: { color: "#94a3b8", fontSize: 9 },
        axisLine: { lineStyle: { color: "#475569" } },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: "応答時間 (秒)",
        color: "#06b6d4",
        type: "line",
        showSymbol: true,
        symbolSize: 4,
        data: [] as any[],
      },
      {
        name: "送信TTL",
        color: "#eab308",
        type: "line",
        showSymbol: false,
        yAxisIndex: 1,
        data: [] as any[],
      },
      {
        name: "受信TTL",
        color: "#f43f5e",
        type: "line",
        showSymbol: false,
        yAxisIndex: 1,
        data: [] as any[],
      },
    ],
  };
};

export const showPingHistogram = (divId: string, results: any[]) => {
  const container = document.getElementById(divId);
  if (!container || !results || results.length === 0) return null;
  chart = echarts.init(container, "dark");

  const rtts = results
    .filter((r) => r.Stat === 1)
    .map((r) => Number((r.Time / 1e6).toFixed(2)));

  if (rtts.length === 0) return null;

  const min = Math.min(...rtts);
  const max = Math.max(...rtts);
  const binCount = Math.min(15, Math.max(5, Math.ceil(Math.sqrt(rtts.length))));
  const step = (max - min) / binCount || 1;
  const bins = new Array(binCount).fill(0);
  const labels: string[] = [];

  for (let i = 0; i < binCount; i++) {
    const start = min + i * step;
    const end = start + step;
    labels.push(`${start.toFixed(1)}-${end.toFixed(1)}ms`);
  }

  for (const v of rtts) {
    let idx = Math.floor((v - min) / step);
    if (idx >= binCount) idx = binCount - 1;
    bins[idx]++;
  }

  const option = {
    title: { show: false },
    tooltip: { trigger: "axis" },
    grid: { left: "8%", right: "8%", top: 30, bottom: 45 },
    xAxis: {
      type: "category",
      data: labels,
      axisLabel: { color: "#94a3b8", fontSize: 9, rotate: 30 },
      axisLine: { lineStyle: { color: "#475569" } },
    },
    yAxis: {
      type: "value",
      name: "回数",
      axisLabel: { color: "#94a3b8", fontSize: 9 },
      splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.08)" } },
    },
    series: [
      {
        name: "回数",
        type: "bar",
        color: "#06b6d4",
        barWidth: "60%",
        data: bins,
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
};

export const showPingSmokeChart = (divId: string, results: any[]) => {
  const container = document.getElementById(divId);
  if (!container || !results || results.length === 0) return null;
  chart = echarts.init(container, "dark");

  const data: [number, number, number][] = [];
  results.forEach((r, idx) => {
    if (r.Stat === 1) {
      data.push([idx, Number((r.Time / 1e6).toFixed(2)), r.Size]);
    }
  });

  const option = {
    title: { show: false },
    tooltip: { trigger: "item" },
    grid: { left: "8%", right: "8%", top: 30, bottom: 40 },
    xAxis: {
      type: "value",
      name: "シーケンス",
      axisLabel: { color: "#94a3b8", fontSize: 9 },
      axisLine: { lineStyle: { color: "#475569" } },
      splitLine: { show: false },
    },
    yAxis: {
      type: "value",
      name: "応答時間 (ms)",
      axisLabel: { color: "#94a3b8", fontSize: 9 },
      splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.08)" } },
    },
    series: [
      {
        type: "scatter",
        symbolSize: (val: any) => Math.max(6, Math.min(20, (val[2] || 64) / 64 * 6)),
        itemStyle: {
          color: (p: any) => {
            const v = p.value[1];
            if (v < 10) return "#10b981";
            if (v < 50) return "#38bdf8";
            if (v < 150) return "#f59e0b";
            return "#ef4444";
          },
        },
        data: data.map((d) => [d[0], d[1], d[2]]),
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
};
