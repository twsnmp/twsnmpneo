<script lang="ts">
  import { saveNetwork, type NetworkEnt } from "../api";
  import { X, Save, Server, Plus, Trash2 } from "@lucide/svelte";

  let { show = $bindable(false), network = $bindable<any>(null), onSave = () => {} } = $props<{
    show: boolean;
    network: any;
    onSave?: (saved: NetworkEnt) => void;
  }>();

  let name = $state("");
  let ip = $state("");
  let descr = $state("");
  let totalPorts = $state(8);
  let hPorts = $state(8);
  let unmanaged = $state(true);
  let ports = $state<any[]>([]);

  $effect(() => {
    if (network) {
      name = network.name || "";
      ip = network.ip || "";
      descr = network.descr || "";
      totalPorts = network.ports?.length || 8;
      hPorts = network.h_ports || 8;
      unmanaged = network.unmanaged ?? true;
      ports = network.ports ? JSON.parse(JSON.stringify(network.ports)) : [];
    } else {
      name = "SW-HUB";
      ip = "";
      descr = "";
      totalPorts = 8;
      hPorts = 8;
      unmanaged = true;
      generateDefaultPorts();
    }
  });

  const generateDefaultPorts = () => {
    ports = [];
    let x = 0;
    let y = 0;
    for (let i = 0; i < totalPorts; i++) {
      ports.push({
        id: `port-${i + 1}`,
        name: `#${i + 1}`,
        state: "down",
        x: x,
        y: y,
      });
      x++;
      if (x >= hPorts) {
        x = 0;
        y++;
      }
    }
  };

  const handleSave = async () => {
    if (!name) return;
    if (unmanaged && ports.length !== totalPorts) {
      generateDefaultPorts();
    }

    const net = {
      ...(network || { id: "" }),
      name,
      ip,
      descr,
      unmanaged,
      h_ports: hPorts,
      ports,
      w: Math.max(hPorts * 45 + 30, 200),
      h: Math.ceil(ports.length / hPorts) * 60 + 50,
    };

    try {
      const saved = await saveNetwork(net);
      onSave(saved);
      show = false;
    } catch (e) {
      console.error("Save network error:", e);
    }
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
    <div class="w-full max-w-2xl rounded-xl border border-border bg-card p-6 shadow-2xl">
      <div class="flex items-center justify-between border-b border-border pb-3">
        <div class="flex items-center gap-2 text-lg font-semibold">
          <Server class="h-5 w-5 text-primary" />
          <span>{network?.id ? "SW-HUB（ネットワークノード）の編集" : "SW-HUB の追加"}</span>
        </div>
        <button onclick={() => (show = false)} class="rounded-lg p-1 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-4 text-sm">
        <div>
          <label class="block text-xs font-medium text-muted-foreground">SW-HUB 名称</label>
          <input type="text" bind:value={name} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
        </div>
        <div>
          <label class="block text-xs font-medium text-muted-foreground">管理 IP アドレス (任意)</label>
          <input type="text" bind:value={ip} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
        </div>

        <div>
          <label class="block text-xs font-medium text-muted-foreground">ポート総数</label>
          <select
            bind:value={totalPorts}
            onchange={generateDefaultPorts}
            class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none"
          >
            <option value={4}>4 ポート</option>
            <option value={8}>8 ポート</option>
            <option value={16}>16 ポート</option>
            <option value={24}>24 ポート</option>
            <option value={48}>48 ポート</option>
          </select>
        </div>

        <div>
          <label class="block text-xs font-medium text-muted-foreground">横並びポート数 (折り返し)</label>
          <input
            type="number"
            min={2}
            max={24}
            bind:value={hPorts}
            onchange={generateDefaultPorts}
            class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none"
          />
        </div>

        <div class="col-span-2 flex items-center gap-2">
          <input type="checkbox" id="unmanaged" bind:checked={unmanaged} class="h-4 w-4 rounded border-border text-primary" />
          <label for="unmanaged" class="text-xs font-medium text-muted-foreground">アンマネージド (非管理) HUB としてポートを自動生成</label>
        </div>

        <!-- Port layout preview -->
        <div class="col-span-2 rounded-lg border border-border/60 bg-muted/20 p-3">
          <span class="text-xs font-semibold text-primary">ポート構成プレビュー ({ports.length} ポート)</span>
          <div class="mt-2 flex max-h-48 flex-wrap gap-2 overflow-y-auto">
            {#each ports as pt}
              <div class="flex items-center gap-1.5 rounded border border-border bg-background/80 px-2 py-1 text-xs shadow-sm">
                <span class="h-2.5 w-2.5 rounded-full {pt.state === 'up' ? 'bg-emerald-500' : 'bg-zinc-500'}"></span>
                <span class="font-medium">{pt.name}</span>
                <span class="text-[10px] text-muted-foreground">({pt.x},{pt.y})</span>
              </div>
            {/each}
          </div>
        </div>
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
