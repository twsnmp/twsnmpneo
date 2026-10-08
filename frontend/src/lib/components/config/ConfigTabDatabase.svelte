<script lang="ts">
  import { uploadGeoIP, deleteGeoIP } from "../../api";
  import { showConfirm } from "../../stores/modalStore";
  import {
    Globe,
    Upload,
    Trash2,
    Database,
    Server,
    Sliders,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    geoIPInfo = $bindable(""),
    logDays = $bindable(14),
    reportDays = $bindable(30),
    reportLimit = $bindable(10000),
    scoreThreshold = $bindable(35.0),
    fumbleThreshold = $bindable(10),
    logFormat = $bindable("parquet"),
    onRefreshConfig,
    onSuccess,
    onError,
  }: {
    geoIPInfo: string;
    logDays: number;
    reportDays: number;
    reportLimit: number;
    scoreThreshold: number;
    fumbleThreshold: number;
    logFormat: string;
    onRefreshConfig?: () => Promise<void>;
    onSuccess?: (msg: string) => void;
    onError?: (msg: string) => void;
  } = $props();

  let geoIPLoading = $state(false);
  let geoIPFileInput: HTMLInputElement | undefined = $state();

  async function handleUploadGeoIP(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    geoIPLoading = true;
    try {
      const res = await uploadGeoIP(file);
      onSuccess?.(`${$_('config.geoipUpdated')} (Ver: ${res.version || "OK"})`);
      await onRefreshConfig?.();
    } catch (err: any) {
      onError?.(`${$_('config.geoipUpdateFailed')}: ${err.message || err}`);
    } finally {
      geoIPLoading = false;
      if (geoIPFileInput) geoIPFileInput.value = "";
    }
  }

  async function handleDeleteGeoIP() {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'GeoIP削除の確認',
      message: $_('config.geoipConfirmDelete'),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    geoIPLoading = true;
    try {
      await deleteGeoIP();
      onSuccess?.($_('config.geoipDeleted'));
      geoIPInfo = "";
      await onRefreshConfig?.();
    } catch (err: any) {
      onError?.(`${$_('config.geoipDeleteFailed')}: ${err.message || err}`);
    } finally {
      geoIPLoading = false;
    }
  }
</script>

