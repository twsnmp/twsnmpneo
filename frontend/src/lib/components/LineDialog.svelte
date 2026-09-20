<script lang="ts">
  import { saveLine, type LineEnt, type NodeEnt, type NetworkEnt, type PollingEnt } from "../api";
  import { X, Save, GitCommitHorizontal, Trash2 } from "@lucide/svelte";

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

  $effect(() => {
    if (line) {
      nodeId1 = line.node_id1 || "";
      nodeId2 = line.node_id2 || "";
      width = line.width || 2;
      state = line.state || "normal";
    }
  });

  const handleSave = async () => {
    if (!nodeId1 || !nodeId2) return;
    const l: LineEnt = {
      ...(line || { id: "" }),
      node_id1: nodeId1,
      node_id2: nodeId2,
      width,
      state,
    };

    try {
      const saved = await saveLine(l);
      onSave(saved);
      show = false;
    } catch (e) {
      console.error("Save line error:", e);
    }
  };

  const handleDelete = () => {
    if (line?.id) {
      onDelete(line.id);
      show = false;
    }
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
    <div class="w-full max-w-lg rounded-xl border border-border bg-card p-6 shadow-2xl">
      <div class="flex items-center justify-between border-b border-border pb-3">
        <div class="flex items-center gap-2 text-lg font-semibold">
          <GitCommitHorizontal class="h-5 w-5 text-primary" />
          <span>{line?.id ? "ライン（結線）の編集" : "ラインの追加"}</span>
        </div>
        <button onclick={() => (show = false)} class="rounded-lg p-1 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-4 text-sm">
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">接続元 (ノード / SW-HUB)</label>
          <select bind:value={nodeId1} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs">
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

        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">接続先 (ノード / SW-HUB)</label>
          <select bind:value={nodeId2} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs">
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
          <label class="block text-xs font-medium text-muted-foreground">線の太さ (1〜10px)</label>
          <input type="number" min={1} max={10} bind:value={width} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs" />
        </div>

        <div>
          <label class="block text-xs font-medium text-muted-foreground">ステータス</label>
          <select bind:value={state} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs">
            <option value="normal">正常 (緑 / 青)</option>
            <option value="low">軽度障害 (黄 / オレンジ)</option>
            <option value="high">重度障害 (赤)</option>
          </select>
        </div>
      </div>

      <div class="mt-6 flex items-center justify-between border-t border-border pt-4">
        {#if line?.id}
          <button onclick={handleDelete} class="flex items-center gap-1.5 rounded-lg border border-destructive/50 px-3 py-1.5 text-xs text-destructive hover:bg-destructive/10">
            <Trash2 class="h-3.5 w-3.5" />
            削除
          </button>
        {:else}
          <div></div>
        {/if}

        <div class="flex gap-2">
          <button onclick={() => (show = false)} class="rounded-lg border border-border px-4 py-1.5 text-sm hover:bg-muted">
            キャンセル
          </button>
          <button onclick={handleSave} class="flex items-center gap-1.5 rounded-lg bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            <Save class="h-4 w-4" />
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
