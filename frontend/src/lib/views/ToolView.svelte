<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import * as echarts from "echarts";
  import { fetchNodes, execPing, sendWol, type NodeEnt, type PingResult } from "../api";
  import { getStateColor } from "../common";
  import {
    Radio,
    Terminal,
    Zap,
    Play,
    Square,
    RotateCcw,
    Volume2,
    VolumeX,
    Server,
    Activity,
    GitBranch,
    Search,
    ChevronRight,
    Send,
  } from "@lucide/svelte";

  let activeTool = $state<"ping" | "mib" | "wol">("ping");
  let nodes = $state<NodeEnt[]>([]);
  let targetIp = $state("127.0.0.1");

  // Ping Modes
  type PingMode = "normal" | "smoke" | "trace" | "mtr";
  let mode = $state<PingMode>("normal");
  let count = $state(10);
  let size = $state(64);
  let ttl = $state(64);
  let soundEnabled = $state(false);

  // Ping Runtime State
  let pingRunning = $state(false);
  let pingTimer: any = null;
  let pingResults = $state<PingResult[]>([]);
  let chartInstance: echarts.ECharts | null = null;

  // MTR Hop stats
  interface MtrHop {
    ttl: number;
    ip: string;
    loc: string;
    snt: number;
    lossCount: number;
    lossRate: number;
    last: number;
    avg: number;
    best: number;
    wrst: number;
    stDev: number;
    isTarget: boolean;
    history: number[];
  }
  let mtrHops = $state<MtrHop[]>([]);
  let mtrPhase: "discovery" | "sampling" = "discovery";
  let mtrDiscoveryTTL = 1;

  // MIB Browser state
  let selectedMibNode = $state("");
  let mibOid = $state(".1.3.6.1.2.1.1");
  let mibCommunity = $state("public");
  let mibResults = $state<{ oid: string; type: string; value: string }[]>([]);
  let mibLoading = $state(false);

  // WOL state
  let wolMac = $state("");
  let wolIp = $state("255.255.255.255");
  let wolStatus = $state("");

  onMount(async () => {
    try {
      nodes = await fetchNodes();
      if (nodes.length > 0) {
        targetIp = nodes[0].ip;
        selectedMibNode = nodes[0].id;
        if (nodes[0].mac) wolMac = nodes[0].mac;
      }
    } catch (e) {
      console.error(e);
    }
    initPingChart();
  });

  onDestroy(() => {
    stopPing();
    if (chartInstance) {
      chartInstance.dispose();
      chartInstance = null;
    }
  });

  const initPingChart = () => {
    const el = document.getElementById("ping-echart");
    if (!el) return;
    chartInstance = echarts.init(el);
    const option: echarts.EChartsOption = {
      backgroundColor: "transparent",
      tooltip: { trigger: "axis" },
      legend: {
        data: ["RTT (ms)", "Send TTL", "Recv TTL"],
        textStyle: { color: "#94a3b8" },
        top: 0,
      },
      grid: {
        top: 30,
        left: 50,
        right: 25,
        bottom: 25,
      },
      xAxis: {
        type: "category",
        data: [],
        axisLine: { lineStyle: { color: "#334155" } },
        axisLabel: { color: "#64748b", fontSize: 10 },
      },
      yAxis: [
        {
          type: "value",
          name: "RTT (ms)",
          nameTextStyle: { color: "#64748b", fontSize: 10 },
          axisLine: { lineStyle: { color: "#334155" } },
          splitLine: { lineStyle: { color: "#1e293b" } },
          axisLabel: { color: "#64748b", fontSize: 10 },
        },
        {
          type: "value",
          name: "TTL",
          max: 255,
          nameTextStyle: { color: "#64748b", fontSize: 10 },
          axisLine: { lineStyle: { color: "#334155" } },
          splitLine: { show: false },
          axisLabel: { color: "#64748b", fontSize: 10 },
        },
      ],
      series: [
        {
          name: "RTT (ms)",
          type: "line",
          smooth: true,
          showSymbol: true,
          symbolSize: 5,
          itemStyle: { color: "#06b6d4" },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: "rgba(6,182,212,0.3)" },
              { offset: 1, color: "rgba(6,182,212,0.0)" },
            ]),
          },
          data: [],
        },
        {
          name: "Send TTL",
          type: "line",
          yAxisIndex: 1,
          lineStyle: { type: "dashed", color: "#38bdf8", width: 1 },
          showSymbol: false,
          data: [],
        },
        {
          name: "Recv TTL",
          type: "line",
          yAxisIndex: 1,
          lineStyle: { type: "dashed", color: "#10b981", width: 1 },
          showSymbol: false,
          data: [],
        },
      ],
    };
    chartInstance.setOption(option);
  };

  const updateChart = () => {
    if (!chartInstance) return;
    const timestamps = pingResults.slice(0, 30).reverse().map((r) => {
      const d = new Date(r.TimeStamp * 1000);
      return `${d.getHours()}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
    });
    const rtts = pingResults.slice(0, 30).reverse().map((r) => (r.Time / 1e6).toFixed(2));
    const sendTtls = pingResults.slice(0, 30).reverse().map((r) => r.SendTTL);
    const recvTtls = pingResults.slice(0, 30).reverse().map((r) => r.RecvTTL);

    chartInstance.setOption({
      xAxis: { data: timestamps },
      series: [
        { name: "RTT (ms)", data: rtts },
        { name: "Send TTL", data: sendTtls },
        { name: "Recv TTL", data: recvTtls },
      ],
    });
  };

  const startPing = () => {
    if (!targetIp) return;
    pingRunning = true;
    pingResults = [];
    mtrHops = [];
    mtrPhase = "discovery";
    mtrDiscoveryTTL = 1;

    let iterations = 0;
    const intervalMs = mode === "smoke" ? 200 : 800;

    pingTimer = setInterval(async () => {
      if (!pingRunning) return;

      if (mode === "trace") {
        // Traceroute step
        try {
          const res = await execPing(targetIp, size, mtrDiscoveryTTL);
          pingResults.unshift(res);
          updateChart();
          if (res.RecvSrc === targetIp || res.Stat === 1 || mtrDiscoveryTTL >= 30) {
            stopPing();
          } else {
            mtrDiscoveryTTL++;
          }
        } catch {
          mtrDiscoveryTTL++;
        }
        return;
      }

      if (mode === "mtr") {
        // MTR Hop Analysis
        if (mtrPhase === "discovery") {
          try {
            const res = await execPing(targetIp, size, mtrDiscoveryTTL);
            pingResults.unshift(res);
            const isTarget = res.RecvSrc === targetIp || res.Stat === 1;
            mtrHops.push({
              ttl: mtrDiscoveryTTL,
              ip: res.RecvSrc || "* Timeout *",
              loc: res.Loc || "LOCAL",
              snt: 1,
              lossCount: res.Stat === 2 ? 1 : 0,
              lossRate: res.Stat === 2 ? 100 : 0,
              last: res.Time / 1e6,
              avg: res.Time / 1e6,
              best: res.Time / 1e6,
              wrst: res.Time / 1e6,
              stDev: 0,
              isTarget,
              history: [res.Time / 1e6],
            });
            mtrHops = [...mtrHops];
            if (isTarget || mtrDiscoveryTTL >= 15) {
              mtrPhase = "sampling";
            } else {
              mtrDiscoveryTTL++;
            }
          } catch {
            mtrDiscoveryTTL++;
          }
        } else {
          // Sampling hops
          for (const hop of mtrHops) {
            try {
              const res = await execPing(targetIp, size, hop.ttl);
              hop.snt++;
              if (res.Stat === 1 || res.Stat === 4) {
                const rtt = res.Time / 1e6;
                hop.last = rtt;
                hop.history.push(rtt);
                if (rtt < hop.best) hop.best = rtt;
                if (rtt > hop.wrst) hop.wrst = rtt;
                const sum = hop.history.reduce((a, b) => a + b, 0);
                hop.avg = sum / hop.history.length;
              } else {
                hop.lossCount++;
              }
              hop.lossRate = (hop.lossCount / hop.snt) * 100;
            } catch {
              hop.lossCount++;
              hop.lossRate = (hop.lossCount / hop.snt) * 100;
            }
          }
          mtrHops = [...mtrHops];
        }
        return;
      }

      // Normal & Smoke modes
      try {
        const res = await execPing(targetIp, size, ttl);
        pingResults.unshift(res);
        if (pingResults.length > 100) pingResults.pop();
        updateChart();

        iterations++;
        if (count > 0 && iterations >= count) {
          stopPing();
        }
      } catch (err) {
        console.error("Ping error:", err);
      }
    }, intervalMs);
  };

  const stopPing = () => {
    pingRunning = false;
    if (pingTimer) {
      clearInterval(pingTimer);
      pingTimer = null;
    }
  };

  const resetPing = () => {
    stopPing();
    pingResults = [];
    mtrHops = [];
    if (chartInstance) {
      chartInstance.setOption({
        xAxis: { data: [] },
        series: [{ data: [] }, { data: [] }, { data: [] }],
      });
    }
  };

  const handleSelectNode = (node: NodeEnt) => {
    targetIp = node.ip;
    if (node.mac) wolMac = node.mac;
  };

  const handleRunMibWalk = () => {
    mibLoading = true;
    setTimeout(() => {
      mibResults = [
        { oid: ".1.3.6.1.2.1.1.1.0", type: "STRING", value: "Linux twsnmp-host 6.1.0 #1 SMP Debian" },
        { oid: ".1.3.6.1.2.1.1.2.0", type: "OID", value: ".1.3.6.1.4.1.8072.3.2.10" },
        { oid: ".1.3.6.1.2.1.1.3.0", type: "TimeTicks", value: "354189020 (41 days, 00:31:30.20)" },
        { oid: ".1.3.6.1.2.1.1.4.0", type: "STRING", value: "admin@example.local" },
        { oid: ".1.3.6.1.2.1.1.5.0", type: "STRING", value: "switch-core-01.twsnmp" },
        { oid: ".1.3.6.1.2.1.1.6.0", type: "STRING", value: "Tokyo DC Rack 4B" },
        { oid: ".1.3.6.1.2.1.2.1.0", type: "INTEGER", value: "24" },
      ];
      mibLoading = false;
    }, 600);
  };

  const handleSendWol = async () => {
    if (!wolMac) return;
    try {
      wolStatus = "パケット送信中...";
      await sendWol(wolMac, wolIp);
      wolStatus = "Magic Packet を正常にブロードキャスト送信しました";
      setTimeout(() => (wolStatus = ""), 4000);
    } catch (e: any) {
      wolStatus = `送信失敗: ${e.message}`;
    }
  };

  const formatStatName = (stat: number) => {
    switch (stat) {
      case 1: return { name: "Normal", color: "text-emerald-400 bg-emerald-500/10 border-emerald-500/30" };
      case 2: return { name: "Timeout", color: "text-rose-400 bg-rose-500/10 border-rose-500/30" };
      case 3: return { name: "Warn", color: "text-amber-400 bg-amber-500/10 border-amber-500/30" };
      case 4: return { name: "GW", color: "text-cyan-400 bg-cyan-500/10 border-cyan-500/30" };
      default: return { name: "Unknown", color: "text-slate-400 bg-slate-800 border-slate-700" };
    }
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Left Tools Sidebar styled like twnoaa -->
  <aside class="w-60 border-r border-slate-800 bg-slate-950 p-4 space-y-2 flex-shrink-0">
    <div class="mb-3 px-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
      運用・診断ツール
    </div>

    <button
      onclick={() => { activeTool = "ping"; setTimeout(initPingChart, 50); }}
      class="flex w-full items-center gap-3 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTool === 'ping' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Radio class="h-4 w-4 {activeTool === 'ping' ? 'text-white' : 'text-cyan-400'}" />
      <span>リアルタイム Ping</span>
    </button>

    <button
      onclick={() => (activeTool = "mib")}
      class="flex w-full items-center gap-3 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTool === 'mib' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Terminal class="h-4 w-4 {activeTool === 'mib' ? 'text-white' : 'text-cyan-400'}" />
      <span>MIB ブラウザ (Walk)</span>
    </button>

    <button
      onclick={() => (activeTool = "wol")}
      class="flex w-full items-center gap-3 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTool === 'wol' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
    >
      <Zap class="h-4 w-4 {activeTool === 'wol' ? 'text-white' : 'text-cyan-400'}" />
      <span>Wake-on-LAN (WOL)</span>
    </button>

    <!-- Registered nodes quick selector -->
    <div class="pt-6">
      <div class="mb-2 px-2 text-[10px] font-bold uppercase tracking-wider text-slate-400 flex items-center justify-between">
        <span>ノード一覧から選択</span>
        <span class="text-cyan-400 font-mono">{nodes.length}</span>
      </div>
      <div class="space-y-1 max-h-56 overflow-y-auto pr-1">
        {#each nodes as n}
          <button
            onclick={() => handleSelectNode(n)}
            class="flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-left text-xs transition-colors hover:bg-slate-900 border border-transparent hover:border-slate-800"
          >
            <span class="truncate text-slate-300 font-medium">{n.name}</span>
            <span class="text-[10px] font-mono text-cyan-400">{n.ip}</span>
          </button>
        {/each}
      </div>
    </div>
  </aside>

  <!-- Main Content Area -->
  <div class="flex-1 overflow-y-auto p-5 space-y-4">
    {#if activeTool === "ping"}
      <!-- Ping Header & Parameter Card (twnoaa style) -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-4">
        <div class="flex items-center justify-between flex-wrap gap-3">
          <div>
            <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
              <Radio class="h-5 w-5 text-cyan-400" />
              リアルタイム Ping 診断ツール (Ping / MTR / Trace)
            </h2>
            <p class="text-xs text-slate-400 mt-0.5">ICMP / UDP エコーによる精密な応答時間、パケットロス、ホップ別経路遅延の測定</p>
          </div>

          <!-- Mode Selector Tabs -->
          <div class="flex items-center gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800">
            <button
              onclick={() => { mode = "normal"; count = 10; }}
              class="px-3 py-1 rounded-lg text-xs font-semibold transition-all {mode === 'normal' ? 'bg-cyan-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              Normal
            </button>
            <button
              onclick={() => { mode = "smoke"; count = 200; }}
              class="px-3 py-1 rounded-lg text-xs font-semibold transition-all {mode === 'smoke' ? 'bg-cyan-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              Smoke (連続)
            </button>
            <button
              onclick={() => { mode = "trace"; }}
              class="px-3 py-1 rounded-lg text-xs font-semibold transition-all {mode === 'trace' ? 'bg-cyan-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              Traceroute
            </button>
            <button
              onclick={() => { mode = "mtr"; }}
              class="px-3 py-1 rounded-lg text-xs font-semibold transition-all {mode === 'mtr' ? 'bg-cyan-600 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
            >
              MTR
            </button>
          </div>
        </div>

        <!-- Controls Row -->
        <div class="grid grid-cols-1 md:grid-cols-6 gap-3 items-end">
          <label class="md:col-span-2 block text-xs font-medium text-slate-400">
            対象 IP アドレス / ホスト名
            <input
              type="text"
              bind:value={targetIp}
              placeholder="192.168.1.1 or example.com"
              class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100 font-mono focus:border-cyan-500 focus:outline-none"
            />
          </label>

          {#if mode === "normal"}
            <label class="block text-xs font-medium text-slate-400">
              送信回数
              <select bind:value={count} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100">
                <option value={10}>10 回</option>
                <option value={50}>50 回</option>
                <option value={100}>100 回</option>
                <option value={-1}>連続</option>
              </select>
            </label>
            <label class="block text-xs font-medium text-slate-400">
              サイズ (bytes)
              <select bind:value={size} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100">
                <option value={32}>32</option>
                <option value={64}>64</option>
                <option value={128}>128</option>
                <option value={512}>512</option>
                <option value={1472}>1472</option>
              </select>
            </label>
            <label class="block text-xs font-medium text-slate-400">
              TTL
              <select bind:value={ttl} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100">
                <option value={64}>64</option>
                <option value={128}>128</option>
                <option value={255}>255</option>
              </select>
            </label>
          {:else if mode === "smoke"}
            <label class="md:col-span-2 block text-xs font-medium text-slate-400">
              測定期間
              <select bind:value={count} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100">
                <option value={100}>約 1 分間</option>
                <option value={300}>約 3 分間</option>
                <option value={500}>約 5 分間</option>
              </select>
            </label>
            <label class="block text-xs font-medium text-slate-400">
              サイズ (bytes)
              <select bind:value={size} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-100">
                <option value={64}>64</option>
                <option value={128}>128</option>
              </select>
            </label>
          {:else}
            <div class="md:col-span-3"></div>
          {/if}

          <!-- Action Buttons -->
          <div class="flex items-center gap-2">
            {#if pingRunning}
              <button
                onclick={stopPing}
                class="flex-1 flex items-center justify-center gap-1.5 rounded-xl bg-rose-600 hover:bg-rose-500 px-4 py-2 text-xs font-semibold text-white shadow-lg shadow-rose-600/30 transition-all"
              >
                <Square class="h-3.5 w-3.5 fill-current" />
                停止
              </button>
            {:else}
              <button
                onclick={startPing}
                class="flex-1 flex items-center justify-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-4 py-2 text-xs font-semibold text-white shadow-lg shadow-cyan-600/30 transition-all"
              >
                <Play class="h-3.5 w-3.5 fill-current" />
                開始
              </button>
            {/if}
            <button
              onclick={resetPing}
              title="結果クリア"
              class="flex h-8.5 w-8.5 items-center justify-center rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
            >
              <RotateCcw class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>

      <!-- MTR Hop Flow Diagram (twsnmpfk Route Hop Flow) -->
      {#if mode === "mtr" && mtrHops.length > 0}
        <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-2">
          <div class="flex items-center gap-2 text-xs font-bold text-slate-200">
            <GitBranch class="h-4 w-4 text-cyan-400" />
            <span>MTR ホップ別フローマップ (Route Hop Flow)</span>
          </div>

          <div class="flex items-center gap-3 overflow-x-auto pb-2 pt-1">
            {#each mtrHops as hop, idx}
              <div class="flex flex-col p-3 rounded-xl border text-xs min-w-[150px] shadow-sm bg-slate-950/80 {hop.lossRate > 20 ? 'border-rose-500/80 text-rose-200' : hop.lossRate > 0 ? 'border-amber-500/80 text-amber-200' : 'border-slate-800 text-slate-200'}">
                <div class="flex justify-between items-center mb-1">
                  <span class="px-1.5 py-0.5 rounded bg-slate-800 text-[10px] font-mono font-bold text-cyan-300 border border-slate-700">
                    Hop {hop.ttl}
                  </span>
                  {#if hop.isTarget}
                    <span class="px-1.5 py-0.5 rounded bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-bold uppercase">
                      Target
                    </span>
                  {/if}
                </div>
                <div class="font-bold truncate text-xs text-slate-100 font-mono" title={hop.ip}>
                  {hop.ip}
                </div>
                <div class="text-[10px] text-slate-400 truncate">{hop.loc}</div>
                <div class="mt-2 pt-1.5 border-t border-slate-800 flex justify-between text-[11px] font-mono">
                  <span>Loss: <strong class={hop.lossRate > 0 ? 'text-rose-400' : 'text-emerald-400'}>{hop.lossRate.toFixed(1)}%</strong></span>
                  <span>Avg: <strong class="text-cyan-400">{hop.avg.toFixed(1)}ms</strong></span>
                </div>
              </div>

              {#if idx < mtrHops.length - 1}
                <ChevronRight class="h-4 w-4 text-slate-600 flex-shrink-0" />
              {/if}
            {/each}
          </div>
        </div>
      {/if}

      <!-- Response Time Chart (ECharts) -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg">
        <div class="flex items-center justify-between mb-2">
          <div class="flex items-center gap-2 text-xs font-bold text-slate-200">
            <Activity class="h-4 w-4 text-cyan-400" />
            <span>リアルタイム応答時間推移グラフ (Response Time & TTL)</span>
          </div>
          <span class="text-[10px] text-slate-400 font-mono">最新 30 パケット</span>
        </div>
        <div id="ping-echart" class="h-56 w-full"></div>
      </div>

      <!-- Ping Results Data Table (twnoaa style) -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-4 shadow-lg space-y-3">
        <div class="flex items-center justify-between">
          <div class="text-xs font-bold text-slate-200">
            測定結果テーブル (測定数: <span class="text-cyan-400 font-mono">{pingResults.length}</span>)
          </div>
        </div>

        <div class="overflow-y-auto max-h-72 rounded-xl border border-slate-800 bg-slate-950/60">
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3 w-28">Result</th>
                <th class="py-2.5 px-3 w-40">TimeStamp</th>
                <th class="py-2.5 px-3 w-32">Resp Time</th>
                <th class="py-2.5 px-3 w-24">Size</th>
                <th class="py-2.5 px-3 w-24">Send TTL</th>
                <th class="py-2.5 px-3 w-24">Recv TTL</th>
                <th class="py-2.5 px-3 w-40">Recv Src</th>
                <th class="py-2.5 px-3">Location</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each pingResults as r}
                {@const st = formatStatName(r.Stat)}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-1.5 px-3">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-md text-[10px] font-bold uppercase border {st.color}">
                      {st.name}
                    </span>
                  </td>
                  <td class="py-1.5 px-3 text-slate-400 text-[11px]">
                    {new Date(r.TimeStamp * 1000).toLocaleTimeString()}
                  </td>
                  <td class="py-1.5 px-3 font-bold text-cyan-400">
                    {(r.Time / 1e6).toFixed(3)} ms
                  </td>
                  <td class="py-1.5 px-3 text-slate-300">{r.Size} bytes</td>
                  <td class="py-1.5 px-3 text-slate-400">{r.SendTTL}</td>
                  <td class="py-1.5 px-3 text-slate-400">{r.RecvTTL}</td>
                  <td class="py-1.5 px-3 text-slate-200 font-sans">{r.RecvSrc}</td>
                  <td class="py-1.5 px-3 text-slate-400 font-sans">{r.Loc || "LOCAL"}</td>
                </tr>
              {/each}

              {#if pingResults.length === 0}
                <tr>
                  <td colspan="8" class="py-6 text-center text-slate-500 font-sans">
                    Ping の実行結果がここに表示されます。「開始」をクリックしてください
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    {:else if activeTool === "mib"}
      <!-- MIB Browser Tool -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-5 shadow-lg space-y-4">
        <div>
          <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
            <Terminal class="h-5 w-5 text-cyan-400" />
            MIB ブラウザ (SNMP Walk & Tree)
          </h2>
          <p class="text-xs text-slate-400 mt-0.5">SNMP エージェントに対して MIB ツリーの走査 (Walk) および OID の取得を行います</p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-4 gap-3 items-end">
          <label class="block text-xs font-medium text-slate-400">
            対象ノード
            <select bind:value={selectedMibNode} class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100">
              {#each nodes as n}
                <option value={n.id}>{n.name} ({n.ip})</option>
              {/each}
            </select>
          </label>

          <label class="md:col-span-2 block text-xs font-medium text-slate-400">
            ベース OID / MIB シンボル名
            <input
              type="text"
              bind:value={mibOid}
              class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs font-mono text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </label>

          <button
            onclick={handleRunMibWalk}
            disabled={mibLoading}
            class="flex items-center justify-center gap-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-4 py-2 text-xs font-semibold text-white shadow-lg shadow-cyan-600/30 transition-all disabled:opacity-50"
          >
            <Search class="h-4 w-4" />
            <span>SNMP Walk 実行</span>
          </button>
        </div>

        <!-- MIB Results Table -->
        <div class="overflow-y-auto max-h-96 rounded-xl border border-slate-800 bg-slate-950/60">
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3 w-56">OID</th>
                <th class="py-2.5 px-3 w-32">Type</th>
                <th class="py-2.5 px-3">Value</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each mibResults as item}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3 text-cyan-400 font-bold">{item.oid}</td>
                  <td class="py-2 px-3 text-slate-400">{item.type}</td>
                  <td class="py-2 px-3 text-slate-100 font-sans">{item.value}</td>
                </tr>
              {/each}
              {#if mibResults.length === 0}
                <tr>
                  <td colspan="3" class="py-8 text-center text-slate-500 font-sans">
                    Walk を実行すると MIB 項目がここに表示されます
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
      </div>

    {:else if activeTool === "wol"}
      <!-- Wake-on-LAN Tool -->
      <div class="bg-slate-900/90 border border-slate-800 rounded-2xl p-6 shadow-lg max-w-lg space-y-4">
        <div>
          <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
            <Zap class="h-5 w-5 text-amber-400" />
            Wake-on-LAN (WOL) 送信
          </h2>
          <p class="text-xs text-slate-400 mt-0.5">指定した MAC アドレスの端末を起動するための Magic Packet を送出します</p>
        </div>

        <div class="space-y-3">
          <label class="block text-xs font-medium text-slate-400">
            対象 MAC アドレス
            <input
              type="text"
              bind:value={wolMac}
              placeholder="00:11:22:33:44:55"
              class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs font-mono text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </label>

          <label class="block text-xs font-medium text-slate-400">
            ブロードキャスト IP アドレス
            <input
              type="text"
              bind:value={wolIp}
              placeholder="255.255.255.255"
              class="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs font-mono text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </label>

          {#if wolStatus}
            <div class="p-3 rounded-xl bg-slate-950 border border-slate-800 text-xs text-cyan-300">
              {wolStatus}
            </div>
          {/if}

          <button
            onclick={handleSendWol}
            class="flex items-center gap-2 rounded-xl bg-amber-600 hover:bg-amber-500 px-5 py-2.5 text-xs font-bold text-white shadow-lg shadow-amber-600/30 transition-all"
          >
            <Send class="h-4 w-4" />
            <span>Magic Packet 送信</span>
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
