<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    fetchNodes,
    sendWol,
    type NodeEnt,
  } from "../api";
  import PingDialog from "../components/PingDialog.svelte";
  import MIBBrowserDialog from "../components/MIBBrowserDialog.svelte";
  import GNMIToolDialog from "../components/GNMIToolDialog.svelte";
  import {
    Wrench,
    Activity,
    FolderTree,
    Terminal,
    Power,
    CheckCircle2,
    AlertCircle,
    Copy,
    Check,
    Send,
    RotateCw,
  } from "@lucide/svelte";

  type ToolTab = "ping" | "mib" | "gnmi" | "wol";
  let activeTool = $state<ToolTab>("ping");

  let nodes = $state<NodeEnt[]>([]);
  let selectedNodeId = $state<string>("");
  let selectedNode = $derived(nodes.find((n) => n.id === selectedNodeId) || null);

  // Dialog triggers for standalone tools embedded as primary full-screen utilities
  let showPingModal = $state(false);
  let showMibModal = $state(false);
  let showGnmiModal = $state(false);

  // WOL states
  let wolMac = $state("");
  let wolSending = $state(false);
  let wolMessage = $state<{ type: "success" | "error"; text: string } | null>(null);
  let wolHistory = $state<{ time: string; mac: string; success: boolean }[]>([]);

  onMount(async () => {
    try {
      nodes = await fetchNodes();
      if (nodes.length > 0) {
        selectedNodeId = nodes[0].id;
      }
    } catch (e) {
      console.error("Failed to fetch nodes for ToolView:", e);
    }
  });

  $effect(() => {
    if (selectedNode) {
      if (selectedNode.mac) {
        wolMac = selectedNode.mac;
      }
    }
  });

  const handleSendWol = async () => {
    if (!wolMac) return;
    wolSending = true;
    wolMessage = null;
    try {
      const res = await sendWol(wolMac.trim());
      wolMessage = { type: "success", text: $_("tools.wolSuccess") || "Magic packet sent successfully" };
      wolHistory = [
        { time: new Date().toLocaleTimeString(), mac: wolMac.trim(), success: true },
        ...wolHistory.slice(0, 9),
      ];
    } catch (err: any) {
      wolMessage = { type: "error", text: err?.message || String(err) };
      wolHistory = [
        { time: new Date().toLocaleTimeString(), mac: wolMac.trim(), success: false },
        ...wolHistory.slice(0, 9),
      ];
    } finally {
      wolSending = false;
    }
  };
</script>

