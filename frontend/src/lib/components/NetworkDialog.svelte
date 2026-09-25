<script lang="ts">
  import { untrack } from "svelte";
  import { saveNetwork, refreshNetworkPorts, type NetworkEnt } from "../api";
  import { checkNetworkPos } from "../map/map";
  import { _ } from "svelte-i18n";
  import { X, Save, Server, Plus, Trash2, Network, RotateCw } from "@lucide/svelte";

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
        if (network && (network.id || network.ID)) {
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
          name = network?.name || $_('network.defaultName');
          ip = network?.ip || "";
          descr = network?.descr || "";
          totalPorts = network?.ports?.length || 8;
          hPorts = network?.h_ports || 8;
          unmanaged = network?.unmanaged ?? true;
          ports = Array.isArray(network?.ports) ? JSON.parse(JSON.stringify(network.ports)) : [];
          if (ports.length === 0) generateDefaultPorts();
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
      saveError = $_('network.requiredName');
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
      saveError = $_('network.saveError') + ": " + (e.message || e);
    }
  };

  let refreshingPorts = $state(false);
  const handleRefreshPorts = async () => {
    const netId = network?.id || network?.ID;
    if (!netId) return;
    refreshingPorts = true;
    saveError = "";
    try {
      const updated = await refreshNetworkPorts(netId);
      if (updated.ports && updated.ports.length > 0) {
        ports = JSON.parse(JSON.stringify(updated.ports));
        totalPorts = ports.length;
      }
      onSave(updated);
    } catch (e: any) {
      saveError = "SNMPポート取得失敗: " + (e.message || e);
    } finally {
      refreshingPorts = false;
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Network class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{network?.id ? $_('network.editTitle') : $_('network.createTitle')}</h2>
            <p class="text-[11px] text-slate-400">{$_('network.subtitle')}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_('common.close')}
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-5 bg-slate-50/60 dark:bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-400" />
            {$_('network.paramsTitle')}
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="hub-name" class="block text-xs font-semibold text-slate-400 mb-1.5">
                {$_('network.name')} <span class="text-rose-400">*</span>
              </label>
              <input
                id="hub-name"
                type="text"
                bind:value={name}
                placeholder="Network-01"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-ip" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('network.ip')}</label>
              <input
                id="hub-ip"
                type="text"
                bind:value={ip}
                placeholder="192.168.1.254"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-total-ports" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('network.totalPorts')}</label>
              <select
                id="hub-total-ports"
                bind:value={totalPorts}
                onchange={generateDefaultPorts}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value={4}>4 {$_('network.portsUnit')}</option>
                <option value={8}>8 {$_('network.portsUnit')}</option>
                <option value={16}>16 {$_('network.portsUnit')}</option>
                <option value={24}>24 {$_('network.portsUnit')}</option>
                <option value={48}>48 {$_('network.portsUnit')}</option>
              </select>
            </div>

            <div>
              <label for="hub-h-ports" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('network.hPorts')}</label>
              <input
                id="hub-h-ports"
                type="number"
                min={2}
                max={24}
                bind:value={hPorts}
                onchange={generateDefaultPorts}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2">
              <label for="hub-descr" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('network.descr')}</label>
              <input
                id="hub-descr"
                type="text"
                bind:value={descr}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2 pt-1">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={unmanaged}
                  class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-600 focus:ring-cyan-500/20"
                />
                <span class="text-xs text-slate-700 dark:text-slate-300 font-medium">{$_('network.unmanaged')}</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Port Preview Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
            <h3 class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
              <Server class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
              {$_('network.portsTitle')} ({ports.length} {$_('network.portsUnit')})
            </h3>
            {#if network?.id || network?.ID}
              <button
                type="button"
                onclick={handleRefreshPorts}
                disabled={refreshingPorts}
                class="flex items-center gap-1.5 px-2.5 py-1 bg-cyan-50 dark:bg-cyan-950/60 hover:bg-cyan-100 dark:hover:bg-cyan-900/60 border border-cyan-300 dark:border-cyan-800/50 text-cyan-700 dark:text-cyan-300 rounded-lg text-[11px] font-medium transition-colors cursor-pointer disabled:opacity-50"
              >
                <RotateCw class="w-3.5 h-3.5 {refreshingPorts ? 'animate-spin' : ''}" />
                <span>{refreshingPorts ? '検索中...' : 'SNMPポート再検索'}</span>
              </button>
            {/if}
          </div>

          <div class="max-h-48 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3">
            <div class="grid grid-cols-4 sm:grid-cols-6 md:grid-cols-8 gap-2">
              {#each ports as p}
                <div class="flex flex-col items-center rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 p-2 text-center shadow-xs">
                  <div class="h-2 w-2 rounded-full {p.state === 'up' ? 'bg-emerald-500 shadow-sm shadow-emerald-500/50' : 'bg-slate-400 dark:bg-slate-600'} mb-1"></div>
                  <span class="text-[10px] font-mono text-cyan-700 dark:text-cyan-300 font-bold">{p.name}</span>
                </div>
              {/each}
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={handleSave}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
        >
          <Save class="w-4 h-4" />
          {$_('common.save')}
        </button>
      </div>
    </div>
  </div>
{/if}
