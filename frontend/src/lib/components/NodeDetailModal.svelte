<script lang="ts">
  import { untrack, onDestroy } from "svelte";
  import { initVPanel, setVPanel, deleteVPanel } from "../map/vpanel";
  import { getStateColor, getStateName, formatTimeStr } from "../common";
  import {
    fetchPollings,
    fetchEventLogs,
    fetchNodePorts,
    fetchNodeHostResource,
    fetchNodeRmon,
    diagnoseNode,
  } from "../api";
  import type {
    NodeEnt,
    PollingEnt,
    EventLogEnt,
    VPanelPortEnt,
    HostResourceEnt,
    RmonEtherStatsEnt,
    NodeDiagnoseResult,
  } from "../api";
  import { _ } from "svelte-i18n";
  import {
    X,
    Box,
    ListTree,
    Activity,
    FileText,
    RotateCw,
    ZoomIn,
    ZoomOut,
    Cpu,
    Info,
    Copy,
    Check,
    Server,
    ShieldAlert,
    Gauge,
    Stethoscope,
    AlertCircle,
    CheckCircle2,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    ChevronsLeft,
    ChevronLeft,
    ChevronRight,
    ChevronsRight,
    Maximize2,
    Minimize2,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    node = null,
    pollings = [],
    logs = [],
    initialTab = "basic",
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    pollings?: PollingEnt[];
    logs?: EventLogEnt[];
    initialTab?: TabType;
  }>();

  type TabType = "basic" | "vpanel" | "ports" | "polling" | "logs" | "hostinfo" | "rmon" | "diagnose";
  let activeTab = $state<TabType>("basic");
  let isMaximized = $state(false);

  // Host resource active sub-tab
  type HrSubTab = "system" | "storage" | "device" | "filesystem" | "process";
  let hrSubTab = $state<HrSubTab>("system");

  // VPanel controls
  let rotate = $state(false);
  let vpanelZoom = $state(1.0);
  let power = $state(true);

  // SNMP status check
  const isSnmpConfigured = $derived(
    !!(node && node.snmp_mode && node.snmp_mode !== "none" && node.snmp_mode !== "")
  );

  // Data states
  let realPorts = $state<VPanelPortEnt[]>([]);
  let isLoadingPorts = $state(false);
  let hostResource = $state<HostResourceEnt | null>(null);
  let isLoadingHostResource = $state(false);
  let hostResourceError = $state<string>("");

  // RMON states
  let rmonStats = $state<RmonEtherStatsEnt[]>([]);
  let isLoadingRmon = $state(false);
  let rmonError = $state<string>("");

  // Diagnose states
  let diagnoseResult = $state<NodeDiagnoseResult | null>(null);
  let isDiagnosing = $state(false);
  let diagnoseError = $state<string>("");

  let internalPollings = $state<PollingEnt[]>([]);
  let internalLogs = $state<EventLogEnt[]>([]);
  let isLoadingPollings = $state(false);
  let isLoadingLogs = $state(false);

  // Copy feedback states
  let copiedIP = $state(false);
  let copiedMAC = $state(false);

  // Effective lists
  const effectivePollings = $derived(pollings.length > 0 ? pollings : internalPollings);
  const effectiveLogs = $derived(logs.length > 0 ? logs : internalLogs);

  // Filtered pollings and logs matching current node
  const targetNodeId = $derived(node?.id || (node as any)?.ID || "");
  const targetNodeName = $derived(node?.name || (node as any)?.Name || "");
  const targetNodeIp = $derived(node?.ip || (node as any)?.IP || "");

  const nodePollings = $derived(
    effectivePollings.filter((p: PollingEnt) => {
      const pNodeId = p.node_id || (p as any).NodeID;
      const pName = (p as any).node_name || (p as any).NodeName;
      const pTarget = p.params || p.target || p.Params || (p as any).Target;
      return (
        (targetNodeId && pNodeId === targetNodeId) ||
        (targetNodeName && pName === targetNodeName) ||
        (targetNodeIp && pTarget === targetNodeIp)
      );
    })
  );

  const nodeLogs = $derived(
    effectiveLogs.filter((l: EventLogEnt) => {
      const lNodeId = l.node_id || (l as any).NodeID;
      const lNodeName = l.node_name || (l as any).NodeName;
      return (
        (targetNodeId && lNodeId === targetNodeId) ||
        (targetNodeName && lNodeName === targetNodeName)
      );
    })
  );

  // Tabs for vertical sidebar navigation (Matching ListView / LogSidebar style)
  const tabs = $derived<
    { id: TabType; name: string; icon: any; count?: number }[]
  >([
    { id: "basic", name: $_('nodeDetail.tabBasic'), icon: Info },
    { id: "diagnose", name: $_('nodeDetail.tabDiagnose'), icon: Stethoscope },
    {
      id: "logs",
      name: $_('nodeDetail.tabLogs'),
      icon: FileText,
      count: nodeLogs.length > 0 ? nodeLogs.length : undefined,
    },
    {
      id: "polling",
      name: $_('nodeDetail.tabPolling'),
      icon: Activity,
      count: nodePollings.length > 0 ? nodePollings.length : undefined,
    },
    ...(isSnmpConfigured
      ? [
          { id: "vpanel" as TabType, name: $_('nodeDetail.tabPanel'), icon: Box },
          {
            id: "ports" as TabType,
            name: $_('nodeDetail.tabPorts'),
            icon: ListTree,
            count: realPorts.length > 0 ? realPorts.length : undefined,
          },
          { id: "hostinfo" as TabType, name: $_('nodeDetail.tabHostInfo'), icon: Server },
          {
            id: "rmon" as TabType,
            name: $_('nodeDetail.tabRmon'),
            icon: Gauge,
            count: rmonStats.length > 0 ? rmonStats.length : undefined,
          },
        ]
      : []),
  ]);

  // Sorting & Pagination States for Tables
  // 1. Ports
  let portsSortCol = $state<string>("Index");
  let portsSortDir = $state<"asc" | "desc">("asc");
  let portsPage = $state(1);
  let portsPageSize = $state(10);

  // 2. Pollings
  let pollSortCol = $state<string>("name");
  let pollSortDir = $state<"asc" | "desc">("asc");
  let pollPage = $state(1);
  let pollPageSize = $state(10);

  // 3. Logs
  let logsSortCol = $state<string>("time");
  let logsSortDir = $state<"asc" | "desc">("desc");
  let logsPage = $state(1);
  let logsPageSize = $state(10);

  // 4. Host Resource tables
  let hrSortCol = $state<string>("Index");
  let hrSortDir = $state<"asc" | "desc">("asc");
  let hrPage = $state(1);
  let hrPageSize = $state(10);

  // Generic Sorter
  function sortItems<T>(items: T[], col: string, dir: "asc" | "desc"): T[] {
    return [...items].sort((a: any, b: any) => {
      let va = a[col];
      let vb = b[col];
      if (va === undefined || va === null) va = "";
      if (vb === undefined || vb === null) vb = "";
      let res = 0;
      if (typeof va === "number" && typeof vb === "number") {
        res = va - vb;
      } else {
        res = String(va).localeCompare(String(vb));
      }
      return dir === "asc" ? res : -res;
    });
  }

  // Sorted and Paginated Data
  const sortedPorts = $derived(sortItems(realPorts, portsSortCol, portsSortDir));
  const paginatedPorts = $derived(
    portsPageSize === -1
      ? sortedPorts
      : sortedPorts.slice((portsPage - 1) * portsPageSize, portsPage * portsPageSize)
  );
  const portsTotalPages = $derived(
    portsPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedPorts.length / portsPageSize))
  );

  const sortedPollings = $derived(sortItems(nodePollings, pollSortCol, pollSortDir));
  const paginatedPollings = $derived(
    pollPageSize === -1
      ? sortedPollings
      : sortedPollings.slice((pollPage - 1) * pollPageSize, pollPage * pollPageSize)
  );
  const pollTotalPages = $derived(
    pollPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedPollings.length / pollPageSize))
  );

  const sortedLogs = $derived(sortItems(nodeLogs, logsSortCol, logsSortDir));
  const paginatedLogs = $derived(
    logsPageSize === -1
      ? sortedLogs
      : sortedLogs.slice((logsPage - 1) * logsPageSize, logsPage * logsPageSize)
  );
  const logsTotalPages = $derived(
    logsPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedLogs.length / logsPageSize))
  );

  // Host Resource current list
  const currentHrList = $derived.by((): any[] => {
    if (!hostResource) return [];
    switch (hrSubTab) {
      case "system":
        return hostResource.System || [];
      case "storage":
        return hostResource.Storage || [];
      case "device":
        return hostResource.Device || [];
      case "filesystem":
        return hostResource.FileSystem || [];
      case "process":
        return hostResource.Process || [];
      default:
        return [];
    }
  });

  const sortedHrList = $derived(sortItems([...currentHrList], hrSortCol, hrSortDir));
  const paginatedHrList = $derived(
    hrPageSize === -1
      ? sortedHrList
      : sortedHrList.slice((hrPage - 1) * hrPageSize, hrPage * hrPageSize)
  );
  const hrTotalPages = $derived(
    hrPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedHrList.length / hrPageSize))
  );

  // 5. RMON
  let rmonSortCol = $state<string>("Index");
  let rmonSortDir = $state<"asc" | "desc">("asc");
  let rmonPage = $state(1);
  let rmonPageSize = $state(10);

  const sortedRmon = $derived(sortItems(rmonStats, rmonSortCol, rmonSortDir));
  const paginatedRmon = $derived(
    rmonPageSize === -1
      ? sortedRmon
      : sortedRmon.slice((rmonPage - 1) * rmonPageSize, rmonPage * rmonPageSize)
  );
  const rmonTotalPages = $derived(
    rmonPageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedRmon.length / rmonPageSize))
  );

  function handleRmonSort(col: string) {
    if (rmonSortCol === col) {
      rmonSortDir = rmonSortDir === "asc" ? "desc" : "asc";
    } else {
      rmonSortCol = col;
      rmonSortDir = "asc";
    }
  }

  const loadRmonData = async () => {
    if (!targetNodeId || !isSnmpConfigured) return;
    isLoadingRmon = true;
    rmonError = "";
    try {
      const res = await fetchNodeRmon(targetNodeId);
      if (res.supported && Array.isArray(res.stats)) {
        rmonStats = res.stats;
      } else if (res.error) {
        rmonError = res.error;
      }
    } catch (e: any) {
      rmonError = String(e?.message || e);
    } finally {
      isLoadingRmon = false;
    }
  };

  const runNodeDiagnose = async () => {
    if (!targetNodeId) return;
    isDiagnosing = true;
    diagnoseError = "";
    try {
      diagnoseResult = await diagnoseNode(targetNodeId);
    } catch (e: any) {
      diagnoseError = String(e?.message || e);
    } finally {
      isDiagnosing = false;
    }
  };

  // Sorting handlers
  function handlePortsSort(col: string) {
    if (portsSortCol === col) {
      portsSortDir = portsSortDir === "asc" ? "desc" : "asc";
    } else {
      portsSortCol = col;
      portsSortDir = "asc";
    }
  }

  function handlePollSort(col: string) {
    if (pollSortCol === col) {
      pollSortDir = pollSortDir === "asc" ? "desc" : "asc";
    } else {
      pollSortCol = col;
      pollSortDir = "asc";
    }
  }

  function handleLogsSort(col: string) {
    if (logsSortCol === col) {
      logsSortDir = logsSortDir === "asc" ? "desc" : "asc";
    } else {
      logsSortCol = col;
      logsSortDir = "desc";
    }
  }

  function handleHrSort(col: string) {
    if (hrSortCol === col) {
      hrSortDir = hrSortDir === "asc" ? "desc" : "asc";
    } else {
      hrSortCol = col;
      hrSortDir = "asc";
    }
  }

  // Load ports & host resource when modal opens
  $effect(() => {
    if (show && node) {
      untrack(() => {
        // Reset states
        activeTab = initialTab;
        realPorts = [];
        hostResource = null;
        // Reset RMON and diagnose states
        rmonStats = [];
        rmonError = "";
        diagnoseResult = null;
        diagnoseError = "";

        // If SNMP is configured, query ports, host resources, and RMON from backend
        if (isSnmpConfigured && targetNodeId) {
          isLoadingPorts = true;
          fetchNodePorts(targetNodeId)
            .then((res) => {
              if (res.supported && Array.isArray(res.ports)) {
                realPorts = res.ports;
              } else {
                realPorts = [];
              }
            })
            .catch(() => {
              realPorts = [];
            })
            .finally(() => {
              isLoadingPorts = false;
            });

          isLoadingHostResource = true;
          fetchNodeHostResource(targetNodeId)
            .then((res) => {
              if (res.supported && res.data) {
                hostResource = res.data;
              } else if (res.error) {
                hostResourceError = res.error;
              }
            })
            .catch((e) => {
              hostResourceError = String(e?.message || e);
            })
            .finally(() => {
              isLoadingHostResource = false;
            });

          loadRmonData();
        }

        // Auto-fetch pollings if not passed
        if (pollings.length === 0 && !isLoadingPollings) {
          isLoadingPollings = true;
          fetchPollings()
            .then((res) => (internalPollings = res))
            .catch(() => {})
            .finally(() => (isLoadingPollings = false));
        }

        // Auto-fetch logs if not passed
        if (logs.length === 0 && !isLoadingLogs) {
          isLoadingLogs = true;
          fetchEventLogs({ limit: 200 })
            .then((res) => (internalLogs = res))
            .catch(() => {})
            .finally(() => (isLoadingLogs = false));
        }
      });
    } else {
      untrack(() => {
        deleteVPanel();
        copiedIP = false;
        copiedMAC = false;
      });
    }
  });

  // Fetch RMON or diagnose on activeTab change if needed
  $effect(() => {
    if (show && node) {
      if (!isSnmpConfigured && (activeTab === "vpanel" || activeTab === "ports" || activeTab === "hostinfo" || activeTab === "rmon")) {
        activeTab = "basic";
      }
      if (activeTab === "rmon" && rmonStats.length === 0 && !isLoadingRmon && isSnmpConfigured) {
        loadRmonData();
      } else if (activeTab === "diagnose" && !diagnoseResult && !isDiagnosing) {
        runNodeDiagnose();
      }
    }
  });

  const vpanelAction = (container: HTMLElement) => {
    const timer = setTimeout(() => {
      initVPanel("vpanel-canvas-container");
      // Use realPorts or fallback
      const vports = realPorts.length > 0
        ? realPorts.map((p) => ({ State: p.State, Speed: p.Speed }))
        : [];
      setVPanel(vports, power, rotate, vpanelZoom, 12);
    }, 50);

    return {
      destroy() {
        clearTimeout(timer);
        deleteVPanel();
      },
    };
  };

  const toggleRotate = () => {
    rotate = !rotate;
    const vports = realPorts.length > 0
      ? realPorts.map((p) => ({ State: p.State, Speed: p.Speed }))
      : [];
    setVPanel(vports, power, rotate, vpanelZoom, 12);
  };

  const handleZoom = (inZoom: boolean) => {
    vpanelZoom = inZoom ? Math.min(vpanelZoom + 0.2, 3.0) : Math.max(vpanelZoom - 0.2, 0.4);
    const vports = realPorts.length > 0
      ? realPorts.map((p) => ({ State: p.State, Speed: p.Speed }))
      : [];
    setVPanel(vports, power, rotate, vpanelZoom, 12);
  };

  const copyToClipboard = async (text: string, type: "ip" | "mac") => {
    try {
      await navigator.clipboard.writeText(text);
      if (type === "ip") {
        copiedIP = true;
        setTimeout(() => (copiedIP = false), 2000);
      } else {
        copiedMAC = true;
        setTimeout(() => (copiedMAC = false), 2000);
      }
    } catch (e) {
      console.error("Failed to copy:", e);
    }
  };

  onDestroy(() => {
    deleteVPanel();
  });
