<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Activity,
    Server,
    AlertTriangle,
    Network,
    BarChart3,
    Sparkles,
    Shield,
    RefreshCw,
    Radio
  } from '@lucide/svelte';
  import type { NodeEnt, LineEnt, PollingEnt, EventLogEnt, SystemHealth } from '$lib/api';
  import { fetchHealth, fetchNodes, fetchLines, fetchPollings, fetchEventLogs } from '$lib/api';

  import TopologyMap from '$lib/map/TopologyMap.svelte';
  import MetricCharts from '$lib/charts/MetricCharts.svelte';
  import AlertGrid from '$lib/components/AlertGrid.svelte';
  import NodeTable from '$lib/components/NodeTable.svelte';
  import AIAssistant from '$lib/mcp/AIAssistant.svelte';

  // Navigation tabs
  type Tab = 'dashboard' | 'map' | 'nodes' | 'alerts' | 'ai';
  let activeTab = $state<Tab>('dashboard');

  // App state
  let health = $state<SystemHealth | null>(null);
  let nodes = $state<NodeEnt[]>([]);
  let lines = $state<LineEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let eventLogs = $state<EventLogEnt[]>([]);
  let selectedNode = $state<NodeEnt | null>(null);
  let isRefreshing = $state(false);

  async function loadData() {
    isRefreshing = true;
    try {
      const [h, n, l, p, e] = await Promise.all([
        fetchHealth().catch(() => ({ status: 'Online', time: new Date().toISOString(), version: 'v0.1.0' })),
        fetchNodes().catch(() => []),
        fetchLines().catch(() => []),
        fetchPollings().catch(() => []),
        fetchEventLogs().catch(() => []),
      ]);

      health = h;
      pollings = p;
      eventLogs = e;
      lines = l;

      // If database is completely empty on first launch, provide initial topology
      if (n.length === 0) {
        nodes = [
          { id: 'gw-1', name: 'Core Gateway', ip: '192.168.1.1', state: 'normal', x: 260, y: 160 },
          { id: 'sw-1', name: 'Dist Switch A', ip: '192.168.1.10', state: 'normal', x: 160, y: 280 },
          { id: 'sw-2', name: 'Dist Switch B', ip: '192.168.1.20', state: 'warn', x: 380, y: 280 },
          { id: 'srv-1', name: 'App Server', ip: '192.168.1.50', state: 'normal', x: 100, y: 400 },
          { id: 'srv-2', name: 'Database Primary', ip: '192.168.1.60', state: 'normal', x: 220, y: 400 },
          { id: 'edge-1', name: 'Branch Edge Router', ip: '192.168.1.99', state: 'error', x: 440, y: 400 },
        ];
        lines = [
          { id: 'l-1', node_id1: 'gw-1', node_id2: 'sw-1', state: 'normal' },
          { id: 'l-2', node_id1: 'gw-1', node_id2: 'sw-2', state: 'warn' },
          { id: 'l-3', node_id1: 'sw-1', node_id2: 'srv-1', state: 'normal' },
          { id: 'l-4', node_id1: 'sw-1', node_id2: 'srv-2', state: 'normal' },
          { id: 'l-5', node_id1: 'sw-2', node_id2: 'edge-1', state: 'error' },
        ];
        if (e.length === 0) {
          eventLogs = [
            { time: Date.now() * 1_000_000 - 300_000_000, type: 'polling', level: 'error', node_id: 'edge-1', node_name: 'Branch Edge Router', event: 'Ping packet loss 100%' },
            { time: Date.now() * 1_000_000 - 120_000_000, type: 'snmp', level: 'warn', node_id: 'sw-2', node_name: 'Dist Switch B', event: 'Interface GigabitEthernet0/2 error rate exceeds 5%' },
            { time: Date.now() * 1_000_000 - 30_000_000, type: 'system', level: 'info', node_name: 'Core Gateway', event: 'Configuration backup verified successfully' },
          ];
        }
      } else {
        nodes = n;
      }
    } finally {
      isRefreshing = false;
    }
  }

  onMount(() => {
    loadData();
    const interval = setInterval(loadData, 6000);
    return () => clearInterval(interval);
  });

  let activeAlertsCount = $derived(
    eventLogs.filter((l) => l.level === 'warn' || l.level === 'high' || l.level === 'error').length
  );
</script>

