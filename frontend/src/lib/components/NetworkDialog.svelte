<script lang="ts">
  import { untrack } from "svelte";
  import { saveNetwork, refreshNetworkPorts, type NetworkEnt } from "../api";
  import { checkNetworkPos } from "../map/map";
  import { _ } from "svelte-i18n";
  import { X, Save, Server, Network, RotateCw, AlertTriangle, ShieldCheck, Globe } from "@lucide/svelte";

  let { show = $bindable(false), network = $bindable<any>(null), onSave = () => {} } = $props<{
    show: boolean;
    network: any;
    onSave?: (saved: NetworkEnt) => void;
  }>();

  let name = $state("");
  let ip = $state("");
  let descr = $state("");
  let url = $state("");
  let totalPorts = $state(8);
  let hPorts = $state(8);
  let unmanaged = $state(false);
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpUser = $state("");
  let snmpPassword = $state("");
  let snmpPort = $state(161);
  let arpWatch = $state(false);
  let lldp = $state(false);
  let errorMsg = $state("");
  let ports = $state<any[]>([]);
  let saveError = $state("");

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (network && (network.id || network.ID)) {
          name = network.name || network.Name || "";
          ip = network.ip || network.IP || "";
          descr = network.descr || network.Descr || "";
          url = network.url || network.URL || "";
          const rawPorts = network.ports || network.Ports;
          totalPorts = Array.isArray(rawPorts) ? rawPorts.length : 8;
          hPorts = network.h_ports || network.HPorts || 8;
          unmanaged = network.unmanaged ?? network.Unmanaged ?? false;
          snmpMode = network.snmp_mode || network.SnmpMode || "v2c";
          community = network.community || network.Community || "public";
          snmpUser = network.user || network.User || "";
          snmpPassword = network.password || network.Password || "";
          snmpPort = Number(network.snmp_port || network.SnmpPort || 161);
          arpWatch = network.arp_watch ?? network.ArpWatch ?? false;
          lldp = network.lldp ?? network.LLDP ?? false;
          errorMsg = network.error || network.Error || "";
          ports = Array.isArray(rawPorts) ? JSON.parse(JSON.stringify(rawPorts)) : [];
          if (ports.length === 0 && unmanaged) generateDefaultPorts();
        } else {
          name = network?.name || $_('network.defaultName');
          ip = network?.ip || "";
          descr = network?.descr || "";
          url = network?.url || "";
          totalPorts = network?.ports?.length || 8;
          hPorts = network?.h_ports || 8;
          unmanaged = network?.unmanaged ?? false;
          snmpMode = network?.snmp_mode || "v2c";
          community = network?.community || "public";
          snmpUser = network?.user || "";
          snmpPassword = network?.password || "";
          snmpPort = Number(network?.snmp_port || 161);
          arpWatch = network?.arp_watch ?? false;
          lldp = network?.lldp ?? false;
          errorMsg = "";
          ports = Array.isArray(network?.ports) ? JSON.parse(JSON.stringify(network.ports)) : [];
          if (ports.length === 0 && unmanaged) generateDefaultPorts();
        }
      });
    }
  });

  const generateDefaultPorts = () => {
    ports = [];
    let x = 0;
    let y = 0;
    for (let i = 0; i < totalPorts; i++) {
      ports.push({
        id: `port-${i + 1}`,
        name: `#${i + 1}`,
        state: "down",
        x: x,
        y: y,
      });
      x++;
      if (x >= hPorts) {
        x = 0;
        y++;
      }
    }
  };

  const handleSave = async () => {
    if (!name) {
      saveError = $_('network.requiredName');
      return;
    }
    if (unmanaged && ports.length !== totalPorts) {
      generateDefaultPorts();
    }

    const net = {
      ...(network || { id: "", x: 240, y: 120 }),
      name,
      ip,
      descr,
      url,
      unmanaged,
      snmp_mode: snmpMode,
      community,
      user: snmpUser,
      password: snmpPassword,
      snmp_port: Number(snmpPort) || 161,
      arp_watch: arpWatch,
      lldp,
      h_ports: Number(hPorts) || 8,
      ports,
      x: typeof network?.x === "number" && network.x > 0 ? network.x : 240,
      y: typeof network?.y === "number" && network.y > 0 ? network.y : 120,
      w: Math.max(hPorts * 45 + 30, 200),
      h: Math.ceil(ports.length / hPorts) * 60 + 50,
    };
    checkNetworkPos(net);

    try {
      const saved = await saveNetwork(net);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = $_('network.saveError') + ": " + (e.message || e);
    }
  };

  let refreshingPorts = $state(false);
  const handleRefreshPorts = async () => {
    const netId = network?.id || network?.ID;
    if (!netId) return;
    refreshingPorts = true;
    saveError = "";
    try {
      // First save current SNMP settings so backend uses the up-to-date community/mode
      const netToSave = {
        ...(network || { id: netId }),
        name,
        ip,
        descr,
        url,
        unmanaged,
        snmp_mode: snmpMode,
        community,
        user: snmpUser,
        password: snmpPassword,
        snmp_port: Number(snmpPort) || 161,
        arp_watch: arpWatch,
        h_ports: Number(hPorts) || 8,
      };
      await saveNetwork(netToSave);

      const updated = await refreshNetworkPorts(netId);
      if (updated.ports && updated.ports.length > 0) {
        ports = JSON.parse(JSON.stringify(updated.ports));
        totalPorts = ports.length;
      }
      lldp = updated.lldp ?? updated.LLDP ?? false;
      errorMsg = updated.error || updated.Error || "";
      onSave(updated);
    } catch (e: any) {
      saveError = $_('network.refreshError') + ": " + (e.message || e);
    } finally {
      refreshingPorts = false;
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-auto max-h-[92vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Network class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{network?.id ? $_('network.editTitle') : $_('network.createTitle')}</h2>
            <p class="text-[11px] text-slate-400">{$_('network.subtitle')}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_('common.close')}
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-5 bg-slate-50/60 dark:bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        {#if errorMsg}
          <div class="flex items-center gap-2 rounded-xl border border-amber-800/40 bg-amber-950/40 p-3.5 text-xs font-medium text-amber-300 shadow-sm">
            <AlertTriangle class="h-4 w-4 text-amber-400 shrink-0" />
            <span>SNMP状態: {errorMsg}</span>
          </div>
        {/if}

        <!-- Basic Parameters -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
            {$_('network.paramsTitle')}
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="hub-name" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                {$_('network.name')} <span class="text-rose-400">*</span>
              </label>
              <input
                id="hub-name"
                type="text"
                bind:value={name}
                placeholder="Network-01"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-ip" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.ip')}</label>
              <input
                id="hub-ip"
                type="text"
                bind:value={ip}
                placeholder="192.168.1.254"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-url" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                <span class="flex items-center gap-1.5">
                  <Globe class="w-3.5 h-3.5 text-slate-400" />
                  {$_('network.url')}
                </span>
              </label>
              <input
                id="hub-url"
                type="text"
                bind:value={url}
                placeholder="http://192.168.1.254"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="hub-descr" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.descr')}</label>
              <input
                id="hub-descr"
                type="text"
                bind:value={descr}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2 pt-1 border-t border-slate-100 dark:border-slate-800/60">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={unmanaged}
                  class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-600 focus:ring-cyan-500/20"
                />
                <span class="text-xs text-slate-700 dark:text-slate-300 font-medium">{$_('network.unmanaged')}</span>
              </label>
            </div>
          </div>
        </div>

        {#if unmanaged}
          <!-- Unmanaged Settings -->
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
            <h3 class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
              <Server class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
              {$_('network.portsTitle')}
            </h3>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label for="hub-total-ports" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.totalPorts')}</label>
                <select
                  id="hub-total-ports"
                  bind:value={totalPorts}
                  onchange={generateDefaultPorts}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                >
                  <option value={4}>4 {$_('network.portsUnit')}</option>
                  <option value={8}>8 {$_('network.portsUnit')}</option>
                  <option value={16}>16 {$_('network.portsUnit')}</option>
                  <option value={24}>24 {$_('network.portsUnit')}</option>
                  <option value={48}>48 {$_('network.portsUnit')}</option>
                </select>
              </div>

              <div>
                <label for="hub-h-ports-unmanaged" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.hPorts')}</label>
                <input
                  id="hub-h-ports-unmanaged"
                  type="number"
                  min={2}
                  max={48}
                  bind:value={hPorts}
                  onchange={generateDefaultPorts}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
            </div>
          </div>
        {:else}
          <!-- Managed SNMP Settings -->
          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
              <h3 class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
                <ShieldCheck class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                {$_('network.snmpTitle')}
              </h3>
              <div>
                {#if lldp}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                    {$_('network.lldpSupport')}
                  </span>
                {:else}
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-slate-500/10 text-slate-400 border border-slate-500/20">
                    {$_('network.lldpNotSupport')}
                  </span>
                {/if}
              </div>
            </div>

            <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label for="hub-snmp-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.snmpMode')}</label>
                <select
                  id="hub-snmp-mode"
                  bind:value={snmpMode}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                >
                  <option value="v1">SNMP v1</option>
                  <option value="v2c">SNMP v2c</option>
                  <option value="v3auth">SNMP v3 (AuthNoPriv / SHA)</option>
                  <option value="v3authpriv">SNMP v3 (AuthPriv / SHA-AES)</option>
                  <option value="v3authprivex">SNMP v3 (AuthPriv / SHA256-AES256)</option>
                  <option value="v3sha256aes128">SNMP v3 (AuthPriv / SHA256-AES128)</option>
                  <option value="v3sha512aes256">SNMP v3 (AuthPriv / SHA512-AES256)</option>
                </select>
              </div>

              {#if snmpMode === "v1" || snmpMode === "v2c"}
                <div>
                  <label for="hub-snmp-community" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.community')}</label>
                  <input
                    id="hub-snmp-community"
                    type="text"
                    bind:value={community}
                    placeholder="public"
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                  />
                </div>
              {:else}
                <div>
                  <label for="hub-snmp-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.snmpUser')}</label>
                  <input
                    id="hub-snmp-user"
                    type="text"
                    bind:value={snmpUser}
                    placeholder="user"
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                  />
                </div>
                <div>
                  <label for="hub-snmp-password" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.snmpPassword')}</label>
                  <input
                    id="hub-snmp-password"
                    type="password"
                    bind:value={snmpPassword}
                    placeholder="••••••••"
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                  />
                </div>
              {/if}

              <div>
                <label for="hub-snmp-port" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.snmpPort')}</label>
                <input
                  id="hub-snmp-port"
                  type="number"
                  min={1}
                  max={65535}
                  bind:value={snmpPort}
                  placeholder="161"
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>

              <div>
                <label for="hub-h-ports-managed" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('network.hPorts')}</label>
                <input
                  id="hub-h-ports-managed"
                  type="number"
                  min={2}
                  max={48}
                  bind:value={hPorts}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>

              <div class="flex items-center pt-6">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    bind:checked={arpWatch}
                    class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-600 focus:ring-cyan-500/20"
                  />
                  <span class="text-xs text-slate-700 dark:text-slate-300 font-medium">{$_('network.arpWatch')}</span>
                </label>
              </div>
            </div>
          </div>
        {/if}

        <!-- Port Preview Table -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
          <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
            <h3 class="text-xs font-bold text-slate-800 dark:text-slate-100 flex items-center gap-2">
              <Server class="w-4 h-4 text-cyan-600 dark:text-cyan-400" />
              {$_('network.portsTitle')} ({ports.length} {$_('network.portsUnit')})
            </h3>
            {#if (network?.id || network?.ID) && !unmanaged}
              <button
                type="button"
                onclick={handleRefreshPorts}
                disabled={refreshingPorts}
                class="flex items-center gap-1.5 px-2.5 py-1 bg-cyan-50 dark:bg-cyan-950/60 hover:bg-cyan-100 dark:hover:bg-cyan-900/60 border border-cyan-300 dark:border-cyan-800/50 text-cyan-700 dark:text-cyan-300 rounded-lg text-[11px] font-medium transition-colors cursor-pointer disabled:opacity-50"
              >
                <RotateCw class="w-3.5 h-3.5 {refreshingPorts ? 'animate-spin' : ''}" />
                <span>{refreshingPorts ? $_('network.refreshing') : $_('network.refreshPorts')}</span>
              </button>
            {/if}
          </div>

          <div class="max-h-48 overflow-y-auto rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-3">
            {#if ports.length === 0}
              <div class="py-6 text-center text-xs text-slate-400">
                ポートが未登録です。「{$_('network.refreshPorts')}」を実行してください。
              </div>
            {:else}
              <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-8 gap-2">
                {#each ports as p}
                  <div class="flex flex-col items-center rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/90 p-2 text-center shadow-xs">
                    <div
                      class="h-2 w-2 rounded-full mb-1 {p.state === 'up' ? 'bg-emerald-500 shadow-sm shadow-emerald-500/50' : p.state === 'down' ? 'bg-rose-500 shadow-sm shadow-rose-500/50' : 'bg-slate-400 dark:bg-slate-600'}"
                      title="State: {p.state || 'unknown'}"
                    ></div>
                    <span class="text-[10px] font-mono text-cyan-700 dark:text-cyan-300 font-bold truncate max-w-full" title={p.name}>
                      {p.name}
                    </span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={handleSave}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
        >
          <Save class="w-4 h-4" />
          {$_('common.save')}
        </button>
      </div>
    </div>
  </div>
{/if}
