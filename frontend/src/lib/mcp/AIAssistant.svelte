<script lang="ts">
  import { askAI } from '$lib/api';
  import { Bot, Send, User, Sparkles, Loader2, AlertCircle, X } from '@lucide/svelte';

  let { onClose = () => {} } = $props<{ onClose?: () => void }>();

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
      content: 'こんにちは！TWSNMP NEO AI アシスタントです。ネットワークのトポロジー、障害アラート、ログ、テレメトリ解析について何でもご質問ください。',
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    },
  ]);

  let inputPrompt = $state('');
  let isLoading = $state(false);
  let chatError = $state<string | null>(null);

  const quickPrompts = [
    '直近のアラートと障害状況を要約して',
    'SNMP v3 の推奨設定手順を教えて',
    'スイッチ間のPing遅延増加の原因調査方法は？',
    'Syslog の Parquet 保存と保持期間のベストプラクティス',
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
      const systemPrompt = 'You are TWSNMP NEO AI Assistant, an expert network engineer, SRE, and cybersecurity analyst. Respond in Japanese in a clear, concise, and actionable manner.';
      const answer = await askAI(text, systemPrompt);

      const aiMsg: ChatMessage = {
        id: `ai-${Date.now()}`,
        role: 'assistant',
        content: answer,
        time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      };
      messages = [...messages, aiMsg];
    } catch (e: any) {
      chatError = e.message || 'AI 応答の生成に失敗しました';
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

<div class="flex flex-col h-full bg-slate-900 border-l border-slate-800 shadow-2xl overflow-hidden font-sans">
  <!-- Header -->
  <div class="px-5 py-3.5 bg-slate-950 border-b border-slate-800 flex items-center justify-between">
    <div class="flex items-center space-x-2.5">
      <div class="p-2 bg-cyan-500/10 text-cyan-400 border border-cyan-500/30 rounded-xl">
        <Sparkles class="w-4 h-4" />
      </div>
      <div>
        <h3 class="text-sm font-bold text-slate-100 flex items-center gap-2">
          AI Assistant & MCP Client
          <span class="text-[10px] px-2 py-0.5 rounded-full bg-cyan-500/20 text-cyan-300 font-mono border border-cyan-500/30">
            Copilot
          </span>
        </h3>
        <p class="text-xs text-slate-400">Intelligent network root-cause & query copilot</p>
      </div>
    </div>
    <button
      onclick={onClose}
      class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-white transition-colors"
      title="閉じる"
    >
      <X class="w-5 h-5" />
    </button>
  </div>

  <!-- Messages List -->
  <div class="flex-1 overflow-y-auto p-4 space-y-4">
    {#each messages as msg}
      <div class="flex items-start gap-3 {msg.role === 'user' ? 'flex-row-reverse' : ''}">
        <div class="w-8 h-8 rounded-full flex items-center justify-center shrink-0 {msg.role === 'user' ? 'bg-cyan-600 text-white shadow-md shadow-cyan-600/30' : 'bg-slate-800 text-cyan-400 border border-slate-700'}">
          {#if msg.role === 'user'}
            <User class="w-4 h-4" />
          {:else}
            <Bot class="w-4 h-4" />
          {/if}
        </div>

        <div class="max-w-[85%] space-y-1">
          <div class="flex items-center gap-2 {msg.role === 'user' ? 'justify-end' : ''}">
            <span class="text-xs font-semibold text-slate-300">
              {msg.role === 'user' ? 'あなた' : 'NEO Copilot'}
            </span>
            <span class="text-[10px] text-slate-500 font-mono">{msg.time}</span>
          </div>
          <div class="p-3.5 rounded-2xl text-xs leading-relaxed whitespace-pre-wrap shadow-md {
            msg.role === 'user'
              ? 'bg-cyan-600 text-white rounded-tr-none'
              : 'bg-slate-950/80 text-slate-200 border border-slate-800 rounded-tl-none'
          }">
            {msg.content}
          </div>
        </div>
      </div>
    {/each}

    {#if isLoading}
      <div class="flex items-start gap-3">
        <div class="w-8 h-8 rounded-full bg-slate-800 text-cyan-400 border border-slate-700 flex items-center justify-center shrink-0">
          <Bot class="w-4 h-4" />
        </div>
        <div class="p-3.5 bg-slate-950/80 text-slate-300 border border-slate-800 rounded-2xl rounded-tl-none flex items-center space-x-2 text-xs">
          <Loader2 class="w-4 h-4 animate-spin text-cyan-400" />
          <span>LLM 推論中... ネットワーク状態を解析しています</span>
        </div>
      </div>
    {/if}

    {#if chatError}
      <div class="p-3 bg-rose-950/60 border border-rose-800/80 text-rose-300 rounded-xl text-xs flex items-center gap-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-rose-400" />
        <span>{chatError}</span>
      </div>
    {/if}
  </div>

  <!-- Quick Prompts -->
  <div class="px-4 py-2.5 bg-slate-950/70 border-t border-slate-800 flex items-center gap-2 overflow-x-auto text-xs">
    <span class="text-slate-500 text-[11px] whitespace-nowrap">Suggested:</span>
    {#each quickPrompts as prompt}
      <button
        onclick={() => handleSend(prompt)}
        class="px-2.5 py-1 rounded-full bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700 transition-colors whitespace-nowrap shrink-0 text-[11px]"
      >
        {prompt}
      </button>
    {/each}
  </div>

  <!-- Input Box -->
  <div class="p-3.5 bg-slate-950 border-t border-slate-800">
    <div class="flex items-center gap-2">
      <input
        type="text"
        bind:value={inputPrompt}
        onkeydown={handleKeydown}
        placeholder="ネットワークの障害や設定について質問..."
        disabled={isLoading}
        class="flex-1 bg-slate-900 border border-slate-700 rounded-xl px-3.5 py-2 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 disabled:opacity-50 font-sans"
      />
      <button
        onclick={() => handleSend()}
        disabled={isLoading || !inputPrompt.trim()}
        class="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white rounded-xl transition-all flex items-center justify-center shrink-0 shadow-md shadow-cyan-600/30"
      >
        <Send class="w-4 h-4" />
      </button>
    </div>
  </div>
</div>
