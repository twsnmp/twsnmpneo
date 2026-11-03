<script lang="ts">
  import { untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    fetchGNMICapabilities,
    runGNMIGet,
    type GNMICapabilitiesEnt,
    type GNMIValueEnt,
    type NodeEnt,
    type PollingEnt,
  } from "../api";
  import {
    X,
    Radio,
    Play,
    Copy,
    Check,
    RefreshCw,
    Plus,
    FileSpreadsheet,
    Layers,
    Search,
    ExternalLink,
    Zap,
  } from "@lucide/svelte";
  import TablePagination from "./common/TablePagination.svelte";

  let {
    show = $bindable(false),
    node = null,
    onAddPolling = undefined,
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    onAddPolling?: (p: Partial<PollingEnt>) => void;
  }>();

  let path = $state("/interfaces");
  let target = $state("");
  let encoding = $state("json_ietf");
  let history = $state<string[]>(["/interfaces", "/system", "/components"]);

  const shortcutPaths = [
    "/interfaces",
    "/system",
    "/components",
    "/network-instances",
    "/platform",
    "/acl",
    "/qos",
    "/lldp",
  ];

  let isLoading = $state(false);
  let isCapLoading = $state(false);
  let errorMessage = $state("");
  let capabilities = $state<GNMICapabilitiesEnt | null>(null);
  let results = $state<GNMIValueEnt[]>([]);
  let selectedIndices = $state<number[]>([]);
  let copied = $state(false);
  let showCapDetails = $state(false);
  let modelFilter = $state("");

  let filterText = $state("");
  let pageSize = $state(25);
  let currentPage = $state(1);

  const nodeId = $derived(node?.id || (node as any)?.ID || "");
  const targetName = $derived(node?.name || (node as any)?.Name || "");

  let lastOpened = false;
  let lastNodeId = "";

  $effect(() => {
    const isShowing = show;
    const currentId = node ? (node.id || (node as any).ID || "") : "";

    if (isShowing && currentId) {
      if (!lastOpened || lastNodeId !== currentId) {
        lastOpened = true;
        lastNodeId = currentId;
        untrack(() => {
          errorMessage = "";
          results = [];
          selectedIndices = [];
          filterText = "";
          currentPage = 1;
          capabilities = null;
          showCapDetails = false;
          modelFilter = "";
          const port = node?.gnmi_port || (node as any)?.GNMIPort || "57400";
          target = `${node?.ip || (node as any)?.IP || ""}:${port}`;
          encoding = node?.gnmi_encoding || (node as any)?.GNMIEncoding || "json_ietf";
        });
      }
      // Do NOT auto-fetch on dialog open. Wait until user clicks Get or Capabilities.
    } else if (!isShowing) {
      lastOpened = false;
      lastNodeId = "";
    }
  });

  const loadCapabilities = async () => {
    if (!nodeId) return;
    isCapLoading = true;
    errorMessage = "";
    try {
      capabilities = await fetchGNMICapabilities(nodeId, target.trim());
      showCapDetails = true;
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isCapLoading = false;
    }
  };

  const openYangInfo = () => {
    window.open("https://github.com/YangModels/yang", "_blank", "noopener,noreferrer");
  };

  const runGet = async () => {
    if (!nodeId || !path.trim()) return;
    isLoading = true;
    errorMessage = "";
    selectedIndices = [];
    currentPage = 1;
    try {
      const q = path.trim();
      if (!history.includes(q)) {
        history = [q, ...history.slice(0, 9)];
      }
      results = await runGNMIGet(nodeId, q, encoding, target.trim());
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isLoading = false;
    }
  };

  const selectShortcut = (p: string) => {
    path = p;
  };

  const dblClickShortcut = (p: string) => {
    path = p;
    runGet();
  };

  const handleRowDblClick = (r: GNMIValueEnt) => {
    if (r.Path) {
      path = r.Path;
    }
  };

  const toggleSelect = (idx: number) => {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  };

  interface IndexedGNMIValue extends GNMIValueEnt {
    _originalIdx: number;
  }

  const filteredResults = $derived.by<IndexedGNMIValue[]>(() => {
    const list: IndexedGNMIValue[] = results.map((r, i) => ({ ...r, _originalIdx: i }));
    const q = filterText.trim().toLowerCase();
    if (!q) return list;
    return list.filter(
      (r) =>
        r.Path.toLowerCase().includes(q) ||
        (r.Index && r.Index.toLowerCase().includes(q)) ||
        r.Value.toLowerCase().includes(q)
    );
  });

  const paginatedResults = $derived.by<IndexedGNMIValue[]>(() => {
    if (pageSize === -1) return filteredResults;
    const start = (currentPage - 1) * pageSize;
    return filteredResults.slice(start, start + pageSize);
  });

  const copySelected = async () => {
    const list = selectedIndices.length > 0
      ? selectedIndices.map((i) => results[i]).filter(Boolean)
      : filteredResults;
    if (list.length === 0) return;
    const lines = [`${$_("gnmi.path")}\t${$_("gnmi.index")}\t${$_("gnmi.value")}`];
    list.forEach((r) => {
      lines.push(`${r.Path}\t${r.Index || ""}\t${r.Value}`);
    });
    await navigator.clipboard.writeText(lines.join("\n"));
    copied = true;
    setTimeout(() => (copied = false), 2000);
  };

  const exportCSV = () => {
    const list = selectedIndices.length > 0
      ? selectedIndices.map((i) => results[i]).filter(Boolean)
      : filteredResults;
    if (list.length === 0) return;
    const lines = [`${$_("gnmi.path")},${$_("gnmi.index")},${$_("gnmi.value")}`];
    list.forEach((r) => {
      const v = `"${r.Value.replace(/"/g, '""')}"`;
      lines.push(`"${r.Path}","${r.Index || ""}",${v}`);
    });
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `gnmi_${targetName}_${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCreatePolling = () => {
    if (selectedIndices.length !== 1 || !onAddPolling) return;
    const r = results[selectedIndices[0]];
    if (!r) return;
    onAddPolling({
      node_id: nodeId,
      name: `gNMI ${r.Path}`,
      type: "gnmi",
      params: r.Path,
      state: "unknown",
    });
    show = false;
  };

  const filteredModels = $derived.by(() => {
    if (!capabilities?.models) return [];
    const q = modelFilter.trim().toLowerCase();
    if (!q) return capabilities.models;
    return capabilities.models.filter(
      (m) =>
        m.name.toLowerCase().includes(q) ||
        (m.organization && m.organization.toLowerCase().includes(q)) ||
        (m.version && m.version.toLowerCase().includes(q))
    );
  });
</script>

{#if show}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_("gnmi.dialogTitle")}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[92vh] w-full max-w-5xl 2xl:max-w-6xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <Radio class="h-5 w-5 text-cyan-500" />
          <div>
            <h2 class="text-sm font-bold">{$_("gnmi.dialogTitle")} — {targetName}</h2>
            <p class="text-[11px] font-mono text-slate-500 dark:text-slate-400">{target || $_("gnmi.noTarget")}</p>
          </div>
        </div>
        <div class="flex items-center gap-3">
          <span class="hidden sm:inline text-[11px] text-slate-400">{$_("gnmi.dblClickTip")}</span>
          <button
            type="button"
            aria-label={$_("common.close")}
            onclick={() => (show = false)}
            class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
      </header>

      <!-- Main Controls Toolbar matching PingDialog style -->
      <div class="flex flex-col gap-2.5 border-b border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/40">
        <div class="flex flex-wrap items-center gap-2">
          <!-- Target IP:Port Input -->
          <input
            type="text"
            bind:value={target}
            placeholder={$_("gnmi.target")}
            title={$_("gnmi.target")}
            class="w-44 sm:w-52 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
          />

          <!-- Encoding Selector -->
          <select
            bind:value={encoding}
            title={$_("gnmi.encoding")}
            class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950"
          >
            <option value="json_ietf">json_ietf</option>
            <option value="json">json</option>
            <option value="proto">proto</option>
            <option value="bytes">bytes</option>
            <option value="ascii">ascii</option>
          </select>

          <!-- Path Input -->
          <div class="relative min-w-[200px] flex-1">
            <Search class="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              bind:value={path}
              placeholder={$_("gnmi.pathPlaceholder")}
              onkeydown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  runGet();
                }
              }}
              class="w-full rounded-lg border border-slate-300 bg-white pl-8 pr-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
            />
          </div>

          <!-- History Selector -->
          <select
            onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) path = v; }}
            class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950"
          >
            <option value="">{$_("gnmi.history")}</option>
            {#each history as h}
              <option value={h}>{h}</option>
            {/each}
          </select>

          <!-- Capabilities Button -->
          <button
            type="button"
            disabled={isCapLoading || !nodeId}
            onclick={loadCapabilities}
            class="flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:hover:bg-slate-800 disabled:opacity-50 transition-colors cursor-pointer"
            title={$_("gnmi.capabilities")}
          >
            {#if isCapLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin text-cyan-500" />
            {:else}
              <Layers class="h-3.5 w-3.5 text-cyan-500" />
            {/if}
            {$_("gnmi.capabilities")}
          </button>

          <!-- YANG情報 Button -->
          <button
            type="button"
            onclick={openYangInfo}
            class="flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 dark:hover:bg-slate-800 transition-colors cursor-pointer"
            title={$_("gnmi.yangInfo")}
          >
            <ExternalLink class="h-3.5 w-3.5 text-cyan-500" />
            {$_("gnmi.yangInfo")}
          </button>

          <!-- 取得 (Get) Button -->
          <button
            type="button"
            disabled={isLoading || !path.trim() || !nodeId}
            onclick={runGet}
            class="flex items-center gap-1.5 rounded-lg bg-cyan-600 px-4 py-1.5 text-xs font-semibold text-white shadow hover:bg-cyan-500 disabled:opacity-50 transition-colors cursor-pointer"
          >
            {#if isLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin" />
            {:else}
              <Play class="h-3.5 w-3.5 fill-current" />
            {/if}
            {$_("gnmi.get")}
          </button>
        </div>

        <!-- Shortcuts row (matches twsnmpfk styling) -->
        <div class="flex items-center gap-1.5 overflow-x-auto text-xs py-0.5">
          <div class="flex items-center gap-1 text-[11px] font-semibold text-slate-600 dark:text-slate-300 shrink-0">
            <Zap class="h-3.5 w-3.5 text-amber-500 fill-amber-500" />
            <span>{$_("gnmi.shortcuts")}:</span>
          </div>
          {#each shortcutPaths as p}
            <button
              type="button"
              onclick={() => selectShortcut(p)}
              ondblclick={() => dblClickShortcut(p)}
              class="px-2.5 py-0.5 rounded-md text-[11px] font-mono font-semibold bg-cyan-50 dark:bg-cyan-950/60 border border-cyan-200 dark:border-cyan-700/80 text-cyan-700 dark:text-cyan-300 hover:bg-cyan-100 dark:hover:bg-cyan-900 hover:text-cyan-900 dark:hover:text-cyan-100 hover:border-cyan-400 dark:hover:border-cyan-500 transition-colors shadow-2xs shrink-0 cursor-pointer"
              title={`${p} (${$_("gnmi.shortcutsTip")})`}
            >
              {p}
            </button>
          {/each}
        </div>

        <!-- Result Stats, Filter Input & Action Tools -->
        <div class="flex flex-wrap items-center justify-between gap-3 text-xs pt-0.5">
          <div class="flex flex-wrap items-center gap-3">
            {#if capabilities}
              <button
                type="button"
                onclick={() => (showCapDetails = !showCapDetails)}
                class="flex items-center gap-1 rounded bg-slate-200 px-2 py-0.5 text-[11px] font-medium text-slate-700 hover:bg-slate-300 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 cursor-pointer"
              >
                <Layers class="h-3 w-3 text-cyan-500" />
                Capabilities: {capabilities.version || "v0"} ({capabilities.models?.length || 0} models)
              </button>
            {/if}
            <span class="text-slate-500 dark:text-slate-400 font-mono">
              {#if filterText.trim()}
                {$_("gnmi.filteredResultCount", { values: { filtered: filteredResults.length, total: results.length } })}
              {:else}
                {$_("gnmi.resultCount", { values: { count: results.length } })}
              {/if}
            </span>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <!-- Search / Filter input for results table -->
            <div class="relative w-48 sm:w-60">
              <Search class="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                bind:value={filterText}
                oninput={() => (currentPage = 1)}
                placeholder={$_("gnmi.searchPlaceholder")}
                class="w-full rounded-lg border border-slate-300 bg-white pl-8 pr-7 py-1 text-xs text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
              />
              {#if filterText}
                <button
                  type="button"
                  onclick={() => { filterText = ""; currentPage = 1; }}
                  class="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 cursor-pointer"
                >
                  <X class="h-3 w-3" />
                </button>
              {/if}
            </div>

            {#if selectedIndices.length === 1 && onAddPolling}
              <button
                type="button"
                onclick={handleCreatePolling}
                class="flex items-center gap-1 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500 cursor-pointer shadow-xs"
              >
                <Plus class="h-3 w-3" />{$_("gnmi.createPolling")}
              </button>
            {/if}
            <button
              type="button"
              disabled={filteredResults.length === 0}
              onclick={copySelected}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 cursor-pointer shadow-xs"
            >
              {#if copied}<Check class="h-3 w-3 text-emerald-500" />{:else}<Copy class="h-3 w-3" />{/if}
              {selectedIndices.length > 0
                ? $_("gnmi.copySelected", { values: { count: selectedIndices.length } })
                : $_("gnmi.copyAll")}
            </button>
            <button
              type="button"
              disabled={filteredResults.length === 0}
              onclick={exportCSV}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200 cursor-pointer shadow-xs"
            >
              <FileSpreadsheet class="h-3 w-3 text-emerald-500" />CSV
            </button>
          </div>
        </div>

        <!-- Expandable Capabilities Card -->
        {#if showCapDetails && capabilities}
          <div class="relative rounded-xl border border-cyan-500/30 bg-cyan-500/5 p-4 text-xs dark:bg-cyan-950/20">
            <div class="flex items-center justify-between pb-2 border-b border-cyan-500/20">
              <div class="flex items-center gap-2">
                <Layers class="h-4 w-4 text-cyan-500" />
                <span class="font-bold text-slate-800 dark:text-slate-200">{$_("gnmi.capabilitiesTitle")}</span>
                <span class="rounded bg-cyan-500/20 px-2 py-0.5 text-[11px] font-mono text-cyan-700 dark:text-cyan-300">
                  Version: {capabilities.version || "N/A"}
                </span>
              </div>
              <button
                type="button"
                onclick={() => (showCapDetails = false)}
                class="rounded p-1 text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800"
              >
                <X class="h-3.5 w-3.5" />
              </button>
            </div>

            <div class="mt-2 text-xs">
              <span class="font-semibold text-slate-700 dark:text-slate-300">{$_("gnmi.encoding")}</span>
              <span class="ml-1.5 font-mono text-cyan-600 dark:text-cyan-400">{capabilities.encodings || "N/A"}</span>
            </div>

            <div class="mt-3">
              <div class="flex items-center justify-between gap-2 pb-1.5">
                <span class="font-semibold text-slate-700 dark:text-slate-300">
                  {$_("gnmi.supportedModels")} ({capabilities.models?.length || 0})
                </span>
                <input
                  type="text"
                  bind:value={modelFilter}
                  placeholder={$_("gnmi.searchModels")}
                  class="w-48 rounded-lg border border-slate-300 bg-white px-2.5 py-1 text-[11px] text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
                />
              </div>

              <div class="max-h-48 overflow-auto rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-950">
                <table class="w-full text-left text-[11px]">
                  <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900 text-slate-600 dark:text-slate-400">
                    <tr>
                      <th class="p-1.5 font-mono">{$_("gnmi.modelName")}</th>
                      <th class="p-1.5 font-mono">{$_("gnmi.organization")}</th>
                      <th class="p-1.5 font-mono w-24">{$_("gnmi.version")}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
                    {#each filteredModels as m}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-900/60 font-mono">
                        <td class="p-1.5 text-cyan-700 dark:text-cyan-300 break-all font-medium">{m.name}</td>
                        <td class="p-1.5 text-slate-600 dark:text-slate-400">{m.organization || "—"}</td>
                        <td class="p-1.5 text-slate-500">{m.version || "—"}</td>
                      </tr>
                    {:else}
                      <tr>
                        <td colspan="3" class="p-3 text-center text-slate-400">{$_("gnmi.noModelInfo")}</td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        {/if}
      </div>

      <!-- Result Table Area -->
      <div class="min-h-0 flex-1 flex flex-col overflow-hidden p-4">
        {#if errorMessage}
          <div class="mb-3 shrink-0 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
            {errorMessage}
          </div>
        {/if}

        <div class="flex-1 min-h-0 overflow-auto rounded-t-xl border border-slate-200 dark:border-slate-800">
          <table class="w-full min-w-[700px] table-fixed text-left text-xs">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900 shadow-xs z-10 select-none">
              <tr>
                <th class="p-2 w-12 text-center shrink-0">#</th>
                <th class="p-2 font-mono w-7/12">{$_("gnmi.path")}</th>
                <th class="p-2 font-mono w-20 text-center shrink-0">{$_("gnmi.index")}</th>
                <th class="p-2 font-mono">{$_("gnmi.value")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
              {#each paginatedResults as r, pageIndex}
                {@const originalIdx = r._originalIdx}
                {@const isSelected = selectedIndices.includes(originalIdx)}
                {@const rowNum = (currentPage - 1) * (pageSize === -1 ? filteredResults.length : pageSize) + pageIndex + 1}
                <tr
                  onclick={() => toggleSelect(originalIdx)}
                  ondblclick={() => handleRowDblClick(r)}
                  class={`cursor-pointer transition-colors ${isSelected ? "bg-cyan-50 dark:bg-cyan-950/40" : "hover:bg-slate-50 dark:hover:bg-slate-900/60"}`}
                >
                  <td class="p-2 text-center text-slate-400 font-mono text-[10px]">{rowNum}</td>
                  <td class="p-2 font-mono text-slate-700 dark:text-slate-300 break-all" title={r.Path}>{r.Path}</td>
                  <td class="p-2 font-mono text-center text-slate-500">{r.Index || "—"}</td>
                  <td class="p-2 font-mono break-all font-medium text-slate-800 dark:text-slate-200" title={r.Value}>{r.Value}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="p-6 text-center text-slate-500">
                    {#if isLoading}
                      {$_("gnmi.loading")}
                    {:else if results.length > 0}
                      {$_("gnmi.noFilterMatch")}
                    {:else}
                      {$_("gnmi.noResults")}
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <div class="shrink-0 rounded-b-xl border border-t-0 border-slate-200 dark:border-slate-800 overflow-hidden">
          <TablePagination
            bind:pageSize
            bind:currentPage
            totalCount={filteredResults.length}
          />
        </div>
      </div>
    </div>
  </div>
{/if}
