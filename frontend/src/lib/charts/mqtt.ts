import * as echarts from "echarts";
import { isDarkMode } from "./utils";
import { locale } from "svelte-i18n";
import { get } from "svelte/store";

const getIsJa = () => (get(locale) || "ja").startsWith("ja");

let currentChart: echarts.ECharts | undefined;

export const disposeChart = () => {
  if (currentChart) {
    currentChart.dispose();
    currentChart = undefined;
  }
};

// 1. クライアントID別 (By Client ID)
export const showMqttClientIDChart = (div: string, stats: any[]) => {
  disposeChart();
  const map = new Map<string, { count: number; bytes: number; topics: Set<string> }>();
  if (stats) {
    stats.forEach((s) => {
      const cid = s.ClientID || "(Unknown)";
      const item = map.get(cid) || { count: 0, bytes: 0, topics: new Set<string>() };
      item.count += s.Count || 0;
      item.bytes += s.Bytes || 0;
      if (s.Topic) item.topics.add(s.Topic);
      map.set(cid, item);
    });
  }

  const list = Array.from(map.entries()).map(([cid, val]) => ({
    cid,
    count: val.count,
    bytes: val.bytes,
    topicCount: val.topics.size,
  }));
  list.sort((a, b) => b.count - a.count);

  const categories: string[] = [];
  const counts: number[] = [];
  const bytesMB: number[] = [];
  const topList = list.slice(0, 30).reverse();

  topList.forEach((item) => {
    categories.push(item.cid);
    counts.push(item.count);
    bytesMB.push(Number((item.bytes / (1024 * 1024)).toFixed(2)));
  });

  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "クライアントID別受信統計" : "Received Stats by Client ID",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
    },
    legend: {
      top: 30,
      textStyle: { color: dark ? "#94a3b8" : "#64748b" },
      data: [isJa ? "受信回数" : "Received Count", isJa ? "データ量 (MB)" : "Data Size (MB)"],
    },
    grid: {
      left: "15%",
      right: "10%",
      top: 70,
      bottom: 40,
      containLabel: true,
    },
    xAxis: [
      {
        type: "value",
        name: isJa ? "受信回数" : "Count",
        axisLabel: { color: dark ? "#94a3b8" : "#64748b" },
        nameTextStyle: { color: dark ? "#94a3b8" : "#64748b" },
        splitLine: { lineStyle: { color: dark ? "#1e293b" : "#f1f5f9" } },
      },
      {
        type: "value",
        name: "MB",
        axisLabel: { color: dark ? "#94a3b8" : "#64748b" },
        nameTextStyle: { color: dark ? "#94a3b8" : "#64748b" },
        splitLine: { show: false },
      },
    ],
    yAxis: {
      type: "category",
      data: categories,
      axisLabel: { color: dark ? "#cbd5e1" : "#334155", fontSize: 11 },
      axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
    },
    series: [
      {
        name: isJa ? "受信回数" : "Received Count",
        type: "bar",
        data: counts,
        itemStyle: { color: "#06b6d4" },
      },
      {
        name: isJa ? "データ量 (MB)" : "Data Size (MB)",
        type: "bar",
        xAxisIndex: 1,
        data: bytesMB,
        itemStyle: { color: "#f43f5e" },
      },
    ],
  });
  return chart;
};

// 2. 送信元別 (By Remote IP)
export const showMqttRemoteChart = (div: string, stats: any[]) => {
  disposeChart();
  const map = new Map<string, { count: number; bytes: number }>();
  if (stats) {
    stats.forEach((s) => {
      const ip = s.Remote || "(Unknown)";
      const item = map.get(ip) || { count: 0, bytes: 0 };
      item.count += s.Count || 0;
      item.bytes += s.Bytes || 0;
      map.set(ip, item);
    });
  }

  const data = Array.from(map.entries()).map(([ip, val]) => ({
    name: ip,
    value: val.count,
    bytes: val.bytes,
  }));
  data.sort((a, b) => b.value - a.value);

  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "送信元IP別受信統計" : "Received Stats by Remote IP",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      trigger: "item",
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
      formatter: (params: any) => {
        const bytesMB = (params.data.bytes / (1024 * 1024)).toFixed(2);
        return `${params.name}<br/>${isJa ? "受信回数" : "Count"}: <strong>${params.value.toLocaleString()}</strong><br/>${isJa ? "データ量" : "Data Size"}: <strong>${bytesMB} MB</strong> (${params.percent}%)`;
      },
    },
    legend: {
      type: "scroll",
      orient: "vertical",
      right: 10,
      top: 40,
      bottom: 20,
      textStyle: { color: dark ? "#94a3b8" : "#64748b" },
    },
    series: [
      {
        name: isJa ? "送信元別" : "By Remote",
        type: "pie",
        radius: ["30%", "70%"],
        center: ["40%", "55%"],
        data: data.slice(0, 30),
        itemStyle: {
          borderColor: dark ? "#0f172a" : "#ffffff",
          borderWidth: 1,
        },
        emphasis: {
          itemStyle: {
            shadowBlur: 10,
            shadowOffsetX: 0,
            shadowColor: "rgba(0, 0, 0, 0.5)",
          },
        },
      },
    ],
  });
  return chart;
};

