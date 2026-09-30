<script lang="ts">
  import { untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    fetchMIBTree,
    runSNMPTool,
    type MIBTreeEnt,
    type SNMPToolResult,
    type NodeEnt,
    type NetworkEnt,
    type PollingEnt,
  } from "../api";
  import {
    X,
    FolderTree,
    Play,
    Copy,
    Check,
    Search,
    RefreshCw,
    Plus,
    FileSpreadsheet,
    HelpCircle,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    node = null,
    network = null,
    onAddPolling = undefined,
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    network?: NetworkEnt | null;
    onAddPolling?: (p: Partial<PollingEnt>) => void;
  }>();

  let nameOrOid = $state(".1.3.6.1.2.1.1");
  let history = $state<string[]>([".1.3.6.1.2.1.1", ".1.3.6.1.2.1.2.2", ".1.3.6.1.2.1.25"]);
  let mode = $state<"get" | "getnext" | "walk" | "table">("walk");
  let scalarOnly = $state(false);
  let rawData = $state(false);

  let isLoading = $state(false);
  let errorMessage = $state("");
  let results = $state<SNMPToolResult[]>([]);
  let selectedIndices = $state<number[]>([]);
  let copied = $state(false);

  // MIB Tree Modal State
  let showMIBTreeModal = $state(false);
  let mibTreeData = $state<MIBTreeEnt[]>([]);
  let mibTreeFilter = $state("");

  const targetName = $derived(node?.name || (node as any)?.Name || network?.name || (network as any)?.Name || "");
  const targetIP = $derived(node?.ip || (node as any)?.IP || network?.ip || (network as any)?.IP || "");
  const targetNodeId = $derived(node?.id || (node as any)?.ID || "");
  const targetNetworkId = $derived(network?.id || (network as any)?.ID || "");

  $effect(() => {
    if (show && (node || network)) {
      untrack(() => {
        errorMessage = "";
        results = [];
        selectedIndices = [];
      });
    }
  });

  const runQuery = async () => {
    if (!nameOrOid.trim() || (!targetNodeId && !targetNetworkId)) return;
    isLoading = true;
    errorMessage = "";
    selectedIndices = [];
    try {
      const q = nameOrOid.trim();
      if (!history.includes(q)) {
        history = [q, ...history.slice(0, 9)];
      }
      results = await runSNMPTool(
        { nodeId: targetNodeId || undefined, networkId: targetNetworkId || undefined },
        q,
        mode,
        rawData
      );
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isLoading = false;
    }
  };

  const handleOpenMIBTree = async () => {
    showMIBTreeModal = true;
    if (mibTreeData.length === 0) {
      try {
        mibTreeData = await fetchMIBTree();
      } catch (e) {
        errorMessage = e instanceof Error ? e.message : String(e);
      }
    }
  };

  const selectTreeOid = (oid: string) => {
    nameOrOid = oid;
    showMIBTreeModal = false;
  };

  const toggleSelect = (idx: number, isShift: boolean) => {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  };

  const copySelected = async () => {
    const list = selectedIndices.length > 0 ? selectedIndices.map((i) => results[i]) : results;
    if (list.length === 0) return;
    const lines = ["名前\tOID\t型\t値"];
    list.forEach((r) => {
      lines.push(`${r.name || r.oid}\t${r.oid}\t${r.type}\t${r.value}`);
    });
    await navigator.clipboard.writeText(lines.join("\n"));
    copied = true;
    setTimeout(() => (copied = false), 2000);
  };

  const exportCSV = () => {
    if (results.length === 0) return;
    const lines = ["Name,OID,Type,Value"];
    results.forEach((r) => {
      const v = `"${r.value.replace(/"/g, '""')}"`;
      lines.push(`"${r.name || r.oid}","${r.oid}",${r.type},${v}`);
    });
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `mib_${targetName}_${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCreatePolling = () => {
    if (selectedIndices.length !== 1 || !onAddPolling) return;
    const r = results[selectedIndices[0]];
    if (!r) return;
    onAddPolling({
      node_id: targetNodeId,
      name: `SNMP ${r.name || r.oid}`,
      type: "snmp",
      params: r.name || r.oid,
      state: "unknown",
    });
    show = false;
  };

  const filteredTreeNodes = $derived.by(() => {
    const rows: { name: string; oid: string; depth: number }[] = [];
    const visit = (items: MIBTreeEnt[], depth: number) => {
      for (const item of items) {
        const needle = mibTreeFilter.trim().toLowerCase();
        if (!needle || item.name.toLowerCase().includes(needle) || item.oid.includes(needle)) {
          rows.push({ name: item.name, oid: item.oid, depth });
        }
        if (item.children?.length) {
          visit(item.children, depth + 1);
        }
      }
    };
    visit(mibTreeData, 0);
    return rows.slice(0, 400);
  });

  const displayedResults = $derived.by(() => {
    let list = results;
    if (scalarOnly) {
      list = list.filter((r) => r.oid.endsWith(".0") || (Boolean(r.name) && r.name.endsWith(".0")));
    }
    return list;
  });
</script>

{#if show}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="MIBブラウザー"
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <FolderTree class="h-5 w-5 text-teal-500" />
          <div>
            <h2 class="text-sm font-bold">MIBブラウザー — {targetName}</h2>
            <p class="text-[11px] font-mono text-slate-500 dark:text-slate-400">{targetIP || "No IP"}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <!-- Search & Controls Toolbar -->
      <div class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/40">
        <div class="flex flex-wrap items-center gap-2">
          <div class="relative flex-1 min-w-[200px]">
            <input
              type="text"
              bind:value={nameOrOid}
              placeholder="オブジェクト名またはOID (例: .1.3.6.1.2.1.1)"
              class="w-full rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
            />
          </div>

          <button
            type="button"
            onclick={handleOpenMIBTree}
            class="flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:hover:bg-slate-800"
            title="MIBツリーから選択"
          >
            <FolderTree class="h-3.5 w-3.5 text-teal-500" />ツリー
          </button>

          <select
            onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) nameOrOid = v; }}
            class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950"
          >
            <option value="">履歴</option>
            {#each history as h}
              <option value={h}>{h}</option>
            {/each}
          </select>

          <select bind:value={mode} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
            <option value="walk">SNMP Walk</option>
            <option value="get">SNMP Get</option>
            <option value="getnext">SNMP GetNext</option>
            <option value="table">SNMP Table</option>
          </select>

          <button
            type="button"
            disabled={isLoading || !nameOrOid.trim()}
            onclick={runQuery}
            class="flex items-center gap-1.5 rounded-lg bg-teal-600 px-4 py-1.5 text-xs font-semibold text-white shadow hover:bg-teal-500 disabled:opacity-50"
          >
            {#if isLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin" />
            {:else}
              <Play class="h-3.5 w-3.5 fill-current" />
            {/if}
            実行
          </button>
        </div>

        <!-- Options row -->
        <div class="flex flex-wrap items-center justify-between gap-3 text-xs">
          <div class="flex items-center gap-4">
            <label class="flex items-center gap-1.5 cursor-pointer text-slate-600 dark:text-slate-300">
              <input type="checkbox" bind:checked={scalarOnly} class="rounded text-teal-600 focus:ring-teal-500" />
              スカラーのみ (.0)
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer text-slate-600 dark:text-slate-300">
              <input type="checkbox" bind:checked={rawData} class="rounded text-teal-600 focus:ring-teal-500" />
              Rawデータ
            </label>
            <span class="text-slate-400">取得件数: {displayedResults.length}</span>
          </div>

          <div class="flex items-center gap-2">
            {#if selectedIndices.length === 1 && onAddPolling}
              <button
                type="button"
                onclick={handleCreatePolling}
                class="flex items-center gap-1 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500"
              >
                <Plus class="h-3 w-3" />ポーリング作成
              </button>
            {/if}
            <button
              type="button"
              disabled={displayedResults.length === 0}
              onclick={copySelected}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              {#if copied}<Check class="h-3 w-3 text-emerald-500" />{:else}<Copy class="h-3 w-3" />{/if}
              {selectedIndices.length > 0 ? `選択(${selectedIndices.length})コピー` : "全てコピー"}
            </button>
            <button
              type="button"
              disabled={displayedResults.length === 0}
              onclick={exportCSV}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              <FileSpreadsheet class="h-3 w-3 text-emerald-500" />CSV
            </button>
          </div>
        </div>
      </div>

      <!-- Result Table Area -->
      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        {#if errorMessage}
          <div class="mb-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
            {errorMessage}
          </div>
        {/if}

        <div class="overflow-auto rounded-xl border border-slate-200 dark:border-slate-800">
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900">
              <tr>
                <th class="p-2 w-8 text-center">#</th>
                <th class="p-2 font-mono">オブジェクト名 / OID</th>
                <th class="p-2">型</th>
                <th class="p-2">値</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
              {#each displayedResults as r, idx}
                {@const isSelected = selectedIndices.includes(idx)}
                <tr
                  onclick={() => toggleSelect(idx, false)}
                  class={`cursor-pointer transition-colors ${isSelected ? "bg-teal-50 dark:bg-teal-950/40" : "hover:bg-slate-50 dark:hover:bg-slate-900/60"}`}
                >
                  <td class="p-2 text-center text-slate-400 font-mono text-[10px]">{idx + 1}</td>
                  <td class="p-2 font-mono text-slate-700 dark:text-slate-300 break-all">
                    <div class="font-medium text-slate-900 dark:text-slate-100">{r.name || r.oid}</div>
                    {#if r.name && r.name !== r.oid}
                      <div class="text-[10px] text-slate-400 dark:text-slate-500">{r.oid}</div>
                    {/if}
                  </td>
                  <td class="p-2 text-slate-500 whitespace-nowrap">{r.type}</td>
                  <td class="p-2 font-mono break-all font-medium">{r.value}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="p-6 text-center text-slate-500">
                    {#if isLoading}読み込み中...{:else}結果はありません (OIDを入力して「実行」を押してください){/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Sub-Modal: MIB Tree Selector -->
{#if showMIBTreeModal}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="MIBツリー"
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showMIBTreeModal = false)}
  >
    <div class="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3 dark:border-slate-800">
        <div class="flex items-center gap-2">
          <FolderTree class="h-4 w-4 text-teal-500" />
          <h3 class="text-xs font-bold">MIBツリーからOIDを選択</h3>
        </div>
        <button
          type="button"
          onclick={() => (showMIBTreeModal = false)}
          class="rounded-lg p-1 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>
      <div class="border-b border-slate-200 p-3 dark:border-slate-800">
        <div class="relative">
          <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
          <input
            type="text"
            bind:value={mibTreeFilter}
            placeholder="シンボル名またはOIDでフィルター..."
            class="w-full rounded-lg border border-slate-300 bg-white pl-8 pr-3 py-1.5 text-xs text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
          />
        </div>
      </div>
      <div class="min-h-0 flex-1 overflow-y-auto p-2">
        {#each filteredTreeNodes as node (node.oid)}
          <button
            type="button"
            onclick={() => selectTreeOid(node.oid)}
            class="block w-full truncate rounded px-2.5 py-1 text-left font-mono text-[11px] hover:bg-teal-50 dark:hover:bg-slate-800"
            style="padding-left: {8 + node.depth * 14}px"
          >
            <span class="font-semibold text-teal-600 dark:text-teal-400">{node.name}</span>
            <span class="ml-2 text-slate-400">{node.oid}</span>
          </button>
        {:else}
          <p class="p-4 text-center text-xs text-slate-500">一致するMIB項目がありません</p>
        {/each}
      </div>
    </div>
  </div>
{/if}
