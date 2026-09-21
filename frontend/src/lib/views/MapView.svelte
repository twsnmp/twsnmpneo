<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    initMAP,
    updateMAP,
    resetMap,
    zoom,
    horizontal,
    vertical,
    circle,
    getNodeBounds,
    getMapSize,
  } from "../map/map";
  import NodeDialog from "../components/NodeDialog.svelte";
  import NetworkDialog from "../components/NetworkDialog.svelte";
  import DrawItemDialog from "../components/DrawItemDialog.svelte";
  import LineDialog from "../components/LineDialog.svelte";
  import NetworkLinesDialog from "../components/NetworkLinesDialog.svelte";
  import FindNeighborDialog from "../components/FindNeighborDialog.svelte";
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
    AlignCenterHorizontal,
    AlignCenterVertical,
    CircleDot,
    Compass,
    Network,
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

  let showNetworkLinesDialog = $state(false);
  let targetHubNetwork = $state<NetworkEnt | null>(null);

  let showFindNeighborDialog = $state(false);
  let findNeighborTargetId = $state("");

  let showNodeDetailModal = $state(false);
  let detailNode = $state<NodeEnt | null>(null);

  // Context menu state
  let showContextMenu = $state(false);
  let contextX = $state(0);
  let contextY = $state(0);
  let contextMapX = $state(0);
  let contextMapY = $state(0);
  let contextTargetNode = $state("");
  let contextTargetNet = $state("");
  let contextTargetItem = $state("");

  // Multi-node format menu state
  let showFormatMenu = $state(false);
  let formatNodesList = $state<string[]>([]);
  let formatPosX = $state(0);
  let formatPosY = $state(0);

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
        if (ev?.type === "contextmenu" || ev?.Cmd === "contextMenu") {
          contextX = Math.min(ev.x, window.innerWidth - 200);
          contextY = Math.min(ev.y, window.innerHeight - 260);
          contextMapX = ev.mapX ?? 300;
          contextMapY = ev.mapY ?? 200;
          contextTargetNode = ev.nodeId || ev.Node || "";
          contextTargetNet = ev.networkId || ev.Network || "";
          contextTargetItem = ev.itemId || ev.DrawItem || "";
          showFormatMenu = false;
          showContextMenu = true;
        } else if (ev?.type === "formatNodes" || ev?.Cmd === "formatNodes") {
          formatNodesList = ev.Nodes || [];
          formatPosX = Math.min(ev.x, window.innerWidth - 200);
          formatPosY = Math.min(ev.y, window.innerHeight - 220);
          showContextMenu = false;
          showFormatMenu = true;
        } else if (ev?.type === "dblclick") {
          if (ev.nodeId) {
            const n = nodes.find((item) => (item.id || item.ID) === ev.nodeId);
            if (n) {
              detailNode = n;
              showNodeDetailModal = true;
            }
          } else if (ev.networkId) {
            const net = networks.find((item) => (item.id || item.ID) === ev.networkId);
            if (net) {
              selectedNetwork = { ...net };
              showNetworkDialog = true;
            }
          } else if (ev.itemId) {
            const it = drawItems.find((item) => (item.id || item.ID) === ev.itemId);
            if (it) {
              selectedDrawItem = { ...it };
              showDrawItemDialog = true;
            }
          }
        } else if (ev?.Cmd === "deleteNodes" && ev.Param) {
          (async () => {
            for (const id of ev.Param) {
              await deleteNode(id).catch(console.error);
            }
            await reloadAllData();
          })();
        } else if (ev?.Cmd === "deleteDrawItems" && ev.Param) {
          (async () => {
            for (const id of ev.Param) {
              await deleteDrawItem(id).catch(console.error);
            }
            await reloadAllData();
          })();
        } else if (ev?.Cmd === "deleteNetwork" && ev.Param) {
          (async () => {
            await deleteNetwork(ev.Param).catch(console.error);
            await reloadAllData();
          })();
        } else if (ev?.type === "editLine" || ev?.Cmd === "editLine") {
          if (ev.Param && ev.Param.length === 2) {
            const id1 = ev.Param[0];
            const id2 = ev.Param[1];
            const existing = lines.find((l) => {
              const n1 = l.node_id1 || (l as any).NodeID1;
              const n2 = l.node_id2 || (l as any).NodeID2;
              return (n1 === id1 && n2 === id2) || (n1 === id2 && n2 === id1);
            });
            if (existing) {
              selectedLine = { ...existing };
            } else {
              selectedLine = {
                id: "",
                node_id1: id1,
                node_id2: id2,
                state: "normal",
                width: 2,
              };
            }
            showLineDialog = true;
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
    const { halfW, topH, bottomH } = getNodeBounds();
    const mapSize = getMapSize();
    const targetX = contextMapX > 0 ? contextMapX : 320;
    const targetY = contextMapY > 0 ? contextMapY : 200;
    const clampedX = Math.max(halfW, Math.min(mapSize.width - halfW, targetX));
    const clampedY = Math.max(topH, Math.min(mapSize.height - bottomH, targetY));

    selectedNode = {
      id: "",
      name: "新規ノード",
      ip: "192.168.1.10",
      mac: "",
      descr: "",
      icon: "desktop",
      state: "normal",
      x: clampedX,
      y: clampedY,
    };
    showNodeDialog = true;
    showContextMenu = false;
  };

  const handleOpenAddNetwork = () => {
    const mapSize = getMapSize();
    const targetX = contextMapX > 0 ? contextMapX : 240;
    const targetY = contextMapY > 0 ? contextMapY : 120;
    const clampedX = Math.max(8, Math.min(mapSize.width - 420 - 8, targetX));
    const clampedY = Math.max(8, Math.min(mapSize.height - 90 - 8, targetY));

    selectedNetwork = {
      id: "",
      name: "ネットワーク",
      ip: "192.168.1.254",
      x: clampedX,
      y: clampedY,
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
    const mapSize = getMapSize();
    const targetX = contextMapX > 0 ? contextMapX : 200;
    const targetY = contextMapY > 0 ? contextMapY : 200;
    const clampedX = Math.max(8, Math.min(mapSize.width - 120 - 8, targetX));
    const clampedY = Math.max(8, Math.min(mapSize.height - 40 - 8, targetY));

    selectedDrawItem = {
      id: "",
      type: 0,
      x: clampedX,
      y: clampedY,
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

  const handleFindNeighborNode = () => {
    findNeighborTargetId = "NODE:" + contextTargetNode;
    showFindNeighborDialog = true;
    showContextMenu = false;
  };

  const handleFindNeighborNet = () => {
    findNeighborTargetId = "NET:" + contextTargetNet;
    showFindNeighborDialog = true;
    showContextMenu = false;
  };

  const handleOpenNetworkLines = () => {
    const net = networks.find((item) => (item.id || item.ID) === contextTargetNet);
    if (net) {
      targetHubNetwork = net;
      showNetworkLinesDialog = true;
    }
    showContextMenu = false;
  };

  const handleEditLineFromHub = (l: LineEnt) => {
    selectedLine = { ...l };
    showLineDialog = true;
  };

  const handleEditTargetDrawItem = () => {
    const it = drawItems.find((item) => (item.id || item.ID) === contextTargetItem);
    if (it) {
      selectedDrawItem = { ...it };
      showDrawItemDialog = true;
    }
    showContextMenu = false;
  };

  const handleDeleteTargetDrawItem = async () => {
    if (contextTargetItem) {
      await deleteDrawItem(contextTargetItem);
      await reloadAllData();
    }
    showContextMenu = false;
  };

  const handleFormat = async (type: 'horizontal' | 'vertical' | 'circle') => {
    showFormatMenu = false;
    if (type === 'horizontal') await horizontal(formatNodesList);
    else if (type === 'vertical') await vertical(formatNodesList);
    else if (type === 'circle') await circle(formatNodesList);
    await reloadAllData();
  };

  const handleDeleteSelectedNodes = async () => {
    showFormatMenu = false;
    for (const id of formatNodesList) {
      await deleteNode(id).catch(console.error);
    }
    formatNodesList = [];
    await reloadAllData();
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

<svelte:window
  onclick={(e) => {
    if (e.button === 0) {
      showContextMenu = false;
      showFormatMenu = false;
    }
  }}
  onkeydown={(e) => {
    if (e.key === "Escape") {
      showContextMenu = false;
      showFormatMenu = false;
    }
  }}
/>

<!-- Two-tier layout matching twsnmpfk Image 1 + twnoaa styling -->
<div class="flex h-[calc(100vh-4.25rem)] w-full flex-col overflow-hidden bg-[#0b1329]">
  <!-- Upper Section: Topology Map Canvas (approx 62%) -->
  <div class="relative h-[62%] w-full overflow-hidden border-b border-slate-800">
    <!-- Top-right Pinned Reload Button -->
    <div class="absolute top-3 right-4 z-20">
      <button
        onclick={reloadAllData}
        title="再読み込み"
        class="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-700 bg-slate-900/90 text-slate-200 shadow-xl hover:bg-slate-800 hover:border-cyan-500/50 hover:text-cyan-400 transition-all backdrop-blur-md active:scale-95"
      >
        <RefreshCw class="h-4 w-4" />
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

    <!-- p5.js Canvas Container with native scrolling matching twsnmpfk -->
    <div id="p5-map-canvas" class="h-full w-full overflow-auto"></div>
  </div>

  <!-- Lower Section: Realtime Event Log Table (approx 38%) matching Image 1 + twnoaa style -->
  <div class="flex h-[38%] w-full flex-col min-h-0 bg-[#0b1329] p-3 space-y-2 overflow-hidden">
    <div class="flex items-center justify-between flex-shrink-0 px-1">
      <div class="flex items-center gap-2">
        <Calendar class="h-4 w-4 text-cyan-400" />
        <span class="text-xs font-bold text-slate-200">リアルタイム イベントログ (Event Log)</span>
        <span class="text-[11px] font-mono text-slate-400">({eventLogs.length} 件)</span>
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
      class="fixed z-50 min-w-[180px] rounded-xl border border-slate-700 bg-slate-900/95 p-1.5 text-xs shadow-2xl backdrop-blur-md"
      style="left: {contextX}px; top: {contextY}px;"
      onclick={(e) => e.stopPropagation()}
      oncontextmenu={(e) => { e.preventDefault(); e.stopPropagation(); }}
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
        <button onclick={handleFindNeighborNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-200 hover:bg-slate-800">
          <Compass class="h-3.5 w-3.5 text-indigo-400" />
          接続先を探す
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={handleDeleteTargetNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
          <Trash2 class="h-3.5 w-3.5" />
          ノードの削除
        </button>
      {:else if contextTargetNet}
        <button onclick={handleEditTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Network class="h-3.5 w-3.5 text-cyan-400" />
          ネットワークの編集
        </button>
        <button onclick={handleOpenNetworkLines} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-200 hover:bg-slate-800">
          <Activity class="h-3.5 w-3.5 text-emerald-400" />
          ライン編集
        </button>
        <button onclick={handleFindNeighborNet} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-200 hover:bg-slate-800">
          <Compass class="h-3.5 w-3.5 text-indigo-400" />
          接続先を探す
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={handleDeleteTargetNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
          <Trash2 class="h-3.5 w-3.5" />
          ネットワークの削除
        </button>
      {:else if contextTargetItem}
        <button onclick={handleEditTargetDrawItem} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Palette class="h-3.5 w-3.5 text-purple-400" />
          描画アイテムの編集
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={handleDeleteTargetDrawItem} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
          <Trash2 class="h-3.5 w-3.5" />
          描画アイテムの削除
        </button>
      {:else}
        <button onclick={handleOpenAddNode} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Plus class="h-3.5 w-3.5 text-cyan-400" />
          ノードの追加
        </button>
        <button onclick={handleOpenAddNetwork} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Network class="h-3.5 w-3.5 text-emerald-400" />
          ネットワークの追加
        </button>
        <button onclick={handleOpenAddDrawItem} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Palette class="h-3.5 w-3.5 text-purple-400" />
          描画アイテムの追加
        </button>
        <button onclick={handleOpenAddLine} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800 font-medium">
          <Activity class="h-3.5 w-3.5 text-cyan-400" />
          ライン結線
        </button>
        <div class="my-1 border-t border-slate-800"></div>
        <button onclick={reloadAllData} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-300 hover:bg-slate-800">
          <RefreshCw class="h-3.5 w-3.5 text-slate-400" />
          再読み込み
        </button>
      {/if}
    </div>
  {/if}

  <!-- Multi-Node Alignment Context Menu -->
  {#if showFormatMenu}
    <div
      role="menu"
      tabindex="-1"
      class="fixed z-50 min-w-[180px] rounded-xl border border-slate-700 bg-slate-900/95 p-1.5 text-xs shadow-2xl backdrop-blur-md"
      style="left: {formatPosX}px; top: {formatPosY}px;"
      onclick={(e) => e.stopPropagation()}
      oncontextmenu={(e) => { e.preventDefault(); e.stopPropagation(); }}
      onkeydown={(e) => e.key === 'Escape' && (showFormatMenu = false)}
    >
      <div class="px-3 py-1.5 text-[11px] font-semibold text-slate-400">選択ノード ({formatNodesList.length}個)</div>
      <button onclick={() => handleFormat('horizontal')} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
        <AlignCenterHorizontal class="h-3.5 w-3.5 text-cyan-400" />
        水平に整列
      </button>
      <button onclick={() => handleFormat('vertical')} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
        <AlignCenterVertical class="h-3.5 w-3.5 text-cyan-400" />
        垂直に整列
      </button>
      <button onclick={() => handleFormat('circle')} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-slate-100 hover:bg-slate-800">
        <CircleDot class="h-3.5 w-3.5 text-cyan-400" />
        円形に配置
      </button>
      <div class="my-1 border-t border-slate-800"></div>
      <button onclick={handleDeleteSelectedNodes} class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-rose-400 hover:bg-rose-500/10">
        <Trash2 class="h-3.5 w-3.5" />
        選択ノードを削除
      </button>
    </div>
  {/if}

  <!-- Modals -->
  <NodeDialog bind:show={showNodeDialog} node={selectedNode} onSave={reloadAllData} />
  <NetworkDialog bind:show={showNetworkDialog} network={selectedNetwork} onSave={reloadAllData} />
  <DrawItemDialog bind:show={showDrawItemDialog} item={selectedDrawItem} {nodes} {pollings} onSave={reloadAllData} />
  <LineDialog bind:show={showLineDialog} line={selectedLine} {nodes} {networks} {pollings} onSave={reloadAllData} onDelete={() => reloadAllData()} />
  <NetworkLinesDialog
    bind:show={showNetworkLinesDialog}
    network={targetHubNetwork}
    {lines}
    {nodes}
    {networks}
    {pollings}
    onEditLine={handleEditLineFromHub}
    onDeleteLine={() => reloadAllData()}
  />
  <FindNeighborDialog
    bind:show={showFindNeighborDialog}
    targetId={findNeighborTargetId}
    {nodes}
    {networks}
    onConnect={reloadAllData}
  />
  <NodeDetailModal bind:show={showNodeDetailModal} node={detailNode} {pollings} logs={eventLogs} />
</div>