// 3. トピック別 (By Topic)
export const showMqttTopicChart = (div: string, stats: any[]) => {
  disposeChart();
  const map = new Map<string, { count: number; bytes: number }>();
  if (stats) {
    stats.forEach((s) => {
      const topic = s.Topic || "(Unknown)";
      const item = map.get(topic) || { count: 0, bytes: 0 };
      item.count += s.Count || 0;
      item.bytes += s.Bytes || 0;
      map.set(topic, item);
    });
  }

  const list = Array.from(map.entries()).map(([topic, val]) => ({
    topic,
    count: val.count,
    bytes: val.bytes,
  }));
  list.sort((a, b) => b.count - a.count);

  const categories: string[] = [];
  const counts: number[] = [];
  const topList = list.slice(0, 25).reverse();

  topList.forEach((item) => {
    categories.push(item.topic);
    counts.push(item.count);
  });

  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "トピック別受信回数 (TOP 25)" : "Received Count by Topic (TOP 25)",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
    },
    grid: {
      left: "20%",
      right: "10%",
      top: 50,
      bottom: 30,
      containLabel: true,
    },
    xAxis: {
      type: "value",
      name: isJa ? "受信回数" : "Count",
      axisLabel: { color: dark ? "#94a3b8" : "#64748b" },
      nameTextStyle: { color: dark ? "#94a3b8" : "#64748b" },
      splitLine: { lineStyle: { color: dark ? "#1e293b" : "#f1f5f9" } },
    },
    yAxis: {
      type: "category",
      data: categories,
      axisLabel: {
        color: dark ? "#cbd5e1" : "#334155",
        fontSize: 10,
        formatter: (val: string) => (val.length > 40 ? val.substring(0, 37) + "..." : val),
      },
      axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
    },
    series: [
      {
        name: isJa ? "受信回数" : "Count",
        type: "bar",
        data: counts,
        itemStyle: { color: "#10b981" },
      },
    ],
  });
  return chart;
};