<div class="flex h-screen bg-slate-950 text-slate-100 overflow-hidden font-sans">
  <!-- Sidebar -->
  <aside class="w-64 bg-slate-900 border-r border-slate-800 flex flex-col shrink-0">
    <!-- Brand -->
    <div class="h-16 flex items-center px-6 border-b border-slate-800 gap-3">
      <div class="p-2 bg-gradient-to-tr from-blue-600 to-indigo-600 rounded-lg text-white shadow-md shadow-blue-500/20">
        <Server class="w-5 h-5" />
      </div>
      <div>
        <h1 class="text-base font-bold tracking-tight text-white flex items-center gap-1.5">
          TWSNMP <span class="text-blue-400 font-extrabold">NEO</span>
        </h1>
        <span class="text-[10px] text-slate-400 font-mono">Next-Gen Monitor</span>
      </div>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 px-3 py-4 space-y-1.5 overflow-y-auto">
      <button
        onclick={() => activeTab = 'dashboard'}
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all {
          activeTab === 'dashboard'
            ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
        }"
      >
        <BarChart3 class="w-4 h-4" />
        <span>Dashboard</span>
      </button>

      <button
        onclick={() => activeTab = 'map'}
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all {
          activeTab === 'map'
            ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
        }"
      >
        <Network class="w-4 h-4" />
        <span>Topology Map</span>
      </button>

      <button
        onclick={() => activeTab = 'nodes'}
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all {
          activeTab === 'nodes'
            ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
        }"
      >
        <Server class="w-4 h-4" />
        <span>Nodes & Pollings</span>
      </button>

      <button
        onclick={() => activeTab = 'alerts'}
        class="w-full flex items-center justify-between px-3 py-2.5 rounded-lg text-sm font-medium transition-all {
          activeTab === 'alerts'
            ? 'bg-blue-600 text-white shadow-md shadow-blue-600/30'
            : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
        }"
      >
        <div class="flex items-center gap-3">
          <AlertTriangle class="w-4 h-4 text-amber-400" />
          <span>Alerts & Events</span>
        </div>
        {#if activeAlertsCount > 0}
          <span class="px-2 py-0.5 text-[11px] font-bold rounded-full {
            activeTab === 'alerts' ? 'bg-white/20 text-white' : 'bg-amber-500/20 text-amber-400'
          }">
            {activeAlertsCount}
          </span>
        {/if}
      </button>

      <button
        onclick={() => activeTab = 'ai'}
        class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all {
          activeTab === 'ai'
            ? 'bg-purple-600 text-white shadow-md shadow-purple-600/30'
            : 'text-purple-300 hover:text-white hover:bg-purple-950/40'
        }"
      >
        <Sparkles class="w-4 h-4 text-purple-400" />
        <span>AI Assistant</span>
      </button>
    </nav>

    <!-- System Status Footer -->
    <div class="p-4 border-t border-slate-800 text-xs text-slate-400 space-y-2">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
          <span class="font-medium text-slate-300">{health?.status || 'Online'}</span>
        </div>
        <span class="font-mono text-[11px]">{health?.version || 'v0.1.0'}</span>
      </div>
      <div class="text-[11px] text-slate-500 truncate">
        Private PKI & Parquet Ready
      </div>
    </div>
  </aside>

  <!-- Main Content Area -->
  <div class="flex-1 flex flex-col min-w-0 overflow-hidden">
    <!-- Top Header -->
    <header class="h-16 bg-slate-900/60 backdrop-blur border-b border-slate-800 px-6 flex items-center justify-between shrink-0">
      <div class="flex items-center gap-3">
        <h2 class="text-lg font-bold text-white capitalize">
          {activeTab === 'ai' ? 'AI Assistant & Root-Cause Diagnosis' : activeTab}
        </h2>
        <span class="text-xs bg-slate-800 text-slate-400 px-2 py-0.5 rounded border border-slate-700">
          Autonomous Network Intelligence
        </span>
      </div>

      <div class="flex items-center gap-3">
        <button
          onclick={loadData}
          disabled={isRefreshing}
          class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700 text-xs font-medium transition-colors"
        >
          <RefreshCw class="w-3.5 h-3.5 {isRefreshing ? 'animate-spin' : ''}" />
          Refresh
        </button>
      </div>
    </header>

    <!-- View Body -->
    <main class="flex-1 p-6 overflow-y-auto space-y-6">
      {#if activeTab === 'dashboard'}
        <!-- KPI Cards -->
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <div class="bg-slate-800/80 border border-slate-700/70 p-4 rounded-xl flex items-center space-x-3 shadow-sm">
            <div class="p-3 bg-blue-500/10 text-blue-400 rounded-lg">
              <Server class="w-6 h-6" />
            </div>
            <div>
              <div class="text-xs text-slate-400">Total Nodes</div>
              <div class="text-xl font-bold text-white">{nodes.length}</div>
            </div>
          </div>

          <div class="bg-slate-800/80 border border-slate-700/70 p-4 rounded-xl flex items-center space-x-3 shadow-sm">
            <div class="p-3 bg-emerald-500/10 text-emerald-400 rounded-lg">
              <Activity class="w-6 h-6" />
            </div>
            <div>
              <div class="text-xs text-slate-400">Active Pollings</div>
              <div class="text-xl font-bold text-white">{pollings.length}</div>
            </div>
          </div>

          <div class="bg-slate-800/80 border border-slate-700/70 p-4 rounded-xl flex items-center space-x-3 shadow-sm">
            <div class="p-3 bg-amber-500/10 text-amber-400 rounded-lg">
              <AlertTriangle class="w-6 h-6" />
            </div>
            <div>
              <div class="text-xs text-slate-400">Active Alerts</div>
              <div class="text-xl font-bold text-amber-400">{activeAlertsCount}</div>
            </div>
          </div>

          <div class="bg-slate-800/80 border border-slate-700/70 p-4 rounded-xl flex items-center space-x-3 shadow-sm">
            <div class="p-3 bg-purple-500/10 text-purple-400 rounded-lg">
              <Radio class="w-6 h-6" />
            </div>
            <div>
              <div class="text-xs text-slate-400">Protocol Ingestion</div>
              <div class="text-xl font-bold text-purple-400">Syslog / TRAP</div>
            </div>
          </div>
        </div>

        <!-- Metric Charts -->
        <MetricCharts {nodes} {eventLogs} />

        <!-- Topology Map Preview -->
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <h3 class="text-sm font-semibold text-slate-200">Network Topology</h3>
            <button
              onclick={() => activeTab = 'map'}
              class="text-xs text-blue-400 hover:text-blue-300 font-medium"
            >
              Open Fullscreen Map &rarr;
            </button>
          </div>
          <div class="h-80">
            <TopologyMap {nodes} {lines} onSelectNode={(n: NodeEnt | null) => selectedNode = n} />
          </div>
        </div>

        <!-- Alerts Grid -->
        <AlertGrid {eventLogs} onRefresh={loadData} />

      {:else if activeTab === 'map'}
        <div class="flex flex-col h-full gap-4">
          <div class="flex-1 relative min-h-[500px]">
            <TopologyMap {nodes} {lines} onSelectNode={(n: NodeEnt | null) => selectedNode = n} />

            {#if selectedNode}
              <div class="absolute top-4 right-4 bg-slate-800/90 backdrop-blur border border-slate-700 p-4 rounded-xl shadow-xl w-72 space-y-2 text-sm">
                <div class="flex items-center justify-between border-b border-slate-700 pb-2">
                  <span class="font-bold text-white truncate">{selectedNode.name}</span>
                  <button onclick={() => selectedNode = null} class="text-slate-400 hover:text-white">&times;</button>
                </div>
                <div class="text-xs text-slate-400">IP: <span class="font-mono text-slate-200">{selectedNode.ip}</span></div>
                <div class="text-xs text-slate-400">Status: <span class="font-semibold capitalize text-emerald-400">{selectedNode.state}</span></div>
                {#if selectedNode.descr}
                  <div class="text-xs text-slate-400">Description: <span class="text-slate-300">{selectedNode.descr}</span></div>
                {/if}
              </div>
            {/if}
          </div>
        </div>

      {:else if activeTab === 'nodes'}
        <NodeTable {nodes} {pollings} onRefresh={loadData} />

      {:else if activeTab === 'alerts'}
        <AlertGrid {eventLogs} onRefresh={loadData} />

      {:else if activeTab === 'ai'}
        <div class="h-[calc(100vh-140px)]">
          <AIAssistant />
        </div>
      {/if}
    </main>
  </div>
</div>
