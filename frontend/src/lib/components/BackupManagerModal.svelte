<script lang="ts">
  import { onMount } from "svelte";
  import {
    fetchBackupStatus,
    runBackup,
    stopBackup,
    deleteBackup,
    getBackupDownloadUrl,
    type BackupStatusEnt,
    type BackupFileInfo,
  } from "../api";
  import { showConfirm, showLoading, hideLoading } from "../stores/modalStore";
  import { renderBytes, formatTimeStr } from "../common";
  import { _ } from "svelte-i18n";
  import {
    Database,
    Download,
    Trash2,
    X,
    RotateCcw,
    CheckCircle2,
    Clock,
    AlertTriangle,
    ShieldCheck,
    StopCircle,
    Layers,
    FileCode,
  } from "@lucide/svelte";

  let {
    isOpen = $bindable(false),
    onClose,
  }: {
    isOpen: boolean;
    onClose?: () => void;
  } = $props();

  let loading = $state(false);
  let actionLoading = $state(false);
  let status = $state<BackupStatusEnt | null>(null);
  let errorMsg = $state<string | null>(null);
  let successMsg = $state<string | null>(null);

  async function loadStatus() {
    try {
      loading = true;
      errorMsg = null;
      status = await fetchBackupStatus();
    } catch (e: any) {
      errorMsg = e.message || $_('system.backupFailed');
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (isOpen) {
      loadStatus();
    }
  });

  async function handleRunBackup(configOnly: boolean) {
    const title = configOnly ? $_('system.btnConfigOnlyBackup') : $_('system.btnFullBackup');
    const ok = await showConfirm({
      title: title || $_('system.backup'),
      message: $_('system.confirmBackup'),
      type: 'warning',
      confirmText: $_('common.ok'),
    });
    if (!ok) return;

    actionLoading = true;
    errorMsg = null;
    successMsg = null;
    showLoading({
      title: title || $_('system.backup'),
      message: $_('system.backupCreating'),
    });

    try {
      const res = await runBackup(configOnly);
      successMsg = `${$_('system.backupSuccess')} ${res.file} (${renderBytes(res.size)})`;
      await loadStatus();
      setTimeout(() => {
        successMsg = null;
      }, 6000);
    } catch (e: any) {
      errorMsg = e.message || $_('system.backupFailed');
      setTimeout(() => {
        errorMsg = null;
      }, 6000);
    } finally {
      actionLoading = false;
      hideLoading();
    }
  }

  async function handleStop() {
    try {
      actionLoading = true;
      await stopBackup();
      successMsg = $_('system.backupStopped');
      await loadStatus();
    } catch (e: any) {
      errorMsg = e.message;
    } finally {
      actionLoading = false;
    }
  }

  async function handleDelete(file: BackupFileInfo) {
    const ok = await showConfirm({
      title: $_('common.confirmDelete'),
      message: `${$_('system.confirmDeleteBackup')}\n\n${file.name}`,
      type: 'danger',
      confirmText: $_('common.delete'),
    });
    if (!ok) return;

    try {
      actionLoading = true;
      await deleteBackup(file.name);
      successMsg = $_('system.backupDeleted');
      await loadStatus();
      setTimeout(() => {
        successMsg = null;
      }, 5000);
    } catch (e: any) {
      errorMsg = e.message || $_('system.backupDeleteFailed');
    } finally {
      actionLoading = false;
    }
  }

  function handleCloseModal() {
    isOpen = false;
    onClose?.();
  }
</script>

