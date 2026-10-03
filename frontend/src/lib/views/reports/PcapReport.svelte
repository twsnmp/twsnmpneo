<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Activity,
    Network,
    Globe,
    Shield,
    Lock,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    CheckCircle2,
    AlertTriangle,
    XCircle,
    ChevronDown,
    ChevronRight,
    Server,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import { formatTimeStr } from "../../common";
  import {
    fetchLogReport,
    resetLogReport,
    type EtherTypeEnt,
    type DNSQEnt,
    type RADIUSFlowEnt,
    type TLSFlowEnt,
    type PollingEnt,
    type NodeEnt,
  } from "../../api";

  let {
    searchQuery = "",
    nodes = [],
    onRefresh = () => {},
    loading = false,
  }: {
    searchQuery?: string;
    nodes?: NodeEnt[];
    onRefresh?: () => void;
    loading?: boolean;
  } = $props();

  type TabType = "ether" | "dns" | "radius" | "tls";
  let activeTab = $state<TabType>("ether");

  let ethers = $state<EtherTypeEnt[]>([]);
  let dnsList = $state<DNSQEnt[]>([]);
  let radiusFlows = $state<RADIUSFlowEnt[]>([]);
  let tlsFlows = $state<TLSFlowEnt[]>([]);
  let internalLoading = $state(false);
  let expandedId = $state<string | null>(null);
  let showPollingModal = $state(false);
  let editingPolling = $state<PollingEnt | null>(null);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  const loadAll = async () => {
    internalLoading = true;
    try {
      const [e, d, r, t] = await Promise.all([
        fetchLogReport<EtherTypeEnt>("etherType"),
        fetchLogReport<DNSQEnt>("dnsq"),
        fetchLogReport<RADIUSFlowEnt>("radiusFlow"),
        fetchLogReport<TLSFlowEnt>("tlsFlow"),
      ]);
      ethers = e;
      dnsList = d;
      radiusFlows = r;
      tlsFlows = t;
    } catch (err) {
      console.error("Failed to load Pcap reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      ether: { kind: "etherType", name: "EtherType" },
      dns: { kind: "dnsq", name: "DNSクエリ" },
      radius: { kind: "radiusFlow", name: "RADIUSフロー" },
      tls: { kind: "tlsFlow", name: "TLSフロー" },
    };
    const target = tabKindMap[activeTab];
    if (!confirm($_("report.confirmClearItem", { values: { name: target.name } }) || `${target.name}データを全消去しますか？`)) return;
    try {
      await resetLogReport(target.kind);
      await loadAll();
      onRefresh();
    } catch (err: any) {
      alert($_("report.alertResetFailed", { values: { error: err.message || err } }));
    }
  }

  onMount(() => {
    loadAll();
  });

  const handleTabChange = (tab: TabType) => {
    activeTab = tab;
    expandedId = null;
    currentPage = 1;
    sortColumn = "LastTime";
    sortDirection = "desc";
  };

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const currentList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (activeTab === "ether") {
      let res = ethers;
      if (q) {
        res = res.filter(
          (e) =>
            e.Name?.toLowerCase().includes(q) ||
            e.Type?.toLowerCase().includes(q) ||
            e.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "dns") {
      let res = dnsList;
      if (q) {
        res = res.filter(
          (d) =>
            d.Name?.toLowerCase().includes(q) ||
            d.Server?.toLowerCase().includes(q) ||
            d.Type?.toLowerCase().includes(q) ||
            d.LastClient?.toLowerCase().includes(q) ||
            d.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "radius") {
      let res = radiusFlows;
      if (q) {
        res = res.filter(
          (r) =>
            r.Client?.toLowerCase().includes(q) ||
            r.ClientName?.toLowerCase().includes(q) ||
            r.Server?.toLowerCase().includes(q) ||
            r.ServerName?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "tls") {
      let res = tlsFlows;
      if (q) {
        res = res.filter(
          (t) =>
            t.Client?.toLowerCase().includes(q) ||
            t.ClientName?.toLowerCase().includes(q) ||
            t.Server?.toLowerCase().includes(q) ||
            t.ServerName?.toLowerCase().includes(q) ||
            t.Service?.toLowerCase().includes(q) ||
            t.Version?.toLowerCase().includes(q) ||
            t.Cipher?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }
    return [];
  });

  const paginatedList = $derived.by(() => {
    if (pageSize === -1) return currentList;
    const start = (currentPage - 1) * pageSize;
    return currentList.slice(start, start + pageSize);
  });

  export function handleAddNewPolling(): void {
    editingPolling = {
      id: "",
      name: "twpcap",
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "syslog",
      mode: "twpcap",
      params: "",
      filter: "",
      extractor: "",
      script: "",
      level: "off",
      poll_int: 300,
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

  export function exportCSV(): void {
    let header = "";
    let rows = "";
    if (activeTab === "ether") {
      header = "Host,Type,Name,Count,FirstTime,LastTime\n";
      rows = (currentList as EtherTypeEnt[])
        .map((e) => `"${e.Host}","${e.Type}","${e.Name}","${e.Count}","${formatTimeStr(e.FirstTime)}","${formatTimeStr(e.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "dns") {
      header = "Host,Type,Server,Name,Count,Change,LastClient,LastMAC,FirstTime,LastTime\n";
      rows = (currentList as DNSQEnt[])
        .map((d) => `"${d.Host}","${d.Type}","${d.Server}","${d.Name}","${d.Count}","${d.Change}","${d.LastClient}","${d.LastMAC || ""}","${formatTimeStr(d.FirstTime)}","${formatTimeStr(d.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "radius") {
      header = "Client,ClientName,Server,ServerName,Accept,Reject,Request,Challenge,Count,Penalty,Score,LastTime\n";
      rows = (currentList as RADIUSFlowEnt[])
        .map((r) => `"${r.Client}","${r.ClientName || ""}","${r.Server}","${r.ServerName || ""}","${r.Accept}","${r.Reject}","${r.Request}","${r.Challenge}","${r.Count}","${r.Penalty}","${r.Score?.toFixed(1) || ""}","${formatTimeStr(r.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "tls") {
      header = "Client,ClientName,Server,ServerName,Service,Version,Cipher,Count,Penalty,Score,LastTime\n";
      rows = (currentList as TLSFlowEnt[])
        .map((t) => `"${t.Client}","${t.ClientName || ""}","${t.Server}","${t.ServerName || ""}","${t.Service}","${t.Version}","${t.Cipher}","${t.Count}","${t.Penalty}","${t.Score?.toFixed(1) || ""}","${formatTimeStr(t.LastTime)}"`)
        .join("\n");
    }

    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_pcap_${activeTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <!-- Summary Cards -->
  <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
    <button
      type="button"
      onclick={() => handleTabChange("ether")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'ether' ? 'border-cyan-500 bg-cyan-50/50 dark:bg-cyan-950/30 ring-2 ring-cyan-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/50 border border-cyan-200 dark:border-cyan-800/60 text-cyan-600 dark:text-cyan-400">
        <Network class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          EtherType フレーム
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {ethers.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("dns")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'dns' ? 'border-emerald-500 bg-emerald-50/50 dark:bg-emerald-950/30 ring-2 ring-emerald-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/50 border border-emerald-200 dark:border-emerald-800/60 text-emerald-600 dark:text-emerald-400">
        <Globe class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          DNS クエリ集計
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {dnsList.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("radius")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'radius' ? 'border-purple-500 bg-purple-50/50 dark:bg-purple-950/30 ring-2 ring-purple-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-purple-50 dark:bg-purple-950/50 border border-purple-200 dark:border-purple-800/60 text-purple-600 dark:text-purple-400">
        <Shield class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          RADIUS フロー
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {radiusFlows.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("tls")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'tls' ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/30 ring-2 ring-amber-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/50 border border-amber-200 dark:border-amber-800/60 text-amber-600 dark:text-amber-400">
        <Lock class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          TLS フロー
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {tlsFlows.length.toLocaleString()}
        </div>
      </div>
    </button>
  </div>

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Activity class="w-4 h-4 text-cyan-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {#if activeTab === "ether"}EtherType プロトコル別集計
          {:else if activeTab === "dns"}DNS クエリ / 応答集計
          {:else if activeTab === "radius"}RADIUS 認証フロー分析
          {:else}TLS 暗号通信フロー分析{/if}
        </span>
        <span class="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 font-mono text-slate-600 dark:text-slate-300">
          {currentList.length}
        </span>
      </div>
    </div>

    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs border-collapse font-sans">
        <thead class="bg-slate-50 dark:bg-slate-950/80 border-b border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400 font-semibold select-none">
          <tr>
            <th class="py-2.5 px-3 w-8"></th>
            {#if activeTab === "ether"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Type")}>Type (Hex)</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>プロトコル名</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>パケット数</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Host")}>キャプチャ元</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終確認</th>
            {:else if activeTab === "dns"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>クエリ名 (FQDN)</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Type")}>レコード種別</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>問い合わせ先DNS</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>回数</th>
              <th class="py-2.5 px-3 text-right">応答変更</th>
              <th class="py-2.5 px-3">最終クライアント</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終確認</th>
            {:else if activeTab === "radius"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Client")}>RADIUS クライアント</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>RADIUS サーバー</th>
              <th class="py-2.5 px-3 text-right font-bold text-emerald-500">Accept</th>
              <th class="py-2.5 px-3 text-right font-bold text-rose-500">Reject</th>
              <th class="py-2.5 px-3 text-right">Req / Challenge</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Score")}>信用スコア</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終確認</th>
            {:else if activeTab === "tls"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Client")}>クライアント</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Server")}>サーバー</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Service")}>サービス</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Version")}>TLSバージョン</th>
              <th class="py-2.5 px-3">暗号スイート</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Score")}>信用スコア</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終確認</th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 font-mono">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="9" class="py-8 text-center text-slate-400 font-sans">
                {internalLoading ? $_("report.loading") : $_("report.noDataFound")}
              </td>
            </tr>
          {:else}
            {#each paginatedList as item}
              {@const isExpanded = expandedId === item.ID}
              <tr
                class="hover:bg-cyan-50/50 dark:hover:bg-cyan-950/20 transition-colors cursor-pointer {isExpanded ? 'bg-cyan-50/30 dark:bg-cyan-950/10' : ''}"
                onclick={() => (expandedId = isExpanded ? null : item.ID)}
              >
                <td class="py-2 px-3 text-center text-slate-400">
                  {#if isExpanded}
                    <ChevronDown class="w-4 h-4 text-cyan-500" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </td>

                {#if activeTab === "ether"}
                  {@const e = item as EtherTypeEnt}
                  <td class="py-2 px-3 font-mono text-cyan-600 dark:text-cyan-400 font-semibold">
                    {e.Type}
                  </td>
                  <td class="py-2 px-3 font-sans font-semibold text-slate-800 dark:text-slate-200">
                    {e.Name}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-slate-700 dark:text-slate-300">
                    {e.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-slate-600 dark:text-slate-400 text-[11px]">
                    {e.Host}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(e.LastTime)}
                  </td>
                {:else if activeTab === "dns"}
                  {@const d = item as DNSQEnt}
                  <td class="py-2 px-3 font-sans font-semibold text-slate-800 dark:text-slate-200 truncate max-w-[200px]" title={d.Name}>
                    {d.Name}
                  </td>
                  <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 font-bold">
                    {d.Type}
                  </td>
                  <td class="py-2 px-3 text-slate-600 dark:text-slate-400 text-[11px]">
                    {d.Server}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-slate-700 dark:text-slate-300">
                    {d.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-right text-slate-500 dark:text-slate-400">
                    {d.Change > 0 ? `Δ ${d.Change}` : "-"}
                  </td>
                  <td class="py-2 px-3 text-[11px] text-slate-600 dark:text-slate-400">
                    {d.LastClient}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(d.LastTime)}
                  </td>
                {:else if activeTab === "radius"}
                  {@const r = item as RADIUSFlowEnt}
                  <td class="py-2 px-3 text-slate-800 dark:text-slate-200 font-sans">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{r.Client}</span>
                    {#if r.ClientName && r.ClientName !== r.Client}
                      <span class="ml-1 text-[11px] text-slate-500">({r.ClientName})</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-slate-800 dark:text-slate-200 font-sans">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{r.Server}</span>
                    {#if r.ServerName && r.ServerName !== r.Server}
                      <span class="ml-1 text-[11px] text-slate-500">({r.ServerName})</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-emerald-500">
                    {r.Accept.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-rose-500">
                    {r.Reject.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-right text-slate-500 dark:text-slate-400 text-[11px]">
                    {r.Request} / {r.Challenge}
                  </td>
                  <td class="py-2 px-3 text-right">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {r.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {r.Score?.toFixed(1) ?? "-"} (P:{r.Penalty})
                    </span>
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(r.LastTime)}
                  </td>
                {:else if activeTab === "tls"}
                  {@const t = item as TLSFlowEnt}
                  <td class="py-2 px-3 font-sans text-slate-800 dark:text-slate-200">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{t.Client}</span>
                    {#if t.ClientName && t.ClientName !== t.Client}
                      <span class="ml-1 text-[11px] text-slate-500">({t.ClientName})</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 font-sans text-slate-800 dark:text-slate-200">
                    <span class="font-mono text-cyan-600 dark:text-cyan-400">{t.Server}</span>
                    {#if t.ServerName && t.ServerName !== t.Server}
                      <span class="ml-1 text-[11px] text-slate-500">({t.ServerName})</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 font-semibold text-slate-700 dark:text-slate-300">
                    {t.Service}
                  </td>
                  <td class="py-2 px-3 text-[11px] {t.Version.includes('1.3') ? 'text-emerald-500 font-bold' : t.Version.includes('1.2') ? 'text-cyan-500' : 'text-rose-500 font-bold'}">
                    {t.Version || "-"}
                  </td>
                  <td class="py-2 px-3 font-mono text-[11px] text-slate-600 dark:text-slate-400 truncate max-w-[150px]" title={t.Cipher}>
                    {t.Cipher || "-"}
                  </td>
                  <td class="py-2 px-3 text-right">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {t.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {t.Score?.toFixed(1) ?? "-"} (P:{t.Penalty})
                    </span>
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(t.LastTime)}
                  </td>
                {/if}
              </tr>

              {#if isExpanded}
                <tr class="bg-slate-50 dark:bg-slate-950/60 font-sans">
                  <td colspan="9" class="p-4 space-y-3">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">識別ID</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px] break-all">{item.ID}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">初回検知日時</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr((item as any).FirstTime)}</div>
                      </div>
                      {#if activeTab === "tls"}
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">GeoIP ロケーション</div>
                          <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px]">
                            Server: {(item as TLSFlowEnt).ServerLoc || "LOCAL"} / Client: {(item as TLSFlowEnt).ClientLoc || "LOCAL"}
                          </div>
                        </div>
                      {/if}
                    </div>
                  </td>
                </tr>
              {/if}
            {/each}
          {/if}
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <ReportPagination
      bind:pageSize
      bind:currentPage
      totalCount={currentList.length}
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
        onRefresh();
      }}
    />
  {/if}
</div>