<div class="flex h-full w-full flex-col overflow-hidden bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-100">
  <!-- Top Navigation & Target Selector Bar -->
  <div class="flex flex-wrap items-center justify-between border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-6 py-3 shrink-0 shadow-sm">
    <div class="flex items-center gap-3">
      <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 dark:bg-cyan-500/20 text-blue-600 dark:text-cyan-400 border border-blue-500/20 dark:border-cyan-500/30">
        <Wrench class="h-5 w-5" />
      </div>
      <div>
        <h1 class="text-base font-bold text-slate-900 dark:text-slate-100">
          {$_("tools.title") || "Network Diagnostic Tools"}
        </h1>
        <p class="text-xs text-slate-500 dark:text-slate-400">
          {$_("tools.subtitle") || "Standalone diagnostic suite: Ping, MIB Browser, gNMI, and Wake-on-LAN"}
        </p>
      </div>
    </div>

    <!-- Node Selector -->
    <div class="flex items-center gap-2">
      <span class="text-xs text-slate-500 dark:text-slate-400 font-medium">{$_("tools.targetNode") || "Target Node"}:</span>
      <select
        bind:value={selectedNodeId}
        class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
      >
        <option value="">{$_("tools.manualTarget") || "-- Manual IP / Host --"}</option>
        {#each nodes as n}
          <option value={n.id}>{n.name} ({n.ip})</option>
        {/each}
      </select>
    </div>
  </div>

  <!-- Sub Navigation Tabs -->
  <div class="flex border-b border-slate-200 dark:border-slate-800 bg-slate-100/80 dark:bg-slate-900/60 px-6 py-2 gap-2 shrink-0">
    <button
      type="button"
      onclick={() => (activeTool = "ping")}
      class="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTool === 'ping' ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200/60 dark:hover:bg-slate-800'}"
    >
      <Activity class="h-4 w-4" />
      <span>Ping Tool</span>
    </button>
    <button
      type="button"
      onclick={() => (activeTool = "mib")}
      class="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTool === 'mib' ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200/60 dark:hover:bg-slate-800'}"
    >
      <FolderTree class="h-4 w-4" />
      <span>MIB Browser</span>
    </button>
    <button
      type="button"
      onclick={() => (activeTool = "gnmi")}
      class="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTool === 'gnmi' ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200/60 dark:hover:bg-slate-800'}"
    >
      <Terminal class="h-4 w-4" />
      <span>gNMI Tool</span>
    </button>
    <button
      type="button"
      onclick={() => (activeTool = "wol")}
      class="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTool === 'wol' ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-200/60 dark:hover:bg-slate-800'}"
    >
      <Power class="h-4 w-4" />
      <span>Wake-on-LAN (WOL)</span>
    </button>
  </div>

  <!-- Main Tool Workspace -->
  <div class="flex-1 overflow-auto p-6 flex flex-col min-h-0">
    {#if activeTool === "ping"}
      <!-- Ping Standalone Panel -->
      <div class="flex flex-col h-full rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-6">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4 mb-4">
          <div>
            <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100">ICMP / Ping & MTR Analyzer</h2>
            <p class="text-xs text-slate-500">Real-time latency histogram, packet loss tracking, and traceroute hop analysis</p>
          </div>
          <button
            type="button"
            onclick={() => (showPingModal = true)}
            class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-sm transition-all cursor-pointer"
          >
            <Activity class="h-4 w-4" />
            Launch Ping Utility
          </button>
        </div>

        <div class="flex flex-1 flex-col items-center justify-center p-12 text-center text-slate-500 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl bg-slate-50/50 dark:bg-slate-950/30">
          <Activity class="h-12 w-12 text-blue-500/50 mb-3" />
          <h3 class="text-sm font-bold text-slate-700 dark:text-slate-300">Target: {selectedNode?.name || selectedNode?.ip || "Manual IP"}</h3>
          <p class="text-xs text-slate-400 max-w-md mt-1 mb-4">Click below to open the real-time interactive ping graph, configure packet payload size, timeout, and continuous monitoring modes.</p>
          <button
            type="button"
            onclick={() => (showPingModal = true)}
            class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-md transition-all cursor-pointer"
          >
            Start Ping Session
          </button>
        </div>
      </div>

    {:else if activeTool === "mib"}
      <!-- MIB Browser Standalone Panel -->
      <div class="flex flex-col h-full rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-6">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4 mb-4">
          <div>
            <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100">SNMP MIB Browser & Tree Explorer</h2>
            <p class="text-xs text-slate-500">Explore RFC standard MIBs and custom enterprise OID structures, execute SNMP Get / Walk / Table queries</p>
          </div>
          <button
            type="button"
            onclick={() => (showMibModal = true)}
            class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-sm transition-all cursor-pointer"
          >
            <FolderTree class="h-4 w-4" />
            Launch MIB Browser
          </button>
        </div>

        <div class="flex flex-1 flex-col items-center justify-center p-12 text-center text-slate-500 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl bg-slate-50/50 dark:bg-slate-950/30">
          <FolderTree class="h-12 w-12 text-blue-500/50 mb-3" />
          <h3 class="text-sm font-bold text-slate-700 dark:text-slate-300">Target Node: {selectedNode?.name || selectedNode?.ip || "Manual"}</h3>
          <p class="text-xs text-slate-400 max-w-md mt-1 mb-4">Query RFC1213-MIB, HOST-RESOURCES-MIB, IF-MIB, RMON-MIB, or user-uploaded enterprise MIB definitions with formatted ASN.1 translation.</p>
          <button
            type="button"
            onclick={() => (showMibModal = true)}
            class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-md transition-all cursor-pointer"
          >
            Open MIB Browser
          </button>
        </div>
      </div>

    {:else if activeTool === "gnmi"}
      <!-- gNMI Standalone Panel -->
      <div class="flex flex-col h-full rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-6">
        <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-4 mb-4">
          <div>
            <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100">gNMI (gRPC Network Management Interface) Explorer</h2>
            <p class="text-xs text-slate-500">Query streaming telemetry, capabilities, and Yang paths from modern networking switches and routers</p>
          </div>
          <button
            type="button"
            onclick={() => (showGnmiModal = true)}
            class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-sm transition-all cursor-pointer"
          >
            <Terminal class="h-4 w-4" />
            Launch gNMI Utility
          </button>
        </div>

        <div class="flex flex-1 flex-col items-center justify-center p-12 text-center text-slate-500 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl bg-slate-50/50 dark:bg-slate-950/30">
          <Terminal class="h-12 w-12 text-blue-500/50 mb-3" />
          <h3 class="text-sm font-bold text-slate-700 dark:text-slate-300">Target: {selectedNode?.name || selectedNode?.ip || "Manual"}</h3>
          <p class="text-xs text-slate-400 max-w-md mt-1 mb-4">Execute gNMI Capabilities discovery, streaming telemetry subscription, and Get requests against openconfig models.</p>
          <button
            type="button"
            onclick={() => (showGnmiModal = true)}
            class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-700 text-white text-xs font-semibold shadow-md transition-all cursor-pointer"
          >
            Open gNMI Explorer
          </button>
        </div>
      </div>

    {:else if activeTool === "wol"}
      <!-- Wake on LAN Dedicated View -->
      <div class="flex flex-col max-w-2xl mx-auto w-full rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm p-6 space-y-6">
        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500 border border-amber-500/20">
            <Power class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">Wake-on-LAN Magic Packet Dispatcher</h2>
            <p class="text-xs text-slate-500">Send standard broadcast magic packets (UDP port 9) to wake sleeping devices</p>
          </div>
        </div>

        <div class="space-y-4">
          <div>
            <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1" for="wol-mac-input">
              Target MAC Address
            </label>
            <div class="flex gap-2">
              <input
                id="wol-mac-input"
                type="text"
                placeholder="e.g. 00:11:22:33:44:55 or 00-11-22-33-44-55"
                bind:value={wolMac}
                class="flex-1 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3.5 py-2 font-mono text-xs text-slate-900 dark:text-white shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <button
                type="button"
                disabled={wolSending || !wolMac}
                onclick={handleSendWol}
                class="inline-flex items-center gap-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white px-5 py-2 text-xs font-semibold shadow-sm transition-all disabled:opacity-50 cursor-pointer"
              >
                <Send class="h-3.5 w-3.5 {wolSending ? 'animate-spin' : ''}" />
                {wolSending ? "Sending..." : "Send Magic Packet"}
              </button>
            </div>
            {#if selectedNode}
              <p class="text-[11px] text-slate-400 mt-1">
                Auto-populated from selected node: <span class="font-semibold text-slate-600 dark:text-slate-300">{selectedNode.name}</span> ({selectedNode.ip})
              </p>
            {/if}
          </div>

          {#if wolMessage}
            <div class="flex items-center gap-2 rounded-xl p-3 text-xs {wolMessage.type === 'success' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20' : 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20'}">
              {#if wolMessage.type === 'success'}
                <CheckCircle2 class="h-4 w-4 shrink-0" />
              {:else}
                <AlertCircle class="h-4 w-4 shrink-0" />
              {/if}
              <span>{wolMessage.text}</span>
            </div>
          {/if}

          <!-- WOL History -->
          {#if wolHistory.length > 0}
            <div class="pt-4 border-t border-slate-100 dark:border-slate-800">
              <h3 class="text-xs font-bold text-slate-700 dark:text-slate-300 mb-2">Recent Dispatches</h3>
              <div class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-[11px]">
                {#each wolHistory as h}
                  <div class="flex items-center justify-between py-1.5">
                    <span class="text-slate-500">{h.time}</span>
                    <span class="font-semibold text-slate-800 dark:text-slate-200">{h.mac}</span>
                    <span class="rounded px-2 py-0.5 text-[10px] font-semibold {h.success ? 'bg-emerald-500/10 text-emerald-600' : 'bg-rose-500/10 text-rose-600'}">
                      {h.success ? "SENT" : "FAILED"}
                    </span>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>

<!-- Embedded Tool Dialogs -->
<PingDialog
  bind:show={showPingModal}
  node={selectedNode}
/>

<MIBBrowserDialog
  bind:show={showMibModal}
  node={selectedNode}
/>

<GNMIToolDialog
  bind:show={showGnmiModal}
  node={selectedNode}
/>
