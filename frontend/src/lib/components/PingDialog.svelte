<script lang="ts">
  import { tick, untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import { execPing, type PingResult, type NodeEnt, type NetworkEnt } from "../api";
  import { getPingChartOption, showPingHistogram, showPingSmokeChart } from "../charts/ping";
  import { showMtrProfileChart, type MTRHopStat } from "../charts/mtr";
  import * as echarts from "echarts";
  import {
    X,
    Play,
    Square,
    Activity,
    Cloud,
    GitBranch,
    Route,
    RotateCcw,
    BarChart3,
    TrendingUp,
  } from "@lucide/svelte";

  type PingMode = "normal" | "smoke" | "trace" | "mtr";
  type ActiveTab = "ping" | "mtr" | "smoke" | "histogram";

  let {
    show = $bindable(false),
    node = null,
    network = null,
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    network?: NetworkEnt | null;
  }>();

  let mode = $state<PingMode>("normal");
  let activeTab = $state<ActiveTab>("ping");
  let ip = $state("");
  let size = $state(64);
  let count = $state(10);
  let ttl = $state(64);
  let isRunning = $state(false);
  let stopFlag = $state(false);
  let results = $state<PingResult[]>([]);
  let mtrHops = $state<(MTRHopStat & { history: number[]; lossCount: number })[]>([]);

  let mainChartEl = $state<HTMLDivElement | null>(null);
  let histChartEl = $state<HTMLDivElement | null>(null);
  let smokeChartEl = $state<HTMLDivElement | null>(null);
  let mtrChartEl = $state<HTMLDivElement | null>(null);

  let chartInstance: echarts.ECharts | null = null;
  let chartOption: any = null;

  const titleName = $derived(node?.name || (node as any)?.Name || network?.name || (network as any)?.Name || "");
  const canShowMtr = $derived(mtrHops.length > 0);
  const canShowStats = $derived(!isRunning && results.length > 0);

  const pingChartLabels = () => ({
    responseTime: $_("Ping.responseTimeSeconds"),
    sendTtl: $_("Ping.sendTtl"),
    recvTtl: $_("Ping.recvTtl"),
  });

  const mtrChartLabels = () => ({
    averageRtt: $_("Ping.averageRtt"),
    rttRange: $_("Ping.rttRange"),
    lossRate: $_("Ping.lossRate"),
    rtt: $_("Ping.rtt"),
    minimumRtt: $_("Ping.minimumRtt"),
  });

  $effect(() => {
    if (show && (node || network)) {
      untrack(() => {
        resetState();
        ip = node?.ip || (node as any)?.IP || network?.ip || (network as any)?.IP || "";
      });
      void initMainChart();
    } else {
      stopFlag = true;
      isRunning = false;
      if (chartInstance) {
        chartInstance.dispose();
        chartInstance = null;
      }
    }
  });

  const resetState = () => {
    stopFlag = true;
    isRunning = false;
    results = [];
    mtrHops = [];
    activeTab = "ping";
    mode = "normal";
    ttl = 64;
    count = 10;
    size = 64;
    if (chartOption?.series) {
      chartOption.series[0].data = [];
      chartOption.series[1].data = [];
      chartOption.series[2].data = [];
    }
  };

  const changeMode = (m: PingMode) => {
    mode = m;
    if (m === "normal") {
      ttl = 64;
      count = 10;
      size = 64;
    } else if (m === "smoke") {
      ttl = 64;
      count = 2001; // 1 min
      size = 64;
    } else if (m === "trace") {
      ttl = -1;
      count = -1;
      size = 64;
    } else if (m === "mtr") {
      ttl = -2;
      count = 2001;
      size = 64;
    }
  };

  const initMainChart = async () => {
    await tick();
    if (!mainChartEl) return;
    if (chartInstance) chartInstance.dispose();
    chartInstance = echarts.init(mainChartEl, "dark");
    chartOption = getPingChartOption(pingChartLabels());
    chartInstance.setOption(chartOption);
  };

  const updateMainChart = (r: PingResult) => {
    if (!chartInstance || !chartOption) return;
    if (r.Stat === 1 || r.Stat === 4) {
      const t = new Date(r.TimeStamp * 1000);
      const ts = echarts.time.format(t, "{yyyy}/{MM}/{dd} {HH}:{mm}:{ss}", false);
      chartOption.series[0].data.push({
        ts,
        value: [t, r.Time / 1e9],
      });
      chartOption.series[1].data.push({
        ts,
        value: [t, r.SendTTL],
      });
      chartOption.series[2].data.push({
        ts,
        value: [t, r.RecvTTL],
      });
      chartInstance.setOption(chartOption);
      chartInstance.resize();
    }
  };

  const handleStart = async () => {
    if (!ip.trim() || isRunning) return;
    stopFlag = false;
    isRunning = true;
    results = [];
    mtrHops = [];
    activeTab = mode === "mtr" ? "mtr" : "ping";

    await initMainChart();

    if (mode === "mtr") {
      await runMtrProcess();
    } else {
      await runPingProcess();
    }
    isRunning = false;
  };

  const handleStop = () => {
    stopFlag = true;
    isRunning = false;
  };

  const runPingProcess = async () => {
    let sentCount = 0;
    let currentTtl = mode === "trace" ? 1 : ttl;
    let currentSize = size < 0 ? 64 : size;
    const startTime = Date.now();

    let maxDurationSec = 0;
    if (count === 2001) maxDurationSec = 60;
    if (count === 2003) maxDurationSec = 180;
    if (count === 2005) maxDurationSec = 300;
    if (count === 2010) maxDurationSec = 600;

    while (!stopFlag) {
      if (maxDurationSec > 0 && (Date.now() - startTime) / 1000 >= maxDurationSec) break;
      if (count > 0 && maxDurationSec === 0 && sentCount >= count) break;

      try {
        const res = await execPing(ip.trim(), currentSize, currentTtl);
        results = [res, ...results];
        updateMainChart(res);

        if (mode === "trace") {
          if (res.Stat === 1 || res.RecvSrc === ip.trim() || currentTtl >= 32) break;
          currentTtl++;
        }
        if (size < 0) {
          currentSize += 64;
          if (currentSize > 1500) currentSize = 64;
        }
      } catch (e) {
        // ignore single error and continue
      }

      sentCount++;
      await new Promise((r) => setTimeout(r, mode === "smoke" ? 200 : 1000));
    }
  };

  const runMtrProcess = async () => {
    // 1. Discovery phase
    const hops: (MTRHopStat & { history: number[]; lossCount: number })[] = [];
    let targetReached = false;

    for (let t = 1; t <= 32 && !stopFlag && !targetReached; t++) {
      try {
        const res = await execPing(ip.trim(), size < 0 ? 64 : size, t);
        results = [res, ...results];
        const recvIp = res.RecvSrc || "";
        const isSuccess = res.Stat === 1 || res.Stat === 4;
        const isTarget = res.Stat === 1 || recvIp === ip.trim();
        const rttMs = isSuccess ? res.Time / 1e6 : -1;

        const stat: MTRHopStat & { history: number[]; lossCount: number } = {
          ttl: t,
          ip: recvIp,
          loc: res.Loc || "",
          snt: 1,
          lossRate: isSuccess ? 0 : 100,
          last: rttMs,
          avg: rttMs,
          best: rttMs,
          wrst: rttMs,
          stDev: 0,
          isTarget: isTarget,
          history: isSuccess ? [rttMs] : [],
          lossCount: isSuccess ? 0 : 1,
        };
        hops.push(stat);
        mtrHops = [...hops];
        if (activeTab === "mtr") {
          await tick();
          showMtrProfileChart("mtrChartContainer", mtrHops, mtrChartLabels());
        }
        if (isTarget) {
          targetReached = true;
          break;
        }
      } catch (e) {
        hops.push({
          ttl: t,
          ip: "",
          loc: "",
          snt: 1,
          lossRate: 100,
          last: -1,
          avg: -1,
          best: -1,
          wrst: -1,
          stDev: 0,
          isTarget: false,
          history: [],
          lossCount: 1,
        });
        mtrHops = [...hops];
      }
      await new Promise((r) => setTimeout(r, 100));
    }

    // 2. Sampling phase
    let round = 1;
    const startTime = Date.now();
    let maxDurationSec = 0;
    if (count === 2001) maxDurationSec = 60;
    if (count === 2003) maxDurationSec = 180;
    if (count === 2005) maxDurationSec = 300;
    if (count === 2010) maxDurationSec = 600;

    while (!stopFlag) {
      if (maxDurationSec > 0 && (Date.now() - startTime) / 1000 >= maxDurationSec) break;
      if (count > 0 && maxDurationSec === 0 && round >= count) break;

      for (let i = 0; i < hops.length; i++) {
        if (stopFlag) break;
        const hop = hops[i];
        try {
          const res = await execPing(ip.trim(), size < 0 ? 64 : size, hop.ttl);
          results = [res, ...results];
          hop.snt++;
          if (res.Stat === 1 || res.Stat === 4) {
            const ms = res.Time / 1e6;
            hop.last = ms;
            hop.history.push(ms);
            if (hop.best < 0 || ms < hop.best) hop.best = ms;
            if (ms > hop.wrst) hop.wrst = ms;
            const sum = hop.history.reduce((a, b) => a + b, 0);
            hop.avg = sum / hop.history.length;
            if (hop.history.length > 1) {
              const mean = hop.avg;
              const variance = hop.history.reduce((acc, val) => acc + Math.pow(val - mean, 2), 0) / hop.history.length;
              hop.stDev = Math.sqrt(variance);
            } else {
              hop.stDev = 0;
            }
          } else {
            hop.lossCount++;
          }
          hop.lossRate = (hop.lossCount / hop.snt) * 100;
          if (res.RecvSrc && !hop.ip) {
            hop.ip = res.RecvSrc;
          }
          if (res.Loc && !hop.loc) {
            hop.loc = res.Loc;
          }
        } catch {
          hop.snt++;
          hop.lossCount++;
          hop.lossRate = (hop.lossCount / hop.snt) * 100;
        }
      }
      mtrHops = [...hops];
      if (activeTab === "mtr") {
        await tick();
        showMtrProfileChart("mtrChartContainer", mtrHops, mtrChartLabels());
      }
      round++;
      await new Promise((r) => setTimeout(r, 500));
    }
  };

  const handleTabSwitch = async (tab: ActiveTab) => {
    activeTab = tab;
    await tick();
    if (tab === "ping") {
      chartInstance?.resize();
    } else if (tab === "mtr") {
      showMtrProfileChart("mtrChartContainer", mtrHops, mtrChartLabels());
    } else if (tab === "histogram") {
      showPingHistogram("histogramContainer", results, $_("Ping.count"));
    } else if (tab === "smoke") {
      showPingSmokeChart("smokeContainer", results, {
        sequence: $_("Ping.sequence"),
        responseTime: $_("Ping.responseTimeMs"),
      });
    }
  };

  const renderStatText = (stat: number) => {
    switch (stat) {
      case 1:
        return { text: $_("Ping.statusOk"), color: "bg-emerald-500/20 text-emerald-400 border-emerald-500/30" };
      case 2:
        return { text: $_("Ping.statusTimeout"), color: "bg-rose-500/20 text-rose-400 border-rose-500/30" };
      case 3:
        return { text: $_("Ping.statusWarn"), color: "bg-amber-500/20 text-amber-400 border-amber-500/30" };
      case 4:
        return { text: $_("Ping.statusGateway"), color: "bg-sky-500/20 text-sky-400 border-sky-500/30" };
      default:
        return { text: $_("Ping.statusUnknown"), color: "bg-slate-500/20 text-slate-400 border-slate-500/30" };
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="PING"
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <Activity class="h-5 w-5 text-emerald-500" />
          <div>
            <h2 class="text-sm font-bold">PING — {titleName}</h2>
            <p class="text-[11px] font-mono text-slate-500 dark:text-slate-400">{ip || $_("Ping.noIp")}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <!-- Main Controls Toolbar matching TWSNMP FK -->
      <div class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/40 shrink-0">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex items-center gap-1.5">
            <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("Ping.Mode")}:</span>
            <div class="flex rounded-lg border border-slate-300 bg-white p-0.5 dark:border-slate-700 dark:bg-slate-950">
              <button
                type="button"
                onclick={() => changeMode("normal")}
                class={`flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${mode === "normal" ? "bg-cyan-600 text-white shadow-sm" : "text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white"}`}
              >
                <Activity class="h-3 w-3" />{$_("Ping.ModeNormal")}
              </button>
              <button
                type="button"
                onclick={() => changeMode("smoke")}
                class={`flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${mode === "smoke" ? "bg-cyan-600 text-white shadow-sm" : "text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white"}`}
              >
                <Cloud class="h-3 w-3" />{$_("Ping.ModeSmoke")}
              </button>
              <button
                type="button"
                onclick={() => changeMode("trace")}
                class={`flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${mode === "trace" ? "bg-cyan-600 text-white shadow-sm" : "text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white"}`}
              >
                <GitBranch class="h-3 w-3" />{$_("Ping.ModeTrace")}
              </button>
              <button
                type="button"
                onclick={() => changeMode("mtr")}
                class={`flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${mode === "mtr" ? "bg-cyan-600 text-white shadow-sm" : "text-slate-600 hover:text-slate-900 dark:text-slate-300 dark:hover:text-white"}`}
              >
                <Route class="h-3 w-3" />{$_("Ping.ModeMtr")}
              </button>
            </div>
          </div>

          <div class="flex items-center gap-2">
            {#if isRunning}
              <button
                type="button"
                onclick={handleStop}
                class="flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-1.5 text-xs font-semibold text-white shadow hover:bg-rose-500"
              >
                <Square class="h-3.5 w-3.5 fill-current" />{$_("Ping.Stop")}
              </button>
            {:else}
              <button
                type="button"
                onclick={handleStart}
                disabled={!ip.trim()}
                class="flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-1.5 text-xs font-semibold text-white shadow hover:bg-emerald-500 disabled:opacity-50"
              >
                <Play class="h-3.5 w-3.5 fill-current" />{$_("Ping.Start")}
              </button>
            {/if}
            <button
              type="button"
              onclick={resetState}
              disabled={isRunning}
              class="rounded-lg border border-slate-300 bg-white p-1.5 text-slate-600 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-300"
              title={$_("Ping.Reset")}
            >
              <RotateCcw class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-3">
          <input
            type="text"
            bind:value={ip}
            placeholder={$_("Ping.IPOrHost")}
            class="min-w-[160px] flex-1 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
          />

          {#if mode === "normal"}
            <select bind:value={count} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={10}>10 {$_("Ping.times")}</option>
              <option value={1}>1 {$_("Ping.times")}</option>
              <option value={3}>3 {$_("Ping.times")}</option>
              <option value={5}>5 {$_("Ping.times")}</option>
              <option value={20}>20 {$_("Ping.times")}</option>
              <option value={30}>30 {$_("Ping.times")}</option>
              <option value={50}>50 {$_("Ping.times")}</option>
              <option value={100}>100 {$_("Ping.times")}</option>
              <option value={-1}>{$_("Ping.unlimited")}</option>
            </select>
            <select bind:value={size} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={64}>64 {$_("Ping.bytes")}</option>
              <option value={128}>128 {$_("Ping.bytes")}</option>
              <option value={256}>256 {$_("Ping.bytes")}</option>
              <option value={512}>512 {$_("Ping.bytes")}</option>
              <option value={1024}>1024 {$_("Ping.bytes")}</option>
              <option value={1500}>1500 {$_("Ping.bytes")}</option>
              <option value={-1}>{$_("Ping.sizeAuto")}</option>
            </select>
            <select bind:value={ttl} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={64}>TTL 64</option>
              <option value={128}>TTL 128</option>
              <option value={254}>TTL 254</option>
              <option value={1}>TTL 1</option>
              <option value={2}>TTL 2</option>
              <option value={4}>TTL 4</option>
              <option value={8}>TTL 8</option>
              <option value={16}>TTL 16</option>
              <option value={32}>TTL 32</option>
            </select>
          {:else if mode === "smoke" || mode === "mtr"}
            <select bind:value={count} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={2001}>1 {$_("Ping.minutes")}</option>
              <option value={2003}>3 {$_("Ping.minutes")}</option>
              <option value={2005}>5 {$_("Ping.minutes")}</option>
              <option value={2010}>10 {$_("Ping.minutes")}</option>
              <option value={-1}>{$_("Ping.unlimited")}</option>
            </select>
            <select bind:value={size} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={64}>64 {$_("Ping.bytes")}</option>
              <option value={128}>128 {$_("Ping.bytes")}</option>
              <option value={256}>256 {$_("Ping.bytes")}</option>
              <option value={512}>512 {$_("Ping.bytes")}</option>
              <option value={1024}>1024 {$_("Ping.bytes")}</option>
              <option value={1500}>1500 {$_("Ping.bytes")}</option>
            </select>
          {:else if mode === "trace"}
            <select bind:value={size} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
              <option value={64}>64 {$_("Ping.bytes")}</option>
              <option value={128}>128 {$_("Ping.bytes")}</option>
              <option value={256}>256 {$_("Ping.bytes")}</option>
            </select>
          {/if}
        </div>
      </div>

      <!-- Navigation Tabs matching TWSNMP FK -->
      <div class="flex items-center gap-1 border-b border-slate-200 px-5 pt-2 text-xs font-semibold dark:border-slate-800 shrink-0">
        <button
          type="button"
          onclick={() => handleTabSwitch("ping")}
          class={`flex items-center gap-1.5 border-b-2 px-3 py-2 transition-colors ${activeTab === "ping" ? "border-cyan-500 text-cyan-600 dark:text-cyan-400" : "border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"}`}
        >
          <Activity class="h-3.5 w-3.5" />{$_("Ping.tabPing")}
        </button>
        {#if canShowMtr}
          <button
            type="button"
            onclick={() => handleTabSwitch("mtr")}
            class={`flex items-center gap-1.5 border-b-2 px-3 py-2 transition-colors ${activeTab === "mtr" ? "border-cyan-500 text-cyan-600 dark:text-cyan-400" : "border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"}`}
          >
            <Route class="h-3.5 w-3.5" />{$_("Ping.ModeMtr")}
          </button>
        {/if}
        {#if canShowStats}
          <button
            type="button"
            onclick={() => handleTabSwitch("smoke")}
            class={`flex items-center gap-1.5 border-b-2 px-3 py-2 transition-colors ${activeTab === "smoke" ? "border-cyan-500 text-cyan-600 dark:text-cyan-400" : "border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"}`}
          >
            <Cloud class="h-3.5 w-3.5" />{$_("Ping.tabSmoke")}
          </button>
          <button
            type="button"
            onclick={() => handleTabSwitch("histogram")}
            class={`flex items-center gap-1.5 border-b-2 px-3 py-2 transition-colors ${activeTab === "histogram" ? "border-cyan-500 text-cyan-600 dark:text-cyan-400" : "border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200"}`}
          >
            <BarChart3 class="h-3.5 w-3.5" />{$_("Ping.tabHistogram")}
          </button>
        {/if}
      </div>

      <!-- Tab Content Area -->
      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        {#if activeTab === "ping"}
          <div class="space-y-4">
            <div bind:this={mainChartEl} class="h-56 w-full rounded-xl border border-slate-200 bg-slate-900 p-2 dark:border-slate-800"></div>

            <!-- Ping Table with fixed max-height & scrollable tbody -->
            <div class="max-h-60 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800 shadow-inner">
              <table class="w-full text-left text-xs border-collapse">
                <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-900 shadow-sm">
                  <tr>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.result")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.time")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-mono font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.responseTime")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.size")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.sendTtl")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.recvTtl")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.source")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.location")}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
                  {#each results as r}
                    {@const stat = renderStatText(r.Stat)}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-900/60">
                      <td class="p-2">
                        <span class={`rounded border px-1.5 py-0.5 text-[10px] font-semibold ${stat.color}`}>{stat.text}</span>
                      </td>
                      <td class="p-2 text-slate-500">{new Date(r.TimeStamp * 1000).toLocaleTimeString()}</td>
                      <td class="p-2 font-mono font-medium">{r.Stat === 1 || r.Stat === 4 ? `${(r.Time / 1e6).toFixed(3)} ms` : "—"}</td>
                      <td class="p-2 text-slate-500">{r.Size} B</td>
                      <td class="p-2 font-mono text-slate-500">{r.SendTTL}</td>
                      <td class="p-2 font-mono text-slate-500">{r.RecvTTL || "—"}</td>
                      <td class="p-2 font-mono">{r.RecvSrc || "—"}</td>
                      <td class="p-2 text-slate-400">{r.Loc || "—"}</td>
                    </tr>
                  {:else}
                    <tr><td colspan="8" class="p-4 text-center text-slate-500">{$_("Ping.noData")}</td></tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {:else if activeTab === "mtr"}
          <div class="space-y-4">
            <!-- Hop Flow Cards -->
            <div class="flex items-center gap-2 overflow-x-auto rounded-xl border border-slate-200 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-900/60">
              {#each mtrHops as hop}
                <div class={`min-w-[140px] rounded-lg border p-2.5 text-xs shadow-sm ${hop.lossRate > 20 ? "border-rose-500/80 bg-rose-950/40 text-rose-200" : hop.lossRate > 0 ? "border-amber-500/80 bg-amber-950/40 text-amber-200" : "border-slate-300 bg-white text-slate-800 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"}`}>
                  <div class="flex items-center justify-between font-mono font-bold text-[10px]">
                    <span class="rounded bg-slate-200 px-1 py-0.5 text-slate-700 dark:bg-slate-800 dark:text-slate-300">{$_("Ping.hop")} {hop.ttl}</span>
                    {#if hop.isTarget}<span class="rounded bg-cyan-600 px-1 py-0.5 text-white">{$_("Ping.target")}</span>{/if}
                  </div>
                  <div class="mt-1.5 truncate font-mono font-semibold" title={hop.ip || $_("Ping.noResponse")}>
                    {hop.ip || `* ${$_("Ping.noResponse")} *`}
                  </div>
                  <div class="mt-1 flex justify-between text-[11px] text-slate-500 dark:text-slate-400">
                    <span>{$_("Ping.loss")}: <strong class={hop.lossRate > 0 ? "text-rose-400" : "text-emerald-400"}>{hop.lossRate.toFixed(1)}%</strong></span>
                    <span>{$_("Ping.average")}: {hop.avg >= 0 ? `${hop.avg.toFixed(1)}ms` : "—"}</span>
                  </div>
                </div>
              {/each}
            </div>

            <!-- MTR Table with fixed max-height & scrollable tbody -->
            <div class="max-h-56 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800 shadow-inner">
              <table class="w-full text-left text-xs border-collapse">
                <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-900 shadow-sm">
                  <tr>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.hop")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.hostIp")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.lossRate")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.sent")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.latest")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.average")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.minimum")}</th>
                    <th class="p-2 bg-slate-100 dark:bg-slate-900 font-semibold text-slate-700 dark:text-slate-200">{$_("Ping.maximum")}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
                  {#each mtrHops as hop}
                    <tr class="hover:bg-slate-50 dark:hover:bg-slate-900/60 font-mono">
                      <td class="p-2 font-bold">{hop.ttl}</td>
                      <td class="p-2">{hop.ip || "—"}</td>
                      <td class="p-2 font-bold {hop.lossRate > 0 ? 'text-rose-400' : 'text-emerald-400'}">{hop.lossRate.toFixed(1)}%</td>
                      <td class="p-2">{hop.snt}</td>
                      <td class="p-2">{hop.last >= 0 ? `${hop.last.toFixed(2)} ms` : "—"}</td>
                      <td class="p-2 font-semibold">{hop.avg >= 0 ? `${hop.avg.toFixed(2)} ms` : "—"}</td>
                      <td class="p-2">{hop.best >= 0 ? `${hop.best.toFixed(2)} ms` : "—"}</td>
                      <td class="p-2">{hop.wrst >= 0 ? `${hop.wrst.toFixed(2)} ms` : "—"}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <div id="mtrChartContainer" class="h-52 w-full rounded-xl border border-slate-200 bg-slate-900 p-2 dark:border-slate-800"></div>
          </div>
        {:else if activeTab === "smoke"}
          <div id="smokeContainer" class="h-80 w-full rounded-xl border border-slate-200 bg-slate-900 p-2 dark:border-slate-800"></div>
        {:else if activeTab === "histogram"}
          <div id="histogramContainer" class="h-80 w-full rounded-xl border border-slate-200 bg-slate-900 p-2 dark:border-slate-800"></div>
        {/if}
      </div>
    </div>
  </div>
{/if}