{#if isOpen}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-200">
    <div class="relative w-full max-w-3xl max-h-[90vh] flex flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden">
      <!-- Header -->
      <header class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50/50 dark:bg-slate-950/40">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <Database class="w-5 h-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              {$_('system.backupManagerTitle')}
              {#if status?.running}
                <span class="inline-flex items-center gap-1.5 rounded-full bg-cyan-500/10 border border-cyan-500/30 px-2.5 py-0.5 text-[11px] font-semibold text-cyan-500 animate-pulse">
                  <span class="h-1.5 w-1.5 rounded-full bg-cyan-500 animate-ping"></span>
                  {$_('system.backupRunning')}
                </span>
              {/if}
            </h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              {$_('system.backupManagerDesc')}
            </p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={loadStatus}
            disabled={loading}
            class="p-2 text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
            title={$_('system.btnReload')}
          >
            <RotateCcw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
          </button>
          <button
            type="button"
            onclick={handleCloseModal}
            class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-xl hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
            title={$_('common.close')}
          >
            <X class="w-5 h-5" />
          </button>
        </div>
      </header>

      <!-- Alert Banners -->
      {#if successMsg}
        <div class="mx-6 mt-4 flex items-center justify-between rounded-xl border border-emerald-500/40 bg-emerald-500/10 p-3 text-xs text-emerald-600 dark:text-emerald-300">
          <div class="flex items-center gap-2">
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span class="font-medium">{successMsg}</span>
          </div>
          <button onclick={() => (successMsg = null)} class="cursor-pointer hover:opacity-75">
            <X class="w-4 h-4" />
          </button>
        </div>
      {/if}

      {#if errorMsg}
        <div class="mx-6 mt-4 flex items-center justify-between rounded-xl border border-rose-500/40 bg-rose-500/10 p-3 text-xs text-rose-600 dark:text-rose-300">
          <div class="flex items-center gap-2">
            <AlertTriangle class="w-4 h-4 shrink-0" />
            <span class="font-medium">{errorMsg}</span>
          </div>
          <button onclick={() => (errorMsg = null)} class="cursor-pointer hover:opacity-75">
            <X class="w-4 h-4" />
          </button>
        </div>
      {/if}

      <!-- Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-6">
        <!-- Status & Action Bar -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Schedule Info Box -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                <Clock class="w-3.5 h-3.5 text-cyan-500" />
                {$_('config.backupMode')}
              </span>
              <span class="text-xs font-mono font-semibold {status?.config.Mode ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-500'}">
                {status?.config.Mode ? $_('config.backupModeDaily') : $_('config.backupModeDisabled')}
              </span>
            </div>

            <div class="pt-1 text-[11px] text-slate-600 dark:text-slate-400 font-mono space-y-1">
              {#if status?.next_backup_time && status?.config.Mode}
                <div class="flex justify-between">
                  <span>{$_('system.nextBackup')}:</span>
                  <span class="text-cyan-600 dark:text-cyan-400 font-semibold">{formatTimeStr(status.next_backup_time)}</span>
                </div>
              {/if}
              {#if status?.last_backup_time}
                <div class="flex justify-between">
                  <span>{$_('system.lastBackup')}:</span>
                  <span class="text-slate-800 dark:text-slate-200">{formatTimeStr(status.last_backup_time)}</span>
                </div>
              {/if}
            </div>
          </div>

          <!-- Quick Action Box -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 flex flex-col justify-between gap-3">
            <div>
              <span class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">
                {$_('system.btnCreateBackup')}
              </span>
              <span class="block text-[11px] text-slate-500 dark:text-slate-400">
                {$_('system.createBackupDesc')}
              </span>
            </div>

            <div class="flex items-center gap-2">
              <button
                type="button"
                onclick={() => handleRunBackup(true)}
                disabled={actionLoading || status?.running}
                class="flex-1 flex items-center justify-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3 py-2 text-xs font-semibold text-cyan-600 dark:text-cyan-300 transition-colors cursor-pointer disabled:opacity-50"
                title={$_('system.btnConfigOnlyBackupTitle')}
              >
                <FileCode class="w-3.5 h-3.5" />
                <span>{$_('system.btnConfigOnlyBackup')}</span>
              </button>

              <button
                type="button"
                onclick={() => handleRunBackup(false)}
                disabled={actionLoading || status?.running}
                class="flex-1 flex items-center justify-center gap-1.5 rounded-xl border border-emerald-500/30 bg-emerald-500/10 hover:bg-emerald-500/20 px-3 py-2 text-xs font-semibold text-emerald-600 dark:text-emerald-300 transition-colors cursor-pointer disabled:opacity-50"
                title={$_('system.btnFullBackupTitle')}
              >
                <Database class="w-3.5 h-3.5" />
                <span>{$_('system.btnFullBackup')}</span>
              </button>

              {#if status?.running}
                <button
                  type="button"
                  onclick={handleStop}
                  disabled={actionLoading}
                  class="flex items-center gap-1 rounded-xl border border-rose-500/40 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-500 transition-colors cursor-pointer"
                  title={$_('system.btnStopBackupTitle')}
                >
                  <StopCircle class="w-3.5 h-3.5" />
                  <span>{$_('system.btnStopBackup')}</span>
                </button>
              {/if}
            </div>
          </div>
        </div>

        <!-- Saved Backups Table -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h4 class="text-xs font-bold text-slate-800 dark:text-slate-200 flex items-center gap-2">
              <Layers class="w-4 h-4 text-cyan-500" />
              {$_('system.backupListTitle')}
              <span class="text-slate-400 font-mono text-[11px]">({status?.backups.length || 0})</span>
            </h4>
          </div>

          <div class="rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden bg-white dark:bg-slate-900/60 shadow-xs">
            {#if !status?.backups || status.backups.length === 0}
              <div class="p-8 text-center text-xs text-slate-400 font-medium">
                {$_('system.noBackups')}
              </div>
            {:else}
              <div class="overflow-x-auto">
                <table class="w-full text-left text-xs border-collapse">
                  <thead>
                    <tr class="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/70 text-slate-500 dark:text-slate-400 font-semibold">
                      <th class="px-4 py-2.5">{$_('system.backupFileName')}</th>
                      <th class="px-4 py-2.5">{$_('system.backupFileType')}</th>
                      <th class="px-4 py-2.5">{$_('system.backupFileSize')}</th>
                      <th class="px-4 py-2.5">{$_('system.backupFileDate')}</th>
                      <th class="px-4 py-2.5 text-right">{$_('common.action')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                    {#each status.backups as file (file.name)}
                      <tr class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="px-4 py-2.5 text-slate-900 dark:text-slate-200 font-medium">
                          {file.name}
                        </td>
                        <td class="px-4 py-2.5">
                          {#if file.config_only}
                            <span class="inline-flex items-center rounded-md bg-cyan-500/10 border border-cyan-500/20 px-2 py-0.5 text-[10px] font-semibold text-cyan-600 dark:text-cyan-400">
                              {$_('system.backupTypeConfig')}
                            </span>
                          {:else}
                            <span class="inline-flex items-center rounded-md bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
                              {$_('system.backupTypeFull')}
                            </span>
                          {/if}
                        </td>
                        <td class="px-4 py-2.5 text-slate-600 dark:text-slate-400">
                          {renderBytes(file.size)}
                        </td>
                        <td class="px-4 py-2.5 text-slate-500 dark:text-slate-400">
                          {formatTimeStr(file.mod_time)}
                        </td>
                        <td class="px-4 py-2.5 text-right">
                          <div class="flex items-center justify-end gap-1.5">
                            <a
                              href={getBackupDownloadUrl(file.name)}
                              download={file.name}
                              class="p-1.5 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-500/10 rounded-lg transition-colors cursor-pointer"
                              title={$_('system.backupDownload')}
                            >
                              <Download class="w-4 h-4" />
                            </a>
                            <button
                              type="button"
                              onclick={() => handleDelete(file)}
                              disabled={actionLoading}
                              class="p-1.5 text-rose-500 hover:bg-rose-500/10 rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                              title={$_('system.backupDelete')}
                            >
                              <Trash2 class="w-4 h-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Footer -->
      <footer class="flex items-center justify-end border-t border-slate-200 dark:border-slate-800 px-6 py-3 bg-slate-50/50 dark:bg-slate-950/40">
        <button
          type="button"
          onclick={handleCloseModal}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
        >
          {$_('common.close')}
        </button>
      </footer>
    </div>
  </div>
{/if}
