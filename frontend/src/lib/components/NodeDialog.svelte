<script lang="ts">
  import { untrack } from "svelte";
  import { defaultIconList, getImageIconList, addrModeList, snmpModeList, getIconImage, getIconCode } from "../common";
  import { saveNode, type NodeEnt } from "../api";
  import { checkNodePos } from "../map/map";
  import { X, Save, Cpu, Shield, Key, Network, Globe, Plus, Trash2, ExternalLink, Eye, EyeOff, Terminal } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let { show = $bindable(false), node = $bindable<NodeEnt | null>(null), onSave = () => {} } = $props<{
    show: boolean;
    node: NodeEnt | null;
    onSave?: (saved: NodeEnt) => void;
  }>();

  // Basic Info
  let name = $state("");
  let ip = $state("");
  let mac = $state("");
  let descr = $state("");
  let icon = $state("desktop");
  let image = $state("");
  let addrMode = $state("ip");
  let autoAck = $state(false);

  // URLs (Multiple list editor)
  let urlList = $state<string[]>([""]);

  // Credentials (Common user & password)
  let user = $state("");
  let password = $state("");
  let showPassword = $state(false);

  // SNMP
  let enableSnmp = $state(false);
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpPort = $state(161);

  // SSH
  let enableSsh = $state(false);
  let sshUser = $state("");
  let publicKey = $state("");

  // gNMI
  let enableGnmi = $state(false);
  let gnmiPort = $state("57400");
  let gnmiEncoding = $state("json_ietf");
  let gnmiUser = $state("");
  let gnmiPassword = $state("");

  let saveError = $state("");

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        showPassword = false;

        if (node && node.id) {
          name = node.name || (node as any).Name || "";
          ip = node.ip || (node as any).IP || "";
          mac = node.mac || (node as any).MAC || "";
          descr = node.descr || (node as any).Descr || "";
          icon = node.icon || (node as any).Icon || "desktop";
          image = node.image || (node as any).Image || "";
          addrMode = (node as any).addr_mode || (node as any).AddrMode || "ip";
          autoAck = (node as any).auto_ack ?? (node as any).AutoAck ?? false;

          // Parse URLs
          const rawUrl = (node as any).url || (node as any).URL || "";
          const parsed = rawUrl
            ? rawUrl.split(/[,;\n]/).map((u: string) => u.trim()).filter((u: string) => u.length > 0)
            : [];
          urlList = parsed.length > 0 ? parsed : [""];

          // Node Credentials
          user = (node as any).user || (node as any).User || "";
          password = (node as any).password || (node as any).Password || "";

          // SNMP
          const rawSnmpMode = (node as any).snmp_mode || (node as any).SnmpMode;
          const rawCommunity = (node as any).community || (node as any).Community;
          if (rawSnmpMode && rawSnmpMode !== "none" && rawSnmpMode !== "") {
            enableSnmp = true;
            snmpMode = rawSnmpMode;
          } else if (rawCommunity) {
            enableSnmp = true;
            snmpMode = "v2c";
          } else {
            enableSnmp = false;
            snmpMode = "v2c";
          }
          community = rawCommunity || "public";
          snmpPort = Number((node as any).snmp_port || (node as any).SnmpPort || 161);

          // SSH
          sshUser = (node as any).ssh_user || (node as any).SSHUser || "";
          publicKey = (node as any).public_key || (node as any).PublicKey || "";
          enableSsh = !!(sshUser || publicKey);

          // gNMI
          gnmiPort = (node as any).gnmi_port || (node as any).GNMIPort || "57400";
          gnmiEncoding = (node as any).gnmi_encoding || (node as any).GNMIEncoding || "json_ietf";
          gnmiUser = (node as any).gnmi_user || (node as any).GNMIUser || "";
          gnmiPassword = (node as any).gnmi_password || (node as any).GNMIPassword || "";
          enableGnmi = !!(gnmiUser || gnmiPassword || ((node as any).gnmi_port && (node as any).gnmi_port !== "57400"));
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
          urlList = [""];

          user = "";
          password = "";

          enableSnmp = false;
          snmpMode = "v2c";
          community = "public";
          snmpPort = 161;

          enableSsh = false;
          sshUser = "";
          publicKey = "";

          enableGnmi = false;
          gnmiPort = "57400";
          gnmiEncoding = "json_ietf";
          gnmiUser = "";
          gnmiPassword = "";
        }
      });
    }
  });

  const addUrl = () => {
    urlList = [...urlList, ""];
  };

  const removeUrl = (index: number) => {
    urlList = urlList.filter((_, i) => i !== index);
    if (urlList.length === 0) {
      urlList = [""];
    }
  };

  const openUrl = (targetUrl: string) => {
    if (!targetUrl) return;
    const trimmed = targetUrl.trim();
    if (!trimmed) return;
    const fullUrl = trimmed.startsWith("http://") || trimmed.startsWith("https://")
      ? trimmed
      : "http://" + trimmed;
    window.open(fullUrl, "_blank", "noopener,noreferrer");
  };

  const handleSave = async () => {
    if (!name || (!ip && addrMode !== "host")) {
      saveError = $_('node.saveError');
      return;
    }

    const finalUrl = urlList.map((u) => u.trim()).filter((u) => u.length > 0).join(",");

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
      url: finalUrl,
      snmp_mode: enableSnmp ? snmpMode : "none",
      community: enableSnmp ? community : "",
      snmp_port: enableSnmp ? (Number(snmpPort) || 161) : 0,
      user,
      password,
      ssh_user: enableSsh ? sshUser : "",
      public_key: enableSsh ? publicKey : "",
      gnmi_port: enableGnmi ? gnmiPort : "",
      gnmi_encoding: enableGnmi ? gnmiEncoding : "",
      gnmi_user: enableGnmi ? gnmiUser : "",
      gnmi_password: enableGnmi ? gnmiPassword : "",
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
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-auto max-h-[92vh] w-full max-w-4xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header -->
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
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-800 dark:hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body (2-Column Grid on large screens) -->
      <div class="flex-1 overflow-y-auto p-6 bg-slate-50/60 dark:bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm mb-5">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
          <!-- LEFT COLUMN: Basic Information & Web URLs -->
          <div class="space-y-5">
            <!-- Section 1: Basic Information -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
              <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                {$_('node.tabBasic')}
              </h3>

              <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
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
                      class="flex-1 min-w-0 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
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
                      class="flex-1 min-w-0 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    >
                      <option value="">{$_('node.imageNone')}</option>
                      {#each getImageIconList() as imgIc}
                        <option value={imgIc.name || imgIc.value}>{imgIc.name}</option>
                      {/each}
                    </select>
                  </div>
                </div>

                <div class="sm:col-span-2">
                  <label for="node-descr" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.descr')}</label>
                  <input
                    id="node-descr"
                    type="text"
                    bind:value={descr}
                    placeholder=""
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                  />
                </div>

                <div class="sm:col-span-2 pt-1">
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

            <!-- Section 2: Web URLs (Multiple list editor) -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <Globe class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('node.urlList')}
                </h3>
                <button
                  type="button"
                  onclick={addUrl}
                  class="flex items-center gap-1 px-2.5 py-1 text-[11px] font-semibold text-cyan-600 dark:text-cyan-400 hover:text-cyan-500 bg-cyan-500/10 hover:bg-cyan-500/20 rounded-lg transition-colors cursor-pointer"
                >
                  <Plus class="w-3.5 h-3.5" />
                  {$_('node.addUrl')}
                </button>
              </div>

              <div class="space-y-2">
                {#each urlList as u, idx}
                  <div class="flex items-center gap-2">
                    <input
                      type="text"
                      bind:value={urlList[idx]}
                      placeholder="http://example.com"
                      class="flex-1 min-w-0 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                    <button
                      type="button"
                      disabled={!u.trim()}
                      onclick={() => openUrl(u)}
                      title={$_('node.openUrl')}
                      class="p-2 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-500 hover:text-cyan-500 hover:border-cyan-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors shrink-0"
                    >
                      <ExternalLink class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => removeUrl(idx)}
                      title={$_('node.removeUrl')}
                      class="p-2 rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-400 hover:text-rose-500 hover:border-rose-500/40 transition-colors shrink-0"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                {/each}
              </div>
            </div>
          </div>

          <!-- RIGHT COLUMN: Credentials & Protocols (SNMP, SSH, gNMI) -->
          <div class="space-y-5">
            <!-- Section 3: Shared Node Credentials -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
              <div class="border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <Key class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('node.tabCredentials')}
                </h3>
                <p class="text-[11px] text-slate-400 mt-1">{$_('node.credentialsDesc')}</p>
              </div>

              <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
                <div>
                  <label for="node-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                    {$_('node.snmpUser')}
                  </label>
                  <input
                    id="node-user"
                    type="text"
                    bind:value={user}
                    autocomplete="username"
                    placeholder=""
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                  />
                </div>

                <div>
                  <label for="node-pass" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                    {$_('node.snmpPassword')}
                  </label>
                  <div class="relative">
                    <input
                      id="node-pass"
                      type={showPassword ? "text" : "password"}
                      bind:value={password}
                      autocomplete="current-password"
                      placeholder=""
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 pl-3.5 pr-10 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                    <button
                      type="button"
                      onclick={() => (showPassword = !showPassword)}
                      class="absolute right-2.5 top-1/2 -translate-y-1/2 p-1 text-slate-400 hover:text-slate-200 cursor-pointer"
                    >
                      {#if showPassword}
                        <EyeOff class="w-3.5 h-3.5" />
                      {:else}
                        <Eye class="w-3.5 h-3.5" />
                      {/if}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            <!-- Section 4: Protocols (SNMP, SSH, gNMI) with Toggle Switches -->
            <!-- SNMP Card -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <Shield class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  SNMP
                </h3>
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-medium {enableSnmp ? 'text-cyan-500 dark:text-cyan-400' : 'text-slate-400'}">
                    {enableSnmp ? $_('common.enabled') : $_('common.disabled')}
                  </span>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={enableSnmp}
                    onclick={() => (enableSnmp = !enableSnmp)}
                    class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none {enableSnmp ? 'bg-cyan-500' : 'bg-slate-300 dark:bg-slate-700'}"
                  >
                    <span
                      class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out {enableSnmp ? 'translate-x-4' : 'translate-x-0'}"
                    />
                  </button>
                </div>
              </div>

              {#if enableSnmp}
                <div class="grid grid-cols-1 sm:grid-cols-3 gap-3.5 pt-1">
                  <div>
                    <label for="snmp-ver" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.snmpMode')}
                    </label>
                    <select
                      id="snmp-ver"
                      bind:value={snmpMode}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    >
                      {#each snmpModeList as sm}
                        <option value={sm.value}>{sm.name}</option>
                      {/each}
                    </select>
                  </div>

                  <div>
                    <label for="snmp-community" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.community')}
                    </label>
                    <input
                      id="snmp-community"
                      type="text"
                      bind:value={community}
                      placeholder="public"
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                  </div>

                  <div>
                    <label for="snmp-port" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.snmpPort')}
                    </label>
                    <input
                      id="snmp-port"
                      type="number"
                      min={1}
                      max={65535}
                      bind:value={snmpPort}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                  </div>
                </div>
              {/if}
            </div>

            <!-- SSH Card -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <Terminal class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  SSH
                </h3>
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-medium {enableSsh ? 'text-cyan-500 dark:text-cyan-400' : 'text-slate-400'}">
                    {enableSsh ? $_('common.enabled') : $_('common.disabled')}
                  </span>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={enableSsh}
                    onclick={() => (enableSsh = !enableSsh)}
                    class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none {enableSsh ? 'bg-cyan-500' : 'bg-slate-300 dark:bg-slate-700'}"
                  >
                    <span
                      class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out {enableSsh ? 'translate-x-4' : 'translate-x-0'}"
                    />
                  </button>
                </div>
              </div>

              {#if enableSsh}
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
                  <div>
                    <label for="ssh-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.sshUser')}
                    </label>
                    <input
                      id="ssh-user"
                      type="text"
                      bind:value={sshUser}
                      placeholder=""
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                  </div>
                  <div>
                    <label for="ssh-key" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.publicKey')}
                    </label>
                    <input
                      id="ssh-key"
                      type="text"
                      bind:value={publicKey}
                      placeholder=""
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                    />
                  </div>
                </div>
              {/if}
            </div>

            <!-- gNMI Card -->
            <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
              <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                  <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  gNMI
                </h3>
                <div class="flex items-center gap-2">
                  <span class="text-[11px] font-medium {enableGnmi ? 'text-cyan-500 dark:text-cyan-400' : 'text-slate-400'}">
                    {enableGnmi ? $_('common.enabled') : $_('common.disabled')}
                  </span>
                  <button
                    type="button"
                    role="switch"
                    aria-checked={enableGnmi}
                    onclick={() => (enableGnmi = !enableGnmi)}
                    class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none {enableGnmi ? 'bg-cyan-500' : 'bg-slate-300 dark:bg-slate-700'}"
                  >
                    <span
                      class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-md ring-0 transition duration-200 ease-in-out {enableGnmi ? 'translate-x-4' : 'translate-x-0'}"
                    />
                  </button>
                </div>
              </div>

              {#if enableGnmi}
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
                  <div>
                    <label for="gnmi-port" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.gnmiPort')}
                    </label>
                    <input
                      id="gnmi-port"
                      type="text"
                      bind:value={gnmiPort}
                      placeholder="57400"
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="gnmi-encoding" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.gnmiEncoding')}
                    </label>
                    <select
                      id="gnmi-encoding"
                      bind:value={gnmiEncoding}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="json_ietf">JSON_IETF</option>
                      <option value="json">JSON</option>
                      <option value="proto">PROTO</option>
                    </select>
                  </div>
                  <div>
                    <label for="gnmi-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.gnmiUser')}
                    </label>
                    <input
                      id="gnmi-user"
                      type="text"
                      bind:value={gnmiUser}
                      autocomplete="off"
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="gnmi-password" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('node.gnmiPassword')}
                    </label>
                    <input
                      id="gnmi-password"
                      type="password"
                      bind:value={gnmiPassword}
                      autocomplete="new-password"
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
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
