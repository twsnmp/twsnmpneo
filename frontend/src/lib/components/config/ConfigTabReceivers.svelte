<script lang="ts">
  import { Radio } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    enableSyslogd = $bindable(true),
    enableTrapd = $bindable(true),
    enableNetflowd = $bindable(false),
    enableSFlowd = $bindable(false),
    enableArpWatch = $bindable(false),
    enableSshd = $bindable(false),
    enableTcpd = $bindable(false),
    enableOTel = $bindable(false),
    enableMqtt = $bindable(false),
    arpWatchRange = $bindable(""),
    arpTimeout = $bindable(60),
  }: {
    enableSyslogd: boolean;
    enableTrapd: boolean;
    enableNetflowd: boolean;
    enableSFlowd: boolean;
    enableArpWatch: boolean;
    enableSshd: boolean;
    enableTcpd: boolean;
    enableOTel: boolean;
    enableMqtt: boolean;
    arpWatchRange: string;
    arpTimeout: number;
  } = $props();
</script>

<div class="space-y-6 max-w-2xl">
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Radio class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.receiverTitle')}
    </h3>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableSyslogd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.syslogd')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.syslogDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableTrapd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.trapd')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.trapDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableNetflowd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.netflowd')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.netflowDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableSFlowd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.sflowd')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.sflowDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableArpWatch} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.arpwatch')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.arpWatchDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableOTel} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.otel')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.otelDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={enableMqtt} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.mqtt')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.mqttDesc')}</span>
        </div>
      </label>
    </div>

    {#if enableArpWatch}
      <div class="mt-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 space-y-3">
        <label for="arp-range" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('config.arpRangeLabel')}</label>
        <input
          id="arp-range"
          type="text"
          bind:value={arpWatchRange}
          placeholder={$_('config.arpRangePlaceholder')}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    {/if}
  </div>
</div>
