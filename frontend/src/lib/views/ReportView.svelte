<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Laptop,
    Network,
    Activity,
    BarChart3,
    Share2,
    FileText,
    ScrollText,
    Radio,
    Eye,
    ShieldCheck,
    Thermometer,
    Sparkles,
    Search,
    RefreshCw,
    Download,
    Trash2,
    Plus,
    Wifi,
    Bluetooth,
    Binary,
    Terminal,
  } from "@lucide/svelte";
  import {
    fetchNodes,
    fetchPollings,
    fetchEventLogs,
    fetchArpTable,
    fetchIPAM,
    queryParquetLogs,
    fetchCertMonitors,
    fetchMqttStats,
    resetArpTable,
    deletePolling,
    savePolling,
    type NodeEnt,
    type PollingEnt,
    type EventLogEnt,
    type ArpEnt,
    type IPAMReportResp,
    type ParquetLogRecord,
    type CertMonitorEnt,
    type MqttStatEnt,
  } from "../api";

  import PollingDialog from "../components/PollingDialog.svelte";
  import DeviceReport from "./reports/DeviceReport.svelte";
  import IpamReport from "./reports/IpamReport.svelte";
  import PollingReport from "./reports/PollingReport.svelte";
  import NetFlowReport from "./reports/NetFlowReport.svelte";
  import SFlowReport from "./reports/SFlowReport.svelte";
  import EventReport from "./reports/EventReport.svelte";
  import SyslogReport from "./reports/SyslogReport.svelte";
  import TrapReport from "./reports/TrapReport.svelte";
  import ArpReport from "./reports/ArpReport.svelte";
  import CertReport from "./reports/CertReport.svelte";
  import EnvSensor from "./reports/EnvSensor.svelte";
  import AnomalyReport from "./reports/AnomalyReport.svelte";
  import WifiReport from "./reports/WifiReport.svelte";
  import BluetoothReport from "./reports/BluetoothReport.svelte";
  import PcapReport from "./reports/PcapReport.svelte";
  import WinLogReport from "./reports/WinLogReport.svelte";

  type ReportCategory =
    | "device"
    | "ipam"
    | "polling"
    | "event"
    | "syslog"
    | "trap"
    | "flow"
    | "sflow"
    | "arp"
    | "cert"
    | "sensor"
    | "wifi"
    | "bluetooth"
    | "pcap"
    | "winlog"
    | "ai";

  let activeReport = $state<ReportCategory>("device");
  let activeReportRef = $state<any>(null);
  let searchQuery = $state("");
  let loading = $state(false);

  // Data sets
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let logs = $state<EventLogEnt[]>([]);
  let arpList = $state<ArpEnt[]>([]);
  let arpLogs = $state<ParquetLogRecord[]>([]);
  let ipamReport = $state<IPAMReportResp>({
    Ranges: [],
    TotalRanges: 0,
    TotalSize: 0,
    TotalUsed: 0,
    TotalUsage: 0,
  });
  let netflowLogs = $state<ParquetLogRecord[]>([]);
  let sflowLogs = $state<ParquetLogRecord[]>([]);
  let syslogLogs = $state<ParquetLogRecord[]>([]);
  let trapLogs = $state<ParquetLogRecord[]>([]);
  let certMonitors = $state<CertMonitorEnt[]>([]);
  let mqttStats = $state<MqttStatEnt[]>([]);

  const hasBlueScanPolling = $derived(
    pollings.some(
      (p) =>
        p.type === "twbluescan" ||
        (p.type === "syslog" &&
          (p.mode === "twbluescan" || (p as any).Mode === "twbluescan"))
    )
  );

  $effect(() => {
    if (activeReport === "sensor" && !hasBlueScanPolling) {
      activeReport = "device";
    }
  });

  const categories = $derived<{ id: ReportCategory; name: string; icon: any }[]>([
    { id: "device", name: $_("report.tabDevice"), icon: Laptop },
    { id: "ipam", name: $_("report.tabIpam"), icon: Network },
    { id: "polling", name: $_("report.tabPolling"), icon: Activity },
    { id: "event", name: $_("report.tabEvent"), icon: FileText },
    { id: "syslog", name: $_("report.tabSyslog"), icon: ScrollText },
    { id: "trap", name: $_("report.tabTrap"), icon: Radio },
    { id: "flow", name: $_("report.tabFlow"), icon: BarChart3 },
    { id: "sflow", name: $_("report.tabSFlow"), icon: Share2 },
    { id: "arp", name: $_("report.tabArp"), icon: Eye },
    { id: "cert", name: $_("report.tabCert"), icon: ShieldCheck },
    ...(hasBlueScanPolling
      ? [{ id: "sensor" as const, name: $_("report.tabSensor"), icon: Thermometer }]
      : []),
    { id: "wifi", name: $_("report.tabWifi"), icon: Wifi },
    { id: "bluetooth", name: $_("report.tabBluetooth"), icon: Bluetooth },
    { id: "pcap", name: $_("report.tabPcap"), icon: Binary },
    { id: "winlog", name: $_("report.tabWinlog"), icon: Terminal },
    { id: "ai", name: $_("report.tabAi"), icon: Sparkles },
  ]);

  const loadData = async () => {
    loading = true;
    const now = Date.now();
    const start24h = (now - 24 * 60 * 60 * 1000) * 1e6; // UnixNano
    try {
      const [n, p, l, a, ipam, netflows, sflows, certs, mqtt, arps, syslogs, traps] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
        fetchEventLogs().catch(() => []),
        fetchArpTable().catch(() => []),
        fetchIPAM().catch(() => ({
          Ranges: [],
          TotalRanges: 0,
          TotalSize: 0,
          TotalUsed: 0,
          TotalUsage: 0,
        })),
        queryParquetLogs({ type: "netflow", start: start24h, limit: 10000 }).catch(() => []),
        queryParquetLogs({ type: "sflow", start: start24h, limit: 10000 }).catch(() => []),
        fetchCertMonitors().catch(() => []),
        fetchMqttStats().catch(() => []),
        queryParquetLogs({ type: "arplog", limit: 10000 }).catch(() => []),
        queryParquetLogs({ type: "syslog", start: start24h, limit: 10000 }).catch(() => []),
        queryParquetLogs({ type: "trap", start: start24h, limit: 10000 }).catch(() => []),
      ]);
      nodes = n;
      pollings = p;
      logs = l;
      arpList = a;
      ipamReport = ipam;
      netflowLogs = netflows;
      sflowLogs = sflows;
      certMonitors = certs;
      mqttStats = mqtt;
      arpLogs = arps;
      syslogLogs = syslogs;
      trapLogs = traps;
    } catch (e) {
      console.error("Failed to load report data:", e);
    } finally {
      loading = false;
      if (activeReportRef && typeof activeReportRef.refresh === "function") {
        activeReportRef.refresh();
      }
    }
  };

  onMount(() => {
    loadData();
  });

  const handleResetArp = async () => {
    if (!confirm($_("report.confirmResetArp"))) return;
    try {
      await resetArpTable();
      await loadData();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  };

  const exportCSV = () => {
    if (activeReportRef && typeof activeReportRef.exportCSV === "function") {
      activeReportRef.exportCSV();
    }
  };

  const getSyslogReportMode = (cat: ReportCategory): string | null => {
    switch (cat) {
      case "wifi":
        return "twwifiscan";
      case "bluetooth":
        return "twbluescan";
      case "pcap":
        return "twpcap";
      case "winlog":
        return "twwinlog";
      default:
        return null;
    }
  };

  const currentReportMode = $derived(getSyslogReportMode(activeReport));

  const existingReportPolling = $derived.by(() => {
    if (!currentReportMode) return null;
    return (
      pollings.find(
        (p) =>
          p.type === currentReportMode ||
          (p.type === "syslog" &&
            (p.mode === currentReportMode || (p as any).Mode === currentReportMode))
      ) || null
    );
  });

  let showPollingModal = $state(false);
  let editingPolling = $state<PollingEnt | null>(null);

  const handleAddNewReportPolling = (mode: string) => {
    const nameMap: Record<string, string> = {
      twwifiscan: "twWifiScan",
      twbluescan: "twBlueScan",
      twpcap: "twpcap",
      twwinlog: "twwinlog",
    };
    editingPolling = {
      id: "",
      name: nameMap[mode] || mode,
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "syslog",
      mode: mode,
      params: "",
      filter: "",
      extractor: "",
      script: "",
      level: "off",
      poll_int: 300,
      timeout: 5,
      retry: 1,
      log_mode: 0,
      next_time: 0,
      last_time: 0,
      result: {},
      state: "unknown",
      fail_action: "",
      repair_action: "",
      ai_mode: "default",
      vector_cols: "",
      mqtt_url: "",
      mqtt_topic: "",
      mqtt_cols: "",
      fail_time: 0,
    } as PollingEnt;
    showPollingModal = true;
  };

  const handleSavePolling = async (p: PollingEnt) => {
    try {
      await savePolling(p);
      showPollingModal = false;
      await loadData();
    } catch (err: any) {
      alert("ポーリングの保存に失敗しました: " + (err.message || err));
    }
  };

  const handleDeleteReportPolling = async () => {
    if (!existingReportPolling) return;
    const pName =
      existingReportPolling.name ||
      (existingReportPolling as any).Name ||
      "Syslog Report Polling";
    const pId = existingReportPolling.id || (existingReportPolling as any).ID;
    if (
      !confirm(
        $_("report.confirmDeleteReportPolling", { values: { name: pName } }) ||
          `ポーリング「${pName}」を削除しますか？`
      )
    ) {
      return;
    }
    try {
      await deletePolling(pId);
      await loadData();
    } catch (err: any) {
      alert($_("report.alertDeleteFailed", { values: { error: err.message || err } }));
    }
  };

  const hasClearSupport = $derived(
    (activeReport === "device" && arpList.length > 0) ||
    activeReport === "wifi" ||
    activeReport === "bluetooth" ||
    activeReport === "sensor" ||
    activeReport === "pcap" ||
    activeReport === "winlog"
  );

  const handleClearReport = async () => {
    if (activeReport === "device") {
      await handleResetArp();
    } else if (activeReportRef && typeof activeReportRef.handleClear === "function") {
      await activeReportRef.handleClear();
    }
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-50 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans">
  <!-- Sidebar Navigation -->
  <div class="w-64 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/70 p-3 space-y-1.5 shrink-0 flex flex-col justify-between">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
        {$_("report.titleSuite")}
      </div>
      {#each categories as cat}
        <button
          type="button"
          onclick={() => { activeReport = cat.id; searchQuery = ""; }}
          class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeReport === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-800 dark:text-slate-200'}"
        >
          <cat.icon class="h-4 w-4 shrink-0 text-cyan-600 dark:text-cyan-400" />
          <span class="truncate">{cat.name}</span>
        </button>
      {/each}
    </div>

    <!-- Live Status Pill in Sidebar -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-100/90 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-1.5">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-700 dark:text-slate-200">{$_("report.dataSync")}</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          {$_("report.realtime")}
        </span>
      </div>
      <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400">
        {$_("report.nodesAndPollings", { values: { nodes: nodes.length, pollings: pollings.length } })}
      </div>
    </div>
  </div>

  <!-- Main Report Canvas -->
  <div class="flex-1 overflow-y-auto p-6 space-y-6">
    {#if currentReportMode && !existingReportPolling}
      <!-- When Syslog Report Polling is Not Configured: Show ONLY guidance card -->
      <div class="flex flex-col items-center justify-center min-h-[420px] p-8 text-center rounded-3xl border border-dashed border-slate-300 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 backdrop-blur-xs shadow-xs">
        <div class="p-4 rounded-2xl bg-amber-50 dark:bg-amber-950/60 border border-amber-200 dark:border-amber-800/80 text-amber-600 dark:text-amber-400 mb-4 shadow-sm">
          <Radio class="w-8 h-8 animate-pulse" />
        </div>
        <h3 class="text-base font-bold text-slate-800 dark:text-slate-100 mb-2">
          {$_("report.pollingNotConfiguredTitle")}
        </h3>
        <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md mb-6 leading-relaxed">
          {$_("report.pollingNotConfiguredDesc")}
        </p>
        <button
          type="button"
          onclick={() => handleAddNewReportPolling(currentReportMode)}
          class="flex items-center gap-2 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-5 py-2.5 text-xs font-bold text-white shadow-lg shadow-cyan-600/30 hover:shadow-cyan-600/50 transition-all cursor-pointer"
        >
          <Plus class="h-4 w-4" />
          <span>{$_("report.btnAddPolling")}</span>
        </button>
      </div>
    {:else}
      <!-- Top Action Bar -->
      <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg">
        <div class="flex items-center gap-3">
          <div class="relative w-72">
            <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              placeholder={$_("report.searchPlaceholder")}
              bind:value={searchQuery}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
            />
          </div>
        </div>

        <div class="flex items-center gap-2.5">
          {#if activeReport === "cert"}
            <button
              type="button"
              onclick={() => activeReportRef?.handleAddNewCertPolling?.()}
              class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
              title={$_("report.btnAddPolling")}
            >
              <Plus class="h-3.5 w-3.5" />
              <span>{$_("report.btnAddPolling")}</span>
            </button>
          {:else if currentReportMode && existingReportPolling}
            <button
              type="button"
              onclick={handleDeleteReportPolling}
              class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 px-3.5 py-1.5 text-xs font-semibold text-rose-700 dark:text-rose-300 transition-colors cursor-pointer shadow-xs"
              title={$_("report.btnDeletePolling")}
            >
              <Trash2 class="h-3.5 w-3.5 text-rose-500 dark:text-rose-400" />
              <span>{$_("report.btnDeletePolling")}</span>
            </button>
          {/if}
          <button
            type="button"
            onclick={exportCSV}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
          >
            <Download class="h-3.5 w-3.5" />
            <span>{$_("report.btnExportCsv")}</span>
          </button>
          {#if hasClearSupport}
            <button
              type="button"
              onclick={handleClearReport}
              disabled={loading}
              class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 px-3.5 py-1.5 text-xs font-semibold text-rose-700 dark:text-rose-300 transition-colors cursor-pointer shadow-xs"
              title={activeReport === "device" ? $_("report.clearArpTitle") : $_("report.clearTitle")}
            >
              <Trash2 class="h-3.5 w-3.5 text-rose-500 dark:text-rose-400" />
              <span>{$_("report.btnClear")}</span>
            </button>
          {/if}
          <button
            type="button"
            onclick={loadData}
            disabled={loading}
            class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
          >
            <RefreshCw class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 {loading ? 'animate-spin' : ''}" />
            <span>{$_("report.btnRefresh")}</span>
          </button>
        </div>
      </div>

      <!-- Active Report Subcomponent -->
      {#if activeReport === "device"}
        <DeviceReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          {arpList}
          {arpLogs}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "ipam"}
        <IpamReport
          bind:this={activeReportRef}
          {searchQuery}
          {ipamReport}
          {nodes}
          {arpList}
        />
      {:else if activeReport === "polling"}
        <PollingReport
          bind:this={activeReportRef}
          {searchQuery}
          {pollings}
          {nodes}
        />
      {:else if activeReport === "event"}
        <EventReport
          bind:this={activeReportRef}
          {searchQuery}
          {logs}
          {nodes}
        />
      {:else if activeReport === "syslog"}
        <SyslogReport
          bind:this={activeReportRef}
          {searchQuery}
          {syslogLogs}
          {nodes}
        />
      {:else if activeReport === "trap"}
        <TrapReport
          bind:this={activeReportRef}
          {searchQuery}
          {trapLogs}
          {nodes}
        />
      {:else if activeReport === "flow"}
        <NetFlowReport
          bind:this={activeReportRef}
          {searchQuery}
          {netflowLogs}
        />
      {:else if activeReport === "sflow"}
        <SFlowReport
          bind:this={activeReportRef}
          {searchQuery}
          {sflowLogs}
        />
      {:else if activeReport === "arp"}
        <ArpReport
          bind:this={activeReportRef}
          {searchQuery}
          {arpLogs}
          {nodes}
        />
      {:else if activeReport === "cert"}
        <CertReport
          bind:this={activeReportRef}
          {searchQuery}
          {pollings}
          {nodes}
          onReload={loadData}
        />
      {:else if activeReport === "sensor"}
        <EnvSensor
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "wifi"}
        <WifiReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "bluetooth"}
        <BluetoothReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "pcap"}
        <PcapReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "winlog"}
        <WinLogReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          onRefresh={loadData}
          {loading}
        />
      {:else if activeReport === "ai"}
        <AnomalyReport
          bind:this={activeReportRef}
          {searchQuery}
          {nodes}
          {pollings}
          {logs}
          onReload={loadData}
        />
      {/if}
    {/if}
  </div>
</div>

{#if showPollingModal && editingPolling}
  <PollingDialog
    bind:show={showPollingModal}
    bind:polling={editingPolling}
    {nodes}
    onSave={handleSavePolling}
  />
{/if}
