<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Terminal,
    KeyRound,
    UserCheck,
    ShieldAlert,
    Cpu,
    CalendarClock,
    Lock,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    AlertCircle,
    ChevronDown,
    ChevronRight,
    Users,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import { formatTimeStr } from "../../common";
  import {
    fetchLogReport,
    resetLogReport,
    type WinEventIDEnt,
    type WinLogonEnt,
    type WinAccountEnt,
    type WinKerberosEnt,
    type WinPrivilegeEnt,
    type WinProcessEnt,
    type WinTaskEnt,
    type NodeEnt,
  } from "../../api";
  import { showConfirm, showAlert } from "../../stores/modalStore";

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

  type TabType =
    | "eventid"
    | "logon"
    | "account"
    | "kerberos"
    | "privilege"
    | "process"
    | "task";

  let activeTab = $state<TabType>("eventid");

  let eventIDs = $state<WinEventIDEnt[]>([]);
  let logons = $state<WinLogonEnt[]>([]);
  let accounts = $state<WinAccountEnt[]>([]);
  let kerberosList = $state<WinKerberosEnt[]>([]);
  let privileges = $state<WinPrivilegeEnt[]>([]);
  let processes = $state<WinProcessEnt[]>([]);
  let tasks = $state<WinTaskEnt[]>([]);

  let internalLoading = $state(false);
  let expandedId = $state<string | null>(null);

  let sortColumn = $state("LastTime");
  let sortDirection = $state<"asc" | "desc">("desc");
  let pageSize = $state(25);
  let currentPage = $state(1);

  const loadAll = async () => {
    internalLoading = true;
    try {
      const [e, l, a, k, p, pr, t] = await Promise.all([
        fetchLogReport<WinEventIDEnt>("winEventID"),
        fetchLogReport<WinLogonEnt>("winLogon"),
        fetchLogReport<WinAccountEnt>("winAccount"),
        fetchLogReport<WinKerberosEnt>("winKerberos"),
        fetchLogReport<WinPrivilegeEnt>("winPrivilege"),
        fetchLogReport<WinProcessEnt>("winProcess"),
        fetchLogReport<WinTaskEnt>("winTask"),
      ]);
      eventIDs = e;
      logons = l;
      accounts = a;
      kerberosList = k;
      privileges = p;
      processes = pr;
      tasks = t;
    } catch (err) {
      console.error("Failed to load Windows reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      eventid: { kind: "winEventID", name: $_("report.winTabEventID") || "Event ID" },
      logon: { kind: "winLogon", name: $_("report.winTabLogon") || "ログオン" },
      account: { kind: "winAccount", name: $_("report.winTabAccount") || "アカウント操作" },
      kerberos: { kind: "winKerberos", name: $_("report.winTabKerberos") || "Kerberos" },
      privilege: { kind: "winPrivilege", name: $_("report.winTabPrivilege") || "特権利用" },
      process: { kind: "winProcess", name: $_("report.winTabProcess") || "プロセス" },
      task: { kind: "winTask", name: $_("report.winTabTask") || "タスク" },
    };
    const target = tabKindMap[activeTab];
    const ok = await showConfirm({
      title: $_('common.confirmClear') || 'データ消去の確認',
      message: $_("report.confirmClearItem", { values: { name: target.name } }) || `${target.name}データを全消去しますか？`,
      type: 'warning',
      confirmText: $_('common.clear') || '消去',
    });
    if (!ok) return;
    try {
      await resetLogReport(target.kind);
      await loadAll();
      onRefresh();
    } catch (err: any) {
      showAlert({
        title: $_('common.error') || 'エラー',
        message: $_("report.alertResetFailed", { values: { error: err.message || err } }),
        type: 'danger',
      });
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
    let res: any[] = [];
    if (activeTab === "eventid") res = eventIDs;
    else if (activeTab === "logon") res = logons;
    else if (activeTab === "account") res = accounts;
    else if (activeTab === "kerberos") res = kerberosList;
    else if (activeTab === "privilege") res = privileges;
    else if (activeTab === "process") res = processes;
    else if (activeTab === "task") res = tasks;

    if (q) {
      res = res.filter((item) => {
        return Object.values(item).some(
          (v) => typeof v === "string" && v.toLowerCase().includes(q)
        );
      });
    }

    return [...res].sort((a, b) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];
      if (typeof valA === "string") {
        const cmp = valA.localeCompare(valB || "");
        return sortDirection === "asc" ? cmp : -cmp;
      }
      valA = Number(valA || 0);
      valB = Number(valB || 0);
      return sortDirection === "asc" ? valA - valB : valB - valA;
    });
  });

  const paginatedList = $derived.by(() => {
    if (pageSize === -1) return currentList;
    const start = (currentPage - 1) * pageSize;
    return currentList.slice(start, start + pageSize);
  });

  export function exportCSV(): void {
    let header = "";
    let rows = "";
    if (activeTab === "eventid") {
      header = "Computer,Provider,EventID,Channel,Level,Count,FirstTime,LastTime\n";
      rows = (currentList as WinEventIDEnt[])
        .map((e) => `"${e.Computer}","${e.Provider}","${e.EventID}","${e.Channel}","${e.Level}","${e.Count}","${formatTimeStr(e.FirstTime)}","${formatTimeStr(e.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "logon") {
      header = "Target,Computer,IP,Count,Logon,Failed,Logoff,Penalty,Score,FirstTime,LastTime\n";
      rows = (currentList as WinLogonEnt[])
        .map((l) => `"${l.Target}","${l.Computer}","${l.IP || ""}","${l.Count}","${l.Logon}","${l.Failed}","${l.Logoff}","${l.Penalty}","${l.Score?.toFixed(1) || ""}","${formatTimeStr(l.FirstTime)}","${formatTimeStr(l.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "account") {
      header = "Target,Computer,Subject,Count,Edit,Password,Other,FirstTime,LastTime\n";
      rows = (currentList as WinAccountEnt[])
        .map((a) => `"${a.Target}","${a.Computer}","${a.Subject}","${a.Count}","${a.Edit}","${a.Password}","${a.Other}","${formatTimeStr(a.FirstTime)}","${formatTimeStr(a.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "kerberos") {
      header = "Target,Computer,IP,Service,TicketType,Count,Failed,Penalty,Score,FirstTime,LastTime\n";
      rows = (currentList as WinKerberosEnt[])
        .map((k) => `"${k.Target}","${k.Computer}","${k.IP || ""}","${k.Service}","${k.TicketType}","${k.Count}","${k.Failed}","${k.Penalty}","${k.Score?.toFixed(1) || ""}","${formatTimeStr(k.FirstTime)}","${formatTimeStr(k.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "privilege") {
      header = "Subject,Computer,Count,FirstTime,LastTime\n";
      rows = (currentList as WinPrivilegeEnt[])
        .map((p) => `"${p.Subject}","${p.Computer}","${p.Count}","${formatTimeStr(p.FirstTime)}","${formatTimeStr(p.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "process") {
      header = "Process,Computer,Count,Start,Exit,LastSubject,LastParent,FirstTime,LastTime\n";
      rows = (currentList as WinProcessEnt[])
        .map((p) => `"${p.Process}","${p.Computer}","${p.Count}","${p.Start}","${p.Exit}","${p.LastSubject || ""}","${p.LastParent || ""}","${formatTimeStr(p.FirstTime)}","${formatTimeStr(p.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "task") {
      header = "TaskName,Computer,Subject,Count,FirstTime,LastTime\n";
      rows = (currentList as WinTaskEnt[])
        .map((t) => `"${t.TaskName}","${t.Computer}","${t.Subject}","${t.Count}","${formatTimeStr(t.FirstTime)}","${formatTimeStr(t.LastTime)}"`)
        .join("\n");
    }

    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_winlog_${activeTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <!-- Sub-tab Selector Pills -->
  <div class="flex flex-wrap items-center gap-2 p-1.5 rounded-2xl bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800">
    <button
      type="button"
      onclick={() => handleTabChange("eventid")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'eventid' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Terminal class="w-3.5 h-3.5" />
      <span>{$_("report.winTabEventID")} ({eventIDs.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("logon")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'logon' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <KeyRound class="w-3.5 h-3.5" />
      <span>{$_("report.winTabLogon")} ({logons.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("account")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'account' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Users class="w-3.5 h-3.5" />
      <span>{$_("report.winTabAccount")} ({accounts.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("kerberos")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'kerberos' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Lock class="w-3.5 h-3.5" />
      <span>{$_("report.winTabKerberos")} ({kerberosList.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("privilege")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'privilege' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <ShieldAlert class="w-3.5 h-3.5" />
      <span>{$_("report.winTabPrivilege")} ({privileges.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("process")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'process' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <Cpu class="w-3.5 h-3.5" />
      <span>{$_("report.winTabProcess")} ({processes.length})</span>
    </button>
    <button
      type="button"
      onclick={() => handleTabChange("task")}
      class="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl text-xs font-semibold transition-all cursor-pointer {activeTab === 'task' ? 'bg-cyan-600 text-white shadow-md' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100'}"
    >
      <CalendarClock class="w-3.5 h-3.5" />
      <span>{$_("report.winTabTask")} ({tasks.length})</span>
    </button>
  </div>

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Terminal class="w-4 h-4 text-cyan-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {#if activeTab === "eventid"}{$_("report.winTitleEventID")}
          {:else if activeTab === "logon"}{$_("report.winTitleLogon")}
          {:else if activeTab === "account"}{$_("report.winTitleAccount")}
          {:else if activeTab === "kerberos"}{$_("report.winTitleKerberos")}
          {:else if activeTab === "privilege"}{$_("report.winTitlePrivilege")}
          {:else if activeTab === "process"}{$_("report.winTitleProcess")}
          {:else}{$_("report.winTitleTask")}{/if}
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
            {#if activeTab === "eventid"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Provider")}>{$_("report.winColProvider")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("EventID")}>{$_("report.winColEventID")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Channel")}>{$_("report.winColChannel")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Level")}>{$_("report.winColLevel")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "logon"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Target")}>{$_("report.winColTargetAccount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("IP")}>{$_("report.winColSrcIp")}</th>
              <th class="py-2.5 px-3 text-right font-bold text-emerald-500">{$_("report.winColLogon")}</th>
              <th class="py-2.5 px-3 text-right font-bold text-rose-500">{$_("report.winColFailed")}</th>
              <th class="py-2.5 px-3 text-right">{$_("report.winColLogoff")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Score")}>{$_("report.winColTrustScore")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "account"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Target")}>{$_("report.winColTarget")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Subject")}>{$_("report.winColSubject")}</th>
              <th class="py-2.5 px-3 text-right">{$_("report.winColEditPassOther")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColTotalCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "kerberos"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Target")}>{$_("report.winColUser")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Service")}>{$_("report.winColService")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("TicketType")}>{$_("report.winColTicketType")}</th>
              <th class="py-2.5 px-3 text-right font-bold text-rose-500">{$_("report.winColFailed")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Score")}>{$_("report.winColTrustScore")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "privilege"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Subject")}>{$_("report.winColPrivUser")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColPrivCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "process"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Process")}>{$_("report.winColProcessName")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastSubject")}>{$_("report.winColExecutor")}</th>
              <th class="py-2.5 px-3">{$_("report.winColParentProcess")}</th>
              <th class="py-2.5 px-3 text-right">{$_("report.winColStartExit")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColTimes")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
            {:else if activeTab === "task"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("TaskName")}>{$_("report.winColTaskName")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Computer")}>{$_("report.winColComputer")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Subject")}>{$_("report.winColExecAccount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>{$_("report.winColExecCount")}</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>{$_("report.winColLastTime")}</th>
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
                <td class="py-1 px-2 text-center text-slate-400">
                  {#if isExpanded}
                    <ChevronDown class="w-4 h-4 text-cyan-500" />
                  {:else}
                    <ChevronRight class="w-4 h-4" />
                  {/if}
                </td>

                {#if activeTab === "eventid"}
                  {@const e = item as WinEventIDEnt}
                  <td class="py-1 px-2 font-sans text-slate-800 dark:text-slate-200">{e.Computer}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{e.Provider}</td>
                  <td class="py-1 px-2 font-bold text-cyan-600 dark:text-cyan-400">{e.EventID}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px]">{e.Channel}</td>
                  <td class="py-1 px-2">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {e.Level === 'error' ? 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400' : e.Level === 'warn' ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400' : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'}">
                      {e.Level}
                    </span>
                  </td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{e.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(e.LastTime)}</td>

                {:else if activeTab === "logon"}
                  {@const l = item as WinLogonEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">{l.Target}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{l.Computer}</td>
                  <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 text-[11px]">{l.IP || "-"}</td>
                  <td class="py-1 px-2 text-right font-bold text-emerald-500">{l.Logon.toLocaleString()}</td>
                  <td class="py-1 px-2 text-right font-bold text-rose-500">{l.Failed.toLocaleString()}</td>
                  <td class="py-1 px-2 text-right text-slate-500">{l.Logoff.toLocaleString()}</td>
                  <td class="py-1 px-2 text-right">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {l.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {l.Score?.toFixed(1) ?? "-"} (P:{l.Penalty})
                    </span>
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(l.LastTime)}</td>

                {:else if activeTab === "account"}
                  {@const a = item as WinAccountEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">{a.Target}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{a.Computer}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{a.Subject}</td>
                  <td class="py-1 px-2 text-right text-[11px] text-slate-500">{a.Edit} / {a.Password} / {a.Other}</td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{a.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(a.LastTime)}</td>

                {:else if activeTab === "kerberos"}
                  {@const k = item as WinKerberosEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">{k.Target}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{k.Computer}</td>
                  <td class="py-1 px-2 text-cyan-600 dark:text-cyan-400 text-[11px]">{k.Service}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{k.TicketType}</td>
                  <td class="py-1 px-2 text-right font-bold text-rose-500">{k.Failed.toLocaleString()}</td>
                  <td class="py-1 px-2 text-right">
                    <span class="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold {k.Score >= 50 ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400'}">
                      {k.Score?.toFixed(1) ?? "-"} (P:{k.Penalty})
                    </span>
                  </td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(k.LastTime)}</td>

                {:else if activeTab === "privilege"}
                  {@const p = item as WinPrivilegeEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">{p.Subject}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{p.Computer}</td>
                  <td class="py-1 px-2 text-right font-bold text-amber-500">{p.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(p.LastTime)}</td>

                {:else if activeTab === "process"}
                  {@const pr = item as WinProcessEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200 truncate max-w-[200px]" title={pr.Process}>{pr.Process}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{pr.Computer}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{pr.LastSubject || "-"}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] truncate max-w-[120px]" title={pr.LastParent}>{pr.LastParent || "-"}</td>
                  <td class="py-1 px-2 text-right text-[11px] text-slate-500">{pr.Start} / {pr.Exit}</td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{pr.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(pr.LastTime)}</td>

                {:else if activeTab === "task"}
                  {@const t = item as WinTaskEnt}
                  <td class="py-1 px-2 font-semibold font-sans text-slate-800 dark:text-slate-200">{t.TaskName}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{t.Computer}</td>
                  <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">{t.Subject}</td>
                  <td class="py-1 px-2 text-right font-bold text-slate-700 dark:text-slate-300">{t.Count.toLocaleString()}</td>
                  <td class="py-1 px-2 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">{formatTimeStr(t.LastTime)}</td>
                {/if}
              </tr>

              {#if isExpanded}
                <tr class="bg-slate-50 dark:bg-slate-950/60 font-sans">
                  <td colspan="9" class="p-4 space-y-3">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.winDetailId")}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px] break-all">{item.ID}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.winDetailFirstSeen")}</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr((item as any).FirstTime)}</div>
                      </div>
                      {#if activeTab === "logon" && (item as WinLogonEnt).LogonType}
                        <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                          <div class="text-[11px] font-bold text-slate-400 uppercase">{$_("report.winDetailLogonType")}</div>
                          <div class="font-mono text-slate-800 dark:text-slate-200 text-[11px]">
                            {JSON.stringify((item as WinLogonEnt).LogonType)}
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
</div>
