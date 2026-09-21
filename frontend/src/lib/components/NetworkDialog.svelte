<script lang="ts">
  import { untrack } from "svelte";
  import { saveNetwork, type NetworkEnt } from "../api";
  import { checkNetworkPos } from "../map/map";
  import { X, Save, Server, Plus, Trash2, Network } from "@lucide/svelte";

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
  let saveError = $state("");

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (network) {
          name = network.name || network.Name || "";
          ip = network.ip || network.IP || "";
          descr = network.descr || network.Descr || "";
          const rawPorts = network.ports || network.Ports;
          totalPorts = Array.isArray(rawPorts) ? rawPorts.length : 8;
          hPorts = network.h_ports || network.HPorts || 8;
          unmanaged = network.unmanaged ?? network.Unmanaged ?? true;
          ports = Array.isArray(rawPorts) ? JSON.parse(JSON.stringify(rawPorts)) : [];
          if (ports.length === 0) generateDefaultPorts();
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
    if (!name) {
      saveError = "SW-HUB 名称は必須入力です。";
      return;
    }
    if (unmanaged && ports.length !== totalPorts) {
      generateDefaultPorts();
    }

    const net = {
      ...(network || { id: "", x: 240, y: 120 }),
      name,
      ip,
      descr,
      unmanaged,
      h_ports: Number(hPorts) || 8,
      ports,
      x: typeof network?.x === "number" && network.x > 0 ? network.x : 240,
      y: typeof network?.y === "number" && network.y > 0 ? network.y : 120,
      w: Math.max(hPorts * 45 + 30, 200),
      h: Math.ceil(ports.length / hPorts) * 60 + 50,
    };
    checkNetworkPos(net);

    try {
      const saved = await saveNetwork(net);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = "保存エラー: " + (e.message || e);
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Server class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">{network?.id ? "SW-HUB（ネットワークノード）の編集" : "SW-HUB の追加"}</h2>
            <p class="text-[11px] text-slate-400">スイッチングHUBのポートレイアウトおよび管理接続設定</p>
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
      <div class="flex-1 overflow-y-auto p-6 space-y-5 bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-400" />
            SW-HUB パラメータ
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="hub-name" class="block text-xs font-semibold text-slate-400 mb-1.5">
                SW-HUB 名称 <span class="text-rose-400">*</span>
              </label>
              <input
                id="hub-name"
                type="text"
                bind:value={name}
                placeholder="例: Core-SW-01"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-ip" class="block text-xs font-semibold text-slate-400 mb-1.5">管理 IP アドレス (任意)</label>
              <input
                id="hub-ip"
                type="text"
                bind:value={ip}
                placeholder="192.168.1.254"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-total-ports" class="block text-xs font-semibold text-slate-400 mb-1.5">ポート総数</label>
              <select
                id="hub-total-ports"
                bind:value={totalPorts}
                onchange={generateDefaultPorts}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value={4}>4 ポート</option>
                <option value={8}>8 ポート</option>
                <option value={16}>16 ポート</option>
                <option value={24}>24 ポート</option>
                <option value={48}>48 ポート</option>
              </select>
            </div>

            <div>
              <label for="hub-h-ports" class="block text-xs font-semibold text-slate-400 mb-1.5">横並びポート数 (行折り返し)</label>
              <input
                id="hub-h-ports"
                type="number"
                min={2}
                max={24}
                bind:value={hPorts}
                onchange={generateDefaultPorts}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2">
              <label for="hub-descr" class="block text-xs font-semibold text-slate-400 mb-1.5">説明・設置場所</label>
              <input
                id="hub-descr"
                type="text"
                bind:value={descr}
                placeholder="例: 本社 MDF ラック 1U"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2 pt-1">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={unmanaged}
                  class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20"
                />
                <span class="text-xs text-slate-300 font-medium">アンマネージド (非管理) HUB としてポートを自動生成・配置</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Port Preview Table -->
        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-3">
          <div class="flex items-center justify-between border-b border-slate-800 pb-2.5">
            <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2">
              <Server class="w-4 h-4 text-cyan-400" />
              ポート一覧プレビュー ({ports.length} ポート)
            </h3>
          </div>

          <div class="max-h-48 overflow-y-auto rounded-xl border border-slate-800/80 bg-slate-950/60 p-3">
            <div class="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 gap-2">
              {#each ports as p}
                <div class="flex flex-col items-center rounded-lg border border-slate-800 bg-slate-900/90 p-2 text-center shadow-xs">
                  <div class="h-2 w-2 rounded-full {p.state === 'up' ? 'bg-emerald-400 shadow-sm shadow-emerald-400/50' : 'bg-slate-600'} mb-1"></div>
                  <span class="text-[10px] font-mono text-cyan-300 font-bold">{p.name}</span>
                </div>
              {/each}
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
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
{/if}
