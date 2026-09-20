<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { initMAP, updateMAP, resetMap, zoom } from "../map/map";
  import NodeDialog from "../components/NodeDialog.svelte";
  import NetworkDialog from "../components/NetworkDialog.svelte";
  import DrawItemDialog from "../components/DrawItemDialog.svelte";
  import LineDialog from "../components/LineDialog.svelte";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import {
    fetchNodes,
    fetchLines,
    fetchNetworks,
    fetchDrawItems,
    fetchEventLogs,
    fetchPollings,
    deleteNode,
    deleteNetwork,
    deleteDrawItem,
    deleteLine,
    type NodeEnt,
    type LineEnt,
    type NetworkEnt,
    type DrawItemEnt,
    type EventLogEnt,
    type PollingEnt,
  } from "../api";
  import { getStateColor } from "../common";
  import {
    ZoomIn,
    ZoomOut,
    Save,
    RefreshCw,
    Plus,
    Server,
    Palette,
    Activity,
    Edit3,
    Trash2,
    Info,
    Calendar,
    CheckCircle2,
    AlertTriangle,
    XCircle,
    HelpCircle,
    RotateCcw,
  } from "@lucide/svelte";

  let nodes = $state<NodeEnt[]>([]);
  let lines = $state<LineEnt[]>([]);
  let networks = $state<NetworkEnt[]>([]);
  let drawItems = $state<DrawItemEnt[]>([]);
  let eventLogs = $state<EventLogEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);

  // Dialog controls
  let showNodeDialog = $state(false);
  let selectedNode = $state<NodeEnt | null>(null);

  let showNetworkDialog = $state(false);
  let selectedNetwork = $state<any>(null);

  let showDrawItemDialog = $state(false);
  let selectedDrawItem = $state<DrawItemEnt | null>(null);

  let showLineDialog = $state(false);
  let selectedLine = $state<LineEnt | null>(null);

  let showNodeDetailModal = $state(false);
  let detailNode = $state<NodeEnt | null>(null);

  // Context menu state
  let showContextMenu = $state(false);
  let contextX = $state(0);
  let contextY = $state(0);
  let contextTargetNode = $state("");
  let contextTargetNet = $state("");

  let refreshTimer: any = null;

  const reloadAllData = async () => {
    try {
      const [n, l, net, di, el, pl] = await Promise.all([
        fetchNodes(),
        fetchLines(),
        fetchNetworks(),
        fetchDrawItems(),
        fetchEventLogs(),
        fetchPollings(),
      ]);
      nodes = n;
      lines = l;
      networks = net;
      drawItems = di;
      eventLogs = el;
      pollings = pl;
      await updateMAP();
    } catch (e) {
      console.error("Failed to load map data:", e);
    }
  };

  onMount(async () => {
    const canvasDiv = document.getElementById("p5-map-canvas");
    if (canvasDiv) {
      await initMAP(canvasDiv, (ev: any) => {
        if (ev?.type === "contextmenu") {
          contextX = ev.x;
          contextY = ev.y;
          contextTargetNode = ev.nodeId || "";
          contextTargetNet = ev.networkId || "";
          showContextMenu = true;
        } else if (ev?.type === "dblclick" && ev.nodeId) {
          const n = nodes.find((item) => (item.id || item.ID) === ev.nodeId);
          if (n) {
            detailNode = n;
            showNodeDetailModal = true;
          }
        }
      });
    }

    await reloadAllData();

    // Periodic refresh every 5 seconds
    refreshTimer = setInterval(async () => {
      try {
        const [el, pl] = await Promise.all([fetchEventLogs(), fetchPollings()]);
        eventLogs = el;
        pollings = pl;
      } catch (e) {
        console.error("Polling error:", e);
      }
    }, 5000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    resetMap();
  });

  const handleOpenAddNode = () => {
    selectedNode = {
      id: "",
      name: "新規ノード",
      ip: "192.168.1.10",
      mac: "",
      descr: "",
      icon: "desktop",
      state: "normal",
      x: contextX > 0 ? contextX : 320,
      y: contextY > 0 ? contextY : 200,
    };
    showNodeDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddNetwork = () => {
    selectedNetwork = {
      id: "",
      name: "SW-HUB",
      ip: "192.168.1.254",
      x: contextX > 0 ? contextX : 240,
      y: contextY > 0 ? contextY : 120,
      w: 420,
      h: 90,
      h_ports: 8,
      ports: Array.from({ length: 8 }).map((_, i) => ({
        id: `p${i + 1}`,
        name: `Port ${i + 1}`,
        x: i % 8,
        y: Math.floor(i / 8),
        state: "none",
      })),
    };
    showNetworkDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddLine = () => {
    selectedLine = {
      id: "",
      node_id1: nodes[0]?.id || "",
      node_id2: nodes[1]?.id || (networks[0] ? `NET:${networks[0].id}` : ""),
      state: "normal",
      width: 2,
    };
    showLineDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddDrawItem = () => {
    selectedDrawItem = {
      id: "",
      type: 0,
      x: contextX > 0 ? contextX : 200,
      y: contextY > 0 ? contextY : 200,
      w: 120,
      h: 40,
      text: "新規アイテム",
      color: "#06b6d4",
    };
    showDrawItemDialog = true;
    showContextMenu = false;
  };

  const handleEditTargetNode = () => {
    const n = nodes.find((item) => (item.id || item.ID) === contextTargetNode);
    if (n) {
      selectedNode = { ...n };
      showNodeDialog = true;
    }
    showContextMenu = false;
  };

  const handleShowNodeDetail = () => {
    const n = nodes.find((item) => (item.id || item.ID) === contextTargetNode);
    if (n) {
      detailNode = n;
      showNodeDetailModal = true;
    }
    showContextMenu = false;
  };

  const handleDeleteTargetNode = async () => {
    if (contextTargetNode) {
      await deleteNode(contextTargetNode);
      await reloadAllData();
    }
    showContextMenu = false;
  };

  const handleEditTargetNetwork = () => {
    const net = networks.find((item) => (item.id || item.ID) === contextTargetNet);
    if (net) {
      selectedNetwork = { ...net };
      showNetworkDialog = true;
    }
    showContextMenu = false;
  };

  const handleDeleteTargetNetwork = async () => {
    if (contextTargetNet) {
      await deleteNetwork(contextTargetNet);
      await reloadAllData();
    }
    showContextMenu = false;
  };

  const formatLogTime = (ts: number): string => {
    if (!ts) return "-";
    const d = new Date(ts > 1e12 ? ts / 1e6 : ts * 1000);
    return `${d.getFullYear()}/${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
  };

  const getLevelBadgeClass = (lvl: string) => {
    switch (lvl?.toLowerCase()) {
      case "normal":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "repair":
        return "bg-cyan-500/10 text-cyan-300 border-cyan-500/30";
      case "warn":
      case "low":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "high":
      case "error":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      case "info":
        return "bg-sky-500/10 text-sky-400 border-sky-500/30";
      default:
        return "bg-slate-800 text-slate-400 border-slate-700";
    }
  };
</script>

<svelte:window onclick={() => (showContextMenu = false)} />

<!-- Two-tier layout matching twsnmpfk Image 1 + twnoaa styling -->
<div class="flex h-[calc(100vh-4.25rem)] w-full flex-col overflow-hidden bg-[#0b1329]">
  <!-- Upper Section: Topology Map Canvas (approx 62%) -->
  <div class="relative h-[62%] w-full overflow-hidden border-b border-slate-800">
    <!-- Top-left Quick Action Toolbar -->
    <div class="absolute top-3 left-4 z-20 flex items-center gap-2 rounded-xl border border-slate-800 bg-slate-950/90 p-1.5 shadow-xl backdrop-blur-md">
      <button
        onclick={handleOpenAddNode}
        class="flex items-center gap-1.5 rounded-lg bg-cyan-600 px-3 py-1.5 text-xs font-semibold text-white shadow-md shadow-cyan-600/30 hover:bg-cyan-500 transition-all"
      >
        <Plus class="h-3.5 w-3.5" />
        ノード追加
      </button>
      <button
        onclick={handleOpenAddNetwork}
        class="flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-800/80 px-3 py-1.5 text-xs font-semibold text-slate-200 hover:bg-slate-700 transition-all"
      >
        <Server class="h-3.5 w-3.5 text-cyan-400" />
        SW-HUB追加
      </button>
      <button
        onclick={handleOpenAddLine}
        class="flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-800/80 px-3 py-1.5 text-xs font-semibold text-slate-200 hover:bg-slate-700 transition-all"
      >
        <Activity class="h-3.5 w-3.5 text-emerald-400" />
        ライン結線
      </button>
      <button
        onclick={handleOpenAddDrawItem}
        class="flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-800/80 px-3 py-1.5 text-xs font-semibold text-slate-200 hover:bg-slate-700 transition-all"
      >
        <Palette class="h-3.5 w-3.5 text-purple-400" />
        描画アイテム
      </button>

      <div class="mx-1 h-4 w-[1px] bg-slate-800"></div>

      <button
        onclick={reloadAllData}
        title="再読み込み"
        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
      >
        <RefreshCw class="h-4 w-4 text-cyan-400" />
      </button>
    </div>

    <!-- Bottom-right Floating Map Controls (Save, Zoom In, Zoom Out) matching twsnmpfk -->
    <div class="absolute bottom-4 right-4 z-20 flex flex-col items-end gap-2">
      <button
        onclick={() => alert("マップ配置を保存しました")}
        title="マップ保存"
        class="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-700 bg-slate-900/90 text-slate-200 shadow-xl hover:bg-slate-800 hover:border-slate-600 transition-all backdrop-blur-md"
      >
        <Save class="h-4 w-4 text-cyan-400" />
      </button>
      <div class="flex items-center gap-1.5">
        <button
          onclick={() => zoom(true)}
          title="拡大"
          class="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-700 bg-slate-900/90 text-slate-200 shadow-xl hover:bg-slate-800 hover:border-slate-600 transition-all backdrop-blur-md"
        >
          <ZoomIn class="h-4 w-4" />
        </button>
        <button
          onclick={() => zoom(false)}
          title="縮小"
          class="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-700 bg-slate-900/90 text-slate-200 shadow-xl hover:bg-slate-800 hover:border-slate-600 transition-all backdrop-blur-md"
        >
          <ZoomOut class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- p5.js Canvas Container -->
    <div id="p5-map-canvas" class="h-full w-full"></div>
  </div>

  <!-- Lower Section: Realtime Event Log Table (approx 38%) matching Image 1 + twnoaa style -->
  <div class="flex h-[38%] w-full flex-col min-h-0 bg-[#0b1329] p-3 space-y-2 overflow-hidden">
    <div class="flex items-center justify-between flex-shrink-0 px-1">
      <div class="flex items-center gap-2">
        <Calendar class="h-4 w-4 text-cyan-400" />
        <span class="text-xs font-bold text-slate-200">リアルタイム イベントログ (Event Log)</span>
        <span class="text-[11px] font-mono text-slate-400">({eventLogs.length} 件)</span>
      </div>
      <div class="text-[11px] text-slate-400 font-mono">
        <span>ノード: <strong class="text-cyan-400">{nodes.length}</strong></span>
        <span class="mx-1.5">•</span>
        <span>SW-HUB: <strong class="text-emerald-400">{networks.length}</strong></span>
        <span class="mx-1.5">•</span>
        <span>結線: <strong class="text-purple-400">{lines.length}</strong></span>
      </div>
    </div>

    <!-- twnoaa style table container -->
    <div class="flex-1 overflow-y-auto overflow-x-auto rounded-xl border border-slate-800 bg-slate-950/70 shadow-lg min-h-0">
      <table class="w-full text-left text-xs">
        <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
          <tr>
            <th class="py-2 px-3 w-28">Level</th>
            <th class="py-2 px-3 w-44">Time</th>
            <th class="py-2 px-3 w-28">Type</th>
            <th class="py-2 px-3 w-48">Node</th>
            <th class="py-2 px-3">Event</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
          {#each eventLogs as log}
            <tr class="hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-3">
                <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getLevelBadgeClass(log.level || (log as any).Level)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(log.level || (log as any).Level)}"></span>
                  {log.level || (log as any).Level || 'info'}
                </span>
              </td>
              <td class="py-1 px-3 text-slate-400 text-[11px]">
                {formatLogTime(log.time || (log as any).Time)}
              </td>
              <td class="py-1 px-3 text-slate-400 text-[11px] font-sans">
                {log.type || (log as any).Type || 'system'}
              </td>
              <td class="py-1 px-3 font-semibold text-slate-200 font-sans truncate">
                {log.node_name || (log as any).NodeName || '-'}
              </td>
              <td class="py-1 px-3 text-slate-100 font-sans truncate" title={log.event || (log as any).Event}>
                {log.event || (log as any).Event || '-'}
              </td>
            </tr>
          {/each}

          {#if eventLogs.length === 0}
            <tr>
              <td colspan="5" class="py-8 text-center text-slate-500 font-sans">
                記録されたイベントログはありません
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Right Click Context Menu -->
  {#if showContextMenu}
    <div
      role="menu"
      tabindex="-1"
      class="fixed z-50 min-w-[170px] rounded-xl border border-slate-700 bg-slate-900/95 p-1.5 text-xs shadow-2xl backdrop-blur-md"
      style="left: {contextX}px; top: {contextY}px;"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.key === 'Escape' && (showContextMenu = false)}
    >
      {#if contextTargetNode}
        <button onclick={handleShowNodeDetail} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Info class="h-3.5 w-3.5 text-cyan-400" />
          3D パネル / 詳細
        </button>
        <button onclick={handleEditTargetNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-200 hover:bg-slate-800">
          <Edit3 class="h-3.5 w-3.5 text-slate-400" />
          ノードの編集
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={handleDeleteTargetNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
          <Trash2 class="h-3.5 w-3.5" />
          ノードの削除
        </button>
      {:else if contextTargetNet}
        <button onclick={handleEditTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Server class="h-3.5 w-3.5 text-cyan-400" />
          SW-HUB の編集
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={handleDeleteTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
          <Trash2 class="h-3.5 w-3.5" />
          SW-HUB の削除
        </button>
      {:else}
        <button onclick={handleOpenAddNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
          <Plus class="h-3.5 w-3.5 text-cyan-400" />
          ノードの追加
        </button>
        <button onclick={handleOpenAddNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
          <Server class="h-3.5 w-3.5 text-emerald-400" />
          SW-HUB の追加
        </button>
        <button onclick={handleOpenAddDrawItem} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
          <Palette class="h-3.5 w-3.5 text-purple-400" />
          描画アイテムの追加
        </button>
        <button onclick={handleOpenAddLine} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
          <Activity class="h-3.5 w-3.5 text-cyan-400" />
          ライン結線
        </button>
      {/if}
    </div>
  {/if}

  <!-- Modals -->
  <NodeDialog bind:show={showNodeDialog} node={selectedNode} onSave={reloadAllData} />
  <NetworkDialog bind:show={showNetworkDialog} network={selectedNetwork} onSave={reloadAllData} />
  <DrawItemDialog bind:show={showDrawItemDialog} item={selectedDrawItem} {nodes} {pollings} onSave={reloadAllData} />
  <LineDialog bind:show={showLineDialog} line={selectedLine} {nodes} {networks} {pollings} onSave={reloadAllData} onDelete={() => reloadAllData()} />
  <NodeDetailModal bind:show={showNodeDetailModal} node={detailNode} {pollings} logs={eventLogs} />
</div>
