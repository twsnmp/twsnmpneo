<script lang="ts">
  import { untrack } from "svelte";
  import { _, locale } from "svelte-i18n";
  import {
    fetchMIBTree,
    runSNMPTool,
    type MIBTreeEnt,
    type SNMPToolResult,
    type NodeEnt,
    type NetworkEnt,
    type PollingEnt,
  } from "../api";
  import { showLoading, hideLoading } from "../stores/modalStore";
  import {
    X,
    FolderTree,
    Play,
    Copy,
    Check,
    Search,
    RefreshCw,
    Plus,
    FileSpreadsheet,
    HelpCircle,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    ArrowUpDown,
    ArrowUp,
    ArrowDown,
  } from "@lucide/svelte";
  import MIBTree from "./MIBTree.svelte";

  let {
    show = $bindable(false),
    node = null,
    network = null,
    onAddPolling = undefined,
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    network?: NetworkEnt | null;
    onAddPolling?: (p: Partial<PollingEnt>) => void;
  }>();

  let nameOrOid = $state("");
  let history = $state<string[]>([]);
  let scalarOnly = $state(false);
  let rawData = $state(false);
  let tableSearchQuery = $state("");
  let sortColumn = $state<string | null>(null);
  let sortAsc = $state(true);

  let lastQueryIsTable = $state(false);
  let lastQueryName = $state("");

  let isLoading = $state(false);
  let errorMessage = $state("");
  let results = $state<SNMPToolResult[]>([]);
  let selectedIndices = $state<number[]>([]);
  let copied = $state(false);
  let pageSize = $state(25);
  let currentPage = $state(1);

  // Hover Tooltip state
  let hoveredInfo = $state<{
    name: string;
    oid: string;
    type?: string;
    tooltip: string;
    x: number;
    y: number;
  } | null>(null);

  // MIB Tree Modal State
  let showMIBTreeModal = $state(false);
  let mibTreeData = $state<MIBTreeEnt[]>([]);

  const targetName = $derived(node?.name || (node as any)?.Name || network?.name || (network as any)?.Name || "");
  const targetIP = $derived(node?.ip || (node as any)?.IP || network?.ip || (network as any)?.IP || "");
  const targetNodeId = $derived(node?.id || (node as any)?.ID || "");
  const targetNetworkId = $derived(network?.id || (network as any)?.ID || "");

  const isTable = $derived(lastQueryIsTable);

  $effect(() => {
    if (show && (node || network)) {
      untrack(() => {
        errorMessage = "";
        results = [];
        selectedIndices = [];
        currentPage = 1;
        tableSearchQuery = "";
        sortColumn = null;
        lastQueryIsTable = false;
        lastQueryName = "";
        hoveredInfo = null;
        nameOrOid = history[0] ?? "";
      });
    }
  });

  const runQuery = async () => {
    if (!nameOrOid.trim() || (!targetNodeId && !targetNetworkId)) return;
    isLoading = true;
    errorMessage = "";
    selectedIndices = [];
    currentPage = 1;
    tableSearchQuery = "";
    sortColumn = null;
    hoveredInfo = null;
    showLoading({
      title: $_('mib.dialogTitle') || 'MIBブラウザー',
      message: `${targetName || targetIP} から MIB「${nameOrOid.trim()}」を取得中...`,
    });
    try {
      const q = nameOrOid.trim();
      const res = await runSNMPTool(
        { nodeId: targetNodeId || undefined, networkId: targetNetworkId || undefined },
        q,
        "walk",
        rawData
      );
      results = res;
      lastQueryName = q;
      lastQueryIsTable = q.toLowerCase().endsWith("table");
      history = [q, ...history.filter((item) => item !== q)].slice(0, 10);
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isLoading = false;
      hideLoading();
    }
  };

  const handleOpenMIBTree = async () => {
    showMIBTreeModal = true;
    if (mibTreeData.length === 0) {
      showLoading({
        title: $_('mib.treeTitle') || 'MIBツリー',
        message: 'MIBツリー情報を読み込んでいます...',
      });
      try {
        mibTreeData = await fetchMIBTree();
      } catch (e) {
        errorMessage = e instanceof Error ? e.message : String(e);
      } finally {
        hideLoading();
      }
    }
  };

  const selectTreeName = (selectedName: string) => {
    nameOrOid = selectedName;
    showMIBTreeModal = false;
  };

  const toggleSelect = (idx: number, isShift: boolean) => {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  };

  // Tooltip helper
  const getTooltipForMIB = (r: SNMPToolResult) => {
    const isJa = $locale === "ja" || (!$locale && typeof navigator !== "undefined" && navigator.language?.startsWith("ja"));
    const mib = r.mib;
    let desc = "";
    if (mib) {
      if (isJa) {
        desc = mib.descriptionJa || mib.description || mib.descriptionEn || "";
      } else {
        desc = mib.descriptionEn || mib.description || "";
      }
    }
    return desc || `OID: ${r.oid}`;
  };

  const handleMouseEnterRow = (e: MouseEvent, r: SNMPToolResult) => {
    const tooltip = getTooltipForMIB(r);
    hoveredInfo = {
      name: r.name || r.oid,
      oid: r.oid,
      type: r.type,
      tooltip,
      x: e.clientX,
      y: e.clientY,
    };
  };

  const handleMouseMoveRow = (e: MouseEvent) => {
    if (hoveredInfo) {
      hoveredInfo = {
        ...hoveredInfo,
        x: e.clientX,
        y: e.clientY,
      };
    }
  };

  const handleMouseLeaveRow = () => {
    hoveredInfo = null;
  };

  // Table structure computation for Table MIB
  const tableData = $derived.by(() => {
    if (!isTable) {
      return { columns: [], rows: [] as Record<string, any>[] };
    }
    const colNames: string[] = [];
    const rowIndices: string[] = [];
    const rowMap = new Map<string, Record<string, string>>();

    results.forEach((r) => {
      const n = r.name || r.oid;
      const dotIdx = n.indexOf(".");
      if (dotIdx > 0) {
        const base = n.substring(0, dotIdx);
        const index = n.substring(dotIdx + 1);
        if (index === "0") return;
        if (!colNames.includes(base)) {
          colNames.push(base);
        }
        if (!rowMap.has(index)) {
          rowIndices.push(index);
          rowMap.set(index, {});
        }
        rowMap.get(index)![base] = r.value;
      }
    });

    const columns = ["Index", ...colNames];
    const rows: Record<string, any>[] = rowIndices.map((idx, i) => {
      const data = rowMap.get(idx) || {};
      return {
        Index: i + 1,
        _rawIndex: idx,
        ...data,
      } as Record<string, any>;
    });

    return { columns, rows };
  });

  const colMibMap = $derived.by(() => {
    const map = new Map<string, SNMPToolResult>();
    if (!isTable) return map;
    for (const r of results) {
      const n = r.name || r.oid;
      const dot = n.indexOf(".");
      const base = dot > 0 ? n.substring(0, dot) : n;
      if (!map.has(base)) {
        map.set(base, r);
      }
    }
    return map;
  });

  const handleMouseEnterCol = (e: MouseEvent, col: string) => {
    if (col === "Index") return;
    const r = colMibMap.get(col);
    if (!r) return;
    const tooltip = getTooltipForMIB(r);
    let colOid = r.oid;
    const dot = colOid.lastIndexOf(".");
    if (dot > 0 && r.name && r.name.includes(".")) {
      colOid = colOid.substring(0, dot);
    }
    hoveredInfo = {
      name: col,
      oid: colOid,
      type: r.type,
      tooltip,
      x: e.clientX,
      y: e.clientY,
    };
  };

  const displayedTableRows = $derived.by(() => {
    if (!isTable) return [];
    const q = tableSearchQuery.trim().toLowerCase();
    if (!q) return tableData.rows;
    return tableData.rows.filter((row) => {
      return tableData.columns.some((col) => {
        const val = row[col];
        return val !== undefined && String(val).toLowerCase().includes(q);
      });
    });
  });

  const sortedTableRows = $derived.by(() => {
    const list = [...displayedTableRows];
    if (!sortColumn) return list;
    return list.sort((a, b) => {
      const valA = a[sortColumn!] ?? "";
      const valB = b[sortColumn!] ?? "";
      const numA = Number(valA);
      const numB = Number(valB);
      let cmp = 0;
      if (!isNaN(numA) && !isNaN(numB) && valA !== "" && valB !== "") {
        cmp = numA - numB;
      } else {
        cmp = String(valA).localeCompare(String(valB));
      }
      return sortAsc ? cmp : -cmp;
    });
  });

  const handleSort = (col: string) => {
    if (sortColumn === col) {
      sortAsc = !sortAsc;
    } else {
      sortColumn = col;
      sortAsc = true;
    }
  };

  const paginatedTableRows = $derived.by(() => {
    if (pageSize === -1) return sortedTableRows;
    const start = (currentPage - 1) * pageSize;
    return sortedTableRows.slice(start, start + pageSize);
  });

  // Non-table list results
  const displayedResults = $derived.by(() => {
    let list = results;
    if (scalarOnly) {
      list = list.filter((r) => {
        const name = r.name ?? "";
        return r.oid.endsWith(".0") || name.endsWith(".0");
      });
    }
    return list;
  });

  const paginatedResults = $derived.by(() => {
    if (pageSize === -1) return displayedResults;
    const start = (currentPage - 1) * pageSize;
    return displayedResults.slice(start, start + pageSize);
  });

  const currentTotalCount = $derived(isTable ? displayedTableRows.length : displayedResults.length);
  const totalPages = $derived(
    pageSize === -1 ? 1 : Math.max(1, Math.ceil(currentTotalCount / pageSize))
  );

  const copySelected = async () => {
    if (isTable) {
      const list = selectedIndices.length > 0
        ? selectedIndices.map((i) => sortedTableRows[i]).filter(Boolean)
        : sortedTableRows;
      if (list.length === 0) return;
      const lines = [tableData.columns.join("\t")];
      list.forEach((r) => {
        lines.push(tableData.columns.map((col) => r[col] ?? "").join("\t"));
      });
      await navigator.clipboard.writeText(lines.join("\n"));
      copied = true;
      setTimeout(() => (copied = false), 2000);
      return;
    }

    const list = selectedIndices.length > 0
      ? selectedIndices.map((i) => displayedResults[i]).filter(Boolean)
      : displayedResults;
    if (list.length === 0) return;
    const lines = [`${$_("mib.name")}\t${$_("mib.oid")}\t${$_("mib.type")}\t${$_("mib.value")}`];
    list.forEach((r) => {
      lines.push(`${r.name || r.oid}\t${r.oid}\t${r.type}\t${r.value}`);
    });
    await navigator.clipboard.writeText(lines.join("\n"));
    copied = true;
    setTimeout(() => (copied = false), 2000);
  };

  const exportCSV = () => {
    if (isTable) {
      if (displayedTableRows.length === 0) return;
      const lines = [tableData.columns.join(",")];
      displayedTableRows.forEach((r) => {
        const rowVals = tableData.columns.map((col) => {
          const v = r[col] !== undefined ? String(r[col]) : "";
          return `"${v.replace(/"/g, '""')}"`;
        });
        lines.push(rowVals.join(","));
      });
      const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `mib_${targetName}_${lastQueryName || nameOrOid.trim()}_${Date.now()}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      return;
    }

    if (results.length === 0) return;
    const lines = [`${$_("mib.name")},${$_("mib.oid")},${$_("mib.type")},${$_("mib.value")}`];
    results.forEach((r) => {
      const v = `"${r.value.replace(/"/g, '""')}"`;
      lines.push(`"${r.name || r.oid}","${r.oid}",${r.type},${v}`);
    });
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `mib_${targetName}_${lastQueryName || nameOrOid.trim()}_${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCreatePolling = () => {
    if (selectedIndices.length !== 1 || !onAddPolling || isTable) return;
    const r = displayedResults[selectedIndices[0]];
    if (!r) return;
    onAddPolling({
      node_id: targetNodeId,
      name: `SNMP ${r.name || r.oid}`,
      type: "snmp",
      params: r.name || r.oid,
      state: "unknown",
    });
    show = false;
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_("mib.dialogTitle")}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <FolderTree class="h-5 w-5 text-teal-500" />
          <div>
            <h2 class="text-sm font-bold">{$_("mib.dialogTitle")} — {targetName}</h2>
            <p class="text-[11px] font-mono text-slate-500 dark:text-slate-400">{targetIP || $_("mib.noIp")}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <!-- Search & Controls Toolbar -->
      <div class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/40">
        <div class="flex flex-wrap items-center gap-2">
          <div class="relative flex-1 min-w-[200px]">
            <input
              type="text"
              bind:value={nameOrOid}
              placeholder={$_("mib.objectOrOid")}
              class="w-full rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
            />
          </div>

          <button
            type="button"
            onclick={handleOpenMIBTree}
            class="flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:hover:bg-slate-800"
            title={$_("mib.selectFromTree")}
          >
            <FolderTree class="h-3.5 w-3.5 text-teal-500" />{$_("mib.tree")}
          </button>

          <select
            onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) nameOrOid = v; }}
            class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950"
          >
            <option value="">{$_("mib.history")}</option>
            {#each history as h}
              <option value={h}>{h}</option>
            {/each}
          </select>

          <button
            type="button"
            disabled={isLoading || !nameOrOid.trim()}
            onclick={runQuery}
            class="flex items-center gap-1.5 rounded-lg bg-teal-600 px-4 py-1.5 text-xs font-semibold text-white shadow hover:bg-teal-500 disabled:opacity-50"
          >
            {#if isLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin" />
            {:else}
              <Play class="h-3.5 w-3.5 fill-current" />
            {/if}
            {$_("mib.execute")}
          </button>
        </div>

        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span class="text-slate-500 dark:text-slate-400">{$_("mib.favorites")}</span>
          {#each [
            { name: "system", oid: "system" },
            { name: "ifTable", oid: "ifTable" },
            { name: "ifXTable", oid: "ifXTable" },
            { name: "hrStorageTable", oid: "hrStorageTable" },
          ] as item}
            <button
              type="button"
              onclick={() => (nameOrOid = item.oid)}
              class="rounded-md border border-slate-300 bg-white px-2 py-1 font-mono text-teal-700 hover:border-teal-400 hover:bg-teal-50 dark:border-slate-700 dark:bg-slate-950 dark:text-teal-300 dark:hover:bg-slate-800"
            >
              {item.name}
            </button>
          {/each}
        </div>

        <!-- Options row -->
        <div class="flex flex-wrap items-center justify-between gap-3 text-xs">
          <div class="flex items-center gap-4 flex-wrap">
            <label class="flex items-center gap-1.5 cursor-pointer text-slate-600 dark:text-slate-300">
              <input
                type="checkbox"
                bind:checked={scalarOnly}
                disabled={isTable}
                onchange={() => { currentPage = 1; selectedIndices = []; }}
                class="rounded text-teal-600 focus:ring-teal-500 disabled:opacity-40"
              />
              <span class={isTable ? "opacity-40" : ""}>{$_("mib.scalarOnly")}</span>
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer text-slate-600 dark:text-slate-300">
              <input type="checkbox" bind:checked={rawData} class="rounded text-teal-600 focus:ring-teal-500" />
              {$_("mib.rawData")}
            </label>
            <span class="text-slate-400 font-mono">{$_("mib.resultCount", { values: { count: currentTotalCount } })}</span>
          </div>

          <div class="flex items-center gap-2 flex-wrap">
            {#if isTable}
              <div class="flex items-center gap-1.5 mr-2">
                <span class="text-xs text-slate-500 dark:text-slate-400">{$_("common.search", { default: "検索" })}:</span>
                <input
                  type="text"
                  bind:value={tableSearchQuery}
                  placeholder="..."
                  class="w-32 sm:w-44 rounded-md border border-slate-300 bg-white px-2 py-0.5 text-xs text-slate-800 focus:border-teal-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
                />
              </div>
            {/if}

            {#if !isTable && selectedIndices.length === 1 && onAddPolling}
              <button
                type="button"
                onclick={handleCreatePolling}
                class="flex items-center gap-1 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500"
              >
                <Plus class="h-3 w-3" />{$_("mib.createPolling")}
              </button>
            {/if}
            <button
              type="button"
              disabled={currentTotalCount === 0}
              onclick={copySelected}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              {#if copied}<Check class="h-3 w-3 text-emerald-500" />{:else}<Copy class="h-3 w-3" />{/if}
              {selectedIndices.length > 0
                ? $_("mib.copySelected", { values: { count: selectedIndices.length } })
                : $_("mib.copyAll")}
            </button>
            <button
              type="button"
              disabled={currentTotalCount === 0}
              onclick={exportCSV}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              <FileSpreadsheet class="h-3 w-3 text-emerald-500" />CSV
            </button>
          </div>
        </div>
      </div>

      <!-- Result Table Area -->
      <div class="flex min-h-0 flex-1 flex-col p-4">
        {#if errorMessage}
          <div class="mb-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
            {errorMessage}
          </div>
        {/if}

        <div class="min-h-0 flex-1 overflow-auto rounded-xl border border-slate-200 dark:border-slate-800">
          {#if isTable}
            <!-- 2D Table Layout for Table MIBs -->
            <table class="w-full text-left text-xs border-collapse">
              <thead class="sticky top-0 z-10 bg-slate-100 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800">
                <tr>
                  {#each tableData.columns as col}
                    <th
                      onclick={() => handleSort(col)}
                      onmouseenter={(e) => handleMouseEnterCol(e, col)}
                      onmousemove={handleMouseMoveRow}
                      onmouseleave={handleMouseLeaveRow}
                      class="p-2 whitespace-nowrap font-semibold cursor-pointer select-none text-slate-700 hover:bg-slate-200 dark:text-slate-200 dark:hover:bg-slate-800 {col === 'Index' ? 'text-center w-14' : ''}"
                    >
                      <div class="flex items-center gap-1 {col === 'Index' ? 'justify-center' : ''}">
                        <span>{col}</span>
                        {#if sortColumn === col}
                          {#if sortAsc}
                            <ArrowUp class="h-3 w-3 text-teal-500" />
                          {:else}
                            <ArrowDown class="h-3 w-3 text-teal-500" />
                          {/if}
                        {:else}
                          <ArrowUpDown class="h-3 w-3 opacity-30" />
                        {/if}
                      </div>
                    </th>
                  {/each}
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
                {#each paginatedTableRows as row, pageIndex}
                  {@const idx = (currentPage - 1) * (pageSize === -1 ? displayedTableRows.length : pageSize) + pageIndex}
                  {@const isSelected = selectedIndices.includes(idx)}
                  <tr
                    onclick={() => toggleSelect(idx, false)}
                    class={`cursor-pointer transition-colors ${isSelected ? "bg-teal-50 dark:bg-teal-950/40" : "hover:bg-slate-50 dark:hover:bg-slate-900/60"}`}
                  >
                    {#each tableData.columns as col}
                      <td class="py-1.5 px-2 font-mono {col === 'Index' ? 'text-center text-slate-400 text-[10px]' : 'text-slate-800 dark:text-slate-200 whitespace-pre-wrap break-all'}">
                        {row[col] ?? ""}
                      </td>
                    {/each}
                  </tr>
                {:else}
                  <tr>
                    <td colspan={Math.max(1, tableData.columns.length)} class="p-6 text-center text-slate-500">
                      {#if isLoading}{$_("mib.loading")}{:else}{$_("mib.noResults")}{/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {:else}
            <!-- Standard List Layout for Non-Table MIBs -->
            <table class="w-full text-left text-xs">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900">
                <tr>
                  <th class="p-2 w-8 text-center">#</th>
                  <th class="p-2 font-mono">{$_("mib.objectNameOid")}</th>
                  <th class="p-2">{$_("mib.type")}</th>
                  <th class="p-2">{$_("mib.value")}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
                {#each paginatedResults as r, pageIndex}
                  {@const idx = (currentPage - 1) * (pageSize === -1 ? displayedResults.length : pageSize) + pageIndex}
                  {@const isSelected = selectedIndices.includes(idx)}
                  <tr
                    onclick={() => toggleSelect(idx, false)}
                    class={`cursor-pointer transition-colors ${isSelected ? "bg-teal-50 dark:bg-teal-950/40" : "hover:bg-slate-50 dark:hover:bg-slate-900/60"}`}
                  >
                    <td class="py-1 px-2 text-center text-slate-400 font-mono text-[10px]">{idx + 1}</td>
                    <td
                      class="py-1 px-2 font-mono text-slate-700 dark:text-slate-300 break-all"
                      onmouseenter={(e) => handleMouseEnterRow(e, r)}
                      onmousemove={handleMouseMoveRow}
                      onmouseleave={handleMouseLeaveRow}
                    >
                      <div class="font-medium text-slate-900 dark:text-slate-100 hover:text-teal-600 dark:hover:text-teal-400 transition-colors cursor-help inline-block">
                        {r.name || r.oid}
                      </div>
                    </td>
                    <td class="py-1 px-2 text-slate-500 whitespace-nowrap">{r.type}</td>
                    <td class="py-1 px-2 font-mono break-all font-medium">{r.value}</td>
                  </tr>
                {:else}
                  <tr>
                    <td colspan="4" class="p-6 text-center text-slate-500">
                      {#if isLoading}{$_("mib.loading")}{:else}{$_("mib.noResults")}{/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>

        <div class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-slate-200 px-1 pt-3 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-400">
          <div class="flex items-center gap-3">
            <label class="flex items-center gap-2">
              <span>{$_("report.pageShowCount")}</span>
              <select
                bind:value={pageSize}
                onchange={() => (currentPage = 1)}
                class="rounded-lg border border-slate-300 bg-white px-2 py-1 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200"
              >
                <option value={25}>{$_("report.itemsPerPage", { values: { count: 25 } })}</option>
                <option value={50}>{$_("report.itemsPerPage", { values: { count: 50 } })}</option>
                <option value={100}>{$_("report.itemsPerPage", { values: { count: 100 } })}</option>
                <option value={-1}>{$_("report.showAll")}</option>
              </select>
            </label>
            <span class="font-mono text-[11px] text-slate-400">
              {#if currentTotalCount > 0}
                {$_("report.paginationRange", {
                  values: {
                    total: currentTotalCount.toLocaleString(),
                    from: (currentPage - 1) * (pageSize === -1 ? currentTotalCount : pageSize) + 1,
                    to: pageSize === -1 ? currentTotalCount : Math.min(currentPage * pageSize, currentTotalCount),
                  },
                })}
              {:else}
                {$_("report.totalZero")}
              {/if}
            </span>
          </div>

          {#if pageSize !== -1 && totalPages > 1}
            <div class="flex items-center gap-1">
              <button
                type="button"
                disabled={currentPage <= 1}
                onclick={() => (currentPage = 1)}
                class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-900 disabled:opacity-30 disabled:hover:bg-transparent dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
                title={$_("report.firstPage")}
                aria-label={$_("report.firstPage")}
              >
                <ChevronsLeft class="h-4 w-4" />
              </button>
              <button
                type="button"
                disabled={currentPage <= 1}
                onclick={() => currentPage--}
                class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-900 disabled:opacity-30 disabled:hover:bg-transparent dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
                title={$_("report.prevPage")}
                aria-label={$_("report.prevPage")}
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <span class="px-2 font-mono text-xs text-slate-600 dark:text-slate-300">
                {currentPage} / {totalPages}
              </span>
              <button
                type="button"
                disabled={currentPage >= totalPages}
                onclick={() => currentPage++}
                class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-900 disabled:opacity-30 disabled:hover:bg-transparent dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
                title={$_("report.nextPage")}
                aria-label={$_("report.nextPage")}
              >
                <ChevronRight class="h-4 w-4" />
              </button>
              <button
                type="button"
                disabled={currentPage >= totalPages}
                onclick={() => (currentPage = totalPages)}
                class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-200 hover:text-slate-900 disabled:opacity-30 disabled:hover:bg-transparent dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
                title={$_("report.lastPage")}
                aria-label={$_("report.lastPage")}
              >
                <ChevronsRight class="h-4 w-4" />
              </button>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
{/if}

<!-- Hover Tooltip for MIB Object Info -->
{#if hoveredInfo}
  <div
    class="pointer-events-none fixed z-[9999] max-w-lg rounded-xl border border-slate-700 bg-slate-900/95 p-3 text-xs text-slate-100 shadow-2xl backdrop-blur-md"
    style="left: {Math.min(hoveredInfo.x + 16, (typeof window !== 'undefined' ? window.innerWidth - 420 : 500))}px; top: {Math.min(hoveredInfo.y + 16, (typeof window !== 'undefined' ? window.innerHeight - 260 : 500))}px;"
  >
    <div class="flex items-center gap-1.5 font-mono font-bold text-teal-400 pb-1 mb-1 border-b border-slate-700/80">
      <FolderTree class="h-3.5 w-3.5 text-teal-400" />
      <span>{hoveredInfo.name}</span>
      <span class="text-slate-400 font-normal">({hoveredInfo.oid}{hoveredInfo.type ? `:${hoveredInfo.type}` : ""})</span>
    </div>
    {#if hoveredInfo.tooltip}
      <pre class="font-sans whitespace-pre-wrap leading-relaxed text-slate-300 max-h-56 overflow-y-auto text-[11px] select-text">{hoveredInfo.tooltip}</pre>
    {/if}
  </div>
{/if}

<!-- Sub-Modal: MIB Tree Selector -->
{#if showMIBTreeModal}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_("mib.treeStructure")}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showMIBTreeModal = false)}
  >
    <div class="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3 dark:border-slate-800">
        <div class="flex items-center gap-2">
          <FolderTree class="h-4 w-4 text-teal-500" />
          <h3 class="text-xs font-bold">{$_("mib.treeTitle")}</h3>
        </div>
        <button
          type="button"
          onclick={() => (showMIBTreeModal = false)}
          class="rounded-lg p-1 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>
      <div class="min-h-0 flex-1 overflow-hidden p-4">
        <MIBTree
          treeData={mibTreeData}
          onselect={selectTreeName}
          heightClass="h-[60vh]"
        />
      </div>
    </div>
  </div>
{/if}
