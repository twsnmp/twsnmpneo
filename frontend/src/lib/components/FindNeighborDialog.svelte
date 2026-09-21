<script lang="ts">
  import { untrack } from "svelte";
  import {
    fetchNeighbors,
    connectLines,
    type NeighborLineEnt,
    type NetworkEnt,
    type NodeEnt,
  } from "../api";
  import { X, Search, Link, Check, AlertCircle, Server, Laptop, Compass, Sparkles } from "@lucide/svelte";

  let {
    show = $bindable(false),
    targetId = "",
    nodes = [],
    networks = [],
    onConnect = () => {},
  } = $props<{
    show: boolean;
    targetId: string;
    nodes?: NodeEnt[];
    networks?: NetworkEnt[];
    onConnect?: () => void;
  }>();

  let loading = $state(false);
  let errorMsg = $state("");
  let successMsg = $state("");
  let candidateLines = $state<NeighborLineEnt[]>([]);
  let candidateNetworks = $state<NetworkEnt[]>([]);
  let selectedIndices = $state<number[]>([]);

  let isNode = $derived(!targetId.startsWith("NET:"));
  let cleanID = $derived(targetId.replace(/^(NODE:|NET:)/, ""));

  let targetName = $derived.by(() => {
    if (isNode) {
      const n = nodes.find((item) => (item.id || (item as any).ID) === cleanID);
      return n ? `${n.name || (n as any).Name || cleanID} (${n.ip || (n as any).IP || ""})` : cleanID;
    } else {
      const net = networks.find((item) => (item.id || (item as any).ID) === cleanID);
      return net ? `${net.name || (net as any).Name || cleanID}` : cleanID;
    }
  });

  const getEntityLabel = (id: string, pollingOrPortId?: string) => {
    if (!id) return { name: "-", port: "" };
    if (id.startsWith("NET:")) {
      const nid = id.replace("NET:", "");
      const net = networks.find((n) => (n.id || (n as any).ID) === nid);
      const name = net ? net.name || (net as any).Name || nid : nid;
      let port = "";
      if (net && net.ports && pollingOrPortId) {
        const p = net.ports.find((port) => (port.id || (port as any).ID) === pollingOrPortId);
        if (p) port = p.name || (p as any).Name || pollingOrPortId;
      }
      return { name, port: port ? `ポート ${port}` : "", isNet: true };
    } else {
      const n = nodes.find((node) => (node.id || (node as any).ID) === id);
      const name = n ? `${n.name || (n as any).Name || id}` : id;
      return { name, port: "", isNet: false };
    }
  };

  const loadNeighbors = async () => {
    if (!targetId) return;
    loading = true;
    errorMsg = "";
    successMsg = "";
    candidateLines = [];
    candidateNetworks = [];
    selectedIndices = [];

    try {
      const resp = await fetchNeighbors(targetId);
      candidateLines = resp.Lines || [];
      candidateNetworks = resp.Networks || [];
      // Select all candidate lines by default
      selectedIndices = candidateLines.map((_, i) => i);
    } catch (e: any) {
      errorMsg = "探索エラー: " + (e.message || e);
    } finally {
      loading = false;
    }
  };

  $effect(() => {
    if (show && targetId) {
      untrack(() => {
        loadNeighbors();
      });
    }
  });

  const toggleSelectAll = () => {
    if (selectedIndices.length === candidateLines.length) {
      selectedIndices = [];
    } else {
      selectedIndices = candidateLines.map((_, i) => i);
    }
  };

  const toggleSelectIndex = (idx: number) => {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  };

  const handleConnect = async () => {
    if (selectedIndices.length === 0) {
      errorMsg = "接続するラインを選択してください。";
      return;
    }
    const linesToConnect = selectedIndices.map((i) => candidateLines[i]);
    loading = true;
    errorMsg = "";
    try {
      const res = await connectLines(linesToConnect);
      successMsg = `${res.connected} 本のラインを正常に接続しました。`;
      onConnect();
      setTimeout(() => {
        show = false;
      }, 1000);
    } catch (e: any) {
      errorMsg = "接続保存エラー: " + (e.message || e);
    } finally {
      loading = false;
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-auto max-h-[85vh] w-full max-w-3xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Compass class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100 flex items-center gap-2">
              <span>{isNode ? "ノードの接続先を探す" : "ネットワークの接続先を探す"}</span>
              <span class="text-xs font-normal text-cyan-300">({targetName})</span>
            </h2>
            <p class="text-[11px] text-slate-400">SNMP (LLDP/CDP/FDB/ARP) およびサブネット推測による自動トポロジー探索</p>
          </div>
        </div>
        <button
          type="button"
          aria-label="閉じる"
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-y-auto p-6 space-y-4 bg-slate-900/30 text-xs">
        {#if errorMsg}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <AlertCircle class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        {/if}

        {#if successMsg}
          <div class="flex items-center gap-2 rounded-xl border border-emerald-800/40 bg-emerald-950/40 p-3.5 text-xs font-medium text-emerald-300 shadow-sm">
            <Check class="h-4 w-4 text-emerald-400 shrink-0" />
            <span>{successMsg}</span>
          </div>
        {/if}

        {#if loading}
          <div class="flex flex-col items-center justify-center py-12 space-y-3 text-slate-400">
            <div class="h-8 w-8 animate-spin rounded-full border-2 border-cyan-400 border-t-transparent"></div>
            <p class="text-xs font-medium">トポロジーと接続先を探索中...</p>
          </div>
        {:else if candidateLines.length === 0}
          <div class="flex flex-col items-center justify-center py-12 text-slate-500">
            <Search class="h-10 w-10 mb-3 opacity-30" />
            <p class="text-sm font-semibold">接続可能な候補が見つかりませんでした</p>
            <p class="text-xs text-slate-500 mt-1">SW-HUBのIPアドレスやSNMP設定、または同一サブネット内のノード登録を確認してください</p>
          </div>
        {:else}
          <div class="flex items-center justify-between px-1">
            <span class="text-xs font-bold text-slate-200">
              検出された接続候補 ({candidateLines.length} 件)
            </span>
            <button
              type="button"
              onclick={toggleSelectAll}
              class="text-[11px] font-semibold text-cyan-400 hover:text-cyan-300 transition-colors cursor-pointer"
            >
              {selectedIndices.length === candidateLines.length ? "すべて解除" : "すべて選択"}
            </button>
          </div>

          <div class="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/60 shadow-lg">
            <table class="w-full text-left text-xs text-slate-300">
              <thead class="bg-slate-950/80 uppercase font-mono text-[11px] text-slate-400 border-b border-slate-800">
                <tr>
                  <th class="py-2.5 px-3 w-10 text-center">選択</th>
                  <th class="py-2.5 px-3">接続元</th>
                  <th class="py-2.5 px-3">接続先</th>
                  <th class="py-2.5 px-3">判定理由</th>
                  <th class="py-2.5 px-3 text-center">確信度</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60">
                {#each candidateLines as l, idx}
                  {@const e1 = getEntityLabel(l.node_id1 || (l as any).NodeID1, l.polling_id1 || (l as any).PollingID1)}
                  {@const e2 = getEntityLabel(l.node_id2 || (l as any).NodeID2, l.polling_id2 || (l as any).PollingID2)}
                  {@const isSelected = selectedIndices.includes(idx)}
                  <tr
                    onclick={() => toggleSelectIndex(idx)}
                    class="cursor-pointer transition-colors {isSelected ? 'bg-cyan-950/20' : 'hover:bg-slate-800/30'}"
                  >
                    <td class="py-2.5 px-3 text-center" onclick={(e) => e.stopPropagation()}>
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onchange={() => toggleSelectIndex(idx)}
                        class="rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/30 cursor-pointer"
                      />
                    </td>
                    <td class="py-2.5 px-3 font-medium text-slate-100">
                      <div class="flex items-center gap-1.5">
                        {#if e1.isNet}
                          <Server class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                        {:else}
                          <Laptop class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                        {/if}
                        <span class="truncate">{e1.name}</span>
                        {#if e1.port}
                          <span class="text-[10px] text-cyan-300 font-mono">[{e1.port}]</span>
                        {/if}
                      </div>
                    </td>
                    <td class="py-2.5 px-3 font-medium text-slate-100">
                      <div class="flex items-center gap-1.5">
                        {#if e2.isNet}
                          <Server class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                        {:else}
                          <Laptop class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                        {/if}
                        <span class="truncate">{e2.name}</span>
                        {#if e2.port}
                          <span class="text-[10px] text-cyan-300 font-mono">[{e2.port}]</span>
                        {/if}
                      </div>
                    </td>
                    <td class="py-2.5 px-3 font-mono text-[11px] text-slate-400">
                      {l.Reason || l.Info || "Heuristic"}
                    </td>
                    <td class="py-2.5 px-3 text-center">
                      <span
                        class="inline-flex items-center rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border {l.Confidence === 'strict' ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-400' : 'border-amber-500/30 bg-amber-500/10 text-amber-300'}"
                      >
                        {l.Confidence === "strict" ? "確実 (Strict)" : "推測 (Speculative)"}
                      </span>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-between border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={loadNeighbors}
          disabled={loading}
          class="flex items-center gap-1.5 px-3.5 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-xl text-xs font-semibold text-slate-300 transition-colors cursor-pointer disabled:opacity-50"
        >
          <Search class="w-3.5 h-3.5" />
          再探索
        </button>

        <div class="flex items-center gap-3">
          <button
            type="button"
            onclick={() => (show = false)}
            class="px-4 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-xl text-xs font-medium text-slate-300 transition-colors cursor-pointer"
          >
            閉じる
          </button>
          <button
            type="button"
            onclick={handleConnect}
            disabled={loading || selectedIndices.length === 0}
            class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Link class="w-4 h-4" />
            選択したラインを接続 ({selectedIndices.length})
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
