<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import {
    getDiscoverConf,
    saveDiscoverConf,
    startDiscover,
    stopDiscover,
    getDiscoverStats,
    getDiscoverAddressRange,
    getMapConf,
    type DiscoverConfEnt,
    type DiscoverStat,
    type SnmpConfEnt,
    type MapConfEnt,
  } from "../api";
  import { _ } from "svelte-i18n";
  import {
    X,
    Play,
    Square,
    Wand2,
    Plus,
    Trash2,
    ArrowUp,
    ArrowDown,
    Radar,
    Loader2,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    posX = 0,
    posY = 0,
    onComplete = () => {},
  }: {
    show: boolean;
    posX?: number;
    posY?: number;
    onComplete?: () => void;
  } = $props();

  let loading = $state(false);
  let showStats = $state(false);
  let showStop = $state(true);
  let showHelp = $state(false);
  let timer: any = $state(undefined);

  let mapConf = $state<MapConfEnt | SnmpConfEnt | null>(null);
  let conf = $state<DiscoverConfEnt>({
    StartIP: "",
    EndIP: "",
    Timeout: 1,
    Retry: 1,
    X: 0,
    Y: 0,
    AddPolling: true,
    PortScan: true,
    ReCheck: false,
    AddNetwork: true,
    AutoDetect: false,
    AutoDetectAI: false,
    AutoLine: 0,
    AutoLayout: 0,
    SnmpConfigs: [],
  });

  let stats = $state<DiscoverStat>({
    Running: false,
    Total: 0,
    Sent: 0,
    Found: 0,
    Snmp: 0,
    Web: 0,
    Mail: 0,
    SSH: 0,
    File: 0,
    RDP: 0,
    LDAP: 0,
    Wait: 0,
    StartTime: 0,
    Now: 0,
  });

  let newSnmp = $state<SnmpConfEnt>({
    SnmpMode: "v2c",
    Community: "public",
    SnmpUser: "",
    SnmpPassword: "",
  });

  let ipRanges = $state<string[]>([]);
  let selIPRange = $state(0);

  const snmpModes = [
    { value: "v1", label: "SNMPv1" },
    { value: "v2c", label: "SNMPv2c" },
    { value: "v3auth", label: "SNMPv3 Auth (SHA/MD5)" },
    { value: "v3authpriv", label: "SNMPv3 AuthPriv (SHA/AES)" },
  ];

  const updateDiscover = async () => {
    try {
      const s = await getDiscoverStats();
      stats = s;
      if (!s.Running) {
        if (timer) {
          clearTimeout(timer);
          timer = undefined;
        }
        showStop = false;
        onComplete();
        return false;
      }
      timer = setTimeout(() => {
        updateDiscover();
      }, 1500);
      return true;
    } catch {
      return false;
    }
  };

  const onOpen = async () => {
    loading = true;
    try {
      const [mc, dc] = await Promise.all([getMapConf(), getDiscoverConf()]);
      mapConf = mc;
      if (dc) {
        conf = {
          ...dc,
          X: posX,
          Y: posY,
          SnmpConfigs: dc.SnmpConfigs || [],
        };
      }
      const isRunning = await updateDiscover();
      if (isRunning) {
        showStats = true;
        showStop = true;
      } else {
        showStats = false;
      }
    } catch (e) {
      console.error("Discover init error:", e);
    } finally {
      loading = false;
    }
  };

  $effect(() => {
    if (show) {
      onOpen();
    } else {
      if (timer) {
        clearTimeout(timer);
        timer = undefined;
      }
      showStats = false;
    }
  });

  onDestroy(() => {
    if (timer) {
      clearTimeout(timer);
      timer = undefined;
    }
  });

  const handleStart = async () => {
    conf.Timeout = Number(conf.Timeout) || 1;
    conf.Retry = Number(conf.Retry) || 1;
    conf.AutoLine = Number(conf.AutoLine) || 0;
    conf.AutoLayout = Number(conf.AutoLayout) || 0;
    conf.SnmpConfigs = conf.SnmpConfigs || [];

    try {
      const res = await startDiscover(conf);
      if (res && res.ok) {
        showStop = true;
        showStats = true;
        await updateDiscover();
      }
    } catch (e) {
      console.error("Start discover error:", e);
    }
  };

  const handleStop = async () => {
    showStop = false;
    await stopDiscover();
  };

  const handleGetIPRange = async () => {
    if (ipRanges.length < 2) {
      ipRanges = await getDiscoverAddressRange();
    }
    if (ipRanges.length >= 2) {
      conf.StartIP = ipRanges[selIPRange];
      conf.EndIP = ipRanges[selIPRange + 1];
      selIPRange += 2;
      if (selIPRange >= ipRanges.length) {
        selIPRange = 0;
      }
    }
  };

  const addSnmpConfig = () => {
    if (!newSnmp.SnmpMode) return;
    if (newSnmp.SnmpMode === "v1" || newSnmp.SnmpMode === "v2c") {
      if (!newSnmp.Community) return;
    } else {
      if (!newSnmp.SnmpUser) return;
    }
    conf.SnmpConfigs = [
      ...conf.SnmpConfigs,
      {
        SnmpMode: newSnmp.SnmpMode,
        Community: newSnmp.Community,
        SnmpUser: newSnmp.SnmpUser,
        SnmpPassword: newSnmp.SnmpPassword,
      },
    ];
    newSnmp = {
      SnmpMode: "v2c",
      Community: "public",
      SnmpUser: "",
      SnmpPassword: "",
    };
  };

  const removeSnmpConfig = (idx: number) => {
    conf.SnmpConfigs = conf.SnmpConfigs.filter((_, i) => i !== idx);
  };

  const moveSnmpConfig = (idx: number, dir: number) => {
    const targetIdx = idx + dir;
    if (targetIdx < 0 || targetIdx >= conf.SnmpConfigs.length) return;
    const list = [...conf.SnmpConfigs];
    const tmp = list[idx];
    list[idx] = list[targetIdx];
    list[targetIdx] = tmp;
    conf.SnmpConfigs = list;
  };

  const calcPercent = (n: number, d: number) => {
    if (!d || d <= 0) return 0;
    return Math.min(100, Math.max(0, Math.round((n / d) * 100)));
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape" && !stats.Running) show = false;
    }}
  >
    <div
      class="w-full max-w-2xl rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] p-6 shadow-2xl transition-colors text-slate-800 dark:text-slate-200 max-h-[92vh] overflow-y-auto"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800 pb-3 mb-4">
        <div class="flex items-center gap-2.5">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-blue-500/10 border border-blue-500/20 text-blue-600 dark:text-blue-400">
            <Radar class="h-5 w-5 animate-pulse" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-slate-100">
              {$_("Discover.Discover") || "Discover (自動発見)"}
            </h3>
            {#if showStats}
              <p class="text-xs text-slate-500">
                {$_("Discover.Stats") || "進捗状況"} — {Math.max(0, stats.Now - stats.StartTime)}s
              </p>
            {/if}
          </div>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      {#if loading}
        <div class="flex flex-col items-center justify-center py-16 gap-3">
          <Loader2 class="h-8 w-8 animate-spin text-blue-500" />
          <span class="text-sm text-slate-500">読み込み中...</span>
        </div>
      {:else if showStats}
        <!-- Running Progress Statistics -->
        <div class="space-y-4 py-2">
          <!-- Total Sent Progress -->
          <div class="space-y-1.5">
            <div class="flex justify-between text-xs font-semibold">
              <span>{$_("Discover.Total") || "送信進捗"}</span>
              <span class="font-mono">{stats.Sent} / {stats.Total} ({calcPercent(stats.Sent, stats.Total)}%)</span>
            </div>
            <div class="w-full h-3 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
              <div
                class="h-full bg-blue-500 transition-all duration-300 rounded-full"
                style="width: {calcPercent(stats.Sent, stats.Total)}%"
              ></div>
            </div>
          </div>

          <!-- Found Nodes Progress -->
          <div class="space-y-1.5">
            <div class="flex justify-between text-xs font-semibold">
              <span class="text-indigo-600 dark:text-indigo-400">{$_("Discover.Found") || "発見ノード"}</span>
              <span class="font-mono text-indigo-600 dark:text-indigo-400">{stats.Found} / {stats.Total}</span>
            </div>
            <div class="w-full h-3 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
              <div
                class="h-full bg-indigo-500 transition-all duration-300 rounded-full"
                style="width: {calcPercent(stats.Found, stats.Total)}%"
              ></div>
            </div>
          </div>

          <!-- SNMP Detection -->
          <div class="space-y-1.5">
            <div class="flex justify-between text-xs font-semibold">
              <span class="text-amber-600 dark:text-amber-400">SNMP応答</span>
              <span class="font-mono text-amber-600 dark:text-amber-400">{stats.Snmp} / {stats.Found}</span>
            </div>
            <div class="w-full h-2.5 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
              <div
                class="h-full bg-amber-500 transition-all duration-300 rounded-full"
                style="width: {calcPercent(stats.Snmp, stats.Found)}%"
              ></div>
            </div>
          </div>

          <!-- Port scan stats grid -->
          {#if conf.PortScan}
            <div class="grid grid-cols-2 gap-3 pt-2 border-t border-slate-100 dark:border-slate-800/80">
              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">Web (HTTP/S)</span>
                  <span class="font-mono font-bold">{stats.Web}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-emerald-500" style="width: {calcPercent(stats.Web, stats.Found)}%"></div>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">Mail (SMTP/POP/IMAP)</span>
                  <span class="font-mono font-bold">{stats.Mail}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-cyan-500" style="width: {calcPercent(stats.Mail, stats.Found)}%"></div>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">SSH</span>
                  <span class="font-mono font-bold">{stats.SSH}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-purple-500" style="width: {calcPercent(stats.SSH, stats.Found)}%"></div>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">File (CIFS/NFS)</span>
                  <span class="font-mono font-bold">{stats.File}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-yellow-500" style="width: {calcPercent(stats.File, stats.Found)}%"></div>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">RDP / VNC</span>
                  <span class="font-mono font-bold">{stats.RDP}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-rose-500" style="width: {calcPercent(stats.RDP, stats.Found)}%"></div>
                </div>
              </div>

              <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-2.5 bg-slate-50/50 dark:bg-slate-900/50">
                <div class="flex justify-between text-xs mb-1">
                  <span class="font-medium text-slate-600 dark:text-slate-400">LDAP / AD</span>
                  <span class="font-mono font-bold">{stats.LDAP}</span>
                </div>
                <div class="w-full h-1.5 bg-slate-200 dark:bg-slate-800 rounded-full overflow-hidden">
                  <div class="h-full bg-teal-500" style="width: {calcPercent(stats.LDAP, stats.Found)}%"></div>
                </div>
              </div>
            </div>
          {/if}

          <!-- Footer Actions when Running/Finished -->
          <div class="flex items-center justify-end gap-2.5 pt-4 border-t border-slate-100 dark:border-slate-800">
            {#if showStop}
              <button
                type="button"
                onclick={handleStop}
                class="flex items-center gap-2 rounded-xl bg-rose-600 hover:bg-rose-700 text-white font-medium px-4 py-2 text-sm shadow-lg shadow-rose-500/20 transition-all"
              >
                <Square class="h-4 w-4 fill-current" />
                {$_("Discover.Stop") || "Stop"}
              </button>
            {/if}
            <button
              type="button"
              onclick={() => (show = false)}
              class="flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 font-medium px-4 py-2 text-sm transition-all"
            >
              {$_("Discover.Close") || "Close"}
            </button>
          </div>
        </div>
      {:else}
        <!-- Form Settings View -->
        <form class="space-y-4" onsubmit={(e) => { e.preventDefault(); handleStart(); }}>
          <!-- Start IP & End IP -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="start-ip" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.StartIP") || "Start IP"}
              </label>
              <input
                id="start-ip"
                type="text"
                bind:value={conf.StartIP}
                placeholder="192.168.1.0"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
              />
            </div>
            <div>
              <label for="end-ip" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.EndIP") || "End IP"}
              </label>
              <input
                id="end-ip"
                type="text"
                bind:value={conf.EndIP}
                placeholder="192.168.1.254"
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
              />
            </div>
          </div>

          <!-- Timeout & Retry -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="timeout" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.Timeout") || "Timeout (seconds)"}
              </label>
              <input
                id="timeout"
                type="number"
                min="1"
                max="120"
                bind:value={conf.Timeout}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
              />
            </div>
            <div>
              <label for="retry" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.Retry") || "Retry"}
              </label>
              <input
                id="retry"
                type="number"
                min="0"
                max="10"
                bind:value={conf.Retry}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 font-mono"
              />
            </div>
          </div>

          <!-- Checkboxes Grid -->
          <div class="grid grid-cols-2 md:grid-cols-3 gap-3 p-3.5 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50 text-xs">
            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.PortScan}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.PortScan") || "Port scan"}</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.AddPolling}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.AutoAddPolling") || "Automatic Polling Setup"}</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.ReCheck}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.ReCheck") || "Re-check"}</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.AddNetwork}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.AddNetwork") || "Add network"}</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.AutoDetect}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.AutoDetect") || "Auto Detect Node Type"}</span>
            </label>

            <label class="flex items-center gap-2 cursor-pointer font-medium select-none">
              <input
                type="checkbox"
                bind:checked={conf.AutoDetectAI}
                class="rounded text-blue-600 focus:ring-blue-500 border-slate-300 dark:border-slate-700"
              />
              <span>{$_("Discover.AutoDetectAI") || "Use AI Detection"}</span>
            </label>
          </div>

          <!-- Dropdowns (Auto Connect Lines & Auto Layout) -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="auto-line" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.AutoLine") || "Auto Connect Lines"}
              </label>
              <select
                id="auto-line"
                bind:value={conf.AutoLine}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              >
                <option value={0}>{$_("Discover.AutoLineNone") || "Do not connect"}</option>
                <option value={1}>{$_("Discover.AutoLineStrict") || "Strict (LLDP/CDP/STP)"}</option>
                <option value={2}>{$_("Discover.AutoLineSpeculative") || "Speculative (Subnet/FDB)"}</option>
              </select>
            </div>
            <div>
              <label for="auto-layout" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_("Discover.AutoLayout") || "Auto Layout"}
              </label>
              <select
                id="auto-layout"
                bind:value={conf.AutoLayout}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-xs focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
              >
                <option value={0}>{$_("Discover.AutoLayoutNone") || "Sequential Grid (Default)"}</option>
                <option value={1}>{$_("Discover.AutoLayoutHierarchical") || "Hierarchical (Tree)"}</option>
                <option value={2}>{$_("Discover.AutoLayoutCluster") || "Cluster (Hub & Spoke)"}</option>
                <option value={3}>{$_("Discover.AutoLayoutCategorized") || "Categorized (Device Type)"}</option>
              </select>
            </div>
          </div>

          <!-- Additional SNMP Settings Box -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 p-4 space-y-3 bg-slate-50/50 dark:bg-slate-900/40">
            <div class="flex items-center justify-between">
              <span class="text-xs font-bold text-slate-800 dark:text-slate-200">
                {$_("Discover.SnmpConfigs") || "Additional SNMP Settings"}
              </span>
              {#if mapConf}
                <span class="text-xs text-slate-500">
                  Base Map Settings:
                  <span class="font-mono font-semibold text-blue-600 dark:text-blue-400">
                    {mapConf.SnmpMode} ({mapConf.SnmpMode === "v1" || mapConf.SnmpMode === "v2c" ? mapConf.Community : mapConf.SnmpUser})
                  </span>
                </span>
              {/if}
            </div>

            <!-- List of added SNMP settings -->
            {#if conf.SnmpConfigs && conf.SnmpConfigs.length > 0}
              <div class="space-y-1.5 max-h-36 overflow-y-auto pr-1">
                {#each conf.SnmpConfigs as snmp, index}
                  <div class="flex items-center justify-between p-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-xs">
                    <div class="flex items-center space-x-3 truncate">
                      <span class="font-bold text-slate-700 dark:text-slate-300 w-24 truncate">{snmp.SnmpMode}</span>
                      <span class="text-slate-600 dark:text-slate-400 font-mono truncate">
                        {snmp.SnmpMode === "v1" || snmp.SnmpMode === "v2c" ? snmp.Community : snmp.SnmpUser}
                      </span>
                      {#if snmp.SnmpPassword}
                        <span class="text-slate-400 font-mono">••••••••</span>
                      {/if}
                    </div>
                    <div class="flex items-center space-x-1 flex-shrink-0">
                      <button
                        type="button"
                        class="p-1 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 rounded disabled:opacity-30"
                        disabled={index === 0}
                        onclick={() => moveSnmpConfig(index, -1)}
                      >
                        <ArrowUp class="h-3.5 w-3.5" />
                      </button>
                      <button
                        type="button"
                        class="p-1 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-500 rounded disabled:opacity-30"
                        disabled={index === conf.SnmpConfigs.length - 1}
                        onclick={() => moveSnmpConfig(index, 1)}
                      >
                        <ArrowDown class="h-3.5 w-3.5" />
                      </button>
                      <button
                        type="button"
                        class="p-1 hover:bg-rose-100 dark:hover:bg-rose-900/30 text-rose-500 rounded"
                        onclick={() => removeSnmpConfig(index)}
                      >
                        <Trash2 class="h-3.5 w-3.5" />
                      </button>
                    </div>
                  </div>
                {/each}
              </div>
            {:else}
              <div class="text-center py-2 text-xs italic text-slate-400">
                {$_("Discover.NoSnmpConfigs") || "No additional SNMP settings (Map settings will be used)"}
              </div>
            {/if}

            <!-- Add SNMP Configuration Row -->
            <div class="grid grid-cols-1 md:grid-cols-12 gap-2 pt-2 border-t border-slate-200/80 dark:border-slate-800">
              <div class="md:col-span-3">
                <select
                  bind:value={newSnmp.SnmpMode}
                  class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs focus:border-blue-500 focus:outline-none"
                >
                  {#each snmpModes as m}
                    <option value={m.value}>{m.label}</option>
                  {/each}
                </select>
              </div>

              {#if newSnmp.SnmpMode === "v1" || newSnmp.SnmpMode === "v2c"}
                <div class="md:col-span-6">
                  <input
                    type="text"
                    bind:value={newSnmp.Community}
                    placeholder="Community String (e.g. public)"
                    class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs font-mono focus:border-blue-500 focus:outline-none"
                  />
                </div>
              {:else}
                <div class="md:col-span-3">
                  <input
                    type="text"
                    bind:value={newSnmp.SnmpUser}
                    placeholder="User Name"
                    class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs font-mono focus:border-blue-500 focus:outline-none"
                  />
                </div>
                <div class="md:col-span-3">
                  <input
                    type="password"
                    bind:value={newSnmp.SnmpPassword}
                    placeholder="Password / Auth Key"
                    class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 px-2.5 py-1.5 text-xs font-mono focus:border-blue-500 focus:outline-none"
                  />
                </div>
              {/if}

              <div class="md:col-span-3">
                <button
                  type="button"
                  onclick={addSnmpConfig}
                  class="w-full flex items-center justify-center gap-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium px-3 py-1.5 text-xs shadow-sm transition-all"
                >
                  <Plus class="h-3.5 w-3.5" />
                  <span>{$_("Discover.AddSnmpConfig") || "Add SNMP Setting"}</span>
                </button>
              </div>
            </div>
          </div>

          <!-- Bottom Actions -->
          <div class="flex flex-wrap items-center justify-between gap-3 pt-3 border-t border-slate-100 dark:border-slate-800">
            <div class="flex items-center gap-2">
              <button
                type="button"
                onclick={handleGetIPRange}
                class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-800 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 text-rose-700 dark:text-rose-300 font-medium px-3.5 py-2 text-xs shadow-sm transition-all"
              >
                <Wand2 class="h-4 w-4" />
                <span>{$_("Discover.AutoIPRange") || "Auto IP range"}</span>
              </button>
            </div>

            <div class="flex items-center gap-2.5">
              <button
                type="submit"
                class="flex items-center gap-2 rounded-xl bg-blue-600 hover:bg-blue-700 text-white font-medium px-5 py-2 text-sm shadow-lg shadow-blue-500/25 transition-all"
              >
                <Play class="h-4 w-4 fill-current" />
                <span>{$_("Discover.Start") || "Start"}</span>
              </button>
              <button
                type="button"
                onclick={() => (show = false)}
                class="rounded-xl border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-300 font-medium px-4 py-2 text-sm transition-all"
              >
                {$_("Discover.Close") || "Close"}
              </button>
            </div>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}
