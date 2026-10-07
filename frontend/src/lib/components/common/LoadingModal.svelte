<script lang="ts">
  import { _ } from 'svelte-i18n';
  import { modalStore } from '../../stores/modalStore';
  import AnimatedCatLogo from './AnimatedCatLogo.svelte';
  import { Loader2, X } from '@lucide/svelte';

  const state = $derived(modalStore.loadingState);

  const handleCancel = () => {
    if (state.onCancel) {
      state.onCancel();
    }
    modalStore.hideLoading();
  };
</script>

{#if state.show}
  <div class="fixed inset-0 z-[110] flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-200">
    <div
      class="relative flex flex-col items-center w-full max-w-sm rounded-2xl border border-cyan-500/30 bg-white/95 p-6 shadow-2xl backdrop-blur-xl dark:border-cyan-500/30 dark:bg-slate-900/95 dark:shadow-cyan-950/50 transition-all overflow-hidden z-10"
      role="dialog"
      aria-modal="true"
    >
      <!-- Optional Close Button if Cancelable -->
      {#if state.cancelable}
        <button
          type="button"
          onclick={handleCancel}
          class="absolute top-4 right-4 rounded-xl p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
          aria-label="Cancel"
        >
          <X class="h-4 w-4" />
        </button>
      {/if}

      <!-- Animated Cat Logo with Radar & Scan Effects -->
      <div class="my-2">
        <AnimatedCatLogo mode="loading" size="md" />
      </div>

      <!-- Title & Spinner -->
      <div class="text-center mt-3 space-y-1.5 w-full">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center justify-center gap-2">
          <Loader2 class="h-4 w-4 animate-spin text-cyan-500" />
          <span>{state.title || $_('common.processing') || '処理中...'}</span>
        </h3>

        {#if state.message}
          <p class="text-xs text-slate-500 dark:text-slate-400 font-medium whitespace-pre-wrap leading-relaxed">
            {state.message}
          </p>
        {/if}

        <!-- Animated Cyber Loading Bar -->
        <div class="mt-4 w-full bg-slate-100 dark:bg-slate-800 rounded-full h-1.5 overflow-hidden border border-slate-200/50 dark:border-slate-700/50">
          <div class="h-full bg-gradient-to-r from-cyan-500 via-teal-400 to-blue-500 rounded-full animate-pulse w-full"></div>
        </div>
      </div>

      <!-- Optional Cancel Button -->
      {#if state.cancelable}
        <div class="mt-5 w-full">
          <button
            type="button"
            onclick={handleCancel}
            class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700/80 transition-all cursor-pointer shadow-xs"
          >
            {state.cancelText || $_('common.cancel') || 'キャンセル'}
          </button>
        </div>
      {/if}
    </div>
  </div>
{/if}