// 4. ヒートマップ (Heatmap - Time activity or Client x Topic)
export const showMqttHeatmap = (div: string, stats: any[], mode: "time" | "client_topic" = "time") => {
  disposeChart();
  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;

  if (mode === "client_topic") {
    const clientSet = new Set<string>();
    const topicSet = new Set<string>();
    const matrixMap = new Map<string, number>();

    if (stats) {
      stats.forEach((s) => {
        const cid = s.ClientID || "(Unknown)";
        const topic = s.Topic || "(Unknown)";
        clientSet.add(cid);
        topicSet.add(topic);
        const key = `${cid}\t${topic}`;
        matrixMap.set(key, (matrixMap.get(key) || 0) + (s.Count || 0));
      });
    }

    const clients = Array.from(clientSet).slice(0, 25);
    const topics = Array.from(topicSet).slice(0, 25);

    const heatData: [number, number, number][] = [];
    let maxVal = 0;

    clients.forEach((c, cIdx) => {
      topics.forEach((t, tIdx) => {
        const val = matrixMap.get(`${c}\t${t}`) || 0;
        if (val > 0) {
          heatData.push([tIdx, cIdx, val]);
          if (val > maxVal) maxVal = val;
        }
      });
    });

    chart.setOption({
      backgroundColor: "transparent",
      title: {
        text: isJa ? "クライアント × トピック 受信回数ヒートマップ" : "Client x Topic Ingestion Heatmap",
        left: "center",
        textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
      },
      tooltip: {
        position: "top",
        backgroundColor: dark ? "#0f172a" : "#ffffff",
        borderColor: dark ? "#334155" : "#cbd5e1",
        textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
        formatter: (params: any) => {
          const tName = topics[params.data[0]];
          const cName = clients[params.data[1]];
          return `Client: ${cName}<br/>Topic: ${tName}<br/>Count: ${params.data[2].toLocaleString()}`;
        },
      },
      grid: {
        left: "15%",
        right: "10%",
        top: 60,
        bottom: 80,
        containLabel: true,
      },
      xAxis: {
        type: "category",
        data: topics,
        axisLabel: {
          color: dark ? "#94a3b8" : "#64748b",
          fontSize: 9,
          rotate: 30,
          formatter: (val: string) => (val.length > 20 ? val.substring(0, 17) + "..." : val),
        },
        axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
      },
      yAxis: {
        type: "category",
        data: clients,
        axisLabel: { color: dark ? "#cbd5e1" : "#334155", fontSize: 9 },
        axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
      },
      visualMap: {
        min: 0,
        max: maxVal || 1,
        calculable: true,
        orient: "horizontal",
        left: "center",
        bottom: 10,
        textStyle: { color: dark ? "#94a3b8" : "#64748b" },
        inRange: {
          color: dark
            ? ["#0f172a", "#0284c7", "#38bdf8", "#fbbf24", "#f43f5e"]
            : ["#f8fafc", "#bae6fd", "#38bdf8", "#fbbf24", "#f43f5e"],
        },
      },
      series: [
        {
          name: "Count",
          type: "heatmap",
          data: heatData,
        },
      ],
    });
    return chart;
  }

  // Time Activity Heatmap
  const hours = Array.from({ length: 24 }, (_, i) => `${i}:00`);
  const dateMap = new Map<string, number[]>();

  if (stats) {
    stats.forEach((s) => {
      const firstMs = s.First ? Math.floor(s.First / 1e6) : 0;
      const lastMs = s.Last ? Math.floor(s.Last / 1e6) : firstMs;
      const count = s.Count || 1;

      if (!firstMs) return;

      const spanHours = Math.max(1, Math.ceil((lastMs - firstMs) / (3600 * 1000)));
      const countPerHour = count / spanHours;

      let currentMs = firstMs;
      while (currentMs <= lastMs || currentMs === firstMs) {
        const d = new Date(currentMs);
        const dateStr = `${d.getFullYear()}/${String(d.getMonth() + 1).padStart(2, "0")}/${String(d.getDate()).padStart(2, "0")}`;
        const hour = d.getHours();

        if (!dateMap.has(dateStr)) {
          dateMap.set(dateStr, new Array(24).fill(0));
        }
        dateMap.get(dateStr)![hour] += countPerHour;

        if (spanHours <= 1 || currentMs + 3600 * 1000 > lastMs + 3600 * 1000) {
          break;
        }
        currentMs += 3600 * 1000;
      }
    });
  }

  const sortedDates = Array.from(dateMap.keys()).sort();
  const heatData: [number, number, number][] = [];
  let maxCount = 0;

  sortedDates.forEach((dateStr, dIdx) => {
    const hourCounts = dateMap.get(dateStr)!;
    hourCounts.forEach((val, hIdx) => {
      if (val > 0) {
        const rounded = Number(val.toFixed(1));
        heatData.push([dIdx, hIdx, rounded]);
        if (rounded > maxCount) maxCount = rounded;
      }
    });
  });

  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "日別・時間帯別 受信ヒートマップ" : "Daily & Hourly Ingestion Heatmap",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      position: "top",
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
      formatter: (params: any) => {
        const dStr = sortedDates[params.data[0]];
        const hStr = hours[params.data[1]];
        return `${dStr} ${hStr}<br/>${isJa ? "推定回数" : "Est. Count"}: ${params.data[2]}`;
      },
    },
    grid: {
      left: "10%",
      right: "5%",
      top: 60,
      bottom: 80,
    },
    xAxis: {
      type: "category",
      data: sortedDates,
      axisLabel: { color: dark ? "#94a3b8" : "#64748b", fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
    },
    yAxis: {
      type: "category",
      data: hours,
      axisLabel: { color: dark ? "#cbd5e1" : "#334155", fontSize: 10 },
      axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
    },
    visualMap: {
      min: 0,
      max: maxCount || 1,
      calculable: true,
      orient: "horizontal",
      left: "center",
      bottom: 10,
      textStyle: { color: dark ? "#94a3b8" : "#64748b" },
      inRange: {
        color: dark
          ? ["#0f172a", "#1e293b", "#0284c7", "#38bdf8", "#fbbf24", "#f43f5e"]
          : ["#f8fafc", "#bae6fd", "#0284c7", "#38bdf8", "#fbbf24", "#f43f5e"],
      },
    },
    series: [
      {
        name: "Activity",
        type: "heatmap",
        data: heatData,
      },
    ],
  });
  return chart;
};

