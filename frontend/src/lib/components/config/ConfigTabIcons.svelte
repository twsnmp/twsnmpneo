<script lang="ts">
  import {
    fetchCustomIcons,
    saveCustomIcon,
    saveCustomIcons,
    deleteCustomIcon,
    type IconEnt,
  } from "../../api";
  import { setCustomIcons } from "../../common";
  import { showConfirm } from "../../stores/modalStore";
  import { allMdiIcons } from "../../mdiIcons";
  import ListPagination from "../../views/list/ListPagination.svelte";
  import {
    Sparkles,
    Plus,
    Upload,
    Download,
    Search,
    ArrowUp,
    ArrowDown,
    Edit3,
    Trash2,
    X,
    Save,
    FileUp,
    Image,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    onSuccess,
    onError,
  }: {
    onSuccess?: (msg: string) => void;
    onError?: (msg: string) => void;
  } = $props();

  let customIcons = $state<IconEnt[]>([]);
  let iconLoading = $state(false);
  let iconFilter = $state("");
  let iconSortColumn = $state<"name" | "type" | "code" | null>(null);
  let iconSortDirection = $state<"asc" | "desc">("asc");
  let iconPageSize = $state(10);
  let iconCurrentPage = $state(1);

  // Icon Dialog state
  let showIconDialog = $state(false);
  let editingIcon = $state<IconEnt | null>(null);
  let dialogIconType = $state<"mdi" | "image">("mdi");
  let dialogIconImage = $state("");
  let dialogMdiFilter = $state("");
  let dialogSelectedMdi = $state("access-point");
  let dialogIconName = $state("");
  let dialogIconCode = $state<string | number>(983043);
  let dialogIconError = $state("");
  let dialogParsedCode = $derived(parseIconCode(dialogIconCode));

  let iconFileInput = $state<HTMLInputElement | null>(null);
  let imageFileInput = $state<HTMLInputElement | null>(null);

  const filteredMdiList = $derived.by(() => {
    const q = dialogMdiFilter.trim().toLowerCase();
    if (!q) {
      return allMdiIcons.slice(0, 300);
    }
    return allMdiIcons.filter(ic => ic.name.toLowerCase().includes(q)).slice(0, 500);
  });

  function parseIconCode(input: string | number): number {
    if (typeof input === "number") return input;
    const s = String(input).trim();
    if (!s) return 0;
    if (s.startsWith("0x") || s.startsWith("0X")) {
      return parseInt(s, 16) || 0;
    }
    if (/^[0-9a-fA-F]{4,6}$/.test(s) && (s.toUpperCase().startsWith("F") || s.toUpperCase().startsWith("E"))) {
      return parseInt(s, 16) || 0;
    }
    return parseInt(s, 10) || 0;
  }

  function formatCodePreview(codeNum: number): string {
    if (!codeNum || codeNum <= 0) return "-";
    try {
      return String.fromCodePoint(codeNum);
    } catch {
      return "?";
    }
  }

  export async function loadIcons() {
    iconLoading = true;
    try {
      const list = await fetchCustomIcons();
      customIcons = list || [];
      setCustomIcons(customIcons);
    } catch (e: any) {
      console.error("Failed to load custom icons:", e);
      onError?.(e.message || String(e));
    } finally {
      iconLoading = false;
    }
  }

  function handleSelectMdi(mdiName: string) {
    const found = allMdiIcons.find((m) => m.name === mdiName);
    if (found) {
      dialogSelectedMdi = found.name;
      dialogIconCode = found.code;
      if (!editingIcon) {
        dialogIconName = found.name;
      }
    }
  }

  function openAddIconDialog() {
    editingIcon = null;
    dialogIconType = "mdi";
    dialogIconImage = "";
    dialogMdiFilter = "";
    dialogSelectedMdi = "access-point";
    dialogIconCode = 983043;
    dialogIconName = "access-point";
    dialogIconError = "";
    showIconDialog = true;
  }

  function openEditIconDialog(item: IconEnt) {
    editingIcon = item;
    const nameVal = item.name || item.Name || "";
    const codeVal = Number(item.code ?? item.Code ?? 0);
    const typeVal = (item.type || item.Type || (item.image || item.Image ? "image" : "mdi")) as "mdi" | "image";
    const imageVal = item.image || item.Image || "";
    dialogIconType = typeVal;
    dialogIconImage = imageVal;
    dialogIconName = nameVal;
    dialogIconCode = codeVal;
    if (typeVal === "mdi") {
      const found = allMdiIcons.find((m) => m.code === codeVal || m.name === nameVal);
      dialogSelectedMdi = found ? found.name : "";
      dialogMdiFilter = found ? found.name : "";
    } else {
      dialogSelectedMdi = "";
      dialogMdiFilter = "";
    }
    dialogIconError = "";
    showIconDialog = true;
  }

  function handleImageFileSelected(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    if (file.size > 2 * 1024 * 1024) {
      dialogIconError = $_('config.iconImageSizeError');
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      dialogIconImage = reader.result as string;
      if (!dialogIconName) {
        const base = file.name.replace(/\.[^/.]+$/, "");
        dialogIconName = base;
      }
      dialogIconError = "";
    };
    reader.onerror = () => {
      dialogIconError = $_('config.iconImageReadError');
    };
    reader.readAsDataURL(file);
  }

  async function handleSaveIcon() {
    const name = dialogIconName.trim();
    if (!name) {
      dialogIconError = $_('config.iconName') + " is required";
      return;
    }
    if (dialogIconType === "image") {
      if (!dialogIconImage) {
        dialogIconError = $_('config.iconSelectImageFile');
        return;
      }
      try {
        await saveCustomIcon({
          id: editingIcon?.id || editingIcon?.ID,
          name,
          code: 0,
          type: "image",
          image: dialogIconImage,
        });
        showIconDialog = false;
        await loadIcons();
        onSuccess?.($_('config.iconSaveSuccess'));
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } catch (e: any) {
        dialogIconError = e.message || String(e);
      }
    } else {
      const code = parseIconCode(dialogIconCode);
      if (!code || code <= 0) {
        dialogIconError = $_('config.iconCode') + " is required";
        return;
      }
      try {
        await saveCustomIcon({
          id: editingIcon?.id || editingIcon?.ID,
          name,
          code,
          type: "mdi",
          image: "",
        });
        showIconDialog = false;
        await loadIcons();
        onSuccess?.($_('config.iconSaveSuccess'));
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } catch (e: any) {
        dialogIconError = e.message || String(e);
      }
    }
  }

  async function handleDeleteIcon(item: IconEnt) {
    const name = item.name || item.Name || "";
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'アイコン削除の確認',
      message: $_('config.iconDeleteConfirm', { values: { name } }),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    try {
      await deleteCustomIcon(name);
      await loadIcons();
      onSuccess?.($_('config.iconDeleteSuccess'));
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
      }
    } catch (e: any) {
      onError?.(e.message || String(e));
    }
  }

  async function handleImportIcons(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    try {
      const text = await file.text();
      let list: Partial<IconEnt>[] = [];
      if (file.name.endsWith(".json")) {
        const parsed = JSON.parse(text);
        if (Array.isArray(parsed)) {
          list = parsed.map((item: any) => {
            const isImg = item.type === "image" || item.Type === "image" || Boolean(item.image || item.Image);
            return {
              name: item.name || item.Name || "",
              code: isImg ? 0 : parseIconCode(item.code ?? item.Code),
              type: (isImg ? "image" : "mdi") as IconEnt["type"],
              image: item.image || item.Image || "",
            };
          }).filter((x) => x.name && (x.type === "image" ? Boolean(x.image) : Boolean(x.code)));
        }
      } else {
        const lines = text.split(/\r?\n/);
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("名前") || trimmed.toLowerCase().startsWith("name")) continue;
          const parts = trimmed.split(/,|\t/).map((p) => p.replace(/^["']|["']$/g, '').trim());
          if (parts.length >= 2) {
            const name = parts[0];
            const code = parseIconCode(parts[1]);
            if (name && code) {
              list.push({ name, code, type: "mdi" });
            }
          }
        }
      }
      if (list.length > 0) {
        await saveCustomIcons(list);
        await loadIcons();
        onSuccess?.($_('config.iconImportSuccess', { values: { count: list.length } }));
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } else {
        onError?.($_('config.iconImportFailed'));
      }
    } catch (err: any) {
      onError?.(`${$_('config.iconImportFailed')}: ${err.message || err}`);
    } finally {
      if (iconFileInput) iconFileInput.value = "";
    }
  }

  function handleExportIcons() {
    const data = JSON.stringify(customIcons, null, 2);
    const blob = new Blob([data], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `twsnmp_icons_${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function handleIconSort(col: "name" | "type" | "code") {
    if (iconSortColumn === col) {
      iconSortDirection = iconSortDirection === "asc" ? "desc" : "asc";
    } else {
      iconSortColumn = col;
      iconSortDirection = "asc";
    }
  }

  const filteredCustomIcons = $derived.by(() => {
    const q = iconFilter.trim().toLowerCase();
    if (!q) return customIcons;
    return customIcons.filter((ic) => {
      const name = (ic.name || ic.Name || "").toLowerCase();
      const type = (ic.type || ic.Type || (ic.image || ic.Image ? "image" : "mdi")).toLowerCase();
      const code = String(ic.code ?? ic.Code ?? "");
      const hexCode = "0x" + (Number(ic.code ?? ic.Code ?? 0)).toString(16).toLowerCase();
      return name.includes(q) || type.includes(q) || code.includes(q) || hexCode.includes(q);
    });
  });

  const sortedCustomIcons = $derived.by(() => {
    const list = [...filteredCustomIcons];
    if (!iconSortColumn) return list;
    const dir = iconSortDirection === "asc" ? 1 : -1;
    return list.sort((a, b) => {
      if (iconSortColumn === "name") {
        const na = a.name || a.Name || "";
        const nb = b.name || b.Name || "";
        return dir * na.localeCompare(nb, undefined, { numeric: true });
      } else if (iconSortColumn === "type") {
        const ta = a.type || a.Type || (a.image || a.Image ? "image" : "mdi");
        const tb = b.type || b.Type || (b.image || b.Image ? "image" : "mdi");
        return dir * ta.localeCompare(tb);
      } else if (iconSortColumn === "code") {
        const ca = Number(a.code ?? a.Code ?? 0);
        const cb = Number(b.code ?? b.Code ?? 0);
        return dir * (ca - cb);
      }
      return 0;
    });
  });

  const paginatedCustomIcons = $derived.by(() => {
    if (iconPageSize === -1) return sortedCustomIcons;
    const totalPages = Math.max(1, Math.ceil(sortedCustomIcons.length / iconPageSize));
    const page = Math.min(Math.max(1, iconCurrentPage), totalPages);
    const start = (page - 1) * iconPageSize;
    return sortedCustomIcons.slice(start, start + iconPageSize);
  });

  $effect(() => {
    loadIcons();
  });
</script>

<div class="space-y-6">
  <!-- Header info card -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Sparkles class="h-4 w-4 text-cyan-500" />
          {$_('config.iconTitle')}
        </h3>
        <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
          {$_('config.iconDesc')}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          onclick={openAddIconDialog}
          class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition-colors cursor-pointer"
        >
          <Plus class="h-3.5 w-3.5" />
          {$_('config.iconAdd')}
        </button>
        <label class="flex items-center gap-1.5 rounded-xl bg-teal-600 hover:bg-teal-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm cursor-pointer transition-colors">
          <Upload class="h-3.5 w-3.5" />
          {$_('config.iconImport')}
          <input
            type="file"
            bind:this={iconFileInput}
            accept=".json,.csv,.txt"
            onchange={handleImportIcons}
            class="hidden"
          />
        </label>
        <button
          type="button"
          onclick={handleExportIcons}
          disabled={customIcons.length === 0}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm disabled:opacity-50 cursor-pointer"
        >
          <Download class="h-3.5 w-3.5 text-blue-500" />
          {$_('config.iconExport')}
        </button>
      </div>
    </div>

    <!-- Search and Stats Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-100 dark:border-slate-800/80">
      <div class="relative flex-1 min-w-[220px]">
        <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
        <input
          type="text"
          bind:value={iconFilter}
          oninput={() => (iconCurrentPage = 1)}
          placeholder={$_('config.iconSearch')}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div class="flex items-center gap-3">
        <select
          bind:value={iconPageSize}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2.5 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          {#each [10, 25, 50, 100, -1] as size}
            {#if size === -1}
              <option value={-1}>{$_('common.showAll')}</option>
            {:else}
              <option value={size}>{$_('common.showCount', { values: { count: size } })}</option>
            {/if}
          {/each}
        </select>
        <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
          {$_('config.iconTotalCount')}: <span class="font-bold text-slate-800 dark:text-slate-200">{filteredCustomIcons.length}</span> / {customIcons.length}
        </div>
      </div>
    </div>
  </div>

  <!-- Icon Table -->
  <div class="overflow-x-auto rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm dark:shadow-lg">
    <table class="w-full min-w-[600px] whitespace-nowrap text-left text-xs">
      <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
        <tr>
          <th class="p-3 w-16 text-center">{$_('config.iconColIcon')}</th>
          <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("name")}>
            <div class="flex items-center gap-1.5">
              <span>{$_('config.iconColName')}</span>
              {#if iconSortColumn === "name"}
                {#if iconSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {/if}
            </div>
          </th>
          <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("type")}>
            <div class="flex items-center gap-1.5">
              <span>{$_('config.iconColType')}</span>
              {#if iconSortColumn === "type"}
                {#if iconSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {/if}
            </div>
          </th>
          <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("code")}>
            <div class="flex items-center gap-1.5">
              <span>{$_('config.iconColCode')}</span>
              {#if iconSortColumn === "code"}
                {#if iconSortDirection === "asc"}
                  <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {:else}
                  <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                {/if}
              {/if}
            </div>
          </th>
          <th class="p-3 text-center w-28">{$_('config.iconColAction')}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
        {#each paginatedCustomIcons as ic (ic.id || ic.ID || ic.name || ic.Name)}
          {@const codeVal = Number(ic.code ?? ic.Code ?? 0)}
          {@const nameVal = ic.name || ic.Name || ""}
          {@const typeVal = ic.type || ic.Type || (ic.image || ic.Image ? "image" : "mdi")}
          {@const isImg = typeVal === "image" || Boolean(ic.image || ic.Image)}
          {@const imgVal = ic.image || ic.Image || ""}
          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
            <td class="px-2 py-1 text-center">
              {#if isImg}
                <div class="w-8 h-8 mx-auto rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 p-0.5 flex items-center justify-center overflow-hidden">
                  <img src={imgVal} class="max-w-full max-h-full object-contain" alt={nameVal} />
                </div>
              {:else}
                <span class="text-2xl text-cyan-500 dark:text-cyan-400 inline-block align-middle" style="font-family: 'Material Design Icons'">
                  {formatCodePreview(codeVal)}
                </span>
              {/if}
            </td>
            <td class="px-2 py-1 font-semibold text-slate-800 dark:text-slate-100 font-sans">
              {nameVal}
            </td>
            <td class="px-2 py-1 font-sans">
              {#if isImg}
                <span class="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20">
                  {$_('config.iconTypeImage')}
                </span>
              {:else}
                <span class="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
                  {$_('config.iconTypeMdi')}
                </span>
              {/if}
            </td>
            <td class="px-2 py-1 text-slate-600 dark:text-slate-300">
              {#if isImg}
                <span class="text-[11px] text-slate-400 font-sans">{$_('config.iconTypeImage')}</span>
              {:else}
                <span>{codeVal}</span>
                <span class="ml-2 text-[11px] text-slate-400">(0x{codeVal.toString(16).toUpperCase()})</span>
              {/if}
            </td>
            <td class="px-1 py-0 text-center">
              <div class="flex items-center justify-center font-sans">
                <button
                  type="button"
                  onclick={() => openEditIconDialog(ic)}
                  class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                  title={$_('config.iconEdit')}
                >
                  <Edit3 class="h-4 w-4" />
                </button>
                <button
                  type="button"
                  onclick={() => handleDeleteIcon(ic)}
                  class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                  title={$_('config.iconDelete')}
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="5" class="py-10 px-4 text-center text-slate-400 font-sans">
              {#if iconLoading}
                {$_('common.loading')}
              {:else}
                {$_('config.iconNoData')}
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
    <ListPagination
      bind:pageSize={iconPageSize}
      bind:currentPage={iconCurrentPage}
      totalCount={sortedCustomIcons.length}
    />
  </div>
</div>

<!-- Sub-Modal: Add / Edit Custom Icon -->
{#if showIconDialog}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={editingIcon ? $_('config.iconEditTitle') : $_('config.iconAddTitle')}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showIconDialog = false)}
  >
    <div class="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-6 py-4 dark:border-slate-800">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/20 text-cyan-400">
            <Sparkles class="h-4 w-4" />
          </div>
          <h3 class="text-sm font-bold">{editingIcon ? $_('config.iconEditTitle') : $_('config.iconAddTitle')}</h3>
        </div>
        <button
          type="button"
          onclick={() => (showIconDialog = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <div class="p-6 space-y-4 overflow-y-auto text-xs">
        {#if dialogIconError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3 text-xs font-medium text-rose-300">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{dialogIconError}</span>
          </div>
        {/if}

        <!-- Icon Type Selector Tab -->
        {#if !editingIcon}
          <div>
            <span class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconType')}
            </span>
            <div class="flex rounded-xl bg-slate-100 dark:bg-slate-950 p-1 border border-slate-200 dark:border-slate-800">
              <button
                type="button"
                onclick={() => { dialogIconType = "mdi"; }}
                class="flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all cursor-pointer {dialogIconType === 'mdi' ? 'bg-cyan-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                {$_('config.iconTypeMdi')}
              </button>
              <button
                type="button"
                onclick={() => { dialogIconType = "image"; }}
                class="flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all cursor-pointer {dialogIconType === 'image' ? 'bg-purple-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                {$_('config.iconTypeImage')}
              </button>
            </div>
          </div>
        {/if}

        {#if dialogIconType === "mdi"}
          {#if !editingIcon}
            <!-- Icon Filter Keyword -->
            <div>
              <label for="dialog-icon-filter" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_('config.iconFilterKeyword')}
              </label>
              <div class="relative">
                <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  id="dialog-icon-filter"
                  type="text"
                  bind:value={dialogMdiFilter}
                  placeholder={$_('config.iconFilterPlaceholder')}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 pl-8 pr-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>

            <!-- MDI Icon Selection Dropdown -->
            <div>
              <div class="flex items-center justify-between mb-1.5">
                <label for="dialog-icon-select" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                  {$_('config.iconSelect')} <span class="text-rose-500">*</span>
                </label>
                <span class="text-[11px] text-slate-400 font-mono">
                  {$_('config.iconMatchedCount', { values: { count: filteredMdiList.length } })}
                </span>
              </div>
              <select
                id="dialog-icon-select"
                value={dialogSelectedMdi}
                onchange={(e) => handleSelectMdi((e.target as HTMLSelectElement).value)}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none font-mono"
              >
                {#if filteredMdiList.length === 0}
                  <option value="" disabled>{$_('config.iconNoMatch')}</option>
                {:else}
                  {#each filteredMdiList as mdi}
                    <option value={mdi.name}>
                      {mdi.name} (0x{mdi.code.toString(16).toUpperCase()})
                    </option>
                  {/each}
                {/if}
              </select>
            </div>
          {/if}

          <!-- Custom Icon Name -->
          <div>
            <label for="dialog-icon-name" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconName')} <span class="text-rose-500">*</span>
            </label>
            <input
              id="dialog-icon-name"
              type="text"
              bind:value={dialogIconName}
              placeholder={$_('config.iconNamePlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <!-- Live Glyph Preview -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 flex items-center gap-4">
            <div class="w-16 h-16 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-4xl text-cyan-500 dark:text-cyan-400 shrink-0 shadow-inner">
              <span style="font-family: 'Material Design Icons'">{formatCodePreview(dialogParsedCode)}</span>
            </div>
            <div class="space-y-1 min-w-0">
              <div class="text-xs font-bold text-slate-800 dark:text-slate-200">
                {dialogSelectedMdi || dialogIconName || $_('config.iconPreview')}
              </div>
              <div class="text-[11px] font-mono text-slate-500">{$_('config.iconDecimal')} <span class="text-cyan-600 dark:text-cyan-400 font-bold">{dialogParsedCode || 0}</span></div>
              <div class="text-[11px] font-mono text-slate-500">{$_('config.iconHex')} <span class="text-emerald-600 dark:text-emerald-400 font-bold">0x{(dialogParsedCode || 0).toString(16).toUpperCase()}</span></div>
            </div>
          </div>
        {:else}
          <!-- Image Icon Mode -->
          <div>
            <span class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconSelectImageFile')} <span class="text-rose-500">*</span>
            </span>
            <div class="flex items-center gap-3">
              <input
                type="file"
                bind:this={imageFileInput}
                accept="image/*"
                onchange={handleImageFileSelected}
                class="hidden"
              />
              <button
                type="button"
                onclick={() => imageFileInput?.click()}
                class="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-xl text-xs font-semibold transition-colors cursor-pointer"
              >
                <FileUp class="h-4 w-4" />
                {$_('config.iconSelectImageFile')}
              </button>
              <span class="text-[11px] text-slate-500 dark:text-slate-400">
                PNG, JPG, SVG, WebP, GIF (Max 2MB)
              </span>
            </div>
          </div>

          <!-- Custom Icon Name -->
          <div>
            <label for="dialog-icon-name-img" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconName')} <span class="text-rose-500">*</span>
            </label>
            <input
              id="dialog-icon-name-img"
              type="text"
              bind:value={dialogIconName}
              placeholder={$_('config.iconNamePlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <!-- Live Image Preview -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 flex items-center gap-4">
            <div class="w-16 h-16 rounded-xl bg-purple-500/10 border border-purple-500/30 flex items-center justify-center shrink-0 shadow-inner overflow-hidden p-1">
              {#if dialogIconImage}
                <img src={dialogIconImage} class="max-w-full max-h-full object-contain" alt="Preview" />
              {:else}
                <Image class="h-8 w-8 text-slate-400" />
              {/if}
            </div>
            <div class="space-y-1 min-w-0">
              <div class="text-xs font-bold text-slate-800 dark:text-slate-200 truncate">
                {dialogIconName || $_('config.iconPreview')}
              </div>
              <div class="text-[11px] text-slate-500">
                {dialogIconImage ? $_('config.iconTypeImage') : $_('config.iconDropImageHint')}
              </div>
            </div>
          </div>
        {/if}
      </div>

      <footer class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-3.5 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60">
        <button
          type="button"
          onclick={() => (showIconDialog = false)}
          class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-800 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={handleSaveIcon}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-1.5 transition-all cursor-pointer"
        >
          <Save class="w-4 h-4" />
          {$_('common.save')}
        </button>
      </footer>
    </div>
  </div>
{/if}
