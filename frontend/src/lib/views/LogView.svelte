<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    fetchEventLogs,
    queryParquetLogs,
    getLogCounts,
    deleteEventLogs,
    deleteParquetLogs,
    askAI,
    type EventLogEnt,
  } from "../api";
  import LogFilterModal from "../components/LogFilterModal.svelte";
  import LogReportModal from "../components/LogReportModal.svelte";
  import LogAIDialog from "../components/LogAIDialog.svelte";
  import LogSidebar from "./log/LogSidebar.svelte";
  import LogChart from "./log/LogChart.svelte";
  import LogActionBar from "./log/LogActionBar.svelte";
  import LogTable from "./log/LogTable.svelte";
  import {
    parseEventLogs,
    parseParquetLogs,
    getCategoryColumns,
    exportLogsCSV,
  } from "./log/logUtils";
  import type { LogCategory, ColumnDef, FilterState, LogItem } from "./log/types";

  let activeTab = $state<LogCategory>("event");
  let eventLogs = $state<EventLogEnt[]>([]);
  let rawParquetLogs = $state<any[]>([]);
  let logCounts = $state<Record<string, number>>({});
  let loading = $state(false);

  // Reception Chart state
  let chartZoomRange = $state<{ st: number; et: number } | null>(null);

  // Search & Filter state
  let searchQuery = $state("");
  let fetchLimit = $state(10000);
  let showFilterModal = $state(false);
  let filterState = $state<FilterState>({
    start: "",
    end: "",
    level: "all",
    type: "",
    source: "",
    keyword: "",
  });

  // Report & AI Modal state
  let showReportModal = $state(false);
  let showAIDialog = $state(false);
  let selectedLogText = $state("");
  let aiAnswer = $state("");
  let aiLoading = $state(false);

  // Column Visibility & Sflow Mode
  let columnVisibility = $state<Record<string, boolean>>({});
  let sflowCounter = $state(false);

  // Pagination state
  let currentPage = $state(1);
  let pageSize = $state(25);

  // Sorting state
  let sortColumn = $state("time");
  let sortDirection = $state<"asc" | "desc">("desc");

  // Columns per category
  const categoryColumns = $derived<Record<string, ColumnDef[]>>(getCategoryColumns($_));

  const currentColumns = $derived<ColumnDef[]>(
    activeTab === "sflow" && sflowCounter
      ? categoryColumns["sflowCounter"]
      : categoryColumns[activeTab] || []
  );

  const visibleColumns = $derived<ColumnDef[]>(
    currentColumns.filter((col: ColumnDef) => {
      const colTab = activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab;
      const key = `${colTab}_${col.key}`;
      return columnVisibility[key] !== false;
    })
  );

  const toggleColumn = (key: string) => {
    const colTab = activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab;
    const colKey = `${colTab}_${key}`;
    columnVisibility[colKey] = columnVisibility[colKey] === false ? true : false;
  };

  const refreshCounts = async () => {
    try {
      const counts = await getLogCounts();
      if (counts) logCounts = counts;
    } catch (e) {
      console.error(e);
    }
  };

  const loadCurrentLogs = async () => {
    loading = true;
    try {
      refreshCounts();
      let startTime = 0;
      let endTime = 0;
      if (filterState.start) startTime = new Date(filterState.start).getTime() * 1e6;
      if (filterState.end) endTime = new Date(filterState.end).getTime() * 1e6;

      if (activeTab === "event") {
        eventLogs = await fetchEventLogs({
          start: startTime || undefined,
          end: endTime || undefined,
          level: filterState.level !== "all" ? filterState.level : undefined,
          type: filterState.type || undefined,
          nodeName: filterState.source || undefined,
          filter: filterState.keyword || undefined,
          limit: fetchLimit,
        });
        logCounts["event"] = eventLogs.length;
      } else {
        const queryType = activeTab === "arp" ? "arplog" : activeTab === "sflow" && sflowCounter ? "sflowCounter" : activeTab;
        const pq = await queryParquetLogs({
          type: queryType,
          src: filterState.source || undefined,
          filter: filterState.keyword || undefined,
          start: startTime || undefined,
          end: endTime || undefined,
          limit: fetchLimit,
        });
        rawParquetLogs = pq;
      }
      currentPage = 1;
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  };

  // Structured Item Converter
  const structuredLogs = $derived<LogItem[]>(
    activeTab === "event"
      ? parseEventLogs(eventLogs)
      : parseParquetLogs(rawParquetLogs, activeTab, sflowCounter)
  );

  // Filtered Logs
  const filteredLogs = $derived<LogItem[]>(
    structuredLogs.filter((item) => {
      // 1. Chart zoom range filter
      if (chartZoomRange && chartZoomRange.st > 0 && chartZoomRange.et > 0) {
        const itemTime = item.time;
        if (itemTime < chartZoomRange.st || itemTime > chartZoomRange.et) {
          return false;
        }
      }

      // 2. Incremental search filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        if (!item.fullText.toLowerCase().includes(q)) {
          return false;
        }
      }

      // 3. Level filter
      if (filterState.level !== "all" && item.level) {
        if (item.level.toLowerCase() !== filterState.level.toLowerCase()) {
          return false;
        }
      }

      return true;
    })
  );

  // Sorted Logs
  const sortedLogs = $derived<LogItem[]>(
    [...filteredLogs].sort((a: any, b: any) => {
      let valA = a[sortColumn];
      let valB = b[sortColumn];

      if (valA === undefined || valA === null) valA = "";
      if (valB === undefined || valB === null) valB = "";

      let comparison = 0;
      if (typeof valA === "number" && typeof valB === "number") {
        comparison = valA - valB;
      } else {
        comparison = String(valA).localeCompare(String(valB));
      }

      return sortDirection === "asc" ? comparison : -comparison;
    })
  );

  // Pagination Slice
  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(sortedLogs.length / pageSize))
  );

  const paginatedLogs = $derived<LogItem[]>(
    pageSize === -1
      ? sortedLogs
      : sortedLogs.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  const handleSort = (colKey: string) => {
    if (sortColumn === colKey) {
      sortDirection = sortDirection === "asc" ? "desc" : "asc";
    } else {
      sortColumn = colKey;
      sortDirection = "desc";
    }
  };

  const handleTabSelect = (tab: LogCategory) => {
    activeTab = tab;
    searchQuery = "";
    currentPage = 1;
    sortColumn = "time";
    sortDirection = "desc";
  };

  const handleAskAI = async (logText: string) => {
    selectedLogText = logText;
    showAIDialog = true;
    aiLoading = true;
    aiAnswer = "";
    try {
      aiAnswer = await askAI(
        $_('log.aiPrompt', { values: { log: logText } }),
        $_('log.aiSystem')
      );
    } catch (e: any) {
      aiAnswer = $_('log.aiError', { values: { error: e.message } });
    } finally {
      aiLoading = false;
    }
  };

  const handleDeleteAll = async () => {
    const confirmMsg = $_('log.deleteConfirm', { values: { type: activeTab.toUpperCase() } });
    if (!confirm(confirmMsg)) {
      return;
    }
    try {
      if (activeTab === "event") {
        await deleteEventLogs();
      } else if (activeTab === "sflow") {
        await deleteParquetLogs("sflow");
        await deleteParquetLogs("sflowCounter");
      } else {
        await deleteParquetLogs(activeTab === "arp" ? "arplog" : activeTab);
      }
      chartZoomRange = null;
      loadCurrentLogs();
    } catch (e) {
      alert(`${$_('log.deleteFailed')}: ${e}`);
    }
  };

  const handleExportCSV = () => {
    exportLogsCSV(sortedLogs, visibleColumns, activeTab, sflowCounter);
  };

  const handleExportExcel = () => {
    exportLogsCSV(sortedLogs, visibleColumns, activeTab, sflowCounter);
  };

  onMount(() => {
    loadCurrentLogs();
  });

  $effect(() => {
    activeTab;
    chartZoomRange = null;
    loadCurrentLogs();
  });
</script>

<div class="flex h-[calc(100vh-4.25rem)] overflow-hidden bg-slate-100 dark:bg-[#0b1329] text-slate-800 dark:text-slate-100 font-sans transition-colors">
  <!-- Left Sidebar -->
  <LogSidebar
    bind:activeTab
    {logCounts}
    currentTabCount={structuredLogs.length}
    hitCount={filteredLogs.length}
    bind:fetchLimit
    onTabSelect={handleTabSelect}
    onLimitChange={loadCurrentLogs}
  />

  <!-- Right Main Content Canvas -->
  <div class="flex-1 overflow-hidden flex flex-col p-4 gap-3 min-w-0">
    <!-- Top Reception Status Graph (Collapsible) -->
    <LogChart
      {activeTab}
      logs={structuredLogs}
      bind:chartZoomRange
    />

    <!-- Action Bar: Search, Filters, Column Selector, Report, Export -->
    <LogActionBar
      {activeTab}
      bind:searchQuery
      {filterState}
      {currentColumns}
      {columnVisibility}
      bind:sflowCounter
      {loading}
      onOpenFilter={() => (showFilterModal = true)}
      onToggleColumn={toggleColumn}
      onSflowCounterToggle={() => {
        currentPage = 1;
        loadCurrentLogs();
      }}
      onOpenReport={() => (showReportModal = true)}
      onDeleteAll={handleDeleteAll}
      onExportCSV={handleExportCSV}
      onExportExcel={handleExportExcel}
      onRefresh={loadCurrentLogs}
    />

    <!-- Table Container -->
    <LogTable
      {activeTab}
      {visibleColumns}
      {paginatedLogs}
      totalLogsCount={sortedLogs.length}
      {loading}
      bind:sortColumn
      bind:sortDirection
      bind:currentPage
      bind:pageSize
      {totalPages}
      onSort={handleSort}
      onAskAI={handleAskAI}
      onPageChange={(page) => (currentPage = page)}
      onPageSizeChange={(size) => {
        pageSize = size;
        currentPage = 1;
      }}
    />
  </div>

  <!-- Detailed Filter Modal -->
  <LogFilterModal
    bind:show={showFilterModal}
    logCategory={activeTab}
    bind:filterState
    onApply={loadCurrentLogs}
  />

  <!-- Analytics Report Modal -->
  <LogReportModal
    bind:show={showReportModal}
    logs={structuredLogs.map((l) => l.raw)}
    logCategory={activeTab}
  />

  <!-- AI Analysis Modal -->
  <LogAIDialog
    bind:show={showAIDialog}
    {selectedLogText}
    {aiAnswer}
    {aiLoading}
  />
</div>
