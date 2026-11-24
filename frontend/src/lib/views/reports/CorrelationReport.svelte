<script lang="ts">
  import { _ } from "svelte-i18n";
  import { GitCompareArrows } from "@lucide/svelte";
  import CorrelationAnalysis from "../../components/CorrelationAnalysis.svelte";

  let {
    searchQuery = "",
  }: {
    searchQuery?: string;
  } = $props();
</script>

<div class="flex flex-col gap-4">
  <div
    class="flex flex-wrap items-center justify-between gap-3 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-xs"
  >
    <div class="flex items-center gap-3">
      <div
        class="p-2.5 rounded-xl bg-blue-50 dark:bg-blue-950/50 text-blue-600 dark:text-blue-400 border border-blue-100 dark:border-blue-900"
      >
        <GitCompareArrows class="w-5 h-5" />
      </div>
      <div>
        <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">
          {$_("Correlation.title")}
        </h2>
        <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
          {$_("Correlation.subtitle")}
        </p>
      </div>
    </div>
  </div>

  <CorrelationAnalysis {searchQuery} />
</div>
