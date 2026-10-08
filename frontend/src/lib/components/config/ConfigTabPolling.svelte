<script lang="ts">
  import { Network } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    pollInt = $bindable(60),
    timeout = $bindable(1),
    retry = $bindable(1),
    snmpMode = $bindable("v2c"),
    community = $bindable("public"),
    snmpUser = $bindable(""),
    snmpPassword = $bindable(""),
  }: {
    pollInt: number;
    timeout: number;
    retry: number;
    snmpMode: string;
    community: string;
    snmpUser: string;
    snmpPassword: string;
  } = $props();
</script>

<div class="space-y-6 max-w-2xl">
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.pollingSnmpTitle')}
    </h3>
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div>
        <label for="poll-int" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.pollInt')}</label>
        <input
          id="poll-int"
          type="number"
          min={5}
          bind:value={pollInt}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="poll-timeout" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.timeout')}</label>
        <input
          id="poll-timeout"
          type="number"
          min={1}
          bind:value={timeout}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="poll-retry" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.retry')}</label>
        <input
          id="poll-retry"
          type="number"
          min={0}
          bind:value={retry}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
      <div>
        <label for="snmp-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.snmpMode')}</label>
        <select
          id="snmp-mode"
          bind:value={snmpMode}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          <option value="v2c">SNMP v2c ({$_('config.recommended')})</option>
          <option value="v3">SNMP v3</option>
          <option value="v1">SNMP v1</option>
        </select>
      </div>
      <div>
        <label for="snmp-comm" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.community')}</label>
        <input
          id="snmp-comm"
          type="text"
          bind:value={community}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>

    {#if snmpMode === "v3"}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-200 dark:border-slate-800/60">
        <div>
          <label for="snmp-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpUser')}</label>
          <input
            id="snmp-user"
            type="text"
            bind:value={snmpUser}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
          />
        </div>
        <div>
          <label for="snmp-pwd" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpPassword')}</label>
          <input
            id="snmp-pwd"
            type="password"
            bind:value={snmpPassword}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
          />
        </div>
      </div>
    {/if}
  </div>
</div>