<div class="space-y-6 max-w-2xl">
  <!-- GeoIP Database Section -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Globe class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        {$_('config.geoipTitle')}
      </h3>
      {#if geoIPInfo}
        <span class="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse"></span>
          {$_('config.geoipActive')} (Ver: {geoIPInfo})
        </span>
      {:else}
        <span class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-medium text-slate-600 dark:text-slate-400">
          {$_('config.geoipInactive')}
        </span>
      {/if}
    </div>

    <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
      {$_('config.geoipDesc')}
    </p>

    <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-4 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="space-y-0.5">
          <span class="block text-xs font-semibold text-slate-800 dark:text-slate-200">{$_('config.geoipFile')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.geoipFileDesc')}</span>
        </div>

        <div class="flex items-center gap-2">
          <input
            type="file"
            accept=".mmdb"
            bind:this={geoIPFileInput}
            onchange={handleUploadGeoIP}
            class="hidden"
            id="geoip-file-input"
          />
          <label
            for="geoip-file-input"
            class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold text-cyan-600 dark:text-cyan-300 transition-colors cursor-pointer {geoIPLoading ? 'opacity-50 pointer-events-none' : ''}"
          >
            <Upload class="w-3.5 h-3.5" />
            <span>{geoIPLoading ? $_('common.loading') : $_('config.geoipApply')}</span>
          </label>

          {#if geoIPInfo}
            <button
              type="button"
              onclick={handleDeleteGeoIP}
              disabled={geoIPLoading}
              class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-600 dark:text-rose-300 transition-colors cursor-pointer disabled:opacity-50"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>{$_('common.delete')}</span>
            </button>
          {/if}
        </div>
      </div>
    </div>
  </div>

  <!-- Log Retention Section -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Database class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.logRetentionTitle')}
    </h3>
    <div class="max-w-sm">
      <label for="log-days" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.logDays')}</label>
      <input
        id="log-days"
        type="number"
        min={1}
        max={365}
        bind:value={logDays}
        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
      />
    </div>
  </div>

  <!-- Datastore Status Cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <!-- BBolt Status Card -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
        <div class="flex items-center gap-2">
          <Database class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.bboltTitle')}</h4>
        </div>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          {$_('config.statusRunning')}
        </span>
      </div>
      <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
        {$_('config.bboltDesc')}
      </p>
      <div class="pt-2 text-[11px] text-slate-700 dark:text-slate-300 font-mono space-y-1">
        <div class="flex justify-between">
          <span class="text-slate-500 dark:text-slate-400">{$_('config.dbPath')}</span>
          <span class="text-slate-900 dark:text-slate-200">./data/twsnmpneo.db</span>
        </div>
        <div class="flex justify-between">
          <span class="text-slate-500 dark:text-slate-400">{$_('config.integrityCheck')}</span>
          <span class="text-emerald-600 dark:text-emerald-400 font-semibold">{$_('config.statusNormalOk')}</span>
        </div>
      </div>
    </div>

    <!-- Parquet Status Card -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
        <div class="flex items-center gap-2">
          <Server class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.parquetTitle')}</h4>
        </div>
        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
          {$_('config.statusRunning')}
        </span>
      </div>
      <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
        {$_('config.parquetDesc')}
      </p>
      <div class="pt-2 text-[11px] text-slate-700 dark:text-slate-300 font-mono space-y-1">
        <div class="flex justify-between">
          <span class="text-slate-500 dark:text-slate-400">{$_('config.saveDir')}</span>
          <span class="text-slate-900 dark:text-slate-200">./data/logs</span>
        </div>
        <div class="flex justify-between">
          <span class="text-slate-500 dark:text-slate-400">{$_('config.compression')}</span>
          <span class="text-cyan-600 dark:text-cyan-400">Snappy / Parquet v2</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Format selector -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
    <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.logFormatTitle')}</h4>
    <p class="text-[11px] text-slate-600 dark:text-slate-400">
      {$_('config.logFormatDesc')}
    </p>
    <div class="flex items-center gap-4 pt-1">
      <label class="flex items-center gap-2 cursor-pointer">
        <input type="radio" bind:group={logFormat} value="parquet" class="text-cyan-500 focus:ring-cyan-500/20" />
        <span class="text-xs text-slate-800 dark:text-slate-200 font-medium">{$_('config.parquetOption')}</span>
      </label>
    </div>
  </div>

  <!-- Report Retention & Anomaly Score Thresholds -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="border-b border-slate-200 dark:border-slate-800 pb-2.5">
      <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Sliders class="w-4 h-4 text-cyan-500" />
        {$_('config.reportSettingTitle')}
      </h4>
      <p class="text-[11px] text-slate-600 dark:text-slate-400 mt-1">
        {$_('config.reportSettingDesc')}
      </p>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="report-days" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
          {$_('config.reportDaysLabel')}
        </label>
        <input
          id="report-days"
          type="number"
          min="1"
          max="365"
          bind:value={reportDays}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="report-limit" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
          {$_('config.reportLimitLabel')}
        </label>
        <input
          id="report-limit"
          type="number"
          min="100"
          max="1000000"
          step="500"
          bind:value={reportLimit}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="score-threshold" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
          {$_('config.scoreThresholdLabel')}
        </label>
        <input
          id="score-threshold"
          type="number"
          min="1"
          max="100"
          step="0.5"
          bind:value={scoreThreshold}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="fumble-threshold" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
          {$_('config.fumbleThresholdLabel')}
        </label>
        <input
          id="fumble-threshold"
          type="number"
          min="1"
          max="1000"
          bind:value={fumbleThreshold}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>
  </div>
</div>