</script>

{#if show && node}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") show = false;
    }}
  >
    <!-- Wide Landscape Dialog Container -->
    <div
      class="flex flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200 transition-all duration-200 {isMaximized
        ? 'h-[96vh] w-[98vw] max-w-none'
        : 'h-[88vh] w-[96vw] max-w-[1440px]'}"
    >
      <!-- Modal Header -->
      <div
        class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3 shrink-0"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 dark:bg-cyan-500/20 border border-blue-500/20 dark:border-cyan-500/30 text-blue-600 dark:text-cyan-400"
          >
            <Cpu class="h-5 w-5" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{node.name}</h2>
              <span
                class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold border"
                style="background-color: {getStateColor(node.state)}18; border-color: {getStateColor(node.state)}50; color: {getStateColor(node.state)}"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  style="background-color: {getStateColor(node.state)}"
                ></span>
                {getStateName(node.state, $_)}
              </span>

              {#if !isSnmpConfigured}
                <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-semibold bg-amber-500/10 border border-amber-500/30 text-amber-600 dark:text-amber-400">
                  <ShieldAlert class="h-3 w-3" />
                  {$_('nodeDetail.snmpNotSupported')}
                </span>
              {/if}
            </div>
            <p class="text-[11px] font-mono text-slate-500 dark:text-cyan-400">
              {node.ip} {node.mac ? `(${node.mac})` : ""}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1.5">
          <!-- Maximize / Restore Toggle Button -->
          <button
            type="button"
            title={isMaximized ? $_('nodeDetail.restore') : $_('nodeDetail.maximize')}
            aria-label={isMaximized ? $_('nodeDetail.restore') : $_('nodeDetail.maximize')}
            onclick={() => {
              isMaximized = !isMaximized;
              if (activeTab === "vpanel") {
                setTimeout(() => {
                  initVPanel("vpanel-canvas-container");
                  const vports = realPorts.length > 0
                    ? realPorts.map((p) => ({ State: p.State, Speed: p.Speed }))
                    : [];
                  setVPanel(vports, power, rotate, vpanelZoom, 12);
                }, 100);
              }
            }}
            class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
          >
            {#if isMaximized}
              <Minimize2 class="h-4 w-4" />
            {:else}
              <Maximize2 class="h-4 w-4" />
            {/if}
          </button>

          <!-- Close Button -->
          <button
            type="button"
            aria-label={$_('common.close')}
            onclick={() => (show = false)}
            class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-200 hover:text-slate-700 dark:hover:bg-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
          >
            <X class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- Main Body Container (Sidebar + Content) -->
      <div class="flex flex-1 overflow-hidden min-h-0">
        <!-- Left Sidebar Navigation (Matching ListView / LogSidebar specification) -->
        <div
          class="w-60 border-r border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-950/70 p-3 space-y-2 shrink-0 flex flex-col justify-between overflow-y-auto transition-colors"
        >
          <div class="space-y-1">
            <div
              class="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400"
            >
              {$_('nodeDetail.navTitle')}
            </div>

            {#each tabs as tab}
              <button
                type="button"
                onclick={() => (activeTab = tab.id)}
                class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === tab.id
                  ? 'bg-gradient-to-r from-blue-600 to-blue-500 text-white shadow-md shadow-blue-600/30'
                  : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                <div class="flex items-center gap-2.5 truncate">
                  <tab.icon
                    class="h-4 w-4 shrink-0 {activeTab === tab.id
                      ? 'text-white'
                      : 'text-blue-600 dark:text-cyan-400'}"
                  />
                  <span class="truncate">{tab.name}</span>
                </div>
                {#if tab.count !== undefined}
                  <span
                    class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeTab === tab.id
                      ? 'bg-white/20 text-white'
                      : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border border-slate-200 dark:border-transparent'}"
                  >
                    {tab.count}
                  </span>
                {/if}
              </button>
            {/each}
          </div>

          <!-- Node Status & Telemetry Summary Card -->
          <div
            class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-2 transition-colors"
          >
            <div class="flex items-center justify-between">
              <span class="font-semibold text-slate-700 dark:text-slate-200">SNMP</span>
              {#if isSnmpConfigured}
                <span
                  class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400"
                >
                  ● {node.snmp_mode || "v2c"}
                </span>
              {:else}
                <span
                  class="inline-flex items-center gap-1 rounded-full bg-amber-100 dark:bg-amber-950/80 border border-amber-300 dark:border-amber-800/60 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-400"
                >
                  {$_('nodeDetail.snmpNotSupported')}
                </span>
              {/if}
            </div>
            {#if node.ip}
              <div
                class="pt-1.5 border-t border-slate-200 dark:border-slate-800/80 flex items-center justify-between text-[10px]"
              >
                <span class="text-slate-500 dark:text-slate-400">IP</span>
                <span
                  class="font-mono text-slate-700 dark:text-cyan-400 font-medium truncate max-w-[120px]"
                  title={node.ip}
                >
                  {node.ip}
                </span>
              </div>
            {/if}
            {#if node.mac}
              <div class="flex items-center justify-between text-[10px]">
                <span class="text-slate-500 dark:text-slate-400">MAC</span>
                <span
                  class="font-mono text-slate-700 dark:text-slate-300 font-medium truncate max-w-[120px]"
                  title={node.mac}
                >
                  {node.mac}
                </span>
              </div>
            {/if}
          </div>
        </div>

      <!-- Tab Content Area -->
      <div
        class="relative flex-1 overflow-hidden p-5 bg-slate-50/70 dark:bg-slate-900/40 text-xs flex flex-col min-h-0"
      >
        <!-- 1. Basic Info Tab (twsnmpfk style) -->
        {#if activeTab === "basic"}
          <div
            class="h-full overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm"
          >
            <table class="w-full text-left text-xs border-collapse">
              <thead
                class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-700 dark:text-slate-300 uppercase text-[11px] font-bold border-b border-slate-200 dark:border-slate-800"
              >
                <tr>
                  <th class="py-1 px-2 w-1/4">{$_('nodeDetail.colItem')}</th>
                  <th class="py-1 px-2 w-3/4">{$_('nodeDetail.colContent')}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-800/60 font-medium">
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.name')}
                  </td>
                  <td class="py-1 px-2 text-slate-900 dark:text-slate-100 text-sm font-bold">
                    {node.name}
                  </td>
                </tr>
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.status')}
                  </td>
                  <td class="py-1 px-2">
                    <div class="inline-flex items-center gap-2">
                      <span
                        class="h-3 w-3 rounded-full shadow-sm"
                        style="background-color: {getStateColor(node.state)}"
                      ></span>
                      <span
                        class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-bold border"
                        style="background-color: {getStateColor(node.state)}18; border-color: {getStateColor(node.state)}50; color: {getStateColor(node.state)}"
                      >
                        {getStateName(node.state, $_)}
                      </span>
                    </div>
                  </td>
                </tr>
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.ip')}
                  </td>
                  <td class="py-1 px-2">
                    <div class="inline-flex items-center gap-3">
                      <span class="font-mono text-sm text-slate-900 dark:text-slate-100 font-semibold">
                        {node.ip || "-"}
                      </span>
                      {#if node.ip}
                        <button
                          type="button"
                          onclick={() => copyToClipboard(node.ip, "ip")}
                          class="inline-flex items-center gap-1 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-2 py-1 text-[11px] font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors cursor-pointer"
                        >
                          {#if copiedIP}
                            <Check class="h-3.5 w-3.5 text-emerald-500" />
                            <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_('nodeDetail.copied')}</span>
                          {:else}
                            <Copy class="h-3.5 w-3.5" />
                            <span>{$_('nodeDetail.copy')}</span>
                          {/if}
                        </button>
                      {/if}
                    </div>
                  </td>
                </tr>
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.mac')}
                  </td>
                  <td class="py-1 px-2">
                    <div class="inline-flex items-center gap-3">
                      <span class="font-mono text-xs text-slate-900 dark:text-slate-100 font-semibold">
                        {node.mac || "-"}
                      </span>
                      {#if node.mac}
                        <button
                          type="button"
                          onclick={() => copyToClipboard(node.mac || "", "mac")}
                          class="inline-flex items-center gap-1 rounded-md border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-2 py-1 text-[11px] font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors cursor-pointer"
                        >
                          {#if copiedMAC}
                            <Check class="h-3.5 w-3.5 text-emerald-500" />
                            <span class="text-emerald-600 dark:text-emerald-400 font-bold">{$_('nodeDetail.copied')}</span>
                          {:else}
                            <Copy class="h-3.5 w-3.5" />
                            <span>{$_('nodeDetail.copy')}</span>
                          {/if}
                        </button>
                      {/if}
                    </div>
                  </td>
                </tr>
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.vendor')}
                  </td>
                  <td class="py-1 px-2 text-slate-800 dark:text-slate-200 font-medium">
                    {node.vendor || (node as any).Vendor || "-"}
                  </td>
                </tr>
                <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 font-semibold">
                    {$_('nodeDetail.descr')}
                  </td>
                  <td class="py-1 px-2 text-slate-700 dark:text-slate-300 whitespace-pre-wrap">
                    {node.descr || (node as any).Descr || "-"}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

        <!-- 2. VPanel Tab -->
        {:else if activeTab === "vpanel"}
          {#if !isSnmpConfigured}
            <div class="flex h-full flex-col items-center justify-center rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-8 text-center shadow-sm">
              <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-500/10 border border-amber-500/30 text-amber-500 mb-4">
                <ShieldAlert class="h-8 w-8" />
              </div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 mb-1.5">
                {$_('nodeDetail.snmpNotSupportedTitle')}
              </h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md leading-relaxed">
                {$_('nodeDetail.snmpNotSupportedDesc')}
              </p>
            </div>
          {:else}
            <div
              class="relative flex h-full flex-col items-center justify-center rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-[#080d1e] overflow-hidden shadow-inner"
            >
              <div
                class="absolute top-3 right-3 z-10 flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-1.5 backdrop-blur-md shadow-lg"
              >
                <button
                  type="button"
                  onclick={toggleRotate}
                  class="flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-semibold transition-all cursor-pointer {rotate ? 'bg-blue-600 text-white shadow-sm shadow-blue-600/30' : 'bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
                >
                  <RotateCw class="h-3.5 w-3.5" />
                  {$_('nodeDetail.autoRotate')}
                </button>
                <button
                  type="button"
                  aria-label={$_('nodeDetail.zoomIn')}
                  onclick={() => handleZoom(true)}
                  class="rounded-lg p-1 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-200 cursor-pointer"
                >
                  <ZoomIn class="h-4 w-4" />
                </button>
                <button
                  type="button"
                  aria-label={$_('nodeDetail.zoomOut')}
                  onclick={() => handleZoom(false)}
                  class="rounded-lg p-1 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-900 dark:hover:text-slate-200 cursor-pointer"
                >
                  <ZoomOut class="h-4 w-4" />
                </button>
              </div>

              <div use:vpanelAction id="vpanel-canvas-container" class="h-full w-full"></div>
            </div>
          {/if}

        <!-- 3. Ports Tab -->
        {:else if activeTab === "ports"}
          {#if !isSnmpConfigured}
            <div class="flex h-full flex-col items-center justify-center rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-8 text-center shadow-sm">
              <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-500/10 border border-amber-500/30 text-amber-500 mb-4">
                <ShieldAlert class="h-8 w-8" />
              </div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 mb-1.5">
                {$_('nodeDetail.snmpNotSupportedTitle')}
              </h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md leading-relaxed">
                {$_('nodeDetail.snmpNotSupportedDesc')}
              </p>
            </div>
          {:else if isLoadingPorts}
            <div class="flex h-full items-center justify-center text-xs text-slate-500">
              <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 dark:border-cyan-400 mr-2"></div>
              {$_('common.loading')}
            </div>
          {:else if realPorts.length === 0}
            <div class="flex h-full flex-col items-center justify-center text-slate-400 p-8 text-center">
              <ListTree class="h-10 w-10 text-slate-300 dark:text-slate-600 mb-2" />
              <p class="text-xs font-semibold">{$_('nodeDetail.noPortsFound')}</p>
            </div>
          {:else}
            <!-- Ports Table with Sort & Pagination -->
            <div class="flex h-full flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden min-h-0">
              <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
                <table class="w-full text-left text-xs border-collapse font-mono">
                  <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePortsSort("Name")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colPort')}</span>
                          {#if portsSortCol === "Name"}
                            {#if portsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePortsSort("State")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colStatus')}</span>
                          {#if portsSortCol === "State"}
                            {#if portsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePortsSort("Speed")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colSpeed')}</span>
                          {#if portsSortCol === "Speed"}
                            {#if portsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePortsSort("InBytes")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colIn')}</span>
                          {#if portsSortCol === "InBytes"}
                            {#if portsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePortsSort("OutBytes")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colOut')}</span>
                          {#if portsSortCol === "OutBytes"}
                            {#if portsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-200 dark:divide-slate-800/60">
                    {#each paginatedPorts as pt}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="p-3 font-semibold text-slate-900 dark:text-slate-100">{pt.Name}</td>
                        <td class="p-3">
                          <span
                            class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-semibold text-[11px] {pt.State === 'up' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30' : 'bg-slate-100 dark:bg-slate-800 text-slate-500 border border-slate-200 dark:border-slate-700'}"
                          >
                            <span class="h-1.5 w-1.5 rounded-full {pt.State === 'up' ? 'bg-emerald-500' : 'bg-slate-400 dark:bg-slate-500'}"></span>
                            {pt.State.toUpperCase()}
                          </span>
                        </td>
                        <td class="p-3 text-blue-600 dark:text-cyan-400">{pt.Speed > 0 ? `${(pt.Speed / 1000000).toFixed(0)} Mbps` : "-"}</td>
                        <td class="p-3 text-slate-700 dark:text-slate-300">{pt.InBytes ? `${(pt.InBytes / 1024 / 1024).toFixed(2)} MB` : "-"}</td>
                        <td class="p-3 text-slate-700 dark:text-slate-300">{pt.OutBytes ? `${(pt.OutBytes / 1024 / 1024).toFixed(2)} MB` : "-"}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>

              <!-- Ports Pagination -->
              <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 shrink-0">
                <div class="flex items-center gap-2">
                  <span>{$_('nodeDetail.itemsPerPage')}:</span>
                  <select bind:value={portsPageSize} onchange={() => (portsPage = 1)} class="rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs text-slate-800 dark:text-slate-200">
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={-1}>{$_('nodeDetail.showAll')}</option>
                  </select>
                  <span class="font-mono text-[11px] ml-2">
                    {sortedPorts.length} {$_('nodeDetail.recordsUnit')} ({portsPageSize === -1 ? sortedPorts.length : Math.min(portsPage * portsPageSize, sortedPorts.length)} / {sortedPorts.length})
                  </span>
                </div>
                {#if portsPageSize !== -1 && portsTotalPages > 1}
                  <div class="flex items-center gap-1">
                    <button type="button" disabled={portsPage <= 1} onclick={() => (portsPage = 1)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsLeft class="h-4 w-4" /></button>
                    <button type="button" disabled={portsPage <= 1} onclick={() => portsPage--} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronLeft class="h-4 w-4" /></button>
                    <span class="px-2 font-mono text-[11px]">{portsPage} / {portsTotalPages}</span>
                    <button type="button" disabled={portsPage >= portsTotalPages} onclick={() => portsPage++} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronRight class="h-4 w-4" /></button>
                    <button type="button" disabled={portsPage >= portsTotalPages} onclick={() => (portsPage = portsTotalPages)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsRight class="h-4 w-4" /></button>
                  </div>
                {/if}
              </div>
            </div>
          {/if}

        <!-- 4. Polling Tab (Table View with Sort & Pagination) -->
        {:else if activeTab === "polling"}
          {#if isLoadingPollings}
            <div class="flex h-full items-center justify-center text-xs text-slate-500">
              <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 dark:border-cyan-400 mr-2"></div>
              {$_('common.loading')}
            </div>
          {:else if nodePollings.length === 0}
            <div class="flex h-full items-center justify-center text-xs text-slate-500">
              {$_('nodeDetail.noPolling')}
            </div>
          {:else}
            <div class="flex h-full flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden min-h-0">
              <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="py-1.5 px-3 w-28 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("state")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('list.table.status')}</span>
                          {#if pollSortCol === "state"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("name")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('list.table.pollingName')}</span>
                          {#if pollSortCol === "name"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 w-24 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("type")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colType')}</span>
                          {#if pollSortCol === "type"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("target")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colTarget')}</span>
                          {#if pollSortCol === "target"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 w-28 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("last_val")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colLastVal')}</span>
                          {#if pollSortCol === "last_val"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1 px-2 w-40 whitespace-nowrap cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handlePollSort("last_time")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colLastTime')}</span>
                          {#if pollSortCol === "last_time"}
                            {#if pollSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
                    {#each paginatedPollings as p}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="py-1 px-2 whitespace-nowrap">
                          <span
                            class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold border"
                            style="background-color: {getStateColor((p as PollingEnt).state)}18; border-color: {getStateColor((p as PollingEnt).state)}50; color: {getStateColor((p as PollingEnt).state)}"
                          >
                            <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor((p as PollingEnt).state)}"></span>
                            {getStateName((p as PollingEnt).state, $_)}
                          </span>
                        </td>
                        <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{(p as PollingEnt).name}</td>
                        <td class="py-1 px-2 whitespace-nowrap">
                          <span class="rounded-md px-2 py-0.5 text-[10px] font-semibold uppercase bg-blue-50 dark:bg-cyan-500/10 text-blue-700 dark:text-cyan-300 border border-blue-200 dark:border-cyan-500/30">
                            {(p as PollingEnt).type}
                          </span>
                        </td>
                        <td class="py-1 px-2 text-slate-600 dark:text-slate-400 truncate max-w-xs text-[11px]">{(p as PollingEnt).params || (p as PollingEnt).target || "-"}</td>
                        <td class="py-1 px-2 text-blue-600 dark:text-cyan-400 font-semibold whitespace-nowrap text-[11px]">{(p as PollingEnt).last_val ?? "-"}</td>
                        <td class="py-1 px-2 text-slate-500 text-[11px] whitespace-nowrap">{(p as PollingEnt).last_time ? formatTimeStr((p as PollingEnt).last_time) : "-"}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>

              <!-- Polling Pagination -->
              <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 shrink-0">
                <div class="flex items-center gap-2">
                  <span>{$_('nodeDetail.itemsPerPage')}:</span>
                  <select bind:value={pollPageSize} onchange={() => (pollPage = 1)} class="rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs text-slate-800 dark:text-slate-200">
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={-1}>{$_('nodeDetail.showAll')}</option>
                  </select>
                  <span class="font-mono text-[11px] ml-2">
                    {sortedPollings.length} {$_('nodeDetail.recordsUnit')} ({pollPageSize === -1 ? sortedPollings.length : Math.min(pollPage * pollPageSize, sortedPollings.length)} / {sortedPollings.length})
                  </span>
                </div>
                {#if pollPageSize !== -1 && pollTotalPages > 1}
                  <div class="flex items-center gap-1">
                    <button type="button" disabled={pollPage <= 1} onclick={() => (pollPage = 1)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsLeft class="h-4 w-4" /></button>
                    <button type="button" disabled={pollPage <= 1} onclick={() => pollPage--} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronLeft class="h-4 w-4" /></button>
                    <span class="px-2 font-mono text-[11px]">{pollPage} / {pollTotalPages}</span>
                    <button type="button" disabled={pollPage >= pollTotalPages} onclick={() => pollPage++} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronRight class="h-4 w-4" /></button>
                    <button type="button" disabled={pollPage >= pollTotalPages} onclick={() => (pollPage = pollTotalPages)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsRight class="h-4 w-4" /></button>
                  </div>
                {/if}
              </div>
            </div>
          {/if}

        <!-- 5. Logs Tab (Table View with Sort & Pagination) -->
        {:else if activeTab === "logs"}
          {#if isLoadingLogs}
            <div class="flex h-full items-center justify-center text-xs text-slate-500 font-sans">
              <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 dark:border-cyan-400 mr-2"></div>
              {$_('common.loading')}
            </div>
          {:else if nodeLogs.length === 0}
            <div class="flex h-full items-center justify-center text-slate-500 font-sans text-xs">
              {$_('nodeDetail.noLogs')}
            </div>
          {:else}
            <div class="flex h-full flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden min-h-0">
              <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="py-1.5 px-3 w-20 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handleLogsSort("level")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colLevel')}</span>
                          {#if logsSortCol === "level"}
                            {#if logsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 w-40 whitespace-nowrap cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handleLogsSort("time")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colTime')}</span>
                          {#if logsSortCol === "time"}
                            {#if logsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 w-24 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handleLogsSort("type")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colType')}</span>
                          {#if logsSortCol === "type"}
                            {#if logsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-1.5 px-3 cursor-pointer select-none hover:text-slate-900 dark:hover:text-slate-200" onclick={() => handleLogsSort("event")}>
                        <div class="inline-flex items-center gap-1">
                          <span>{$_('nodeDetail.colEvent')}</span>
                          {#if logsSortCol === "event"}
                            {#if logsSortDir === "asc"}<ArrowUp class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{:else}<ArrowDown class="h-3 w-3 text-blue-600 dark:text-cyan-400" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 text-slate-400" />
                          {/if}
                        </div>
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
                    {#each paginatedLogs as l}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="py-1 px-2 whitespace-nowrap">
                          <span
                            class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-bold uppercase border"
                            style="background-color: {getStateColor((l as EventLogEnt).level)}18; border-color: {getStateColor((l as EventLogEnt).level)}50; color: {getStateColor((l as EventLogEnt).level)}"
                          >
                            {(l as EventLogEnt).level}
                          </span>
                        </td>
                        <td class="py-1 px-2 text-blue-600 dark:text-cyan-400 font-semibold text-[11px] whitespace-nowrap">{formatTimeStr((l as EventLogEnt).time)}</td>
                        <td class="py-1 px-2 whitespace-nowrap">
                          <span class="font-semibold uppercase text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-950 px-1.5 py-0.5 rounded border border-slate-200 dark:border-slate-800 text-[10px]">
                            {(l as EventLogEnt).type}
                          </span>
                        </td>
                        <td class="py-1 px-2 font-sans text-xs text-slate-800 dark:text-slate-100">{(l as EventLogEnt).event}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>

              <!-- Logs Pagination -->
              <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 shrink-0">
                <div class="flex items-center gap-2">
                  <span>{$_('nodeDetail.itemsPerPage')}:</span>
                  <select bind:value={logsPageSize} onchange={() => (logsPage = 1)} class="rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs text-slate-800 dark:text-slate-200">
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={-1}>{$_('nodeDetail.showAll')}</option>
                  </select>
                  <span class="font-mono text-[11px] ml-2">
                    {sortedLogs.length} {$_('nodeDetail.recordsUnit')} ({logsPageSize === -1 ? sortedLogs.length : Math.min(logsPage * logsPageSize, sortedLogs.length)} / {sortedLogs.length})
                  </span>
                </div>
                {#if logsPageSize !== -1 && logsTotalPages > 1}
                  <div class="flex items-center gap-1">
                    <button type="button" disabled={logsPage <= 1} onclick={() => (logsPage = 1)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsLeft class="h-4 w-4" /></button>
                    <button type="button" disabled={logsPage <= 1} onclick={() => logsPage--} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronLeft class="h-4 w-4" /></button>
                    <span class="px-2 font-mono text-[11px]">{logsPage} / {logsTotalPages}</span>
                    <button type="button" disabled={logsPage >= logsTotalPages} onclick={() => logsPage++} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronRight class="h-4 w-4" /></button>
                    <button type="button" disabled={logsPage >= logsTotalPages} onclick={() => (logsPage = logsTotalPages)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsRight class="h-4 w-4" /></button>
                  </div>
                {/if}
              </div>
            </div>
          {/if}

        <!-- 6. Host Resource Tab (System, Storage, Device, FileSystem, Process) -->
        {:else if activeTab === "hostinfo"}
          {#if !isSnmpConfigured}
            <div class="flex h-full flex-col items-center justify-center rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-8 text-center shadow-sm">
              <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-amber-500/10 border border-amber-500/30 text-amber-500 mb-4">
                <ShieldAlert class="h-8 w-8" />
              </div>
              <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 mb-1.5">
                {$_('nodeDetail.snmpNotSupportedTitle')}
              </h3>
              <p class="text-xs text-slate-500 dark:text-slate-400 max-w-md leading-relaxed">
                {$_('nodeDetail.snmpNotSupportedDesc')}
              </p>
            </div>
          {:else if isLoadingHostResource}
            <div class="flex h-full items-center justify-center text-xs text-slate-500">
              <div class="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600 dark:border-cyan-400 mr-2"></div>
              {$_('common.loading')}
            </div>
          {:else if !hostResource || (currentHrList.length === 0 && !hostResourceError)}
            <div class="flex h-full flex-col items-center justify-center text-slate-400 p-8 text-center">
              <Server class="h-10 w-10 text-slate-300 dark:text-slate-600 mb-2" />
              <p class="text-xs font-semibold">{$_('nodeDetail.noHostResourceFound')}</p>
              {#if hostResourceError}
                <p class="text-[11px] text-rose-500 mt-1">{hostResourceError}</p>
              {/if}
            </div>
          {:else}
            <!-- Host Resource Sub-Tabs -->
            <div class="flex h-full flex-col min-h-0 space-y-3">
              <div class="flex items-center gap-1.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-950 p-1 text-xs shrink-0 self-start">
                <button
                  type="button"
                  onclick={() => { hrSubTab = "system"; hrSortCol = "Index"; hrSortDir = "asc"; hrPage = 1; }}
                  class="rounded-md px-3 py-1 font-semibold transition-all cursor-pointer {hrSubTab === 'system' ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'}"
                >
                  {$_('nodeDetail.tabSystem')} ({hostResource.System?.length || 0})
                </button>
                <button
                  type="button"
                  onclick={() => { hrSubTab = "storage"; hrSortCol = "Index"; hrSortDir = "asc"; hrPage = 1; }}
                  class="rounded-md px-3 py-1 font-semibold transition-all cursor-pointer {hrSubTab === 'storage' ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'}"
                >
                  {$_('nodeDetail.tabStorage')} ({hostResource.Storage?.length || 0})
                </button>
                <button
                  type="button"
                  onclick={() => { hrSubTab = "device"; hrSortCol = "Index"; hrSortDir = "asc"; hrPage = 1; }}
                  class="rounded-md px-3 py-1 font-semibold transition-all cursor-pointer {hrSubTab === 'device' ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'}"
                >
                  {$_('nodeDetail.tabDevice')} ({hostResource.Device?.length || 0})
                </button>
                <button
                  type="button"
                  onclick={() => { hrSubTab = "filesystem"; hrSortCol = "Index"; hrSortDir = "asc"; hrPage = 1; }}
                  class="rounded-md px-3 py-1 font-semibold transition-all cursor-pointer {hrSubTab === 'filesystem' ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'}"
                >
                  {$_('nodeDetail.tabFileSystem')} ({hostResource.FileSystem?.length || 0})
                </button>
                <button
                  type="button"
                  onclick={() => { hrSubTab = "process"; hrSortCol = "PID"; hrSortDir = "asc"; hrPage = 1; }}
                  class="rounded-md px-3 py-1 font-semibold transition-all cursor-pointer {hrSubTab === 'process' ? 'bg-blue-600 text-white shadow-sm' : 'text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200'}"
                >
                  {$_('nodeDetail.tabProcess')} ({hostResource.Process?.length || 0})
                </button>
              </div>

              <!-- Host Resource Sub-Table Container -->
              <div class="flex-1 flex flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm overflow-hidden min-h-0">
                <div class="flex-1 overflow-y-auto overflow-x-auto min-h-0">
                  <table class="w-full text-left text-xs border-collapse">
                    {#if hrSubTab === "system"}
                      <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                        <tr>
                          <th class="py-1 px-2 w-16 cursor-pointer select-none" onclick={() => handleHrSort("Index")}>
                            No
                          </th>
                          <th class="py-1 px-2 w-1/3 cursor-pointer select-none" onclick={() => handleHrSort("Key")}>
                            <div class="inline-flex items-center gap-1">
                              <span>{$_('nodeDetail.colKey')}</span>
                              {#if hrSortCol === "Key"}{#if hrSortDir === "asc"}<ArrowUp class="h-3 w-3" />{:else}<ArrowDown class="h-3 w-3" />{/if}{/if}
                            </div>
                          </th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Value")}>
                            <div class="inline-flex items-center gap-1">
                              <span>{$_('nodeDetail.colValue')}</span>
                              {#if hrSortCol === "Value"}{#if hrSortDir === "asc"}<ArrowUp class="h-3 w-3" />{:else}<ArrowDown class="h-3 w-3" />{/if}{/if}
                            </div>
                          </th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                        {#each paginatedHrList as sys}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            <td class="py-1 px-2 text-slate-400">{"Index" in sys ? sys.Index : ""}</td>
                            <td class="py-1 px-2 font-bold font-sans text-slate-800 dark:text-slate-200">{"Key" in sys ? sys.Key : ""}</td>
                            <td class="py-1 px-2 text-blue-600 dark:text-cyan-400">{"Value" in sys ? sys.Value : ""}</td>
                          </tr>
                        {/each}
                      </tbody>

                    {:else if hrSubTab === "storage"}
                      <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                        <tr>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Descr")}>{$_('nodeDetail.descr')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Type")}>{$_('nodeDetail.colType')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Size")}>{$_('nodeDetail.colSize')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Used")}>{$_('nodeDetail.colUsed')}</th>
                          <th class="py-1 px-2 w-36 cursor-pointer select-none" onclick={() => handleHrSort("Rate")}>{$_('nodeDetail.colRate')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                        {#each paginatedHrList as s}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            <td class="py-1 px-2 font-sans font-semibold text-slate-800 dark:text-slate-200">{s.Descr}</td>
                            <td class="py-1 px-2 text-slate-500">{s.Type}</td>
                            <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{(s.Size / 1024 / 1024).toFixed(1)} MB</td>
                            <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{(s.Used / 1024 / 1024).toFixed(1)} MB</td>
                            <td class="py-1 px-2">
                              <div class="flex items-center gap-2">
                                <div class="flex-1 h-2 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                                  <div class="h-full rounded-full {s.Rate >= 90 ? 'bg-rose-500' : s.Rate >= 75 ? 'bg-amber-500' : 'bg-blue-600 dark:bg-cyan-500'}" style="width: {Math.min(100, Math.max(0, s.Rate))}%"></div>
                                </div>
                                <span class="text-[11px] font-bold w-12 text-right">{s.Rate.toFixed(1)}%</span>
                              </div>
                            </td>
                          </tr>
                        {/each}
                      </tbody>

                    {:else if hrSubTab === "device"}
                      <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                        <tr>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Descr")}>{$_('nodeDetail.descr')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Type")}>{$_('nodeDetail.colType')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Status")}>{$_('nodeDetail.status')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Errors")}>{$_('nodeDetail.errors')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                        {#each paginatedHrList as d}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            <td class="py-1 px-2 font-sans font-semibold text-slate-800 dark:text-slate-200">{d.Descr}</td>
                            <td class="py-1 px-2 text-slate-500">{d.Type}</td>
                            <td class="py-1 px-2">
                              <span class="rounded px-2 py-0.5 text-[10px] font-semibold uppercase {d.Status === 'running' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-slate-100 dark:bg-slate-800 text-slate-500'}">
                                {d.Status}
                              </span>
                            </td>
                            <td class="py-1 px-2 text-slate-400">{d.Errors || "-"}</td>
                          </tr>
                        {/each}
                      </tbody>

                    {:else if hrSubTab === "filesystem"}
                      <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                        <tr>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Mount")}>{$_('nodeDetail.colMount')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Remote")}>{$_('nodeDetail.colRemote')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Type")}>{$_('nodeDetail.colType')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                        {#each paginatedHrList as f}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            <td class="py-1 px-2 font-sans font-bold text-slate-800 dark:text-slate-200">{f.Mount}</td>
                            <td class="py-1 px-2 text-slate-500">{f.Remote || "-"}</td>
                            <td class="py-1 px-2 text-blue-600 dark:text-cyan-400">{f.Type}</td>
                          </tr>
                        {/each}
                      </tbody>

                    {:else if hrSubTab === "process"}
                      <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                        <tr>
                          <th class="py-1 px-2 w-20 cursor-pointer select-none" onclick={() => handleHrSort("PID")}>{$_('nodeDetail.colPID')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Name")}>{$_('nodeDetail.name')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Status")}>{$_('nodeDetail.status')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("CPU")}>{$_('nodeDetail.colCPU')}</th>
                          <th class="py-1 px-2 cursor-pointer select-none" onclick={() => handleHrSort("Mem")}>{$_('nodeDetail.colMem')}</th>
                        </tr>
                      </thead>
                      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                        {#each paginatedHrList as pr}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            <td class="py-1 px-2 text-slate-400">{pr.PID}</td>
                            <td class="py-1 px-2 font-bold font-sans text-slate-900 dark:text-slate-100">{pr.Name}</td>
                            <td class="py-1 px-2">
                              <span class="rounded px-2 py-0.5 text-[10px] font-semibold uppercase {pr.Status === 'running' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-slate-100 dark:bg-slate-800 text-slate-500'}">
                                {pr.Status}
                              </span>
                            </td>
                            <td class="py-1 px-2 text-blue-600 dark:text-cyan-400">{pr.CPU}</td>
                            <td class="py-1 px-2 text-slate-700 dark:text-slate-300">{(pr.Mem / 1024).toFixed(1)} MB</td>
                          </tr>
                        {/each}
                      </tbody>
                    {/if}
                  </table>
                </div>

                <!-- Host Resource Pagination -->
                <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 shrink-0">
                  <div class="flex items-center gap-2">
                    <span>{$_('nodeDetail.itemsPerPage')}:</span>
                    <select bind:value={hrPageSize} onchange={() => (hrPage = 1)} class="rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs text-slate-800 dark:text-slate-200">
                      <option value={10}>10</option>
                      <option value={25}>25</option>
                      <option value={50}>50</option>
                      <option value={-1}>{$_('nodeDetail.showAll')}</option>
                    </select>
                    <span class="font-mono text-[11px] ml-2">
                      {sortedHrList.length} {$_('nodeDetail.recordsUnit')} ({hrPageSize === -1 ? sortedHrList.length : Math.min(hrPage * hrPageSize, sortedHrList.length)} / {sortedHrList.length})
                    </span>
                  </div>
                  {#if hrPageSize !== -1 && hrTotalPages > 1}
                    <div class="flex items-center gap-1">
                      <button type="button" disabled={hrPage <= 1} onclick={() => (hrPage = 1)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsLeft class="h-4 w-4" /></button>
                      <button type="button" disabled={hrPage <= 1} onclick={() => hrPage--} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronLeft class="h-4 w-4" /></button>
                      <span class="px-2 font-mono text-[11px]">{hrPage} / {hrTotalPages}</span>
                      <button type="button" disabled={hrPage >= hrTotalPages} onclick={() => hrPage++} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronRight class="h-4 w-4" /></button>
                      <button type="button" disabled={hrPage >= hrTotalPages} onclick={() => (hrPage = hrTotalPages)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsRight class="h-4 w-4" /></button>
                    </div>
                  {/if}
                </div>
              </div>
            </div>
          {/if}

        <!-- 7. RMON Tab -->
        {:else if activeTab === "rmon"}
          {#if !isSnmpConfigured}
            <div class="flex h-full flex-col items-center justify-center text-slate-500">
              <ShieldAlert class="h-10 w-10 text-amber-500 mb-2" />
              <p class="font-semibold text-sm">{$_('nodeDetail.snmpNotSupported')}</p>
              <p class="text-xs text-slate-400 mt-1">{$_('nodeDetail.snmpConfigureHint')}</p>
            </div>
          {:else if isLoadingRmon}
            <div class="flex h-full items-center justify-center">
              <div class="flex flex-col items-center gap-2">
                <RotateCw class="h-6 w-6 animate-spin text-blue-600 dark:text-cyan-400" />
                <span class="text-xs text-slate-500">{$_('common.loading')}</span>
              </div>
            </div>
          {:else if rmonError}
            <div class="flex h-full flex-col items-center justify-center text-slate-500">
              <AlertCircle class="h-10 w-10 text-rose-500 mb-2" />
              <p class="font-semibold text-sm text-rose-600 dark:text-rose-400">{rmonError}</p>
              <button
                type="button"
                onclick={loadRmonData}
                class="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-blue-600 text-white text-xs font-semibold cursor-pointer"
              >
                <RotateCw class="h-3.5 w-3.5" />
                {$_('common.retry')}
              </button>
            </div>
          {:else if rmonStats.length === 0}
            <div class="flex h-full flex-col items-center justify-center text-slate-500">
              <Gauge class="h-10 w-10 text-slate-400 mb-2" />
              <p class="font-semibold text-sm">{$_('nodeDetail.noRmonData')}</p>
              <p class="text-xs text-slate-400 mt-1">{$_('nodeDetail.noRmonDataHint')}</p>
              <button
                type="button"
                onclick={loadRmonData}
                class="mt-3 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-700 dark:text-slate-200 text-xs font-semibold cursor-pointer"
              >
                <RotateCw class="h-3.5 w-3.5" />
                {$_('common.reload')}
              </button>
            </div>
          {:else}
            <div class="flex h-full flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 overflow-hidden shadow-sm">
              <div class="flex-1 overflow-auto">
                <table class="w-full text-left text-xs border-collapse">
                  <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="py-2 px-2.5 cursor-pointer select-none" onclick={() => handleRmonSort("Index")}>
                        <div class="flex items-center gap-1">
                          <span>Index</span>
                          {#if rmonSortCol === "Index"}
                            {#if rmonSortDir === "asc"}<ArrowUp class="h-3 w-3" />{:else}<ArrowDown class="h-3 w-3" />{/if}
                          {:else}
                            <ArrowUpDown class="h-3 w-3 opacity-30" />
                          {/if}
                        </div>
                      </th>
                      <th class="py-2 px-2.5 cursor-pointer select-none" onclick={() => handleRmonSort("DataSource")}>{$_('nodeDetail.rmonDataSource')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("DropEvents")}>{$_('nodeDetail.rmonDropEvents')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("Octets")}>{$_('nodeDetail.rmonOctets')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("Pkts")}>{$_('nodeDetail.rmonPkts')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("BroadcastPkts")}>{$_('nodeDetail.rmonBroadcast')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("MulticastPkts")}>{$_('nodeDetail.rmonMulticast')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("CRCAlignErrors")}>{$_('nodeDetail.rmonCRCErrors')}</th>
                      <th class="py-2 px-2.5 cursor-pointer select-none text-right" onclick={() => handleRmonSort("Collisions")}>{$_('nodeDetail.rmonCollisions')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                    {#each paginatedRmon as r}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                        <td class="py-1.5 px-2.5 font-bold text-slate-900 dark:text-slate-100">{r.Index}</td>
                        <td class="py-1.5 px-2.5 text-slate-600 dark:text-slate-400 font-sans">{r.DataSource || "-"}</td>
                        <td class="py-1.5 px-2.5 text-right {r.DropEvents > 0 ? 'text-rose-600 font-bold' : 'text-slate-500'}">{r.DropEvents.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right text-slate-700 dark:text-slate-300">{r.Octets.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right font-bold text-blue-600 dark:text-cyan-400">{r.Pkts.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right text-slate-600 dark:text-slate-400">{r.BroadcastPkts.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right text-slate-600 dark:text-slate-400">{r.MulticastPkts.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right {r.CRCAlignErrors > 0 ? 'text-rose-600 font-bold' : 'text-slate-500'}">{r.CRCAlignErrors.toLocaleString()}</td>
                        <td class="py-1.5 px-2.5 text-right {r.Collisions > 0 ? 'text-amber-600 font-bold' : 'text-slate-500'}">{r.Collisions.toLocaleString()}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>

              <!-- RMON Pagination -->
              <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 px-4 py-2 text-xs text-slate-500 dark:text-slate-400 shrink-0">
                <div class="flex items-center gap-2">
                  <span>{$_('nodeDetail.itemsPerPage')}:</span>
                  <select bind:value={rmonPageSize} onchange={() => (rmonPage = 1)} class="rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2 py-0.5 text-xs text-slate-800 dark:text-slate-200">
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={-1}>{$_('nodeDetail.showAll')}</option>
                  </select>
                  <span class="font-mono text-[11px] ml-2">
                    {sortedRmon.length} {$_('nodeDetail.recordsUnit')} ({rmonPageSize === -1 ? sortedRmon.length : Math.min(rmonPage * rmonPageSize, sortedRmon.length)} / {sortedRmon.length})
                  </span>
                </div>
                {#if rmonPageSize !== -1 && rmonTotalPages > 1}
                  <div class="flex items-center gap-1">
                    <button type="button" disabled={rmonPage <= 1} onclick={() => (rmonPage = 1)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsLeft class="h-4 w-4" /></button>
                    <button type="button" disabled={rmonPage <= 1} onclick={() => rmonPage--} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronLeft class="h-4 w-4" /></button>
                    <span class="px-2 font-mono text-[11px]">{rmonPage} / {rmonTotalPages}</span>
                    <button type="button" disabled={rmonPage >= rmonTotalPages} onclick={() => rmonPage++} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronRight class="h-4 w-4" /></button>
                    <button type="button" disabled={rmonPage >= rmonTotalPages} onclick={() => (rmonPage = rmonTotalPages)} class="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"><ChevronsRight class="h-4 w-4" /></button>
                  </div>
                {/if}
              </div>
            </div>
          {/if}

        <!-- 8. Diagnose Tab -->
        {:else if activeTab === "diagnose"}
          <div class="flex h-full flex-col gap-4 overflow-y-auto pr-1">
            <!-- Header Action Card -->
            <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-4 shadow-sm">
              <div class="flex items-center gap-3">
                <div class="flex h-10 w-10 items-center justify-center rounded-xl {diagnoseResult?.status === 'healthy' ? 'bg-emerald-500/10 text-emerald-500 border border-emerald-500/20' : diagnoseResult?.status === 'warning' ? 'bg-amber-500/10 text-amber-500 border border-amber-500/20' : diagnoseResult?.status === 'critical' ? 'bg-rose-500/10 text-rose-500 border border-rose-500/20' : 'bg-blue-500/10 text-blue-500 border border-blue-500/20'}">
                  <Stethoscope class="h-5 w-5" />
                </div>
                <div>
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-bold text-slate-900 dark:text-slate-100">{$_('nodeDetail.diagnoseTitle')}</span>
                    {#if diagnoseResult}
                      <span class="rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase {diagnoseResult.status === 'healthy' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30' : diagnoseResult.status === 'warning' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30' : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/30'}">
                        {diagnoseResult.status}
                      </span>
                    {/if}
                  </div>
                  <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                    {diagnoseResult?.summary || $_('nodeDetail.diagnoseSubtitle')}
                  </p>
                </div>
              </div>

              <button
                type="button"
                disabled={isDiagnosing}
                onclick={runNodeDiagnose}
                class="inline-flex items-center gap-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 text-xs font-semibold shadow-sm transition-all disabled:opacity-50 cursor-pointer"
              >
                <RotateCw class="h-3.5 w-3.5 {isDiagnosing ? 'animate-spin' : ''}" />
                {isDiagnosing ? $_('common.running') : $_('nodeDetail.runDiagnose')}
              </button>
            </div>

            {#if isDiagnosing && !diagnoseResult}
              <div class="flex flex-1 items-center justify-center p-8">
                <div class="flex flex-col items-center gap-3">
                  <RotateCw class="h-8 w-8 animate-spin text-blue-600 dark:text-cyan-400" />
                  <span class="text-xs font-semibold text-slate-600 dark:text-slate-300">{$_('nodeDetail.diagnosingProbes')}</span>
                </div>
              </div>
            {:else if diagnoseError}
              <div class="rounded-xl border border-rose-200 dark:border-rose-900 bg-rose-50/50 dark:bg-rose-950/20 p-4 text-rose-600 dark:text-rose-400 text-xs">
                {diagnoseError}
              </div>
            {:else if diagnoseResult}
              <!-- Probes Result Grid -->
              <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
                <!-- Ping Probe Card -->
                <div class="flex flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 p-4 shadow-sm">
                  <div class="flex items-center justify-between mb-2">
                    <span class="font-bold text-xs text-slate-800 dark:text-slate-200">ICMP / Ping Probe</span>
                    {#if diagnoseResult.ping.success}
                      <span class="inline-flex items-center gap-1 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                        <CheckCircle2 class="h-3 w-3" />
                        {$_('nodeDetail.probeSuccess')}
                      </span>
                    {:else}
                      <span class="inline-flex items-center gap-1 text-[10px] font-semibold text-rose-600 dark:text-rose-400 bg-rose-500/10 px-2 py-0.5 rounded-full border border-rose-500/20">
                        <AlertCircle class="h-3 w-3" />
                        {$_('nodeDetail.probeFailed')}
                      </span>
                    {/if}
                  </div>
                  <div class="space-y-1.5 text-[11px] font-mono">
                    <div class="flex justify-between text-slate-600 dark:text-slate-400">
                      <span>{$_('nodeDetail.pingPackets')}:</span>
                      <span>{diagnoseResult.ping.received} / {diagnoseResult.ping.sent}</span>
                    </div>
                    <div class="flex justify-between text-slate-600 dark:text-slate-400">
                      <span>{$_('nodeDetail.pingLoss')}:</span>
                      <span class="{diagnoseResult.ping.loss > 0 ? 'text-rose-500 font-bold' : ''}">{diagnoseResult.ping.loss.toFixed(1)}%</span>
                    </div>
                    <div class="flex justify-between text-slate-600 dark:text-slate-400">
                      <span>{$_('nodeDetail.pingAvgRtt')}:</span>
                      <span class="font-bold text-blue-600 dark:text-cyan-400">{diagnoseResult.ping.avgRtt.toFixed(2)} ms</span>
                    </div>
                    {#if diagnoseResult.ping.error}
                      <p class="text-[10px] text-rose-500 font-sans mt-1">{diagnoseResult.ping.error}</p>
                    {/if}
                  </div>
                </div>

                <!-- SNMP Probe Card -->
                <div class="flex flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 p-4 shadow-sm">
                  <div class="flex items-center justify-between mb-2">
                    <span class="font-bold text-xs text-slate-800 dark:text-slate-200">SNMP Probe</span>
                    {#if !diagnoseResult.snmp.configured}
                      <span class="text-[10px] text-slate-400 bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-full">
                        {$_('nodeDetail.notConfigured')}
                      </span>
                    {:else if diagnoseResult.snmp.success}
                      <span class="inline-flex items-center gap-1 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded-full border border-emerald-500/20">
                        <CheckCircle2 class="h-3 w-3" />
                        {$_('nodeDetail.probeSuccess')}
                      </span>
                    {:else}
                      <span class="inline-flex items-center gap-1 text-[10px] font-semibold text-rose-600 dark:text-rose-400 bg-rose-500/10 px-2 py-0.5 rounded-full border border-rose-500/20">
                        <AlertCircle class="h-3 w-3" />
                        {$_('nodeDetail.probeFailed')}
                      </span>
                    {/if}
                  </div>
                  <div class="space-y-1.5 text-[11px] font-mono">
                    {#if diagnoseResult.snmp.configured}
                      <div class="flex justify-between text-slate-600 dark:text-slate-400">
                        <span>RTT:</span>
                        <span class="font-bold text-blue-600 dark:text-cyan-400">{diagnoseResult.snmp.rtt.toFixed(2)} ms</span>
                      </div>
                      {#if diagnoseResult.snmp.sysUpTime}
                        <div class="flex justify-between text-slate-600 dark:text-slate-400">
                          <span>UpTime:</span>
                          <span class="truncate ml-2">{diagnoseResult.snmp.sysUpTime}</span>
                        </div>
                      {/if}
                      {#if diagnoseResult.snmp.sysDescr}
                        <div class="text-[10px] text-slate-500 dark:text-slate-400 truncate mt-1">
                          {diagnoseResult.snmp.sysDescr}
                        </div>
                      {/if}
                      {#if diagnoseResult.snmp.error}
                        <p class="text-[10px] text-rose-500 font-sans mt-1">{diagnoseResult.snmp.error}</p>
                      {/if}
                    {:else}
                      <p class="text-xs text-slate-400 font-sans">{$_('nodeDetail.snmpNotSupported')}</p>
                    {/if}
                  </div>
                </div>

                <!-- Web HTTP / HTTPS Probe Card -->
                <div class="flex flex-col rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/60 p-4 shadow-sm">
                  <div class="flex items-center justify-between mb-2">
                    <span class="font-bold text-xs text-slate-800 dark:text-slate-200">Web Ports (80 / 443)</span>
                    <span class="text-[10px] text-slate-500">HTTP/HTTPS</span>
                  </div>
                  <div class="space-y-2 text-[11px] font-mono">
                    <!-- HTTP 80 -->
                    <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/60 pb-1.5">
                      <span class="text-slate-600 dark:text-slate-400">HTTP (80):</span>
                      {#if diagnoseResult.web.http?.success}
                        <span class="font-bold text-emerald-600 dark:text-emerald-400">{diagnoseResult.web.http.statusCode} ({diagnoseResult.web.http.rtt.toFixed(1)}ms)</span>
                      {:else}
                        <span class="text-slate-400">{$_('nodeDetail.noResponse')}</span>
                      {/if}
                    </div>

                    <!-- HTTPS 443 -->
                    <div>
                      <div class="flex items-center justify-between">
                        <span class="text-slate-600 dark:text-slate-400">HTTPS (443):</span>
                        {#if diagnoseResult.web.https?.success}
                          <span class="font-bold text-emerald-600 dark:text-emerald-400">{diagnoseResult.web.https.statusCode} ({diagnoseResult.web.https.rtt.toFixed(1)}ms)</span>
                        {:else}
                          <span class="text-slate-400">{$_('nodeDetail.noResponse')}</span>
                        {/if}
                      </div>
                      {#if diagnoseResult.web.https?.certIssuer}
                        <div class="text-[10px] text-slate-500 dark:text-slate-400 mt-1 truncate">
                          Issuer: {diagnoseResult.web.https.certIssuer}
                          {#if diagnoseResult.web.https.remainingDays !== undefined}
                            <span class="ml-1 {diagnoseResult.web.https.remainingDays < 30 ? 'text-amber-500 font-bold' : ''}">
                              ({diagnoseResult.web.https.remainingDays} days left)
                            </span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
      </div>

      <!-- Modal Footer (Matching twsnmpfk Close button) -->
      <div
        class="flex items-center justify-end border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-2.5 shrink-0"
      >
        <button
          type="button"
          onclick={() => (show = false)}
          class="inline-flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 shadow-sm hover:bg-slate-100 dark:hover:bg-slate-700 hover:text-slate-900 dark:hover:text-white transition-all cursor-pointer"
        >
          <X class="h-4 w-4" />
          {$_('nodeDetail.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
