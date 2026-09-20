<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNodes, deleteNode, type NodeEnt } from "../api";
  import { getStateColor, getStateName } from "../common";
  import NodeDialog from "../components/NodeDialog.svelte";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import { Search, Plus, Trash2, Edit3, Box, RefreshCw, Laptop } from "@lucide/svelte";

  let nodes = $state<NodeEnt[]>([]);
  let searchQuery = $state("");
  let statusFilter = $state("all");
  let showNodeDialog = $state(false);
  let selectedNode = $state<NodeEnt | null>(null);
  let showDetailModal = $state(false);
  let detailNode = $state<NodeEnt | null>(null);

  const loadNodes = async () => {
    try {
      nodes = await fetchNodes();
    } catch (e) {
      console.error(e);
    }
  };

  onMount(loadNodes);

  const filteredNodes = $derived(
    nodes.filter((n) => {
      const matchSearch =
        n.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        n.ip.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (n.descr && n.descr.toLowerCase().includes(searchQuery.toLowerCase()));
      const matchStatus = statusFilter === "all" || n.state === statusFilter;
      return matchSearch && matchStatus;
    })
  );

  const handleOpenAdd = () => {
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
  };

  const handleEdit = (n: NodeEnt) => {
    selectedNode = { ...n };
    showNodeDialog = true;
  };

  const handleDetail = (n: NodeEnt) => {
    detailNode = n;
    showDetailModal = true;
  };

  const handleDelete = async (id: string) => {
    if (confirm("このノードを削除してもよろしいですか？")) {
      await deleteNode(id);
      await loadNodes();
    }
  };

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
</script>

<div class="flex h-[calc(100vh-4.25rem)] flex-col gap-4 p-5 overflow-hidden bg-[#0b1329] text-slate-100 font-sans">
  <!-- Controls Bar (twnoaa style) -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/90 p-4 shadow-lg">
    <div class="flex items-center gap-3">
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder="ノード名・IPで検索..."
          bind:value={searchQuery}
          class="w-full rounded-xl border border-slate-700 bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-100 placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>

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
    </div>

    <div class="flex items-center gap-2">
      <button
        onclick={loadNodes}
        class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-200 transition-all"
      >
        <RefreshCw class="h-3.5 w-3.5 text-cyan-400" />
        更新
      </button>
      <button
        onclick={handleOpenAdd}
        class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all"
      >
        <Plus class="h-4 w-4" />
        ノード追加
      </button>
    </div>
  </div>

  <!-- Table Container (twnoaa style) -->
  <div class="flex-1 overflow-hidden rounded-2xl border border-slate-800 bg-slate-900/90 shadow-lg flex flex-col min-h-0">
    <div class="h-full overflow-y-auto overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="sticky top-0 z-10 border-b border-slate-800 bg-slate-950 text-[10px] font-semibold uppercase text-slate-400">
          <tr>
            <th class="py-2.5 px-3.5 w-32">ステータス</th>
            <th class="py-2.5 px-3.5">ノード名</th>
            <th class="py-2.5 px-3.5 w-36">IP アドレス</th>
            <th class="py-2.5 px-3.5 w-40">MAC アドレス</th>
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
              <td class="py-2 px-3.5 font-bold text-slate-100 font-sans">
                <button onclick={() => handleDetail(n)} class="hover:text-cyan-400 hover:underline">
                  {n.name}
                </button>
              </td>
              <td class="py-2 px-3.5 text-cyan-400 font-mono">{n.ip}</td>
              <td class="py-2 px-3.5 text-slate-400 font-mono">{n.mac || "-"}</td>
              <td class="py-2 px-3.5 text-slate-300 font-sans truncate">{n.descr || "-"}</td>
              <td class="py-2 px-3.5 text-right">
                <div class="flex items-center justify-end gap-1.5 font-sans">
                  <button onclick={() => handleDetail(n)} class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-cyan-400 transition-colors" title="3Dパネル・詳細">
                    <Box class="h-4 w-4" />
                  </button>
                  <button onclick={() => handleEdit(n)} class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-200 transition-colors" title="編集">
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button onclick={() => handleDelete(n.id)} class="rounded-lg p-1.5 text-slate-400 hover:bg-rose-500/10 hover:text-rose-400 transition-colors" title="削除">
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredNodes.length === 0}
            <tr>
              <td colspan="6" class="py-12 text-center text-slate-500 font-sans">
                登録されているノードがありません
              </td>
            </tr>
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <NodeDialog bind:show={showNodeDialog} node={selectedNode} onSave={loadNodes} />
  <NodeDetailModal bind:show={showDetailModal} node={detailNode} />
</div>
