<script lang="ts">
  import { untrack } from "svelte";
  import { saveLine, type LineEnt, type NodeEnt, type NetworkEnt, type PollingEnt } from "../api";
  import { X, Save, GitCommitHorizontal, Trash2, Network } from "@lucide/svelte";

  let { show = $bindable(false), line = $bindable<LineEnt | null>(null), nodes = [], networks = [], pollings = [], onSave = () => {}, onDelete = () => {} } = $props<{
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
  let width = $state(2);
  let state = $state("normal");
  let saveError = $state("");

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (line) {
          nodeId1 = line.node_id1 || (line as any).NodeID1 || "";
          nodeId2 = line.node_id2 || (line as any).NodeID2 || "";
          width = line.width || (line as any).Width || 2;
          state = line.state || (line as any).State || "normal";
        } else {
          nodeId1 = nodes[0]?.id || "";
          nodeId2 = nodes[1]?.id || "";
          width = 2;
          state = "normal";
        }
      });
    }
  });

  const handleSave = async () => {
    if (!nodeId1 || !nodeId2) {
      saveError = "接続元と接続先を選択してください。";
      return;
    }
    if (nodeId1 === nodeId2) {
      saveError = "接続元と接続先は異なる機器を選択してください。";
      return;
    }
    const l: LineEnt = {
      ...(line || { id: "" }),
      node_id1: nodeId1,
      node_id2: nodeId2,
      width: Number(width) || 2,
      state,
    };

    try {
      const saved = await saveLine(l);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = "保存エラー: " + (e.message || e);
    }
  };

  const handleDelete = () => {
    if (line?.id) {
      if (confirm("このライン（結線）を削除しますか？")) {
        onDelete(line.id);
        show = false;
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-lg flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <GitCommitHorizontal class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">{line?.id ? "ライン（結線）の編集" : "ライン（結線）の追加"}</h2>
            <p class="text-[11px] text-slate-400">ノードおよびSW-HUBポート間の接続線設定</p>
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

        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-400" />
            結線パラメータ
          </h3>

          <div class="space-y-3.5">
            <div>
              <label for="line-src" class="block text-xs font-semibold text-slate-400 mb-1.5">接続元 (ノード / SW-HUB)</label>
              <select
                id="line-src"
                bind:value={nodeId1}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <optgroup label="ノード">
                  {#each nodes as n}
                    <option value={n.id}>{n.name} ({n.ip})</option>
                  {/each}
                </optgroup>
                {#if networks.length > 0}
                  <optgroup label="SW-HUB">
                    {#each networks as net}
                      <option value={`NET:${net.id}`}>{net.name}</option>
                    {/each}
                  </optgroup>
                {/if}
              </select>
            </div>

            <div>
              <label for="line-dst" class="block text-xs font-semibold text-slate-400 mb-1.5">接続先 (ノード / SW-HUB)</label>
              <select
                id="line-dst"
                bind:value={nodeId2}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <optgroup label="ノード">
                  {#each nodes as n}
                    <option value={n.id}>{n.name} ({n.ip})</option>
                  {/each}
                </optgroup>
                {#if networks.length > 0}
                  <optgroup label="SW-HUB">
                    {#each networks as net}
                      <option value={`NET:${net.id}`}>{net.name}</option>
                    {/each}
                  </optgroup>
                {/if}
              </select>
            </div>

            <div class="grid grid-cols-2 gap-3.5 pt-1">
              <div>
                <label for="line-width" class="block text-xs font-semibold text-slate-400 mb-1.5">線の太さ (1〜10px)</label>
                <input
                  id="line-width"
                  type="number"
                  min={1}
                  max={10}
                  bind:value={width}
                  class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>

              <div>
                <label for="line-state" class="block text-xs font-semibold text-slate-400 mb-1.5">初期ステータス</label>
                <select
                  id="line-state"
                  bind:value={state}
                  class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                >
                  <option value="normal">正常 (Normal / Cyan-Green)</option>
                  <option value="warn">注意 (Warn / Amber)</option>
                  <option value="low">軽度障害 (Low / Orange)</option>
                  <option value="high">重度障害 (High / Red)</option>
                </select>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        {#if line?.id}
          <button
            type="button"
            onclick={handleDelete}
            class="flex items-center gap-1.5 rounded-xl border border-rose-800/50 bg-rose-950/30 px-3.5 py-2 text-xs font-semibold text-rose-400 hover:bg-rose-900/40 transition-colors cursor-pointer"
          >
            <Trash2 class="h-3.5 w-3.5" />
            削除
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
            onclick={handleSave}
            class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
          >
            <Save class="w-4 h-4" />
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
