<script lang="ts">
  import {
    fetchMIBModules,
    uploadMIBModule,
    deleteMIBModule,
    reloadMIBModules,
    fetchMIBTree,
    type MIBModuleEnt,
    type MIBTreeEnt,
  } from "../../api";
  import { showConfirm } from "../../stores/modalStore";
  import ListPagination from "../../views/list/ListPagination.svelte";
  import MIBTree from "../MIBTree.svelte";
  import {
    FolderTree,
    RefreshCw,
    Upload,
    Trash2,
    Search,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    CheckCircle,
    AlertTriangle,
    CheckCircle2,
    X,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    onSuccess,
    onError,
  }: {
    onSuccess?: (msg: string) => void;
    onError?: (msg: string) => void;
  } = $props();

  let mibModules = $state<MIBModuleEnt[]>([]);
  let mibLoading = $state(false);
  let mibFilter = $state("");
  let mibMsg = $state("");
  let mibError = $state("");
  let mibFileInput = $state<HTMLInputElement | null>(null);
  let showMIBTreeModal = $state(false);
  let mibTreeData = $state<MIBTreeEnt[]>([]);
  let mibSortColumn = $state<"index" | "type" | "name" | "file" | "status" | null>(null);
  let mibSortDirection = $state<"asc" | "desc">("asc");
  let mibPageSize = $state(25);
  let mibCurrentPage = $state(1);

  export async function loadMIBModules() {
    mibLoading = true;
    mibError = "";
    try {
      mibModules = await fetchMIBModules();
    } catch (e: any) {
      mibError = e?.message || String(e);
      onError?.(mibError);
    } finally {
      mibLoading = false;
    }
  }

  function handleMibSort(col: "index" | "type" | "name" | "file" | "status") {
    if (mibSortColumn === col) {
      mibSortDirection = mibSortDirection === "asc" ? "desc" : "asc";
    } else {
      mibSortColumn = col;
      mibSortDirection = "asc";
    }
  }

  async function handleReloadMIB() {
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await reloadMIBModules();
      mibMsg = $_('config.mibReloadSuccess');
      onSuccess?.(mibMsg);
      setTimeout(() => (mibMsg = ""), 3000);
    } catch (e: any) {
      mibError = e?.message || String(e);
      onError?.(mibError);
    } finally {
      mibLoading = false;
    }
  }

  async function handleUploadMIB(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await uploadMIBModule(file);
      mibMsg = $_('config.mibUploadSuccess', { values: { file: file.name } });
      onSuccess?.(mibMsg);
      setTimeout(() => (mibMsg = ""), 4000);
    } catch (e: any) {
      mibError = e?.message || String(e);
      onError?.(mibError);
    } finally {
      mibLoading = false;
      if (mibFileInput) mibFileInput.value = "";
    }
  }

  async function handleDeleteMIB(filePath: string) {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'MIB削除の確認',
      message: $_("config.mibDeleteConfirm"),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await deleteMIBModule(filePath);
      mibMsg = $_('config.mibDeleteSuccess');
      onSuccess?.(mibMsg);
      setTimeout(() => (mibMsg = ""), 3000);
    } catch (e: any) {
      mibError = e?.message || String(e);
      onError?.(mibError);
    } finally {
      mibLoading = false;
    }
  }

  async function handleOpenMIBTree() {
    showMIBTreeModal = true;
    if (mibTreeData.length === 0) {
      try {
        mibTreeData = await fetchMIBTree();
      } catch (e: any) {
        mibError = e?.message || String(e);
        onError?.(mibError);
      }
    }
  }

  const filteredMibModules = $derived.by(() => {
    const q = mibFilter.trim().toLowerCase();
    if (!q) return mibModules;
    return mibModules.filter((m) => {
      const name = (m.name || m.Name || "").toLowerCase();
      const file = (m.file || m.File || "").toLowerCase();
      const err = (m.error || m.Error || "").toLowerCase();
      const type = (m.type || m.Type || "").toLowerCase();
      return name.includes(q) || file.includes(q) || err.includes(q) || type.includes(q);
    });
  });

  const sortedMibModules = $derived.by(() => {
    const indexed = filteredMibModules.map((m, originalIdx) => ({
      mod: m,
      originalIdx,
    }));

    if (!mibSortColumn || mibSortColumn === "index") {
      if (mibSortColumn === "index" && mibSortDirection === "desc") {
        return [...indexed].reverse().map((i) => i.mod);
      }
      return indexed.map((i) => i.mod);
    }

    const direction = mibSortDirection === "asc" ? 1 : -1;
    return [...indexed].sort((a, b) => {
      let valA = "";
      let valB = "";
      if (mibSortColumn === "type") {
        valA = a.mod.type || a.mod.Type || "";
        valB = b.mod.type || b.mod.Type || "";
      } else if (mibSortColumn === "name") {
        valA = a.mod.name || a.mod.Name || "";
        valB = b.mod.name || b.mod.Name || "";
      } else if (mibSortColumn === "file") {
        valA = a.mod.file || a.mod.File || "";
        valB = b.mod.file || b.mod.File || "";
      } else if (mibSortColumn === "status") {
        valA = a.mod.error || a.mod.Error || "OK";
        valB = b.mod.error || b.mod.Error || "OK";
      }
      const cmp = valA.localeCompare(valB, undefined, { numeric: true, sensitivity: "base" });
      if (cmp !== 0) return direction * cmp;
      return a.originalIdx - b.originalIdx;
    }).map((i) => i.mod);
  });

  const paginatedMibModules = $derived.by(() => {
    if (mibPageSize === -1) return sortedMibModules;
    const totalPages = Math.max(1, Math.ceil(sortedMibModules.length / mibPageSize));
    const page = Math.min(Math.max(1, mibCurrentPage), totalPages);
    const start = (page - 1) * mibPageSize;
    return sortedMibModules.slice(start, start + mibPageSize);
  });

  $effect(() => {
    loadMIBModules();
  });
