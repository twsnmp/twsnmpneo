<script lang="ts">
  import { onMount, onDestroy, tick } from "svelte";
  import {
    fetchSystemInfo,
    fetchMonitorData,
    execBackup,
    fetchUpdateCheck,
    fetchMe,
    type SystemInfo,
    type MonitorDataEnt,
    type UpdateInfo,
  } from "../api";
  import { showConfirm, showLoading, hideLoading, showAlert } from "../stores/modalStore";
  import SystemUpdateModal from "../components/SystemUpdateModal.svelte";
  import { _ } from "svelte-i18n";
  import { renderBytes, renderSpeed, renderPercent, formatTimeStr, formatVersion } from "../common";
  import {
    showMonitorResChart,
    showMonitorNetChart,
    showMonitorForecastChart,
    resizeMonitorChart,
    disposeMonitorCharts,
  } from "../charts/system";
  import {
    Server,
    Activity,
    Clock,
    CheckCircle2,
    Cpu,
    Database,
    RotateCcw,
    TrendingUp,
    FileSpreadsheet,
    Layers,
    Search,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    ArrowUpDown,
    ArrowUp,
    ArrowDown,
    X,
    Radio,
    ShieldCheck,
    AlertTriangle,
    BarChart3,
    Table as TableIcon,
    Sparkles,
    Download,
  } from "@lucide/svelte";

  type TabMode = "status" | "monitor";

  let activeTab = $state<TabMode>("status");
  let loading = $state(false);
  let refreshing = $state(false);

  let sysInfo = $state<SystemInfo | null>(null);
  let monitorLogs = $state<MonitorDataEnt[]>([]);

  const formatReceiverPort = (port?: string) => {
    if (!port || port === "-") return $_('system.notConfigured');
    return port;
  };

  const getTimezoneLabel = (timeStr?: string) => {
    if (!timeStr) return "";
    const match = timeStr.match(/([+-]\d{2}:\d{2}|Z)$/);
    const offset = match ? (match[1] === "Z" ? "UTC" : match[1]) : "";
    const tzName = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (tzName && offset) {
      return `${$_('system.tz')}: ${tzName} (${offset})`;
    } else if (offset) {
      return `${$_('system.tz')}: ${offset}`;
    }
    return tzName ? `${$_('system.tz')}: ${tzName}` : "";
  };
  let updateInfo = $state<UpdateInfo | null>(null);
  let checkingUpdate = $state(false);
  let showUpdateModal = $state(false);
  let isAdmin = $state(false);

  // Chart DOM elements
  let resChartElem = $state<HTMLElement | null>(null);
  let netChartElem = $state<HTMLElement | null>(null);
  let forecastChartElem = $state<HTMLElement | null>(null);

  // Modals & Notifications
  let showForecastModal = $state(false);
  let backupStatus = $state<{ file: string; size: number; time: string } | null>(null);
  let backupError = $state<string | null>(null);
  let backupLoading = $state(false);

  // Table State
  let tableSearch = $state("");
  let tablePage = $state(1);
  let tablePageSize = $state(25);
  let sortColumn = $state<keyof MonitorDataEnt>("Time");
  let sortDirection = $state<"asc" | "desc">("desc");

  // Load initial data
  const loadData = async () => {
    loading = true;
    try {
      const [info, mon, me] = await Promise.all([
        fetchSystemInfo().catch(() => null),
        fetchMonitorData().catch(() => []),
        fetchMe().catch(() => null),
      ]);
      if (info) sysInfo = info;
      monitorLogs = mon;
      if (me) isAdmin = me.role === "admin";

      if (activeTab === "monitor") {
        await tick();
        renderOverviewCharts();
      }

      // Check for updates asynchronously in background without blocking monitor rendering
      fetchUpdateCheck(false)
        .then((upd) => {
          if (upd) updateInfo = upd;
        })
        .catch(() => null);
    } catch (e) {
      console.error("Failed to load system monitor data:", e);
    } finally {
      loading = false;
      refreshing = false;
    }
  };

  const handleCheckUpdate = async () => {
    checkingUpdate = true;
    try {
      const info = await fetchUpdateCheck(true);
      updateInfo = info;
      showUpdateModal = true;
    } catch (e: any) {
      showAlert({
        title: $_("system.checkUpdate") || "アップデート確認",
        message: e.message || "アップデートの確認に失敗しました",
        type: "danger",
      });
    } finally {
      checkingUpdate = false;
    }
  };

  const handleRefresh = async () => {
    refreshing = true;
    await loadData();
  };

  const renderOverviewCharts = () => {
    if (resChartElem && monitorLogs.length > 0) {
      showMonitorResChart(resChartElem, monitorLogs);
    }
    if (netChartElem && monitorLogs.length > 0) {
      showMonitorNetChart(netChartElem, monitorLogs);
    }
  };

  // Switch Tab
  const setTab = async (tab: TabMode) => {
    activeTab = tab;
    if (tab === "monitor") {
      await tick();
      renderOverviewCharts();
    } else {
      disposeMonitorCharts();
    }
  };

  // Open Forecast Modal
  const openForecast = async () => {
    showForecastModal = true;
    await tick();
    if (forecastChartElem && monitorLogs.length > 0) {
      showMonitorForecastChart(forecastChartElem, monitorLogs);
    }
  };

  // Handle Backup
  const handleBackup = async () => {
    const ok = await showConfirm({
      title: $_('common.confirm') || 'バックアップ確認',
      message: $_('system.confirmBackup'),
      type: 'warning',
      confirmText: $_('system.backup') || 'バックアップ作成',
    });
    if (!ok) {
      return;
    }
    showLoading({
      title: $_('system.backup') || 'バックアップ',
      message: 'システムバックアップを作成しています...',
    });
    backupLoading = true;
    backupStatus = null;
    backupError = null;
    try {
      const res = await execBackup();
      backupStatus = res;
      setTimeout(() => {
        backupStatus = null;
      }, 7000);
    } catch (e: any) {
      backupError = e.message || $_('system.backupFailed');
      setTimeout(() => {
        backupError = null;
      }, 7000);
    } finally {
      backupLoading = false;
      hideLoading();
    }
  };

  // Latest snapshot helper
  const latestSnapshot = $derived.by<MonitorDataEnt | null>(() => {
    if (monitorLogs.length === 0) return null;
    return monitorLogs[monitorLogs.length - 1];
  });

  // Table filtering and sorting
  const filteredLogs = $derived.by(() => {
    const q = tableSearch.trim().toLowerCase();
    if (!q) return monitorLogs;
    return monitorLogs.filter((m) => {
      const timeStr = formatTimeStr(Math.floor(m.Time / 1e6)).toLowerCase();
      return timeStr.includes(q);
    });
  });

  const sortedLogs = $derived.by(() => {
    const list = [...filteredLogs];
    const key = sortColumn;
    const dir = sortDirection === "asc" ? 1 : -1;
    list.sort((a, b) => {
      const valA = a[key] ?? 0;
      const valB = b[key] ?? 0;
      return (valA - valB) * dir;
    });
    return list;
  });

  const totalPages = $derived(
    tablePageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedLogs.length / tablePageSize))
  );

  const paginatedLogs = $derived.by(() => {
    if (tablePageSize === -1) return sortedLogs;
    const start = (tablePage - 1) * tablePageSize;
    return sortedLogs.slice(start, start + tablePageSize);
  });

  const handleSort = (col: keyof MonitorDataEnt) => {
    if (sortColumn === col) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = col;
      sortDirection = "desc";
    }
  };

  // Dynamic color coding for resource stats
  const getPercentClass = (val: number | undefined, warn = 80, crit = 90) => {
    if (val == null) return "text-slate-700 dark:text-slate-300";
    if (val >= crit) return "text-rose-600 dark:text-rose-400 font-bold";
    if (val >= warn) return "text-amber-600 dark:text-amber-400 font-semibold";
    return "text-slate-700 dark:text-slate-300";
  };

  const getLoadClass = (val: number | undefined, warn = 4.0, crit = 8.0) => {
    if (val == null) return "text-slate-700 dark:text-slate-300";
    if (val >= crit) return "text-rose-600 dark:text-rose-400 font-bold";
    if (val >= warn) return "text-amber-600 dark:text-amber-400 font-semibold";
    return "text-slate-700 dark:text-slate-300";
  };

  // Export CSV
  const exportCSV = () => {
    if (monitorLogs.length === 0) return;
    const header = [
      "Time",
      "CPU(%)",
      "Mem(%)",
      "MyCPU(%)",
      "MyMem(%)",
      "Swap(%)",
      "Disk(%)",
      "Load",
      "Net(bps)",
      "Conn",
      "Proc",
      "NumGoroutine",
      "HeapAlloc(Bytes)",
      "Sys(Bytes)",
      "DBSize(Bytes)",
    ].join(",");

    const rows = sortedLogs.map((m) => {
      const timeStr = formatTimeStr(Math.floor(m.Time / 1e6));
      return [
        `"${timeStr}"`,
        m.CPU?.toFixed(2) ?? "0",
        m.Mem?.toFixed(2) ?? "0",
        m.MyCPU?.toFixed(2) ?? "0",
        m.MyMem?.toFixed(2) ?? "0",
        m.Swap?.toFixed(2) ?? "0",
        m.Disk?.toFixed(2) ?? "0",
        m.Load?.toFixed(2) ?? "0",
        m.Net?.toFixed(2) ?? "0",
        m.Conn ?? "0",
        m.Proc ?? "0",
        m.NumGoroutine ?? "0",
        m.HeapAlloc ?? "0",
        m.Sys ?? "0",
        m.DBSize ?? "0",
      ].join(",");
    });

    const csvContent = "\uFEFF" + [header, ...rows].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `twsnmpneo_system_monitor_${Date.now()}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  };

  onMount(() => {
    loadData();

    const handleResize = () => {
      resizeMonitorChart(showForecastModal);
    };
    window.addEventListener("resize", handleResize);

    // Watch for dark/light class change on html root
    const observer = new MutationObserver(() => {
      if (activeTab === "monitor") {
        renderOverviewCharts();
      }
      if (showForecastModal && forecastChartElem && monitorLogs.length > 0) {
        showMonitorForecastChart(forecastChartElem, monitorLogs);
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

    return () => {
      window.removeEventListener("resize", handleResize);
      observer.disconnect();
      disposeMonitorCharts();
    };
  });
</script>

<div class="flex h-[calc(100vh-4.25rem)] w-full flex-col overflow-hidden bg-slate-50 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans">
  <!-- Top Navigation & Action Header -->
  <header class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 px-6 py-3 shrink-0 shadow-sm dark:shadow-lg">
    <!-- Title & Tabs -->
    <div class="flex items-center gap-6">
      <div>
        <div class="flex items-center gap-2">
          <Server class="h-5 w-5 text-cyan-500 dark:text-cyan-400" />
          <h1 class="text-base font-bold text-slate-900 dark:text-slate-100 tracking-wide">{$_('system.title')}</h1>
        </div>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{$_('system.subtitle')}</p>
      </div>

      <!-- Navigation Tabs -->
      <div class="flex items-center rounded-xl bg-slate-100 dark:bg-slate-950 p-1 border border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => setTab("status")}
          class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'status' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <Activity class="h-3.5 w-3.5" />
          <span>{$_('system.tabOverview')}</span>
        </button>

        <button
          type="button"
          onclick={() => setTab("monitor")}
          class="flex items-center gap-2 rounded-lg px-3 py-1.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'monitor' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
        >
          <BarChart3 class="h-3.5 w-3.5" />
          <span>{$_('system.tabHistory')}</span>
          {#if monitorLogs.length > 0}
            <span class="ml-1 rounded-md px-1.5 py-0.5 text-[10px] font-mono {activeTab === 'monitor' ? 'bg-white/25 text-white' : 'bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300'}">
              {monitorLogs.length}
            </span>
          {/if}
        </button>
      </div>
    </div>

    <!-- Action Buttons -->
    <div class="flex items-center gap-2.5">
      <button
        type="button"
        onclick={openForecast}
        class="flex items-center gap-1.5 rounded-xl border border-amber-300 dark:border-amber-800/60 bg-amber-50 dark:bg-amber-950/40 hover:bg-amber-100 dark:hover:bg-amber-900/60 px-3.5 py-1.5 text-xs font-semibold text-amber-800 dark:text-amber-300 transition-colors cursor-pointer shadow-xs"
        title={$_('system.btnCapacityForecastTitle')}
      >
        <TrendingUp class="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
        <span>{$_('system.btnCapacityForecast')}</span>
      </button>

      <!-- Check for Updates Button -->
      <button
        type="button"
        onclick={handleCheckUpdate}
        disabled={checkingUpdate}
        class="flex items-center gap-1.5 rounded-xl border px-3.5 py-1.5 text-xs font-semibold transition-colors cursor-pointer shadow-xs {updateInfo?.has_update ? 'border-cyan-400 bg-cyan-50 dark:bg-cyan-950/40 hover:bg-cyan-100 dark:hover:bg-cyan-900/60 text-cyan-800 dark:text-cyan-300 ring-2 ring-cyan-500/20 animate-pulse' : 'border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200'}"
        title={$_('system.checkUpdate')}
      >
        <Sparkles class="h-3.5 w-3.5 {updateInfo?.has_update ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-500 dark:text-slate-400'} {checkingUpdate ? 'animate-spin' : ''}" />
        <span>{checkingUpdate ? $_('system.checkingUpdate') : (updateInfo?.has_update ? `${$_('system.updateAvailable')} (${formatVersion(updateInfo.latest_version)})` : $_('system.checkUpdate'))}</span>
      </button>

      <button
        type="button"
        onclick={handleBackup}
        disabled={backupLoading}
        class="flex items-center gap-1.5 rounded-xl border border-emerald-300 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40 hover:bg-emerald-100 dark:hover:bg-emerald-900/60 px-3.5 py-1.5 text-xs font-semibold text-emerald-800 dark:text-emerald-300 transition-colors cursor-pointer shadow-xs"
        title={$_('system.btnBackupTitle')}
      >
        <Database class="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 {backupLoading ? 'animate-pulse' : ''}" />
        <span>{$_('system.btnBackup')}</span>
      </button>

      {#if activeTab === "monitor"}
        <button
          type="button"
          onclick={exportCSV}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
          title={$_('system.btnExportCsvTitle')}
        >
          <FileSpreadsheet class="h-3.5 w-3.5 text-cyan-500 dark:text-cyan-400" />
          <span>{$_('system.btnExportCsv')}</span>
        </button>
      {/if}

      <button
        type="button"
        onclick={handleRefresh}
        disabled={refreshing || loading}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
      >
        <RotateCcw class="h-3.5 w-3.5 text-cyan-500 dark:text-cyan-400 {refreshing ? 'animate-spin' : ''}" />
        <span>{$_('system.btnReload')}</span>
      </button>
    </div>
  </header>

  <!-- Backup Notification Banner -->
  {#if backupStatus}
    <div class="mx-6 mt-3 flex items-center justify-between rounded-xl border border-emerald-500/50 bg-emerald-950/70 p-3 text-xs text-emerald-200 shadow-md">
      <div class="flex items-center gap-2">
        <CheckCircle2 class="h-4 w-4 text-emerald-400 shrink-0" />
        <div>
          <span class="font-bold">{$_('system.backupSuccess')}</span>
          <span class="ml-1 font-mono text-emerald-300">{backupStatus.file}</span>
          <span class="ml-2 font-mono text-slate-300">({renderBytes(backupStatus.size)})</span>
        </div>
      </div>
      <button onclick={() => (backupStatus = null)} class="text-emerald-400 hover:text-emerald-200 cursor-pointer">
        <X class="h-4 w-4" />
      </button>
    </div>
  {/if}

  {#if backupError}
    <div class="mx-6 mt-3 flex items-center justify-between rounded-xl border border-rose-500/50 bg-rose-950/70 p-3 text-xs text-rose-200 shadow-md">
      <div class="flex items-center gap-2">
        <AlertTriangle class="h-4 w-4 text-rose-400 shrink-0" />
        <span>{$_('system.backupError')} {backupError}</span>
      </div>
      <button onclick={() => (backupError = null)} class="text-rose-400 hover:text-rose-200 cursor-pointer">
        <X class="h-4 w-4" />
      </button>
    </div>
  {/if}

  <!-- Tab Content: System Status -->
  {#if activeTab === "status"}
    <div class="flex-1 overflow-y-auto p-6 space-y-6">
      <!-- 6-Card High-Level Overview Grid -->
      <div class="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-6 gap-4">
        <!-- Card 1: Daemon Health -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Server class="h-3.5 w-3.5" />
              <span>{$_('system.daemonStatus')}</span>
            </span>
            <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/80 px-2 py-0.2 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
              ● ACTIVE
            </span>
          </div>
          <div class="my-2 flex items-center gap-2 text-2xl font-bold text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 class="h-6 w-6" />
            <span>{sysInfo?.status?.toUpperCase() || "HEALTHY"}</span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono flex items-center justify-between">
            <span>{$_('system.uptime')} <span class="text-cyan-600 dark:text-cyan-300 font-semibold">{sysInfo?.uptime || $_('system.starting')}</span></span>
          </div>
        </div>

        <!-- Card 2: Version -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Activity class="h-3.5 w-3.5" />
              <span>{$_('system.version')}</span>
            </span>
            {#if updateInfo?.has_update}
              <button
                type="button"
                onclick={() => (showUpdateModal = true)}
                class="flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-400 border border-cyan-500/30 transition-colors cursor-pointer animate-pulse"
                title={$_('system.updateAvailable')}
              >
                <Sparkles class="h-2.5 w-2.5" />
                <span>{$_('system.updateAvailable')}: {formatVersion(updateInfo.latest_version)}</span>
              </button>
            {/if}
          </div>
          <div class="my-2 flex items-baseline gap-2">
            <span class="text-xl font-bold font-mono text-slate-900 dark:text-slate-100">
              {formatVersion(sysInfo?.version) || "v0.2.0"}
            </span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate flex items-center justify-between">
            <span>
              {#if sysInfo?.commit}
                ({sysInfo.commit})
              {:else}
                (none)
              {/if}
            </span>
            {#if updateInfo?.has_update}
              <button
                type="button"
                onclick={() => (showUpdateModal = true)}
                class="text-[10px] text-cyan-600 dark:text-cyan-400 hover:underline font-medium"
              >
                詳細を見る &rarr;
              </button>
            {/if}
          </div>
        </div>

        <!-- Card 3: Server Clock -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Clock class="h-3.5 w-3.5" />
              <span>{$_('system.serverTime')}</span>
            </span>
          </div>
          <div class="my-2 text-base font-bold font-mono text-slate-900 dark:text-slate-100 leading-tight">
            {formatTimeStr(sysInfo?.time || new Date().toISOString())}
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
            {getTimezoneLabel(sysInfo?.time)}
          </div>
        </div>

        <!-- Card 4: CPU & Load -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Cpu class="h-3.5 w-3.5" />
              <span>{$_('system.cpuLoad')}</span>
            </span>
            <span class="text-[10px] font-mono text-slate-500 dark:text-slate-400">{sysInfo?.num_cpu || 1} {$_('system.cores')}</span>
          </div>
          <div class="my-2 flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono text-sky-600 dark:text-sky-400">
              {renderPercent(latestSnapshot?.CPU)}
            </span>
            <span class="text-xs text-slate-500 dark:text-slate-400 font-mono">
              Load {latestSnapshot?.Load?.toFixed(2) ?? "0.00"}
            </span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate">
            {$_('system.myCpu')} <span class="text-cyan-600 dark:text-cyan-300 font-semibold">{renderPercent(latestSnapshot?.MyCPU)}</span>
          </div>
        </div>

        <!-- Card 5: Memory -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Layers class="h-3.5 w-3.5" />
              <span>{$_('system.memoryUsage')}</span>
            </span>
            <span class="text-[10px] font-mono text-slate-500 dark:text-slate-400">Swap {renderPercent(latestSnapshot?.Swap)}</span>
          </div>
          <div class="my-2 flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono text-purple-600 dark:text-purple-400">
              {renderPercent(latestSnapshot?.Mem)}
            </span>
            <span class="text-xs text-slate-500 dark:text-slate-400 font-mono">
              My {renderPercent(latestSnapshot?.MyMem)}
            </span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate">
            Heap: <span class="text-purple-600 dark:text-purple-300 font-semibold">{renderBytes(latestSnapshot?.HeapAlloc || 0)}</span>
          </div>
        </div>

        <!-- Card 6: Database & Storage -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-4 shadow-sm dark:shadow-lg hover:border-slate-300 dark:hover:border-slate-700 transition-all flex flex-col justify-between">
          <div class="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400 font-medium">
            <span class="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
              <Database class="h-3.5 w-3.5" />
              <span>{$_('system.storageDb')}</span>
            </span>
            <span class="text-[10px] font-mono text-emerald-600 dark:text-emerald-400">bbolt+pq</span>
          </div>
          <div class="my-2 flex items-baseline gap-2">
            <span class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
              {renderBytes(latestSnapshot?.DBSize || 0)}
            </span>
          </div>
          <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono truncate">
            Disk: <span class="text-amber-600 dark:text-amber-300 font-semibold">{renderPercent(latestSnapshot?.Disk)}</span> {$_('system.diskUsed')}
          </div>
        </div>
      </div>

      <!-- Protocol Servers Overview Card -->
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <div class="flex items-center gap-2">
            <ShieldCheck class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <h3 class="text-xs font-bold text-slate-800 dark:text-slate-200 uppercase tracking-wider">{$_('system.serverHealth')}</h3>
          </div>
          <div class="flex items-center gap-4 text-xs font-mono text-slate-500 dark:text-slate-400">
            <span>{$_('system.monitoredNodes')} <strong class="text-cyan-600 dark:text-cyan-400">{sysInfo?.node_count ?? 0}</strong></span>
            <span>{$_('system.activePolling')} <strong class="text-cyan-600 dark:text-cyan-400">{sysInfo?.poll_count ?? 0}</strong></span>
          </div>
        </div>

        {#snippet statusBadge(status?: string)}
          {#if status === "running"}
            <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.2 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
              {$_('system.running')}
            </span>
          {:else if status === "disabled"}
            <span class="inline-flex items-center gap-1 rounded-full bg-slate-100 dark:bg-slate-800/80 border border-slate-300 dark:border-slate-700/60 px-2 py-0.2 text-[10px] font-semibold text-slate-500 dark:text-slate-400">
              {$_('system.disabled')}
            </span>
          {:else}
            <span class="inline-flex items-center gap-1 rounded-full bg-slate-100 dark:bg-slate-800/80 border border-slate-300 dark:border-slate-700/60 px-2 py-0.2 text-[10px] font-semibold text-slate-500 dark:text-slate-400">
              {$_('system.stopped')}
            </span>
          {/if}
        {/snippet}

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 text-xs">
          <!-- Syslog -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">Syslog</span>
              {@render statusBadge(sysInfo?.receivers?.syslog?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.syslog?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.syslog?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">RFC 3164 / RFC 5424</div>
          </div>

          <!-- SNMP TRAP -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">SNMP TRAP</span>
              {@render statusBadge(sysInfo?.receivers?.trap?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.trap?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.trap?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">SNMP TRAP / InformRequest</div>
          </div>

          <!-- NetFlow / IPFIX -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">NetFlow / IPFIX</span>
              {@render statusBadge(sysInfo?.receivers?.netflow?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.netflow?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.netflow?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">NetFlow v5 / v9 / IPFIX</div>
          </div>

          <!-- MCP Server -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">{$_('system.mcpServer')}</span>
              {@render statusBadge(sysInfo?.receivers?.mcp?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.mcp?.status === 'running' ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.mcp?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_('system.mcpDesc')}</div>
          </div>

          <!-- OpenTelemetry -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">OpenTelemetry</span>
              {@render statusBadge(sysInfo?.receivers?.otel?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.otel?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.otel?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">OTLP Traces / Metrics / Logs</div>
          </div>

          <!-- MQTT Broker -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">{$_('system.mqttBroker')}</span>
              {@render statusBadge(sysInfo?.receivers?.mqtt?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.mqtt?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.mqtt?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_('system.mqttDesc')}</div>
          </div>

          <!-- sFlow -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">sFlow</span>
              {@render statusBadge(sysInfo?.receivers?.sflow?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.sflow?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {formatReceiverPort(sysInfo?.receivers?.sflow?.port)}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_('system.sflowDesc')}</div>
          </div>

          <!-- ARP Watch -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">ARP Watch</span>
              {@render statusBadge(sysInfo?.receivers?.arp?.status)}
            </div>
            <div class="font-mono font-semibold {sysInfo?.receivers?.arp?.status === 'running' ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              {sysInfo?.receivers?.arp?.status === 'running' ? (sysInfo?.receivers?.arp?.range || $_('system.arpScan')) : $_('system.disabled')}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">{$_('system.arpDesc')}</div>
          </div>

          <!-- PING (ICMP Engine) -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3.5 space-y-1.5 hover:border-slate-300 dark:hover:border-slate-700 transition-colors">
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-800 dark:text-slate-200">{$_('system.pingEngine') || "PING (ICMP)"}</span>
              {@render statusBadge(sysInfo?.ping_mode ? 'running' : 'stopped')}
            </div>
            <div class="font-mono font-semibold {sysInfo?.ping_mode ? 'text-cyan-600 dark:text-cyan-400' : 'text-slate-400 dark:text-slate-500'}">
              Mode: {sysInfo?.ping_mode?.toUpperCase() || "UDP"}
            </div>
            <div class="text-[10px] text-slate-500 dark:text-slate-400">
              {sysInfo?.ping_mode === "icmp" ? ($_('system.pingModeIcmp') || "RAW ICMP Socket (Privileged)") : ($_('system.pingModeUdp') || "Non-privileged ICMP over UDP")}
            </div>
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tab Content: Resource Monitor (Charts & History Table) -->
  {#if activeTab === "monitor"}
    <div class="flex-1 overflow-y-auto p-6 space-y-6">
      <!-- Telemetry Charts Section (2-Column Grid) -->
      <div class="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <!-- Resource Chart -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg flex flex-col">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 mb-2">
            <div class="flex items-center gap-2">
              <Cpu class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
              <h3 class="text-xs font-bold text-slate-800 dark:text-slate-200 uppercase tracking-wider">
                {$_('system.resChartTitle')}
              </h3>
            </div>
            <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">
              {monitorLogs.length} {$_('system.samples')}
            </span>
          </div>
          <div bind:this={resChartElem} class="w-full h-72"></div>
        </div>

        <!-- Network & Connections Chart -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg flex flex-col">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3 mb-2">
            <div class="flex items-center gap-2">
              <Radio class="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
              <h3 class="text-xs font-bold text-slate-800 dark:text-slate-200 uppercase tracking-wider">
                {$_('system.netChartTitle')}
              </h3>
            </div>
            <span class="text-[10px] text-slate-500 dark:text-slate-400 font-mono">
              {$_('system.current')} {renderSpeed(latestSnapshot?.Net || 0)} / {latestSnapshot?.Conn || 0} conn
            </span>
          </div>
          <div bind:this={netChartElem} class="w-full h-72"></div>
        </div>
      </div>

      <!-- Data History Table Section -->
      <div class="space-y-4">
        <!-- Table Filter Bar -->
        <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg shrink-0">
          <div class="flex items-center gap-3">
            <div class="relative w-72">
              <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                placeholder={$_('system.searchPlaceholder')}
                bind:value={tableSearch}
                oninput={() => (tablePage = 1)}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
              />
            </div>
            <span class="text-xs text-slate-500 dark:text-slate-400">
              {$_('system.recordsCount', { values: { count: filteredLogs.length } })}
            </span>
          </div>

          <div class="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
            <button
              type="button"
              onclick={exportCSV}
              class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 hover:bg-slate-100 dark:hover:bg-slate-800 px-3 py-1 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
              title={$_('system.btnExportCsvTitle')}
            >
              <FileSpreadsheet class="h-3.5 w-3.5 text-cyan-500 dark:text-cyan-400" />
              <span>{$_('system.btnExportCsv')}</span>
            </button>

            <span>{$_('system.displayCount')}</span>
            <select
              bind:value={tablePageSize}
              onchange={() => (tablePage = 1)}
              class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2.5 py-1 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none cursor-pointer shadow-xs"
            >
              <option value={15}>15 {$_('system.items')}</option>
              <option value={25}>25 {$_('system.items')}</option>
              <option value={50}>50 {$_('system.items')}</option>
              <option value={100}>100 {$_('system.items')}</option>
              <option value={-1}>{$_('system.all')}</option>
            </select>
          </div>
        </div>

        <!-- Table Container -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
          <div class="overflow-x-auto">
            <table class="w-full border-collapse text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 text-[10px] font-semibold uppercase tracking-wider select-none">
              <tr>
                <th onclick={() => handleSort("Time")} class="px-1 py-2 cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">
                  <div class="flex items-center gap-1">
                    <span>{$_('system.time')}</span>
                    {#if sortColumn === "Time"}
                      {#if sortDirection === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 text-slate-400 dark:text-slate-600" />
                    {/if}
                  </div>
                </th>
                <th onclick={() => handleSort("CPU")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">
                  <div class="flex items-center justify-end gap-1">
                    <span>CPU</span>
                    {#if sortColumn === "CPU"}
                      {#if sortDirection === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                    {/if}
                  </div>
                </th>
                <th onclick={() => handleSort("Mem")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">
                  <div class="flex items-center justify-end gap-1">
                    <span>{$_('system.memory')}</span>
                    {#if sortColumn === "Mem"}
                      {#if sortDirection === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                    {/if}
                  </div>
                </th>
                <th onclick={() => handleSort("MyCPU")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">
                  <div class="flex items-center justify-end gap-1">
                    <span>My CPU</span>
                    {#if sortColumn === "MyCPU"}
                      {#if sortDirection === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                    {/if}
                  </div>
                </th>
                <th onclick={() => handleSort("MyMem")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">
                  <div class="flex items-center justify-end gap-1">
                    <span>{$_('system.myMemory')}</span>
                    {#if sortColumn === "MyMem"}
                      {#if sortDirection === "asc"}<ArrowUp class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />{/if}
                    {/if}
                  </div>
                </th>
                <th onclick={() => handleSort("Swap")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Swap</th>
                <th onclick={() => handleSort("Disk")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Disk</th>
                <th onclick={() => handleSort("Load")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Load</th>
                <th onclick={() => handleSort("Net")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">{$_('system.netSpeed')}</th>
                <th onclick={() => handleSort("Conn")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Conn</th>
                <th onclick={() => handleSort("Proc")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Proc</th>
                <th onclick={() => handleSort("NumGoroutine")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Goroutine</th>
                <th onclick={() => handleSort("HeapAlloc")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Heap</th>
                <th onclick={() => handleSort("Sys")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">Sys</th>
                <th onclick={() => handleSort("DBSize")} class="px-2 py-1 text-right cursor-pointer hover:text-slate-900 dark:hover:text-slate-200">DB Size</th>
              </tr>
            </thead>
            <tbody class="font-mono text-[11px]">
              {#if paginatedLogs.length === 0}
                <tr>
                  <td colspan="15" class="py-12 text-center text-slate-500 font-sans">
                    {$_('system.noData')}
                  </td>
                </tr>
              {:else}
                {#each paginatedLogs as row}
                  <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="px-2 py-1 text-slate-800 dark:text-slate-200 whitespace-nowrap">
                      {formatTimeStr(Math.floor(row.Time / 1e6))}
                    </td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.CPU, 80, 90)}">{renderPercent(row.CPU)}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.Mem, 85, 95)}">{renderPercent(row.Mem)}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.MyCPU, 50, 80)}">{renderPercent(row.MyCPU)}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.MyMem, 50, 80)}">{renderPercent(row.MyMem)}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.Swap, 60, 80)}">{renderPercent(row.Swap)}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.Disk, 85, 95)}">{renderPercent(row.Disk)}</td>
                    <td class="px-2 py-1 text-right {getLoadClass(row.Load, 4.0, 8.0)}">{row.Load?.toFixed(2) ?? "0.00"}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{renderSpeed(row.Net || 0)}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{row.Conn ?? 0}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{row.Proc ?? 0}</td>
                    <td class="px-2 py-1 text-right {getPercentClass(row.NumGoroutine, 500, 2000)}">{row.NumGoroutine ?? 0}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{renderBytes(row.HeapAlloc || 0)}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{renderBytes(row.Sys || 0)}</td>
                    <td class="px-2 py-1 text-right text-slate-700 dark:text-slate-300">{renderBytes(row.DBSize || 0)}</td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Pagination Controls -->
        {#if tablePageSize !== -1 && totalPages > 1}
          <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-4 py-3 shrink-0 text-xs text-slate-600 dark:text-slate-400">
            <div>
              <span>{$_('system.pageInfo', { values: { total: filteredLogs.length, from: (tablePage - 1) * tablePageSize + 1, to: Math.min(tablePage * tablePageSize, filteredLogs.length) } })}</span>
            </div>

            <div class="flex items-center gap-1.5">
              <button
                type="button"
                onclick={() => (tablePage = 1)}
                disabled={tablePage === 1}
                class="rounded-lg border border-slate-300 dark:border-slate-700 p-1 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                type="button"
                onclick={() => (tablePage = Math.max(1, tablePage - 1))}
                disabled={tablePage === 1}
                class="rounded-lg border border-slate-300 dark:border-slate-700 p-1 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <span class="px-2 text-slate-600 dark:text-slate-300 font-mono">
                {tablePage} / {totalPages}
              </span>
              <button
                type="button"
                onclick={() => (tablePage = Math.min(totalPages, tablePage + 1))}
                disabled={tablePage === totalPages}
                class="rounded-lg border border-slate-300 dark:border-slate-700 p-1 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                type="button"
                onclick={() => (tablePage = totalPages)}
                disabled={tablePage === totalPages}
                class="rounded-lg border border-slate-300 dark:border-slate-700 p-1 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
              >
                <ChevronsRight class="h-4 w-4" />
              </button>
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
</div>

<!-- Size Forecast Modal -->
{#if showForecastModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-xs p-4">
    <div class="w-full max-w-4xl rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl flex flex-col overflow-hidden text-slate-800 dark:text-slate-100 animate-in fade-in zoom-in-95 duration-150">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4">
        <div class="flex items-center gap-2.5">
          <TrendingUp class="h-5 w-5 text-amber-500 dark:text-amber-400" />
          <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100">{$_('system.forecastModalTitle')}</h2>
        </div>
        <button
          type="button"
          onclick={() => (showForecastModal = false)}
          class="rounded-lg p-1 text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-6">
        <div class="mb-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-3 text-xs text-slate-600 dark:text-slate-400">
          {$_('system.forecastModalDesc')}
        </div>
        <div bind:this={forecastChartElem} class="w-full h-96"></div>
      </div>

      <!-- Modal Footer -->
      <div class="flex justify-end border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-6 py-3">
        <button
          type="button"
          onclick={() => (showForecastModal = false)}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_('system.close')}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- System Update Modal -->
<SystemUpdateModal
  open={showUpdateModal}
  {updateInfo}
  {isAdmin}
  onClose={() => (showUpdateModal = false)}
/>

