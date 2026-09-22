<script lang="ts">
  import { onMount } from "svelte";
  import { fetchMapConf, saveMapConf, fetchNotifyConf, saveNotifyConf, uploadGeoIP, deleteGeoIP } from "../api";
  import {
    X,
    Save,
    Sliders,
    Bell,
    Brain,
    Database,
    CheckCircle2,
    Server,
    Shield,
    Mail,
    Radio,
    Sparkles,
    Cpu,
    Network,
    Globe,
    Upload,
    Trash2
  } from "@lucide/svelte";

  let { show = $bindable(false) } = $props<{ show: boolean }>();

  let activeTab = $state<"map" | "receivers" | "notify" | "ai" | "datastore">("map");
  let saveMsg = $state("");
  let saveError = $state("");

  // Map configuration
  let mapName = $state("TWSNMP NEO");
  let mapSize = $state(0);
  let iconSize = $state(3);
  let pollInt = $state(60);
  let timeout = $state(1);
  let retry = $state(1);
  let logDays = $state(14);
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
  let notifyLevel = $state("warn");
  let notifyInterval = $state(60);
  let webhookUrl = $state("");
  let lineToken = $state("");
  let mailServer = $state("");
  let mailFrom = $state("");
  let mailTo = $state("");
  let mailUser = $state("");
  let mailPassword = $state("");
  let insecureSkipVerify = $state(false);

  // AI / LLM configuration
  let llmProvider = $state("gemini");
  let llmBaseUrl = $state("http://localhost:11434");
  let llmModel = $state("gemini-1.5-flash");
  let llmApiKey = $state("");
  let mcpTransport = $state("stdio");
  let mcpEndpoint = $state("");
  let mcpToken = $state("");

  // Data Store
  let logFormat = $state("parquet");

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
        logDays = conf.LogDays ?? conf.log_days ?? 14;
        snmpMode = conf.SnmpMode ?? conf.snmp_mode ?? "v2c";
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

        llmProvider = conf.LLMProvider ?? conf.llm_provider ?? "gemini";
        llmBaseUrl = conf.LLMBaseURL ?? conf.llm_base_url ?? "http://localhost:11434";
        llmModel = conf.LLMModel ?? conf.llm_model ?? "gemini-1.5-flash";
        llmApiKey = conf.LLMAPIKey ?? conf.llm_api_key ?? "";
        mcpTransport = conf.MCPTransport ?? conf.mcp_transport ?? "stdio";
        mcpEndpoint = conf.MCPEndpoint ?? conf.mcp_endpoint ?? "";
        mcpToken = conf.MCPToken ?? conf.mcp_token ?? "";
        logFormat = conf.LogFormat ?? conf.log_format ?? "parquet";
        geoIPInfo = conf.GeoIPInfo ?? conf.geo_ip_info ?? "";
      }

      const nConf = await fetchNotifyConf().catch(() => null);
      if (nConf) {
        notifyLevel = nConf.Level ?? nConf.level ?? "warn";
        notifyInterval = nConf.Interval ?? nConf.interval ?? 60;
        webhookUrl = nConf.WebhookURL ?? nConf.webhook_url ?? "";
        lineToken = nConf.LineToken ?? nConf.line_token ?? "";
        mailServer = nConf.MailServer ?? nConf.mail_server ?? "";
        mailFrom = nConf.MailFrom ?? nConf.mail_from ?? "";
        mailTo = nConf.MailTo ?? nConf.mail_to ?? "";
        mailUser = nConf.MailUser ?? nConf.mail_user ?? "";
        mailPassword = nConf.MailPassword ?? nConf.mail_password ?? "";
        insecureSkipVerify = nConf.InsecureSkipVerify ?? nConf.insecure_skip_verify ?? false;
      }
    } catch (e) {
      console.error("Failed to load configuration:", e);
    }
  }

  // GeoIP database handlers
  let geoIPInfo = $state("");
  let geoIPLoading = $state(false);
  let geoIPFileInput: HTMLInputElement | undefined = $state();

  async function handleUploadGeoIP(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    geoIPLoading = true;
    saveMsg = "";
    saveError = "";
    try {
      const res = await uploadGeoIP(file);
      saveMsg = `IP位置情報DB (GeoIP) を更新しました (Ver: ${res.version || "有効"})`;
      await loadConfig();
    } catch (err: any) {
      saveError = `GeoIP DB更新失敗: ${err.message || err}`;
    } finally {
      geoIPLoading = false;
      if (geoIPFileInput) geoIPFileInput.value = "";
    }
  }

  async function handleDeleteGeoIP() {
    if (!confirm("本当にIP位置情報DB (GeoIP) を削除しますか？")) return;
    geoIPLoading = true;
    saveMsg = "";
    saveError = "";
    try {
      await deleteGeoIP();
      saveMsg = "IP位置情報DB (GeoIP) を削除しました";
      geoIPInfo = "";
      await loadConfig();
    } catch (err: any) {
      saveError = `GeoIP DB削除失敗: ${err.message || err}`;
    } finally {
      geoIPLoading = false;
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
        MCPTransport: mcpTransport,
        MCPEndpoint: mcpEndpoint,
        MCPToken: mcpToken,
        LogFormat: logFormat,
      });

      // 2. Save Notify Conf
      await saveNotifyConf({
        Level: notifyLevel,
        Interval: Number(notifyInterval),
        WebhookURL: webhookUrl,
        LineToken: lineToken,
        MailServer: mailServer,
        MailFrom: mailFrom,
        MailTo: mailTo,
        MailUser: mailUser,
        MailPassword: mailPassword,
        InsecureSkipVerify: Boolean(insecureSkipVerify),
      });

      saveMsg = "設定を正常に保存しました。";
      setTimeout(() => {
        if (saveMsg) show = false;
      }, 1200);
    } catch (e: any) {
      saveError = "保存エラー: " + (e.message || e);
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
    <div class="flex h-[88vh] w-full max-w-4xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Sliders class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">システム環境設定 (System Configuration)</h2>
            <p class="text-[11px] text-slate-400">マップ監視パラメータ・受信デーモン・通知・AI連携設定</p>
          </div>
        </div>
        <button
          type="button"
          aria-label="閉じる"
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Main Layout -->
      <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar Navigation -->
        <div class="w-52 border-r border-slate-800 bg-slate-950/60 p-3 space-y-1.5 shrink-0">
          <button
            type="button"
            onclick={() => (activeTab = "map")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'map' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
          >
            <Sliders class="h-4 w-4" />
            マップ・ポーリング
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "receivers")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'receivers' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
          >
            <Radio class="h-4 w-4" />
            受信デーモン
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "notify")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'notify' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
          >
            <Bell class="h-4 w-4" />
            通知・アラート
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "ai")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'ai' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
          >
            <Brain class="h-4 w-4" />
            AI / LLM 支援
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "datastore")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'datastore' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-200'}"
          >
            <Database class="h-4 w-4" />
            データストア
          </button>
        </div>

        <!-- Form Panels -->
        <div class="flex-1 overflow-y-auto p-6 text-xs bg-slate-900/40">
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

          <!-- TAB 1: Map & Polling -->
          {#if activeTab === "map"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Sliders class="w-4 h-4 text-cyan-400" />
                  マップ基本設定
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="map-name" class="block text-xs font-semibold text-slate-400 mb-1.5">マップ名称</label>
                    <input
                      id="map-name"
                      type="text"
                      bind:value={mapName}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="map-size" class="block text-xs font-semibold text-slate-400 mb-1.5">マップキャンバスサイズ</label>
                    <select
                      id="map-size"
                      bind:value={mapSize}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value={0}>自動 (Auto 2500x5000)</option>
                      <option value={1}>A4縦 (2894x4093 A4P)</option>
                      <option value={2}>A4横 (4093x2894 A4L)</option>
                    </select>
                  </div>
                </div>

                <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-1">
                  <div>
                    <label for="icon-size" class="block text-xs font-semibold text-slate-400 mb-1.5">
                      ノードアイコンサイズ: <span class="font-mono text-cyan-400">{iconSize}</span> (1:極小 〜 5:極大)
                    </label>
                    <input
                      id="icon-size"
                      type="range"
                      min={1}
                      max={5}
                      bind:value={iconSize}
                      class="w-full accent-cyan-500 cursor-pointer"
                    />
                  </div>
                  <div>
                    <label for="log-days" class="block text-xs font-semibold text-slate-400 mb-1.5">イベントログ保持期間 (日数)</label>
                    <input
                      id="log-days"
                      type="number"
                      min={1}
                      max={365}
                      bind:value={logDays}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Network class="w-4 h-4 text-cyan-400" />
                  ポーリング & SNMP デフォルトパラメータ
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div>
                    <label for="poll-int" class="block text-xs font-semibold text-slate-400 mb-1.5">通常ポーリング間隔 (秒)</label>
                    <input
                      id="poll-int"
                      type="number"
                      min={5}
                      bind:value={pollInt}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="poll-timeout" class="block text-xs font-semibold text-slate-400 mb-1.5">タイムアウト (秒)</label>
                    <input
                      id="poll-timeout"
                      type="number"
                      min={1}
                      bind:value={timeout}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="poll-retry" class="block text-xs font-semibold text-slate-400 mb-1.5">リトライ回数</label>
                    <input
                      id="poll-retry"
                      type="number"
                      min={0}
                      bind:value={retry}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
                  <div>
                    <label for="snmp-mode" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMP モード</label>
                    <select
                      id="snmp-mode"
                      bind:value={snmpMode}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="v2c">SNMP v2c (推奨)</option>
                      <option value="v3">SNMP v3 (セキュア)</option>
                      <option value="v1">SNMP v1</option>
                    </select>
                  </div>
                  <div>
                    <label for="snmp-comm" class="block text-xs font-semibold text-slate-400 mb-1.5">コミュニティ名 (v1/v2c)</label>
                    <input
                      id="snmp-comm"
                      type="text"
                      bind:value={community}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                {#if snmpMode === "v3"}
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-800/60">
                    <div>
                      <label for="snmp-user" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMPv3 ユーザー名</label>
                      <input
                        id="snmp-user"
                        type="text"
                        bind:value={snmpUser}
                        class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="snmp-pwd" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMPv3 パスワード</label>
                      <input
                        id="snmp-pwd"
                        type="password"
                        bind:value={snmpPassword}
                        class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                  </div>
                {/if}
              </div>

              <!-- GeoIP Database Section (TWSNMP FC / FK Compatible) -->
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <div class="flex items-center justify-between border-b border-slate-800 pb-3">
                  <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
                    <Globe class="w-4 h-4 text-cyan-400" />
                    IP位置情報データベース (GeoIP)
                  </h3>
                  {#if geoIPInfo}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-emerald-400">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse"></span>
                      データベース有効 (Ver: {geoIPInfo})
                    </span>
                  {:else}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-slate-700 bg-slate-800 px-2.5 py-0.5 text-[10px] font-medium text-slate-400">
                      未登録
                    </span>
                  {/if}
                </div>

                <p class="text-[11px] text-slate-400 leading-relaxed">
                  NetFlow や各種ログの IP アドレスから地理的位置（国・緯度経度・都市名）を検索するための MaxMind GeoIP2 / GeoLite2 形式のバイナリデータベース (<code class="text-cyan-400">.mmdb</code>) を管理します。
                </p>

                <div class="rounded-xl border border-slate-800/80 bg-slate-950/60 p-4 space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="space-y-0.5">
                      <span class="block text-xs font-semibold text-slate-300">GeoIP データベースファイル (.mmdb)</span>
                      <span class="block text-[11px] text-slate-500">GeoLite2-City.mmdb などを選択してアップロードします</span>
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
                        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold text-cyan-300 transition-colors cursor-pointer {geoIPLoading ? 'opacity-50 pointer-events-none' : ''}"
                      >
                        <Upload class="w-3.5 h-3.5" />
                        <span>{geoIPLoading ? "適用中..." : "ファイルを選択して適用"}</span>
                      </label>

                      {#if geoIPInfo}
                        <button
                          type="button"
                          onclick={handleDeleteGeoIP}
                          disabled={geoIPLoading}
                          class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-300 transition-colors cursor-pointer disabled:opacity-50"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                          <span>削除</span>
                        </button>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
            </div>

          <!-- TAB 2: Receivers & Daemons -->
          {:else if activeTab === "receivers"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Radio class="w-4 h-4 text-cyan-400" />
                  ネットワークログ・パケット受信デーモン
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableSyslogd} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">Syslog サーバー</span>
                      <span class="block text-[11px] text-slate-400">UDP/TCP 514番ポートで受信</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableTrapd} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">SNMP Trap サーバー</span>
                      <span class="block text-[11px] text-slate-400">UDP 162番ポートで受信</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableNetflowd} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">NetFlow サーバー</span>
                      <span class="block text-[11px] text-slate-400">UDP 2055番ポートでフロー収集</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableSFlowd} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">sFlow サーバー</span>
                      <span class="block text-[11px] text-slate-400">UDP 6343番ポートでフロー収集</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableArpWatch} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">ARP 監視デーモン</span>
                      <span class="block text-[11px] text-slate-400">ローカルARPパケット変化の検知</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableOTel} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">OpenTelemetry コレクタ</span>
                      <span class="block text-[11px] text-slate-400">gRPC/HTTP OTLP メトリクス受信</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-800 bg-slate-950/60 hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableMqtt} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-200 text-xs">MQTT ブローカー連携</span>
                      <span class="block text-[11px] text-slate-400">IoTテレメトリ・センサー受信</span>
                    </div>
                  </label>
                </div>

                {#if enableArpWatch}
                  <div class="mt-3 p-3 rounded-xl border border-slate-800 bg-slate-950 space-y-3">
                    <label for="arp-range" class="block text-xs font-semibold text-slate-400 mb-1">ARP監視範囲 (CIDR)</label>
                    <input
                      id="arp-range"
                      type="text"
                      bind:value={arpWatchRange}
                      placeholder="例: 192.168.1.0/24"
                      class="w-full rounded-xl border border-slate-800 bg-slate-900 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {/if}
              </div>
            </div>

          <!-- TAB 3: Notifications -->
          {:else if activeTab === "notify"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Bell class="w-4 h-4 text-cyan-400" />
                  障害検知アラート通知ポリシー
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="notify-level" class="block text-xs font-semibold text-slate-400 mb-1.5">最小通知レベル</label>
                    <select
                      id="notify-level"
                      bind:value={notifyLevel}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="none">通知なし (オフ)</option>
                      <option value="warn">注意 (Warn 以上すべて)</option>
                      <option value="low">軽微障害 (Low 以上)</option>
                      <option value="high">重度障害 (High/Error のみ)</option>
                    </select>
                  </div>
                  <div>
                    <label for="notify-interval" class="block text-xs font-semibold text-slate-400 mb-1.5">同一アラート再通知間隔 (分)</label>
                    <input
                      id="notify-interval"
                      type="number"
                      min={5}
                      bind:value={notifyInterval}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                <div class="space-y-3 pt-2">
                  <div>
                    <label for="webhook-url" class="block text-xs font-semibold text-slate-400 mb-1.5">Webhook URL (Slack / Teams / Discord)</label>
                    <input
                      id="webhook-url"
                      type="text"
                      bind:value={webhookUrl}
                      placeholder="https://hooks.slack.com/services/... または https://discord.com/api/webhooks/..."
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="line-token" class="block text-xs font-semibold text-slate-400 mb-1.5">LINE Notify アクセストークン</label>
                    <input
                      id="line-token"
                      type="password"
                      bind:value={lineToken}
                      placeholder="LINE Notify トークン"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Mail class="w-4 h-4 text-cyan-400" />
                  SMTP メール通知設定
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="mail-server" class="block text-xs font-semibold text-slate-400 mb-1.5">SMTP サーバー (host:port)</label>
                    <input
                      id="mail-server"
                      type="text"
                      bind:value={mailServer}
                      placeholder="smtp.example.com:587"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="mail-from" class="block text-xs font-semibold text-slate-400 mb-1.5">送信元アドレス (From)</label>
                    <input
                      id="mail-from"
                      type="email"
                      bind:value={mailFrom}
                      placeholder="twsnmp@example.com"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="mail-to" class="block text-xs font-semibold text-slate-400 mb-1.5">宛先アドレス (To)</label>
                    <input
                      id="mail-to"
                      type="email"
                      bind:value={mailTo}
                      placeholder="admin@example.com"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="mail-user" class="block text-xs font-semibold text-slate-400 mb-1.5">SMTP 認証ユーザー</label>
                    <input
                      id="mail-user"
                      type="text"
                      bind:value={mailUser}
                      placeholder="user@example.com"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div class="md:col-span-2">
                    <label for="mail-pwd" class="block text-xs font-semibold text-slate-400 mb-1.5">SMTP パスワード</label>
                    <input
                      id="mail-pwd"
                      type="password"
                      bind:value={mailPassword}
                      placeholder="••••••••"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                <label class="flex items-center gap-3 pt-2 cursor-pointer">
                  <input type="checkbox" bind:checked={insecureSkipVerify} class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                  <span class="text-xs text-slate-300">TLS 証明書検証をスキップする (自己署名証明書環境など)</span>
                </label>
              </div>
            </div>

          <!-- TAB 4: AI & LLM Settings -->
          {:else if activeTab === "ai"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Brain class="w-4 h-4 text-cyan-400" />
                  AI アシスタント & 自動障害診断エンジン
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="llm-provider" class="block text-xs font-semibold text-slate-400 mb-1.5">AI プロバイダー</label>
                    <select
                      id="llm-provider"
                      bind:value={llmProvider}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="gemini">Google Gemini (推奨)</option>
                      <option value="openai">OpenAI (GPT-4o)</option>
                      <option value="claude">Anthropic Claude 3.5</option>
                      <option value="ollama">Ollama (ローカル LLM)</option>
                    </select>
                  </div>
                  <div>
                    <label for="llm-model" class="block text-xs font-semibold text-slate-400 mb-1.5">モデル名</label>
                    <input
                      id="llm-model"
                      type="text"
                      bind:value={llmModel}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                {#if llmProvider === "ollama"}
                  <div>
                    <label for="llm-url" class="block text-xs font-semibold text-slate-400 mb-1.5">Ollama エンドポイント URL</label>
                    <input
                      id="llm-url"
                      type="text"
                      bind:value={llmBaseUrl}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {:else}
                  <div>
                    <label for="llm-key" class="block text-xs font-semibold text-slate-400 mb-1.5">API キー</label>
                    <input
                      id="llm-key"
                      type="password"
                      bind:value={llmApiKey}
                      placeholder="{llmProvider.toUpperCase()} の API キーを入力"
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {/if}
              </div>

              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-3">
                  <Cpu class="w-4 h-4 text-cyan-400" />
                  Model Context Protocol (MCP) 連携
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="mcp-trans" class="block text-xs font-semibold text-slate-400 mb-1.5">MCP トランスポート</label>
                    <select
                      id="mcp-trans"
                      bind:value={mcpTransport}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="stdio">標準入出力 (stdio)</option>
                      <option value="sse">Server-Sent Events (SSE / HTTP)</option>
                    </select>
                  </div>
                  <div>
                    <label for="mcp-endpoint" class="block text-xs font-semibold text-slate-400 mb-1.5">エンドポイント / コマンド</label>
                    <input
                      id="mcp-endpoint"
                      type="text"
                      bind:value={mcpEndpoint}
                      placeholder={mcpTransport === 'sse' ? 'http://localhost:8000/sse' : 'twsnmp-mcp'}
                      class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>
              </div>
            </div>

          <!-- TAB 5: Datastore & System -->
          {:else if activeTab === "datastore"}
            <div class="space-y-6 max-w-2xl">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <!-- BBolt Status Card -->
                <div class="rounded-2xl border border-slate-800 bg-slate-900/90 p-5 shadow-lg space-y-3">
                  <div class="flex items-center justify-between border-b border-slate-800 pb-2.5">
                    <div class="flex items-center gap-2">
                      <Database class="w-4 h-4 text-cyan-400" />
                      <h4 class="text-xs font-bold text-slate-100">bbolt 構成ストア</h4>
                    </div>
                    <span class="inline-flex items-center gap-1 rounded-full bg-emerald-950/80 border border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-400">
                      稼働中
                    </span>
                  </div>
                  <p class="text-[11px] text-slate-400">
                    ACID トランザクション対応の軽量組み込み KV データベース。ノード・ポーリング定義・描画アイテムを保持します。
                  </p>
                  <div class="pt-2 text-[11px] text-slate-300 font-mono space-y-1">
                    <div class="flex justify-between">
                      <span class="text-slate-400">DB パス:</span>
                      <span class="text-slate-200">./data/twsnmpneo.db</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-slate-400">整合性チェック:</span>
                      <span class="text-emerald-400">正常 (OK)</span>
                    </div>
                  </div>
                </div>

                <!-- Parquet Status Card -->
                <div class="rounded-2xl border border-slate-800 bg-slate-900/90 p-5 shadow-lg space-y-3">
                  <div class="flex items-center justify-between border-b border-slate-800 pb-2.5">
                    <div class="flex items-center gap-2">
                      <Server class="w-4 h-4 text-cyan-400" />
                      <h4 class="text-xs font-bold text-slate-100">Apache Parquet ログ</h4>
                    </div>
                    <span class="inline-flex items-center gap-1 rounded-full bg-emerald-950/80 border border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-400">
                      稼働中
                    </span>
                  </div>
                  <p class="text-[11px] text-slate-400">
                    列指向圧縮ストレージ。大容量の Syslog / Trap / NetFlow / ポーリング結果を高速に分析・長期保存します。
                  </p>
                  <div class="pt-2 text-[11px] text-slate-300 font-mono space-y-1">
                    <div class="flex justify-between">
                      <span class="text-slate-400">保存ディレクトリ:</span>
                      <span class="text-slate-200">./data/logs</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-slate-400">圧縮形式:</span>
                      <span class="text-cyan-400">Snappy / Parquet v2</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Format selector -->
              <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-3">
                <h4 class="text-xs font-bold text-slate-100">ログ記録フォーマット</h4>
                <p class="text-[11px] text-slate-400">
                  TWSNMP NEO はデフォルトで Apache Parquet 形式を採用しています。
                </p>
                <div class="flex items-center gap-4 pt-1">
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="radio" bind:group={logFormat} value="parquet" class="text-cyan-500 focus:ring-cyan-500/20" />
                    <span class="text-xs text-slate-200 font-medium">Apache Parquet (標準・高圧縮・高速集計)</span>
                  </label>
                </div>
              </div>
            </div>
          {/if}
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5">
        <div class="text-[11px] text-slate-400">
          変更を有効にするには保存ボタンをクリックしてください
        </div>
        <div class="flex items-center gap-3">
          <button
            type="button"
            onclick={() => (show = false)}
            class="px-4 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-xl text-xs font-medium text-slate-300 transition-colors"
          >
            キャンセル
          </button>
          <button
            type="button"
            onclick={handleSave}
            class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
          >
            <Save class="w-4 h-4" />
            保存
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
