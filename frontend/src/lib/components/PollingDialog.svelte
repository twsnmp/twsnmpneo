<script lang="ts">
  import { untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import { savePolling, type PollingEnt, type NodeEnt } from "../api";
  import { X, Save, CheckSquare } from "@lucide/svelte";
  import CodeJar from "./CodeJar.svelte";
  import Prism from "prismjs";
  import "prismjs/components/prism-regex";
  import "prismjs/components/prism-javascript";

  // Grok pattern syntax
  Prism.languages["grok"] = {
    number: /%\{.+?\}/,
    string: /\.\+/,
    regex: /\\s\+/,
  };

  // TWSNMP action syntax (wol, mail, webhook, wait, cmd)
  Prism.languages["twaction"] = {
    regex: /[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}:[0-9a-fA-F]{2}/,
    keyword: /(wol|mail|webhook|wait|cmd)/,
    number: /-?\b\d+(?:\.\d+)?(?:e[+-]?\d+)?\b/i,
    string: /\b(?:false|true|up|down)\b/,
    url: /https?:\/\/(www\.)?[-a-zA-Z0-9@:%._+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_+.~#?&/=]*)/,
  };

  const highlight = (code: string, syntax: string | undefined): string => {
    if (!syntax || !Prism.languages[syntax]) return code;
    return Prism.highlight(code, Prism.languages[syntax], syntax);
  };

  let {
    show = $bindable(false),
    polling = $bindable<PollingEnt | null>(null),
    nodes = [],
    onSave = () => {}
  } = $props<{
    show: boolean;
    polling: PollingEnt | null;
    nodes?: NodeEnt[];
    onSave?: (saved: PollingEnt) => void;
  }>();

  let id = $state("");
  let name = $state("");
  let nodeId = $state("");
  let type = $state("ping");
  let mode = $state("");
  let params = $state("");
  let filter = $state("");
  let extractor = $state("");
  let script = $state("");
  let level = $state("off");
  let pollInt = $state(60);
  let timeout = $state(1);
  let retry = $state(1);
  let logMode = $state(0);
  let failAction = $state("");
  let repairAction = $state("");
  let aiMode = $state("default");
  let vectorCols = $state("");
  let mqttURL = $state("");
  let mqttTopic = $state("");
  let mqttCols = $state("");
  let saveError = $state("");
  let isSubmitting = $state(false);

  // True when editing an existing polling (id is set)
  const isEditing = $derived(id !== "");

  // Node display name for read-only mode when editing
  const selectedNodeName = $derived(() => {
    if (!isEditing) return "";
    const n = nodes.find((node: NodeEnt) => (node.id || node.ID) === nodeId);
    if (!n) return nodeId;
    return `${n.name || n.Name} (${n.ip || n.IP})`;
  });

  const pollingTypes = [
    { value: "ping", label: "PING" },
    { value: "snmp", label: "SNMP" },
    { value: "gnmi", label: "gNMI" },
    { value: "tcp", label: "TCP" },
    { value: "http", label: "HTTP" },
    { value: "tls", label: "TLS" },
    { value: "dns", label: "DNS" },
    { value: "ntp", label: "NTP" },
    { value: "syslog", label: "SYSLOG" },
    { value: "trap", label: "SNMP TRAP" },
    { value: "arplog", label: "ARP Log" },
    { value: "netflow", label: "NetFlow" },
    { value: "cmd", label: "Command" },
    { value: "ssh", label: "SSH" },
    { value: "report", label: "Report" },
    { value: "twsnmp", label: "TWSNMP" },
    { value: "twlogeye", label: "TwLogEye" },
    { value: "pihole", label: "Pi-Hole" },
    { value: "lxi", label: "LXI" },
    { value: "monitor", label: "Monitor" },
    { value: "mqtt", label: "MQTT" },
    { value: "email", label: "EMAIL" },
    { value: "stun", label: "STUN" },
    { value: "twwifiscan", label: "twWifiScan Report" },
    { value: "twbluescan", label: "twBlueScan Report" },
    { value: "twpcap", label: "twpcap Report" },
    { value: "twwinlog", label: "twwinlog Report" },
  ];

  const controlClass =
    "w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none";
  const labelClass = "block font-semibold text-slate-700 dark:text-slate-300";
  const levels = ["high", "low", "warn", "info", "off"];

  $effect(() => {
    if (!show) return;
    untrack(() => {
      saveError = "";
      id = polling?.id ?? polling?.ID ?? "";
      name = polling?.name ?? polling?.Name ?? $_("polling.defaultName");
      nodeId = polling?.node_id ?? polling?.NodeID ?? (nodes[0]?.id || nodes[0]?.ID || "");
      type = polling?.type ?? polling?.Type ?? "ping";
      mode = polling?.mode ?? polling?.Mode ?? "";
      params = polling?.params ?? polling?.Params ?? "";
      filter = polling?.filter ?? polling?.Filter ?? "";
      extractor = polling?.extractor ?? polling?.Extractor ?? "";
      script = polling?.script ?? polling?.Script ?? "";
      level = polling?.level ?? polling?.Level ?? "off";
      pollInt = polling?.poll_int ?? polling?.PollInt ?? 60;
      timeout = polling?.timeout ?? polling?.Timeout ?? 1;
      retry = polling?.retry ?? polling?.Retry ?? 1;
      logMode = polling?.log_mode ?? polling?.LogMode ?? 0;
      failAction = polling?.fail_action ?? polling?.FailAction ?? "";
      repairAction = polling?.repair_action ?? polling?.RepairAction ?? "";
      aiMode = polling?.ai_mode ?? polling?.AIMode ?? "default";
      vectorCols = polling?.vector_cols ?? polling?.VectorCols ?? "";
      mqttURL = polling?.mqtt_url ?? polling?.MqttURL ?? "";
      mqttTopic = polling?.mqtt_topic ?? polling?.MqttTopic ?? "";
      mqttCols = polling?.mqtt_cols ?? polling?.MqttCols ?? "";
    });
  });

  const handleSave = async (e: Event) => {
    e.preventDefault();
    if (!name.trim()) {
      saveError = $_("polling.errNameRequired");
      return;
    }
    if (!nodeId) {
      saveError = $_("polling.errNodeRequired");
      return;
    }

    isSubmitting = true;
    saveError = "";
    try {
      const saved = await savePolling({
        id,
        name,
        node_id: nodeId,
        type,
        mode,
        params,
        filter,
        extractor,
        script,
        level,
        poll_int: pollInt,
        timeout,
        retry,
        log_mode: logMode,
        fail_action: failAction,
        repair_action: repairAction,
        ai_mode: aiMode,
        vector_cols: vectorCols,
        mqtt_url: mqttURL,
        mqtt_topic: mqttTopic,
        mqtt_cols: mqttCols,
      });
      onSave(saved);
      show = false;
    } catch (err: unknown) {
      saveError = err instanceof Error ? err.message : $_("polling.errSaveFailed");
    } finally {
      isSubmitting = false;
    }
  };
</script>

{#if show}
  <div
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div
      class="w-full max-w-2xl rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden flex flex-col max-h-[90vh] text-slate-800 dark:text-slate-100 font-sans"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-5 py-3.5 bg-slate-50 dark:bg-slate-950/80">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-500 dark:text-cyan-400">
            <CheckSquare class="h-4 w-4" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
              {isEditing ? $_("polling.editTitle") : $_("polling.createTitle")}
            </h3>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">{$_("polling.subtitle")}</p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
          title={$_("common.close")}
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <form onsubmit={handleSave} class="flex-1 overflow-y-auto p-5 space-y-4 text-xs">
        {#if saveError}
          <div class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-rose-600 dark:text-rose-300 font-medium">
            {saveError}
          </div>
        {/if}

        <!-- Name + Node -->
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <label for="poll-name" class={labelClass}>
              {$_("polling.name")} <span class="text-rose-500">*</span>
            </label>
            <input id="poll-name" class={controlClass} bind:value={name} required />
          </div>
          <div class="space-y-1.5">
            <label for="poll-node" class={labelClass}>
              {$_("polling.targetNode")} <span class="text-rose-500">*</span>
            </label>
            {#if isEditing}
              <!-- 編集時はノード変更不可 -->
              <div
                class="w-full rounded-xl border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-3 py-2 text-slate-600 dark:text-slate-400 cursor-not-allowed"
                title={$_("polling.nodeLockedHint")}
              >
                {selectedNodeName()}
              </div>
            {:else}
              <select
                id="poll-node"
                bind:value={nodeId}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              >
                {#if nodes.length === 0}
                  <option value="">{$_("polling.noNodes")}</option>
                {/if}
                {#each nodes as n}
                  <option value={n.id || n.ID}>{n.name || n.Name} ({n.ip || n.IP})</option>
                {/each}
              </select>
            {/if}
          </div>
        </div>

        <!-- Type / Mode / Level / LogMode -->
        <div class="grid gap-3 sm:grid-cols-4">
          <div class="space-y-1.5">
            <label for="poll-type" class={labelClass}>{$_("polling.type")}</label>
            <select id="poll-type" bind:value={type} class={controlClass} disabled={isEditing}>
              {#if !pollingTypes.some((pt) => pt.value === type)}
                <option value={type}>{type.toUpperCase()}</option>
              {/if}
              {#each pollingTypes as pt}
                <option value={pt.value}>{pt.label}</option>
              {/each}
            </select>
          </div>
          <div class="space-y-1.5">
            <label for="poll-mode" class={labelClass}>{$_("polling.mode")}</label>
            {#if type === "ping"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">{$_("polling.modeDefault")}</option>
                <option value="line">{$_("polling.modeLine")}</option>
                <option value="smoke">{$_("polling.modeSmoke")}</option>
              </select>
            {:else if type === "snmp"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">get (OID GET / 値取得)</option>
                <option value="delta">delta (前回値との差分)</option>
                <option value="ps">ps (1秒あたりの増分 rate)</option>
                <option value="sysUpTime">sysUpTime (稼働時間・再起動検知)</option>
                <option value="ifOperStatus">ifOperStatus (ポート稼働状態)</option>
                <option value="traffic">traffic (トラフィック bps/pps)</option>
                <option value="count">count (Walk マッチ行数カウント)</option>
                <option value="process">process (プロセス一覧・変化検知)</option>
                <option value="stats">stats (テーブル数値集計 sum/avg)</option>
                <option value="hrSystemDate">hrSystemDate (システム時刻差分)</option>
                <option value="script">script (snmpGet スクリプト)</option>
              </select>
            {:else if type === "http" || type === "https"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">{$_("polling.modeDefault")}</option>
                <option value="hash">hash (コンテンツSHA256変化検知)</option>
                <option value="metrics">metrics (サーバーメトリクス解析)</option>
              </select>
            {:else if type === "tcp" || type === "tls"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">{type === "tls" ? "verify" : $_("polling.modeDefault")}</option>
                {#if type === "tls"}
                  <option value="cert">cert (サーバー証明書レポート)</option>
                {/if}
                <option value="verify">verify (証明書検証)</option>
                <option value="version">version (バージョン確認)</option>
                <option value="expire">expire (有効期限確認)</option>
              </select>
            {:else if type === "dns"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="ipaddr">ipaddr (A/AAAA・IP変化検知)</option>
                <option value="addr">addr (正引き)</option>
                <option value="host">host (逆引き)</option>
                <option value="mx">mx (メールサーバー)</option>
                <option value="ns">ns (ネームサーバー)</option>
                <option value="txt">txt (テキストレコード)</option>
                <option value="cname">cname (エイリアス)</option>
              </select>
            {:else if type === "stun"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">udp4 (IPv4)</option>
                <option value="ipv6">ipv6 / udp6 (IPv6)</option>
              </select>
            {:else if type === "syslog"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">count (ログ件数カウント)</option>
                <option value="pri">pri (プライオリティ別集計)</option>
                <option value="stats">stats (統計集計)</option>
                <option value="sigma">sigma (Sigma脅威検知ルール)</option>
              </select>
            {:else if type === "trap" || type === "snmptrap"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">count (TRAP件数カウント)</option>
                <option value="stats">stats (統計集計)</option>
              </select>
            {:else if type === "arplog"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">count (ARPイベントカウント)</option>
                <option value="stats">stats (統計集計)</option>
              </select>
            {:else if type === "netflow"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="traffic">traffic (流量 bps/pps)</option>
                <option value="count">count (フロー件数)</option>
                <option value="stats">stats (統計集計)</option>
              </select>
            {:else if type === "gnmi"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">get (テレメトリ取得)</option>
                <option value="subscribe">subscribe (購読ストリーム)</option>
              </select>
            {:else if type === "mqtt"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">subscribe (トピック購読)</option>
                <option value="connect">connect (接続確認・RTT)</option>
              </select>
            {:else if type === "email"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">stats (メール件数・サイズ)</option>
                <option value="login">login (接続確認・RTT)</option>
              </select>
            {:else if type === "twlogeye"}
              <select id="poll-mode" bind:value={mode} class={controlClass}>
                <option value="">notify (通知監視)</option>
                <option value="report.syslog">report.syslog</option>
                <option value="report.trap">report.trap</option>
                <option value="report.netflow">report.netflow</option>
                <option value="report.winevent">report.winevent</option>
                <option value="report.mqtt">report.mqtt</option>
                <option value="report.otel">report.otel</option>
                <option value="report.anomaly">report.anomaly</option>
              </select>
            {:else}
              <input id="poll-mode" class={controlClass} bind:value={mode} />
            {/if}
          </div>
          <div class="space-y-1.5">
            <label for="poll-level" class={labelClass}>{$_("polling.level")}</label>
            <select id="poll-level" bind:value={level} class={controlClass}>
              {#each levels as value}
                <option {value}>{$_(`polling.levels.${value}`)}</option>
              {/each}
            </select>
          </div>
          <div class="space-y-1.5">
            <label for="poll-log-mode" class={labelClass}>{$_("polling.logMode")}</label>
            <select
              id="poll-log-mode"
              value={logMode}
              onchange={(e) => (logMode = Number((e.currentTarget as HTMLSelectElement).value))}
              class={controlClass}
            >
              <option value={0}>{$_("polling.logModes.none")}</option>
              <option value={1}>{$_("polling.logModes.always")}</option>
              <option value={2}>{$_("polling.logModes.onChange")}</option>
              <option value={3}>{$_("polling.logModes.ai")}</option>
            </select>
          </div>
        </div>

        <!-- Params -->
        <div class="space-y-1.5">
          <label for="poll-params" class={labelClass}>{$_("polling.params")}</label>
          <input id="poll-params" class={`${controlClass} font-mono`} bind:value={params} />
          <p class="text-[11px] text-slate-500 dark:text-slate-400">
            {type === "ping" ? $_("polling.pingParamsHelp") : $_("polling.paramsHelp")}
          </p>
        </div>

        <!-- Interval / Timeout / Retry -->
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="space-y-1.5">
            <label for="poll-interval" class={labelClass}>{$_("polling.pollInterval")}</label>
            <input id="poll-interval" class={controlClass} type="number" min="1" max="86400" bind:value={pollInt} />
          </div>
          <div class="space-y-1.5">
            <label for="poll-timeout" class={labelClass}>{$_("polling.timeout")}</label>
            <input id="poll-timeout" class={controlClass} type="number" min="0" max="3600" bind:value={timeout} />
          </div>
          <div class="space-y-1.5">
            <label for="poll-retry" class={labelClass}>{$_("polling.retry")}</label>
            <input id="poll-retry" class={controlClass} type="number" min="0" max="50" bind:value={retry} />
          </div>
        </div>

        <!-- Filter (regex) + Extractor (grok) -->
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="space-y-1.5">
            <label for="poll-filter" class={labelClass}>{$_("polling.filter")}</label>
            <CodeJar id="poll-filter" syntax="regex" {highlight} bind:value={filter} />
          </div>
          <div class="space-y-1.5">
            <label for="poll-extractor" class={labelClass}>{$_("polling.extractor")}</label>
            <CodeJar id="poll-extractor" syntax="grok" {highlight} bind:value={extractor} />
          </div>
        </div>

        <!-- Script (JavaScript) -->
        <div class="space-y-1.5">
          <label for="poll-script" class={labelClass}>{$_("polling.script")}</label>
          <CodeJar id="poll-script" syntax="javascript" {highlight} catchTab={true} bind:value={script} />
          <p class="text-[11px] text-slate-500 dark:text-slate-400">{$_("polling.scriptHelp")}</p>
        </div>

        <!-- AI options (logMode === 3) -->
        {#if logMode === 3}
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="space-y-1.5">
              <label for="poll-ai-mode" class={labelClass}>{$_("polling.aiMode")}</label>
              <select id="poll-ai-mode" class={controlClass} bind:value={aiMode}>
                <option value="default">{$_("polling.aiModes.default")}</option>
                <option value="iforest">{$_("polling.aiModes.iforest")}</option>
                <option value="zscore">{$_("polling.aiModes.zscore")}</option>
                <option value="lof">{$_("polling.aiModes.lof")}</option>
                <option value="knn">{$_("polling.aiModes.knn")}</option>
                <option value="mahalanobis">{$_("polling.aiModes.mahalanobis")}</option>
                <option value="hotelling">{$_("polling.aiModes.hotelling")}</option>
                <option value="autoencoder">{$_("polling.aiModes.autoencoder")}</option>
                <option value="lstm">{$_("polling.aiModes.lstm")}</option>
              </select>
            </div>
            <div class="space-y-1.5">
              <label for="poll-vector-cols" class={labelClass}>{$_("polling.vectorCols")}</label>
              <input id="poll-vector-cols" class={controlClass} bind:value={vectorCols} placeholder="rtt,loss,load..." />
            </div>
          </div>
        {/if}

        <!-- FailAction + RepairAction (twaction) -->
        <div class="space-y-1.5">
          <label for="poll-fail-action" class={labelClass}>{$_("polling.failAction")}</label>
          <CodeJar id="poll-fail-action" syntax="twaction" {highlight} catchTab={true} bind:value={failAction} />
        </div>
        <div class="space-y-1.5">
          <label for="poll-repair-action" class={labelClass}>{$_("polling.repairAction")}</label>
          <CodeJar id="poll-repair-action" syntax="twaction" {highlight} catchTab={true} bind:value={repairAction} />
        </div>

        <!-- MQTT -->
        <fieldset class="space-y-3 rounded-xl border border-slate-200 dark:border-slate-800 p-3">
          <legend class="px-1 font-semibold">{$_("polling.mqtt")}</legend>
          <div class="grid gap-3 sm:grid-cols-3">
            <div class="space-y-1.5">
              <label for="poll-mqtt-url" class={labelClass}>{$_("polling.mqttURL")}</label>
              <input id="poll-mqtt-url" class={controlClass} bind:value={mqttURL} />
            </div>
            <div class="space-y-1.5">
              <label for="poll-mqtt-topic" class={labelClass}>{$_("polling.mqttTopic")}</label>
              <input id="poll-mqtt-topic" class={controlClass} bind:value={mqttTopic} />
            </div>
            <div class="space-y-1.5">
              <label for="poll-mqtt-cols" class={labelClass}>{$_("polling.mqttCols")}</label>
              <input id="poll-mqtt-cols" class={controlClass} bind:value={mqttCols} />
            </div>
          </div>
        </fieldset>

        <!-- Footer -->
        <div class="flex items-center justify-end gap-2.5 pt-4 border-t border-slate-200 dark:border-slate-800">
          <button
            type="button"
            onclick={() => (show = false)}
            class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-2 font-semibold text-slate-700 dark:text-slate-300 transition-colors"
          >
            {$_("common.cancel")}
          </button>
          <button
            type="submit"
            disabled={isSubmitting}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-5 py-2 font-bold text-white shadow-md shadow-cyan-600/30 transition-all disabled:opacity-50"
          >
            <Save class="h-3.5 w-3.5" />
            <span>{isSubmitting ? $_("common.saving") : $_("common.save")}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
