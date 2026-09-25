<script lang="ts">
  import { onMount } from "svelte";
  import MapView from "./lib/views/MapView.svelte";
  import ListView from "./lib/views/ListView.svelte";
  import LogView from "./lib/views/LogView.svelte";
  import OTelView from "./lib/views/OTelView.svelte";
  import MQTTView from "./lib/views/MQTTView.svelte";
  import ReportView from "./lib/views/ReportView.svelte";
  import SystemView from "./lib/views/SystemView.svelte";
  import ConfigModal from "./lib/components/ConfigModal.svelte";
  import AIAssistant from "./lib/mcp/AIAssistant.svelte";
  import CatAvatar from "./lib/components/CatAvatar.svelte";
  import { fetchMapConf } from "./lib/api";
  import { _ } from "svelte-i18n";
  import { switchLocale, getSavedLocale, type SupportedLocale } from "./lib/i18n";
  import {
    Network,
    Layers,
    Calendar,
    BarChart3,
    Info,
    Settings,
    Moon,
    Sun,
    Bot,
    HelpCircle,
    Activity,
    Radio,
    Languages,
  } from "@lucide/svelte";

  type PageType = "map" | "list" | "logs" | "otel" | "mqtt" | "reports" | "system";

  let currentPage = $state<PageType>("map");
  let currentLocale = $state<SupportedLocale>(getSavedLocale());
  let isDark = $state(
    typeof window !== "undefined"
      ? localStorage.getItem("twsnmp_theme") !== "light"
      : true
  );
  let showConfig = $state(false);
  let showAI = $state(false);
  let mapName = $state("My Network");

  const refreshConf = async () => {
    try {
      const conf = await fetchMapConf();
      if (conf?.MapName) mapName = conf.MapName;
    } catch {
      // default
    }
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
    }
  };

  onMount(async () => {
    const savedTheme = localStorage.getItem("twsnmp_theme");
    if (savedTheme === "light") {
      isDark = false;
      document.documentElement.classList.remove("dark");
    } else {
      isDark = true;
      document.documentElement.classList.add("dark");
    }
    await refreshConf();
  });

  const toggleTheme = () => {
    isDark = !isDark;
    if (isDark) {
      document.documentElement.classList.add("dark");
      localStorage.setItem("twsnmp_theme", "dark");
    } else {
      document.documentElement.classList.remove("dark");
      localStorage.setItem("twsnmp_theme", "light");
    }
  };

  const toggleLocale = () => {
    const next: SupportedLocale = currentLocale === "ja" ? "en" : "ja";
    currentLocale = next;
    switchLocale(next);
  };

  const navItems = [
    { id: "map", icon: Network },
    { id: "list", icon: Layers },
    { id: "logs", icon: Calendar },
    { id: "otel", icon: Activity, mdi: "mdi-telescope" },
    { id: "mqtt", icon: Radio, mdi: "mdi-access-point-network" },
    { id: "reports", icon: BarChart3 },
    { id: "system", icon: Info },
  ];
</script>

