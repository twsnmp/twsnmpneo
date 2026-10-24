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
    Layers,
    Bot,
  } from "@lucide/svelte";
  import {
    fetchNodes,
    fetchPollings,
    fetchEventLogs,
    fetchArpTable,
    fetchIPAM,
    queryParquetLogs,
    fetchMqttStats,
    resetArpTable,
    type NodeEnt,
    type PollingEnt,
    type EventLogEnt,
    type ArpEnt,
    type IPAMReportResp,
    type ParquetLogRecord,
    type MqttStatEnt,
  } from "../api";
  import { showConfirm, showAlert } from "../stores/modalStore";

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
  import SensorReport from "./reports/SensorReport.svelte";
  import EnvSensor from "./reports/EnvSensor.svelte";
  import AnomalyReport from "./reports/AnomalyReport.svelte";
  import WifiReport from "./reports/WifiReport.svelte";
  import BluetoothReport from "./reports/BluetoothReport.svelte";
  import PcapReport from "./reports/PcapReport.svelte";
  import WinLogReport from "./reports/WinLogReport.svelte";
  import MqttReport from "./reports/MqttReport.svelte";
  import OTelReport from "./reports/OTelReport.svelte";
  import AgentReport from "./reports/AgentReport.svelte";

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
    | "env"
    | "wifi"
    | "bluetooth"
    | "pcap"
    | "winlog"
    | "twagent"
    | "mqtt"
    | "otel"
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
  let mqttStats = $state<MqttStatEnt[]>([]);

  type DatasetKey =
    | "nodes"
    | "pollings"
    | "logs"
    | "arpList"
    | "arpLogs"
    | "ipam"
    | "netflowLogs"
    | "sflowLogs"
    | "syslogLogs"
    | "trapLogs"
    | "mqttStats";

  const loaded = $state<Record<DatasetKey, boolean>>({
    nodes: false,
    pollings: false,
    logs: false,
    arpList: false,
    arpLogs: false,
    ipam: false,
    netflowLogs: false,
    sflowLogs: false,
    syslogLogs: false,
    trapLogs: false,
    mqttStats: false,
  });

  const getRequiredDatasets = (category: ReportCategory): DatasetKey[] => {
    switch (category) {
      case "device":
        return ["nodes", "arpList", "arpLogs"];
      case "ipam":
        return ["nodes", "arpList", "ipam"];
      case "polling":
      case "cert":
      case "twagent":
        return ["nodes", "pollings"];
      case "event":
        return ["nodes", "logs"];
      case "syslog":
        return ["nodes", "syslogLogs"];
      case "trap":
        return ["nodes", "trapLogs"];
      case "flow":
        return ["netflowLogs"];
      case "sflow":
        return ["sflowLogs"];
      case "arp":
        return ["nodes", "arpLogs"];
      case "mqtt":
        return ["nodes", "mqttStats"];
      case "sensor":
      case "env":
      case "wifi":
      case "bluetooth":
      case "pcap":
      case "winlog":
      case "ai":
        return ["nodes"];
      case "otel":
      default:
        return [];
    }
  };

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
    { id: "sensor", name: $_("report.tabSensor"), icon: Radio },
    { id: "env", name: $_("report.tabEnv"), icon: Thermometer },
    { id: "wifi", name: $_("report.tabWifi"), icon: Wifi },
    { id: "bluetooth", name: $_("report.tabBluetooth"), icon: Bluetooth },
    { id: "pcap", name: $_("report.tabPcap"), icon: Binary },
    { id: "winlog", name: $_("report.tabWinlog"), icon: Terminal },
    { id: "twagent", name: $_("report.tabTwagent") || "twagentneo", icon: Bot },
    { id: "mqtt", name: $_("report.tabMqtt") || "MQTT", icon: Radio },
    { id: "otel", name: $_("report.tabOtel") || "OpenTelemetry", icon: Layers },
    { id: "ai", name: $_("report.tabAi"), icon: Sparkles },
  ]);

  const loadCategoryData = async (category: ReportCategory = activeReport, force = false) => {
    const required = new Set<DatasetKey>(getRequiredDatasets(category));
    // Ensure sidebar status pill (nodes & pollings count) is populated on initial load
    if (!loaded.nodes) required.add("nodes");
    if (!loaded.pollings) required.add("pollings");

    const toFetch = [...required].filter((key) => force || !loaded[key]);
    const shouldRefreshChild =
      force && activeReportRef && typeof activeReportRef.refresh === "function";

    if (toFetch.length === 0 && !shouldRefreshChild) {
      return;
    }

    loading = true;
    const now = Date.now();
    const start24h = (now - 24 * 60 * 60 * 1000) * 1e6; // UnixNano

    try {
      const tasks: Promise<void>[] = toFetch.map(async (key) => {
        switch (key) {
          case "nodes":
            nodes = await fetchNodes().catch(() => []);
            loaded.nodes = true;
            break;
          case "pollings":
            pollings = await fetchPollings().catch(() => []);
            loaded.pollings = true;
            break;
          case "logs":
            logs = await fetchEventLogs().catch(() => []);
            loaded.logs = true;
            break;
          case "arpList":
            arpList = await fetchArpTable().catch(() => []);
            loaded.arpList = true;
            break;
          case "arpLogs":
            arpLogs = await queryParquetLogs({ type: "arplog", limit: 10000 }).catch(() => []);
            loaded.arpLogs = true;
            break;
          case "ipam":
            ipamReport = await fetchIPAM().catch(() => ({
              Ranges: [],
              TotalRanges: 0,
              TotalSize: 0,
              TotalUsed: 0,
              TotalUsage: 0,
            }));
            loaded.ipam = true;
            break;
          case "netflowLogs":
            netflowLogs = await queryParquetLogs({ type: "netflow", start: start24h, limit: 10000 }).catch(() => []);
            loaded.netflowLogs = true;
            break;
          case "sflowLogs":
            sflowLogs = await queryParquetLogs({ type: "sflow", start: start24h, limit: 10000 }).catch(() => []);
            loaded.sflowLogs = true;
            break;
          case "syslogLogs":
            syslogLogs = await queryParquetLogs({ type: "syslog", start: start24h, limit: 10000 }).catch(() => []);
            loaded.syslogLogs = true;
            break;
          case "trapLogs":
            trapLogs = await queryParquetLogs({ type: "trap", start: start24h, limit: 10000 }).catch(() => []);
            loaded.trapLogs = true;
            break;
          case "mqttStats":
            mqttStats = await fetchMqttStats().catch(() => []);
            loaded.mqttStats = true;
            break;
        }
      });

      if (shouldRefreshChild) {
        tasks.push(Promise.resolve(activeReportRef.refresh()));
      }

      await Promise.all(tasks);
    } catch (e) {
      console.error("Failed to load report data:", e);
    } finally {
      loading = false;
    }
  };

  const handleRefresh = () => loadCategoryData(activeReport, true);

  onMount(() => {
    void loadCategoryData(activeReport, false);
  });

  const handleResetArp = async () => {
    const ok = await showConfirm({
      title: $_('common.confirm') || 'ARPテーブルリセット確認',
      message: $_("report.confirmResetArp"),
      type: 'warning',
      confirmText: $_('common.reset') || 'リセット',
    });
    if (!ok) return;
    try {
      await resetArpTable();
      loaded.ipam = false;
      await loadCategoryData(activeReport, true);
    } catch (err: any) {
      showAlert({
        title: $_('common.error') || 'エラー',
        message: $_("report.alertResetFailed", { values: { error: err.message || err } }),
        type: 'danger',
      });
    }
  };

  const exportCSV = () => {
    if (activeReportRef && typeof activeReportRef.exportCSV === "function") {
      activeReportRef.exportCSV();
    }
  };

  const hasClearSupport = $derived(
    (activeReport === "device" && arpList.length > 0) ||
    activeReport === "flow" ||
    activeReport === "sflow" ||
    activeReport === "syslog" ||
    activeReport === "trap" ||
    activeReport === "wifi" ||
    activeReport === "bluetooth" ||
    activeReport === "env" ||
    activeReport === "pcap" ||
    activeReport === "winlog" ||
    activeReport === "twagent"
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
          onclick={() => {
            activeReport = cat.id;
            searchQuery = "";
            void loadCategoryData(cat.id, false);
          }}
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
          onclick={handleRefresh}
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
        onRefresh={() => {
          loaded.ipam = false;
          void loadCategoryData("device", true);
        }}
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
        onReload={() => loadCategoryData("cert", true)}
      />
    {:else if activeReport === "sensor"}
      <SensorReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "env"}
      <EnvSensor
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "wifi"}
      <WifiReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "bluetooth"}
      <BluetoothReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "pcap"}
      <PcapReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "winlog"}
      <WinLogReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {loading}
      />
    {:else if activeReport === "twagent"}
      <AgentReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {pollings}
        onRefresh={() => loadCategoryData("twagent", true)}
        {loading}
      />
    {:else if activeReport === "mqtt"}
      <MqttReport
        bind:this={activeReportRef}
        {searchQuery}
        {mqttStats}
        {nodes}
        onRefresh={() => loadCategoryData("mqtt", true)}
        {loading}
      />
    {:else if activeReport === "otel"}
      <OTelReport
        bind:this={activeReportRef}
        {searchQuery}
        {loading}
      />
    {:else if activeReport === "ai"}
      <AnomalyReport
        bind:this={activeReportRef}
        {searchQuery}
        {nodes}
        {pollings}
        {logs}
      />
    {/if}
  </div>
</div>
