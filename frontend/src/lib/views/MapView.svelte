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
    Maximize2,
    RefreshCw,
    Plus,
    Server,
    Palette,
    Cpu,
    Activity,
    Edit3,
    Trash2,
    Info,
    Search,
    Radio,
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

  // Quick filter / search
  let selectedNodeId = $state("");

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
    const container = document.getElementById("p5-map-canvas");
    if (container) {
      await initMAP(container, (event: any) => {
        if (event.Cmd === "contextMenu") {
          contextTargetNode = event.Node || "";
          contextTargetNet = event.Network || "";
          contextX = event.x;
          contextY = event.y;
          showContextMenu = true;
        } else if (event.Cmd === "nodeDoubleClicked") {
          const n = nodes.find((x) => x.id === event.Param);
          if (n) {
            detailNode = n;
            showNodeDetailModal = true;
          }
        } else if (event.Cmd === "networkDoubleClicked") {
          const net = networks.find((x) => x.id === event.Param);
          if (net) {
            selectedNetwork = net;
            showNetworkDialog = true;
          }
        }
      });
      await reloadAllData();
    }

    refreshTimer = setInterval(reloadAllData, 5000);
  });

  onDestroy(() => {
    if (refreshTimer) clearInterval(refreshTimer);
    resetMap();
  });

  // Actions
  const handleOpenAddNode = () => {
    selectedNode = null;
    showNodeDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddNetwork = () => {
    selectedNetwork = null;
    showNetworkDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddDrawItem = () => {
    selectedDrawItem = null;
    showDrawItemDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddLine = () => {
    selectedLine = null;
    showLineDialog = true;
    showContextMenu = false;
  };

  const handleEditTargetNode = () => {
    const n = nodes.find((x) => x.id === contextTargetNode);
    if (n) {
      selectedNode = n;
      showNodeDialog = true;
    }
    showContextMenu = false;
  };

  const handleShowNodeDetail = () => {
    const n = nodes.find((x) => x.id === contextTargetNode);
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
    const net = networks.find((x) => x.id === contextTargetNet);
    if (net) {
      selectedNetwork = net;
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
</script>

<div class="relative h-[calc(100vh-4rem)] w-full overflow-hidden bg-background" onclick={() => (showContextMenu = false)}>
  <!-- Map Floating Toolbar -->
  <div class="absolute top-4 left-4 z-20 flex items-center gap-2 rounded-xl border border-border/80 bg-card/90 p-1.5 shadow-lg backdrop-blur-md">
    <!-- Quick Add Menu -->
    <button
      onclick={handleOpenAddNode}
      class="flex items-center gap-1.5 rounded-lg bg-primary/10 px-3 py-1.5 text-xs font-semibold text-primary hover:bg-primary/20 transition-all"
    >
      <Plus class="h-3.5 w-3.5" />
      ノード追加
    </button>
    <button
      onclick={handleOpenAddNetwork}
      class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-muted transition-all"
    >
      <Server class="h-3.5 w-3.5" />
      SW-HUB追加
    </button>
    <button
      onclick={handleOpenAddLine}
      class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-muted transition-all"
    >
      <Activity class="h-3.5 w-3.5" />
      ライン結線
    </button>
    <button
      onclick={handleOpenAddDrawItem}
      class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-muted transition-all"
    >
      <Palette class="h-3.5 w-3.5" />
      描画アイテム
    </button>

    <div class="mx-1 h-4 w-[1px] bg-border"></div>

    <!-- Zoom & Refresh Controls -->
    <button onclick={() => zoom(true)} class="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">
      <ZoomIn class="h-4 w-4" />
    </button>
    <button onclick={() => zoom(false)} class="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">
      <ZoomOut class="h-4 w-4" />
    </button>
    <button onclick={reloadAllData} class="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground">
      <RefreshCw class="h-4 w-4" />
    </button>
  </div>

  <!-- p5.js Map Canvas Container -->
  <div id="p5-map-canvas" class="h-full w-full"></div>

  <!-- Bottom Event Log Bar (Ticker) -->
  <div class="absolute bottom-3 left-4 right-4 z-20 flex items-center justify-between rounded-xl border border-border/80 bg-card/90 px-4 py-2 text-xs shadow-lg backdrop-blur-md">
    <div class="flex items-center gap-3 overflow-hidden">
      <div class="flex items-center gap-1.5 font-semibold text-primary shrink-0">
        <Radio class="h-3.5 w-3.5 animate-pulse text-emerald-500" />
        <span>最新イベント:</span>
      </div>
      {#if eventLogs.length > 0}
        <div class="flex items-center gap-2 truncate text-muted-foreground">
          <span class="font-mono text-[10px]">{new Date(eventLogs[0].time * 1000).toLocaleTimeString()}</span>
          <span class="rounded px-1.5 py-0.2 text-[10px] uppercase font-bold" style="background-color: {getStateColor(eventLogs[0].level)}22; color: {getStateColor(eventLogs[0].level)}">
            {eventLogs[0].type}
          </span>
          <span class="truncate text-foreground font-medium">{eventLogs[0].event}</span>
        </div>
      {:else}
        <span class="text-muted-foreground">イベントはありません</span>
      {/if}
    </div>
    <span class="text-[10px] text-muted-foreground shrink-0">{nodes.length} ノード / {networks.length} SW-HUB / {lines.length} ライン</span>
  </div>

  <!-- Right Click Context Menu -->
  {#if showContextMenu}
    <div
      class="fixed z-50 min-w-[160px] rounded-xl border border-border bg-card p-1.5 text-xs shadow-2xl backdrop-blur-md"
      style="left: {contextX}px; top: {contextY}px;"
      onclick={(e) => e.stopPropagation()}
    >
      {#if contextTargetNode}
        <button onclick={handleShowNodeDetail} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted font-medium">
          <Info class="h-3.5 w-3.5 text-primary" />
          3D パネル / 詳細
        </button>
        <button onclick={handleEditTargetNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted">
          <Edit3 class="h-3.5 w-3.5" />
          ノードの編集
        </button>
        <div class="my-1 border-t border-border"></div>
        <button onclick={handleDeleteTargetNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-destructive hover:bg-destructive/10">
          <Trash2 class="h-3.5 w-3.5" />
          ノードの削除
        </button>
      {:else if contextTargetNet}
        <button onclick={handleEditTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted font-medium">
          <Server class="h-3.5 w-3.5 text-primary" />
          SW-HUB の編集
        </button>
        <div class="my-1 border-t border-border"></div>
        <button onclick={handleDeleteTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-destructive hover:bg-destructive/10">
          <Trash2 class="h-3.5 w-3.5" />
          SW-HUB の削除
        </button>
      {:else}
        <button onclick={handleOpenAddNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted">
          <Plus class="h-3.5 w-3.5 text-primary" />
          ノードの追加
        </button>
        <button onclick={handleOpenAddNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted">
          <Server class="h-3.5 w-3.5" />
          SW-HUB の追加
        </button>
        <button onclick={handleOpenAddDrawItem} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted">
          <Palette class="h-3.5 w-3.5" />
          描画アイテムの追加
        </button>
        <button onclick={handleOpenAddLine} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-foreground hover:bg-muted">
          <Activity class="h-3.5 w-3.5" />
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
