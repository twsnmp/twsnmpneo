<script lang="ts">
  import { _, locale } from "svelte-i18n";
  import {
    Radio,
    Users,
    Activity,
    AlertTriangle,
    Download,
    ExternalLink,
    Sparkles,
    Search,
    ChevronDown,
    ChevronRight,
  } from "@lucide/svelte";
  import type { MqttStatEnt, NodeEnt } from "../../api";
  import { renderBytes, formatTimeStr } from "../../common";
  import ReportAIExplainModal from "../../components/ReportAIExplainModal.svelte";

  interface Props {
    searchQuery?: string;
    mqttStats?: MqttStatEnt[];
    nodes?: NodeEnt[];
    loading?: boolean;
    onRefresh?: () => void;
  }

  let {
    searchQuery = "",
    mqttStats = [],
    nodes = [],
    loading = false,
    onRefresh,
  }: Props = $props();

  let localSearch = $state("");
  let showAiModal = $state(false);
  let page = $state(1);
  let pageSize = $state(20);
  let sortKey = $state<string>("Count");
  let sortDir = $state<"asc" | "desc">("desc");

  // Filtered & Sorted Topics
  const filteredStats = $derived.by(() => {
    const q = (localSearch || searchQuery || "").toLowerCase();
    let res = (mqttStats || []).filter((s) => {
      if (!q) return true;
      return (
        s.Topic.toLowerCase().includes(q) ||
        (s.ClientID || "").toLowerCase().includes(q) ||
        (s.Remote || "").toLowerCase().includes(q) ||
        (s.State || "").toLowerCase().includes(q)
      );
    });

    res = [...res].sort((a: any, b: any) => {
      const valA = a[sortKey] ?? 0;
      const valB = b[sortKey] ?? 0;
      if (typeof valA === "number" && typeof valB === "number") {
        return sortDir === "asc" ? valA - valB : valB - valA;
      }
      return sortDir === "asc"
        ? String(valA).localeCompare(String(valB))
        : String(valB).localeCompare(String(valA));
    });

    return res;
  });

  const pagedStats = $derived(
    filteredStats.slice((page - 1) * pageSize, page * pageSize)
  );

  const totalPages = $derived(
    Math.max(1, Math.ceil(filteredStats.length / pageSize))
  );

  // KPIs
  const totalMessages = $derived(
    (mqttStats || []).reduce((acc: number, s: MqttStatEnt) => acc + (s.Count || 0), 0)
  );
  const totalBytes = $derived(
    (mqttStats || []).reduce((acc: number, s: MqttStatEnt) => acc + (s.Bytes || 0), 0)
  );
  const warnOrLowStats = $derived(
    (mqttStats || []).filter((s: MqttStatEnt) => s.State === "warn" || s.State === "low")
  );
  const uniqueClients = $derived(
    new Set((mqttStats || []).map((s: MqttStatEnt) => s.ClientID).filter(Boolean)).size
  );
  const uniqueRemotes = $derived(
    new Set((mqttStats || []).map((s: MqttStatEnt) => s.Remote).filter(Boolean)).size
  );

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);

  // AI Summary & Items
  const aiSummaryText = $derived(
    isJaLocale
      ? `MQTTトピック総数: ${mqttStats.length}件, 総メッセージ数: ${totalMessages.toLocaleString()}件, 総データ量: ${renderBytes(totalBytes)}, 接続クライアント数: ${uniqueClients}台 (接続元IP数: ${uniqueRemotes}), 異常状態(Warn/Low): ${warnOrLowStats.length}件`
      : `Total MQTT Topics: ${mqttStats.length}, Total Messages: ${totalMessages.toLocaleString()}, Total Data: ${renderBytes(totalBytes)}, Connected Clients: ${uniqueClients} (Remote IPs: ${uniqueRemotes}), Problem State (Warn/Low): ${warnOrLowStats.length}`
  );

  const aiItemsList = $derived.by(() => {
    const items: string[] = [];
    for (const s of warnOrLowStats.slice(0, 20)) {
      items.push(isJaLocale
        ? `[異常状態: ${s.State?.toUpperCase()}] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, Remote: ${s.Remote || "-"}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`
        : `[Warning/Low: ${s.State?.toUpperCase()}] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, Remote: ${s.Remote || "-"}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`);
    }
    const topByCount = [...mqttStats].sort((a, b) => (b.Count || 0) - (a.Count || 0)).slice(0, 15);
    for (const s of topByCount) {
      if (!warnOrLowStats.includes(s)) {
        items.push(isJaLocale
          ? `[トラフィック上位] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, State: ${s.State}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`
          : `[Top Traffic] Topic: ${s.Topic}, Client: ${s.ClientID || "-"}, State: ${s.State}, MsgCount: ${s.Count || 0}, Bytes: ${renderBytes(s.Bytes || 0)}`);
      }
    }
    return items;
  });

  const handleSort = (key: string) => {
    if (sortKey === key) {
      sortDir = sortDir === "asc" ? "desc" : "asc";
    } else {
      sortKey = key;
      sortDir = key === "Count" || key === "Bytes" || key === "Last" ? "desc" : "asc";
    }
  };

  const jumpToMqttView = () => {
    window.dispatchEvent(new CustomEvent("twsnmp:change-page", { detail: "mqtt" }));
  };

  export const exportCSV = () => {
    const header = ["Topic", "Client ID", "Remote", "Count", "Bytes", "State", "Last Time"];
    const rows = filteredStats.map((s) => [
      `"${s.Topic.replace(/"/g, '""')}"`,
      `"${(s.ClientID || "").replace(/"/g, '""')}"`,
      `"${s.Remote || ""}"`,
      s.Count || 0,
      s.Bytes || 0,
      s.State || "normal",
      formatTimeStr(s.Last),
    ]);
    const csvContent = "\uFEFF" + [header.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `mqtt_report_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };
</script>

<div class="space-y-6">
  <!-- Top KPI Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <!-- Card 1: Total Topics -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("mqtt.totalTopics") || "総トピック数"}</p>
        <h3 class="text-2xl font-bold font-mono text-slate-800 dark:text-slate-100 mt-1">{mqttStats.length.toLocaleString()}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("mqtt.registeredCount", { values: { count: mqttStats.length } }) || `${mqttStats.length} 件 登録中`}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-cyan-600 dark:text-cyan-400">
        <Radio class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 2: Total Messages -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("mqtt.totalReceived") || "総メッセージ数"}</p>
        <h3 class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400 mt-1">{totalMessages.toLocaleString()}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("mqtt.totalDataSize", { values: { size: renderBytes(totalBytes) } }) || `総データ量: ${renderBytes(totalBytes)}`}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
        <Activity class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 3: Connected Clients -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("mqtt.remoteClients") || "接続クライアント"}</p>
        <h3 class="text-2xl font-bold font-mono text-indigo-600 dark:text-indigo-400 mt-1">{uniqueClients}</h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("mqtt.remoteIpCount", { values: { count: uniqueRemotes } }) || `送信元IP: ${uniqueRemotes} 箇所`}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl bg-indigo-500/10 border border-indigo-500/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
        <Users class="h-5 w-5" />
      </div>
    </div>

    <!-- Card 4: Anomalous States -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm flex items-center justify-between">
      <div>
        <p class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("mqtt.anomalousStates") || "異常状態 (Warn/Low)"}</p>
        <h3 class="text-2xl font-bold font-mono {warnOrLowStats.length > 0 ? 'text-rose-500 dark:text-rose-400' : 'text-slate-700 dark:text-slate-200'} mt-1">
          {warnOrLowStats.length}
        </h3>
        <p class="text-[10px] text-slate-500 mt-1">{$_("mqtt.topicsToCheck") || "要確認トピック"}</p>
      </div>
      <div class="h-11 w-11 rounded-2xl {warnOrLowStats.length > 0 ? 'bg-rose-500/10 border-rose-500/30 text-rose-500' : 'bg-slate-500/10 border-slate-500/30 text-slate-400'} flex items-center justify-center">
        <AlertTriangle class="h-5 w-5" />
      </div>
    </div>
  </div>

  <!-- Action Header Bar -->
  <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm">
    <div class="flex items-center gap-3">
      <div class="relative w-64">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <input
          type="text"
          placeholder={$_("mqtt.searchPlaceholderReport") || "トピック・クライアント検索..."}
          bind:value={localSearch}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 focus:border-cyan-500 focus:outline-none font-sans"
        />
      </div>
      <span class="text-xs text-slate-500">
        {$_("common.totalCount", { values: { count: filteredStats.length } }) || `全 ${filteredStats.length} 件`}
      </span>
    </div>

    <div class="flex items-center gap-2.5">
      <button
        type="button"
        onclick={() => (showAiModal = true)}
        class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-indigo-600 hover:from-cyan-500 hover:to-indigo-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-indigo-600/25 transition-all cursor-pointer"
      >
        <Sparkles class="h-3.5 w-3.5 text-cyan-200" />
        <span>{$_("report.btnAiMqtt") || "AI MQTT統計診断"}</span>
      </button>

      <button
        type="button"
        onclick={jumpToMqttView}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
        title={$_("mqtt.btnOpenMqttMonitorTooltip") || "独立したMQTTトピック監視・ログ画面を開きます"}
      >
        <ExternalLink class="h-3.5 w-3.5 text-slate-500 dark:text-slate-400" />
        <span>{$_("mqtt.btnOpenMqttMonitor") || "MQTTモニター画面へ"}</span>
      </button>

      <button
        type="button"
        onclick={exportCSV}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
      >
        <Download class="h-3.5 w-3.5" />
        <span>CSV</span>
      </button>
    </div>
  </div>

  <!-- Topics Table -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs text-slate-700 dark:text-slate-300">
        <thead class="bg-slate-100/90 dark:bg-slate-950/80 text-slate-600 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800 uppercase font-semibold">
          <tr>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Topic")}>
              {$_("mqtt.colTopic") || "トピック (Topic)"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("ClientID")}>
              {$_("mqtt.colClientId") || "クライアントID"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Remote")}>
              {$_("mqtt.colRemote") || "送信元 (Remote)"}
            </th>
            <th class="py-2.5 px-3 text-right cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Count")}>
              {$_("mqtt.colMessageCount") || "メッセージ数"}
            </th>
            <th class="py-2.5 px-3 text-right cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Bytes")}>
              {$_("mqtt.colDataSize") || "データ量"}
            </th>
            <th class="py-2.5 px-3 text-center cursor-pointer hover:text-cyan-500" onclick={() => handleSort("State")}>
              {$_("mqtt.colState") || "状態"}
            </th>
            <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-500" onclick={() => handleSort("Last")}>
              {$_("mqtt.colLastTime") || "最終受信"}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/60 font-mono text-[11px]">
          {#if pagedStats.length === 0}
            <tr>
              <td colspan="7" class="py-8 text-center text-slate-500">
                {$_("mqtt.noStats") || "MQTTトピック統計データはありません"}
              </td>
            </tr>
          {:else}
            {#each pagedStats as s}
              <tr class="hover:bg-slate-50/80 dark:hover:bg-slate-800/40 transition-colors">
                <td class="py-1 px-2 font-semibold text-slate-800 dark:text-slate-100 truncate max-w-[280px]" title={s.Topic}>
                  {s.Topic}
                </td>
                <td class="py-1 px-2 text-slate-600 dark:text-slate-400 truncate max-w-[180px]">
                  {s.ClientID || "-"}
                </td>
                <td class="py-1 px-2 text-slate-500">{s.Remote || "-"}</td>
                <td class="py-1 px-2 text-right text-emerald-600 dark:text-emerald-400 font-bold">
                  {(s.Count || 0).toLocaleString()}
                </td>
                <td class="py-1 px-2 text-right text-slate-600 dark:text-slate-400">
                  {renderBytes(s.Bytes || 0)}
                </td>
                <td class="py-1 px-2 text-center">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold {s.State === 'normal' ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300' : s.State === 'warn' ? 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300' : s.State === 'low' ? 'bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300' : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'}">
                    {s.State || "normal"}
                  </span>
                </td>
                <td class="py-1 px-2 text-slate-500 whitespace-nowrap">
                  {formatTimeStr(s.Last)}
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    {#if totalPages > 1}
      <div class="flex items-center justify-between p-3 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40 text-xs">
        <span class="text-slate-500">
          {(page - 1) * pageSize + 1} - {Math.min(page * pageSize, filteredStats.length)} / {filteredStats.length} 件
        </span>
        <div class="flex items-center gap-1">
          <button
            disabled={page <= 1}
            onclick={() => (page = page - 1)}
            class="px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-50 cursor-pointer"
          >
            {$_("mqtt.prev") || "前へ"}
          </button>
          <span class="px-2 text-slate-600 dark:text-slate-400">{page} / {totalPages}</span>
          <button
            disabled={page >= totalPages}
            onclick={() => (page = page + 1)}
            class="px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 disabled:opacity-50 cursor-pointer"
          >
            {$_("mqtt.next") || "次へ"}
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>

{#if showAiModal}
  <ReportAIExplainModal
    bind:show={showAiModal}
    reportType="mqtt"
    title={$_("report.aiMqttTitle") || "MQTT トピック & 統計傾向 AI診断"}
    summary={aiSummaryText}
    items={aiItemsList}
  />
{/if}
