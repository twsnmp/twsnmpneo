<script lang="ts">
  import { _ } from 'svelte-i18n';
  import { modalStore } from '../../stores/modalStore';
  import AnimatedCatLogo from './AnimatedCatLogo.svelte';
  import { AlertTriangle, AlertCircle, Info, X } from '@lucide/svelte';

  const state = $derived(modalStore.confirmState);

  const handleCancel = () => {
    modalStore.resolveConfirm(false);
  };

  const handleConfirm = () => {
    modalStore.resolveConfirm(true);
  };

  const handleKeydown = (e: KeyboardEvent) => {
    if (!state.show) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      handleCancel();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      handleConfirm();
    }
  };

  const defaultTitle = $derived.by(() => {
    if (state.title) return state.title;
    switch (state.type) {
      case 'danger':
        return $_('common.confirmDelete') || '削除の確認';
      case 'warning':
        return $_('common.confirm') || '確認';
      case 'info':
      default:
        return $_('common.notice') || 'お知らせ';
    }
  });

  const confirmButtonClass = $derived.by(() => {
    switch (state.type) {
      case 'danger':
        return 'bg-gradient-to-r from-rose-600 to-red-600 hover:from-rose-500 hover:to-red-500 text-white shadow-rose-600/30 focus:ring-rose-500';
      case 'warning':
        return 'bg-gradient-to-r from-amber-600 to-orange-600 hover:from-amber-500 hover:to-orange-500 text-white shadow-amber-600/30 focus:ring-amber-500';
      case 'info':
      default:
        return 'bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white shadow-cyan-600/30 focus:ring-cyan-500';
    }
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if state.show}
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-200">
    <!-- Backdrop dismiss disabled to avoid accidental close, but user can cancel via button or Esc -->
    <div
      class="relative flex flex-col w-full max-w-md rounded-2xl border border-slate-200/80 bg-white/95 p-6 shadow-2xl backdrop-blur-xl dark:border-slate-800/80 dark:bg-slate-900/95 dark:shadow-cyan-950/40 transition-all overflow-hidden z-10"
      role="dialog"
      aria-modal="true"
    >
      <!-- Top corner close button -->
      <button
        type="button"
        onclick={handleCancel}
        class="absolute top-4 right-4 rounded-xl p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
        aria-label="Close"
      >
        <X class="h-4 w-4" />
      </button>

      <!-- Animated Cat Icon Section -->
      <div class="flex justify-center mb-2">
        <AnimatedCatLogo mode="confirm" type={state.type} size="md" />
      </div>

      <!-- Title & Icon -->
      <div class="text-center mt-2">
        <h3 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center justify-center gap-1.5">
          {#if state.type === 'danger'}
            <AlertCircle class="h-5 w-5 text-rose-500 shrink-0" />
          {:else if state.type === 'warning'}
            <AlertTriangle class="h-5 w-5 text-amber-500 shrink-0" />
          {:else}
            <Info class="h-5 w-5 text-cyan-500 shrink-0" />
          {/if}
          <span>{defaultTitle}</span>
        </h3>

        <!-- Message Body -->
        <p class="mt-2 text-sm text-slate-600 dark:text-slate-300 font-medium whitespace-pre-wrap leading-relaxed">
          {state.message}
        </p>

        <!-- Optional Detail Subtext -->
        {#if state.detail}
          <div class="mt-2 rounded-xl bg-slate-100/80 dark:bg-slate-800/50 p-2.5 text-xs text-slate-500 dark:text-slate-400 border border-slate-200/60 dark:border-slate-700/50 text-left font-mono break-all max-h-24 overflow-y-auto">
            {state.detail}
          </div>
        {/if}
      </div>

      <!-- Action Buttons -->
      <div class="mt-6 flex items-center justify-end gap-2.5 pt-2 border-t border-slate-100 dark:border-slate-800/80">
        <button
          type="button"
          onclick={handleCancel}
          class="flex-1 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2.5 text-xs font-bold text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700/80 transition-all cursor-pointer shadow-xs"
        >
          {state.cancelText || $_('common.cancel') || 'キャンセル'}
        </button>
        <button
          type="button"
          onclick={handleConfirm}
          class="flex-1 rounded-xl px-4 py-2.5 text-xs font-bold shadow-md transition-all cursor-pointer focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-slate-900 {confirmButtonClass}"
        >
          {state.confirmText || (state.type === 'danger' ? ($_('common.delete') || '削除') : ($_('common.ok') || 'OK'))}
        </button>
      </div>
    </div>
  </div>
{/if}
