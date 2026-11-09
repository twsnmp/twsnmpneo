<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import {
    fetchPublicDashboard,
    type PublicDashboardData,
    type PublicDashboardItemEnt,
    type PublicAlertState,
  } from "../api";
  import {
    initPublicMAP,
    updatePublicMAP,
    resetMap,
    zoom,
    resetZoom,
    ensureFontsReady,
  } from "../map/map";
  import * as echarts from "echarts";
  import {
    Activity,
    Network,
    RefreshCw,
    Maximize2,
    Minimize2,
    Download,
    ZoomIn,
    ZoomOut,
    RotateCcw,
    AlertTriangle,
    CheckCircle2,
    ShieldAlert,
    Cpu,
    Calendar,
    Radio,
    Clock,
    X,
    Volume2,
    VolumeX,
    Eye,
    Languages,
  } from "@lucide/svelte";
  import { _, locale } from "svelte-i18n";
  import { switchLocale, getSavedLocale, type SupportedLocale } from "../i18n";

  let { key = "" }: { key: string } = $props();

  let data = $state<PublicDashboardData | null>(null);
  let loading = $state(true);
  let errorMsg = $state("");
  let selectedItemId = $state<string>("");
  let autoRefreshInterval = $state<number>(30); // seconds, 0 = off
  let refreshTimer: any = null;
  let isFullscreen = $state(false);
  let isEmbed = $state(false);
  let currentLocale = $state<SupportedLocale>(getSavedLocale());

  const toggleLocale = () => {
    const next: SupportedLocale = currentLocale === "ja" ? "en" : "ja";
    currentLocale = next;
    switchLocale(next);
  };

  $effect(() => {
    if ($locale && ($locale === "ja" || $locale === "en") && $locale !== currentLocale) {
      currentLocale = $locale as SupportedLocale;
    }
  });

  // Drama Alert Overlay State
  let isAlertDismissed = $state(false);
  let alertCount = $state(1);
  let alertFirstTime = $state<string>("");
  let alertLastTime = $state<string>("");
  let prevAlertActive = false; // Plain variable to prevent Svelte 5 reactive loop
  let testAlertActive = $state(false);

  // Chart
  let chartDom: HTMLDivElement | null = $state(null);
  let chartInstance: echarts.ECharts | null = null;

  // Map container
  let mapContainer: HTMLDivElement | null = $state(null);
  let mapInitialized = false;

  // Web Audio Beep Alert
  class BeepPlayer {
    private static ctx: AudioContext | null = null;
    private static timer: any = null;

    static play(timeoutSec = 30) {
      this.stop();
      try {
        const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
        if (!AudioCtx) return;
        this.ctx = new AudioCtx();
        let ticks = 0;
        this.timer = setInterval(() => {
          if (!this.ctx) return;
          try {
            const osc = this.ctx.createOscillator();
            const gain = this.ctx.createGain();
            osc.connect(gain);
            gain.connect(this.ctx.destination);
            osc.type = "sine";
            osc.frequency.setValueAtTime(880, this.ctx.currentTime);
            gain.gain.setValueAtTime(0.12, this.ctx.currentTime);
            gain.gain.exponentialRampToValueAtTime(0.001, this.ctx.currentTime + 0.18);
            osc.start();
            osc.stop(this.ctx.currentTime + 0.2);
          } catch {}
          ticks++;
          if (ticks >= timeoutSec * 2) {
            this.stop();
          }
        }, 500);
      } catch (e) {
        console.warn("BeepPlayer audio init failed:", e);
      }
    }

    static stop() {
      if (this.timer) {
        clearInterval(this.timer);
        this.timer = null;
      }
      if (this.ctx) {
        try { this.ctx.close(); } catch {}
        this.ctx = null;
      }
    }
  }

  const selectedItem = $derived.by(() => {
    if (!data || !data.items || data.items.length === 0) return null;
    return data.items.find((i) => i.id === selectedItemId) || data.items[0];
  });

  const selectedPolling = $derived.by(() => {
    if (!data || !data.pollings || !selectedItem || selectedItem.type !== "polling") return null;
    const pid = selectedItem.pollingId || (selectedItem as any).PollingID || "";
    if (!pid) return null;
    return (
      data.pollings[pid] ||
      Object.values(data.pollings).find((p: any) => (p.ID || p.id) === pid) ||
      null
    );
  });

  const pollingStateStr = $derived(
    (selectedPolling?.State || (selectedPolling as any)?.state || "unknown").toLowerCase()
  );

  const isGlobalCritical = $derived(
    Boolean((data?.summaryKpi?.nodeHigh || 0) > 0 || (data?.summaryKpi?.pollingHigh || 0) > 0)
  );

  const isGlobalWarning = $derived(
    Boolean(
      !isGlobalCritical &&
        ((data?.summaryKpi?.nodeLow || 0) > 0 ||
          (data?.summaryKpi?.nodeWarn || 0) > 0 ||
          (data?.summaryKpi?.pollingLow || 0) > 0 ||
          (data?.summaryKpi?.pollingWarn || 0) > 0)
    )
  );

  const activeAlert = $derived.by(() => {
    if (testAlertActive) {
      return {
        isActive: true,
        nodeName: selectedPolling?.NodeName || (selectedPolling as any)?.nodeName || "Core-Switch-01",
        targetName: selectedItem?.title || $_("publicDashboard.incidentTarget"),
        state: "high",
        valueText: "Threshold exceeded / Connection Down",
        message: $_("publicDashboard.defaultAlertMsg"),
        lastTime: Date.now() * 1000000,
      } as PublicAlertState;
    }
    if (!selectedItem || !selectedItem.alert || !selectedItem.alert.enable) {
      return null;
    }
    // Check item-specific alert from backend
    if (data?.itemAlerts && data.itemAlerts[selectedItem.id]) {
      return data.itemAlerts[selectedItem.id];
    }
    // Fallback: evaluate client-side for polling
    if (selectedItem.type === "polling" && selectedPolling) {
      const st = (selectedPolling.State || (selectedPolling as any).state || "unknown").toLowerCase();
      const isTrigger = selectedItem.alert.level === "high"
        ? st === "high"
        : (st === "high" || st === "low" || st === "warn");
      if (isTrigger) {
        let valStr = "";
        if (selectedPolling.Result) {
          for (const [k, v] of Object.entries(selectedPolling.Result)) {
            valStr = `${k}: ${v}`;
            break;
          }
        }
        return {
          isActive: true,
          nodeName: selectedPolling.NodeName || (selectedPolling as any).nodeName || selectedPolling.NodeID || (selectedPolling as any).node_id,
          targetName: selectedPolling.Name || (selectedPolling as any).name,
          state: selectedPolling.State || (selectedPolling as any).state,
          valueText: valStr,
          message: $_("publicDashboard.pollingAlertMsg", {
            values: {
              name: selectedPolling.Name || (selectedPolling as any).name,
              state: selectedPolling.State || (selectedPolling as any).state,
            },
          }),
          lastTime: selectedPolling.LastTime || Date.now() * 1000000,
        };
      }
    }
    return null;
  });

  const isAlertVisible = $derived(
    activeAlert?.isActive && !isAlertDismissed
  );

  $effect(() => {
    // When activeAlert changes for current item, manage sound and counts
    if (activeAlert?.isActive && !isAlertDismissed) {
      const timeStr = formatTimestamp(activeAlert.lastTime);
      if (!prevAlertActive) {
        alertCount = 1;
        alertFirstTime = timeStr;
        alertLastTime = timeStr;
        if (selectedItem?.alert?.sound) {
          BeepPlayer.play(selectedItem.alert.soundTimeout || 30);
        }
      } else {
        alertLastTime = timeStr;
      }
      prevAlertActive = true;
    } else {
      prevAlertActive = false;
      if (!testAlertActive) {
        BeepPlayer.stop();
      }
    }
  });

  const formatTimestamp = (nsOrSec?: number): string => {
    if (!nsOrSec) return "-";
    const ms = nsOrSec > 1e12 ? Math.floor(nsOrSec / 1e6) : nsOrSec * 1000;
    const d = new Date(ms);
    const isJa = ($locale || "ja").startsWith("ja");
    return d.toLocaleString(isJa ? "ja-JP" : "en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  };

  let isFetching = false;

  const loadData = async () => {
    if (!key) {
      errorMsg = $_("publicDashboard.noKeyError");
      loading = false;
      return;
    }
    if (isFetching) {
      return;
    }
    isFetching = true;
    const t0 = performance.now();
    console.log(`[PublicDashboard View] Updating dashboard data at ${new Date().toLocaleTimeString()}...`);
    try {
      const res = await fetchPublicDashboard(key);
      data = res;
      errorMsg = "";

      // Default item selection
      if (!selectedItemId && res.items && res.items.length > 0) {
        selectedItemId = res.items[0].id;
      }
      console.log(`[PublicDashboard View] View updated in ${Math.round(performance.now() - t0)}ms`);
    } catch (e: any) {
      console.error(`[PublicDashboard View] Load error after ${Math.round(performance.now() - t0)}ms:`, e);
      errorMsg = e.message || $_("publicDashboard.loadError");
    } finally {
      isFetching = false;
      loading = false;
    }
  };

  $effect(() => {
    if (mapContainer && selectedItem?.type === "map" && data?.map) {
      renderMap(data.map);
    }
  });

  $effect(() => {
    // Re-render chart on item selection, data change, or locale change
    const _loc = $locale;
    if (chartDom && selectedItem?.type === "polling" && selectedPolling) {
      renderPollingChart();
    }
    return () => {
      if (chartInstance) {
        try {
          chartInstance.dispose();
        } catch {}
        chartInstance = null;
      }
    };
  });

  const renderMap = (mapData: any) => {
    if (!mapContainer || !mapData) return;
    try {
      if (!mapInitialized) {
        mapInitialized = true;
        initPublicMAP(mapContainer, mapData, key);
      } else {
        updatePublicMAP(mapData, key);
      }
    } catch (err) {
      console.error("renderMap failed:", err);
    }
  };

  const renderPollingChart = () => {
    if (!chartDom || !selectedPolling) return;
    try {
      // If chartInstance is bound to a detached or different DOM, dispose and recreate
      if (chartInstance) {
        if (chartInstance.getDom() !== chartDom || chartInstance.isDisposed()) {
          try {
            chartInstance.dispose();
          } catch {}
          chartInstance = null;
        }
      }
      if (!chartInstance) {
        chartInstance = echarts.init(chartDom);
      }

      const logs = selectedPolling.Logs || [];
      // Extract primary numeric metric from results
      const seriesData: [number, number][] = [];
      let detectedKey = "";

      for (const log of logs) {
        if (!log.Result || typeof log.Result !== "object") continue;
        const t = log.Time > 1e12 ? Math.floor(log.Time / 1e6) : log.Time * 1000;
        if (!detectedKey) {
          for (const [k, v] of Object.entries(log.Result)) {
            if (typeof v === "number" && !isNaN(v)) {
              detectedKey = k;
              break;
            }
          }
        }
        if (detectedKey && typeof log.Result[detectedKey] === "number") {
          seriesData.push([t, log.Result[detectedKey]]);
        }
      }

      const metricTitle = detectedKey || "Value";
      const rawState = selectedPolling.State || (selectedPolling as any).state || "normal";
      const isStateAlarm = ["high", "low", "warn"].includes(rawState.toLowerCase());
      const lineColor = isStateAlarm ? "#ef4444" : "#00d2ff";
      const areaColorFrom = isStateAlarm ? "rgba(239, 68, 68, 0.45)" : "rgba(0, 210, 255, 0.4)";
      const areaColorTo = isStateAlarm ? "rgba(239, 68, 68, 0.0)" : "rgba(0, 210, 255, 0.0)";

      const option: echarts.EChartsOption = {
        backgroundColor: "transparent",
        title: seriesData.length === 0 ? {
          text: $_("publicDashboard.collectingData"),
          left: "center",
          top: "center",
          textStyle: { color: "rgba(255, 255, 255, 0.35)", fontSize: 13, fontWeight: "normal" },
        } : undefined,
        tooltip: {
          trigger: "axis",
          backgroundColor: "rgba(10, 15, 30, 0.95)",
          borderColor: "rgba(255, 255, 255, 0.15)",
          textStyle: { color: "#f8fafc", fontSize: 11 },
          valueFormatter: (val: any) => (val !== undefined && val !== null ? `${val} (${metricTitle})` : "-"),
        },
        grid: {
          left: 45,
          right: 20,
          top: 25,
          bottom: 30,
          containLabel: false,
        },
        xAxis: {
          type: "time",
          axisLine: { lineStyle: { color: "rgba(255, 255, 255, 0.2)" } },
          axisLabel: { color: "rgba(255, 255, 255, 0.6)", fontSize: 10 },
          splitLine: { show: false },
        },
        yAxis: {
          type: "value",
          axisLine: { lineStyle: { color: "rgba(255, 255, 255, 0.2)" } },
          axisLabel: { color: "rgba(255, 255, 255, 0.6)", fontSize: 10 },
          splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.06)" } },
        },
        series: [
          {
            name: metricTitle,
            type: "line",
            smooth: true,
            showSymbol: false,
            data: seriesData,
            lineStyle: { width: 2.5, color: lineColor },
            areaStyle: {
              color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: areaColorFrom },
                { offset: 1, color: areaColorTo },
              ]),
            },
          },
        ],
      };

      chartInstance.setOption(option, true);
      requestAnimationFrame(() => {
        chartInstance?.resize();
      });
    } catch (err) {
      console.error("renderPollingChart failed:", err);
    }
  };

  const handleSelectItem = async (id: string) => {
    if (selectedItemId === id) return;
    if (selectedItem?.type === "map") {
      resetMap();
      mapInitialized = false;
    } else if (selectedItem?.type === "polling") {
      if (chartInstance) {
        try {
          chartInstance.dispose();
        } catch {}
        chartInstance = null;
      }
    }
    selectedItemId = id;
    isAlertDismissed = false;
    prevAlertActive = false;
    await tick();
    if (selectedItem?.type === "map" && data?.map) {
      renderMap(data.map);
    } else if (selectedItem?.type === "polling") {
      // Allow tick and requestAnimationFrame for DOM layout stabilization
      requestAnimationFrame(() => {
        renderPollingChart();
      });
    }
  };

  const dismissAlert = () => {
    isAlertDismissed = true;
    testAlertActive = false;
    BeepPlayer.stop();
  };

  const triggerTestAlert = () => {
    testAlertActive = true;
    isAlertDismissed = false;
    alertCount = 1;
    alertFirstTime = formatTimestamp(Date.now() * 1000000);
    alertLastTime = alertFirstTime;
    if (data?.alertConf?.sound) {
      BeepPlayer.play(data.alertConf.soundTimeout || 30);
    }
  };

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().then(() => (isFullscreen = true)).catch(() => {});
    } else {
      document.exitFullscreen().then(() => (isFullscreen = false)).catch(() => {});
    }
  };

  const downloadMapImage = () => {
    const canvas = mapContainer?.querySelector("canvas");
    if (!canvas) return;
    const a = document.createElement("a");
    a.download = `${(data?.mapName || "Map").replace(/\s+/g, "_")}.png`;
    a.href = canvas.toDataURL("image/png");
    a.click();
  };

  const setupRefreshTimer = () => {
    if (refreshTimer) clearInterval(refreshTimer);
    if (autoRefreshInterval > 0) {
      refreshTimer = setInterval(loadData, autoRefreshInterval * 1000);
    }
  };

  $effect(() => {
    if (autoRefreshInterval >= 0) {
      setupRefreshTimer();
    }
  });

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    isEmbed = params.get("embed") === "true";

    ensureFontsReady();
    loadData();

    const handleResize = () => {
      chartInstance?.resize();
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      if (refreshTimer) clearInterval(refreshTimer);
      BeepPlayer.stop();
      chartInstance?.dispose();
      resetMap();
    };
  });
