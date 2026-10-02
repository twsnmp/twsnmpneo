<script lang="ts">
  import { _ } from "svelte-i18n";
  import { Filter, RotateCcw, Search, X, Clock } from "@lucide/svelte";
  import type { FilterState, LogCategory } from "../views/log/types";

  let {
    show = $bindable(false),
    logCategory = "event" as LogCategory,
    sflowCounter = false,
    filterState = $bindable<FilterState>({
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
    logCategory: LogCategory;
    sflowCounter?: boolean;
    filterState: FilterState;
    onApply: () => void;
  } = $props();

  // Local draft state to isolate changes until Search is clicked
  let draft = $state<FilterState>({
    start: "",
    end: "",
    level: "all",
    type: "",
    source: "",
    keyword: "",
    single: true,
    srcPort: "",
    dstAddr: "",
    dstPort: "",
    protocol: "",
    tcpFlags: "",
    mac: "",
    state: "",
  });

  $effect(() => {
    if (show) {
      draft = {
        start: filterState.start || "",
        end: filterState.end || "",
        level: filterState.level || "all",
        type: filterState.type || "",
        source: filterState.source || "",
        keyword: filterState.keyword || "",
        single: filterState.single !== false,
        srcPort: filterState.srcPort || "",
        dstAddr: filterState.dstAddr || "",
        dstPort: filterState.dstPort || "",
        protocol: filterState.protocol || "",
        tcpFlags: filterState.tcpFlags || "",
        mac: filterState.mac || "",
        state: filterState.state || "",
      };
    }
  });

  function formatLocalIso(date: Date): string {
    const pad = (n: number) => String(n).padStart(2, "0");
    const year = date.getFullYear();
    const month = pad(date.getMonth() + 1);
    const day = pad(date.getDate());
    const hours = pad(date.getHours());
    const minutes = pad(date.getMinutes());
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  }

  const applyShortcut = (type: "1h" | "6h" | "24h" | "today" | "yesterday" | "7d" | "30d" | "clear") => {
    const now = new Date();
    if (type === "clear") {
      draft.start = "";
      draft.end = "";
      return;
    }
    if (type === "1h") {
      draft.start = formatLocalIso(new Date(now.getTime() - 60 * 60 * 1000));
      draft.end = formatLocalIso(now);
    } else if (type === "6h") {
      draft.start = formatLocalIso(new Date(now.getTime() - 6 * 60 * 60 * 1000));
      draft.end = formatLocalIso(now);
    } else if (type === "24h") {
      draft.start = formatLocalIso(new Date(now.getTime() - 24 * 60 * 60 * 1000));
      draft.end = formatLocalIso(now);
    } else if (type === "today") {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0);
      draft.start = formatLocalIso(start);
      draft.end = formatLocalIso(now);
    } else if (type === "yesterday") {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0);
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59);
      draft.start = formatLocalIso(start);
      draft.end = formatLocalIso(end);
    } else if (type === "7d") {
      draft.start = formatLocalIso(new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000));
      draft.end = formatLocalIso(now);
    } else if (type === "30d") {
      draft.start = formatLocalIso(new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000));
      draft.end = formatLocalIso(now);
    }
  };

  const handleReset = () => {
    draft.start = "";
    draft.end = "";
    draft.level = "all";
    draft.type = "";
    draft.source = "";
    draft.keyword = "";
    draft.single = true;
    draft.srcPort = "";
    draft.dstAddr = "";
    draft.dstPort = "";
    draft.protocol = "";
    draft.tcpFlags = "";
    draft.mac = "";
    draft.state = "";
  };

  const handleSearch = () => {
    filterState.start = draft.start;
    filterState.end = draft.end;
    filterState.level = draft.level;
    filterState.type = draft.type;
    filterState.source = draft.source;
    filterState.keyword = draft.keyword;
    filterState.single = draft.single;
    filterState.srcPort = draft.srcPort;
    filterState.dstAddr = draft.dstAddr;
    filterState.dstPort = draft.dstPort;
    filterState.protocol = draft.protocol;
    filterState.tcpFlags = draft.tcpFlags;
    filterState.mac = draft.mac;
    filterState.state = draft.state;
    show = false;
    onApply();
  };

  const getCategoryTitle = (cat: LogCategory, isCounter: boolean) => {
    if (cat === "sflow" && isCounter) {
      return `${$_('log.categories.sflow')} (${$_('log.sflowCounter')})`;
    }
    return $_(`log.categories.${cat}`) || cat.toUpperCase();
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in duration-150">
    <div class="flex max-h-[92vh] w-full max-w-xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-6 shadow-2xl text-slate-800 dark:text-slate-100 font-sans">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-4">
        <div class="flex items-center gap-2.5 text-base font-bold text-cyan-600 dark:text-cyan-400">
          <Filter class="h-5 w-5" />
          <span>{$_('log.filterModal.title')} ({getCategoryTitle(logCategory, sflowCounter)})</span>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-white transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Form Body -->
      <div class="flex-1 overflow-y-auto py-4 space-y-4 text-xs pr-1">
        <!-- Time Range with Shortcuts -->
        <div class="space-y-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-950/40 p-3">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1.5 font-semibold text-slate-700 dark:text-slate-200">
              <Clock class="h-3.5 w-3.5 text-cyan-500" />
              <span>{$_('log.filterModal.timeRange')}</span>
            </div>
            <span class="text-[10px] text-slate-400">{$_('log.filterModal.shortcuts')}</span>
          </div>

          <!-- Shortcuts Pill Bar -->
          <div class="flex flex-wrap items-center gap-1.5 pt-0.5">
            <button
              type="button"
              onclick={() => applyShortcut("1h")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.last1h')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("6h")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.last6h')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("24h")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.last24h')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("today")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.today')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("yesterday")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.yesterday')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("7d")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.last7d')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("30d")}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-[11px] font-medium text-slate-700 dark:text-slate-300 hover:border-cyan-500 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.last30d')}
            </button>
            <button
              type="button"
              onclick={() => applyShortcut("clear")}
              class="rounded-lg border border-rose-300/60 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 px-2 py-1 text-[11px] font-semibold text-rose-700 dark:text-rose-300 hover:bg-rose-100 dark:hover:bg-rose-900/60 transition-colors cursor-pointer"
            >
              {$_('log.filterModal.clearTime')}
            </button>
          </div>

          <!-- Datetime Inputs -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
            <div>
              <span class="text-[10px] text-slate-500 dark:text-slate-400 block mb-1">{$_('log.filterModal.from')}</span>
              <div class="relative">
                <input
                  type="datetime-local"
                  bind:value={draft.start}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
            <div>
              <span class="text-[10px] text-slate-500 dark:text-slate-400 block mb-1">{$_('log.filterModal.to')}</span>
              <div class="flex items-center gap-1.5">
                <input
                  type="datetime-local"
                  bind:value={draft.end}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
                {#if draft.start || draft.end}
                  <button
                    type="button"
                    title={$_('log.filterModal.clearTime')}
                    onclick={() => applyShortcut("clear")}
                    class="rounded-xl border border-rose-300 dark:border-rose-800 bg-rose-50 dark:bg-rose-950/50 p-2 text-rose-600 dark:text-rose-400 hover:bg-rose-100 dark:hover:bg-rose-900/40 transition-colors cursor-pointer shrink-0"
                  >
                    <X class="h-3.5 w-3.5" />
                  </button>
                {/if}
              </div>
            </div>
          </div>
        </div>

        <!-- 1. EVENT LOG SPECIFIC -->
        {#if logCategory === "event"}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.level')}</div>
              <select
                bind:value={draft.level}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none cursor-pointer"
              >
                <option value="all">{$_('log.filterModal.allLevels')}</option>
                <option value="warn">{$_('log.filterModal.warnAbove')}</option>
                <option value="low">{$_('log.filterModal.lowAbove')}</option>
                <option value="high">{$_('log.filterModal.highOnly')}</option>
                <option value="normal">{$_('status.normal')}</option>
                <option value="info">{$_('status.info')}</option>
              </select>
            </div>

            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.type')}</div>
              <input
                type="text"
                placeholder="polling, system, user, ssh..."
                bind:value={draft.type}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.node')}</div>
            <input
              type="text"
              placeholder="192.168.1.1, server01..."
              bind:value={draft.source}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.event')}</div>
            <input
              type="text"
              placeholder="error, timeout, down..."
              bind:value={draft.keyword}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>

        <!-- 2. SYSLOG SPECIFIC -->
        {:else if logCategory === "syslog"}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.level')}</div>
              <select
                bind:value={draft.level}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none cursor-pointer"
              >
                <option value="all">{$_('log.filterModal.allLevels')}</option>
                <option value="info">{$_('log.filterModal.infoAbove')}</option>
                <option value="warn">{$_('log.filterModal.warnAbove')}</option>
                <option value="low">{$_('log.filterModal.lowAbove')}</option>
                <option value="high">{$_('log.filterModal.highOnly')}</option>
              </select>
            </div>

            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.tag')}</div>
              <input
                type="text"
                placeholder="sshd, kernel, cron, sudo..."
                bind:value={draft.type}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.host')}</div>
            <input
              type="text"
              placeholder="192.168.1.1, host01..."
              bind:value={draft.source}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.message')}</div>
            <input
              type="text"
              placeholder="Failed password, Accepted, error..."
              bind:value={draft.keyword}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>

        <!-- 3. SNMP TRAP SPECIFIC -->
        {:else if logCategory === "trap"}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.source')}</div>
              <input
                type="text"
                placeholder="192.168.1.1..."
                bind:value={draft.source}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.trapType')}</div>
              <input
                type="text"
                placeholder="linkDown, warmStart, 1.3.6.1..."
                bind:value={draft.type}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.variables')}</div>
            <input
              type="text"
              placeholder="ifIndex, link, error..."
              bind:value={draft.keyword}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-600 focus:border-cyan-500 focus:outline-none"
            />
          </div>

        <!-- 4. NETFLOW SPECIFIC -->
        {:else if logCategory === "netflow"}
          <div class="flex items-center gap-2 pt-1 pb-1">
            <label class="flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                bind:checked={draft.single}
                class="rounded border-slate-300 dark:border-slate-700 text-cyan-600 focus:ring-cyan-500 h-4 w-4 cursor-pointer"
              />
              <span class="font-semibold text-slate-700 dark:text-slate-300 text-xs">
                {$_('log.filterModal.singleMode')}
              </span>
            </label>
          </div>

          {#if draft.single}
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">IP アドレス (Src / Dst)</div>
                <input
                  type="text"
                  placeholder="192.168.1.1..."
                  bind:value={draft.source}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">ポート (Src / Dst)</div>
                <input
                  type="text"
                  placeholder="80, 443, 22..."
                  bind:value={draft.srcPort}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
          {:else}
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.srcAddr')}</div>
                <input
                  type="text"
                  placeholder="192.168.1.100..."
                  bind:value={draft.source}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.srcPort')}</div>
                <input
                  type="text"
                  placeholder="54321..."
                  bind:value={draft.srcPort}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.dstAddr')}</div>
                <input
                  type="text"
                  placeholder="8.8.8.8..."
                  bind:value={draft.dstAddr}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.dstPort')}</div>
                <input
                  type="text"
                  placeholder="443, 80..."
                  bind:value={draft.dstPort}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
          {/if}

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.protocol')}</div>
              <input
                type="text"
                placeholder="TCP, UDP, ICMP..."
                bind:value={draft.protocol}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.tcpFlags')}</div>
              <input
                type="text"
                placeholder="SYN, ACK, FIN, RST..."
                bind:value={draft.tcpFlags}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>

          <div class="space-y-1">
            <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.keyword')}</div>
            <input
              type="text"
              placeholder="text / regex..."
              bind:value={draft.keyword}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

        <!-- 5. SFLOW SPECIFIC -->
        {:else if logCategory === "sflow"}
          {#if sflowCounter}
            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.remote')}</div>
                <input
                  type="text"
                  placeholder="192.168.1.1..."
                  bind:value={draft.source}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
              <div class="space-y-1">
                <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.counterType')}</div>
                <input
                  type="text"
                  placeholder="generic, ethernet, processor..."
                  bind:value={draft.type}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.keyword')}</div>
              <input
                type="text"
                placeholder="text / regex..."
                bind:value={draft.keyword}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          {:else}
            <div class="flex items-center gap-2 pt-1 pb-1">
              <label class="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  bind:checked={draft.single}
                  class="rounded border-slate-300 dark:border-slate-700 text-cyan-600 focus:ring-cyan-500 h-4 w-4 cursor-pointer"
                />
                <span class="font-semibold text-slate-700 dark:text-slate-300 text-xs">
                  {$_('log.filterModal.singleMode')}
                </span>
              </label>
            </div>
            {#if draft.single}
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">IP アドレス (Src / Dst)</div>
                  <input
                    type="text"
                    placeholder="192.168.1.1..."
                    bind:value={draft.source}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">ポート (Src / Dst)</div>
                  <input
                    type="text"
                    placeholder="80, 443..."
                    bind:value={draft.srcPort}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>
            {:else}
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.srcAddr')}</div>
                  <input
                    type="text"
                    placeholder="192.168.1.10..."
                    bind:value={draft.source}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.srcPort')}</div>
                  <input
                    type="text"
                    placeholder="80, 443..."
                    bind:value={draft.srcPort}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.dstAddr')}</div>
                  <input
                    type="text"
                    placeholder="192.168.1.20..."
                    bind:value={draft.dstAddr}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
                <div class="space-y-1">
                  <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.dstPort')}</div>
                  <input
                    type="text"
                    placeholder="80, 443..."
                    bind:value={draft.dstPort}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>
            {/if}
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.keyword')}</div>
              <input
                type="text"
                placeholder="text / regex..."
                bind:value={draft.keyword}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          {/if}

        <!-- 6. ARP WATCH SPECIFIC -->
        {:else if logCategory === "arp"}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.col.ip')}</div>
              <input
                type="text"
                placeholder="192.168.1.1..."
                bind:value={draft.source}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.mac')}</div>
              <input
                type="text"
                placeholder="aa:bb:cc:dd:ee:ff..."
                bind:value={draft.mac}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.state')}</div>
              <input
                type="text"
                placeholder="new, change, vendor..."
                bind:value={draft.state}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
            <div class="space-y-1">
              <div class="font-semibold text-slate-700 dark:text-slate-300">{$_('log.filterModal.keyword')}</div>
              <input
                type="text"
                placeholder="search keyword..."
                bind:value={draft.keyword}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
              />
            </div>
          </div>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800 pt-4 mt-2">
        <button
          type="button"
          onclick={handleReset}
          class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3.5 py-2 text-xs font-semibold text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          <RotateCcw class="h-3.5 w-3.5 text-slate-400" />
          <span>{$_('log.filterModal.reset')}</span>
        </button>

        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={() => (show = false)}
            class="rounded-xl px-4 py-2 text-xs font-semibold text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 transition-colors cursor-pointer"
          >
            {$_('log.filterModal.cancel')}
          </button>
          <button
            type="button"
            onclick={handleSearch}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-5 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
          >
            <Search class="h-4 w-4" />
            <span>{$_('log.filterModal.apply')}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}
