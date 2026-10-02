<script lang="ts">
  import { _ } from "svelte-i18n";
  import { Sparkles, RefreshCw, X } from "@lucide/svelte";

  interface Props {
    show: boolean;
    selectedLogText: string;
    aiAnswer: string;
    aiLoading: boolean;
    onClose?: () => void;
  }

  let {
    show = $bindable(false),
    selectedLogText = "",
    aiAnswer = "",
    aiLoading = false,
    onClose,
  }: Props = $props();

  const handleClose = () => {
    show = false;
    onClose?.();
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-sm">
    <div
      class="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-6 shadow-2xl overflow-hidden text-slate-800 dark:text-slate-100"
    >
      <!-- Dialog Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
        <div class="flex items-center gap-2 text-base font-bold text-cyan-600 dark:text-cyan-400">
          <Sparkles class="h-5 w-5" />
          <span>{$_('log.aiAssistant')}</span>
        </div>
        <button
          type="button"
          onclick={handleClose}
          aria-label={$_('log.close')}
          class="rounded-lg p-1 text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-800 dark:hover:text-white cursor-pointer transition-colors"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Target Log Text Preview -->
      <div
        class="my-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-3 font-mono text-xs text-slate-700 dark:text-slate-300 break-all max-h-32 overflow-y-auto"
      >
        {selectedLogText}
      </div>

      <!-- AI Response Area -->
      <div
        class="flex-1 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-4 text-xs leading-relaxed text-slate-800 dark:text-slate-200 whitespace-pre-wrap"
      >
        {#if aiLoading}
          <div class="flex items-center gap-2 text-cyan-600 dark:text-cyan-400">
            <RefreshCw class="h-4 w-4 animate-spin" />
            <span>{$_('log.aiReasoning')}</span>
          </div>
        {:else}
          {aiAnswer}
        {/if}
      </div>

      <!-- Footer Actions -->
      <div class="mt-4 flex justify-end border-t border-slate-200 dark:border-slate-800 pt-3">
        <button
          type="button"
          onclick={handleClose}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_('log.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