</script>

<div class="public-dashboard-container relative w-screen h-screen overflow-hidden flex flex-col bg-[#070b14] text-slate-100 select-none font-sans">
  <!-- Top Navigation Bar -->
  <header class="flex items-center justify-between px-4 py-2 border-b border-slate-800/80 bg-slate-900/60 backdrop-blur-md shrink-0 {isEmbed ? 'h-10 px-2.5' : 'h-13'}">
    <!-- Left: Title & Status Badge -->
    <div class="flex items-center gap-3">
      <div class="flex items-center gap-2">
        <div class="h-2.5 w-2.5 rounded-full {isGlobalCritical ? 'bg-rose-500 animate-ping' : isGlobalWarning ? 'bg-amber-500' : 'bg-emerald-400 shadow-[0_0_8px_#34d399]'}"></div>
        <h1 class="font-extrabold tracking-tight text-white {isEmbed ? 'text-sm' : 'text-base sm:text-lg'}">
          {data?.mapName || $_("publicDashboard.title")}
        </h1>
      </div>

      <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[10px] font-bold tracking-wider uppercase border {isGlobalCritical ? 'bg-rose-500/10 text-rose-400 border-rose-500/30' : isGlobalWarning ? 'bg-amber-500/10 text-amber-400 border-amber-500/30' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30'}">
        <span class="h-1.5 w-1.5 rounded-full {isGlobalCritical ? 'bg-rose-500' : isGlobalWarning ? 'bg-amber-400' : 'bg-emerald-400'}"></span>
        {isGlobalCritical ? $_("publicDashboard.critical") : isGlobalWarning ? $_("publicDashboard.warning") : $_("publicDashboard.operational")}
      </span>
    </div>

    <!-- Right Controls -->
    <div class="flex items-center gap-2 text-xs">
      {#if !isEmbed}
        <span class="hidden sm:inline text-[11px] text-slate-400 font-mono">
          {formatTimestamp(data?.lastUpdate)}
        </span>
      {/if}

      <!-- Language Switcher -->
      <button
        type="button"
        onclick={toggleLocale}
        title={$_("publicDashboard.language")}
        class="flex h-7 items-center gap-1 px-2 rounded-lg border border-slate-700/80 bg-slate-800/80 hover:bg-slate-700 text-slate-200 hover:text-white transition-colors cursor-pointer text-xs font-semibold"
      >
        <Languages class="w-3.5 h-3.5 text-cyan-400" />
        <span>{currentLocale.toUpperCase()}</span>
      </button>

      <!-- Auto Refresh Select -->
      <select
        bind:value={autoRefreshInterval}
        class="bg-slate-800/80 border border-slate-700/80 rounded-lg px-2 py-1 text-[11px] text-slate-200 outline-none focus:border-cyan-500"
      >
        <option value={0}>{$_("publicDashboard.autoRefreshOff")}</option>
        <option value={10}>{$_("publicDashboard.sec", { values: { n: 10 } })}</option>
        <option value={30}>{$_("publicDashboard.sec", { values: { n: 30 } })}</option>
        <option value={60}>{$_("publicDashboard.sec", { values: { n: 60 } })}</option>
      </select>

      <button
        type="button"
        onclick={loadData}
        title={$_("publicDashboard.refresh")}
        class="p-1.5 rounded-lg border border-slate-700/80 bg-slate-800/80 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors cursor-pointer"
      >
        <RefreshCw class="w-3.5 h-3.5 {loading ? 'animate-spin text-cyan-400' : ''}" />
      </button>

      {#if !isEmbed}
        <button
          type="button"
          onclick={triggerTestAlert}
          title={$_("publicDashboard.testAlert")}
          class="p-1.5 rounded-lg border border-rose-500/40 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 transition-colors cursor-pointer"
        >
          <ShieldAlert class="w-3.5 h-3.5" />
        </button>

        <button
          type="button"
          onclick={toggleFullscreen}
          title={isFullscreen ? $_("publicDashboard.exitFullscreen") : $_("publicDashboard.fullscreen")}
          class="p-1.5 rounded-lg border border-slate-700/80 bg-slate-800/80 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors cursor-pointer"
        >
          {#if isFullscreen}
            <Minimize2 class="w-3.5 h-3.5" />
          {:else}
            <Maximize2 class="w-3.5 h-3.5" />
          {/if}
        </button>
      {/if}
    </div>
  </header>

  <!-- Clean & Structured Top KPI Container (Nodes, Pollings, 24H Logs, AI Anomaly) -->
  {#if data?.summaryKpi}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2.5 p-3 bg-slate-950/60 border-b border-slate-800/80 shrink-0">
      <!-- 1. Node Status Level KPI -->
      <div class="relative overflow-hidden rounded-xl border border-slate-800 bg-slate-900/70 p-2.5 flex flex-col justify-between shadow-sm">
        <div class="flex items-center justify-between border-b border-slate-800/60 pb-1.5 mb-1.5">
          <div class="flex items-center gap-1.5">
            <Network class="w-4 h-4 text-cyan-400" />
            <span class="text-xs font-bold text-slate-200">{$_("publicDashboard.nodeStatus")}</span>
          </div>
          <span class="text-sm font-mono font-extrabold text-cyan-300">
            {data.summaryKpi.nodeTotal} <span class="text-[10px] text-slate-400 font-normal">{$_("publicDashboard.unitNodes")}</span>
          </span>
        </div>
        <!-- Level Breakdown Pills: grid-cols-5 gives balanced spacing and larger badge items -->
        <div class="grid grid-cols-5 gap-1.5 text-xs font-mono">
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300" title={$_("publicDashboard.high")}>
            <span class="h-2 w-2 rounded-full bg-rose-500 shrink-0 {data.summaryKpi.nodeHigh > 0 ? 'animate-pulse' : ''}"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.nodeHigh}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-orange-500/15 border border-orange-500/30 text-orange-300" title={$_("publicDashboard.low")}>
            <span class="h-2 w-2 rounded-full bg-orange-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.nodeLow}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-amber-500/15 border border-amber-500/30 text-amber-300" title={$_("publicDashboard.warn")}>
            <span class="h-2 w-2 rounded-full bg-amber-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.nodeWarn}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-blue-500/15 border border-blue-500/30 text-blue-300" title={$_("publicDashboard.repair")}>
            <span class="h-2 w-2 rounded-full bg-blue-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.nodeRepair || 0}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-emerald-500/15 border border-emerald-500/30 text-emerald-300" title={$_("publicDashboard.normal")}>
            <span class="h-2 w-2 rounded-full bg-emerald-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.nodeNormal}</span>
          </div>
        </div>
      </div>

      <!-- 2. Polling Status Level KPI -->
      <div class="relative overflow-hidden rounded-xl border border-slate-800 bg-slate-900/70 p-2.5 flex flex-col justify-between shadow-sm">
        <div class="flex items-center justify-between border-b border-slate-800/60 pb-1.5 mb-1.5">
          <div class="flex items-center gap-1.5">
            <Activity class="w-4 h-4 text-blue-400" />
            <span class="text-xs font-bold text-slate-200">{$_("publicDashboard.pollingStatus")}</span>
          </div>
          <span class="text-sm font-mono font-extrabold text-blue-300">
            {data.summaryKpi.pollingTotal} <span class="text-[10px] text-slate-400 font-normal">{$_("publicDashboard.unitItems")}</span>
          </span>
        </div>
        <!-- Level Breakdown Pills: grid-cols-5 gives balanced spacing and larger badge items -->
        <div class="grid grid-cols-5 gap-1.5 text-xs font-mono">
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300" title={$_("publicDashboard.high")}>
            <span class="h-2 w-2 rounded-full bg-rose-500 shrink-0 {data.summaryKpi.pollingHigh > 0 ? 'animate-pulse' : ''}"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.pollingHigh}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-orange-500/15 border border-orange-500/30 text-orange-300" title={$_("publicDashboard.low")}>
            <span class="h-2 w-2 rounded-full bg-orange-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.pollingLow}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-amber-500/15 border border-amber-500/30 text-amber-300" title={$_("publicDashboard.warn")}>
            <span class="h-2 w-2 rounded-full bg-amber-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.pollingWarn}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-blue-500/15 border border-blue-500/30 text-blue-300" title={$_("publicDashboard.repair")}>
            <span class="h-2 w-2 rounded-full bg-blue-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.pollingRepair || 0}</span>
          </div>
          <div class="flex items-center justify-center gap-1.5 py-1 px-1 rounded-lg bg-emerald-500/15 border border-emerald-500/30 text-emerald-300" title={$_("publicDashboard.normal")}>
            <span class="h-2 w-2 rounded-full bg-emerald-400 shrink-0"></span>
            <span class="text-xs sm:text-sm font-bold font-mono">{data.summaryKpi.pollingNormal}</span>
          </div>
        </div>
      </div>

      <!-- 3. 24H Received Logs KPI -->
      <div class="relative overflow-hidden rounded-xl border border-slate-800 bg-slate-900/70 p-2.5 flex flex-col justify-between shadow-sm">
        <div class="flex items-center justify-between border-b border-slate-800/60 pb-1.5 mb-1.5">
          <div class="flex items-center gap-1.5">
            <Calendar class="w-4 h-4 text-indigo-400" />
            <span class="text-xs font-bold text-slate-200">{$_("publicDashboard.log24h")}</span>
          </div>
          <span class="text-sm font-mono font-extrabold text-indigo-300">
            {data.summaryKpi.logEventCount + (data.summaryKpi.logSyslogCount || 0) + (data.summaryKpi.logTrapCount || 0)} <span class="text-[10px] text-slate-400 font-normal">{$_("publicDashboard.unitItems")}</span>
          </span>
        </div>
        <div class="grid grid-cols-3 gap-1.5 text-xs font-mono text-center">
          <div class="py-1 px-1 rounded-lg bg-slate-800/60 border border-slate-700/50" title={$_("publicDashboard.logEvent")}>
            <span class="text-[10px] text-slate-400 block truncate">{$_("publicDashboard.logEvent")}</span>
            <span class="text-xs sm:text-sm font-bold text-slate-200">{data.summaryKpi.logEventCount}</span>
          </div>
          <div class="py-1 px-1 rounded-lg bg-slate-800/60 border border-slate-700/50" title={$_("publicDashboard.logSyslog")}>
            <span class="text-[10px] text-slate-400 block truncate">{$_("publicDashboard.logSyslog")}</span>
            <span class="text-xs sm:text-sm font-bold text-slate-200">{data.summaryKpi.logSyslogCount || 0}</span>
          </div>
          <div class="py-1 px-1 rounded-lg bg-slate-800/60 border border-slate-700/50" title={$_("publicDashboard.logTrap")}>
            <span class="text-[10px] text-slate-400 block truncate">{$_("publicDashboard.logTrap")}</span>
            <span class="text-xs sm:text-sm font-bold text-slate-200">{data.summaryKpi.logTrapCount || 0}</span>
          </div>
        </div>
      </div>

      <!-- 4. AI Anomaly Detection KPI -->
      <div class="relative overflow-hidden rounded-xl border border-slate-800 bg-slate-900/70 p-2.5 flex flex-col justify-between shadow-sm">
        <div class="flex items-center justify-between border-b border-slate-800/60 pb-1.5 mb-1.5">
          <div class="flex items-center gap-1.5">
            <Cpu class="w-4 h-4 text-purple-400" />
            <span class="text-xs font-bold text-slate-200">{$_("publicDashboard.aiAnomaly")}</span>
          </div>
          <span class="text-[10px] font-mono text-slate-400">
            {$_("publicDashboard.maxScore")} <span class="font-bold text-purple-300">{data.summaryKpi.aiMaxScore ? data.summaryKpi.aiMaxScore.toFixed(0) : 0}</span>
          </span>
        </div>
        <div class="flex items-center justify-between py-0.5">
          <div class="flex items-baseline gap-1.5">
            <span class="text-xl font-extrabold font-mono {data.summaryKpi.aiAnomalyCount > 0 ? 'text-purple-300' : 'text-slate-400'}">
              {data.summaryKpi.aiAnomalyCount}
            </span>
            <span class="text-[10px] text-slate-400 font-mono">{$_("publicDashboard.detectedCount")}</span>
          </div>
          <span class="px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider {data.summaryKpi.aiAnomalyCount > 0 ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40' : 'bg-slate-800 text-slate-400'}">
            {data.summaryKpi.aiAnomalyCount > 0 ? $_("publicDashboard.anomaly") : $_("publicDashboard.clear")}
          </span>
        </div>
      </div>
    </div>
  {/if}

  <!-- Items Selector Tabs (When multiple items registered) -->
  {#if data?.items && data.items.length > 1}
    <div class="flex items-center gap-1.5 px-3 py-1.5 border-b border-slate-800/60 bg-slate-900/40 overflow-x-auto shrink-0">
      {#each data.items as item}
        <button
          type="button"
          onclick={() => handleSelectItem(item.id)}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition-all cursor-pointer {selectedItem?.id === item.id ? 'bg-cyan-500/20 text-cyan-300 border border-cyan-500/40 shadow-sm' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50 border border-transparent'}"
        >
          {#if item.type === 'map'}
            <Network class="w-3.5 h-3.5" />
          {:else}
            <Activity class="w-3.5 h-3.5" />
          {/if}
          <span>{item.title || (item.type === 'map' ? $_("publicDashboard.networkMap") : $_("publicDashboard.polling"))}</span>
        </button>
      {/each}
    </div>
  {/if}

  <!-- Main Content Area -->
  <main class="flex-1 min-h-0 relative flex flex-col overflow-hidden">
    {#if errorMsg}
      <div class="m-6 p-4 rounded-xl border border-rose-500/40 bg-rose-950/30 text-rose-300 text-xs flex items-center gap-2">
        <AlertTriangle class="w-5 h-5 text-rose-400 shrink-0" />
        <span>{errorMsg}</span>
      </div>
    {:else if loading && !data}
      <div class="flex-1 flex items-center justify-center text-slate-400 text-xs gap-2">
        <RefreshCw class="w-5 h-5 animate-spin text-cyan-400" />
        <span>{$_("publicDashboard.loadingDashboard")}</span>
      </div>
    {:else if selectedItem?.type === 'map'}
      <!-- Map Display View -->
      <div class="relative w-full h-full overflow-hidden flex flex-col">
        <!-- Floating Zoom & Download Toolbar -->
        <div class="absolute right-4 top-4 z-20 flex items-center gap-1 p-1 bg-slate-900/80 backdrop-blur-md rounded-xl border border-slate-700/80 shadow-xl">
          <button
            type="button"
            onclick={() => zoom(false)}
            title={$_("publicDashboard.zoomOut")}
            class="p-1.5 hover:bg-slate-800 rounded-lg text-slate-300 hover:text-white transition-colors cursor-pointer"
          >
            <ZoomOut class="w-4 h-4" />
          </button>
          <button
            type="button"
            onclick={() => resetZoom()}
            title={$_("publicDashboard.resetZoom")}
            class="p-1.5 hover:bg-slate-800 rounded-lg text-slate-300 hover:text-white transition-colors cursor-pointer"
          >
            <RotateCcw class="w-4 h-4" />
          </button>
          <button
            type="button"
            onclick={() => zoom(true)}
            title={$_("publicDashboard.zoomIn")}
            class="p-1.5 hover:bg-slate-800 rounded-lg text-slate-300 hover:text-white transition-colors cursor-pointer"
          >
            <ZoomIn class="w-4 h-4" />
          </button>
          <div class="w-px h-4 bg-slate-700 my-auto mx-0.5"></div>
          <button
            type="button"
            onclick={downloadMapImage}
            title={$_("publicDashboard.downloadPng")}
            class="p-1.5 hover:bg-slate-800 rounded-lg text-cyan-400 hover:text-cyan-300 transition-colors cursor-pointer"
          >
            <Download class="w-4 h-4" />
          </button>
        </div>

        <div bind:this={mapContainer} class="w-full h-full overflow-auto cursor-grab active:cursor-grabbing"></div>
      </div>
    {:else if selectedItem?.type === 'polling'}
      {#if selectedPolling}
        <!-- Polling Detail View (twFathom Style) -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <!-- Polling Top KPI Cards -->
          <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-3 shadow-sm">
              <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">{$_("publicDashboard.monitoredNode")}</span>
              <span class="text-sm font-bold text-cyan-400 mt-1 block truncate">
                {selectedPolling.NodeName || (selectedPolling as any).nodeName || selectedPolling.NodeID || (selectedPolling as any).node_id || '-'}
              </span>
            </div>

            <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-3 shadow-sm">
              <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">{$_("publicDashboard.currentState")}</span>
              <div class="flex items-center gap-1.5 mt-1">
                <span class="h-2 w-2 rounded-full {pollingStateStr === 'normal' ? 'bg-emerald-400 shadow-[0_0_8px_#34d399]' : pollingStateStr === 'repair' ? 'bg-blue-400 shadow-[0_0_8px_#60a5fa]' : pollingStateStr === 'warn' ? 'bg-amber-400' : 'bg-rose-500'}"></span>
                <span class="text-sm font-bold uppercase {pollingStateStr === 'normal' ? 'text-emerald-400' : pollingStateStr === 'repair' ? 'text-blue-400' : pollingStateStr === 'warn' ? 'text-amber-400' : 'text-rose-400'}">
                  {pollingStateStr}
                </span>
              </div>
            </div>

            <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-3 shadow-sm">
              <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">{$_("publicDashboard.typeMode")}</span>
              <span class="text-sm font-bold text-slate-200 mt-1 block uppercase">
                {selectedPolling.Type || (selectedPolling as any).type || '-'} {(selectedPolling.Mode || (selectedPolling as any).mode) ? `(${selectedPolling.Mode || (selectedPolling as any).mode})` : ''}
              </span>
            </div>

            <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-3 shadow-sm">
              <span class="text-[10px] font-bold text-slate-400 uppercase tracking-wider block">{$_("publicDashboard.lastTime")}</span>
              <span class="text-xs font-mono text-slate-300 mt-1 block">
                {formatTimestamp(selectedPolling.LastTime)}
              </span>
            </div>
          </div>

          <!-- ECharts Time-Series Area Chart -->
          <div class="rounded-2xl border border-slate-800 bg-slate-900/70 p-4 shadow-lg flex flex-col">
            <div class="flex items-center justify-between border-b border-slate-800/80 pb-2.5 mb-3">
              <div class="flex items-center gap-2">
                <Activity class="w-4 h-4 text-cyan-400" />
                <h2 class="text-xs font-bold uppercase tracking-wider text-slate-200">
                  {$_("publicDashboard.timeSeriesChart")} ({selectedPolling.Name || (selectedPolling as any).name || 'Metric'})
                </h2>
              </div>
              <span class="text-[10px] text-slate-400 font-mono">
                {$_("publicDashboard.pastLogs", { values: { n: selectedPolling.Logs?.length || 0 } })}
              </span>
            </div>
            <div bind:this={chartDom} class="w-full h-64 sm:h-80 min-h-[260px]"></div>
          </div>

          <!-- Latest Result Key-Value Table -->
          {#if selectedPolling.Result && typeof selectedPolling.Result === 'object' && Object.keys(selectedPolling.Result).length > 0}
            <div class="rounded-2xl border border-slate-800 bg-slate-900/70 overflow-hidden shadow-lg">
              <div class="px-4 py-2.5 border-b border-slate-800/80 bg-slate-950/40 text-xs font-bold text-slate-300 flex items-center gap-2">
                <Cpu class="w-4 h-4 text-cyan-400" />
                <span>{$_("publicDashboard.latestMetrics")}</span>
              </div>
              <div class="p-3 overflow-x-auto">
                <table class="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr class="border-b border-slate-800 text-[10px] uppercase text-slate-400">
                      <th class="py-1.5 px-3 w-1/3">{$_("publicDashboard.paramName")}</th>
                      <th class="py-1.5 px-3">{$_("publicDashboard.metricValue")}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800/60 font-mono text-[11px]">
                    {#each Object.entries(selectedPolling.Result) as [k, v]}
                      <tr class="hover:bg-slate-800/30">
                        <td class="py-2 px-3 text-slate-400 font-semibold">{k}</td>
                        <td class="py-2 px-3 text-slate-100 font-bold">
                          {typeof v === 'object' ? JSON.stringify(v) : String(v)}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          {/if}
        </div>
      {:else}
        <div class="flex-1 flex flex-col items-center justify-center text-slate-400 text-xs gap-3 p-6 text-center">
          <Activity class="w-8 h-8 text-cyan-500/60 animate-pulse" />
          <div class="font-bold text-slate-300">
            {$_("publicDashboard.loadingPolling")}
          </div>
          <div class="text-[11px] text-slate-500">
            {$_("publicDashboard.loadingPollingDesc", { values: { title: selectedItem.title || $_("publicDashboard.polling") } })}
          </div>
        </div>
      {/if}
    {:else}
      <div class="flex-1 flex items-center justify-center text-slate-400 text-xs">
        {$_("publicDashboard.noItemSelected")}
      </div>
    {/if}
  </main>

  <!-- twFathom Style Drama Alert Overlay -->
  {#if isAlertVisible && activeAlert}
    <div class="drama-alert-overlay fixed inset-0 z-[99999] flex flex-col items-center justify-center p-6 backdrop-blur-md">
      <div class="drama-alert-container relative flex flex-col items-center p-8 rounded-2xl max-w-2xl w-full text-center shadow-2xl overflow-hidden">
        <!-- Hazard Stripes Top & Bottom -->
        <div class="drama-alert-hazard absolute top-0 left-0 right-0 h-4"></div>
        <div class="drama-alert-hazard absolute bottom-0 left-0 right-0 h-4"></div>

        <!-- Alert Title & Pulsing Badge -->
        <div class="mt-4 mb-2 flex items-center gap-2">
          <div class="h-4 w-4 rounded-full bg-rose-500 animate-ping"></div>
          <span class="text-xl sm:text-2xl font-black text-rose-500 tracking-wider">
            {$_("publicDashboard.criticalAlert")}
          </span>
          <div class="h-4 w-4 rounded-full bg-rose-500 animate-ping"></div>
        </div>

        <div class="text-sm font-extrabold text-white uppercase tracking-wider mb-3">
          {activeAlert.nodeName || "MONITORED TARGET"} - {activeAlert.targetName}
        </div>

        <!-- Value & Alert Message -->
        <div class="my-3 px-6 py-4 rounded-xl border border-rose-500/40 bg-rose-950/60 max-w-lg w-full">
          {#if activeAlert.valueText}
            <div class="text-2xl font-black font-mono text-white tracking-wide">
              {activeAlert.valueText}
            </div>
          {/if}
          <div class="text-xs font-semibold text-rose-200 mt-1">
            {activeAlert.message || $_("publicDashboard.defaultAlertMsg")}
          </div>
        </div>

        <!-- Alert Meta Info -->
        <div class="grid grid-cols-3 gap-3 my-4 w-full max-w-md text-xs font-mono">
          <div class="rounded-lg bg-black/40 border border-slate-800 p-2">
            <span class="text-[10px] text-slate-400 block">{$_("publicDashboard.occurrenceCount")}</span>
            <span class="font-bold text-rose-400 text-sm">{alertCount} {$_("publicDashboard.unitTimes")}</span>
          </div>
          <div class="rounded-lg bg-black/40 border border-slate-800 p-2">
            <span class="text-[10px] text-slate-400 block">{$_("publicDashboard.firstSeen")}</span>
            <span class="font-bold text-slate-200 text-[10px] truncate block">{alertFirstTime || '-'}</span>
          </div>
          <div class="rounded-lg bg-black/40 border border-slate-800 p-2">
            <span class="text-[10px] text-slate-400 block">{$_("publicDashboard.lastSeen")}</span>
            <span class="font-bold text-slate-200 text-[10px] truncate block">{alertLastTime || '-'}</span>
          </div>
        </div>

        <!-- Dismiss Button -->
        <button
          type="button"
          onclick={dismissAlert}
          class="mt-2 mb-4 px-8 py-3 rounded-xl font-black text-xs uppercase tracking-wider bg-gradient-to-r from-rose-600 to-red-600 hover:from-rose-500 hover:to-red-500 text-white shadow-[0_0_25px_rgba(239,68,68,0.7)] transition-all cursor-pointer active:scale-95"
        >
          {$_("publicDashboard.dismiss")}
        </button>
      </div>
    </div>
  {/if}

  <!-- Preload Material Design Icons font so browser fetches woff2 immediately -->
  <span class="mdi mdi-monitor pointer-events-none fixed -top-[9999px] -left-[9999px] opacity-0" aria-hidden="true"></span>
</div>

<style>
  /* TV Drama Style Flash Background */
  .drama-alert-overlay {
    background: radial-gradient(circle, rgba(185, 28, 28, 0.95) 0%, rgba(15, 0, 0, 0.98) 100%);
    animation: alertFlashBg 0.8s infinite alternate cubic-bezier(0.4, 0, 0.6, 1);
  }

  @keyframes alertFlashBg {
    0% {
      background-color: rgba(180, 0, 0, 0.95);
      box-shadow: inset 0 0 100px rgba(255, 0, 0, 0.8);
    }
    100% {
      background-color: rgba(30, 0, 0, 0.98);
      box-shadow: inset 0 0 200px rgba(255, 0, 0, 0.3);
    }
  }

  /* Pulsing Hazard Alert Container */
  .drama-alert-container {
    background: rgba(10, 5, 5, 0.96);
    border: 3px solid #ff0000;
    box-shadow: 0 0 50px rgba(255, 0, 0, 0.8), inset 0 0 30px rgba(255, 0, 0, 0.4);
    animation: alertPulseBox 1s infinite alternate ease-in-out;
  }

  @keyframes alertPulseBox {
    0% {
      border-color: #ff0000;
      box-shadow: 0 0 30px rgba(255, 0, 0, 0.6);
    }
    100% {
      border-color: #ff5555;
      box-shadow: 0 0 70px rgba(255, 0, 0, 1.0);
    }
  }

  /* Hazard Diagonal Stripes */
  .drama-alert-hazard {
    background: repeating-linear-gradient(
      -45deg,
      #ffcc00,
      #ffcc00 15px,
      #000000 15px,
      #000000 30px
    );
  }
</style>