<div class="flex h-screen w-screen flex-col overflow-hidden bg-slate-100 text-slate-800 dark:bg-[#0b1329] dark:text-[#f1f5f9] font-sans">
  <!-- Top Navigation Bar matching twsnmpfk Image 1 + twnoaa palette -->
  <header class="flex h-17 shrink-0 items-center justify-between border-b border-slate-200 bg-white/95 dark:border-slate-800 dark:bg-slate-950 px-4 py-2 shadow-sm dark:shadow-xl z-30 transition-colors">
    <!-- Brand -->
    <div class="flex items-center gap-3">
      <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-600 dark:text-cyan-400 shadow-sm">
        <Activity class="h-5 w-5 animate-pulse" />
      </div>
      <div>
        <h1 class="text-sm font-bold tracking-tight text-slate-900 dark:text-slate-100 flex items-center gap-2">
          {$_('app.title')}
          <span class="text-xs font-normal text-slate-500 dark:text-slate-400">- {mapName}</span>
        </h1>
        <p class="text-[10px] text-slate-500 dark:text-slate-400 font-medium">{$_('app.subtitle')}</p>
      </div>
    </div>

    <!-- Center Navigation Tabs with Stacked Icon + Label (twsnmpfk style with twnoaa styling) -->
    <nav class="flex items-center gap-1 bg-slate-100 dark:bg-slate-900/90 p-1 rounded-xl border border-slate-200 dark:border-slate-800">
      {#each navItems as item}
        <button
          onclick={() => (currentPage = item.id as PageType)}
          class="flex flex-col items-center justify-center min-w-[58px] py-1 px-2.5 rounded-lg text-[11px] font-medium transition-all {currentPage === item.id ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30 font-semibold' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-200 dark:text-slate-400 dark:hover:text-slate-200 dark:hover:bg-slate-800/60'}"
        >
          {#if item.mdi}
            <span class="mdi {item.mdi} text-base leading-none mb-0.5 {currentPage === item.id ? 'text-white' : 'text-slate-500 dark:text-slate-400'}"></span>
          {:else}
            <item.icon class="h-4 w-4 mb-0.5 {currentPage === item.id ? 'text-white' : 'text-slate-500 dark:text-slate-400'}" />
          {/if}
          <span>{$_('nav.' + item.id)}</span>
        </button>
      {/each}
    </nav>

    <!-- Right Controls -->
    <div class="flex items-center gap-2">
      <!-- AI Assistant Button -->
      <button
        onclick={() => (showAI = !showAI)}
        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 px-3 py-1.5 text-xs font-semibold text-cyan-600 dark:text-cyan-300 hover:bg-cyan-500/20 transition-all shadow-sm cursor-pointer"
      >
        <CatAvatar size="w-5 h-5" rounded="rounded-md" />
        <span>{$_('nav.aiAssistant')}</span>
      </button>

      <!-- Language Switcher -->
      <button
        onclick={toggleLocale}
        title={$_('nav.language')}
        class="flex h-8 items-center gap-1 px-2.5 rounded-xl border border-slate-200 bg-slate-50 text-slate-700 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-slate-700 dark:hover:text-white transition-colors text-xs font-semibold"
      >
        <Languages class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
        <span>{currentLocale.toUpperCase()}</span>
      </button>

      <!-- Settings Button -->
      <button
        onclick={() => (showConfig = true)}
        title={$_('nav.settings')}
        class="flex h-8 w-8 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white transition-colors"
      >
        <Settings class="h-4 w-4 text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200" />
      </button>

      <!-- Theme Toggle -->
      <button
        onclick={toggleTheme}
        title={$_('nav.theme')}
        class="flex h-8 w-8 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white transition-colors"
      >
        {#if isDark}
          <Sun class="h-4 w-4 text-amber-400" />
        {:else}
          <Moon class="h-4 w-4 text-cyan-600" />
        {/if}
      </button>

      <!-- Help Button -->
      <button
        onclick={() => alert($_('help.mapUsage'))}
        title={$_('nav.help')}
        class="flex h-8 w-8 items-center justify-center rounded-xl border border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white transition-colors"
      >
        <HelpCircle class="h-4 w-4 text-slate-500 dark:text-slate-400" />
      </button>
    </div>
  </header>

  <!-- Main View Area -->
  <main class="flex-1 overflow-hidden relative">
    {#if currentPage === "map"}
      <MapView />
    {:else if currentPage === "list"}
      <ListView />
    {:else if currentPage === "logs"}
      <LogView />
    {:else if currentPage === "otel"}
      <OTelView />
    {:else if currentPage === "mqtt"}
      <MQTTView />
    {:else if currentPage === "reports"}
      <ReportView />
    {:else if currentPage === "system"}
      <SystemView />
    {/if}
  </main>

  <!-- Global Modals & Drawers -->
  <ConfigModal bind:show={showConfig} onSaved={refreshConf} />

  <!-- AI Cat Assistant Drawer (Zero-latency instant slide-in) -->
  <div
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    class="fixed inset-0 z-50 flex justify-end transition-all duration-200 {showAI ? 'pointer-events-auto visible' : 'pointer-events-none invisible'}"
    onkeydown={(e) => e.key === "Escape" && (showAI = false)}
  >
    <button
      type="button"
      class="fixed inset-0 bg-black/50 transition-opacity duration-200 cursor-default {showAI ? 'opacity-100' : 'opacity-0'}"
      aria-label={$_('common.close')}
      onclick={() => (showAI = false)}
    ></button>
    <div
      class="relative z-10 h-full w-full max-w-lg bg-white dark:bg-slate-900 border-l border-slate-200 dark:border-slate-800 shadow-2xl flex flex-col transform transition-transform duration-200 ease-out {showAI ? 'translate-x-0' : 'translate-x-full'}"
    >
      <AIAssistant onClose={() => (showAI = false)} />
    </div>
  </div>

  <!-- Preload Material Design Icons font so browser immediately fetches woff2 -->
  <span class="mdi mdi-monitor pointer-events-none fixed -top-[9999px] -left-[9999px] opacity-0" aria-hidden="true"></span>
</div>