// 5. 状態別 (Count by State)
export const showMqttStateChart = (div: string, stats: any[]) => {
  disposeChart();
  const map = new Map<string, number>();
  if (stats) {
    stats.forEach((s) => {
      const state = s.State || "normal";
      map.set(state, (map.get(state) || 0) + 1);
    });
  }

  const isJa = getIsJa();
  const data = Array.from(map.entries()).map(([state, count]) => ({
    name: state === "normal" ? (isJa ? "正常 (Normal)" : "Normal") : state === "warn" ? (isJa ? "注意 (Warn)" : "Warning") : (isJa ? "未受信 (Low)" : "Inactive"),
    value: count,
  }));

  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "トピック状態別割合" : "Topic State Distribution",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      trigger: "item",
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
      formatter: "{b}: <strong>{c}</strong> ({d}%)",
    },
    legend: {
      orient: "vertical",
      right: 20,
      top: "center",
      textStyle: { color: dark ? "#94a3b8" : "#64748b" },
    },
    series: [
      {
        name: isJa ? "状態別" : "By State",
        type: "pie",
        radius: ["40%", "70%"],
        data: data,
        itemStyle: {
          borderColor: dark ? "#0f172a" : "#ffffff",
          borderWidth: 2,
          color: (params: any) => {
            if (params.name.includes("Normal")) return "#10b981";
            if (params.name.includes("Warn")) return "#f59e0b";
            return "#ef4444";
          },
        },
      },
    ],
  });
  return chart;
};

// 6. トピック階層ツリーマップ (Topic Hierarchy Treemap)
export const showMqttTopicTreemap = (div: string, stats: any[]) => {
  disposeChart();
  interface TreeNode {
    name: string;
    value?: number;
    children?: TreeNode[];
    childrenMap?: Map<string, TreeNode>;
  }

  const rootChildren: Map<string, TreeNode> = new Map();

  if (stats) {
    stats.forEach((s) => {
      const topic = s.Topic || "unknown";
      const parts = topic.split("/").filter((p: string) => p.length > 0);
      const count = s.Count || 1;

      let currentChildren = rootChildren;

      parts.forEach((part: string, idx: number) => {
        let node = currentChildren.get(part);
        if (!node) {
          node = { name: part };
          if (idx < parts.length - 1) {
            node.children = [];
          }
          currentChildren.set(part, node);
        }
        if (idx === parts.length - 1) {
          node.value = (node.value || 0) + count;
        } else {
          if (!node.childrenMap) {
            node.childrenMap = new Map<string, TreeNode>();
          }
          currentChildren = node.childrenMap;
        }
      });
    });
  }

  const buildTree = (m: Map<string, TreeNode>): TreeNode[] => {
    const res: TreeNode[] = [];
    m.forEach((node) => {
      if (node.childrenMap) {
        node.children = buildTree(node.childrenMap);
        delete node.childrenMap;
      }
      res.push(node);
    });
    return res;
  };

  const treeData = buildTree(rootChildren);

  const dom = document.getElementById(div);
  if (!dom) return;
  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: isJa ? "トピック階層ツリーマップ" : "Topic Hierarchy Treemap",
      left: "center",
      textStyle: { fontSize: 14, color: dark ? "#f8fafc" : "#1e293b", fontWeight: 600 },
    },
    tooltip: {
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b" },
      formatter: isJa ? "{b}: <strong>{c}</strong> 回" : "{b}: <strong>{c}</strong>",
    },
    series: [
      {
        name: "Topics",
        type: "treemap",
        data: treeData,
        leafDepth: 2,
        levels: [
          {
            itemStyle: {
              borderColor: dark ? "#1e293b" : "#e2e8f0",
              borderWidth: 2,
              gapWidth: 2,
            },
          },
          {
            colorSaturation: [0.35, 0.6],
            itemStyle: {
              borderWidth: 1,
              gapWidth: 1,
              borderColorSaturation: 0.7,
            },
          },
        ],
      },
    ],
  });
  return chart;
};

/**
 * Overview State Donut Chart for MQTTView Stats Tab
 */
