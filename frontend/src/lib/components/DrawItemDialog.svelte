<script lang="ts">
  import { saveDrawItem, type DrawItemEnt, type NodeEnt, type PollingEnt } from "../api";
  import { X, Save, Palette, Gauge, BarChart3, TrendingUp, CreditCard, Type, Square } from "@lucide/svelte";

  let { show = $bindable(false), item = $bindable<DrawItemEnt | null>(null), nodes = [], pollings = [], onSave = () => {} } = $props<{
    show: boolean;
    item: DrawItemEnt | null;
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    onSave?: (saved: DrawItemEnt) => void;
  }>();

  let type = $state(2);
  let text = $state("");
  let color = $state("#3b82f6");
  let size = $state(14);
  let w = $state(200);
  let h = $state(80);
  let nodeId = $state("");
  let pollingId = $state("");
  let value = $state(50);

  const itemTypes = [
    { value: 2, name: "テキスト (Text)", icon: Type },
    { value: 4, name: "矩形枠 (Container)", icon: Square },
    { value: 6, name: "ラジアルゲージ (Modern Gauge)", icon: Gauge },
    { value: 7, name: "バーグラフ (Bar Chart)", icon: BarChart3 },
    { value: 8, name: "折れ線トレンド (Sparkline)", icon: TrendingUp },
    { value: 11, name: "KPI カード (KPI Card)", icon: CreditCard },
  ];

  $effect(() => {
    if (item) {
      type = item.type || 2;
      text = item.text || "";
      color = item.color || "#3b82f6";
      size = item.size || 14;
      w = item.w || 200;
      h = item.h || 80;
      nodeId = item.node_id || "";
      pollingId = item.polling_id || "";
      value = item.value ?? 50;
    } else {
      type = 2;
      text = "ラベル";
      color = "#3b82f6";
      size = 14;
      w = 200;
      h = 80;
      nodeId = "";
      pollingId = "";
      value = 50;
    }
  });

  const handleSave = async () => {
    const it: DrawItemEnt = {
      ...(item || { x: 100, y: 100 }),
      type,
      text,
      color,
      size,
      w,
      h,
      node_id: nodeId,
      polling_id: pollingId,
      value,
      values: [20, 45, 30, 60, 50, 75, value],
    };

    try {
      const saved = await saveDrawItem(it);
      onSave(saved);
      show = false;
    } catch (e) {
      console.error("Save draw item error:", e);
    }
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
    <div class="w-full max-w-xl rounded-xl border border-border bg-card p-6 shadow-2xl">
      <div class="flex items-center justify-between border-b border-border pb-3">
        <div class="flex items-center gap-2 text-lg font-semibold">
          <Palette class="h-5 w-5 text-primary" />
          <span>{item?.id ? "描画アイテムの編集" : "描画アイテムの追加"}</span>
        </div>
        <button onclick={() => (show = false)} class="rounded-lg p-1 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-4 text-sm">
        <div class="col-span-2">
          <label class="block text-xs font-medium text-muted-foreground">アイテム種別</label>
          <div class="mt-1 grid grid-cols-3 gap-2">
            {#each itemTypes as it}
              <button
                type="button"
                onclick={() => (type = it.value)}
                class="flex items-center gap-2 rounded-lg border p-2 text-left text-xs transition-all {type === it.value ? 'border-primary bg-primary/10 text-primary font-semibold' : 'border-border hover:bg-muted'}"
              >
                <it.icon class="h-4 w-4" />
                <span>{it.name}</span>
              </button>
            {/each}
          </div>
        </div>

        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">表示テキスト / タイトル</label>
          <input type="text" bind:value={text} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
        </div>

        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">テーマカラー</label>
          <div class="mt-1 flex items-center gap-2">
            <input type="color" bind:value={color} class="h-8 w-12 cursor-pointer rounded border border-border bg-transparent p-0.5" />
            <input type="text" bind:value={color} class="w-full rounded-md border border-border bg-background px-2 py-1 text-xs" />
          </div>
        </div>

        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">連動ノード (任意)</label>
          <select bind:value={nodeId} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs">
            <option value="">(なし)</option>
            {#each nodes as n}
              <option value={n.id}>{n.name} ({n.ip})</option>
            {/each}
          </select>
        </div>

        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">連動ポーリング (任意)</label>
          <select bind:value={pollingId} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs">
            <option value="">(なし)</option>
            {#each pollings as p}
              <option value={p.id}>{p.name} ({p.type})</option>
            {/each}
          </select>
        </div>

        {#if type >= 6}
          <div class="col-span-2">
            <label class="block text-xs font-medium text-muted-foreground">現在値 / プレビュー値 ({value})</label>
            <input type="range" min={0} max={100} bind:value={value} class="mt-2 w-full accent-primary" />
          </div>
        {/if}
      </div>

      <div class="mt-6 flex justify-end gap-3 border-t border-border pt-4">
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
{/if}
