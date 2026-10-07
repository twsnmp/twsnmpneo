<script lang="ts">
  import { _ } from "svelte-i18n";
  import {
    ShieldCheck,
    Edit3,
    Trash2,
    Sparkles,
    ChevronDown,
    ChevronRight,
    AlertCircle,
    CheckCircle2,
    AlertTriangle,
    HelpCircle,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
  } from "@lucide/svelte";
  import {
    deletePolling,
    type PollingEnt,
    type NodeEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import ReportPagination from "./components/ReportPagination.svelte";

  let {
    pollings = [],
    nodes = [],
    searchQuery = "",
    onReload,
  }: {
    pollings?: PollingEnt[];
    nodes?: NodeEnt[];
    searchQuery?: string;
    onReload?: () => Promise<void> | void;
  } = $props();

  let expandedIds = $state<Record<string, boolean>>({});
  let showPollingModal = $state(false);
  let editingPolling = $state<PollingEnt | null>(null);
  let showAiExplainModal = $state(false);
  let aiExplainTarget = $state<any>(null);

  // Pagination & Sorting state
  let pageSize = $state(25);
  let currentPage = $state(1);
  let sortColumn = $state<string>("end");
  let sortDirection = $state<"asc" | "desc">("asc");

  // Extract TLS pollings configured with mode="cert"
  const rawCertItems = $derived.by(() => {
    const list = pollings.filter((p) => {
      const pType = (p.type || (p as any).Type || "").toLowerCase();
      const pMode = (p.mode || (p as any).Mode || "").toLowerCase();
      return pType === "tls" && pMode === "cert";
    });

    return list.map((p) => {
      const pid = p.id || (p as any).ID || "";
      const pnodeId = p.node_id || (p as any).NodeID || "";
      const node = nodes.find((n) => (n.id || (n as any).ID) === pnodeId);
      const res = (p.result || (p as any).Result || {}) as Record<string, any>;
      const pName = p.name || (p as any).Name || "TLS Cert";

      let rawTarget =
        res.target || p.params || (p as any).Params || (node ? node.ip || (node as any).IP : "") || "";
      let host = rawTarget;
      let port = 443;
      if (rawTarget.includes(":")) {
        const parts = rawTarget.split(":");
        host = parts[0];
        port = Number(parts[1]) || 443;
      } else if (!isNaN(Number(rawTarget)) && Number(rawTarget) > 0) {
        port = Number(rawTarget);
        host = node ? node.ip || (node as any).IP : "";
      }

      const subject = res.subject || "-";
      const issuer = res.issuer || "-";
      const serialNumber = res.serialNumber || "-";
      const start = res.notBefore || "-";
      const end = res.notAfter || "-";
      const verify = res.verify !== false && res.valid !== "false";
      const errorMsg = res.error || (p.state === "high" ? (p as any).message || "" : "");
      const keyStrength = res.key || "-";
      const version = res.version || "-";

      // Calculate remaining days
      let days = 0;
      if (res.days !== undefined) {
        days = Number(res.days);
      } else if (res.notAfterUnix) {
        const nowSec = Math.floor(Date.now() / 1000);
        days = Math.ceil((Number(res.notAfterUnix) - nowSec) / 86400);
      }

      // Format Last Time
      const rawLastTime = p.last_time || (p as any).LastTime || 0;
      let lastTimeStr = "-";
      if (rawLastTime > 0) {
        const ms = rawLastTime > 1e12 ? rawLastTime / 1e6 : rawLastTime * 1000;
        const d = new Date(ms);
        const pad = (n: number) => String(n).padStart(2, "0");
        lastTimeStr = `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
      }

      // Determine Report State:
      // High: days <= 0, verify failed, or polling level high/error
      // Warn: days <= 30 or polling state warn
      // Normal: valid and days > 30
      const pollState = (p.state || (p as any).State || "").toLowerCase();
      let status: "high" | "warn" | "normal" | "unknown" = "normal";
      if (pollState === "high" || pollState === "error" || !verify || days <= 0 || (errorMsg && errorMsg.length > 0)) {
        status = "high";
      } else if (pollState === "warn" || days <= 30) {
        status = "warn";
      } else if (pollState === "unknown" || rawLastTime === 0) {
        status = "unknown";
      } else {
        status = "normal";
      }

      return {
        id: pid,
        nodeId: pnodeId,
        nodeName: node ? node.name || (node as any).Name : pnodeId,
        pollingName: pName,
        target: host,
        port,
        subject,
        issuer,
        serialNumber,
        start,
        end,
        days,
        verify,
        error: errorMsg,
        key: keyStrength,
        version,
        lastTime: lastTimeStr,
        rawLastTime,
        status,
        polling: p,
      };
    });
  });

  const filteredCertItems = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return rawCertItems;
    return rawCertItems.filter(
      (c) =>
        c.target.toLowerCase().includes(q) ||
        String(c.port).includes(q) ||
        c.subject.toLowerCase().includes(q) ||
        c.issuer.toLowerCase().includes(q) ||
        c.nodeName.toLowerCase().includes(q) ||
        c.pollingName.toLowerCase().includes(q) ||
        c.status.toLowerCase().includes(q) ||
        c.key.toLowerCase().includes(q)
    );
  });

  const handleSort = (column: string) => {
    if (sortColumn === column) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = column;
      sortDirection = "asc";
    }
  };

  const statePriority: Record<string, number> = {
    high: 1,
    warn: 2,
    normal: 3,
    unknown: 4,
  };

  const sortedCertItems = $derived.by(() => {
    const list = [...filteredCertItems];
    list.sort((a, b) => {
      let cmp = 0;
      switch (sortColumn) {
        case "state":
          cmp = (statePriority[a.status] || 99) - (statePriority[b.status] || 99);
          break;
        case "target":
          cmp = a.target.localeCompare(b.target);
          break;
        case "port":
          cmp = a.port - b.port;
          break;
        case "subject":
          cmp = a.subject.localeCompare(b.subject);
          break;
        case "issuer":
          cmp = a.issuer.localeCompare(b.issuer);
          break;
        case "start":
          cmp = a.start.localeCompare(b.start);
          break;
        case "end":
          cmp = a.days - b.days;
          if (cmp === 0) cmp = a.end.localeCompare(b.end);
          break;
        case "lastTime":
          cmp = a.rawLastTime - b.rawLastTime;
          break;
        default:
          cmp = a.target.localeCompare(b.target);
      }
      return sortDirection === "asc" ? cmp : -cmp;
    });
    return list;
  });

  const paginatedCertItems = $derived.by(() => {
    if (pageSize === -1) return sortedCertItems;
    const startIdx = (currentPage - 1) * pageSize;
    return sortedCertItems.slice(startIdx, startIdx + pageSize);
  });

  const toggleExpand = (id: string, e?: MouseEvent) => {
    if (e) e.stopPropagation();
    expandedIds[id] = !expandedIds[id];
  };

  export function handleAddNewCertPolling(): void {
    editingPolling = {
      id: "",
      name: $_("report.certTitle"),
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "tls",
      mode: "cert",
      params: "443",
      filter: "",
      extractor: "",
      script: "30",
      level: "warn",
      poll_int: 3600,
      timeout: 5,
      retry: 1,
      log_mode: 0,
      next_time: 0,
      last_time: 0,
      result: {},
      state: "unknown",
      fail_action: "",
      repair_action: "",
      ai_mode: "default",
      vector_cols: "",
      mqtt_url: "",
      mqtt_topic: "",
      mqtt_cols: "",
      fail_time: 0,
    } as PollingEnt;
    showPollingModal = true;
  }

  const handleEditItem = (item: any) => {
    editingPolling = item.polling;
    showPollingModal = true;
  };

  const handleDeleteItem = async (item: any) => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'ポーリング削除の確認',
      message: `ポーリング「${item.pollingName}」(${item.target}:${item.port}) を削除しますか？`,
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) {
      return;
    }
    try {
      await deletePolling(item.id);
      await onReload?.();
    } catch (e: any) {
      showAlert({
        title: $_('common.error') || 'エラー',
        message: "削除に失敗しました: " + (e?.message || e),
        type: 'danger',
      });
    }
  };

  const handleAiExplainItem = (item: any) => {
    aiExplainTarget = item;
    showAiExplainModal = true;
  };

  export function exportCSV(): void {
    const csv =
      "State,Target,Port,Subject,Issuer,Start,End,RemainingDays,Verify,Error,LastTime,Key,Node,Polling\n" +
      sortedCertItems
        .map(
          (c) =>
            `"${c.status}","${c.target}",${c.port},"${c.subject}","${c.issuer}","${c.start}","${c.end}",${c.days},${c.verify},"${c.error}","${c.lastTime}","${c.key}","${c.nodeName}","${c.pollingName}"`
        )
        .join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_server_cert_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-4">
  <!-- Top Header Title -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <ShieldCheck class="w-5 h-5 text-cyan-500 dark:text-cyan-400" />
        {$_("report.certTitle")}
      </h2>
      <p class="text-xs text-slate-500 dark:text-slate-400 mt-1">{$_("report.certSubtitle")}</p>
    </div>
  </div>

  <!-- KPI Summary Cards -->
  <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.monitoredCerts")}</span>
      <div class="text-2xl font-bold font-mono text-cyan-600 dark:text-cyan-400">
        {rawCertItems.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span>
      </div>
      <div class="text-[10px] text-slate-400">{$_("report.monitoredCertsSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.validCerts")}</span>
      <div class="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
        {rawCertItems.filter((c) => c.status === "normal").length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span>
      </div>
      <div class="text-[10px] text-slate-400">{$_("report.validCertsSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-500 dark:text-slate-400">{$_("report.expiringCerts")}</span>
      <div class="text-2xl font-bold font-mono text-amber-600 dark:text-amber-400">
        {rawCertItems.filter((c) => c.status === "warn" || c.status === "high").length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span>
      </div>
      <div class="text-[10px] text-amber-500/80">{$_("report.expiringCertsSub")}</div>
    </div>
  </div>

  <!-- Server Certificates Table Container -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm dark:shadow-lg overflow-hidden flex flex-col">
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-mono">
        <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-200 dark:border-slate-800 select-none">
          <tr>
            <th class="py-2.5 px-2 text-center w-8"></th>
            <!-- State Sortable -->
            <th
              onclick={() => handleSort("state")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colState")}</span>
                {#if sortColumn === "state"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Target Sortable -->
            <th
              onclick={() => handleSort("target")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colTarget")}</span>
                {#if sortColumn === "target"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Port Sortable -->
            <th
              onclick={() => handleSort("port")}
              class="py-2.5 px-2.5 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colPort")}</span>
                {#if sortColumn === "port"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Subject Sortable -->
            <th
              onclick={() => handleSort("subject")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colSubject")}</span>
                {#if sortColumn === "subject"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Issuer Sortable -->
            <th
              onclick={() => handleSort("issuer")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colIssuer")}</span>
                {#if sortColumn === "issuer"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Start Sortable -->
            <th
              onclick={() => handleSort("start")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colStart")}</span>
                {#if sortColumn === "start"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- End Sortable -->
            <th
              onclick={() => handleSort("end")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colEnd")}</span>
                {#if sortColumn === "end"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Last Time Sortable -->
            <th
              onclick={() => handleSort("lastTime")}
              class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
            >
              <div class="flex items-center gap-1">
                <span>{$_("report.colLastTime")}</span>
                {#if sortColumn === "lastTime"}
                  {#if sortDirection === "asc"}
                    <ArrowUp class="w-3 h-3 text-cyan-500" />
                  {:else}
                    <ArrowDown class="w-3 h-3 text-cyan-500" />
                  {/if}
                {:else}
                  <ArrowUpDown class="w-3 h-3 opacity-30" />
                {/if}
              </div>
            </th>
            <!-- Actions Header -->
            <th class="py-2.5 px-3 text-center w-28">
              {$_("report.colAction")}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
          {#if sortedCertItems.length === 0}
            <tr>
              <td colspan="10" class="p-8 text-center text-slate-500 font-sans">
                {$_("report.noCertsConfigured")}
              </td>
            </tr>
          {:else}
            {#each paginatedCertItems as c}
              {@const isExpanded = !!expandedIds[c.id]}
              <!-- Main Row -->
              <tr
                onclick={() => toggleExpand(c.id)}
                class="transition-colors cursor-pointer select-none hover:bg-slate-50 dark:hover:bg-slate-800/40"
              >
                <!-- Accordion Toggle -->
                <td class="py-1.5 px-2 text-center">
                  <button
                    type="button"
                    onclick={(e) => toggleExpand(c.id, e)}
                    class="p-0.5 rounded hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors cursor-pointer"
                  >
                    {#if isExpanded}
                      <ChevronDown class="w-3.5 h-3.5 text-cyan-500" />
                    {:else}
                      <ChevronRight class="w-3.5 h-3.5 text-slate-400" />
                    {/if}
                  </button>
                </td>

                <!-- State Badge (TWSNMP FK style) -->
                <td class="py-1 px-2">
                  <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10px] font-bold uppercase border leading-none {c.status === 'normal' ? 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800/60' : c.status === 'warn' ? 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800/60' : c.status === 'high' ? 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800/60' : 'bg-slate-100 text-slate-600 border-slate-200 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700'}">
                    {#if c.status === "normal"}
                      <CheckCircle2 class="w-3 h-3 text-emerald-500" />
                      <span>Normal</span>
                    {:else if c.status === "warn"}
                      <AlertTriangle class="w-3 h-3 text-amber-500" />
                      <span>Warn</span>
                    {:else if c.status === "high"}
                      <AlertCircle class="w-3 h-3 text-rose-500" />
                      <span>High</span>
                    {:else}
                      <HelpCircle class="w-3 h-3 text-slate-400" />
                      <span>Unknown</span>
                    {/if}
                  </span>
                </td>

                <!-- Target -->
                <td class="py-1 px-2 font-bold font-sans text-[11px] text-slate-900 dark:text-slate-100">
                  {c.target}
                </td>

                <!-- Port -->
                <td class="py-1 px-2 text-[11px] text-slate-600 dark:text-slate-400">
                  {c.port}
                </td>

                <!-- Subject -->
                <td class="py-1 px-2 text-cyan-700 dark:text-cyan-400 text-[11px] truncate max-w-xs" title={c.subject}>
                  {c.subject}
                </td>

                <!-- Issuer -->
                <td class="py-1 px-2 text-slate-700 dark:text-slate-400 font-sans text-[11px] truncate max-w-xs" title={c.issuer}>
                  {c.issuer}
                </td>

                <!-- Start -->
                <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {c.start}
                </td>

                <!-- End -->
                <td class="py-1 px-2 font-semibold text-[11px] whitespace-nowrap {c.days <= 0 ? 'text-rose-500' : c.days <= 30 ? 'text-amber-500' : 'text-emerald-600 dark:text-emerald-400'}">
                  {c.end}
                </td>

                <!-- Last time -->
                <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                  {c.lastTime}
                </td>

                <!-- Row Actions (Icon Buttons) -->
                <td class="py-1 px-2 text-center whitespace-nowrap" onclick={(e) => e.stopPropagation()}>
                  <div class="inline-flex items-center gap-1 justify-center">
                    <button
                      type="button"
                      onclick={() => handleEditItem(c)}
                      class="p-1 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-800 text-cyan-600 dark:text-cyan-400 hover:bg-cyan-50 dark:hover:bg-cyan-950/40 transition-colors cursor-pointer"
                      title={$_("report.btnEditCertPolling")}
                    >
                      <Edit3 class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => handleDeleteItem(c)}
                      class="p-1 rounded-lg border border-rose-200 dark:border-rose-900/60 bg-white dark:bg-slate-800 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 transition-colors cursor-pointer"
                      title={$_("report.btnDeleteCertPolling")}
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onclick={() => handleAiExplainItem(c)}
                      class="p-1 rounded-lg border border-fuchsia-200 dark:border-fuchsia-900/60 bg-white dark:bg-slate-800 text-fuchsia-600 dark:text-fuchsia-400 hover:bg-fuchsia-50 dark:hover:bg-fuchsia-950/40 transition-colors cursor-pointer"
                      title={$_("report.btnAiExplainCert")}
                    >
                      <Sparkles class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>

              <!-- Accordion Detail Row (TWSNMP FK Accordion View) -->
              {#if isExpanded}
                <tr class="bg-blue-900/10 dark:bg-blue-950/40 border-b border-cyan-500/20">
                  <td colspan="10" class="p-4 text-xs font-mono">
                    <div class="rounded-xl border border-cyan-500/30 bg-slate-950/80 p-4 text-slate-200 shadow-inner space-y-1.5 leading-relaxed">
                      <div><span class="text-cyan-400 font-bold">State:</span> <span class="{c.status === 'high' ? 'text-rose-400 font-bold' : c.status === 'warn' ? 'text-amber-400' : 'text-emerald-400'}">{c.status}</span></div>
                      <div><span class="text-cyan-400 font-bold">Target:Port:</span> {c.target}:{c.port}</div>
                      <div><span class="text-cyan-400 font-bold">Subject:</span> {c.subject}</div>
                      <div><span class="text-cyan-400 font-bold">Issuer:</span> {c.issuer}</div>
                      <div><span class="text-cyan-400 font-bold">Serial Number:</span> {c.serialNumber}</div>
                      <div><span class="text-cyan-400 font-bold">Verify:</span> <span class="{c.verify ? 'text-emerald-400' : 'text-rose-400 font-bold'}">{String(c.verify)}</span></div>
                      {#if c.error}
                        <div><span class="text-rose-400 font-bold">Error:</span> <span class="text-rose-300 font-sans">{c.error}</span></div>
                      {/if}
                      <div><span class="text-cyan-400 font-bold">Term:</span> {c.start} - {c.end}</div>
                      <div><span class="text-cyan-400 font-bold">Days:</span> <span class="{c.days <= 0 ? 'text-rose-400 font-bold' : c.days <= 30 ? 'text-amber-400 font-bold' : 'text-emerald-400'}">{c.days}</span></div>
                      <div><span class="text-cyan-400 font-bold">Key / Strength:</span> {c.key} ({c.version})</div>
                      <div class="pt-2 border-t border-slate-800 text-[11px] text-slate-400 flex items-center gap-4 font-sans">
                        <span><strong class="text-slate-300">Node:</strong> {c.nodeName}</span>
                        <span><strong class="text-slate-300">Polling:</strong> {c.pollingName}</span>
                        <button
                          type="button"
                          onclick={() => { editingPolling = c.polling; showPollingModal = true; }}
                          class="ml-auto text-cyan-400 hover:text-cyan-300 underline cursor-pointer"
                        >
                          ポーリング設定を編集
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination Footer -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={sortedCertItems.length}
    />
  </div>

  <!-- Polling Dialog for Add/Edit -->
  {#if showPollingModal}
    <PollingDialog
      bind:show={showPollingModal}
      polling={editingPolling}
      {nodes}
      onSave={async () => {
        showPollingModal = false;
        await onReload?.();
      }}
    />
  {/if}

  <!-- AI Explain Modal -->
  {#if showAiExplainModal && aiExplainTarget}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
      <div class="w-full max-w-lg rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-6 shadow-2xl space-y-4 font-sans text-xs">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-fuchsia-500" />
            AI 証明書解説・診断
          </h3>
          <button
            type="button"
            onclick={() => { showAiExplainModal = false; }}
            class="rounded-lg p-1 text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors"
          >
            ✕
          </button>
        </div>

        <div class="space-y-3 leading-relaxed text-slate-700 dark:text-slate-300">
          <div class="rounded-xl bg-slate-50 dark:bg-slate-950 p-3 font-mono text-[11px] border border-slate-200 dark:border-slate-800 space-y-1">
            <div><strong>対象:</strong> {aiExplainTarget.target}:{aiExplainTarget.port}</div>
            <div><strong>Subject:</strong> {aiExplainTarget.subject}</div>
            <div><strong>Issuer:</strong> {aiExplainTarget.issuer}</div>
            <div><strong>残り日数:</strong> {aiExplainTarget.days} 日 (有効期限: {aiExplainTarget.end})</div>
            <div><strong>ステータス:</strong> <span class="font-bold uppercase">{aiExplainTarget.status}</span></div>
          </div>

          {#if aiExplainTarget.status === 'high'}
            <div class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-rose-600 dark:text-rose-300">
              <p class="font-bold">⚠️ 重度障害または証明書エラーが検出されました</p>
              {#if aiExplainTarget.days <= 0}
                <p class="mt-1">証明書は既に期限切れとなっています。クライアントからのアクセス時にセキュリティ警告が表示されるため、直ちに証明書の更新が必要です。</p>
              {:else if !aiExplainTarget.verify}
                <p class="mt-1">証明書チェーンの検証に失敗したか、ホスト名と証明書Common Name/SANが一致していません ({aiExplainTarget.error})。設定と証明書を確認してください。</p>
              {:else}
                <p class="mt-1">{aiExplainTarget.error || 'TLS接続または証明書の検証に失敗しました。'}</p>
              {/if}
            </div>
          {:else if aiExplainTarget.status === 'warn'}
            <div class="rounded-xl border border-amber-500/30 bg-amber-500/10 p-3 text-amber-700 dark:text-amber-300">
              <p class="font-bold">⚡ 証明書の更新推奨期間に入っています</p>
              <p class="mt-1">有効期限まで残り <strong>{aiExplainTarget.days}日</strong> です。通常、有効期限の30日前までに認証局（Let's Encrypt / 内部CA等）での更新作業とWebサーバーへの反映を行うことが推奨されます。</p>
            </div>
          {:else}
            <div class="rounded-xl border border-emerald-500/30 bg-emerald-500/10 p-3 text-emerald-700 dark:text-emerald-300">
              <p class="font-bold">✅ 証明書は正常です</p>
              <p class="mt-1">証明書チェーンの検証に合格しており、有効期限まで十分な期間（{aiExplainTarget.days}日）があります。暗号強度も {aiExplainTarget.key} で問題ありません。</p>
            </div>
          {/if}
        </div>

        <div class="flex justify-end pt-3 border-t border-slate-200 dark:border-slate-800">
          <button
            type="button"
            onclick={() => { showAiExplainModal = false; }}
            class="rounded-xl bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 px-4 py-2 text-xs font-semibold text-slate-800 dark:text-slate-200 transition-colors cursor-pointer"
          >
            閉じる
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
