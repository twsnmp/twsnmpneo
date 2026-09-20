<script lang="ts">
  import { askAI } from '$lib/api';
  import { Bot, Send, User, Sparkles, Loader2, AlertCircle } from '@lucide/svelte';

  interface ChatMessage {
    id: string;
    role: 'user' | 'assistant';
    content: string;
    time: string;
  }

  let messages = $state<ChatMessage[]>([
    {
      id: 'msg-init',
      role: 'assistant',
      content: 'Hello! I am your TWSNMP NEO AI Assistant. How can I help you analyze your network topology, alerts, or telemetry data today?',
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    },
  ]);

  let inputPrompt = $state('');
  let isLoading = $state(false);
  let chatError = $state<string | null>(null);

  const quickPrompts = [
    'Assess network stability and recent alerts',
    'Explain how to configure SNMP v3 polling',
    'Troubleshoot high ping latency across switches',
    'Best practices for Syslog log retention',
  ];

  async function handleSend(textToSend?: string) {
    const text = (textToSend || inputPrompt).trim();
    if (!text || isLoading) return;

    chatError = null;
    const userMsg: ChatMessage = {
      id: `usr-${Date.now()}`,
      role: 'user',
      content: text,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    };
    messages = [...messages, userMsg];
    inputPrompt = '';
    isLoading = true;

    try {
      const systemPrompt = 'You are TWSNMP NEO AI Assistant, an expert network engineer, SRE, and cybersecurity analyst. Provide concise, actionable, and structured guidance.';
      const answer = await askAI(text, systemPrompt);

      const aiMsg: ChatMessage = {
        id: `ai-${Date.now()}`,
        role: 'assistant',
        content: answer,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      };
      messages = [...messages, aiMsg];
    } catch (e: any) {
      chatError = e.message || 'Failed to generate response';
    } finally {
      isLoading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }
</script>

<div class="flex flex-col h-full bg-slate-800/80 border border-slate-700/70 rounded-xl overflow-hidden shadow-sm">
  <!-- Header -->
  <div class="px-5 py-3.5 bg-slate-900/80 border-b border-slate-700 flex items-center justify-between">
    <div class="flex items-center space-x-2.5">
      <div class="p-2 bg-purple-600/20 text-purple-400 border border-purple-500/30 rounded-lg">
        <Sparkles class="w-4 h-4" />
      </div>
      <div>
        <h3 class="text-sm font-semibold text-white">AI Assistant & MCP Client</h3>
        <p class="text-xs text-slate-400">Intelligent network root-cause & query copilot</p>
      </div>
    </div>
  </div>

  <!-- Messages List -->
  <div class="flex-1 overflow-y-auto p-4 space-y-4">
    {#each messages as msg}
      <div class="flex items-start gap-3 {msg.role === 'user' ? 'flex-row-reverse' : ''}">
        <div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 {msg.role === 'user' ? 'bg-blue-600 text-white' : 'bg-purple-600 text-white'}">
          {#if msg.role === 'user'}
            <User class="w-4 h-4" />
          {:else}
            <Bot class="w-4 h-4" />
          {/if}
        </div>

        <div class="max-w-[80%] space-y-1">
          <div class="flex items-center gap-2 {msg.role === 'user' ? 'justify-end' : ''}">
            <span class="text-xs font-semibold text-slate-300">
              {msg.role === 'user' ? 'You' : 'Assistant'}
            </span>
            <span class="text-[10px] text-slate-500">{msg.time}</span>
          </div>
          <div class="p-3.5 rounded-2xl text-sm leading-relaxed whitespace-pre-wrap shadow-sm {
            msg.role === 'user'
              ? 'bg-blue-600 text-white rounded-tr-none'
              : 'bg-slate-900/90 text-slate-200 border border-slate-700/80 rounded-tl-none'
          }">
            {msg.content}
          </div>
        </div>
      </div>
    {/each}

    {#if isLoading}
      <div class="flex items-start gap-3">
        <div class="w-8 h-8 rounded-full bg-purple-600 text-white flex items-center justify-center shrink-0">
          <Bot class="w-4 h-4" />
        </div>
        <div class="p-3.5 bg-slate-900/90 text-slate-300 border border-slate-700/80 rounded-2xl rounded-tl-none flex items-center space-x-2 text-sm">
          <Loader2 class="w-4 h-4 animate-spin text-purple-400" />
          <span>Generating analysis...</span>
        </div>
      </div>
    {/if}

    {#if chatError}
      <div class="p-3 bg-red-950/60 border border-red-800/80 text-red-300 rounded-xl text-xs flex items-center gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-red-400" />
        <span>{chatError}</span>
      </div>
    {/if}
  </div>

  <!-- Quick Prompts -->
  <div class="px-4 py-2 bg-slate-900/50 border-t border-slate-700/60 flex items-center gap-2 overflow-x-auto text-xs">
    <span class="text-slate-500 whitespace-nowrap">Suggested:</span>
    {#each quickPrompts as prompt}
      <button
        onclick={() => handleSend(prompt)}
        class="px-2.5 py-1 rounded-full bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700 transition-colors whitespace-nowrap shrink-0"
      >
        {prompt}
      </button>
    {/each}
  </div>

  <!-- Input Box -->
  <div class="p-3 bg-slate-900/90 border-t border-slate-700">
    <div class="flex items-center gap-2">
      <input
        type="text"
        bind:value={inputPrompt}
        onkeydown={handleKeydown}
        placeholder="Ask questions about network telemetry, alerts, or diagnosis..."
        disabled={isLoading}
        class="flex-1 bg-slate-800 border border-slate-700 rounded-lg px-3.5 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:border-purple-500 disabled:opacity-50"
      />
      <button
        onclick={() => handleSend()}
        disabled={isLoading || !inputPrompt.trim()}
        class="px-4 py-2 bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white rounded-lg transition-colors flex items-center justify-center shrink-0 shadow-sm"
      >
        <Send class="w-4 h-4" />
      </button>
    </div>
  </div>
</div>
