<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    applyUpdate,
    fetchUpdateStatus,
    fetchSystemInfo,
    type UpdateInfo,
    type UpdateStatus,
  } from "../api";
  import { showConfirm, showAlert } from "../stores/modalStore";
  import {
    Download,
    RefreshCw,
    CheckCircle2,
    AlertTriangle,
    X,
    ExternalLink,
    Terminal,
    Copy,
    Check,
    Loader2,
    Sparkles,
    ShieldAlert,
  } from "@lucide/svelte";

  interface Props {
    open: boolean;
    updateInfo: UpdateInfo | null;
    isAdmin: boolean;
    onClose: () => void;
  }

  let { open, updateInfo, isAdmin, onClose }: Props = $props();

  let updating = $state(false);
  let status = $state<UpdateStatus>({
    state: "idle",
    progress: 0,
    message: "",
  });
  let pollInterval: ReturnType<typeof setInterval> | null = null;
  let copied = $state(false);
  let brewCopied = $state(false);

  const dockerCommand = "docker compose pull && docker compose up -d";
  const brewCommand = "brew upgrade twsnmp/tap/twsnmpneo";

  const copyCommand = async () => {
    try {
      await navigator.clipboard.writeText(dockerCommand);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 2000);
    } catch {
      // fallback
    }
  };

  const copyBrewCommand = async () => {
    try {
      await navigator.clipboard.writeText(brewCommand);
      brewCopied = true;
      setTimeout(() => {
        brewCopied = false;
      }, 2000);
    } catch {
      // fallback
    }
  };

  const handleStartUpdate = async () => {
    if (!isAdmin) return;

    const ok = await showConfirm({
      title: $_("system.updateModalTitle") || "システムアップデート",
      message: $_("system.confirmUpdate") || "TWSNMP NEO をアップデートしますか？更新処理中、サービスが約1〜2秒間一時的に再起動します。",
      type: "warning",
      confirmText: $_("system.startUpdate") || "アップデートを実行",
    });

    if (!ok) return;

    updating = true;
    try {
      await applyUpdate();
      startStatusPolling();
    } catch (e: any) {
      updating = false;
      showAlert({
        title: $_("system.updateModalTitle") || "システムアップデート",
        message: e.message || "アップデートの開始に失敗しました",
        type: "danger",
      });
    }
  };

  const startStatusPolling = () => {
    if (pollInterval) clearInterval(pollInterval);
    pollInterval = setInterval(async () => {
      try {
        const s = await fetchUpdateStatus();
        status = s;

        if (s.state === "restarting") {
          // Process has restarted or is restarting; switch to pinging system health
          if (pollInterval) clearInterval(pollInterval);
          waitForServerReconnect();
        } else if (s.state === "error") {
          updating = false;
          if (pollInterval) clearInterval(pollInterval);
          showAlert({
            title: $_("system.updateModalTitle") || "システムアップデート",
            message: s.error || s.message || "Update error",
            type: "danger",
          });
        }
      } catch {
        // If status call fails because server is restarting, switch to reconnect check
        if (pollInterval) clearInterval(pollInterval);
        waitForServerReconnect();
      }
    }, 1000);
  };

  const waitForServerReconnect = () => {
    status = {
      state: "restarting",
      progress: 95,
      message: $_("system.restarting") || "サーバーが再起動しています...",
    };

    let attempts = 0;
    const maxAttempts = 30; // 30 seconds max
    const reconnectInterval = setInterval(async () => {
      attempts++;
      try {
        const info = await fetchSystemInfo();
        if (info && info.status === "ok") {
          clearInterval(reconnectInterval);
          status = {
            state: "completed",
            progress: 100,
            message: $_("system.updateSuccess") || "アップデートが完了しました！",
          };
          showAlert({
            title: $_("system.updateModalTitle") || "システムアップデート",
            message: $_("system.updateSuccess") || "アップデートが完了しました！",
            type: "info",
          });
          setTimeout(() => {
            window.location.reload();
          }, 1500);
        }
      } catch {
        if (attempts >= maxAttempts) {
          clearInterval(reconnectInterval);
          updating = false;
          status = {
            state: "error",
            progress: 0,
            message: "再接続タイムアウト",
            error: "サーバーの再起動の確認にタイムアウトしました。手動で画面を更新してください。",
          };
        }
      }
    }, 1000);
  };

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });
</script>

