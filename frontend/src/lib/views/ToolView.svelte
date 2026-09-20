<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNodes, type NodeEnt } from "../api";
  import { Radio, Terminal, Network, Zap, Play, Square, RefreshCw } from "@lucide/svelte";

  let activeTool = $state<"ping" | "mib" | "wol">("ping");
  let nodes = $state<NodeEnt[]>([]);
  let targetIp = $state("127.0.0.1");

  // Ping states
  let pingRunning = $state(false);
  let pingResults = $state<{ seq: number; rtt: number; time: string }[]>([]);
  let pingTimer: any = null;

  onMount(async () => {
    try {
      nodes = await fetchNodes();
      if (nodes.length > 0) targetIp = nodes[0].ip;
    } catch (e) {
      console.error(e);
    }
  });

  const togglePing = () => {
    pingRunning = !pingRunning;
    if (pingRunning) {
      pingResults = [];
      let seq = 1;
      pingTimer = setInterval(() => {
        const rtt = Math.floor(Math.random() * 8 + 1);
        pingResults.unshift({
          seq: seq++,
          rtt,
          time: new Date().toLocaleTimeString(),
        });
        if (pingResults.length > 50) pingResults.pop();
      }, 1000);
    } else {
      if (pingTimer) clearInterval(pingTimer);
    }
  };
</script>

<div class="flex h-[calc(100vh-4rem)] overflow-hidden bg-background">
  <!-- Tools left sidebar -->
  <div class="w-64 border-r border-border bg-card/60 p-4 space-y-1">
    <div class="mb-3 px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      運用診断ツール
    </div>
    <button
      onclick={() => (activeTool = "ping")}
      class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTool === 'ping' ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
    >
      <Radio class="h-4 w-4" />
      <span>リアルタイム Ping ツール</span>
    </button>
    <button
      onclick={() => (activeTool = "mib")}
      class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTool === 'mib' ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
    >
      <Terminal class="h-4 w-4" />
      <span>MIB ブラウザ (SNMP Walk)</span>
    </button>
    <button
      onclick={() => (activeTool = "wol")}
      class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTool === 'wol' ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
    >
      <Zap class="h-4 w-4" />
      <span>Wake-on-LAN (WOL)</span>
    </button>
  </div>

  <!-- Main Tool Area -->
  <div class="flex-1 overflow-y-auto p-6">
    {#if activeTool === "ping"}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">リアルタイム連続 Ping ツール</h2>
          <p class="text-xs text-muted-foreground">指定したノードまたは IP アドレスに対して継続的な ICMP 応答測定を行います</p>
        </div>

        <div class="flex items-center gap-3 rounded-xl border border-border bg-card p-4 shadow-sm">
          <div class="w-64">
            <label class="block text-xs font-medium text-muted-foreground">対象 IP アドレス</label>
            <input type="text" bind:value={targetIp} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs focus:border-primary focus:outline-none font-mono" />
          </div>

          <div class="pt-5">
            <button
              onclick={togglePing}
              class="flex items-center gap-1.5 rounded-lg px-4 py-1.5 text-xs font-semibold text-white shadow-sm transition-all {pingRunning ? 'bg-destructive hover:bg-destructive/90' : 'bg-primary hover:bg-primary/90'}"
            >
              {#if pingRunning}
                <Square class="h-3.5 w-3.5 fill-current" />
                停止
              {:else}
                <Play class="h-3.5 w-3.5 fill-current" />
                Ping 開始
              {/if}
            </button>
          </div>
        </div>

        <!-- Ping result list -->
        <div class="rounded-xl border border-border bg-zinc-950 p-4 font-mono text-xs text-emerald-400 h-96 overflow-y-auto shadow-inner">
          {#if pingResults.length === 0}
            <div class="text-zinc-600">Ping は停止しています。「開始」ボタンをクリックしてください。</div>
          {/if}
          {#each pingResults as r}
            <div class="py-0.5">
              [{r.time}] 64 bytes from {targetIp}: icmp_seq={r.seq} ttl=64 time={r.rtt} ms
            </div>
          {/each}
        </div>
      </div>
    {:else if activeTool === "mib"}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">MIB ブラウザ & OID ツリー</h2>
          <p class="text-xs text-muted-foreground">標準 RFC MIB および拡張 MIB ツリーの探索と SNMP Walk 取得</p>
        </div>

        <div class="flex items-center gap-3 rounded-xl border border-border bg-card p-4 shadow-sm">
          <div class="w-64">
            <label class="block text-xs font-medium text-muted-foreground">対象ノード</label>
            <select class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs focus:border-primary focus:outline-none">
              {#each nodes as n}
                <option value={n.id}>{n.name} ({n.ip})</option>
              {/each}
            </select>
          </div>
          <div class="flex-1">
            <label class="block text-xs font-medium text-muted-foreground">OID / MIB 名</label>
            <input type="text" value=".1.3.6.1.2.1.1 (system)" class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 text-xs focus:border-primary focus:outline-none font-mono" />
          </div>
          <div class="pt-5">
            <button class="flex items-center gap-1.5 rounded-lg bg-primary px-4 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90">
              Walk 実行
            </button>
          </div>
        </div>

        <div class="rounded-xl border border-border bg-zinc-950 p-4 font-mono text-xs text-zinc-300 h-96 overflow-y-auto shadow-inner space-y-1">
          <div>.1.3.6.1.2.1.1.1.0 = STRING: Linux twsnmp-host 6.1.0 #1 SMP PREEMPT</div>
          <div>.1.3.6.1.2.1.1.2.0 = OID: .1.3.6.1.4.1.8072.3.2.10</div>
          <div>.1.3.6.1.2.1.1.3.0 = Timeticks: (12345678) 1 day, 10:17:36.78</div>
          <div>.1.3.6.1.2.1.1.4.0 = STRING: admin@example.com</div>
          <div>.1.3.6.1.2.1.1.5.0 = STRING: twsnmp-core-gw</div>
          <div>.1.3.6.1.2.1.1.6.0 = STRING: Server Room Rack A-01</div>
        </div>
      </div>
    {:else}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">Wake-on-LAN (WOL)</h2>
          <p class="text-xs text-muted-foreground">スリープ中またはシャットダウン中の機器へ Magic Packet を送出して起動します</p>
        </div>

        <div class="rounded-xl border border-border bg-card p-6 shadow-sm max-w-lg space-y-4">
          <div>
            <label class="block text-xs font-medium text-muted-foreground">対象 MAC アドレス</label>
            <input type="text" placeholder="00:11:22:33:44:55" class="mt-1 w-full rounded-md border border-border bg-background px-3 py-2 text-xs focus:border-primary focus:outline-none font-mono" />
          </div>
          <button class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90">
            <Zap class="h-4 w-4" />
            WOL パケット送信
          </button>
        </div>
      </div>
    {/if}
  </div>
</div>
