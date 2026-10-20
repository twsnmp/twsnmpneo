<script lang="ts">
  import { _ } from "svelte-i18n";
  import { X, Map } from "@lucide/svelte";
  import ConfigTabLoc from "./config/ConfigTabLoc.svelte";

  let {
    show = $bindable(false),
    onSaved,
  }: {
    show: boolean;
    onSaved?: () => void;
  } = $props();

  const handleSaved = () => {
    if (onSaved) onSaved();
  };
</script>

{#if show}
  <!-- Backdrop -->
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="flex flex-col w-full max-w-xl max-h-[90vh] rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-4 py-3 bg-slate-50 dark:bg-slate-800/50">
        <div class="flex items-center gap-2">
          <div class="flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Map class="h-4 w-4" />
          </div>
          <h2 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            {$_("location.settings") || "GIS Map Settings"}
          </h2>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-700 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <!-- Body -->
      <div class="flex-1 overflow-y-auto p-4">
        <ConfigTabLoc onSaved={handleSaved} />
      </div>
    </div>
  </div>
{/if}
