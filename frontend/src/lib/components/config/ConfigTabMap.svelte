<script lang="ts">
  import { uploadBackImage, deleteBackImage } from "../../api";
  import {
    Sliders,
    Image,
    Link,
    Unlink,
    Maximize2,
    Upload,
    Trash2,
    FileUp,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    mapName = $bindable("TWSNMP NEO"),
    mapSize = $bindable(0),
    iconSize = $bindable(3),
    backImageX = $bindable(0),
    backImageY = $bindable(0),
    backImageW = $bindable(800),
    backImageH = $bindable(600),
    backImagePath = $bindable(""),
    imgNaturalW = $bindable(0),
    imgNaturalH = $bindable(0),
    lockRatio = $bindable(true),
    onOpenImportMap,
    onError,
  }: {
    mapName: string;
    mapSize: number;
    iconSize: number;
    backImageX: number;
    backImageY: number;
    backImageW: number;
    backImageH: number;
    backImagePath: string;
    imgNaturalW: number;
    imgNaturalH: number;
    lockRatio: boolean;
    onOpenImportMap?: () => void;
    onError?: (msg: string) => void;
  } = $props();

  let backImageUploading = $state(false);
  let backImageFileInput: HTMLInputElement | null = $state(null);

  let curMapW = $derived(
    mapSize === 1 ? 2894 : mapSize === 2 ? 4093 : (typeof window !== "undefined" && window.screen?.width > 4000 ? 5000 : 2500)
  );
  let curMapH = $derived(mapSize === 1 ? 4093 : mapSize === 2 ? 2894 : 5000);

  let previewScale = $derived(
    Math.min(
      340 / (curMapW || 2500),
      150 / (curMapH || 5000)
    )
  );
  let previewCanvasW = $derived(Math.max(40, Math.round((curMapW || 2500) * previewScale)));
  let previewCanvasH = $derived(Math.max(40, Math.round((curMapH || 5000) * previewScale)));

  function updateImageNaturalSize(src: string, forceResetSize: boolean = false) {
    if (!src) {
      imgNaturalW = 0;
      imgNaturalH = 0;
      return;
    }
    const img = new window.Image();
    img.onload = () => {
      imgNaturalW = img.naturalWidth;
      imgNaturalH = img.naturalHeight;
      if (forceResetSize || !backImageW || !backImageH || backImageW <= 0 || backImageH <= 0) {
        backImageW = img.naturalWidth;
        backImageH = img.naturalHeight;
      }
    };
    img.src = src;
  }

  function handleWidthChange(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value, 10) || 0;
    backImageW = val;
    if (lockRatio && imgNaturalW > 0 && imgNaturalH > 0 && val > 0) {
      backImageH = Math.round((val * imgNaturalH) / imgNaturalW);
    }
  }

  function handleHeightChange(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value, 10) || 0;
    backImageH = val;
    if (lockRatio && imgNaturalW > 0 && imgNaturalH > 0 && val > 0) {
      backImageW = Math.round((val * imgNaturalW) / imgNaturalH);
    }
  }

  function resetToOriginalSize() {
    if (imgNaturalW > 0 && imgNaturalH > 0) {
      backImageW = imgNaturalW;
      backImageH = imgNaturalH;
    }
  }

  function setScaleMultiplier(mult: number) {
    if (imgNaturalW > 0 && imgNaturalH > 0) {
      backImageW = Math.round(imgNaturalW * mult);
      backImageH = Math.round(imgNaturalH * mult);
    }
  }

  function fitToMapWidth() {
    if (curMapW > 0) {
      backImageW = curMapW;
      if (imgNaturalW > 0 && imgNaturalH > 0) {
        backImageH = Math.round((curMapW * imgNaturalH) / imgNaturalW);
      }
    }
  }

  function centerOnMap() {
    backImageX = Math.max(0, Math.round((curMapW - backImageW) / 2));
    backImageY = Math.max(0, Math.round((curMapH - backImageH) / 2));
  }

  function alignTopLeft() {
    backImageX = 0;
    backImageY = 0;
  }

  async function handleUploadBackImageFile(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    backImageUploading = true;
    try {
      const file = target.files[0];
      const localUrl = URL.createObjectURL(file);
      updateImageNaturalSize(localUrl, true);
      const res = await uploadBackImage(file);
      if (res?.path) {
        backImagePath = res.path;
      }
    } catch (err: any) {
      onError?.(`${$_('config.backImageUploadFailed')}: ${err.message || err}`);
    } finally {
      backImageUploading = false;
    }
  }

  async function handleClearBackImage() {
    backImagePath = "";
    imgNaturalW = 0;
    imgNaturalH = 0;
    backImageX = 0;
    backImageY = 0;
    backImageW = 0;
    backImageH = 0;
    await deleteBackImage().catch(console.error);
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
    }
  }
