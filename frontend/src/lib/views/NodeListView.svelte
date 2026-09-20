<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNodes, deleteNode, type NodeEnt } from "../api";
  import { getStateColor, getStateName, getIcon } from "../common";
  import NodeDialog from "../components/NodeDialog.svelte";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import { Search, Plus, Trash2, Edit3, Box, RefreshCw, Cpu, CheckCircle, AlertTriangle, AlertCircle } from "@lucide/svelte";

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
    selectedNode = null;
    showNodeDialog = true;
  };

  const handleEdit = (n: NodeEnt) => {
    selectedNode = n;
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
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-4 p-6 overflow-hidden bg-background">
  <!-- Controls bar -->
  <div class="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border bg-card p-4 shadow-sm">
    <div class="flex items-center gap-3">
      <div class="relative w-72">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          placeholder="ノード名・IPで検索..."
          bind:value={searchQuery}
          class="w-full rounded-lg border border-border bg-background py-1.5 pl-9 pr-3 text-xs focus:border-primary focus:outline-none"
        />
      </div>

      <select
        bind:value={statusFilter}
        class="rounded-lg border border-border bg-background px-3 py-1.5 text-xs focus:border-primary focus:outline-none"
      >
        <option value="all">全ステータス ({nodes.length})</option>
        <option value="normal">正常</option>
        <option value="warn">注意</option>
        <option value="low">軽度障害</option>
        <option value="high">重度障害</option>
      </select>
    </div>

    <div class="flex items-center gap-2">
      <button onclick={loadNodes} class="flex items-center gap-1.5 rounded-lg border border-border px-3 py-1.5 text-xs hover:bg-muted font-medium">
        <RefreshCw class="h-3.5 w-3.5" />
        更新
      </button>
      <button onclick={handleOpenAdd} class="flex items-center gap-1.5 rounded-lg bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90">
        <Plus class="h-4 w-4" />
        ノード追加
      </button>
    </div>
  </div>

  <!-- Table container -->
  <div class="flex-1 overflow-hidden rounded-xl border border-border bg-card shadow-sm">
    <div class="h-full overflow-y-auto">
      <table class="w-full text-left text-xs border-collapse">
        <thead class="sticky top-0 z-10 border-b border-border bg-muted/80 backdrop-blur-sm text-muted-foreground">
          <tr>
            <th class="p-3.5">ステータス</th>
            <th class="p-3.5">ノード名</th>
            <th class="p-3.5">IP アドレス</th>
            <th class="p-3.5">MAC アドレス</th>
            <th class="p-3.5">説明</th>
            <th class="p-3.5 text-right">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-border">
          {#each filteredNodes as n}
            <tr class="hover:bg-muted/40 transition-colors">
              <td class="p-3.5">
                <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-medium" style="background-color: {getStateColor(n.state)}20; color: {getStateColor(n.state)}">
                  <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(n.state)}"></span>
                  {getStateName(n.state)}
                </span>
              </td>
              <td class="p-3.5 font-semibold text-foreground">
                <button onclick={() => handleDetail(n)} class="hover:text-primary hover:underline">
                  {n.name}
                </button>
              </td>
              <td class="p-3.5 font-mono text-muted-foreground">{n.ip}</td>
              <td class="p-3.5 font-mono text-muted-foreground">{n.mac || "-"}</td>
              <td class="p-3.5 text-muted-foreground">{n.descr || "-"}</td>
              <td class="p-3.5 text-right">
                <div class="flex items-center justify-end gap-1.5">
                  <button onclick={() => handleDetail(n)} class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground" title="3Dパネル・詳細">
                    <Box class="h-4 w-4" />
                  </button>
                  <button onclick={() => handleEdit(n)} class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground" title="編集">
                    <Edit3 class="h-4 w-4" />
                  </button>
                  <button onclick={() => handleDelete(n.id)} class="rounded p-1 text-destructive hover:bg-destructive/10" title="削除">
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
          {#if filteredNodes.length === 0}
            <tr>
              <td colspan="6" class="p-8 text-center text-muted-foreground">
                ノードが見つかりません
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
