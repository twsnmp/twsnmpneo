<script lang="ts">
  import { onMount } from "svelte";
  import {
    fetchNodes,
    deleteNode,
    fetchPollings,
    deletePolling,
    fetchNetworks,
    deleteNetwork,
    fetchLines,
    deleteLine,
    fetchDrawItems,
    deleteDrawItem,
    type NodeEnt,
    type PollingEnt,
    type NetworkEnt,
    type LineEnt,
    type DrawItemEnt,
  } from "../api";
  import { getStateColor, getStateName, formatTimeStr } from "../common";
  import NodeDialog from "../components/NodeDialog.svelte";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import PollingDialog from "../components/PollingDialog.svelte";
  import NetworkDialog from "../components/NetworkDialog.svelte";
  import LineDialog from "../components/LineDialog.svelte";
  import DrawItemDialog from "../components/DrawItemDialog.svelte";
  import {
    Laptop,
    CheckSquare,
    Network,
    GitCommitHorizontal,
    Boxes,
    Search,
    RefreshCw,
    Plus,
    Trash2,
    Edit3,
    Box,
    AlertTriangle,
    Type,
    Square,
    Gauge,
    BarChart3,
    TrendingUp,
    CreditCard,
    CheckCircle2,
    Activity,
  } from "@lucide/svelte";

  type ListCategory = "nodes" | "pollings" | "networks" | "lines" | "drawitems";

  let activeCategory = $state<ListCategory>("nodes");
  let searchQuery = $state("");
  let statusFilter = $state("all");
  let typeFilter = $state("all");
  let loading = $state(false);

  // Entities
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let networks = $state<NetworkEnt[]>([]);
  let lines = $state<LineEnt[]>([]);
  let drawItems = $state<DrawItemEnt[]>([]);

  // Dialog states
  let showNodeDialog = $state(false);
  let selectedNode = $state<NodeEnt | null>(null);
  let showDetailModal = $state(false);
  let detailNode = $state<NodeEnt | null>(null);

  let showPollingDialog = $state(false);
  let selectedPolling = $state<PollingEnt | null>(null);

  let showNetworkDialog = $state(false);
  let selectedNetwork = $state<NetworkEnt | null>(null);

  let showLineDialog = $state(false);
  let selectedLine = $state<LineEnt | null>(null);

  let showDrawItemDialog = $state(false);
  let selectedDrawItem = $state<DrawItemEnt | null>(null);

  // Categories definition
  const categories = [
    { id: "nodes" as const, name: "ノード (Node)", icon: Laptop },
    { id: "pollings" as const, name: "ポーリング (Polling)", icon: CheckSquare },
    { id: "networks" as const, name: "ネットワーク (Network)", icon: Network },
    { id: "lines" as const, name: "ライン (Line)", icon: GitCommitHorizontal },
    { id: "drawitems" as const, name: "描画アイテム (Draw Item)", icon: Boxes },
  ];

  const loadAll = async () => {
    loading = true;
    try {
      const [n, p, net, l, d] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
        fetchNetworks().catch(() => []),
        fetchLines().catch(() => []),
        fetchDrawItems().catch(() => []),
      ]);
      nodes = n;
      pollings = p;
      networks = net;
      lines = l;
      drawItems = d;
    } catch (e) {
      console.error("Failed to load inventory data:", e);
    } finally {
      loading = false;
    }
  };

  onMount(loadAll);

  // Helpers to resolve target names
  const getNodeName = (id?: string) => {
    if (!id) return "-";
    const found = nodes.find((n) => (n.id || n.ID) === id);
    return found ? (found.name || found.Name || id) : id;
  };

  const getTargetLabel = (id?: string) => {
    if (!id) return "-";
    if (id.startsWith("NET:")) {
      const netId = id.replace("NET:", "");
      const net = networks.find((n) => (n.id || (n as any).ID) === netId);
      return net ? `[Net] ${net.name || (net as any).Name}` : `[Net] ${netId}`;
    }
    const node = nodes.find((n) => (n.id || (n as any).ID) === id);
    return node ? (node.name || (node as any).Name || id) : id;
  };

  // Check if line is orphaned (node or network missing)
  const isLineOrphaned = (l: LineEnt) => {
    const id1 = l.node_id1 || (l as any).NodeID1 || "";
    const id2 = l.node_id2 || (l as any).NodeID2 || "";
    if (!id1 || !id2) return true;

    const exists1 = id1.startsWith("NET:")
      ? networks.some((n) => (n.id || (n as any).ID) === id1.replace("NET:", ""))
      : nodes.some((n) => (n.id || (n as any).ID) === id1);

    const exists2 = id2.startsWith("NET:")
      ? networks.some((n) => (n.id || (n as any).ID) === id2.replace("NET:", ""))
      : nodes.some((n) => (n.id || (n as any).ID) === id2);

    return !exists1 || !exists2;
  };

  // Check if drawitem has questionable position or missing bindings
  const isDrawItemOffscreen = (item: DrawItemEnt) => {
    const x = item.x ?? (item as any).X ?? 0;
    const y = item.y ?? (item as any).Y ?? 0;
    return x < -500 || y < -500 || x > 15000 || y > 15000;
  };

  // Orphan & offscreen counts for summary
  const orphanLinesCount = $derived(lines.filter(isLineOrphaned).length);
  const offscreenDrawItemsCount = $derived(drawItems.filter(isDrawItemOffscreen).length);

  // Filtered lists
  const filteredNodes = $derived(
    nodes.filter((n) => {
      if (!n) return false;
      const q = searchQuery.toLowerCase();
      const name = (n.name || (n as any).Name || "").toLowerCase();
      const ip = (n.ip || (n as any).IP || "").toLowerCase();
      const descr = (n.descr || (n as any).Descr || "").toLowerCase();
      const matchSearch = !q || name.includes(q) || ip.includes(q) || descr.includes(q);
      const st = (n.state || (n as any).State || "normal").toLowerCase();
      const matchStatus = statusFilter === "all" || st === statusFilter.toLowerCase();
      return matchSearch && matchStatus;
    })
  );

  const filteredPollings = $derived(
    pollings.filter((p) => {
      const q = searchQuery.toLowerCase();
      const name = (p.name || (p as any).Name || "").toLowerCase();
      const target = (p.target || (p as any).Target || "").toLowerCase();
      const nodeName = getNodeName(p.node_id || (p as any).NodeID).toLowerCase();
      const matchSearch = !q || name.includes(q) || target.includes(q) || nodeName.includes(q);
      const matchType = typeFilter === "all" || p.type === typeFilter;
      return matchSearch && matchType;
    })
  );

  const filteredNetworks = $derived(
    networks.filter((net) => {
      const q = searchQuery.toLowerCase();
      const name = (net.name || (net as any).Name || "").toLowerCase();
      const ip = (net.ip || (net as any).IP || "").toLowerCase();
      return !q || name.includes(q) || ip.includes(q);
    })
  );

  const filteredLines = $derived(
    lines.filter((l) => {
      const q = searchQuery.toLowerCase();
      const t1 = getTargetLabel(l.node_id1 || (l as any).NodeID1).toLowerCase();
      const t2 = getTargetLabel(l.node_id2 || (l as any).NodeID2).toLowerCase();
      const info = (l.info || (l as any).Info || "").toLowerCase();
      const matchSearch = !q || t1.includes(q) || t2.includes(q) || info.includes(q);

      const orphaned = isLineOrphaned(l);
      if (statusFilter === "orphan") {
        return matchSearch && orphaned;
      }
      return matchSearch;
    })
  );

  const filteredDrawItems = $derived(
    drawItems.filter((d) => {
      const q = searchQuery.toLowerCase();
      const text = (d.text || (d as any).Text || "").toLowerCase();
      const matchSearch = !q || text.includes(q);
      const type = d.type ?? (d as any).Type ?? 2;
      const matchType = typeFilter === "all" || String(type) === typeFilter;
      return matchSearch && matchType;
    })
  );

  // Status badge styling helper
  const getStatusBadge = (state: string) => {
    switch (state?.toLowerCase()) {
      case "normal":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "warn":
      case "low":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "high":
      case "error":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      default:
        return "bg-slate-800 text-slate-400 border-slate-700";
    }
  };

  const getDrawItemTypeName = (type: number) => {
    switch (type) {
      case 2: return "テキスト (Text)";
      case 4: return "矩形枠 (Container)";
      case 6: return "ラジアルゲージ (Gauge)";
      case 7: return "バーグラフ (Bar)";
      case 8: return "折れ線 (Sparkline)";
      case 11: return "KPI カード (KPI)";
      default: return `アイテム (${type})`;
    }
  };

  const getDrawItemIcon = (type: number) => {
    switch (type) {
      case 2: return Type;
      case 4: return Square;
      case 6: return Gauge;
      case 7: return BarChart3;
      case 8: return TrendingUp;
      case 11: return CreditCard;
      default: return Boxes;
    }
  };

  // Add / Edit / Delete handlers
  const handleOpenAdd = () => {
    if (activeCategory === "nodes") {
      selectedNode = {
        id: "",
        name: "新規ノード",
        ip: "192.168.1.10",
        mac: "",
        descr: "",
        icon: "desktop",
        state: "normal",
        x: 320,
        y: 200,
      };
      showNodeDialog = true;
    } else if (activeCategory === "pollings") {
      selectedPolling = null;
      showPollingDialog = true;
    } else if (activeCategory === "networks") {
      selectedNetwork = null;
      showNetworkDialog = true;
    } else if (activeCategory === "lines") {
      selectedLine = null;
      showLineDialog = true;
    } else if (activeCategory === "drawitems") {
      selectedDrawItem = null;
      showDrawItemDialog = true;
    }
  };

  // Node actions
  const handleEditNode = (n: NodeEnt) => {
    selectedNode = { ...n };
    showNodeDialog = true;
  };
  const handleDetailNode = (n: NodeEnt) => {
    detailNode = n;
    showDetailModal = true;
  };
  const handleDeleteNode = async (id: string) => {
    if (confirm("このノードを削除してもよろしいですか？関連するポーリングやラインに影響する場合があります。")) {
      await deleteNode(id);
      await loadAll();
    }
  };

  // Polling actions
  const handleEditPolling = (p: PollingEnt) => {
    selectedPolling = { ...p };
    showPollingDialog = true;
  };
  const handleDeletePolling = async (id: string) => {
    if (confirm("このポーリング設定を削除してもよろしいですか？")) {
      await deletePolling(id);
      await loadAll();
    }
  };

  // Network actions
  const handleEditNetwork = (net: NetworkEnt) => {
    selectedNetwork = { ...net };
    showNetworkDialog = true;
  };
  const handleDeleteNetwork = async (id: string) => {
    if (confirm("このネットワークを削除してもよろしいですか？接続されているラインも削除または孤立します。")) {
      await deleteNetwork(id);
      await loadAll();
    }
  };

  // Line actions
  const handleEditLine = (l: LineEnt) => {
    selectedLine = { ...l };
    showLineDialog = true;
  };
  const handleDeleteLine = async (id: string) => {
    if (confirm("このラインを削除してもよろしいですか？")) {
      await deleteLine(id);
      await loadAll();
    }
  };

  // DrawItem actions
  const handleEditDrawItem = (d: DrawItemEnt) => {
    selectedDrawItem = { ...d };
    showDrawItemDialog = true;
  };
  const handleDeleteDrawItem = async (id: string) => {
    if (confirm("この描画アイテムを削除してもよろしいですか？")) {
      await deleteDrawItem(id);
      await loadAll();
    }
  };
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Left Sidebar (Reports suite layout) -->
  <div class="w-64 border-r border-slate-800 bg-slate-950/70 p-3 space-y-1.5 shrink-0 flex flex-col justify-between">
    <div class="space-y-1">
      <div class="px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-slate-400">
        構成・管理リスト (Items)
      </div>

      {#each categories as cat}
        {@const count =
          cat.id === "nodes" ? nodes.length :
          cat.id === "pollings" ? pollings.length :
          cat.id === "networks" ? networks.length :
          cat.id === "lines" ? lines.length :
          drawItems.length}
        <button
          type="button"
          onclick={() => {
            activeCategory = cat.id;
            searchQuery = "";
            statusFilter = "all";
            typeFilter = "all";
          }}
          class="flex w-full items-center justify-between rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeCategory === cat.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
        >
          <div class="flex items-center gap-2.5 truncate">
            <cat.icon class="h-4 w-4 shrink-0 {activeCategory === cat.id ? 'text-white' : 'text-cyan-400'}" />
            <span class="truncate">{cat.name}</span>
          </div>
          <span class="rounded-full px-2 py-0.5 text-[10px] font-mono {activeCategory === cat.id ? 'bg-white/20 text-white' : 'bg-slate-800 text-slate-400'}">
            {count}
          </span>
        </button>
      {/each}
    </div>

    <!-- Live Status & Orphan Inspection Card -->
    <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-3 text-[11px] text-slate-400 space-y-2">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-200">データ同期</span>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-950/80 border border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-400">
          ● リアルタイム
        </span>
      </div>

      {#if orphanLinesCount > 0 || offscreenDrawItemsCount > 0}
        <div class="rounded-xl border border-amber-500/30 bg-amber-500/10 p-2 text-[10px] text-amber-300 space-y-1">
          <div class="font-bold flex items-center gap-1">
            <AlertTriangle class="h-3 w-3 text-amber-400" />
            <span>マップ外・孤立アイテム検出</span>
          </div>
          {#if orphanLinesCount > 0}
            <div>・未接続ライン: <span class="font-bold">{orphanLinesCount}</span> 件</div>
          {/if}
          {#if offscreenDrawItemsCount > 0}
            <div>・画面外アイテム: <span class="font-bold">{offscreenDrawItemsCount}</span> 件</div>
          {/if}
        </div>
      {/if}

      <div class="text-[10px] font-mono text-slate-400 pt-1 border-t border-slate-800/80">
        全アイテム: <span class="text-cyan-400 font-bold">{nodes.length + pollings.length + networks.length + lines.length + drawItems.length}</span> 件
      </div>
    </div>
  </div>

  <!-- Right Main Content Canvas -->
  <div class="flex-1 overflow-hidden flex flex-col p-5 gap-4 min-w-0">
    <!-- Top Action Bar (twnoaa style) -->
    <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4 shadow-lg shrink-0">
      <div class="flex items-center gap-3">
        <!-- Search -->
        <div class="relative w-72">
          <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder={
              activeCategory === "nodes" ? "ノード名・IP・説明で検索..." :
              activeCategory === "pollings" ? "ポーリング名・ターゲットで検索..." :
              activeCategory === "networks" ? "ネットワーク名・IPで検索..." :
              activeCategory === "lines" ? "接続元・接続先・情報で検索..." :
              "テキスト・ラベルで検索..."
            }
            bind:value={searchQuery}
            class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
          />
        </div>

        <!-- Dynamic Category Filters -->
        {#if activeCategory === "nodes"}
          <select
            bind:value={statusFilter}
            class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
          >
            <option value="all">全ステータス ({nodes.length})</option>
            <option value="normal">正常</option>
            <option value="warn">注意</option>
            <option value="low">軽度障害</option>
            <option value="high">重度障害</option>
          </select>
        {:else if activeCategory === "pollings"}
          <select
            bind:value={typeFilter}
            class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
          >
            <option value="all">全プロトコル ({pollings.length})</option>
            <option value="ping">PING</option>
            <option value="http">HTTP/HTTPS</option>
            <option value="snmp">SNMP</option>
            <option value="tcp">TCP ポート</option>
            <option value="dns">DNS</option>
            <option value="ntp">NTP</option>
          </select>
        {:else if activeCategory === "lines"}
          <select
            bind:value={statusFilter}
            class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
          >
            <option value="all">全ライン ({lines.length})</option>
            {#if orphanLinesCount > 0}
              <option value="orphan">⚠️ 接続先未検出（孤立）のみ ({orphanLinesCount})</option>
            {/if}
          </select>
        {:else if activeCategory === "drawitems"}
          <select
            bind:value={typeFilter}
            class="rounded-xl border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none font-sans"
          >
            <option value="all">全アイテム種別 ({drawItems.length})</option>
            <option value="2">テキスト (Text)</option>
            <option value="4">矩形枠 (Container)</option>
            <option value="6">ラジアルゲージ (Gauge)</option>
            <option value="7">バーグラフ (Bar)</option>
            <option value="8">折れ線 (Sparkline)</option>
            <option value="11">KPI カード (KPI)</option>
          </select>
        {/if}
      </div>

      <div class="flex items-center gap-2">
        <button
          onclick={loadAll}
          disabled={loading}
          class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-all cursor-pointer"
        >
          <RefreshCw class="h-3.5 w-3.5 text-cyan-400 {loading ? 'animate-spin' : ''}" />
          <span>更新</span>
        </button>

        <button
          onclick={handleOpenAdd}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
        >
          <Plus class="h-4 w-4" />
          <span>
            {activeCategory === "nodes" ? "ノード追加" :
             activeCategory === "pollings" ? "ポーリング追加" :
             activeCategory === "networks" ? "ネットワーク追加" :
             activeCategory === "lines" ? "ライン追加" :
             "描画アイテム追加"}
          </span>
        </button>
      </div>
    </div>

    <!-- Table Container (twnoaa style) -->
    <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
      <div class="h-full overflow-y-auto overflow-x-auto">

        <!-- 1. NODES TABLE -->
        {#if activeCategory === "nodes"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-28">ステータス</th>
                <th class="py-2.5 px-3.5">ノード名</th>
                <th class="py-2.5 px-3.5">IP アドレス</th>
                <th class="py-2.5 px-3.5">MAC アドレス</th>
                <th class="py-2.5 px-3.5 w-32">マップ座標 (X, Y)</th>
                <th class="py-2.5 px-3.5">説明</th>
                <th class="py-2.5 px-3.5 text-right w-28">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredNodes as n}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3.5">
                    <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(n.state)}">
                      <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(n.state)}"></span>
                      {getStateName(n.state)}
                    </span>
                  </td>
                  <td class="py-2 px-3.5 font-bold text-slate-100 font-sans flex items-center gap-2">
                    <Laptop class="h-3.5 w-3.5 text-slate-400 shrink-0" />
                    <span>{n.name}</span>
                  </td>
                  <td class="py-2 px-3.5 text-cyan-400">{n.ip}</td>
                  <td class="py-2 px-3.5 text-slate-400">{n.mac || "-"}</td>
                  <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                    ({n.x ?? 0}, {n.y ?? 0})
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 font-sans truncate max-w-xs">{n.descr || "-"}</td>
                  <td class="py-2 px-3.5 text-right font-sans">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        onclick={() => handleDetailNode(n)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-cyan-400 transition-colors"
                        title="詳細情報 / 仮想パネル"
                      >
                        <Box class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleEditNode(n)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
                        title="編集"
                      >
                        <Edit3 class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleDeleteNode(n.id)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                        title="削除"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if filteredNodes.length === 0}
                <tr>
                  <td colspan="7" class="py-12 text-center text-slate-500 font-sans">
                    ノードが見つかりません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>

        <!-- 2. POLLINGS TABLE -->
        {:else if activeCategory === "pollings"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-28">状態</th>
                <th class="py-2.5 px-3.5">ポーリング名</th>
                <th class="py-2.5 px-3.5 w-28">種別</th>
                <th class="py-2.5 px-3.5">ターゲット</th>
                <th class="py-2.5 px-3.5">対象ノード</th>
                <th class="py-2.5 px-3.5 w-32">最新応答値</th>
                <th class="py-2.5 px-3.5 w-40">最終実行日時</th>
                <th class="py-2.5 px-3.5 text-right w-24">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredPollings as p}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3.5">
                    <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(p.state)}">
                      <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(p.state)}"></span>
                      {getStateName(p.state)}
                    </span>
                  </td>
                  <td class="py-2 px-3.5 font-bold text-slate-100 font-sans">{p.name}</td>
                  <td class="py-2 px-3.5">
                    <span class="rounded px-2 py-0.5 font-mono text-[10px] uppercase font-bold bg-cyan-500/10 text-cyan-300 border border-cyan-500/30">
                      {p.type}
                    </span>
                  </td>
                  <td class="py-2 px-3.5 text-slate-300">{p.target || "-"}</td>
                  <td class="py-2 px-3.5 text-cyan-400 font-sans">{getNodeName(p.node_id || (p as any).NodeID)}</td>
                  <td class="py-2 px-3.5 text-emerald-400">
                    {p.last_val !== undefined ? p.last_val.toFixed(2) + " ms" : "-"}
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                    {formatTimeStr(p.last_time)}
                  </td>
                  <td class="py-2 px-3.5 text-right font-sans">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        onclick={() => handleEditPolling(p)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
                        title="編集"
                      >
                        <Edit3 class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleDeletePolling(p.id)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                        title="削除"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if filteredPollings.length === 0}
                <tr>
                  <td colspan="8" class="py-12 text-center text-slate-500 font-sans">
                    ポーリング項目が見つかりません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>

        <!-- 3. NETWORKS TABLE -->
        {:else if activeCategory === "networks"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5">ネットワーク名</th>
                <th class="py-2.5 px-3.5">IP アドレス</th>
                <th class="py-2.5 px-3.5 w-28">ポート数</th>
                <th class="py-2.5 px-3.5 w-32">サイズ (W × H)</th>
                <th class="py-2.5 px-3.5 w-32">マップ座標 (X, Y)</th>
                <th class="py-2.5 px-3.5">説明</th>
                <th class="py-2.5 px-3.5 text-right w-24">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredNetworks as net}
                <tr class="hover:bg-slate-800/40 transition-colors">
                  <td class="py-2 px-3.5 font-bold text-slate-100 font-sans flex items-center gap-2">
                    <Network class="h-3.5 w-3.5 text-cyan-400 shrink-0" />
                    <span>{net.name}</span>
                  </td>
                  <td class="py-2 px-3.5 text-cyan-400">{net.ip || "-"}</td>
                  <td class="py-2 px-3.5">
                    <span class="rounded px-2 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-300 border border-slate-700">
                      {(net.ports || []).length} ポート
                    </span>
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                    {net.w} × {net.h}
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                    ({net.x}, {net.y})
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 font-sans truncate max-w-xs">{net.descr || "-"}</td>
                  <td class="py-2 px-3.5 text-right font-sans">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        onclick={() => handleEditNetwork(net)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
                        title="編集"
                      >
                        <Edit3 class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleDeleteNetwork(net.id)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                        title="削除"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if filteredNetworks.length === 0}
                <tr>
                  <td colspan="7" class="py-12 text-center text-slate-500 font-sans">
                    ネットワークが見つかりません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>

        <!-- 4. LINES TABLE -->
        {:else if activeCategory === "lines"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-28">状態</th>
                <th class="py-2.5 px-3.5">接続元 1</th>
                <th class="py-2.5 px-3.5">接続先 2</th>
                <th class="py-2.5 px-3.5 w-24">太さ</th>
                <th class="py-2.5 px-3.5">情報・ポート</th>
                <th class="py-2.5 px-3.5 w-48">接続健全性</th>
                <th class="py-2.5 px-3.5 text-right w-24">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredLines as l}
                {@const orphaned = isLineOrphaned(l)}
                <tr class="hover:bg-slate-800/40 transition-colors {orphaned ? 'bg-amber-950/20' : ''}">
                  <td class="py-2 px-3.5">
                    <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {getStatusBadge(l.state)}">
                      <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(l.state)}"></span>
                      {getStateName(l.state)}
                    </span>
                  </td>
                  <td class="py-2 px-3.5 font-sans font-medium text-slate-200">
                    {getTargetLabel(l.node_id1 || (l as any).NodeID1)}
                  </td>
                  <td class="py-2 px-3.5 font-sans font-medium text-slate-200">
                    {getTargetLabel(l.node_id2 || (l as any).NodeID2)}
                  </td>
                  <td class="py-2 px-3.5 text-slate-400">{l.width} px</td>
                  <td class="py-2 px-3.5 text-slate-400 font-sans">
                    {l.info || l.port || "-"}
                  </td>
                  <td class="py-2 px-3.5 font-sans">
                    {#if orphaned}
                      <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-400 border border-rose-500/30">
                        <AlertTriangle class="h-3 w-3" />
                        未接続（マップ描画不可）
                      </span>
                    {:else}
                      <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        <CheckCircle2 class="h-3 w-3" />
                        接続正常
                      </span>
                    {/if}
                  </td>
                  <td class="py-2 px-3.5 text-right font-sans">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        onclick={() => handleEditLine(l)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
                        title="編集"
                      >
                        <Edit3 class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleDeleteLine(l.id)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                        title="削除 (マップから消滅したラインの救済削除)"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if filteredLines.length === 0}
                <tr>
                  <td colspan="7" class="py-12 text-center text-slate-500 font-sans">
                    ラインが見つかりません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>

        <!-- 5. DRAW ITEMS TABLE -->
        {:else if activeCategory === "drawitems"}
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
              <tr>
                <th class="py-2.5 px-3.5 w-44">アイテム種別</th>
                <th class="py-2.5 px-3.5">表示テキスト / ラベル</th>
                <th class="py-2.5 px-3.5">バインド情報</th>
                <th class="py-2.5 px-3.5 w-32">マップ座標 (X, Y)</th>
                <th class="py-2.5 px-3.5 w-32">サイズ (W × H)</th>
                <th class="py-2.5 px-3.5 w-28">カラー</th>
                <th class="py-2.5 px-3.5 text-right w-24">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-800/60 font-mono text-slate-300">
              {#each filteredDrawItems as d}
                {@const type = d.type ?? (d as any).Type ?? 2}
                {@const IconComp = getDrawItemIcon(type)}
                {@const offscreen = isDrawItemOffscreen(d)}
                <tr class="hover:bg-slate-800/40 transition-colors {offscreen ? 'bg-amber-950/20' : ''}">
                  <td class="py-2 px-3.5 font-sans font-semibold text-slate-200 flex items-center gap-2">
                    <IconComp class="h-4 w-4 text-cyan-400 shrink-0" />
                    <span>{getDrawItemTypeName(type)}</span>
                  </td>
                  <td class="py-2 px-3.5 text-slate-100 font-sans font-medium">
                    {d.text || (d as any).Text || "-"}
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 font-sans text-[11px]">
                    {#if d.node_id}
                      <span>ノード: {getNodeName(d.node_id)}</span>
                    {:else}
                      <span>-</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3.5 text-[11px]">
                    <div class="flex items-center gap-1.5">
                      <span>({d.x ?? 0}, {d.y ?? 0})</span>
                      {#if offscreen}
                        <span class="rounded bg-amber-500/10 border border-amber-500/30 px-1 py-0.2 text-[9px] font-bold text-amber-400">
                          画面外
                        </span>
                      {/if}
                    </div>
                  </td>
                  <td class="py-2 px-3.5 text-slate-400 text-[11px]">
                    {d.w ?? 0} × {d.h ?? 0}
                  </td>
                  <td class="py-2 px-3.5">
                    <div class="flex items-center gap-1.5">
                      <span class="h-3 w-3 rounded-full border border-slate-700" style="background-color: {d.color || '#06b6d4'}"></span>
                      <span class="text-[10px] text-slate-400">{d.color || "#06b6d4"}</span>
                    </div>
                  </td>
                  <td class="py-2 px-3.5 text-right font-sans">
                    <div class="flex items-center justify-end gap-1">
                      <button
                        onclick={() => handleEditDrawItem(d)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors"
                        title="編集"
                      >
                        <Edit3 class="h-4 w-4" />
                      </button>
                      <button
                        onclick={() => handleDeleteDrawItem(d.id)}
                        class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors"
                        title="削除 (マップ外で拾えなくなったアイテムの削除)"
                      >
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              {/each}
              {#if filteredDrawItems.length === 0}
                <tr>
                  <td colspan="7" class="py-12 text-center text-slate-500 font-sans">
                    描画アイテムが見つかりません
                  </td>
                </tr>
              {/if}
            </tbody>
          </table>
        {/if}

      </div>
    </div>
  </div>
</div>

<!-- Modal Components -->
<NodeDialog
  bind:show={showNodeDialog}
  bind:node={selectedNode}
  onSave={loadAll}
/>

{#if detailNode}
  <NodeDetailModal
    bind:show={showDetailModal}
    node={detailNode}
  />
{/if}

<PollingDialog
  bind:show={showPollingDialog}
  bind:polling={selectedPolling}
  nodes={nodes}
  onSave={loadAll}
/>

<NetworkDialog
  bind:show={showNetworkDialog}
  bind:network={selectedNetwork}
  onSave={loadAll}
/>

<LineDialog
  bind:show={showLineDialog}
  bind:line={selectedLine}
  nodes={nodes}
  networks={networks}
  pollings={pollings}
  onSave={loadAll}
  onDelete={loadAll}
/>

<DrawItemDialog
  bind:show={showDrawItemDialog}
  bind:item={selectedDrawItem}
  nodes={nodes}
  pollings={pollings}
  onSave={loadAll}
/>
