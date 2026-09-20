<script lang="ts">
  import { onMount } from "svelte";
  import MapView from "./lib/views/MapView.svelte";
  import NodeListView from "./lib/views/NodeListView.svelte";
  import PollingListView from "./lib/views/PollingListView.svelte";
  import LogView from "./lib/views/LogView.svelte";
  import ReportView from "./lib/views/ReportView.svelte";
  import ToolView from "./lib/views/ToolView.svelte";
  import SystemView from "./lib/views/SystemView.svelte";
  import ConfigModal from "./lib/components/ConfigModal.svelte";
  import AIAssistant from "./lib/mcp/AIAssistant.svelte";
  import { fetchMapConf } from "./lib/api";
  import {
    Network,
    Laptop,
    CheckSquare,
    Calendar,
    BarChart3,
    Wrench,
    Info,
    Settings,
    Moon,
    Sun,
    Bot,
    HelpCircle,
    Activity,
  } from "@lucide/svelte";

  type PageType = "map" | "nodes" | "pollings" | "logs" | "reports" | "tools" | "system";

  let currentPage = $state<PageType>("map");
  let isDark = $state(true);
  let showConfig = $state(false);
  let showAI = $state(false);
  let mapName = $state("My Network");

  onMount(async () => {
    document.documentElement.classList.add("dark");
    try {
      const conf = await fetchMapConf();
      if (conf?.MapName) mapName = conf.MapName;
    } catch {
      // default
    }
  });

  const toggleTheme = () => {
    isDark = !isDark;
    if (isDark) {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  };

  const navItems = [
    { id: "map", label: "Map", icon: Network },
    { id: "nodes", label: "Node", icon: Laptop },
    { id: "pollings", label: "Polling", icon: CheckSquare },
    { id: "logs", label: "Log", icon: Calendar },
    { id: "reports", label: "Reports", icon: BarChart3 },
    { id: "tools", label: "Tools", icon: Wrench },
    { id: "system", label: "System", icon: Info },
  ];
</script>

<div class="flex h-screen w-screen flex-col overflow-hidden bg-[#0b1329] text-[#f1f5f9] font-sans">
  <!-- Top Navigation Bar matching twsnmpfk Image 1 + twnoaa palette -->
  <header class="flex h-17 shrink-0 items-center justify-between border-b border-slate-800 bg-slate-950 px-4 py-2 shadow-xl z-30">
    <!-- Brand -->
    <div class="flex items-center gap-3">
      <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-400 shadow-sm">
        <Activity class="h-5 w-5 animate-pulse" />
      </div>
      <div>
        <h1 class="text-sm font-bold tracking-tight text-slate-100 flex items-center gap-2">
          TWSNMP NEO
          <span class="text-[10px] px-2 py-0.5 rounded-full bg-cyan-500/20 text-cyan-300 font-mono border border-cyan-500/30">
            v2.0.0
          </span>
          <span class="text-xs font-normal text-slate-400">- {mapName}</span>
        </h1>
        <p class="text-[10px] text-slate-400 font-medium">Next-Gen Intelligent Network Management</p>
      </div>
    </div>

    <!-- Center Navigation Tabs with Stacked Icon + Label (twsnmpfk style with twnoaa styling) -->
    <nav class="flex items-center gap-1 bg-slate-900/90 p-1 rounded-xl border border-slate-800">
      {#each navItems as item}
        <button
          onclick={() => (currentPage = item.id as PageType)}
          class="flex flex-col items-center justify-center min-w-[58px] py-1 px-2.5 rounded-lg text-[11px] font-medium transition-all {currentPage === item.id ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30 font-semibold' : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'}"
        >
          <item.icon class="h-4 w-4 mb-0.5 {currentPage === item.id ? 'text-white' : 'text-slate-400'}" />
          <span>{item.label}</span>
        </button>
      {/each}

      <!-- Settings item in nav -->
      <button
        onclick={() => (showConfig = true)}
        class="flex flex-col items-center justify-center min-w-[58px] py-1 px-2.5 rounded-lg text-[11px] font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 transition-all"
      >
        <Settings class="h-4 w-4 mb-0.5 text-slate-400" />
        <span>Setting</span>
      </button>
    </nav>

    <!-- Right Controls -->
    <div class="flex items-center gap-2">
      <!-- AI Assistant Button -->
      <button
        onclick={() => (showAI = !showAI)}
        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 px-3 py-1.5 text-xs font-semibold text-cyan-300 hover:bg-cyan-500/20 transition-all shadow-sm"
      >
        <Bot class="h-4 w-4 text-cyan-400" />
        <span>AI アシスタント</span>
      </button>

      <!-- Theme Toggle -->
      <button
        onclick={toggleTheme}
        title="テーマ切り替え"
        class="flex h-8 w-8 items-center justify-center rounded-xl border border-slate-800 bg-slate-900 text-slate-300 hover:border-slate-700 hover:text-white transition-colors"
      >
        {#if isDark}
          <Sun class="h-4 w-4 text-amber-400" />
        {:else}
          <Moon class="h-4 w-4 text-cyan-400" />
        {/if}
      </button>

      <!-- Help Button -->
      <button
        onclick={() => alert("TWSNMP NEO ヘルプ: マップ上で右クリックするとノードやSW-HUBの追加、編集、削除メニューが表示されます。")}
        title="ヘルプ"
        class="flex h-8 w-8 items-center justify-center rounded-xl border border-slate-800 bg-slate-900 text-slate-300 hover:border-slate-700 hover:text-white transition-colors"
      >
        <HelpCircle class="h-4 w-4 text-slate-400" />
      </button>
    </div>
  </header>

  <!-- Main View Area -->
  <main class="flex-1 overflow-hidden relative">
    {#if currentPage === "map"}
      <MapView />
    {:else if currentPage === "nodes"}
      <NodeListView />
    {:else if currentPage === "pollings"}
      <PollingListView />
    {:else if currentPage === "logs"}
      <LogView />
    {:else if currentPage === "reports"}
      <ReportView />
    {:else if currentPage === "tools"}
      <ToolView />
    {:else if currentPage === "system"}
      <SystemView />
    {/if}
  </main>

  <!-- Global Modals & Drawers -->
  <ConfigModal bind:show={showConfig} />
  <AIAssistant bind:show={showAI} />
</div>