{#if open && updateInfo}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4">
    <div
      class="flex w-full max-w-xl flex-col rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-200"
    >
      <!-- Modal Header -->
      <div
        class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50 dark:bg-slate-950/50"
      >
        <div class="flex items-center gap-2.5">
          <div
            class="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-500/10 text-cyan-600 dark:text-cyan-400"
          >
            <Sparkles class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">
              {$_("system.updateModalTitle")}
            </h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              GitHub Releases
            </p>
          </div>
        </div>
        {#if !updating}
          <button
            onclick={onClose}
            class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
          >
            <X class="h-5 w-5" />
          </button>
        {/if}
      </div>

      <!-- Modal Body -->
      <div class="flex flex-col gap-5 p-6 max-h-[70vh] overflow-y-auto">
        <!-- Version Compare Cards -->
        <div class="grid grid-cols-2 gap-3">
          <div
            class="flex flex-col p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40"
          >
            <span class="text-[11px] font-medium text-slate-500 dark:text-slate-400">
              {$_("system.currentVersion")}
            </span>
            <span class="text-lg font-bold font-mono text-slate-800 dark:text-slate-200 mt-0.5">
              {updateInfo.current_version || "v0.2.0"}
            </span>
          </div>

          <div
            class="flex flex-col p-3 rounded-xl border border-cyan-500/30 dark:border-cyan-500/40 bg-cyan-500/5 dark:bg-cyan-500/10"
          >
            <span class="text-[11px] font-medium text-cyan-700 dark:text-cyan-300 flex items-center gap-1">
              <Sparkles class="h-3 w-3" />
              {$_("system.latestVersion")}
            </span>
            <span class="text-lg font-bold font-mono text-cyan-700 dark:text-cyan-300 mt-0.5">
              {updateInfo.latest_version}
            </span>
          </div>
        </div>

        <!-- Release Info & Date -->
        {#if updateInfo.published_at}
          <div class="text-xs text-slate-500 dark:text-slate-400 flex items-center justify-between">
            <span>
              {$_("system.publishedAt")}: {new Date(updateInfo.published_at).toLocaleString()}
            </span>
            {#if updateInfo.release_url}
              <a
                href={updateInfo.release_url}
                target="_blank"
                rel="noreferrer"
                class="inline-flex items-center gap-1 text-cyan-600 dark:text-cyan-400 hover:underline"
              >
                <span>GitHub</span>
                <ExternalLink class="h-3 w-3" />
              </a>
            {/if}
          </div>
        {/if}

        <!-- Release Notes / Changelog -->
        {#if updateInfo.release_body}
          <div class="flex flex-col gap-1.5">
            <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
              {$_("system.releaseNotes")}
            </span>
            <div
              class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-3.5 text-xs font-mono text-slate-700 dark:text-slate-300 max-h-48 overflow-y-auto whitespace-pre-wrap leading-relaxed select-text"
            >
              {updateInfo.release_body}
            </div>
          </div>
        {/if}

        <!-- Homebrew Notice -->
        {#if updateInfo.is_homebrew}
          <div
            class="flex flex-col gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-amber-800 dark:text-amber-300 text-xs"
          >
            <div class="flex items-center gap-2 font-bold">
              <AlertTriangle class="h-4 w-4 text-amber-600 dark:text-amber-400" />
              <span>{$_("system.homebrewNoticeTitle")}</span>
            </div>
            <p class="text-slate-600 dark:text-slate-400 leading-relaxed">
              {$_("system.homebrewNoticeDesc")}
            </p>
            <div
              class="flex items-center justify-between gap-2 mt-1 rounded-lg bg-slate-900 px-3 py-2 text-slate-100 font-mono text-[11px]"
            >
              <div class="flex items-center gap-1.5 truncate">
                <Terminal class="h-3.5 w-3.5 text-cyan-400 shrink-0" />
                <span class="truncate">{brewCommand}</span>
              </div>
              <button
                onclick={copyBrewCommand}
                class="flex items-center gap-1 shrink-0 rounded px-2 py-1 bg-slate-800 hover:bg-slate-700 text-cyan-400 transition-colors"
                title={$_("system.copyCommand")}
              >
                {#if brewCopied}
                  <Check class="h-3.5 w-3.5 text-emerald-400" />
                  <span class="text-[10px] text-emerald-400">{$_("system.commandCopied")}</span>
                {:else}
                  <Copy class="h-3.5 w-3.5" />
                  <span class="text-[10px]">{$_("system.copyCommand")}</span>
                {/if}
              </button>
            </div>
          </div>
        {/if}

        <!-- Debug Notice -->
        {#if updateInfo.is_debug}
          <div
            class="flex flex-col gap-2 rounded-xl border border-indigo-500/30 bg-indigo-500/10 p-4 text-indigo-800 dark:text-indigo-300 text-xs"
          >
            <div class="flex items-center gap-2 font-bold">
              <AlertTriangle class="h-4 w-4 text-indigo-600 dark:text-indigo-400" />
              <span>{$_("system.debugNoticeTitle")}</span>
            </div>
            <p class="text-slate-600 dark:text-slate-400 leading-relaxed">
              {$_("system.debugNoticeDesc")}
            </p>
          </div>
        {/if}

        <!-- Container Notice (Docker) -->
        {#if updateInfo.is_container}
          <div
            class="flex flex-col gap-2 rounded-xl border border-amber-500/30 bg-amber-500/10 p-4 text-amber-800 dark:text-amber-300 text-xs"
          >
            <div class="flex items-center gap-2 font-bold">
              <AlertTriangle class="h-4 w-4 text-amber-600 dark:text-amber-400" />
              <span>{$_("system.containerNoticeTitle")}</span>
            </div>
            <p class="text-slate-600 dark:text-slate-400 leading-relaxed">
              {$_("system.containerNoticeDesc")}
            </p>
            <div
              class="flex items-center justify-between gap-2 mt-1 rounded-lg bg-slate-900 px-3 py-2 text-slate-100 font-mono text-[11px]"
            >
              <div class="flex items-center gap-1.5 truncate">
                <Terminal class="h-3.5 w-3.5 text-cyan-400 shrink-0" />
                <span class="truncate">{dockerCommand}</span>
              </div>
              <button
                onclick={copyCommand}
                class="flex items-center gap-1 shrink-0 rounded px-2 py-1 bg-slate-800 hover:bg-slate-700 text-cyan-400 transition-colors"
                title={$_("system.copyCommand")}
              >
                {#if copied}
                  <Check class="h-3.5 w-3.5 text-emerald-400" />
                  <span class="text-[10px] text-emerald-400">{$_("system.commandCopied")}</span>
                {:else}
                  <Copy class="h-3.5 w-3.5" />
                  <span class="text-[10px]">{$_("system.copyCommand")}</span>
                {/if}
              </button>
            </div>
          </div>
        {/if}

        <!-- Non-admin Warning -->
        {#if !isAdmin && !updateInfo.is_container && !updateInfo.is_homebrew && !updateInfo.is_debug}
          <div
            class="flex items-center gap-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100/80 dark:bg-slate-800/40 p-3 text-slate-600 dark:text-slate-400 text-xs"
          >
            <ShieldAlert class="h-4 w-4 text-slate-500" />
            <span>{$_("system.adminOnlyNotice")}</span>
          </div>
        {/if}

        <!-- Updating Progress Bar -->
        {#if updating}
          <div class="flex flex-col gap-2 rounded-xl border border-cyan-500/20 bg-cyan-500/5 p-4">
            <div class="flex items-center justify-between text-xs font-semibold text-slate-800 dark:text-slate-200">
              <span class="flex items-center gap-2">
                <Loader2 class="h-4 w-4 animate-spin text-cyan-600 dark:text-cyan-400" />
                <span>{status.message || $_("system.updating")}</span>
              </span>
              <span class="font-mono text-cyan-600 dark:text-cyan-400">{status.progress}%</span>
            </div>
            <div class="h-2 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-800">
              <div
                class="h-full bg-cyan-600 dark:bg-cyan-500 transition-all duration-300"
                style="width: {status.progress}%"
              ></div>
            </div>
            {#if status.error}
              <span class="text-xs text-rose-500 font-medium">{status.error}</span>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Modal Footer -->
      <div
        class="flex items-center justify-end gap-2 border-t border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/50 px-6 py-3.5"
      >
        <button
          onclick={onClose}
          disabled={updating}
          class="rounded-xl px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-200 dark:text-slate-300 dark:hover:bg-slate-800 transition-colors disabled:opacity-50"
        >
          {$_("common.close") || "閉じる"}
        </button>

        {#if (updateInfo.can_self_update ?? (!updateInfo.is_container && !updateInfo.is_homebrew && !updateInfo.is_debug)) && updateInfo.has_update}
          <button
            onclick={handleStartUpdate}
            disabled={updating || !isAdmin}
            class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-cyan-600/30 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {#if updating}
              <Loader2 class="h-4 w-4 animate-spin" />
              <span>{$_("system.updating")}</span>
            {:else}
              <Download class="h-4 w-4" />
              <span>{$_("system.startUpdate")}</span>
            {/if}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}
