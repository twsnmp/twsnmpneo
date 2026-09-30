import * as echarts from "echarts";

export interface MTRHopStat {
  ttl: number;
  ip: string;
  loc: string;
  snt: number;
  lossRate: number;
  last: number; // ms
  avg: number;  // ms
  best: number; // ms
  wrst: number; // ms
  stDev: number;// ms
  isTarget: boolean;
}

export const showMtrProfileChart = (
  divId: string,
  mtrList: MTRHopStat[],
  labels: {
    averageRtt: string;
    rttRange: string;
    lossRate: string;
    rtt: string;
    minimumRtt: string;
  },
) => {
  const container = document.getElementById(divId);
  if (!container || !mtrList || mtrList.length === 0) return null;

  const chart = echarts.init(container, "dark");

  const hops: string[] = [];
  const minData: (number | null)[] = [];
  const rangeData: (number | null)[] = [];
  const avgData: (number | null)[] = [];
  const lossData: any[] = [];

  mtrList.forEach((h) => {
    const label = `${h.ttl}: ${h.ip || "???"}`;
    hops.push(label);

    if (h.snt > 0 && h.avg >= 0) {
      const best = Number(h.best.toFixed(2));
      const avg = Number(h.avg.toFixed(2));
      const wrst = Number(h.wrst.toFixed(2));
      const range = Number(Math.max(0, wrst - best).toFixed(2));

      minData.push(best);
      rangeData.push(range);
      avgData.push(avg);
    } else {
      minData.push(null);
      rangeData.push(null);
      avgData.push(null);
    }

    const lossVal = Number(h.lossRate.toFixed(1));
    let color = "rgba(34, 197, 94, 0.4)";
    if (lossVal > 20) color = "#ef4444";
    else if (lossVal > 0) color = "#f59e0b";

    lossData.push({
      value: lossVal,
      itemStyle: { color },
    });
  });

  const option = {
    title: { show: false },
    tooltip: { trigger: "axis" },
    grid: { left: "6%", right: "6%", top: 30, bottom: 40 },
    legend: {
      top: 5,
      data: [labels.averageRtt, labels.rttRange, `${labels.lossRate} (%)`],
      textStyle: { color: "#94a3b8", fontSize: 10 },
    },
    xAxis: {
      type: "category",
      data: hops,
      axisLabel: { color: "#94a3b8", fontSize: 9, rotate: hops.length > 10 ? 25 : 0 },
      axisLine: { lineStyle: { color: "#475569" } },
    },
    yAxis: [
      {
        type: "value",
        name: labels.rtt,
        nameTextStyle: { color: "#94a3b8", fontSize: 10 },
        axisLabel: { color: "#94a3b8", fontSize: 9 },
        splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.08)" } },
      },
      {
        type: "value",
        name: `${labels.lossRate} (%)`,
        min: 0,
        max: 100,
        nameTextStyle: { color: "#94a3b8", fontSize: 10 },
        axisLabel: { color: "#94a3b8", fontSize: 9, formatter: "{value}%" },
        splitLine: { show: false },
      },
    ],
    series: [
      {
        name: labels.minimumRtt,
        type: "line",
        stack: "rttRange",
        symbol: "none",
        lineStyle: { opacity: 0 },
        data: minData,
      },
      {
        name: labels.rttRange,
        type: "line",
        stack: "rttRange",
        symbol: "none",
        color: "#38bdf8",
        lineStyle: { opacity: 0 },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: "rgba(56, 189, 248, 0.5)" },
            { offset: 1, color: "rgba(56, 189, 248, 0.1)" },
          ]),
        },
        data: rangeData,
      },
      {
        name: labels.averageRtt,
        type: "line",
        showSymbol: true,
        symbolSize: 6,
        color: "#00fea8",
        itemStyle: { color: "#00fea8" },
        lineStyle: { width: 2, color: "#00fea8" },
        data: avgData,
      },
      {
        name: `${labels.lossRate} (%)`,
        type: "bar",
        yAxisIndex: 1,
        barWidth: "30%",
        data: lossData,
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
};
