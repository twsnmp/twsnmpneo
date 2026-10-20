<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import { fetchLocConf, saveLocConf, type LocConfEnt } from "../../api";
  import {
    locStylePresets,
    iconSizes,
  } from "../../map/locStyles";
  import { Save, AlertCircle, CheckCircle2, RotateCcw } from "@lucide/svelte";

  let {
    onSaved,
  }: {
    onSaved?: () => void;
  } = $props();

  let style = $state("");
  let center = $state("139.75,35.68");
  let zoom = $state(10);
  let iconSize = $state(24);

  let selectedPreset = $state("");
  let styleError = $state("");
  let saveSuccess = $state(false);
  let isSaving = $state(false);

  const loadConf = async () => {
    try {
      const conf = await fetchLocConf();
      style = conf.Style || "";
      center = conf.Center || "139.75,35.68";
      zoom = conf.Zoom || 10;
      iconSize = conf.IconSize || 24;
    } catch (e) {
      console.error("Failed to load GIS map configuration:", e);
    }
  };

  const onSelectPreset = (e: Event) => {
    const val = (e.target as HTMLSelectElement).value;
    if (!val) return;
    const preset = locStylePresets.find((p) => p.key === val);
    if (preset) {
      style = preset.style;
      styleError = "";
    }
    selectedPreset = "";
  };

  const validateStyle = (): boolean => {
    styleError = "";
    const trimmed = style.trim();
    if (!trimmed) return true; // empty = default OSM
    if (trimmed.startsWith("{")) {
      try {
        JSON.parse(trimmed);
        return true;
      } catch (err) {
        styleError = $_("location.styleError") || "Style must be a valid URL or JSON.";
        return false;
      }
    }
    // URL format check
    if (!trimmed.startsWith("http://") && !trimmed.startsWith("https://")) {
      styleError = $_("location.styleError") || "Style must be a valid URL (http/https) or JSON.";
      return false;
    }
    return true;
  };

  export const handleSave = async (): Promise<boolean> => {
    if (!validateStyle()) return false;
    isSaving = true;
    saveSuccess = false;
    try {
      const conf: LocConfEnt = {
        Style: style.trim(),
        Center: center.trim() || "139.75,35.68",
        Zoom: Number(zoom) || 10,
        IconSize: Number(iconSize) || 24,
      };
      const ok = await saveLocConf(conf);
      if (ok) {
        saveSuccess = true;
        setTimeout(() => (saveSuccess = false), 3000);
        if (onSaved) onSaved();
        return true;
      }
    } catch (err) {
      console.error("Failed to save loc conf:", err);
    } finally {
      isSaving = false;
    }
    return false;
  };

  onMount(() => {
    loadConf();
  });
</script>

<div class="space-y-4">
  {#if styleError}
    <div class="flex items-center gap-2 rounded-lg border border-red-500/30 bg-red-50 dark:bg-red-950/30 px-3 py-2 text-xs text-red-600 dark:text-red-400">
      <AlertCircle class="h-4 w-4 shrink-0" />
      <span>{styleError}</span>
    </div>
  {/if}

  {#if saveSuccess}
    <div class="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-50 dark:bg-emerald-950/30 px-3 py-2 text-xs text-emerald-600 dark:text-emerald-400">
      <CheckCircle2 class="h-4 w-4 shrink-0" />
      <span>{$_("location.saved") || "Saved map configuration."}</span>
    </div>
  {/if}

  <!-- Style & Presets -->
  <div class="space-y-1.5">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <label for="loc-style-input" class="text-xs font-semibold text-slate-800 dark:text-slate-200">
        {$_("location.style") || "Map Style (URL or JSON)"}
      </label>
      <div class="flex items-center gap-2">
        <span class="text-[11px] text-slate-500 dark:text-slate-400">
          {$_("location.stylePreset") || "Style Preset"}:
        </span>
        <select
          bind:value={selectedPreset}
          onchange={onSelectPreset}
          class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2.5 py-1 text-xs text-slate-800 dark:text-slate-200"
        >
          <option value="">{$_("location.selectStylePreset") || "Select preset..."}</option>
          {#each locStylePresets as p}
            <option value={p.key}>
              {$_("locale") === "ja" ? p.nameJa : p.nameEn}
            </option>
          {/each}
        </select>
      </div>
    </div>
    <textarea
      id="loc-style-input"
      bind:value={style}
      rows={6}
      placeholder="https://tile.openstreetmap.jp/styles/osm-bright-ja/style.json"
      class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 p-2.5 font-mono text-xs text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-blue-500"
    ></textarea>
  </div>

  <!-- Center, Zoom, IconSize Grid -->
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
    <!-- Center -->
    <div class="space-y-1">
      <label for="loc-center-input" class="text-xs font-semibold text-slate-800 dark:text-slate-200">
        {$_("location.center") || "Center (Lng,Lat)"}
      </label>
      <input
        id="loc-center-input"
        type="text"
        bind:value={center}
        placeholder="139.75,35.68"
        class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs text-slate-900 dark:text-slate-100"
      />
    </div>

    <!-- Zoom -->
    <div class="space-y-1">
      <label for="loc-zoom-input" class="text-xs font-semibold text-slate-800 dark:text-slate-200">
        {$_("location.zoom") || "Zoom (2-18)"}
      </label>
      <input
        id="loc-zoom-input"
        type="number"
        min={2}
        max={18}
        step={0.5}
        bind:value={zoom}
        class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs text-slate-900 dark:text-slate-100"
      />
    </div>

    <!-- Icon Size -->
    <div class="space-y-1">
      <label for="loc-icon-size-select" class="text-xs font-semibold text-slate-800 dark:text-slate-200">
        {$_("location.iconSize") || "Icon Size"}
      </label>
      <select
        id="loc-icon-size-select"
        bind:value={iconSize}
        class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs text-slate-900 dark:text-slate-100"
      >
        {#each iconSizes as s}
          <option value={s.value}>
            {$_("locale") === "ja" ? s.labelJa : s.labelEn}
          </option>
        {/each}
      </select>
    </div>
  </div>

  <!-- Actions -->
  <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
    <button
      type="button"
      onclick={loadConf}
      class="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 dark:border-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
    >
      <RotateCcw class="h-3.5 w-3.5" />
      <span>{$_("common.reset") || "Reset"}</span>
    </button>
    <button
      type="button"
      disabled={isSaving}
      onclick={handleSave}
      class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white px-4 py-1.5 text-xs font-semibold shadow-sm transition-all disabled:opacity-50 cursor-pointer"
    >
      <Save class="h-3.5 w-3.5" />
      <span>{$_("common.save") || "Save"}</span>
    </button>
  </div>
</div>
