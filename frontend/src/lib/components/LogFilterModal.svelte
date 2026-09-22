<script lang="ts">
  import { Filter, RotateCcw, Search, X } from "@lucide/svelte";

  let {
    show = $bindable(false),
    logCategory = "event",
    filterState = $bindable({
      start: "",
      end: "",
      level: "all",
      type: "",
      source: "",
      keyword: "",
    }),
    onApply = () => {},
  }: {
    show: boolean;
    logCategory: string;
    filterState: {
      start: string;
      end: string;
      level: string;
      type: string;
      source: string;
      keyword: string;
    };
    onApply: () => void;
  } = $props();

  const handleReset = () => {
    filterState.start = "";
    filterState.end = "";
    filterState.level = "all";
    filterState.type = "";
    filterState.source = "";
    filterState.keyword = "";
  };

  const handleApply = () => {
    show = false;
    onApply();
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in duration-150">
    <div class="flex max-h-[90vh] w-full max-w-lg flex-col rounded-2xl border border-slate-800 bg-slate-900 p-6 shadow-2xl text-slate-100">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-800 pb-4">
        <div class="flex items-center gap-2.5 text-base font-bold text-cyan-400">
          <Filter class="h-5 w-5" />
          <span>詳細フィルター設定 ({logCategory.toUpperCase()})</span>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Form Body -->
      <div class="flex-1 overflow-y-auto py-4 space-y-4 text-xs">
        <!-- Time Range -->
        <div class="space-y-1.5">
          <div class="font-semibold text-slate-300">時間範囲 (Start / End)</div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <span class="text-[10px] text-slate-400 block mb-1">開始日時 (From)</span>
              <input
                type="datetime-local"
                bind:value={filterState.start}
                class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div>
              <span class="text-[10px] text-slate-400 block mb-1">終了日時 (To)</span>
              <input
                type="datetime-local"
                bind:value={filterState.end}
                class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>
        </div>

        <!-- Level & Type -->
        <div class="grid grid-cols-2 gap-3">
          {#if logCategory === "event" || logCategory === "syslog"}
            <div class="space-y-1">
              <div class="font-semibold text-slate-300">重要度レベル</div>
              <select
                bind:value={filterState.level}
                class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-200 focus:border-cyan-500 focus:outline-none cursor-pointer"
              >
                <option value="all">すべて (All)</option>
                <option value="high">重度障害 (High)</option>
                <option value="low">軽度障害 (Low)</option>
                <option value="warn">注意 (Warn)</option>
                <option value="normal">正常 (Normal)</option>
                <option value="info">情報 (Info)</option>
              </select>
            </div>
          {/if}

          <div class="space-y-1">
            <div class="font-semibold text-slate-300">種別 (Type / Tag)</div>
            <input
              type="text"
              placeholder="例: polling, system, ssh..."
              bind:value={filterState.type}
              class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>
        </div>

        <!-- Source / Node -->
        <div class="space-y-1">
          <div class="font-semibold text-slate-300">
            {logCategory === "event" ? "ノード名 / ノードID" : "送信元 (Src IP / Host)"}
          </div>
          <input
            type="text"
            placeholder={logCategory === "event" ? "ノード名やIDで絞り込み..." : "送信元IPアドレスやホスト名..."}
            bind:value={filterState.source}
            class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
          />
        </div>

        <!-- Keyword / Regex -->
        <div class="space-y-1">
          <div class="font-semibold text-slate-300">キーワード / 正規表現</div>
          <input
            type="text"
            placeholder="イベント内容やログメッセージを検索..."
            bind:value={filterState.keyword}
            class="w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-xs text-slate-100 placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
          />
        </div>
      </div>

      <!-- Actions -->
      <div class="flex items-center justify-between border-t border-slate-800 pt-4 mt-2">
        <button
          type="button"
          onclick={handleReset}
          class="flex items-center gap-1.5 rounded-xl border border-slate-700 bg-slate-800 hover:bg-slate-700 px-3.5 py-2 text-xs font-semibold text-slate-300 transition-colors cursor-pointer"
        >
          <RotateCcw class="h-3.5 w-3.5 text-slate-400" />
          <span>リセット</span>
        </button>

        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={() => (show = false)}
            class="rounded-xl px-4 py-2 text-xs font-semibold text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
          >
            キャンセル
          </button>
          <button
            type="button"
            onclick={handleApply}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-5 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
          >
            <Search class="h-4 w-4" />
            <span>フィルター適用</span>
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
