<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    Cpu,
    Database,
    Download,
    Trash2,
    CheckCircle2,
    AlertCircle,
    X,
    Gauge,
    Sparkles,
    Loader2,
  } from "@lucide/svelte";
  import {
    fetchAIHardwareStatus,
    fetchLocalModels,
    fetchModelPresets,
    downloadModel,
    cancelModelDownload,
    deleteLocalModel,
    downloadGPULibrary,
    cancelGPUDownload,
    fetchAIDownloadStatus,
    type AIHardwareStatus,
    type ModelInfo,
    type PresetModelInfo,
    type AIDownloadStatus,
  } from "../api";
  import { showConfirm } from "../stores/modalStore";

  interface Props {
    show: boolean;
    onclose?: () => void;
    onmodelsChanged?: () => void;
  }

  let { show = $bindable(false), onclose, onmodelsChanged }: Props = $props();

  let hardwareStatus = $state<AIHardwareStatus>({
    acceleration: "",
    detail: "",
    model_dir: "",
    lib_dir: "",
    wgpu_lib_path: "",
    has_gpu_lib: false,
  });

  let localModels = $state<ModelInfo[]>([]);
  let presets = $state<PresetModelInfo[]>([]);
  let selectedPreset = $state("");
  let customTarget = $state("");

  let downloadStatus = $state<AIDownloadStatus>({
    model: { downloading: false, downloaded: 0, total: 0, percent: 0, downloaded_human: "0 B", total_human: "0 B" },
    gpu: { downloading: false, downloaded: 0, total: 0, percent: 0, downloaded_human: "0 B", total_human: "0 B" },
  });

  let errorMsg = $state("");
  let infoMsg = $state("");
  let pollTimer: any = null;

  async function refreshData() {
    try {
      hardwareStatus = await fetchAIHardwareStatus();
      localModels = (await fetchLocalModels()) || [];
      presets = (await fetchModelPresets()) || [];
      if (presets.length > 0 && !selectedPreset) {
        selectedPreset = presets[0].name;
      }
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function checkDownloadStatus() {
    try {
      const st = await fetchAIDownloadStatus();
      const prevModelDownloading = downloadStatus.model.downloading;
      const prevGPUDownloading = downloadStatus.gpu.downloading;

      downloadStatus = st;

      // When model download finishes
      if (prevModelDownloading && !st.model.downloading) {
        if (st.model.error) {
          errorMsg = st.model.error;
        } else {
          infoMsg = $_('modelManager.modelDownloadSuccess') || "Model downloaded successfully.";
          await refreshData();
          onmodelsChanged?.();
        }
      }

      // When GPU download finishes
      if (prevGPUDownloading && !st.gpu.downloading) {
        if (st.gpu.error) {
          errorMsg = st.gpu.error;
        } else {
          infoMsg = $_('modelManager.gpuDownloadSuccess') || "GPU library installed successfully.";
          await refreshData();
        }
      }
    } catch {
      // Ignore polling errors
    }
  }

  $effect(() => {
    if (show) {
      errorMsg = "";
      infoMsg = "";
      refreshData();
      checkDownloadStatus();
      if (!pollTimer) {
        pollTimer = setInterval(() => {
          if (show) {
            checkDownloadStatus();
          }
        }, 1000);
      }
    } else {
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    }
  });

  onDestroy(() => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });

  async function handleDownloadPreset() {
    if (!selectedPreset) return;
    errorMsg = "";
    infoMsg = "";
    try {
      await downloadModel(selectedPreset);
      await checkDownloadStatus();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function handleDownloadCustom() {
    const target = customTarget.trim();
    if (!target) return;
    errorMsg = "";
    infoMsg = "";
    try {
      await downloadModel(target);
      customTarget = "";
      await checkDownloadStatus();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function handleCancelModel() {
    try {
      await cancelModelDownload();
      await checkDownloadStatus();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function handleDeleteModel(name: string) {
    const confirmMsg = $_("modelManager.deleteConfirm", { values: { name } }) || `Are you sure you want to delete ${name}?`;
    const ok = await showConfirm({
      title: $_("common.confirmDelete") || "Delete Model",
      message: confirmMsg,
      type: "danger",
      confirmText: $_("common.delete") || "Delete",
    });
    if (!ok) return;

    errorMsg = "";
    infoMsg = "";
    try {
      await deleteLocalModel(name);
      infoMsg = $_("modelManager.modelDeleteSuccess", { values: { name } }) || `Model ${name} deleted successfully.`;
      await refreshData();
      onmodelsChanged?.();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function handleSetupGPU() {
    errorMsg = "";
    infoMsg = "";
    try {
      await downloadGPULibrary();
      await checkDownloadStatus();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  async function handleCancelGPU() {
    try {
      await cancelGPUDownload();
      await checkDownloadStatus();
    } catch (e: any) {
      errorMsg = e?.message || String(e);
    }
  }

  function handleClose() {
    show = false;
    onclose?.();
  }
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6 overflow-y-auto bg-slate-950/70 backdrop-blur-sm animate-fade-in">
    <div
      class="relative w-full max-w-2xl rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden flex flex-col max-h-[90vh]"
      role="dialog"
      aria-modal="true"
    >
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
        <div class="flex items-center gap-3">
          <div class="p-2 rounded-xl bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
            <Cpu class="w-5 h-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">
              {$_("modelManager.title") || "ローカルLLMモデル & GPU管理"}
            </h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              {$_("modelManager.subtitle") || "データストア専用フォルダにモデルとGPUライブラリを保管・管理します"}
            </p>
          </div>
        </div>
        <button
          type="button"
          onclick={handleClose}
          class="p-2 rounded-xl text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Body -->
      <div class="p-6 space-y-5 overflow-y-auto flex-1">
        <!-- Error Alert -->
        {#if errorMsg}
          <div class="p-3.5 rounded-xl bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900/60 text-xs text-rose-800 dark:text-rose-300 flex items-start justify-between gap-3">
            <div class="flex items-start gap-2">
              <AlertCircle class="w-4 h-4 text-rose-500 shrink-0 mt-0.5" />
              <span>{errorMsg}</span>
            </div>
            <button type="button" onclick={() => (errorMsg = "")} class="text-rose-400 hover:text-rose-600">
              <X class="w-4 h-4" />
            </button>
          </div>
        {/if}

        <!-- Success Alert -->
        {#if infoMsg}
          <div class="p-3.5 rounded-xl bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900/60 text-xs text-emerald-800 dark:text-emerald-300 flex items-start justify-between gap-3">
            <div class="flex items-start gap-2">
              <CheckCircle2 class="w-4 h-4 text-emerald-500 shrink-0 mt-0.5" />
              <span>{infoMsg}</span>
            </div>
            <button type="button" onclick={() => (infoMsg = "")} class="text-emerald-400 hover:text-emerald-600">
              <X class="w-4 h-4" />
            </button>
          </div>
        {/if}

        <!-- Section 1: GPU Acceleration (WebGPU) -->
        <div class="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800 space-y-3">
          <div class="flex items-center gap-2 text-slate-800 dark:text-slate-200">
            <Gauge class="w-4 h-4 text-cyan-500" />
            <h3 class="text-xs font-bold uppercase tracking-wider">
              {$_("modelManager.gpuAcceleration") || "GPUアクセラレーション (WebGPU)"}
            </h3>
          </div>

          <!-- Active Backend -->
          <div class="flex flex-wrap items-center gap-2 text-xs">
            <span class="font-medium text-slate-600 dark:text-slate-400">
              {$_("modelManager.activeBackend") || "現在のアクティブ環境"}:
            </span>
            {#if hardwareStatus.acceleration === "GPU"}
              <span class="px-2 py-0.5 text-[11px] font-bold rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                GPU
              </span>
            {:else if hardwareStatus.acceleration && hardwareStatus.acceleration.includes("SIMD")}
              <span class="px-2 py-0.5 text-[11px] font-bold rounded-md bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">
                SIMD
              </span>
            {:else}
              <span class="px-2 py-0.5 text-[11px] font-bold rounded-md bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20">
                CPU
              </span>
            {/if}
            <span class="text-slate-500 dark:text-slate-400 font-mono text-[11px]">{hardwareStatus.detail}</span>
          </div>

          <!-- WGPU Library Status -->
          <div class="flex flex-wrap items-center justify-between gap-2 text-xs pt-1 border-t border-slate-200/60 dark:border-slate-800/60">
            <div class="flex items-center gap-2">
              <span class="font-medium text-slate-600 dark:text-slate-400">
                {$_("modelManager.wgpuLibrary") || "wgpu-native ライブラリ"}:
              </span>
              {#if hardwareStatus.has_gpu_lib}
                <span class="text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1">
                  <CheckCircle2 class="w-3.5 h-3.5" />
                  {$_("modelManager.gpuLibInstalled") || "インストール済み"}
                </span>
                <span class="text-slate-400 font-mono text-[10px] truncate max-w-xs" title={hardwareStatus.wgpu_lib_path}>
                  ({hardwareStatus.wgpu_lib_path})
                </span>
              {:else}
                <span class="text-amber-600 dark:text-amber-400 font-medium">
                  {$_("modelManager.gpuLibNotInstalled") || "未インストール (CPUモードで動作)"}
                </span>
              {/if}
            </div>

            {#if !hardwareStatus.has_gpu_lib}
              {#if downloadStatus.gpu.downloading}
                <div class="flex items-center gap-2 flex-1 max-w-xs">
                  <div class="flex-1 bg-slate-200 dark:bg-slate-800 rounded-full h-2 overflow-hidden">
                    <div
                      class="bg-cyan-500 h-2 transition-all duration-300"
                      style="width: {downloadStatus.gpu.percent}%"
                    ></div>
                  </div>
                  <span class="text-[10px] font-mono text-slate-500 whitespace-nowrap">
                    {downloadStatus.gpu.percent}%
                  </span>
                  <button
                    type="button"
                    onclick={handleCancelGPU}
                    class="px-2 py-0.5 text-[11px] font-medium text-rose-600 hover:text-rose-700 bg-rose-50 dark:bg-rose-950/40 rounded-lg border border-rose-200 dark:border-rose-900"
                  >
                    {$_("common.cancel") || "キャンセル"}
                  </button>
                </div>
              {:else}
                <button
                  type="button"
                  onclick={handleSetupGPU}
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-sm transition-all"
                >
                  <Download class="w-3.5 h-3.5" />
                  {$_("modelManager.setupGPU") || "GPUライブラリをダウンロードして有効化"}
                </button>
              {/if}
            {/if}
          </div>

          <p class="text-[11px] leading-relaxed text-slate-500 dark:text-slate-400 pt-1">
            {$_("modelManager.gpuHelp") || "wgpu-nativeライブラリをダウンロードすると、Metal/Vulkan/Direct3DによるGPU高速推論が有効になります。"}
          </p>
        </div>

        <!-- Section 2: Downloaded Models List -->
        <div class="rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden bg-white dark:bg-slate-900 shadow-sm">
          <div class="px-4 py-3 bg-slate-50 dark:bg-slate-950/60 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
            <div class="flex items-center gap-2">
              <Database class="w-4 h-4 text-cyan-500" />
              <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 uppercase tracking-wider">
                {$_("modelManager.localModels") || "ダウンロード済みモデル"}
              </h3>
            </div>
            <span class="px-2 py-0.5 text-xs font-bold rounded-full bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300">
              {localModels.length}
            </span>
          </div>

          {#if localModels.length === 0}
            <div class="p-8 text-center space-y-2">
              <Sparkles class="w-8 h-8 text-slate-300 dark:text-slate-600 mx-auto" />
              <p class="text-xs text-slate-500 dark:text-slate-400">
                {$_("modelManager.noModels") || "ローカルモデルがありません。下のプリセットまたはURLからダウンロードしてください。"}
              </p>
            </div>
          {:else}
            <div class="overflow-x-auto max-h-52 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800/80">
              <table class="w-full text-xs text-left">
                <thead class="bg-slate-50/80 dark:bg-slate-950/40 text-[11px] font-semibold text-slate-500 dark:text-slate-400 sticky top-0 backdrop-blur-sm">
                  <tr>
                    <th class="px-4 py-2">{$_("modelManager.name") || "モデル名"}</th>
                    <th class="px-3 py-2 whitespace-nowrap">{$_("modelManager.size") || "サイズ"}</th>
                    <th class="px-3 py-2 whitespace-nowrap">{$_("modelManager.modTime") || "更新日時"}</th>
                    <th class="px-4 py-2 text-right whitespace-nowrap">{$_("modelManager.action") || "操作"}</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
                  {#each localModels as m}
                    <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/50 transition-colors">
                      <td class="px-4 py-2.5 font-medium text-slate-900 dark:text-slate-100 font-mono text-[11px] truncate max-w-[220px]" title={m.name}>
                        {m.name}
                      </td>
                      <td class="px-3 py-2.5 whitespace-nowrap text-slate-600 dark:text-slate-300 font-mono text-[11px]">
                        {m.size_human}
                      </td>
                      <td class="px-3 py-2.5 whitespace-nowrap text-slate-500 dark:text-slate-400 text-[11px]">
                        {new Date(m.mod_time).toLocaleString()}
                      </td>
                      <td class="px-4 py-2.5 text-right whitespace-nowrap">
                        <button
                          type="button"
                          onclick={() => handleDeleteModel(m.name)}
                          class="inline-flex items-center gap-1 px-2.5 py-1 text-xs font-semibold text-rose-600 hover:text-white dark:text-rose-400 hover:bg-rose-600 dark:hover:bg-rose-600 rounded-lg border border-rose-300 dark:border-rose-900/80 transition-colors"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                          {$_("common.delete") || "削除"}
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>

        <!-- Download in Progress Banner -->
        {#if downloadStatus.model.downloading}
          <div class="p-4 rounded-xl bg-cyan-50 dark:bg-cyan-950/40 border border-cyan-200 dark:border-cyan-900/60 space-y-2.5">
            <div class="flex items-center justify-between text-xs">
              <div class="flex items-center gap-2 font-semibold text-cyan-900 dark:text-cyan-200">
                <Loader2 class="w-4 h-4 animate-spin text-cyan-500" />
                <span>
                  {$_("modelManager.downloading") || "ダウンロード中..."} {downloadStatus.model.percent}%
                  ({downloadStatus.model.downloaded_human} / {downloadStatus.model.total_human})
                </span>
              </div>
              <button
                type="button"
                onclick={handleCancelModel}
                class="px-2.5 py-1 text-xs font-semibold text-rose-600 hover:text-rose-700 bg-rose-50 dark:bg-rose-950/40 rounded-lg border border-rose-200 dark:border-rose-900"
              >
                {$_("common.cancel") || "キャンセル"}
              </button>
            </div>
            <div class="w-full bg-slate-200 dark:bg-slate-800 rounded-full h-2 overflow-hidden">
              <div
                class="bg-cyan-500 h-2 transition-all duration-300"
                style="width: {downloadStatus.model.percent}%"
              ></div>
            </div>
          </div>
        {/if}

        <!-- Section 3: Preset Models Download -->
        <div class="p-4 rounded-xl border border-slate-200 dark:border-slate-800 space-y-2.5 bg-slate-50/50 dark:bg-slate-950/30">
          <div>
            <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 uppercase tracking-wider">
              {$_("modelManager.downloadPreset") || "プリセットモデルのダウンロード"}
            </h4>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              {$_("modelManager.presetHelp") || "推奨される軽量LLMモデルです。ワンクリックでダウンロードできます。"}
            </p>
          </div>

          <div class="flex gap-2 items-center">
            <div class="flex-1">
              <select
                bind:value={selectedPreset}
                disabled={downloadStatus.model.downloading}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none disabled:opacity-60"
              >
                {#each presets as p}
                  <option value={p.name}>
                    {p.name} ({p.size}, {p.description})
                  </option>
                {/each}
              </select>
            </div>
            <button
              type="button"
              onclick={handleDownloadPreset}
              disabled={downloadStatus.model.downloading || !selectedPreset}
              class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white text-xs font-semibold shadow-sm transition-all"
            >
              <Download class="w-4 h-4" />
              {$_("modelManager.download") || "ダウンロード"}
            </button>
          </div>
        </div>

        <!-- Section 4: Custom URL / Hugging Face Download -->
        <div class="p-4 rounded-xl border border-slate-200 dark:border-slate-800 space-y-2.5 bg-slate-50/50 dark:bg-slate-950/30">
          <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 uppercase tracking-wider">
            {$_("modelManager.downloadCustom") || "カスタムURL / Hugging Faceからダウンロード"}
          </h4>
          <div class="flex gap-2 items-center">
            <div class="flex-1">
              <input
                type="text"
                bind:value={customTarget}
                placeholder={$_("modelManager.customPlaceholder") || "URL または HFリポジトリ (例: Qwen/Qwen2.5-0.5B-Instruct-GGUF)"}
                disabled={downloadStatus.model.downloading}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none disabled:opacity-60"
              />
            </div>
            <button
              type="button"
              onclick={handleDownloadCustom}
              disabled={downloadStatus.model.downloading || !customTarget.trim()}
              class="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 dark:bg-slate-700 dark:hover:bg-slate-600 disabled:opacity-50 text-white text-xs font-semibold shadow-sm transition-all"
            >
              <Download class="w-4 h-4" />
              {$_("modelManager.download") || "ダウンロード"}
            </button>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-end px-6 py-3.5 border-t border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
        <button
          type="button"
          onclick={handleClose}
          class="px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-slate-800 rounded-xl transition-colors"
        >
          {$_("common.close") || "閉じる"}
        </button>
      </div>
    </div>
  </div>
{/if}
