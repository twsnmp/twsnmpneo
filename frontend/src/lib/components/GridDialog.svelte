<script lang="ts">
  import { X, Play, TestTube2, Grid } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    show = $bindable(false),
    onExec,
    onTest,
  }: {
    show: boolean;
    onExec: (gridSize: number) => void;
    onTest: (gridSize: number) => void;
  } = $props();

  let gridSize = $state(40);
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") show = false;
    }}
  >
    <div
      class="w-full max-w-sm rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] p-5 shadow-2xl transition-colors text-slate-800 dark:text-slate-200"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/80 pb-3">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-600 dark:text-cyan-400">
            <Grid class="h-4 w-4" />
          </div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            {$_("map.gridDialog.title")}
          </h3>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <!-- Body -->
      <div class="space-y-4 py-4">
        <div>
          <label for="grid-size" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
            {$_("map.gridDialog.sizeLabel")}: <span class="font-mono text-cyan-600 dark:text-cyan-400">{gridSize}px</span>
          </label>
          <div class="flex items-center gap-3">
            <input
              id="grid-size-range"
              type="range"
              min={20}
              max={120}
              step={5}
              bind:value={gridSize}
              class="w-full accent-cyan-500 cursor-pointer"
            />
            <input
              id="grid-size"
              type="number"
              min={20}
              max={120}
              step={5}
              bind:value={gridSize}
              class="w-20 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 px-2.5 py-1.5 text-center text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
            />
          </div>
          <p class="mt-2 text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
            {$_("map.gridDialog.description")}
          </p>
        </div>
      </div>

      <!-- Actions -->
      <div class="flex items-center justify-end gap-2 border-t border-slate-100 dark:border-slate-800/80 pt-3.5">
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-xl px-3 py-1.5 text-xs font-medium text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-700 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          onclick={() => {
            onTest(gridSize);
          }}
          class="flex items-center gap-1.5 rounded-xl border border-amber-300 dark:border-amber-800/60 bg-amber-50 dark:bg-amber-950/40 px-3 py-1.5 text-xs font-semibold text-amber-800 dark:text-amber-300 hover:bg-amber-100 dark:hover:bg-amber-900/50 transition-colors cursor-pointer"
        >
          <TestTube2 class="h-3.5 w-3.5" />
          {$_("map.gridDialog.test")}
        </button>
        <button
          type="button"
          onclick={() => {
            show = false;
            onExec(gridSize);
          }}
          class="flex items-center gap-1.5 rounded-xl border border-cyan-500 bg-gradient-to-r from-cyan-600 to-cyan-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 hover:from-cyan-500 hover:to-cyan-400 transition-all cursor-pointer"
        >
          <Play class="h-3.5 w-3.5" />
          {$_("map.gridDialog.exec")}
        </button>
      </div>
    </div>
  </div>
{/if}
