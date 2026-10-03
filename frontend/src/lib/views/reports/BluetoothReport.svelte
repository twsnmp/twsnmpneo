<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Bluetooth,
    Thermometer,
    Zap,
    Footprints,
    Trash2,
    Plus,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Signal,
    Battery,
    Sun,
    Gauge,
    Volume2,
    Wind,
    ToggleLeft,
    ToggleRight,
    AlertCircle,
    ChevronDown,
    ChevronRight,
  } from "@lucide/svelte";
  import ReportPagination from "./components/ReportPagination.svelte";
  import PollingDialog from "../../components/PollingDialog.svelte";
  import { formatTimeStr } from "../../common";
  import {
    fetchLogReport,
    resetLogReport,
    type BlueDeviceEnt,
    type EnvMonitorEnt,
    type PowerMonitorEnt,
    type MotionSensorEnt,
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

  type TabType = "device" | "env" | "power" | "motion";
  let activeTab = $state<TabType>("device");

  let devices = $state<BlueDeviceEnt[]>([]);
  let envs = $state<EnvMonitorEnt[]>([]);
  let powers = $state<PowerMonitorEnt[]>([]);
  let motions = $state<MotionSensorEnt[]>([]);
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
      const [d, e, p, m] = await Promise.all([
        fetchLogReport<BlueDeviceEnt>("blueDevice"),
        fetchLogReport<EnvMonitorEnt>("envMonitor"),
        fetchLogReport<PowerMonitorEnt>("powerMonitor"),
        fetchLogReport<MotionSensorEnt>("motionSensor"),
      ]);
      devices = d;
      envs = e;
      powers = p;
      motions = m;
    } catch (err) {
      console.error("Failed to load Bluetooth reports:", err);
    } finally {
      internalLoading = false;
    }
  };

  export const refresh = () => {
    loadAll();
  };

  export async function handleClear(): Promise<void> {
    const tabKindMap: Record<TabType, { kind: string; name: string }> = {
      device: { kind: "blueDevice", name: "Bluetoothデバイス" },
      env: { kind: "envMonitor", name: "環境センサー" },
      power: { kind: "powerMonitor", name: "電力モニター" },
      motion: { kind: "motionSensor", name: "動体センサー" },
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

  const getLatestRSSI = (rssiList?: { Value: number; Time: number }[]): number => {
    if (rssiList && rssiList.length > 0) {
      return rssiList[rssiList.length - 1].Value;
    }
    return -100;
  };

  // Filtered & Sorted items per tab
  const currentList = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (activeTab === "device") {
      let res = devices;
      if (q) {
        res = res.filter(
          (d) =>
            d.Name?.toLowerCase().includes(q) ||
            d.Address?.toLowerCase().includes(q) ||
            d.Vendor?.toLowerCase().includes(q) ||
            d.Host?.toLowerCase().includes(q) ||
            d.AddressType?.toLowerCase().includes(q) ||
            d.Info?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (sortColumn === "RSSI") {
          valA = getLatestRSSI(a.RSSI);
          valB = getLatestRSSI(b.RSSI);
        }
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "env") {
      let res = envs;
      if (q) {
        res = res.filter(
          (e) =>
            e.Name?.toLowerCase().includes(q) ||
            e.Address?.toLowerCase().includes(q) ||
            e.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (sortColumn === "Temp") {
          valA = a.EnvData?.[a.EnvData.length - 1]?.Temp ?? -999;
          valB = b.EnvData?.[b.EnvData.length - 1]?.Temp ?? -999;
        } else if (sortColumn === "Humidity") {
          valA = a.EnvData?.[a.EnvData.length - 1]?.Humidity ?? -999;
          valB = b.EnvData?.[b.EnvData.length - 1]?.Humidity ?? -999;
        }
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "power") {
      let res = powers;
      if (q) {
        res = res.filter(
          (p) =>
            p.Name?.toLowerCase().includes(q) ||
            p.Address?.toLowerCase().includes(q) ||
            p.Host?.toLowerCase().includes(q)
        );
      }
      return [...res].sort((a, b) => {
        let valA: any = (a as any)[sortColumn];
        let valB: any = (b as any)[sortColumn];
        if (sortColumn === "Load") {
          valA = a.Data?.[a.Data.length - 1]?.Load ?? 0;
          valB = b.Data?.[b.Data.length - 1]?.Load ?? 0;
        }
        if (typeof valA === "string") {
          const cmp = valA.localeCompare(valB || "");
          return sortDirection === "asc" ? cmp : -cmp;
        }
        valA = Number(valA || 0);
        valB = Number(valB || 0);
        return sortDirection === "asc" ? valA - valB : valB - valA;
      });
    }

    if (activeTab === "motion") {
      let res = motions;
      if (q) {
        res = res.filter(
          (m) =>
            m.Name?.toLowerCase().includes(q) ||
            m.Address?.toLowerCase().includes(q) ||
            m.Host?.toLowerCase().includes(q)
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
      name: "twBlueScan",
      node_id: nodes[0]?.id || (nodes[0] as any)?.ID || "",
      type: "syslog",
      mode: "twbluescan",
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
    if (activeTab === "device") {
      header = "Host,Address,Name,AddressType,Vendor,LatestRSSI,Count,Info,FirstTime,LastTime\n";
      rows = (currentList as BlueDeviceEnt[])
        .map((d) => `"${d.Host}","${d.Address}","${d.Name || ""}","${d.AddressType || ""}","${d.Vendor || ""}","${getLatestRSSI(d.RSSI)}","${d.Count}","${d.Info || ""}","${formatTimeStr(d.FirstTime)}","${formatTimeStr(d.LastTime)}"`)
        .join("\n");
    } else if (activeTab === "env") {
      header = "Host,Address,Name,Temp,Humidity,Pressure,Illuminance,Sound,CO2,Battery,Count,FirstTime,LastTime\n";
      rows = (currentList as EnvMonitorEnt[])
        .map((e) => {
          const l = e.EnvData?.[e.EnvData.length - 1];
          return `"${e.Host}","${e.Address}","${e.Name || ""}","${l?.Temp ?? ""}","${l?.Humidity ?? ""}","${l?.BarometricPressure ?? ""}","${l?.Illuminance ?? ""}","${l?.Sound ?? ""}","${l?.ECo2 ?? ""}","${l?.Battery ?? ""}","${e.Count}","${formatTimeStr(e.FirstTime)}","${formatTimeStr(e.LastTime)}"`;
        })
        .join("\n");
    } else if (activeTab === "power") {
      header = "Host,Address,Name,Load(W),Switch,Overload,RSSI,Count,FirstTime,LastTime\n";
      rows = (currentList as PowerMonitorEnt[])
        .map((p) => {
          const l = p.Data?.[p.Data.length - 1];
          return `"${p.Host}","${p.Address}","${p.Name || ""}","${l?.Load ?? ""}","${l?.Switch ?? ""}","${l?.Over ?? ""}","${l?.RSSI ?? ""}","${p.Count}","${formatTimeStr(p.FirstTime)}","${formatTimeStr(p.LastTime)}"`;
        })
        .join("\n");
    } else if (activeTab === "motion") {
      header = "Host,Address,Name,Moving,Light,Battery,LastMove,LastMoveDiff(s),RSSI,Count,FirstTime,LastTime\n";
      rows = (currentList as MotionSensorEnt[])
        .map((m) => {
          const l = m.Data?.[m.Data.length - 1];
          return `"${m.Host}","${m.Address}","${m.Name || ""}","${l?.Moving ?? ""}","${l?.Light ?? ""}","${l?.Battery ?? ""}","${formatTimeStr(l?.LastMove)}","${l?.LastMoveDiff ?? ""}","${l?.RSSI ?? ""}","${m.Count}","${formatTimeStr(m.FirstTime)}","${formatTimeStr(m.LastTime)}"`;
        })
        .join("\n");
    }

    const blob = new Blob([header + rows], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_bluetooth_${activeTab}_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <!-- Summary Cards -->
  <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
    <button
      type="button"
      onclick={() => handleTabChange("device")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'device' ? 'border-cyan-500 bg-cyan-50/50 dark:bg-cyan-950/30 ring-2 ring-cyan-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/50 border border-cyan-200 dark:border-cyan-800/60 text-cyan-600 dark:text-cyan-400">
        <Bluetooth class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          BLE デバイス
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {devices.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("env")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'env' ? 'border-emerald-500 bg-emerald-50/50 dark:bg-emerald-950/30 ring-2 ring-emerald-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/50 border border-emerald-200 dark:border-emerald-800/60 text-emerald-600 dark:text-emerald-400">
        <Thermometer class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          環境センサー
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {envs.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("power")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'power' ? 'border-amber-500 bg-amber-50/50 dark:bg-amber-950/30 ring-2 ring-amber-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-amber-50 dark:bg-amber-950/50 border border-amber-200 dark:border-amber-800/60 text-amber-600 dark:text-amber-400">
        <Zap class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          スマートプラグ
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {powers.length.toLocaleString()}
        </div>
      </div>
    </button>

    <button
      type="button"
      onclick={() => handleTabChange("motion")}
      class="rounded-2xl border p-4 shadow-sm text-left transition-all cursor-pointer flex items-center gap-3 {activeTab === 'motion' ? 'border-purple-500 bg-purple-50/50 dark:bg-purple-950/30 ring-2 ring-purple-500/30' : 'border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90'}"
    >
      <div class="p-2.5 rounded-xl bg-purple-50 dark:bg-purple-950/50 border border-purple-200 dark:border-purple-800/60 text-purple-600 dark:text-purple-400">
        <Footprints class="w-5 h-5" />
      </div>
      <div>
        <div class="text-[11px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
          人感センサー
        </div>
        <div class="text-xl font-bold font-mono text-slate-800 dark:text-slate-100">
          {motions.length.toLocaleString()}
        </div>
      </div>
    </button>
  </div>

  <!-- Main Table Card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm overflow-hidden">
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Bluetooth class="w-4 h-4 text-cyan-500" />
        <span class="text-sm font-bold text-slate-800 dark:text-slate-100">
          {#if activeTab === "device"}BLE デバイス一覧
          {:else if activeTab === "env"}環境センサーデータ一覧
          {:else if activeTab === "power"}スマートプラグ電力監視一覧
          {:else}人感センサー検知一覧{/if}
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
            {#if activeTab === "device"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>名前 / デバイス</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Address")}>Bluetooth アドレス</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("AddressType")}>種別</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Vendor")}>ベンダー</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("RSSI")}>RSSI</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>回数</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終確認</th>
            {:else if activeTab === "env"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>センサー名</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Address")}>アドレス</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Temp")}>温度 (°C)</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Humidity")}>湿度 (%)</th>
              <th class="py-2.5 px-3 text-right">照度 / 気圧</th>
              <th class="py-2.5 px-3 text-right">CO2 / バッテリー</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>サンプル数</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終更新</th>
            {:else if activeTab === "power"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>プラグ名</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Address")}>アドレス</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Load")}>負荷電力 (W)</th>
              <th class="py-2.5 px-3 text-center">スイッチ状態</th>
              <th class="py-2.5 px-3 text-center">過負荷警告</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>記録件数</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終更新</th>
            {:else if activeTab === "motion"}
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Name")}>センサー名</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("Address")}>アドレス</th>
              <th class="py-2.5 px-3 text-center">動体検知状態</th>
              <th class="py-2.5 px-3 text-center">照度 / バッテリー</th>
              <th class="py-2.5 px-3">最終検知時間</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600 text-right" onclick={() => handleSort("Count")}>記録件数</th>
              <th class="py-2.5 px-3 cursor-pointer hover:text-cyan-600" onclick={() => handleSort("LastTime")}>最終更新</th>
            {/if}
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 font-mono">
          {#if paginatedList.length === 0}
            <tr>
              <td colspan="10" class="py-8 text-center text-slate-400 font-sans">
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

                {#if activeTab === "device"}
                  {@const d = item as BlueDeviceEnt}
                  {@const rssi = getLatestRSSI(d.RSSI)}
                  <td class="py-2 px-3 font-semibold font-sans text-slate-800 dark:text-slate-200">
                    {d.Name || "(Unknown BLE)"}
                  </td>
                  <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 text-[11px] font-mono">
                    {d.Address}
                  </td>
                  <td class="py-2 px-3 font-sans text-[11px] text-slate-600 dark:text-slate-400">
                    {d.AddressType || "Public"}
                  </td>
                  <td class="py-2 px-3 font-sans text-slate-600 dark:text-slate-400 text-[11px] truncate max-w-[140px]" title={d.Vendor}>
                    {d.Vendor || "-"}
                  </td>
                  <td class="py-2 px-3 text-right">
                    <span class="inline-flex items-center gap-1 font-bold {rssi > -65 ? 'text-emerald-500' : rssi > -80 ? 'text-amber-500' : 'text-rose-500'}">
                      <Signal class="w-3 h-3" />
                      {rssi} dBm
                    </span>
                  </td>
                  <td class="py-2 px-3 text-right text-slate-700 dark:text-slate-300 font-bold">
                    {d.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(d.LastTime)}
                  </td>
                {:else if activeTab === "env"}
                  {@const e = item as EnvMonitorEnt}
                  {@const l = e.EnvData?.[e.EnvData.length - 1]}
                  <td class="py-2 px-3 font-semibold font-sans text-slate-800 dark:text-slate-200">
                    {e.Name || "環境センサー"}
                  </td>
                  <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 text-[11px]">
                    {e.Address}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-amber-500">
                    {l?.Temp !== undefined ? `${l.Temp.toFixed(1)} °C` : "-"}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-cyan-500">
                    {l?.Humidity !== undefined ? `${l.Humidity.toFixed(1)} %` : "-"}
                  </td>
                  <td class="py-2 px-3 text-right text-[11px] text-slate-600 dark:text-slate-400">
                    {l?.Illuminance !== undefined ? `${l.Illuminance} lx` : ""}
                    {l?.BarometricPressure ? ` / ${l.BarometricPressure.toFixed(0)} hPa` : ""}
                  </td>
                  <td class="py-2 px-3 text-right text-[11px]">
                    {#if l?.ECo2}
                      <span class="text-emerald-500 font-bold">{l.ECo2} ppm</span>
                    {/if}
                    {#if l?.Battery !== undefined && l?.Battery > 0}
                      <span class="ml-1 text-slate-500">({l.Battery}%)</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-slate-700 dark:text-slate-300">
                    {e.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(e.LastTime)}
                  </td>
                {:else if activeTab === "power"}
                  {@const p = item as PowerMonitorEnt}
                  {@const l = p.Data?.[p.Data.length - 1]}
                  <td class="py-2 px-3 font-semibold font-sans text-slate-800 dark:text-slate-200">
                    {p.Name || "スマートプラグ"}
                  </td>
                  <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 text-[11px]">
                    {p.Address}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-base text-amber-500">
                    {l?.Load !== undefined ? `${l.Load.toFixed(1)} W` : "-"}
                  </td>
                  <td class="py-2 px-3 text-center">
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold {l?.Switch ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'}">
                      {l?.Switch ? "ON" : "OFF"}
                    </span>
                  </td>
                  <td class="py-2 px-3 text-center">
                    {#if l?.Over}
                      <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold bg-rose-100 text-rose-700 dark:bg-rose-950 dark:text-rose-400">
                        <AlertCircle class="w-3 h-3" /> 過負荷
                      </span>
                    {:else}
                      <span class="text-emerald-500 text-[11px]">正常</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-slate-700 dark:text-slate-300">
                    {p.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(p.LastTime)}
                  </td>
                {:else if activeTab === "motion"}
                  {@const m = item as MotionSensorEnt}
                  {@const l = m.Data?.[m.Data.length - 1]}
                  <td class="py-2 px-3 font-semibold font-sans text-slate-800 dark:text-slate-200">
                    {m.Name || "人感センサー"}
                  </td>
                  <td class="py-2 px-3 text-cyan-600 dark:text-cyan-400 text-[11px]">
                    {m.Address}
                  </td>
                  <td class="py-2 px-3 text-center">
                    <span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold {l?.Moving ? 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-400 animate-pulse' : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'}">
                      <Footprints class="w-3 h-3" />
                      {l?.Moving ? "動体検知中" : "静止"}
                    </span>
                  </td>
                  <td class="py-2 px-3 text-center text-[11px] text-slate-600 dark:text-slate-400">
                    {l?.Light ? "明" : "暗"} / {l?.Battery ?? 0}%
                  </td>
                  <td class="py-2 px-3 text-[11px] text-slate-700 dark:text-slate-300">
                    {l?.LastMove ? formatTimeStr(l.LastMove) : "-"}
                    {#if l?.LastMoveDiff}
                      <span class="text-[10px] text-slate-400">({l.LastMoveDiff}s前)</span>
                    {/if}
                  </td>
                  <td class="py-2 px-3 text-right font-bold text-slate-700 dark:text-slate-300">
                    {m.Count.toLocaleString()}
                  </td>
                  <td class="py-2 px-3 text-slate-500 dark:text-slate-400 text-[11px] whitespace-nowrap">
                    {formatTimeStr(m.LastTime)}
                  </td>
                {/if}
              </tr>

              {#if isExpanded}
                <tr class="bg-slate-50 dark:bg-slate-950/60 font-sans">
                  <td colspan="10" class="p-4 space-y-3">
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">スキャナーホスト</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{(item as any).Host || "-"}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">初回検知日時</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">{formatTimeStr((item as any).FirstTime)}</div>
                      </div>
                      <div class="p-3 rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 space-y-1">
                        <div class="text-[11px] font-bold text-slate-400 uppercase">履歴データ件数</div>
                        <div class="font-mono text-slate-800 dark:text-slate-200">
                          {activeTab === 'device' ? (item as BlueDeviceEnt).RSSI?.length :
                           activeTab === 'env' ? (item as EnvMonitorEnt).EnvData?.length :
                           activeTab === 'power' ? (item as PowerMonitorEnt).Data?.length :
                           (item as MotionSensorEnt).Data?.length || 0} 件
                        </div>
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
