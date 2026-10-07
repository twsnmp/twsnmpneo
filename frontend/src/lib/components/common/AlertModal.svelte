<script lang="ts">
  import { _ } from 'svelte-i18n';
  import { modalStore } from '../../stores/modalStore';
  import AnimatedCatLogo from './AnimatedCatLogo.svelte';
  import { AlertTriangle, AlertCircle, Info, X } from '@lucide/svelte';

  const state = $derived(modalStore.alertState);

  const handleClose = () => {
    modalStore.resolveAlert();
  };

  const handleKeydown = (e: KeyboardEvent) => {
    if (!state.show) return;
    if (e.key === 'Escape' || e.key === 'Enter') {
      e.preventDefault();
      handleClose();
    }
  };
</script>

<svelte:window onkeydown={handleKeydown} />

{#if state.show}
  <div class="fixed inset-0 z-[105] flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-200">
    <div
      class="relative flex flex-col w-full max-w-md rounded-2xl border border-slate-200/80 bg-white/95 p-6 shadow-2xl backdrop-blur-xl dark:border-slate-800/80 dark:bg-slate-900/95 dark:shadow-cyan-950/40 transition-all overflow-hidden z-10"
      role="dialog"
      aria-modal="true"
    >
      <button
        type="button"
        onclick={handleClose}
        class="absolute top-4 right-4 rounded-xl p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
        aria-label="Close"
      >
        <X class="h-4 w-4" />
      </button>

      <div class="flex justify-center mb-2">
        <AnimatedCatLogo mode="alert" type={state.type} size="md" />
      </div>

      <div class="text-center mt-2">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center justify-center gap-1.5">
          {#if state.type === 'danger'}
            <AlertCircle class="h-5 w-5 text-rose-500 shrink-0" />
          {:else if state.type === 'warning'}
            <AlertTriangle class="h-5 w-5 text-amber-500 shrink-0" />
          {:else}
            <Info class="h-5 w-5 text-cyan-500 shrink-0" />
          {/if}
          <span>{state.title || (state.type === 'danger' ? ($_('common.error') || 'エラー') : ($_('common.notice') || 'お知らせ'))}</span>
        </h3>

        <p class="mt-2 text-sm text-slate-600 dark:text-slate-300 font-medium whitespace-pre-wrap leading-relaxed">
          {state.message}
        </p>

        {#if state.detail}
          <div class="mt-2 rounded-xl bg-slate-100/80 dark:bg-slate-800/50 p-2.5 text-xs text-slate-500 dark:text-slate-400 border border-slate-200/60 dark:border-slate-700/50 text-left font-mono break-all max-h-24 overflow-y-auto">
            {state.detail}
          </div>
        {/if}
      </div>

      <div class="mt-6 flex items-center justify-center pt-2 border-t border-slate-100 dark:border-slate-800/80">
        <button
          type="button"
          onclick={handleClose}
          class="min-w-28 rounded-xl bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 px-6 py-2.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 dark:focus:ring-offset-slate-900"
        >
          {state.confirmText || $_('common.ok') || 'OK'}
        </button>
      </div>
    </div>
  </div>
{/if}
