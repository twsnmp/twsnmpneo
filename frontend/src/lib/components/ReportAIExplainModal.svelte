<script lang="ts">
  import { _, locale } from "svelte-i18n";
  import {
    Sparkles,
    X,
    Check,
    Copy,
    RefreshCw,
    AlertCircle,
    FileText,
  } from "@lucide/svelte";
  import { explainReport } from "../api";
  import AIMarkdown from "./common/AIMarkdown.svelte";
  import AnimatedCatLogo from "./common/AnimatedCatLogo.svelte";

  interface Props {
    show: boolean;
    reportType: string;
    title?: string;
    summary?: string;
    items?: string[];
    onClose?: () => void;
  }

  let {
    show = $bindable(false),
    reportType = "generic",
    title = "",
    summary = "",
    items = [],
    onClose,
  }: Props = $props();

  let aiLoading = $state(false);
  let aiError = $state("");
  let aiResult = $state<string>("");
  let copied = $state(false);

  const isJaLocale = $derived($locale?.startsWith("ja") ?? true);
  let aiLanguage = $state<"ja" | "en">("ja");

  let lastShow = $state(false);
  $effect(() => {
    if (show && !lastShow) {
      lastShow = true;
      aiLanguage = isJaLocale ? "ja" : "en";
      aiError = "";
      aiResult = "";
    } else if (!show && lastShow) {
      lastShow = false;
    }
  });

  const handleGenerate = async () => {
    if (aiLoading) return;
    aiLoading = true;
    aiError = "";
    aiResult = "";

    try {
      const resp = await explainReport(
        reportType,
        {
          title: title || reportType,
          summary: summary || "",
          items: (items || []).slice(0, 100), // Max 100 items for token budget
        },
        "",
        aiLanguage
      );

      aiResult = resp;
    } catch (e: any) {
      aiError = e.message || String(e);
    } finally {
      aiLoading = false;
    }
  };

  const handleCopyExplanation = async () => {
    if (!aiResult) return;
    try {
      await navigator.clipboard.writeText(aiResult);
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
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-3 sm:p-6 backdrop-blur-sm animate-in fade-in duration-150"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && handleClose()}
  >
    <div
      class="flex h-[88vh] max-h-[88vh] w-full max-w-4xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden text-slate-800 dark:text-slate-100"
    >
      <!-- Dialog Header -->
      <div class="shrink-0 flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-6 py-4 bg-slate-50/70 dark:bg-slate-950/70">
        <div class="flex items-center gap-2.5 text-base font-bold text-cyan-600 dark:text-cyan-400">
          <Sparkles class="h-5 w-5" />
          <span>{title || $_('report.aiAnalysisTitle') || 'AIレポート分析・セキュリティ解説'}</span>
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
            aria-label={$_('common.close') || '閉じる'}
            class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          >
            <X class="h-5 w-5" />
          </button>
        </div>
      </div>

      <!-- Main Body Container -->
      <div class="flex-1 flex flex-col min-h-0 overflow-y-auto p-6 space-y-4">
        <!-- Target & Scope Summary Card -->
        <div class="flex flex-wrap items-center justify-between gap-3 bg-slate-50 dark:bg-slate-800/60 px-4 py-3 rounded-xl border border-slate-200 dark:border-slate-700 text-xs">
          <div>
            {#if summary}
              <span class="font-bold text-slate-700 dark:text-slate-300">{$_('report.aiSummaryStatus') || '集計状況: '}</span>
              <span class="text-slate-600 dark:text-slate-300 font-medium">{summary}</span>
            {/if}
            <span class="text-slate-400 text-[11px] block mt-0.5">
              {$_('report.aiTargetRecords', { values: { count: items.length } }) || `分析対象: ${items.length} 件のデータレコード`}
            </span>
          </div>

          <!-- Manual Trigger Button per SPEC 4.9 -->
          <button
            type="button"
            disabled={aiLoading}
            onclick={handleGenerate}
            class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white px-4 py-2 text-xs font-bold shadow-xs cursor-pointer transition-colors"
          >
            {#if aiLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin" />
              <span>{$_('report.aiInferring') || 'AI推論中...'}</span>
            {:else}
              <Sparkles class="h-3.5 w-3.5" />
              <span>{aiResult ? ($_('report.btnReanalyzeAi') || '再分析を実行') : ($_('report.btnRunAiReport') || 'AIレポート分析を実行')}</span>
            {/if}
          </button>
        </div>

        <!-- Loading WAIT Screen with Animated Cat Logo -->
        {#if aiLoading}
          <div class="flex flex-col items-center justify-center p-12 rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/30">
            <AnimatedCatLogo mode="loading" size="lg" />
            <span class="text-xs font-semibold text-cyan-600 dark:text-cyan-400 mt-3 animate-pulse text-center">
              {$_('report.aiReasoningDetailed') || 'マルチLLM推論中... (セキュリティリスク・運用影響・推奨対策を診断中)'}
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

        <!-- Idle State -->
        {#if !aiResult && !aiLoading && !aiError}
          <div class="flex-1 flex flex-col items-center justify-center p-12 text-center text-slate-400 rounded-xl border border-slate-200 dark:border-slate-800">
            <FileText class="h-10 w-10 text-cyan-500/40 mb-3" />
            <p class="text-xs font-medium">{$_('report.aiEmptyDescription') || '「AIレポート分析を実行」ボタンをクリックすると、現在のレポートデータに基づき、セキュリティリスク評価と運用アドバイスをAIが提示します。'}</p>
          </div>
        {/if}

        <!-- Result View -->
        {#if aiResult && !aiLoading}
          <div class="space-y-4">
            <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 p-5 space-y-3">
              <div class="flex items-center justify-between pb-2 border-b border-slate-100 dark:border-slate-800">
                <span class="text-xs font-bold text-slate-700 dark:text-slate-300">{$_('report.aiResultHeading') || 'AI分析レポート & 是正アクション'}</span>
                <button
                  type="button"
                  onclick={handleCopyExplanation}
                  class="flex items-center gap-1 rounded-lg border border-slate-200 dark:border-slate-700 px-2.5 py-1 text-[11px] text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
                >
                  {#if copied}
                    <Check class="h-3.5 w-3.5 text-emerald-500" />
                    <span class="text-emerald-600 font-semibold">{$_('report.copied') || $_('common.copied') || 'コピー完了'}</span>
                  {:else}
                    <Copy class="h-3.5 w-3.5" />
                    <span>{$_('report.copyExplanation') || '解説をコピー'}</span>
                  {/if}
                </button>
              </div>
              <div class="text-xs leading-relaxed text-slate-800 dark:text-slate-200">
                <AIMarkdown content={aiResult} />
              </div>
            </div>
          </div>
        {/if}
      </div>

      <!-- Dialog Footer -->
      <div class="shrink-0 flex items-center justify-end border-t border-slate-200 dark:border-slate-800 px-6 py-3.5 bg-slate-50/70 dark:bg-slate-950/70">
        <button
          type="button"
          onclick={handleClose}
          class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors cursor-pointer"
        >
          {$_('common.close') || '閉じる'}
        </button>
      </div>
    </div>
  </div>
{/if}
