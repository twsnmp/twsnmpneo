<script lang="ts">
  import { _, locale } from "svelte-i18n";
  import { Sparkles, X, Copy, Check, Languages, AlertCircle, RefreshCw } from "@lucide/svelte";
  import { askAI } from "../api";
  import AIMarkdown from "./common/AIMarkdown.svelte";
  import AnimatedCatLogo from "./common/AnimatedCatLogo.svelte";

  interface Props {
    show: boolean;
    selectedLogText: string;
    onClose?: () => void;
  }

  let {
    show = $bindable(false),
    selectedLogText = "",
    onClose,
  }: Props = $props();

  let aiAnswer = $state("");
  let aiLoading = $state(false);
  let aiError = $state("");
  let copied = $state(false);

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);
  let aiLanguage = $state<"ja" | "en">("ja");

  // Track last shown state & log text to reset appropriately
  let lastShow = $state(false);
  let lastLogText = $state("");

  $effect(() => {
    if (show && (!lastShow || selectedLogText !== lastLogText)) {
      lastShow = true;
      lastLogText = selectedLogText;
      aiAnswer = "";
      aiError = "";
      aiLoading = false;
      aiLanguage = isJaLocale ? "ja" : "en";
    } else if (!show && lastShow) {
      lastShow = false;
    }
  });

  const handleQuery = async () => {
    if (!selectedLogText || aiLoading) return;
    aiLoading = true;
    aiError = "";
    aiAnswer = "";

    try {
      let prompt = "";
      let system = "";

      if (aiLanguage === "ja") {
        prompt = `以下のネットワークログを解析し、根本原因の推定、重大度、および推奨される復旧・予防対策を専門家として簡潔に説明してください。日本語で回答してください。\n\nログ:\n${selectedLogText}`;
        system = `ネットワークインフラのトラブルシューティング支援AI。必ず日本語でマークダウン形式（表、太字、箇条書き、見出し等）を用いてわかりやすく解説してください。`;
      } else {
        prompt = `Please analyze the following network log and concisely explain the estimated root cause, severity level, and recommended remediation/prevention actions as an expert. Please answer in English.\n\nLog:\n${selectedLogText}`;
        system = `Network infrastructure troubleshooting assistant AI. Please answer in English using clean markdown format (tables, bold text, bullet lists, headings).`;
      }

      aiAnswer = await askAI(prompt, system);
    } catch (e: any) {
      aiError = e.message || String(e);
    } finally {
      aiLoading = false;
    }
  };

  const handleCopy = async () => {
    if (!aiAnswer) return;
    try {
      await navigator.clipboard.writeText(aiAnswer);
      copied = true;
      setTimeout(() => {
        copied = false;
      }, 2000);
    } catch (e) {
      console.error("Failed to copy AI text:", e);
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
      class="flex h-[90vh] max-h-[90vh] w-full max-w-5xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden text-slate-800 dark:text-slate-100"
    >
      <!-- Dialog Header -->
      <div class="shrink-0 flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50/70 dark:bg-slate-950/70">
        <div class="flex items-center gap-2.5 text-base font-bold text-cyan-600 dark:text-cyan-400">
          <Sparkles class="h-5 w-5" />
          <span>{$_('log.aiAssistant')}</span>
        </div>
        <button
          type="button"
          onclick={handleClose}
          aria-label={$_('log.close')}
          class="rounded-lg p-1.5 text-slate-500 dark:text-slate-400 hover:bg-slate-200 dark:hover:bg-slate-800 hover:text-slate-800 dark:hover:text-white cursor-pointer transition-colors"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Target Log Text Preview -->
      <div class="shrink-0 px-6 pt-3 pb-1">
        <div class="text-[11px] font-semibold text-slate-500 dark:text-slate-400 mb-1">
          {$_('tools.targetNode') || '対象ログ'}:
        </div>
        <div
          class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 p-2.5 font-mono text-xs text-slate-700 dark:text-slate-300 break-all max-h-20 overflow-y-auto shadow-inner"
        >
          {selectedLogText}
        </div>
      </div>

      <!-- Condition Setting & Action Bar -->
      <div class="shrink-0 px-6 py-2.5 flex flex-wrap items-center justify-between gap-3 border-y border-slate-200/80 dark:border-slate-800/80 bg-slate-100/60 dark:bg-slate-950/40">
        <div class="flex items-center gap-3">
          {#if isJaLocale}
            <!-- Language selection (when UI is Japanese) -->
            <div class="flex items-center gap-2 text-xs">
              <span class="flex items-center gap-1 font-semibold text-slate-600 dark:text-slate-300">
                <Languages class="h-3.5 w-3.5 text-cyan-500" />
                <span>{$_('log.aiQueryLanguage') || '回答言語'}:</span>
              </span>
              <div class="inline-flex rounded-lg border border-slate-300 dark:border-slate-700 p-0.5 bg-white dark:bg-slate-800 shadow-xs">
                <button
                  type="button"
                  disabled={aiLoading}
                  onclick={() => (aiLanguage = "ja")}
                  class="px-2.5 py-1 text-xs font-medium rounded-md transition-colors cursor-pointer {aiLanguage === 'ja' ? 'bg-cyan-600 text-white shadow-xs font-semibold' : 'text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'}"
                >
                  {$_('log.aiLangJa') || '日本語'}
                </button>
                <button
                  type="button"
                  disabled={aiLoading}
                  onclick={() => (aiLanguage = "en")}
                  class="px-2.5 py-1 text-xs font-medium rounded-md transition-colors cursor-pointer {aiLanguage === 'en' ? 'bg-cyan-600 text-white shadow-xs font-semibold' : 'text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white'}"
                >
                  {$_('log.aiLangEn') || '英語 (English)'}
                </button>
              </div>
            </div>
          {/if}
        </div>

        <!-- Query Execution Button -->
        <button
          type="button"
          onclick={handleQuery}
          disabled={aiLoading || !selectedLogText}
          class="flex items-center gap-2 rounded-xl bg-gradient-to-r from-cyan-600 to-teal-600 hover:from-cyan-500 hover:to-teal-500 disabled:opacity-50 text-white px-4 py-2 text-xs font-bold transition-all shadow-md shadow-cyan-600/20 cursor-pointer active:scale-95"
        >
          <Sparkles class="h-4 w-4" />
          <span>{aiAnswer ? ($_('log.aiReanalyze') || '再診断を実行') : ($_('log.aiStartAnalysis') || 'AIログ診断を実行')}</span>
        </button>
      </div>

      <!-- AI Response Area -->
      <div
        class="flex-1 min-h-0 overflow-y-auto mx-6 my-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/50 p-5 text-sm leading-relaxed text-slate-800 dark:text-slate-200 shadow-inner"
      >
        {#if aiLoading}
          <!-- Animated Cat Logo WAIT Screen -->
          <div class="flex flex-col items-center justify-center py-16 space-y-4">
            <AnimatedCatLogo mode="loading" size="lg" />
            <div class="text-center space-y-1.5 max-w-md">
              <p class="text-sm font-bold text-cyan-600 dark:text-cyan-400 animate-pulse">
                {aiLanguage === 'ja' ? $_('log.aiReasoning') : 'Multi-LLM reasoning in progress... Analyzing logs and topology.'}
              </p>
              <p class="text-xs text-slate-400">
                {$_('log.aiWaiting') || 'しばらくお待ちください...'}
              </p>
            </div>
          </div>
        {:else if aiError}
          <!-- Error Alert -->
          <div class="p-4 rounded-xl bg-rose-50 dark:bg-rose-950/50 border border-rose-200 dark:border-rose-900/60 text-rose-700 dark:text-rose-300 text-xs space-y-3">
            <div class="flex items-center gap-2 font-semibold">
              <AlertCircle class="h-4 w-4 shrink-0 text-rose-500" />
              <span>{$_('common.error')}: {aiError}</span>
            </div>
            <button
              type="button"
              onclick={handleQuery}
              class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium cursor-pointer shadow-xs"
            >
              <RefreshCw class="h-3.5 w-3.5" />
              <span>{$_('common.retry') || '再試行'}</span>
            </button>
          </div>
        {:else if aiAnswer}
          <!-- Rendered AI Markdown Output -->
          <AIMarkdown content={aiAnswer} />
        {:else}
          <!-- Idle / Ready to Query Guidance -->
          <div class="flex flex-col items-center justify-center py-20 text-center space-y-3">
            <div class="p-3 rounded-2xl bg-cyan-50 dark:bg-cyan-950/40 text-cyan-500 border border-cyan-200 dark:border-cyan-800/50">
              <Sparkles class="h-8 w-8" />
            </div>
            <div class="space-y-1 max-w-md">
              <h4 class="text-sm font-bold text-slate-800 dark:text-slate-200">
                {$_('log.aiAssistant')}
              </h4>
              <p class="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
                {$_('log.aiPromptPlaceholder') || '言語等の条件を確認し、「AIログ診断を実行」を押してください。'}
              </p>
            </div>
          </div>
        {/if}
      </div>

      <!-- Footer Actions -->
      <div class="shrink-0 flex items-center justify-between border-t border-slate-200 dark:border-slate-800 px-6 py-3.5 bg-slate-50/70 dark:bg-slate-950/70">
        <div>
          {#if aiAnswer && !aiLoading}
            <button
              type="button"
              onclick={handleCopy}
              class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
            >
              {#if copied}
                <Check class="h-4 w-4 text-emerald-500" />
                <span class="text-emerald-500 font-medium">{$_('common.copied')}</span>
              {:else}
                <Copy class="h-4 w-4" />
                <span>{$_('common.copy')}</span>
              {/if}
            </button>
          {/if}
        </div>
        <button
          type="button"
          onclick={handleClose}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-5 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
        >
          {$_('log.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
