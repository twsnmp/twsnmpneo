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
  import {
    Map as MapIcon,
    Server,
    Activity,
    FileText,
    BarChart3,
    Wrench,
    Settings,
    Info,
    Moon,
    Sun,
    Bot,
  } from "@lucide/svelte";

  type PageType = "map" | "nodes" | "pollings" | "logs" | "reports" | "tools" | "system";

  let currentPage = $state<PageType>("map");
  let isDark = $state(true);
  let showConfig = $state(false);
  let showAI = $state(false);

  onMount(() => {
    // Default dark theme
    document.documentElement.classList.add("dark");
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
    { id: "map", label: "マップ", icon: MapIcon },
    { id: "nodes", label: "ノード", icon: Server },
    { id: "pollings", label: "ポーリング", icon: Activity },
    { id: "logs", label: "ログ", icon: FileText },
    { id: "reports", label: "レポート", icon: BarChart3 },
    { id: "tools", label: "ツール", icon: Wrench },
    { id: "system", label: "システム", icon: Info },
  ];
</script>

<div class="flex h-screen flex-col bg-background text-foreground overflow-hidden font-sans">
  <!-- Top Navigation Bar (twsnmpfk style) -->
  <header class="flex h-16 shrink-0 items-center justify-between border-b border-border bg-card/80 px-6 backdrop-blur-md z-30">
    <!-- Brand -->
    <div class="flex items-center gap-3">
      <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-tr from-cyan-500 to-blue-600 shadow-md">
        <span class="font-black text-white text-base tracking-wider">NEO</span>
      </div>
      <div>
        <h1 class="text-sm font-bold tracking-tight text-foreground flex items-center gap-1.5">
          TWSNMP NEO
          <span class="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-semibold text-primary">v2.0</span>
        </h1>
      </div>
    </div>

    <!-- Navigation Tabs -->
    <nav class="flex items-center gap-1 rounded-xl border border-border/60 bg-muted/30 p-1">
      {#each navItems as item}
        <button
          onclick={() => (currentPage = item.id as PageType)}
          class="flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-xs font-semibold transition-all {currentPage === item.id ? 'bg-background text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'}"
        >
          <item.icon class="h-4 w-4" />
          <span>{item.label}</span>
        </button>
      {/each}
    </nav>

    <!-- Right Controls -->
    <div class="flex items-center gap-2">
      <!-- AI Assistant Button -->
      <button
        onclick={() => (showAI = !showAI)}
        class="flex items-center gap-1.5 rounded-lg border border-primary/40 bg-primary/10 px-3 py-1.5 text-xs font-semibold text-primary hover:bg-primary/20 transition-colors shadow-sm"
      >
        <Bot class="h-4 w-4" />
        <span>AI アシスタント</span>
      </button>

      <!-- Config Button -->
      <button
        onclick={() => (showConfig = true)}
        class="rounded-lg p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
        title="システム設定"
      >
        <Settings class="h-4 w-4" />
      </button>

      <!-- Theme Toggle -->
      <button
        onclick={toggleTheme}
        class="rounded-lg p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
        title="ダーク/ライトモード切替"
      >
        {#if isDark}
          <Sun class="h-4 w-4" />
        {:else}
          <Moon class="h-4 w-4" />
        {/if}
      </button>
    </div>
  </header>

  <!-- Main View Router -->
  <main class="relative flex-1 overflow-hidden">
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

    <!-- AI Assistant Drawer Overlay -->
    {#if showAI}
      <div class="absolute top-0 right-0 z-40 h-full w-96 border-l border-border bg-card shadow-2xl">
        <AIAssistant />
      </div>
    {/if}
  </main>

  <!-- Global Config Modal -->
  <ConfigModal bind:show={showConfig} />
</div>