</script>

<div class="space-y-6">
  <!-- Header info card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <FolderTree class="h-4 w-4 text-teal-500" />
          {$_('config.mibTitle')}
        </h3>
        <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
          {$_('config.mibDesc')}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={handleOpenMIBTree}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm cursor-pointer"
        >
          <FolderTree class="h-3.5 w-3.5 text-teal-500" />
          {$_('config.mibTree')}
        </button>
        <button
          type="button"
          disabled={mibLoading}
          onclick={handleReloadMIB}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm disabled:opacity-50 cursor-pointer"
        >
          <RefreshCw class={`h-3.5 w-3.5 text-cyan-500 ${mibLoading ? 'animate-spin' : ''}`} />
          {$_('config.mibReload')}
        </button>
        <label class="flex items-center gap-1.5 rounded-xl bg-teal-600 hover:bg-teal-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm cursor-pointer transition-colors">
          <Upload class="h-3.5 w-3.5" />
          {$_('config.mibUpload')}
          <input
            type="file"
            bind:this={mibFileInput}
            accept=".txt,.mib,.asn1,.my"
            onchange={handleUploadMIB}
            class="hidden"
          />
        </label>
      </div>
    </div>

    {#if mibMsg}
      <div class="rounded-xl border border-emerald-300 bg-emerald-50 dark:border-emerald-900/60 dark:bg-emerald-950/40 p-3 text-xs text-emerald-700 dark:text-emerald-300 flex items-center gap-2">
        <CheckCircle2 class="h-4 w-4 shrink-0 text-emerald-500" />
        <span>{mibMsg}</span>
      </div>
    {/if}
    {#if mibError}
      <div class="rounded-xl border border-rose-300 bg-rose-50 dark:border-rose-900/60 dark:bg-rose-950/40 p-3 text-xs text-rose-700 dark:text-rose-300 flex items-center gap-2">
        <X class="h-4 w-4 shrink-0 text-rose-500" />
        <span>{mibError}</span>
      </div>
    {/if}

    <!-- Search and Stats Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-100 dark:border-slate-800/80">
      <div class="relative flex-1 min-w-[220px]">
        <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
        <input
          type="text"
          bind:value={mibFilter}
          oninput={() => (mibCurrentPage = 1)}
          placeholder={$_('config.mibSearch')}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div class="flex items-center gap-3">
        <select
          bind:value={mibPageSize}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2.5 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          <option value={10}>10 件表示</option>
          <option value={25}>25 件表示</option>
          <option value={50}>50 件表示</option>
          <option value={100}>100 件表示</option>
          <option value={-1}>すべて表示</option>
        </select>
        <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
          {$_('config.mibTotalModules')}: <span class="font-bold text-slate-800 dark:text-slate-200">{filteredMibModules.length}</span> / {mibModules.length}
        </div>
      </div>
    </div>
  </div>

  <!-- MIB Modules Table -->
  <div class="overflow-x-auto rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm dark:shadow-lg">
    <table class="w-full min-w-[900px] whitespace-nowrap text-left text-xs">
      <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
        <tr>
          <th class="p-3 w-12 text-center" aria-sort={mibSortColumn === 'index' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
            <button
              type="button"
              class="inline-flex items-center justify-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
              onclick={() => handleMibSort('index')}
            >
              <span>#</span>
              {#if mibSortColumn === 'index'}
                {#if mibSortDirection === 'asc'}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
              {/if}
            </button>
          </th>
          <th class="p-3 w-28" aria-sort={mibSortColumn === 'type' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
            <button
              type="button"
              class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
              onclick={() => handleMibSort('type')}
            >
              <span>{$_('config.mibType')}</span>
              {#if mibSortColumn === 'type'}
                {#if mibSortDirection === 'asc'}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
              {/if}
            </button>
          </th>
          <th class="p-3 w-56" aria-sort={mibSortColumn === 'name' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
            <button
              type="button"
              class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
              onclick={() => handleMibSort('name')}
            >
              <span>{$_('config.mibName')}</span>
              {#if mibSortColumn === 'name'}
                {#if mibSortDirection === 'asc'}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
              {/if}
            </button>
          </th>
          <th class="p-3" aria-sort={mibSortColumn === 'file' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
            <button
              type="button"
              class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
              onclick={() => handleMibSort('file')}
            >
              <span>{$_('config.mibFile')}</span>
              {#if mibSortColumn === 'file'}
                {#if mibSortDirection === 'asc'}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
              {/if}
            </button>
          </th>
          <th class="p-3 w-32" aria-sort={mibSortColumn === 'status' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
            <button
              type="button"
              class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
              onclick={() => handleMibSort('status')}
            >
              <span>{$_('config.mibStatus')}</span>
              {#if mibSortColumn === 'status'}
                {#if mibSortDirection === 'asc'}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {:else}
                <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
              {/if}
            </button>
          </th>
          <th class="p-3 w-16 text-center">{$_('config.mibAction')}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
        {#each paginatedMibModules as mod, idx}
          {@const isExt = mod.type === "ext" || mod.Type === "ext"}
          {@const hasErr = Boolean(mod.error || mod.Error)}
          {@const fileName = mod.file || mod.File}
          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
            <td class="py-1 px-2 text-center text-slate-400 text-[11px]">
              {(mibPageSize === -1 ? 0 : (mibCurrentPage - 1) * mibPageSize) + idx + 1}
            </td>
            <td class="py-1 px-2">
              {#if isExt}
                <span class="inline-flex items-center gap-1 rounded-md bg-indigo-50 dark:bg-indigo-950/60 px-2 py-0.5 text-[10px] font-semibold text-indigo-700 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800/60">
                  {$_('config.mibExt')}
                </span>
              {:else}
                <span class="inline-flex items-center gap-1 rounded-md bg-teal-50 dark:bg-teal-950/60 px-2 py-0.5 text-[10px] font-semibold text-teal-700 dark:text-teal-300 border border-teal-200 dark:border-teal-800/60">
                  {$_('config.mibInt')}
                </span>
              {/if}
            </td>
            <td class="py-1 px-2 font-semibold text-slate-900 dark:text-slate-100">
              {mod.name || mod.Name || "Unknown"}
            </td>
            <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">
              {mod.file || mod.File}
            </td>
            <td class="py-1 px-2">
              {#if hasErr}
                <span class="inline-flex items-center gap-1 text-[11px] text-rose-600 dark:text-rose-400" title={mod.error || mod.Error}>
                  <AlertTriangle class="h-3.5 w-3.5 shrink-0" />
                  <span class="truncate max-w-[120px]">{mod.error || mod.Error}</span>
                </span>
              {:else}
                <span class="inline-flex items-center gap-1 text-[11px] text-emerald-600 dark:text-emerald-400">
                  <CheckCircle class="h-3.5 w-3.5 shrink-0" />
                  OK
                </span>
              {/if}
            </td>
            <td class="py-0 px-1 text-center">
              {#if isExt && fileName}
                <button
                  type="button"
                  onclick={() => handleDeleteMIB(fileName)}
                  class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                  title={$_('common.delete')}
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              {/if}
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="6" class="py-8 px-2 text-center text-slate-500">
              {#if mibLoading}
                {$_("mib.loading")}
              {:else}
                {$_("mib.noModules")}
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <ListPagination
      bind:pageSize={mibPageSize}
      bind:currentPage={mibCurrentPage}
      totalCount={sortedMibModules.length}
    />
  </div>
</div>

<!-- Sub-Modal: MIB Tree Viewer -->
{#if showMIBTreeModal}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_('config.mibTree')}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showMIBTreeModal = false)}
  >
    <div class="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3 dark:border-slate-800">
        <div class="flex items-center gap-2">
          <FolderTree class="h-4 w-4 text-teal-500" />
          <h3 class="text-xs font-bold">{$_('mib.treeStructure')}</h3>
        </div>
        <button
          type="button"
          onclick={() => (showMIBTreeModal = false)}
          class="rounded-lg p-1 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
        >
          <X class="h-4 w-4" />
        </button>
      </header>
      <div class="min-h-0 flex-1 overflow-hidden p-4">
        <MIBTree
          treeData={mibTreeData}
          heightClass="h-[60vh]"
        />
      </div>
    </div>
  </div>
{/if}
