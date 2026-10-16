<script lang="ts">
  import { _, locale } from "svelte-i18n";
  import {
    Sparkles,
    X,
    Check,
    Copy,
    ArrowRight,
    Languages,
    AlertCircle,
    RotateCw,
    Sliders,
    Layers,
    Clock,
    ShieldAlert,
    Terminal,
  } from "@lucide/svelte";
  import { assistPolling, type AssistPollingResp, type PollingEnt } from "../api";
  import AIMarkdown from "./common/AIMarkdown.svelte";
  import AnimatedCatLogo from "./common/AnimatedCatLogo.svelte";

  interface Props {
    show: boolean;
    nodeId?: string;
    nodeName?: string;
    nodeIp?: string;
    onApply?: (polling: Partial<PollingEnt>) => void;
    onClose?: () => void;
  }

  let {
    show = $bindable(false),
    nodeId = "",
    nodeName = "",
    nodeIp = "",
    onApply,
    onClose,
  }: Props = $props();

  let userPrompt = $state("");
  let aiLoading = $state(false);
  let aiError = $state("");
  let aiResult = $state<AssistPollingResp | null>(null);
  let copied = $state(false);

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);
  let aiLanguage = $state<"ja" | "en">("ja");

  // Track show state to reset on open
  let lastShow = $state(false);
  $effect(() => {
    if (show && !lastShow) {
      lastShow = true;
      aiLanguage = isJaLocale ? "ja" : "en";
      aiError = "";
      // Keep previous prompt or set sample default if empty
      if (!userPrompt) {
        userPrompt = isJaLocale
          ? "Ping応答時間が100msを超えたら警告を出したい"
          : "Monitor ping latency and warn if RTT exceeds 100ms";
      }
    } else if (!show && lastShow) {
      lastShow = false;
    }
  });

  const promptPresetsJa = [
    "Ping応答時間が100msを超えたら警告を出したい",
    "HTTP Webサーバーの稼働状態 (ステータスコード200) を監視したい",
    "CPU使用率が90%を超えたら検知してアラートを出したい",
    "TLS/SSL証明書の有効期限が残り30日以下になったら警告したい",
    "Syslogでエラー (error/fatal) が発生した回数を監視したい",
  ];

  const promptPresetsEn = [
    "Monitor ping latency and warn if RTT exceeds 100ms",
    "Check HTTP Web server status code 200",
    "Detect high CPU utilization over 90%",
    "Warn if TLS/SSL certificate expires within 30 days",
    "Monitor syslog error count",
  ];

  const presets = $derived(aiLanguage === "ja" ? promptPresetsJa : promptPresetsEn);

  const handleGenerate = async () => {
    if (!userPrompt.trim() || aiLoading) return;
    aiLoading = true;
    aiError = "";
    aiResult = null;

    try {
      const resp = await assistPolling(nodeId, userPrompt.trim(), aiLanguage);
      if (resp.error) {
        aiError = resp.error;
      } else {
        aiResult = resp;
      }
    } catch (e: any) {
      aiError = e.message || String(e);
    } finally {
      aiLoading = false;
    }
  };

  const handleApply = () => {
    if (!aiResult) return;
    const pollingData: Partial<PollingEnt> = {
      NodeID: nodeId,
      Name: aiResult.name || "AI Generated Polling",
      Type: aiResult.type || "ping",
      Mode: aiResult.mode || "",
      Params: aiResult.params || "",
      Filter: aiResult.filter || "",
      Extractor: aiResult.extractor || "",
      Script: aiResult.script || "",
      Level: aiResult.level || "high",
      PollInt: aiResult.poll_int || 60,
      Timeout: aiResult.timeout || 3,
      Retry: aiResult.retry || 1,
    };
    onApply?.(pollingData);
    show = false;
  };

  const handleCopyAdvice = async () => {
    if (!aiResult?.advice) return;
    try {
      await navigator.clipboard.writeText(aiResult.advice);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch (e) {
      console.error(e);
    }
  };

  const handleClose = () => {
    show = false;
    onClose?.();
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-3 sm:p-6 backdrop-blur-sm animate-in fade-in duration-150">
    <div
      class="flex h-[90vh] max-h-[90vh] w-full max-w-4xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden text-slate-800 dark:text-slate-100"
    >
      <!-- Dialog Header -->
      <div class="shrink-0 flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50/70 dark:bg-slate-950/70">
        <div class="flex items-center gap-2.5 text-base font-bold text-cyan-600 dark:text-cyan-400">
          <Sparkles class="h-5 w-5" />
          <span>{$_('polling.aiAssistTitle') || 'AIポーリング設定支援 (AI Polling Assistant)'}</span>
        </div>

        <div class="flex items-center gap-3">
          <!-- Language Toggle -->
          <div class="inline-flex rounded-lg border border-slate-200 dark:border-slate-700 p-0.5 text-xs bg-white dark:bg-slate-900">
            <button
              type="button"
              class="px-2.5 py-1 rounded-md font-medium transition-colors {aiLanguage === 'ja' ? 'bg-cyan-50 dark:bg-cyan-950/50 text-cyan-700 dark:text-cyan-400 font-bold' : 'text-slate-500 hover:text-slate-900 dark:hover:text-slate-100'}"
              onclick={() => (aiLanguage = "ja")}
            >
              日本語
            </button>
            <button
              type="button"
              class="px-2.5 py-1 rounded-md font-medium transition-colors {aiLanguage === 'en' ? 'bg-cyan-50 dark:bg-cyan-950/50 text-cyan-700 dark:text-cyan-400 font-bold' : 'text-slate-500 hover:text-slate-900 dark:hover:text-slate-100'}"
              onclick={() => (aiLanguage = "en")}
            >
              English
            </button>
          </div>

          <button
            type="button"
            onclick={handleClose}
            class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          >
            <X class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- Main Body Container -->
      <div class="flex-1 flex flex-col min-h-0 overflow-y-auto p-6 space-y-5">
        <!-- Target Node Context -->
        {#if nodeName || nodeIp}
          <div class="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/60 px-3.5 py-2 rounded-xl border border-slate-200 dark:border-slate-700/60">
            <span class="font-bold text-slate-700 dark:text-slate-300">{$_('polling.targetNode') || '対象ノード'}:</span>
            <span class="font-semibold text-cyan-700 dark:text-cyan-400">{nodeName || 'N/A'}</span>
            {#if nodeIp}
              <span class="text-slate-400 font-mono">({nodeIp})</span>
            {/if}
          </div>
        {/if}

        <!-- User Intent Input Area -->
        <div class="space-y-2">
          <label for="ai-polling-goal" class="block text-xs font-bold text-slate-700 dark:text-slate-300">
            {$_('polling.aiPromptLabel') || '監視したい目的・条件を自然言語で入力してください'}
          </label>
          <textarea
            id="ai-polling-goal"
            rows="3"
            bind:value={userPrompt}
            placeholder={aiLanguage === 'ja' ? '例: Webサーバーの応答時間を監視し、2秒を超えたら重度障害としたい' : 'e.g., Monitor Web server latency and alert if RTT exceeds 2000ms'}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 p-3 text-xs focus:border-cyan-500 focus:outline-none focus:ring-1 focus:ring-cyan-500 text-slate-900 dark:text-slate-100 font-sans"
          ></textarea>

          <!-- Prompt Preset Chips -->
          <div class="flex flex-wrap gap-1.5 pt-1">
            <span class="text-[11px] text-slate-400 flex items-center pr-1">例:</span>
            {#each presets as p}
              <button
                type="button"
                onclick={() => (userPrompt = p)}
                class="rounded-full bg-slate-100 dark:bg-slate-800 hover:bg-cyan-50 dark:hover:bg-cyan-950/40 text-[11px] px-2.5 py-1 text-slate-600 dark:text-slate-400 hover:text-cyan-700 dark:hover:text-cyan-300 border border-slate-200 dark:border-slate-700 transition-colors cursor-pointer text-left"
              >
                {p}
              </button>
            {/each}
          </div>
        </div>

        <!-- Action Trigger Button -->
        <div class="flex justify-end">
          <button
            type="button"
            disabled={aiLoading || !userPrompt.trim()}
            onclick={handleGenerate}
            class="inline-flex items-center gap-2 rounded-xl bg-cyan-600 hover:bg-cyan-700 text-white px-5 py-2.5 text-xs font-bold shadow-md shadow-cyan-600/20 transition-all disabled:opacity-50 cursor-pointer"
          >
            {#if aiLoading}
              <RotateCw class="h-4 w-4 animate-spin" />
              <span>{$_('common.generating') || 'AI推論中...'}</span>
            {:else}
              <Sparkles class="h-4 w-4" />
              <span>{aiResult ? ($_('polling.aiRegenerate') || '再生成を実行') : ($_('polling.aiGenerate') || 'AIポーリング設定を生成')}</span>
            {/if}
          </button>
        </div>

        <!-- Loading WAIT Screen with Animated Cat Logo -->
        {#if aiLoading}
          <div class="flex flex-col items-center justify-center p-12 rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/30">
            <AnimatedCatLogo mode="loading" size="lg" />
            <span class="text-xs font-semibold text-cyan-600 dark:text-cyan-400 mt-3 animate-pulse">
              {$_('log.aiReasoning') || 'マルチLLM推論中... (最適な監視テンプレートとスクリプトを設計中)'}
            </span>
          </div>
        {/if}

        <!-- Error Card -->
        {#if aiError}
          <div class="rounded-xl border border-rose-200 dark:border-rose-900 bg-rose-50/60 dark:bg-rose-950/30 p-4 text-rose-700 dark:text-rose-400 text-xs flex items-start gap-2.5">
            <AlertCircle class="h-4 w-4 shrink-0 mt-0.5" />
            <span>{aiError}</span>
          </div>
        {/if}

        <!-- Generated Result View -->
        {#if aiResult && !aiLoading}
          <div class="rounded-2xl border border-cyan-200 dark:border-cyan-900/60 bg-white dark:bg-slate-900 p-5 shadow-sm space-y-4">
            <div class="flex items-center justify-between pb-3 border-b border-slate-200 dark:border-slate-800">
              <div class="flex items-center gap-2">
                <span class="text-xs font-bold uppercase tracking-wider text-cyan-600 dark:text-cyan-400">
                  {$_('polling.aiSuggestedConfig') || 'AI推奨ポーリング構成'}
                </span>
                {#if aiResult.template_id > 0}
                  <span class="rounded-full px-2 py-0.5 text-[10px] font-bold bg-cyan-50 dark:bg-cyan-950 text-cyan-700 dark:text-cyan-300 border border-cyan-200 dark:border-cyan-800">
                    Template #{aiResult.template_id}
                  </span>
                {/if}
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onclick={handleCopyAdvice}
                  class="inline-flex items-center gap-1 rounded-lg border border-slate-200 dark:border-slate-700 px-2.5 py-1 text-[11px] text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
                >
                  {#if copied}
                    <Check class="h-3.5 w-3.5 text-emerald-500" />
                    <span class="text-emerald-600 dark:text-emerald-400 font-semibold">{$_('common.copied') || 'コピー完了'}</span>
                  {:else}
                    <Copy class="h-3.5 w-3.5" />
                    <span>{$_('common.copy') || '解説をコピー'}</span>
                  {/if}
                </button>
              </div>
            </div>

            <!-- Configuration Parameters Grid -->
            <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
              <div class="rounded-xl border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/50 p-3 space-y-1.5">
                <div class="flex justify-between">
                  <span class="text-slate-500 font-medium">{$_('polling.name') || '名前'}:</span>
                  <span class="font-bold text-slate-900 dark:text-slate-100">{aiResult.name}</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-slate-500 font-medium">{$_('polling.type') || '種別'}:</span>
                  <span class="font-bold uppercase text-blue-600 dark:text-cyan-400">{aiResult.type} ({aiResult.mode || 'default'})</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-slate-500 font-medium">{$_('polling.level') || '障害レベル'}:</span>
                  <span class="font-bold uppercase {aiResult.level === 'high' ? 'text-rose-500' : 'text-amber-500'}">{aiResult.level}</span>
                </div>
              </div>

              <div class="rounded-xl border border-slate-100 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/50 p-3 space-y-1.5 font-mono text-[11px]">
                <div class="flex justify-between">
                  <span class="text-slate-500 font-sans">{$_('polling.pollInt') || '間隔'}:</span>
                  <span>{aiResult.poll_int} sec</span>
                </div>
                <div class="flex justify-between">
                  <span class="text-slate-500 font-sans">{$_('polling.timeout') || 'タイムアウト'}:</span>
                  <span>{aiResult.timeout} sec (Retry: {aiResult.retry})</span>
                </div>
                {#if aiResult.params}
                  <div class="flex justify-between truncate">
                    <span class="text-slate-500 font-sans mr-2">Params:</span>
                    <span class="truncate">{aiResult.params}</span>
                  </div>
                {/if}
              </div>
            </div>

            <!-- Script Box -->
            {#if aiResult.script}
              <div class="space-y-1">
                <div class="flex items-center justify-between text-[11px]">
                  <span class="font-bold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                    <Terminal class="h-3.5 w-3.5 text-cyan-500" />
                    <span>{$_('polling.script') || '判定スクリプト (true = 正常 / false = 障害)'}</span>
                  </span>
                </div>
                <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-950 p-3 font-mono text-xs text-emerald-400 overflow-x-auto">
                  <code>{aiResult.script}</code>
                </div>
              </div>
            {/if}

            <!-- Filter & Extractor if present -->
            {#if aiResult.filter || aiResult.extractor}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-2 text-xs font-mono">
                {#if aiResult.filter}
                  <div class="p-2.5 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/50">
                    <span class="text-[10px] text-slate-400 font-sans block mb-1">Filter Regex:</span>
                    <span class="truncate block">{aiResult.filter}</span>
                  </div>
                {/if}
                {#if aiResult.extractor}
                  <div class="p-2.5 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/50">
                    <span class="text-[10px] text-slate-400 font-sans block mb-1">Extractor:</span>
                    <span class="truncate block">{aiResult.extractor}</span>
                  </div>
                {/if}
              </div>
            {/if}

            <!-- AI Advice / Explanation -->
            {#if aiResult.advice}
              <div class="p-4 rounded-xl border border-cyan-100 dark:border-cyan-950/80 bg-cyan-50/40 dark:bg-cyan-950/20 text-xs">
                <span class="font-bold text-cyan-800 dark:text-cyan-300 block mb-2">
                  {$_('polling.aiAdviceTitle') || 'AIによる設定解説 & 運用アドバイス'}:
                </span>
                <AIMarkdown content={aiResult.advice} />
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <!-- Dialog Footer -->
      <div class="shrink-0 flex items-center justify-between border-t border-slate-200 dark:border-slate-800 px-6 py-3.5 bg-slate-50/70 dark:bg-slate-950/70">
        <button
          type="button"
          onclick={handleClose}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors cursor-pointer"
        >
          {$_('common.close') || '閉じる'}
        </button>

        {#if aiResult}
          <button
            type="button"
            onclick={handleApply}
            class="inline-flex items-center gap-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-700 text-white px-5 py-2 text-xs font-bold shadow-md shadow-emerald-600/20 transition-all cursor-pointer"
          >
            <Check class="h-4 w-4" />
            <span>{$_('polling.aiApplyAndCreate') || 'この設定をポーリングに適用'}</span>
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}
