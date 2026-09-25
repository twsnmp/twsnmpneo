<script lang="ts">
  import { untrack } from "svelte";
  import { saveDrawItem, type DrawItemEnt, type NodeEnt, type PollingEnt } from "../api";
  import { checkItemPos } from "../map/map";
  import { _ } from "svelte-i18n";
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
  let color = $state("#06b6d4");
  let size = $state(14);
  let w = $state(200);
  let h = $state(80);
  let nodeId = $state("");
  let pollingId = $state("");
  let value = $state(50);
  let saveError = $state("");

  const rawItemTypes = [
    { value: 2, icon: Type },
    { value: 4, icon: Square },
    { value: 6, icon: Gauge },
    { value: 7, icon: BarChart3 },
    { value: 8, icon: TrendingUp },
    { value: 11, icon: CreditCard },
  ];

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (item && (item.id || (item as any).ID)) {
          type = item.type || (item as any).Type || 2;
          text = item.text || (item as any).Text || "";
          color = item.color || (item as any).Color || "#06b6d4";
          size = item.size || 14;
          w = item.w || (item as any).W || 200;
          h = item.h || (item as any).H || 80;
          nodeId = item.node_id || "";
          pollingId = item.polling_id || "";
          value = item.value ?? 50;
        } else {
          type = item?.type || 2;
          text = item?.text || $_('drawItem.defaultText');
          color = item?.color || "#06b6d4";
          size = item?.size || 14;
          w = item?.w || 200;
          h = item?.h || 80;
          nodeId = item?.node_id || "";
          pollingId = item?.polling_id || "";
          value = item?.value ?? 50;
        }
      });
    }
  });

  const handleSave = async () => {
    const it: DrawItemEnt = {
      ...(item || {}),
      id: item?.id || (item as any)?.ID || "",
      x: typeof item?.x === "number" ? item.x : (typeof (item as any)?.X === "number" ? (item as any).X : 200),
      y: typeof item?.y === "number" ? item.y : (typeof (item as any)?.Y === "number" ? (item as any).Y : 200),
      type,
      text,
      color,
      size: Number(size) || 14,
      w: Number(w) || 200,
      h: Number(h) || 80,
      node_id: nodeId,
      polling_id: pollingId,
      value: Number(value) || 0,
      values: [20, 45, 30, 60, 50, 75, Number(value) || 0],
    };
    checkItemPos(it);

    try {
      const saved = await saveDrawItem(it);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = $_('drawItem.saveError') + ": " + (e.message || e);
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Palette class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{item?.id ? $_('drawItem.editTitle') : $_('drawItem.createTitle')}</h2>
            <p class="text-[11px] text-slate-400">{$_('drawItem.subtitle')}</p>
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
      <div class="flex-1 overflow-y-auto p-6 space-y-4 bg-slate-50/60 dark:bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
          <div>
            <span class="block text-xs font-semibold text-slate-400 mb-2">{$_('drawItem.selectType')}</span>
            <div class="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {#each rawItemTypes as it}
                <button
                  type="button"
                  onclick={() => (type = it.value)}
                  class="flex items-center gap-2 rounded-xl border p-2.5 text-left text-xs transition-all cursor-pointer {type === it.value ? 'border-cyan-500/80 bg-cyan-500/15 text-cyan-300 font-bold shadow-sm shadow-cyan-500/20' : 'border-slate-800 bg-slate-950 hover:bg-slate-900 text-slate-400'}"
                >
                  <it.icon class="h-4 w-4 shrink-0 text-cyan-400" />
                  <span class="truncate">{$_(`drawItem.types.${it.value}`)}</span>
                </button>
              {/each}
            </div>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-800/80">
            <div>
              <label for="item-text" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('drawItem.text')}</label>
              <input
                id="item-text"
                type="text"
                bind:value={text}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="item-color" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('drawItem.color')}</label>
              <div class="flex items-center gap-2">
                <input
                  id="item-color"
                  type="color"
                  bind:value={color}
                  class="h-9 w-12 cursor-pointer rounded-xl border border-slate-800 bg-slate-950 p-1"
                />
                <input
                  type="text"
                  bind:value={color}
                  class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
            </div>

            <div>
              <label for="item-node" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('drawItem.bindNode')}</label>
              <select
                id="item-node"
                bind:value={nodeId}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="">{$_('drawItem.none')}</option>
                {#each nodes as n}
                  <option value={n.id}>{n.name} ({n.ip})</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="item-poll" class="block text-xs font-semibold text-slate-400 mb-1.5">{$_('drawItem.bindPolling')}</label>
              <select
                id="item-poll"
                bind:value={pollingId}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="">{$_('drawItem.none')}</option>
                {#each pollings as p}
                  <option value={p.id}>{p.name} ({p.type})</option>
                {/each}
              </select>
            </div>

            {#if type >= 6}
              <div class="md:col-span-2 pt-1">
                <label for="item-val" class="block text-xs font-semibold text-slate-400 mb-1.5">
                  {$_('drawItem.previewVal')}: <span class="font-mono text-cyan-400">{value}</span>
                </label>
                <input
                  id="item-val"
                  type="range"
                  min={0}
                  max={100}
                  bind:value={value}
                  class="w-full accent-cyan-500 cursor-pointer"
                />
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-800 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
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