export const showMqttOverviewStatePie = (div: string | HTMLElement, stats: any[]): echarts.ECharts | null => {
  const dom = typeof div === "string" ? document.getElementById(div) : div;
  if (!dom) return null;

  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);

  const map = new Map<string, number>();
  if (stats) {
    stats.forEach((s) => {
      const state = s.State || "normal";
      map.set(state, (map.get(state) || 0) + 1);
    });
  }

  const data = [
    { name: isJa ? "正常 (Normal)" : "Normal", value: map.get("normal") || 0, color: "#10b981" },
    { name: isJa ? "注意 (Warn)" : "Warning", value: map.get("warn") || 0, color: "#f59e0b" },
    { name: isJa ? "未受信 (Low)" : "Inactive", value: map.get("low") || 0, color: "#f43f5e" },
  ].filter((d) => d.value > 0);

  const option: echarts.EChartsOption = {
    backgroundColor: "transparent",
    tooltip: {
      trigger: "item",
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b", fontSize: 11 },
      formatter: "{b}: <strong>{c}</strong> ({d}%)",
    },
    legend: {
      orient: "vertical",
      right: "2%",
      top: "middle",
      textStyle: { color: dark ? "#94a3b8" : "#64748b", fontSize: 10 },
      itemWidth: 10,
      itemHeight: 10,
    },
    series: [
      {
        name: isJa ? "状態別" : "By State",
        type: "pie",
        radius: ["45%", "72%"],
        center: ["36%", "50%"],
        avoidLabelOverlap: false,
        itemStyle: {
          borderRadius: 4,
          borderColor: dark ? "#0f172a" : "#ffffff",
          borderWidth: 2,
        },
        label: {
          show: false,
          position: "center",
        },
        emphasis: {
          label: {
            show: true,
            fontSize: 12,
            fontWeight: "bold",
            color: dark ? "#f8fafc" : "#0f172a",
          },
        },
        data: data.length > 0 ? data.map((d) => ({ name: d.name, value: d.value, itemStyle: { color: d.color } })) : [{ name: isJa ? "データなし" : "No Data", value: 0, itemStyle: { color: "#94a3b8" } }],
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
};

/**
 * Overview Top Topics Horizontal Bar Chart for MQTTView Stats Tab
 */
export const showMqttOverviewTopicBar = (div: string | HTMLElement, stats: any[]): echarts.ECharts | null => {
  const dom = typeof div === "string" ? document.getElementById(div) : div;
  if (!dom) return null;

  const dark = isDarkMode();
  const isJa = getIsJa();
  const chart = echarts.init(dom, dark ? "dark" : undefined);

  const sorted = [...(stats || [])].sort((a, b) => (b.Count || 0) - (a.Count || 0)).slice(0, 10).reverse();
  const categories = sorted.map((s) => s.Topic);
  const counts = sorted.map((s) => s.Count || 0);

  const option: echarts.EChartsOption = {
    backgroundColor: "transparent",
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      backgroundColor: dark ? "#0f172a" : "#ffffff",
      borderColor: dark ? "#334155" : "#cbd5e1",
      textStyle: { color: dark ? "#f8fafc" : "#1e293b", fontSize: 11 },
      formatter: (params: any) => {
        if (!params || params.length === 0) return "";
        return isJa
          ? `トピック: <strong class="break-all">${params[0].name}</strong><br/>受信回数: <strong>${params[0].value.toLocaleString()}</strong> 回`
          : `Topic: <strong class="break-all">${params[0].name}</strong><br/>Count: <strong>${params[0].value.toLocaleString()}</strong>`;
      },
    },
    grid: {
      left: "10px",
      right: "45px",
      top: "15px",
      bottom: "10px",
      containLabel: true,
    },
    xAxis: {
      type: "value",
      axisLabel: { color: dark ? "#94a3b8" : "#64748b", fontSize: 9 },
      splitLine: { lineStyle: { color: dark ? "#1e293b" : "#f1f5f9" } },
    },
    yAxis: {
      type: "category",
      data: categories,
      axisLabel: {
        color: dark ? "#cbd5e1" : "#334155",
        fontSize: 10,
        width: 150,
        overflow: "truncate",
      },
      axisLine: { lineStyle: { color: dark ? "#334155" : "#cbd5e1" } },
    },
    series: [
      {
        name: "Count",
        type: "bar",
        data: counts,
        barMaxWidth: 22,
        itemStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
            { offset: 0, color: "#06b6d4" },
            { offset: 1, color: "#3b82f6" },
          ]),
          borderRadius: [0, 4, 4, 0],
        },
        label: {
          show: true,
          position: "right",
          color: dark ? "#cbd5e1" : "#334155",
          fontSize: 10,
          formatter: (val: any) => (val.value || 0).toLocaleString(),
        },
      },
    ],
  };

  chart.setOption(option);
  chart.resize();
  return chart;
};
