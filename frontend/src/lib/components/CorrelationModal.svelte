<script lang="ts">
  import { _ } from "svelte-i18n";
  import { GitCompareArrows, X } from "@lucide/svelte";
  import CorrelationAnalysis from "./CorrelationAnalysis.svelte";

  let {
    show = $bindable(false),
    initialTab = "discover",
    initialPollingID1 = "",
    initialPollingID2 = "",
    initialBasePollingID = "",
  }: {
    show: boolean;
    initialTab?: "discover" | "pair";
    initialPollingID1?: string;
    initialPollingID2?: string;
    initialBasePollingID?: string;
  } = $props();

  const close = () => {
    show = false;
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-xs p-3 sm:p-6"
    role="dialog"
    aria-modal="true"
  >
    <div
      class="bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-6xl max-h-[92vh] flex flex-col overflow-hidden"
    >
      <!-- Header -->
      <div
        class="flex items-center justify-between px-4 py-3 bg-white dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800"
      >
        <div class="flex items-center gap-2">
          <GitCompareArrows class="w-5 h-5 text-blue-600 dark:text-blue-400" />
          <h2 class="text-sm font-semibold text-slate-900 dark:text-slate-100">
            {$_("Correlation.title")}
          </h2>
        </div>
        <button
          type="button"
          onclick={close}
          class="p-1.5 rounded-lg text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="w-4 h-4" />
        </button>
      </div>

      <!-- Body -->
      <div class="flex-1 overflow-y-auto p-4">
        <CorrelationAnalysis
          {initialTab}
          {initialPollingID1}
          {initialPollingID2}
          {initialBasePollingID}
          compact={true}
        />
      </div>
    </div>
  </div>
{/if}