</script>

<div class="space-y-6 max-w-2xl">
  <!-- Map Base Settings -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Sliders class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.tabMapTitle')}
    </h3>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label for="map-name" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mapName')}</label>
        <input
          id="map-name"
          type="text"
          bind:value={mapName}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="map-size" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mapSize')}</label>
        <select
          id="map-size"
          bind:value={mapSize}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          <option value={0}>{$_('config.mapSizeAuto')}</option>
          <option value={1}>A4P (2894x4093)</option>
          <option value={2}>A4L (4093x2894)</option>
        </select>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 pt-1">
      <div>
        <label for="icon-size" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
          {$_('config.iconSize')}: <span class="font-mono text-cyan-600 dark:text-cyan-400">{iconSize}</span> ({$_('config.iconSizeDesc')})
        </label>
        <input
          id="icon-size"
          type="range"
          min={1}
          max={5}
          bind:value={iconSize}
          class="w-full accent-cyan-500 cursor-pointer"
        />
      </div>
    </div>
  </div>

  <!-- Background Image Section -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <div class="flex items-center gap-2">
        <Image class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
          {$_('config.backImageTitle')}
        </h3>
        {#if imgNaturalW > 0}
          <span class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
            ({$_('config.originalSize')}: {imgNaturalW} × {imgNaturalH} px)
          </span>
        {/if}
      </div>
      {#if backImagePath}
        <span class="inline-flex items-center gap-1.5 rounded-full border border-cyan-500/30 bg-cyan-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-cyan-600 dark:text-cyan-400">
          {$_('config.backImageConfigured')}
        </span>
      {/if}
    </div>

    <!-- Dimension and Position Controls -->
    <div class="space-y-3">
      <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div>
          <label for="bg-x" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">X (px)</label>
          <input
            id="bg-x"
            type="number"
            bind:value={backImageX}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
          />
        </div>
        <div>
          <label for="bg-y" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Y (px)</label>
          <input
            id="bg-y"
            type="number"
            bind:value={backImageY}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
          />
        </div>
        <div>
          <div class="flex items-center justify-between mb-1">
            <label for="bg-w" class="text-xs font-semibold text-slate-600 dark:text-slate-400">{$_('drawItem.width')}</label>
            <button
              type="button"
              onclick={() => (lockRatio = !lockRatio)}
              title={lockRatio ? $_('config.unlockRatio') : $_('config.lockRatio')}
              class="text-[10px] p-0.5 rounded transition-colors {lockRatio ? 'text-cyan-600 dark:text-cyan-400 hover:text-cyan-500' : 'text-slate-400 hover:text-slate-600'}"
            >
              {#if lockRatio}
                <Link class="w-3.5 h-3.5" />
              {:else}
                <Unlink class="w-3.5 h-3.5" />
              {/if}
            </button>
          </div>
          <input
            id="bg-w"
            type="number"
            value={backImageW}
            oninput={handleWidthChange}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
          />
        </div>
        <div>
          <label for="bg-h" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('drawItem.height')}</label>
          <input
            id="bg-h"
            type="number"
            value={backImageH}
            oninput={handleHeightChange}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
          />
        </div>
      </div>

      <!-- Quick Presets -->
      {#if backImagePath}
        <div class="flex flex-wrap items-center gap-1.5 pt-1 text-[11px]">
          <span class="text-slate-500 font-medium">{$_('config.presets')}:</span>
          <button
            type="button"
            onclick={resetToOriginalSize}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            100% ({$_('config.originalSize')})
          </button>
          <button
            type="button"
            onclick={() => setScaleMultiplier(0.5)}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            50%
          </button>
          <button
            type="button"
            onclick={() => setScaleMultiplier(2.0)}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            200%
          </button>
          <button
            type="button"
            onclick={fitToMapWidth}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            {$_('config.fitMapWidth')}
          </button>
          <span class="text-slate-300 dark:text-slate-700">|</span>
          <button
            type="button"
            onclick={alignTopLeft}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            (0, 0)
          </button>
          <button
            type="button"
            onclick={centerOnMap}
            class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
          >
            {$_('config.centerMap')}
          </button>
        </div>

        <!-- Mini Canvas Placement Visualizer -->
        <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950 p-3 space-y-2">
          <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400">
            <span class="font-semibold flex items-center gap-1.5">
              <Maximize2 class="w-3.5 h-3.5 text-cyan-500" />
              {$_('config.placementPreview')}
            </span>
            <span class="font-mono text-[10px]">
              {$_('config.mapCanvas')} {curMapW} × {curMapH} px
            </span>
          </div>

          {#if backImageW <= 0 || backImageH <= 0}
            <div class="text-[11px] text-amber-500 dark:text-amber-400 bg-amber-500/10 border border-amber-500/20 rounded-lg p-2">
              {$_('config.backImageZeroWarning')}
            </div>
          {/if}

          <!-- Simulated miniature map canvas container -->
          <div class="relative w-full h-44 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-900/60 overflow-hidden flex items-center justify-center p-3">
            <!-- Canvas Boundary Visual -->
            <div
              class="relative bg-white dark:bg-slate-950 border-2 border-slate-400 dark:border-slate-600 rounded shadow-md overflow-hidden transition-all"
              style="width: {previewCanvasW}px; height: {previewCanvasH}px;"
            >
              <!-- Placed Image Visual Box -->
              <div
                class="absolute border-2 border-cyan-500 bg-cyan-500/20 overflow-hidden transition-all flex items-center justify-center"
                style="
                  left: {((Number(backImageX) || 0) / curMapW) * 100}%;
                  top: {((Number(backImageY) || 0) / curMapH) * 100}%;
                  width: {((Number(backImageW) || 0) / curMapW) * 100}%;
                  height: {((Number(backImageH) || 0) / curMapH) * 100}%;
                "
              >
                <img src={backImagePath} alt="BackImage" class="w-full h-full object-fill opacity-75 pointer-events-none" />
              </div>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <div class="flex items-center gap-3 pt-2">
      <input
        type="file"
        accept="image/*"
        bind:this={backImageFileInput}
        onchange={handleUploadBackImageFile}
        class="hidden"
        id="backimage-file-input"
      />
      <label
        for="backimage-file-input"
        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold text-cyan-600 dark:text-cyan-300 transition-colors cursor-pointer {backImageUploading ? 'opacity-50 pointer-events-none' : ''}"
      >
        <Upload class="w-3.5 h-3.5" />
        <span>{backImageUploading ? $_('common.loading') : $_('config.backImageSelect')}</span>
      </label>

      {#if backImagePath}
        <button
          type="button"
          onclick={handleClearBackImage}
          class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-600 dark:text-rose-300 transition-colors cursor-pointer"
        >
          <Trash2 class="w-3.5 h-3.5" />
          <span>{$_('config.backImageClear')}</span>
        </button>
      {/if}
    </div>
  </div>

  <!-- Map Import Section -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <FileUp class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        {$_('config.importMapTitle')}
      </h3>
    </div>
    <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
      {$_('config.importMapDesc')}
    </p>
    <div class="pt-1">
      <button
        type="button"
        onclick={() => onOpenImportMap?.()}
        class="flex items-center gap-2 rounded-xl border border-cyan-500/40 bg-cyan-50 dark:bg-cyan-950/40 px-4 py-2 text-xs font-bold text-cyan-700 dark:text-cyan-300 hover:bg-cyan-100 dark:hover:bg-cyan-900/50 transition-colors cursor-pointer"
      >
        <FileUp class="w-4 h-4" />
        <span>{$_('config.importMapButton')}</span>
      </button>
    </div>
  </div>
</div>
