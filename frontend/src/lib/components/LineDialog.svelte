<script lang="ts">
  import { untrack } from "svelte";
  import { saveLine, deleteLine, type LineEnt, type NodeEnt, type NetworkEnt, type PollingEnt } from "../api";
  import { X, Save, GitCommitHorizontal, Trash2, Network, Link, Unlink, Server, Laptop, Activity } from "@lucide/svelte";

  let {
    show = $bindable(false),
    line = $bindable<LineEnt | null>(null),
    nodes = [],
    networks = [],
    pollings = [],
    onSave = () => {},
    onDelete = () => {}
  } = $props<{
    show: boolean;
    line: LineEnt | null;
    nodes?: NodeEnt[];
    networks?: NetworkEnt[];
    pollings?: PollingEnt[];
    onSave?: (saved: LineEnt) => void;
    onDelete?: (id: string) => void;
  }>();

  let nodeId1 = $state("");
  let nodeId2 = $state("");
  let pollingId1 = $state("");
  let pollingId2 = $state("");
  let pollingId = $state("");
  let info = $state("");
  let port = $state("");
  let width = $state(2);
  let state = $state("normal");
  let saveError = $state("");

  // Target object helpers
  let isNet1 = $derived(nodeId1.startsWith("NET:"));
  let isNet2 = $derived(nodeId2.startsWith("NET:"));

  let target1 = $derived(
    isNet1
      ? networks.find((n) => (n.id || (n as any).ID) === nodeId1.replace("NET:", ""))
      : nodes.find((n) => (n.id || (n as any).ID) === nodeId1)
  );

  let target2 = $derived(
    isNet2
      ? networks.find((n) => (n.id || (n as any).ID) === nodeId2.replace("NET:", ""))
      : nodes.find((n) => (n.id || (n as any).ID) === nodeId2)
  );

  // Available ports or pollings for target 1
  let options1 = $derived.by(() => {
    if (isNet1 && target1) {
      const net = target1 as NetworkEnt;
      return (net.ports || []).map((p) => ({
        id: p.id || (p as any).ID,
        label: `ポート: ${p.name || (p as any).Name}`,
      }));
    }
    return pollings
      .filter((p) => (p.node_id || (p as any).NodeID) === nodeId1)
      .map((p) => ({
        id: p.id || (p as any).ID,
        label: `${p.name || (p as any).Name} (${p.type || (p as any).Type})`,
      }));
  });

  // Available ports or pollings for target 2
  let options2 = $derived.by(() => {
    if (isNet2 && target2) {
      const net = target2 as NetworkEnt;
      return (net.ports || []).map((p) => ({
        id: p.id || (p as any).ID,
        label: `ポート: ${p.name || (p as any).Name}`,
      }));
    }
    return pollings
      .filter((p) => (p.node_id || (p as any).NodeID) === nodeId2)
      .map((p) => ({
        id: p.id || (p as any).ID,
        label: `${p.name || (p as any).Name} (${p.type || (p as any).Type})`,
      }));
  });

  // Pollings available for general Info Polling
  let infoPollingOptions = $derived.by(() => {
    return pollings
      .filter((p) => {
        const nid = p.node_id || (p as any).NodeID;
        return nid === nodeId1 || nid === nodeId2;
      })
      .map((p) => ({
        id: p.id || (p as any).ID,
        label: `${p.name || (p as any).Name} (${p.type || (p as any).Type})`,
      }));
  });

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (line) {
          nodeId1 = line.node_id1 || (line as any).NodeID1 || "";
          nodeId2 = line.node_id2 || (line as any).NodeID2 || "";
          pollingId1 = line.polling_id1 || (line as any).PollingID1 || "";
          pollingId2 = line.polling_id2 || (line as any).PollingID2 || "";
          pollingId = line.polling_id || (line as any).PollingID || "";
          info = line.info || (line as any).Info || "";
          port = line.port || (line as any).Port || "";
          width = line.width || (line as any).Width || 2;
          state = line.state || (line as any).State || "normal";
        } else {
          nodeId1 = nodes[0]?.id || "";
          nodeId2 = nodes[1]?.id || "";
          pollingId1 = "";
          pollingId2 = "";
          pollingId = "";
          info = "";
          port = "";
          width = 2;
          state = "normal";
        }
      });
    }
  });

  const handleConnectOrUpdate = async () => {
    if (!nodeId1 || !nodeId2) {
      saveError = "接続元と接続先が指定されていません。";
      return;
    }
    if (nodeId1 === nodeId2) {
      saveError = "接続元と接続先は異なる機器を指定してください。";
      return;
    }

    const payload: LineEnt = {
      ...(line || { id: "" }),
      node_id1: nodeId1,
      node_id2: nodeId2,
      polling_id1: pollingId1,
      polling_id2: pollingId2,
      polling_id: pollingId,
      info,
      port,
      width: Number(width) || 2,
      state,
    };

    try {
      const saved = await saveLine(payload);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = "保存エラー: " + (e.message || e);
    }
  };

  const handleDisconnect = async () => {
    if (line?.id) {
      if (confirm("このライン（結線）を切断しますか？")) {
        try {
          await deleteLine(line.id);
          onDelete(line.id);
          show = false;
        } catch (e: any) {
          saveError = "切断エラー: " + (e.message || e);
        }
      }
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <GitCommitHorizontal class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">{line?.id ? "ライン（結線）の編集" : "ライン（結線）の接続"}</h2>
            <p class="text-[11px] text-slate-400">ノードおよびネットワークポート間の結線設定</p>
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

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-4 bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <!-- Connection Endpoints Grid matching twsnmpfk -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Endpoint 1 -->
          <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-4 shadow-lg space-y-3">
            <div class="flex items-center gap-2 text-xs font-bold text-slate-200 border-b border-slate-800/60 pb-2">
              {#if isNet1}
                <Network class="w-4 h-4 text-cyan-400" />
                <span>ネットワーク 1</span>
              {:else}
                <Laptop class="w-4 h-4 text-emerald-400" />
                <span>ノード 1</span>
              {/if}
            </div>
            <div>
              <label for="endpoint-1-name" class="block text-[11px] font-semibold text-slate-400 mb-1">機器名称</label>
              <div id="endpoint-1-name" class="w-full rounded-xl border border-slate-800/80 bg-slate-950/80 px-3 py-2 text-xs font-semibold text-slate-100 truncate">
                {target1?.name || (target1 as any)?.Name || nodeId1 || "-"}
              </div>
            </div>
            <div>
              <label for="endpoint-1-polling" class="block text-[11px] font-semibold text-slate-400 mb-1">
                {isNet1 ? "接続ポート" : "ポーリング (状態連動)"}
              </label>
              <select
                id="endpoint-1-polling"
                bind:value={pollingId1}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="">(指定なし)</option>
                {#each options1 as opt}
                  <option value={opt.id}>{opt.label}</option>
                {/each}
              </select>
            </div>
          </div>

          <!-- Endpoint 2 -->
          <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-4 shadow-lg space-y-3">
            <div class="flex items-center gap-2 text-xs font-bold text-slate-200 border-b border-slate-800/60 pb-2">
              {#if isNet2}
                <Network class="w-4 h-4 text-cyan-400" />
                <span>ネットワーク 2</span>
              {:else}
                <Laptop class="w-4 h-4 text-emerald-400" />
                <span>ノード 2</span>
              {/if}
            </div>
            <div>
              <label for="endpoint-2-name" class="block text-[11px] font-semibold text-slate-400 mb-1">機器名称</label>
              <div id="endpoint-2-name" class="w-full rounded-xl border border-slate-800/80 bg-slate-950/80 px-3 py-2 text-xs font-semibold text-slate-100 truncate">
                {target2?.name || (target2 as any)?.Name || nodeId2 || "-"}
              </div>
            </div>
            <div>
              <label for="endpoint-2-polling" class="block text-[11px] font-semibold text-slate-400 mb-1">
                {isNet2 ? "接続ポート" : "ポーリング (状態連動)"}
              </label>
              <select
                id="endpoint-2-polling"
                bind:value={pollingId2}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="">(指定なし)</option>
                {#each options2 as opt}
                  <option value={opt.id}>{opt.label}</option>
                {/each}
              </select>
            </div>
          </div>
        </div>

        <!-- Line Parameters matching twsnmpfk -->
        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-4 shadow-lg space-y-3.5">
          <div class="flex items-center gap-2 text-xs font-bold text-slate-200 border-b border-slate-800/60 pb-2">
            <Activity class="w-4 h-4 text-cyan-400" />
            <span>ライン属性 & 情報</span>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5">
            <div>
              <label for="line-info-polling" class="block text-[11px] font-semibold text-slate-400 mb-1">情報表示ポーリング</label>
              <select
                id="line-info-polling"
                bind:value={pollingId}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="">(指定なし)</option>
                {#each infoPollingOptions as opt}
                  <option value={opt.id}>{opt.label}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="line-info-text" class="block text-[11px] font-semibold text-slate-400 mb-1">表示情報 / メモ</label>
              <input
                id="line-info-text"
                type="text"
                placeholder="1000Mbps, VLAN 10 など"
                bind:value={info}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-3.5 pt-1">
            <div>
              <label for="line-width" class="block text-[11px] font-semibold text-slate-400 mb-1">線の太さ (1〜5)</label>
              <input
                id="line-width"
                type="number"
                min={1}
                max={5}
                bind:value={width}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="line-state" class="block text-[11px] font-semibold text-slate-400 mb-1">状態 (ステータス)</label>
              <select
                id="line-state"
                bind:value={state}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="normal">正常 (Normal / Green)</option>
                <option value="warn">注意 (Warn / Amber)</option>
                <option value="low">軽度障害 (Low / Orange)</option>
                <option value="high">重度障害 (High / Red)</option>
              </select>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        {#if line?.id}
          <button
            type="button"
            onclick={handleDisconnect}
            class="flex items-center gap-1.5 rounded-xl border border-rose-800/50 bg-rose-950/40 px-4 py-2 text-xs font-bold text-rose-300 hover:bg-rose-900/60 transition-all cursor-pointer shadow-lg shadow-rose-950/50"
          >
            <Unlink class="h-4 w-4" />
            切断
          </button>
        {:else}
          <div></div>
        {/if}

        <div class="flex items-center gap-3">
          <button
            type="button"
            onclick={() => (show = false)}
            class="px-4 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-xl text-xs font-medium text-slate-300 transition-colors cursor-pointer"
          >
            キャンセル
          </button>
          <button
            type="button"
            onclick={handleConnectOrUpdate}
            class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
          >
            {#if line?.id}
              <Save class="w-4 h-4" />
              更新
            {:else}
              <Link class="w-4 h-4" />
              接続
            {/if}
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
