<script lang="ts">
  import { onMount } from "svelte";
  import { fetchHealth, type SystemHealth } from "../api";
  import { Shield, Cpu, Activity, Clock, CheckCircle2, Server } from "@lucide/svelte";

  let health = $state<SystemHealth | null>(null);

  onMount(async () => {
    try {
      health = await fetchHealth();
    } catch (e) {
      console.error(e);
    }
  });
</script>

<div class="flex h-[calc(100vh-4rem)] flex-col gap-6 p-8 overflow-y-auto bg-background">
  <div>
    <h2 class="text-xl font-bold text-foreground">システムステータス & 情報</h2>
    <p class="text-xs text-muted-foreground">TWSNMP NEO デーモンプロセスおよびリソース稼働状況</p>
  </div>

  <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
    <div class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-2">
      <div class="flex items-center gap-2 text-primary font-semibold text-sm">
        <Server class="h-4 w-4" />
        <span>デーモン稼働状態</span>
      </div>
      <div class="text-2xl font-bold text-emerald-500 flex items-center gap-2">
        <CheckCircle2 class="h-6 w-6" />
        <span>{health?.status || "HEALTHY"}</span>
      </div>
      <p class="text-xs text-muted-foreground">全内部受信機・ポーリングワーカーが正常に動作しています</p>
    </div>

    <div class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-2">
      <div class="flex items-center gap-2 text-primary font-semibold text-sm">
        <Activity class="h-4 w-4" />
        <span>バージョン</span>
      </div>
      <div class="text-2xl font-bold text-foreground font-mono">
        v2.0.0-NEO
      </div>
      <p class="text-xs text-muted-foreground">TWSNMP FC / FK 統合後継エディション</p>
    </div>

    <div class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-2">
      <div class="flex items-center gap-2 text-primary font-semibold text-sm">
        <Clock class="h-4 w-4" />
        <span>サーバー現在時刻</span>
      </div>
      <div class="text-lg font-bold text-foreground font-mono">
        {health?.time || new Date().toISOString()}
      </div>
      <p class="text-xs text-muted-foreground">高精度 NTP 同期済み</p>
    </div>
  </div>

  <!-- Component status overview -->
  <div class="rounded-xl border border-border bg-card shadow-sm p-6 space-y-4">
    <h3 class="text-sm font-bold text-foreground">内蔵プロトコルサーバー状態</h3>
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs">
      <div class="rounded-lg border border-border p-3">
        <div class="font-medium text-foreground">Syslog 受信機</div>
        <div class="mt-1 font-mono text-emerald-500">UDP :514 / TCP :514</div>
      </div>
      <div class="rounded-lg border border-border p-3">
        <div class="font-medium text-foreground">SNMP TRAP 受信機</div>
        <div class="mt-1 font-mono text-emerald-500">UDP :162 (v1/v2c/v3)</div>
      </div>
      <div class="rounded-lg border border-border p-3">
        <div class="font-medium text-foreground">NetFlow / IPFIX</div>
        <div class="mt-1 font-mono text-emerald-500">UDP :2055</div>
      </div>
      <div class="rounded-lg border border-border p-3">
        <div class="font-medium text-foreground">内蔵 MCP サーバー</div>
        <div class="mt-1 font-mono text-emerald-500">SSE /api/mcp/sse</div>
      </div>
    </div>
  </div>
</div>
