<script lang="ts">
  import { untrack } from "svelte";
  import { defaultIconList, getImageIconList, addrModeList, snmpModeList, getIconImage, getIconCode } from "../common";
  import { saveNode, type NodeEnt } from "../api";
  import { checkNodePos } from "../map/map";
  import { X, Save, Cpu, Shield, Key, Network, Globe } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let { show = $bindable(false), node = $bindable<NodeEnt | null>(null), onSave = () => {} } = $props<{
    show: boolean;
    node: NodeEnt | null;
    onSave?: (saved: NodeEnt) => void;
  }>();

  let name = $state("");
  let ip = $state("");
  let mac = $state("");
  let descr = $state("");
  let icon = $state("desktop");
  let image = $state("");
  let addrMode = $state("ip");
  let autoAck = $state(false);
  let url = $state("");

  // SNMP
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpPort = $state(161);
  let user = $state("");
  let password = $state("");

  // SSH
  let sshUser = $state("");
  let publicKey = $state("");

  // gNMI
  let gnmiPort = $state("57400");
  let gnmiEncoding = $state("json_ietf");
  let gnmiUser = $state("");
  let gnmiPassword = $state("");

  let saveError = $state("");

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (node && node.id) {
          name = node.name || (node as any).Name || "";
          ip = node.ip || (node as any).IP || "";
          mac = node.mac || (node as any).MAC || "";
          descr = node.descr || (node as any).Descr || "";
          icon = node.icon || (node as any).Icon || "desktop";
          image = node.image || (node as any).Image || "";
          addrMode = (node as any).addr_mode || (node as any).AddrMode || "ip";
          autoAck = (node as any).auto_ack ?? (node as any).AutoAck ?? false;
          url = (node as any).url || (node as any).URL || "";

          snmpMode = (node as any).snmp_mode || (node as any).SnmpMode || "v2c";
          community = (node as any).community || (node as any).Community || "public";
          snmpPort = Number((node as any).snmp_port || (node as any).SnmpPort || 161);
          user = (node as any).user || (node as any).User || "";
          password = (node as any).password || (node as any).Password || "";

          sshUser = (node as any).ssh_user || (node as any).SSHUser || "";
          publicKey = (node as any).public_key || (node as any).PublicKey || "";
          gnmiPort = (node as any).gnmi_port || (node as any).GNMIPort || "57400";
          gnmiEncoding = (node as any).gnmi_encoding || (node as any).GNMIEncoding || "json_ietf";
          gnmiUser = (node as any).gnmi_user || (node as any).GNMIUser || "";
          gnmiPassword = (node as any).gnmi_password || (node as any).GNMIPassword || "";
        } else {
          const rawName = node?.name || (node as any)?.Name || "";
          if (rawName && rawName !== "新規ノード" && rawName !== "New Node") {
            name = rawName;
          } else {
            name = $_('node.defaultName');
          }
          ip = node?.ip || (node as any)?.IP || "";
          mac = node?.mac || (node as any)?.MAC || "";
          descr = node?.descr || (node as any)?.Descr || "";
          icon = node?.icon || (node as any)?.Icon || "desktop";
          image = node?.image || (node as any)?.Image || "";
          addrMode = (node as any)?.addr_mode || (node as any)?.AddrMode || "ip";
          autoAck = (node as any)?.auto_ack ?? (node as any)?.AutoAck ?? false;
          url = (node as any)?.url || (node as any)?.URL || "";
          snmpMode = (node as any)?.snmp_mode || (node as any)?.SnmpMode || "v2c";
          community = (node as any)?.community || (node as any)?.Community || "public";
          snmpPort = Number((node as any)?.snmp_port || (node as any)?.SnmpPort || 161);
          user = (node as any)?.user || (node as any)?.User || "";
          password = (node as any)?.password || (node as any)?.Password || "";
          sshUser = (node as any)?.ssh_user || (node as any)?.SSHUser || "";
          publicKey = (node as any)?.public_key || (node as any)?.PublicKey || "";
          gnmiPort = (node as any)?.gnmi_port || (node as any)?.GNMIPort || "57400";
          gnmiEncoding = (node as any)?.gnmi_encoding || (node as any)?.GNMIEncoding || "json_ietf";
          gnmiUser = (node as any)?.gnmi_user || (node as any)?.GNMIUser || "";
          gnmiPassword = (node as any)?.gnmi_password || (node as any)?.GNMIPassword || "";
        }
      });
    }
  });

  const handleSave = async () => {
    if (!name || (!ip && addrMode !== "host")) {
      saveError = $_('node.saveError');
      return;
    }
    const n: any = {
      ...(node || { id: "", state: "normal" }),
      name,
      ip,
      mac,
      descr,
      icon,
      image,
      addr_mode: addrMode,
      auto_ack: autoAck,
      url,
      snmp_mode: snmpMode,
      community,
      snmp_port: Number(snmpPort) || 161,
      user,
      password,
      ssh_user: sshUser,
      public_key: publicKey,
      gnmi_port: gnmiPort,
      gnmi_encoding: gnmiEncoding,
      gnmi_user: gnmiUser,
      gnmi_password: gnmiPassword,
      x: typeof node?.x === "number" && node.x > 0 ? node.x : 320,
      y: typeof node?.y === "number" && node.y > 0 ? node.y : 200,
    };
    checkNodePos(n);

    try {
      const saved = await saveNode(n);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = $_('node.saveError') + ": " + (e.message || e);
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
    <div class="flex h-auto max-h-[90vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Cpu class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{node?.id ? $_('node.editTitle') : $_('node.createTitle')}</h2>
            <p class="text-[11px] text-slate-400">{$_('app.subtitle')}</p>
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

        <!-- Section 1: Basic Information -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
            {$_('node.tabBasic')}
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="node-name" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                {$_('node.name')} <span class="text-rose-500">*</span>
              </label>
              <input
                id="node-name"
                type="text"
                bind:value={name}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-ip" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                {$_('node.ip')} {#if addrMode !== 'host'}<span class="text-rose-500">*</span>{/if}
              </label>
              <input
                id="node-ip"
                type="text"
                bind:value={ip}
                placeholder={addrMode === 'host' ? $_('node.ipHostPlaceholder') : ''}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-mac" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.mac')}</label>
              <input
                id="node-mac"
                type="text"
                bind:value={mac}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-icon" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.icon')}</label>
              <div class="flex items-center gap-2">
                <div class="w-9 h-9 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 flex items-center justify-center shrink-0 overflow-hidden shadow-inner">
                  <span class="text-xl text-cyan-500 dark:text-cyan-400" style="font-family: 'Material Design Icons'">
                    {getIconCode(icon)}
                  </span>
                </div>
                <select
                  id="node-icon"
                  bind:value={icon}
                  class="flex-1 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                >
                  {#each defaultIconList as ic}
                    <option value={ic.value}>{$_('icons.' + ic.value, { default: ic.name })}</option>
                  {/each}
                </select>
              </div>
            </div>

            <div>
              <label for="node-image" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.image')}</label>
              <div class="flex items-center gap-2">
                <div class="w-9 h-9 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 flex items-center justify-center shrink-0 overflow-hidden shadow-inner p-1">
                  {#if image && getIconImage(image)}
                    <img src={getIconImage(image)} class="max-w-[28px] max-h-[28px] object-contain" alt="" />
                  {:else}
                    <span class="text-[10px] text-slate-400 font-mono">-</span>
                  {/if}
                </div>
                <select
                  id="node-image"
                  bind:value={image}
                  class="flex-1 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                >
                  <option value="">{$_('node.imageNone')}</option>
                  {#each getImageIconList() as imgIc}
                    <option value={imgIc.name || imgIc.value}>{imgIc.name}</option>
                  {/each}
                </select>
              </div>
            </div>

            <div>
              <label for="node-addr-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.addrMode')}</label>
              <select
                id="node-addr-mode"
                bind:value={addrMode}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                <option value="ip">{$_('node.addrModeIp')}</option>
                <option value="mac">{$_('node.addrModeMac')}</option>
                <option value="host">{$_('node.addrModeHost')}</option>
              </select>
            </div>

            <div>
              <label for="node-url" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.url')}</label>
              <input
                id="node-url"
                type="text"
                bind:value={url}
                placeholder="URL"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2">
              <label for="node-descr" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.descr')}</label>
              <input
                id="node-descr"
                type="text"
                bind:value={descr}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2 pt-1">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={autoAck}
                  class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20"
                />
                <span class="text-xs text-slate-700 dark:text-slate-300 font-medium">{$_('node.autoAck')}</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Section 2: SNMP & Authentication -->
        <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
            <Shield class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
            {$_('node.tabSnmp')}
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label for="snmp-ver" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpMode')}</label>
              <select
                id="snmp-ver"
                bind:value={snmpMode}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                {#each snmpModeList as sm}
                  <option value={sm.value}>{sm.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="snmp-community" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.community')}</label>
              <input
                id="snmp-community"
                type="text"
                bind:value={community}
                placeholder="public"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="snmp-port" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpPort')}</label>
              <input
                id="snmp-port"
                type="number"
                min={1}
                max={65535}
                bind:value={snmpPort}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
          </div>

          {#if snmpMode.startsWith("v3")}
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-200 dark:border-slate-800/80">
              <div>
                <label for="snmp-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpUser')}</label>
                <input
                  id="snmp-user"
                  type="text"
                  bind:value={user}
                  placeholder=""
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
              <div>
                <label for="snmp-pass" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpPassword')}</label>
                <input
                  id="snmp-pass"
                  type="password"
                  bind:value={password}
                  placeholder=""
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
            </div>
          {/if}

          <!-- SSH Section -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-200 dark:border-slate-800/80">
            <div>
              <label for="ssh-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.sshUser')}</label>
              <input
                id="ssh-user"
                type="text"
                bind:value={sshUser}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
            <div>
              <label for="ssh-key" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.publicKey')}</label>
              <input
                id="ssh-key"
                type="text"
                bind:value={publicKey}
                placeholder=""
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
          </div>

          <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
            <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
              <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
              gNMI
            </h3>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div>
                <label for="gnmi-port" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.gnmiPort')}</label>
                <input id="gnmi-port" type="text" bind:value={gnmiPort} placeholder="57400" class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none" />
              </div>
              <div>
                <label for="gnmi-encoding" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.gnmiEncoding')}</label>
                <select id="gnmi-encoding" bind:value={gnmiEncoding} class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none">
                  <option value="json_ietf">JSON_IETF</option>
                  <option value="json">JSON</option>
                  <option value="proto">PROTO</option>
                </select>
              </div>
              <div>
                <label for="gnmi-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.gnmiUser')}</label>
                <input id="gnmi-user" type="text" bind:value={gnmiUser} autocomplete="off" class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none" />
              </div>
              <div>
                <label for="gnmi-password" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.gnmiPassword')}</label>
                <input id="gnmi-password" type="password" bind:value={gnmiPassword} autocomplete="new-password" class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer (twnoaa style) -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-800 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
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
