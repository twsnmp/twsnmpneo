<script lang="ts">
  import {
    fetchMapConf,
    saveMapConf,
    fetchNotifyConf,
    saveNotifyConf,
    fetchBackImage,
    saveBackImage,
    fetchNotifyOAuth2Status,
    type UserEnt,
    type ModelInfo,
    type AIHardwareStatus,
    fetchLocalModels,
    fetchAIHardwareStatus,
  } from "../api";
  import ImportMapModal from "./ImportMapModal.svelte";
  import ConfigTabMap from "./config/ConfigTabMap.svelte";
  import ConfigTabPolling from "./config/ConfigTabPolling.svelte";
  import ConfigTabReceivers from "./config/ConfigTabReceivers.svelte";
  import ConfigTabNotify from "./config/ConfigTabNotify.svelte";
  import ConfigTabAi from "./config/ConfigTabAi.svelte";
  import ConfigTabDatabase from "./config/ConfigTabDatabase.svelte";
  import ConfigTabMib from "./config/ConfigTabMib.svelte";
  import ConfigTabIcons from "./config/ConfigTabIcons.svelte";
  import ConfigTabUsers from "./config/ConfigTabUsers.svelte";
  import {
    X,
    Save,
    Sliders,
    Bell,
    Brain,
    Database,
    CheckCircle2,
    Radio,
    Sparkles,
    Network,
    FolderTree,
    Users,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    show = $bindable(false),
    onSaved,
    currentUser = null,
  }: {
    show: boolean;
    onSaved?: () => void;
    currentUser?: UserEnt | null;
  } = $props();

  let activeTab = $state<"map" | "polling" | "receivers" | "notify" | "ai" | "database" | "mib" | "icons" | "users">("map");

  let saveMsg = $state("");
  let saveError = $state("");

  // Map & Background Image configuration
  let mapName = $state("TWSNMP NEO");
  let mapSize = $state(0);
  let iconSize = $state(3);
  let backImageX = $state(0);
  let backImageY = $state(0);
  let backImageW = $state(800);
  let backImageH = $state(600);
  let backImagePath = $state("");
  let imgNaturalW = $state(0);
  let imgNaturalH = $state(0);
  let lockRatio = $state(true);
  let showImportModal = $state(false);

  // Polling & SNMP configuration
  let pollInt = $state(60);
  let timeout = $state(1);
  let retry = $state(1);
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpUser = $state("");
  let snmpPassword = $state("");

  // Receivers & Daemons
  let enableSyslogd = $state(true);
  let enableTrapd = $state(true);
  let enableNetflowd = $state(false);
  let enableSFlowd = $state(false);
  let enableArpWatch = $state(false);
  let enableSshd = $state(false);
  let enableTcpd = $state(false);
  let enableOTel = $state(false);
  let enableMqtt = $state(false);
  let arpWatchRange = $state("");
  let arpTimeout = $state(60);

  // Notifications
  let notifyProvider = $state("smtp");
  let notifyLevel = $state("warn");
  let notifyInterval = $state(60);
  let notifySubject = $state("TWSNMP");
  let mailServer = $state("");
  let mailFrom = $state("");
  let mailTo = $state("");
  let mailUser = $state("");
  let mailPassword = $state("");
  let insecureSkipVerify = $state(false);
  let notifyReport = $state(false);
  let notifyLLMSummary = $state(false);
  let notifyNotifyRepair = $state(false);
  let notifyCheckDependency = $state(false);
  let notifyExecCmd = $state("");
  let notifyWebHookNotify = $state("");
  let notifyWebHookReport = $state("");
  let notifyClientID = $state("");
  let notifyClientSecret = $state("");
  let notifyMSTenant = $state("");
  let notifyOAuth2HasToken = $state(false);

  // AI / LLM configuration
  let llmProvider = $state("tensai");
  let llmBaseUrl = $state("http://localhost:11434");
  let llmModel = $state("qwen2.5-0.5b");
  let llmApiKey = $state("");
  let enableMCP = $state(true);
  let mcpMode = $state("noauth");
  let mcpFrom = $state("");
  let localAIModels = $state<ModelInfo[]>([]);
  let aiHardware = $state<AIHardwareStatus | null>(null);

  // Database configuration
  let geoIPInfo = $state("");
  let logDays = $state(14);
  let reportDays = $state(30);
  let reportLimit = $state(10000);
  let scoreThreshold = $state(35.0);
  let fumbleThreshold = $state(10);
  let logFormat = $state("parquet");

  // Tab Component refs for imperative triggers if needed
  let mibTabRef: { loadMIBModules: () => Promise<void> } | undefined = $state();
  let iconsTabRef: { loadIcons: () => Promise<void> } | undefined = $state();
  let usersTabRef: { loadUsers: () => Promise<void> } | undefined = $state();

  function updateImageNaturalSize(src: string) {
    if (!src) return;
    const img = new window.Image();
    img.onload = () => {
      imgNaturalW = img.naturalWidth;
      imgNaturalH = img.naturalHeight;
      if (!backImageW || !backImageH || backImageW <= 0 || backImageH <= 0) {
        backImageW = img.naturalWidth;
        backImageH = img.naturalHeight;
      }
    };
    img.src = src;
  }

  async function loadLocalAIInfo() {
    try {
      localAIModels = (await fetchLocalModels()) || [];
      aiHardware = await fetchAIHardwareStatus();
      if (llmProvider === "tensai") {
        if (localAIModels.length > 0 && (!llmModel || !localAIModels.some(m => m.name === llmModel))) {
          llmModel = localAIModels[0].name;
        }
      }
    } catch {
      // Ignore
    }
  }

  async function loadConfig() {
    try {
      const conf = await fetchMapConf();
      if (conf) {
        mapName = conf.MapName ?? conf.map_name ?? "TWSNMP NEO";
        mapSize = conf.MapSize ?? conf.map_size ?? 0;
        iconSize = conf.IconSize ?? conf.icon_size ?? 3;
        pollInt = conf.PollInt ?? conf.poll_int ?? 60;
        timeout = conf.Timeout ?? conf.timeout ?? 1;
        retry = conf.Retry ?? conf.retry ?? 1;
        logDays = conf.LogDays ?? (conf as any).log_days ?? 14;
        reportDays = conf.ReportDays ?? (conf as any).report_days ?? 30;
        reportLimit = conf.ReportLimit ?? (conf as any).report_limit ?? 10000;
        scoreThreshold = conf.ScoreThreshold ?? (conf as any).score_threshold ?? 35.0;
        fumbleThreshold = conf.FumbleThreshold ?? (conf as any).fumble_threshold ?? 10;
        snmpMode = conf.SnmpMode ?? (conf as any).snmp_mode ?? "v2c";
        community = conf.Community ?? conf.community ?? "public";
        snmpUser = conf.SnmpUser ?? conf.snmp_user ?? "";
        snmpPassword = conf.SnmpPassword ?? conf.snmp_password ?? "";

        enableSyslogd = conf.EnableSyslogd ?? conf.enable_syslogd ?? true;
        enableTrapd = conf.EnableTrapd ?? conf.enable_trapd ?? true;
        enableNetflowd = conf.EnableNetflowd ?? conf.enable_netflowd ?? false;
        enableSFlowd = conf.EnableSFlowd ?? conf.enable_sflowd ?? false;
        enableArpWatch = conf.EnableArpWatch ?? conf.enable_arp_watch ?? false;
        enableSshd = conf.EnableSshd ?? conf.enable_sshd ?? false;
        enableTcpd = conf.EnableTcpd ?? conf.enable_tcpd ?? false;
        enableOTel = conf.EnableOTel ?? conf.enable_otel ?? false;
        enableMqtt = conf.EnableMqtt ?? conf.enable_mqtt ?? false;
        arpWatchRange = conf.ArpWatchRange ?? conf.arp_watch_range ?? "";
        arpTimeout = conf.ArpTimeout ?? conf.arp_timeout ?? 60;

        llmProvider = conf.LLMProvider ?? conf.llm_provider ?? "tensai";
        llmBaseUrl = conf.LLMBaseURL ?? conf.llm_base_url ?? "http://localhost:11434";
        llmModel = conf.LLMModel ?? conf.llm_model ?? "gemini-1.5-flash";
        llmApiKey = conf.LLMAPIKey ?? conf.llm_api_key ?? "";
        enableMCP = conf.EnableMCP ?? (conf as any).enable_mcp ?? true;
        mcpMode = conf.MCPMode ?? (conf as any).mcp_mode ?? "noauth";
        mcpFrom = conf.MCPFrom ?? (conf as any).mcp_from ?? "";
        logFormat = conf.LogFormat ?? conf.log_format ?? "parquet";
        geoIPInfo = conf.GeoIPInfo ?? conf.geo_ip_info ?? "";
        await loadLocalAIInfo();
      }

      const nConf = await fetchNotifyConf().catch(() => null);
      if (nConf) {
        notifyProvider = nConf.Provider || "smtp";
        notifyLevel = nConf.Level || "warn";
        notifyInterval = nConf.Interval ?? 60;
        notifySubject = nConf.Subject || "TWSNMP";
        mailServer = nConf.MailServer || "";
        mailFrom = nConf.MailFrom || "";
        mailTo = nConf.MailTo || "";
        mailUser = nConf.User || "";
        mailPassword = nConf.Password || "";
        insecureSkipVerify = nConf.InsecureSkipVerify || false;
        notifyReport = nConf.Report || false;
        notifyLLMSummary = nConf.LLMSummary || false;
        notifyNotifyRepair = nConf.NotifyRepair || false;
        notifyCheckDependency = nConf.CheckDependency || false;
        notifyExecCmd = nConf.ExecCmd || "";
        notifyWebHookNotify = nConf.WebHookNotify || "";
        notifyWebHookReport = nConf.WebHookReport || "";
        notifyClientID = nConf.ClientID || "";
        notifyClientSecret = nConf.ClientSecret || "";
        notifyMSTenant = nConf.MSTenant || "";
      }

      const oauth2Status = await fetchNotifyOAuth2Status().catch(() => null);
      if (oauth2Status) {
        notifyOAuth2HasToken = Boolean(oauth2Status.hasToken);
      }

      const bi = await fetchBackImage().catch(() => null);
      if (bi) {
        backImageX = bi.X ?? 0;
        backImageY = bi.Y ?? 0;
        backImageW = (bi.Width && bi.Width > 0) ? bi.Width : 0;
        backImageH = (bi.Height && bi.Height > 0) ? bi.Height : 0;
        backImagePath = bi.Path ?? "";
        if (backImagePath) {
          updateImageNaturalSize(backImagePath);
        }
      }
    } catch (e) {
      console.error("Failed to load configuration:", e);
    }
  }

  $effect(() => {
    if (show) {
      loadConfig();
      saveMsg = "";
      saveError = "";
    }
  });

  const handleSave = async () => {
    saveMsg = "";
    saveError = "";
    try {
      // 1. Save Map & Core Conf
      await saveMapConf({
        MapName: mapName,
        MapSize: Number(mapSize),
        IconSize: Number(iconSize),
        PollInt: Number(pollInt),
        Timeout: Number(timeout),
        Retry: Number(retry),
        LogDays: Number(logDays),
        ReportDays: Number(reportDays),
        ReportLimit: Number(reportLimit),
        ScoreThreshold: Number(scoreThreshold),
        FumbleThreshold: Number(fumbleThreshold),
        SnmpMode: snmpMode,
        Community: community,
        SnmpUser: snmpUser,
        SnmpPassword: snmpPassword,
        EnableSyslogd: Boolean(enableSyslogd),
        EnableTrapd: Boolean(enableTrapd),
        EnableNetflowd: Boolean(enableNetflowd),
        EnableSFlowd: Boolean(enableSFlowd),
        EnableArpWatch: Boolean(enableArpWatch),
        EnableSshd: Boolean(enableSshd),
        EnableTcpd: Boolean(enableTcpd),
        EnableOTel: Boolean(enableOTel),
        EnableMqtt: Boolean(enableMqtt),
        ArpWatchRange: arpWatchRange,
        ArpTimeout: Number(arpTimeout),
        LLMProvider: llmProvider,
        LLMBaseURL: llmBaseUrl,
        LLMModel: llmModel,
        LLMAPIKey: llmApiKey,
        EnableMCP: Boolean(enableMCP),
        MCPMode: mcpMode,
        MCPFrom: mcpFrom,
        LogFormat: logFormat,
      });

      // 2. Save Notify Conf
      await saveNotifyConf({
        Provider: notifyProvider,
        Level: notifyLevel,
        Interval: Number(notifyInterval),
        Subject: notifySubject,
        MailServer: mailServer,
        MailFrom: mailFrom,
        MailTo: mailTo,
        User: mailUser,
        Password: mailPassword,
        InsecureSkipVerify: Boolean(insecureSkipVerify),
        Report: Boolean(notifyReport),
        LLMSummary: Boolean(notifyLLMSummary),
        NotifyRepair: Boolean(notifyNotifyRepair),
        CheckDependency: Boolean(notifyCheckDependency),
        ExecCmd: notifyExecCmd,
        WebHookNotify: notifyWebHookNotify,
        WebHookReport: notifyWebHookReport,
        ClientID: notifyClientID,
        ClientSecret: notifyClientSecret,
        MSTenant: notifyMSTenant,
      });

      // 3. Save BackImage Conf
      await saveBackImage({
        X: Number(backImageX) || 0,
        Y: Number(backImageY) || 0,
        Width: Number(backImageW) || 800,
        Height: Number(backImageH) || 600,
        Path: backImagePath,
      });

      saveMsg = $_('config.saveSuccess');
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
      }
      onSaved?.();
      setTimeout(() => {
        if (saveMsg) show = false;
      }, 1200);
    } catch (e: any) {
      saveError = $_('config.saveError') + ": " + (e.message || e);
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
    <div class="flex h-[88vh] w-full max-w-7xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Sliders class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{$_('config.title')}</h2>
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

      <!-- Main Layout -->
      <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar Navigation -->
        <div class="w-52 border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950/60 p-3 space-y-1.5 shrink-0">
          <button
            type="button"
            onclick={() => (activeTab = "map")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'map' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Sliders class="h-4 w-4" />
            {$_('config.tabMap')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "polling")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'polling' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Network class="h-4 w-4" />
            {$_('config.tabPolling')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "receivers")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'receivers' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Radio class="h-4 w-4" />
            {$_('config.tabReceivers')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "notify")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'notify' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Bell class="h-4 w-4" />
            {$_('config.tabNotify')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "ai")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'ai' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Brain class="h-4 w-4" />
            {$_('config.tabAi')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "database")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'database' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Database class="h-4 w-4" />
            {$_('config.tabDatabase')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "mib"; mibTabRef?.loadMIBModules(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'mib' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <FolderTree class="h-4 w-4" />
            {$_('config.tabMib')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "icons"; iconsTabRef?.loadIcons(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'icons' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Sparkles class="h-4 w-4" />
            {$_('config.tabIcons')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "users"; usersTabRef?.loadUsers(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'users' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Users class="h-4 w-4" />
            {$_('config.tabUsers') || 'ユーザー管理'}
          </button>
        </div>

        <!-- Form Panels -->
        <div class="flex-1 overflow-y-auto p-6 text-xs bg-slate-50/50 dark:bg-slate-900/40">
          {#if saveMsg}
            <div class="mb-5 flex items-center gap-2 rounded-xl border border-emerald-800/40 bg-emerald-950/40 p-3.5 text-xs font-medium text-emerald-300 shadow-sm">
              <CheckCircle2 class="h-4 w-4 text-emerald-400 shrink-0" />
              <span>{saveMsg}</span>
            </div>
          {/if}
          {#if saveError}
            <div class="mb-5 flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
              <X class="h-4 w-4 text-rose-400 shrink-0" />
              <span>{saveError}</span>
            </div>
          {/if}

          {#if activeTab === "map"}
            <ConfigTabMap
              bind:mapName
              bind:mapSize
              bind:iconSize
              bind:backImageX
              bind:backImageY
              bind:backImageW
              bind:backImageH
              bind:backImagePath
              bind:imgNaturalW
              bind:imgNaturalH
              bind:lockRatio
              onOpenImportMap={() => (showImportModal = true)}
              onError={(err) => (saveError = err)}
            />
          {:else if activeTab === "polling"}
            <ConfigTabPolling
              bind:pollInt
              bind:timeout
              bind:retry
              bind:snmpMode
              bind:community
              bind:snmpUser
              bind:snmpPassword
            />
          {:else if activeTab === "receivers"}
            <ConfigTabReceivers
              bind:enableSyslogd
              bind:enableTrapd
              bind:enableNetflowd
              bind:enableSFlowd
              bind:enableArpWatch
              bind:enableSshd
              bind:enableTcpd
              bind:enableOTel
              bind:enableMqtt
              bind:arpWatchRange
              bind:arpTimeout
            />
          {:else if activeTab === "notify"}
            <ConfigTabNotify
              bind:notifyProvider
              bind:notifyLevel
              bind:notifyInterval
              bind:notifySubject
              bind:mailServer
              bind:mailFrom
              bind:mailTo
              bind:mailUser
              bind:mailPassword
              bind:insecureSkipVerify
              bind:notifyReport
              bind:notifyLLMSummary
              bind:notifyNotifyRepair
              bind:notifyCheckDependency
              bind:notifyExecCmd
              bind:notifyWebHookNotify
              bind:notifyWebHookReport
              bind:notifyClientID
              bind:notifyClientSecret
              bind:notifyMSTenant
              bind:notifyOAuth2HasToken
            />
          {:else if activeTab === "ai"}
            <ConfigTabAi
              bind:llmProvider
              bind:llmBaseUrl
              bind:llmModel
              bind:llmApiKey
              bind:enableMCP
              bind:mcpMode
              bind:mcpFrom
              bind:localAIModels
              bind:aiHardware
              onModelsChanged={loadLocalAIInfo}
            />
          {:else if activeTab === "database"}
            <ConfigTabDatabase
              bind:geoIPInfo
              bind:logDays
              bind:reportDays
              bind:reportLimit
              bind:scoreThreshold
              bind:fumbleThreshold
              bind:logFormat
              onRefreshConfig={loadConfig}
              onSuccess={(msg) => (saveMsg = msg)}
              onError={(err) => (saveError = err)}
            />
          {:else if activeTab === "mib"}
            <ConfigTabMib
              bind:this={mibTabRef}
              onSuccess={(msg) => (saveMsg = msg)}
              onError={(err) => (saveError = err)}
            />
          {:else if activeTab === "icons"}
            <ConfigTabIcons
              bind:this={iconsTabRef}
              onSuccess={(msg) => (saveMsg = msg)}
              onError={(err) => (saveError = err)}
            />
          {:else if activeTab === "users"}
            <ConfigTabUsers
              bind:this={usersTabRef}
              {currentUser}
              onError={(err) => (saveError = err)}
            />
          {/if}
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5">
        {#if activeTab === 'mib' || activeTab === 'icons' || activeTab === 'users'}
          <div></div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              onclick={() => (show = false)}
              class="px-5 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
            >
              {$_('common.close')}
            </button>
          </div>
        {:else}
          <div class="text-[11px] text-slate-500 dark:text-slate-400">
            {$_('config.saveHint')}
          </div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              onclick={() => (show = false)}
              class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
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
        {/if}
      </div>
    </div>
  </div>
{/if}

<ImportMapModal
  bind:show={showImportModal}
  onImported={() => { onSaved?.(); loadConfig(); }}
/>
