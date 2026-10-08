<script lang="ts">
  import type { ModelInfo, AIHardwareStatus } from "../../api";
  import ModelManagerDialog from "../ModelManagerDialog.svelte";
  import {
    Brain,
    Cpu,
    Sparkles,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    llmProvider = $bindable("tensai"),
    llmBaseUrl = $bindable("http://localhost:11434"),
    llmModel = $bindable("qwen2.5-0.5b"),
    llmApiKey = $bindable(""),
    enableMCP = $bindable(true),
    mcpMode = $bindable("noauth"),
    mcpFrom = $bindable(""),
    localAIModels = $bindable([]),
    aiHardware = $bindable(null),
    onModelsChanged,
  }: {
    llmProvider: string;
    llmBaseUrl: string;
    llmModel: string;
    llmApiKey: string;
    enableMCP: boolean;
    mcpMode: string;
    mcpFrom: string;
    localAIModels: ModelInfo[];
    aiHardware: AIHardwareStatus | null;
    onModelsChanged?: () => Promise<void>;
  } = $props();

  let showModelManager = $state(false);

  function handleProviderChange() {
    if (llmProvider === "tensai" && (!llmModel || llmModel.includes("gemini") || llmModel.includes("gpt") || llmModel.includes("claude"))) {
      llmModel = "qwen2.5-0.5b";
    } else if (llmProvider === "gemini" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
      llmModel = "gemini-1.5-flash";
    } else if (llmProvider === "openai" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
      llmModel = "gpt-4o-mini";
    } else if (llmProvider === "claude" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
      llmModel = "claude-3-5-sonnet-20241022";
    } else if (llmProvider === "ollama" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
      llmModel = "llama3";
    }
  }
</script>

<div class="space-y-6 max-w-2xl">
  <!-- AI Provider & Model Config -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Brain class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.aiTitle')}
    </h3>
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div>
        <label for="llm-provider" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiProvider')}</label>
        <select
          id="llm-provider"
          bind:value={llmProvider}
          onchange={handleProviderChange}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          <option value="none">{$_('config.aiProviderNone')}</option>
          <option value="tensai">{$_('config.aiTensai')}</option>
          <option value="gemini">{$_('config.aiGeminiRecommended')}</option>
          <option value="openai">{$_('config.aiOpenAI')}</option>
          <option value="claude">{$_('config.aiClaude')}</option>
          <option value="ollama">{$_('config.aiOllama')}</option>
        </select>
      </div>

      {#if llmProvider === "tensai"}
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <label for="llm-model-tensai" class="block text-xs font-semibold text-slate-600 dark:text-slate-400">
              {$_('config.aiTensaiModel')}
            </label>
            {#if aiHardware}
              <div class="flex items-center gap-1.5 text-[11px]">
                <span class="text-slate-400">{$_('config.aiHardware')}:</span>
                {#if aiHardware.acceleration === "GPU"}
                  <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">GPU</span>
                {:else if aiHardware.acceleration && aiHardware.acceleration.includes("SIMD")}
                  <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">SIMD</span>
                {:else}
                  <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20">CPU</span>
                {/if}
              </div>
            {/if}
          </div>

          <div class="flex gap-2 items-center">
            <div class="flex-1">
              {#if localAIModels.length > 0}
                <select
                  id="llm-model-tensai"
                  bind:value={llmModel}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                >
                  {#each localAIModels as m}
                    <option value={m.name}>{m.name} ({m.size_human})</option>
                  {/each}
                </select>
              {:else}
                <button
                  type="button"
                  onclick={() => (showModelManager = true)}
                  class="w-full text-left px-3.5 py-2 rounded-xl border border-amber-300 dark:border-amber-800/80 bg-amber-50 dark:bg-amber-950/30 text-xs font-medium text-amber-800 dark:text-amber-300 hover:bg-amber-100 transition-colors cursor-pointer"
                >
                  {$_('config.aiNoLocalModel')}
                </button>
              {/if}
            </div>
            <button
              type="button"
              onclick={() => (showModelManager = true)}
              class="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-sm transition-all shrink-0 cursor-pointer"
            >
              <Cpu class="w-3.5 h-3.5" />
              <span>{$_('config.aiManageModels')}</span>
            </button>
          </div>
        </div>
      {:else if llmProvider && llmProvider !== "none"}
        <div>
          <label for="llm-model" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiModel')}</label>
          <input
            id="llm-model"
            type="text"
            bind:value={llmModel}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
          />
        </div>
      {/if}
    </div>

    {#if llmProvider === "tensai"}
      <div class="p-3.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/30 border border-cyan-200 dark:border-cyan-900/50 text-xs text-cyan-800 dark:text-cyan-300 flex items-start gap-2.5">
        <Sparkles class="w-4 h-4 text-cyan-500 shrink-0 mt-0.5" />
        <div class="space-y-1">
          <p class="font-semibold">{$_('config.aiTensai')}</p>
          <p class="text-[11px] leading-relaxed text-cyan-700 dark:text-cyan-400">
            {$_('config.aiTensaiDesc')}
          </p>
          <p class="text-[11px] leading-relaxed text-cyan-600/90 dark:text-cyan-400/90">
            {$_('config.aiTensaiModelHelp')}
          </p>
        </div>
      </div>
    {:else if llmProvider === "ollama"}
      <div>
        <label for="llm-url" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiOllamaUrl')}</label>
        <input
          id="llm-url"
          type="text"
          bind:value={llmBaseUrl}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    {:else if llmProvider && llmProvider !== "none"}
      <div>
        <label for="llm-key" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiApiKey')}</label>
        <input
          id="llm-key"
          type="password"
          bind:value={llmApiKey}
          placeholder="{llmProvider.toUpperCase()} {$_('config.aiApiKeyPlaceholder')}"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    {/if}
  </div>

  <!-- MCP Server Settings -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Cpu class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        {$_('config.mcpTitle')}
      </h3>
      <label class="relative inline-flex items-center cursor-pointer">
        <input type="checkbox" bind:checked={enableMCP} class="sr-only peer" />
        <div class="w-9 h-5 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-slate-600 peer-checked:bg-cyan-600"></div>
        <span class="ml-2 text-xs font-medium text-slate-700 dark:text-slate-300">{$_('config.mcpEnable')}</span>
      </label>
    </div>

    {#if enableMCP}
      <div class="space-y-4">
        <!-- Endpoint Display -->
        <div>
          <span class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpEndpoint')}</span>
          <div class="flex items-center gap-2">
            <input
              type="text"
              readonly
              value="/api/mcp"
              class="w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-950/60 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:outline-none select-all"
            />
          </div>
          <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
            {$_('config.mcpEndpointDesc')}
          </p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Auth Mode -->
          <div>
            <label for="mcp-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpMode')}</label>
            <select
              id="mcp-mode"
              bind:value={mcpMode}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
            >
              <option value="noauth">{$_('config.mcpModeNoAuth')}</option>
              <option value="auth">{$_('config.mcpModeAuth')}</option>
            </select>
          </div>

          <!-- Client IP Filter (mcpFrom) -->
          <div>
            <label for="mcp-from" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpFrom')}</label>
            <input
              id="mcp-from"
              type="text"
              bind:value={mcpFrom}
              placeholder={$_('config.mcpFromPlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
            />
          </div>
        </div>
        <p class="text-[11px] text-slate-500 dark:text-slate-400">
          {$_('config.mcpFromHelp')}
        </p>
      </div>
    {/if}
  </div>
</div>

<ModelManagerDialog
  bind:show={showModelManager}
  onmodelsChanged={onModelsChanged}
  onclose={onModelsChanged}
/>
