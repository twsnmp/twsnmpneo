import * as echarts from "echarts";

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
  const chart = echarts.init(dom, "dark");
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: "クライアントID別受信統計",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
    },
    legend: {
      top: 30,
      textStyle: { color: "#94a3b8" },
      data: ["受信回数", "データ量 (MB)"],
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
        name: "受信回数",
        axisLabel: { color: "#94a3b8" },
      },
      {
        type: "value",
        name: "MB",
        axisLabel: { color: "#94a3b8" },
      },
    ],
    yAxis: {
      type: "category",
      data: categories,
      axisLabel: { color: "#cbd5e1", fontSize: 11 },
    },
    series: [
      {
        name: "受信回数",
        type: "bar",
        data: counts,
        itemStyle: { color: "#06b6d4" },
      },
      {
        name: "データ量 (MB)",
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
  const chart = echarts.init(dom, "dark");
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: "送信元IP別受信統計",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      trigger: "item",
      formatter: (params: any) => {
        const bytesMB = (params.data.bytes / (1024 * 1024)).toFixed(2);
        return `${params.name}<br/>受信回数: ${params.value.toLocaleString()}<br/>データ量: ${bytesMB} MB (${params.percent}%)`;
      },
    },
    legend: {
      type: "scroll",
      orient: "vertical",
      right: 10,
      top: 40,
      bottom: 20,
      textStyle: { color: "#94a3b8" },
    },
    series: [
      {
        name: "送信元別",
        type: "pie",
        radius: ["30%", "70%"],
        center: ["40%", "55%"],
        data: data.slice(0, 30),
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
  const chart = echarts.init(dom, "dark");
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: "トピック別受信回数 (TOP 25)",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
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
      name: "受信回数",
      axisLabel: { color: "#94a3b8" },
    },
    yAxis: {
      type: "category",
      data: categories,
      axisLabel: {
        color: "#cbd5e1",
        fontSize: 10,
        formatter: (val: string) => (val.length > 40 ? val.substring(0, 37) + "..." : val),
      },
    },
    series: [
      {
        name: "受信回数",
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
  const chart = echarts.init(dom, "dark");
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
        text: "クライアント × トピック 受信回数ヒートマップ",
        left: "center",
        textStyle: { fontSize: 14, color: "#cbd5e1" },
      },
      tooltip: {
        position: "top",
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
          color: "#94a3b8",
          fontSize: 9,
          rotate: 30,
          formatter: (val: string) => (val.length > 20 ? val.substring(0, 17) + "..." : val),
        },
      },
      yAxis: {
        type: "category",
        data: clients,
        axisLabel: { color: "#cbd5e1", fontSize: 9 },
      },
      visualMap: {
        min: 0,
        max: maxVal || 1,
        calculable: true,
        orient: "horizontal",
        left: "center",
        bottom: 10,
        textStyle: { color: "#94a3b8" },
        inRange: {
          color: ["#0f172a", "#0284c7", "#38bdf8", "#fbbf24", "#f43f5e"],
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
      text: "日別・時間帯別 受信ヒートマップ",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      position: "top",
      formatter: (params: any) => {
        const dStr = sortedDates[params.data[0]];
        const hStr = hours[params.data[1]];
        return `${dStr} ${hStr}<br/>推定回数: ${params.data[2]}`;
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
      axisLabel: { color: "#94a3b8", fontSize: 10 },
    },
    yAxis: {
      type: "category",
      data: hours,
      axisLabel: { color: "#cbd5e1", fontSize: 10 },
    },
    visualMap: {
      min: 0,
      max: maxCount || 1,
      calculable: true,
      orient: "horizontal",
      left: "center",
      bottom: 10,
      textStyle: { color: "#94a3b8" },
      inRange: {
        color: ["#0f172a", "#1e293b", "#0284c7", "#38bdf8", "#fbbf24", "#f43f5e"],
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

  const data = Array.from(map.entries()).map(([state, count]) => ({
    name: state === "normal" ? "正常 (Normal)" : state === "warn" ? "注意 (Warn)" : "未受信 (Low)",
    value: count,
  }));

  const dom = document.getElementById(div);
  if (!dom) return;
  const chart = echarts.init(dom, "dark");
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: "トピック状態別割合",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      trigger: "item",
      formatter: "{b}: {c} ({d}%)",
    },
    legend: {
      orient: "vertical",
      right: 20,
      top: "center",
      textStyle: { color: "#94a3b8" },
    },
    series: [
      {
        name: "状態別",
        type: "pie",
        radius: ["40%", "70%"],
        data: data,
        itemStyle: {
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
  const chart = echarts.init(dom, "dark");
  currentChart = chart;
  chart.setOption({
    backgroundColor: "transparent",
    title: {
      text: "トピック階層ツリーマップ",
      left: "center",
      textStyle: { fontSize: 14, color: "#cbd5e1" },
    },
    tooltip: {
      formatter: "{b}: {c} 回",
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
              borderColor: "#1e293b",
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
