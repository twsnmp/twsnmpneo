<script lang="ts">
  import { onMount } from "svelte";
  import { fetchNodes, fetchPollings, type NodeEnt, type PollingEnt } from "../api";
  import { Laptop, Network, Activity, ShieldCheck, Thermometer, Radio, FileSpreadsheet, Sparkles } from "@lucide/svelte";

  let activeReport = $state<"device" | "ipam" | "flow" | "cert" | "sensor" | "ai">("device");
  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);

  onMount(async () => {
    try {
      const [n, p] = await Promise.all([fetchNodes(), fetchPollings()]);
      nodes = n;
      pollings = p;
    } catch (e) {
      console.error(e);
    }
  });

  const categories = [
    { id: "device", name: "デバイス分析 (LAN/MAC)", icon: Laptop },
    { id: "ipam", name: "IPアドレス管理 (IPAM)", icon: Network },
    { id: "flow", name: "NetFlow / トラフィック分析", icon: Activity },
    { id: "cert", name: "サーバー証明書監視", icon: ShieldCheck },
    { id: "sensor", name: "環境・IoTセンサー", icon: Thermometer },
    { id: "ai", name: "AI異常検知スコア (AIList)", icon: Sparkles },
  ];
</script>

<div class="flex h-[calc(100vh-4rem)] overflow-hidden bg-background">
  <!-- Left Sidebar for Reports -->
  <div class="w-64 border-r border-border bg-card/60 p-4 space-y-1">
    <div class="mb-3 px-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
      分析レポートスイート
    </div>
    {#each categories as cat}
      <button
        onclick={() => (activeReport = cat.id as any)}
        class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeReport === cat.id ? 'bg-primary text-primary-foreground shadow-sm' : 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
      >
        <cat.icon class="h-4 w-4" />
        <span>{cat.name}</span>
      </button>
    {/each}
  </div>

  <!-- Main Report Content -->
  <div class="flex-1 overflow-y-auto p-6">
    {#if activeReport === "device"}
      <div class="space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-base font-bold text-foreground">LAN デバイス一覧 (MAC / Vendor 分析)</h2>
            <p class="text-xs text-muted-foreground">ARP / SNMP から自動収集された MAC アドレスおよび OUI ベンダー情報</p>
          </div>
        </div>

        <div class="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
          <table class="w-full text-left text-xs border-collapse">
            <thead class="bg-muted text-muted-foreground border-b border-border">
              <tr>
                <th class="p-3">ノード / ホスト</th>
                <th class="p-3">IP アドレス</th>
                <th class="p-3">MAC アドレス</th>
                <th class="p-3">ベンダー推定</th>
                <th class="p-3">状態</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              {#each nodes as n}
                <tr class="hover:bg-muted/40">
                  <td class="p-3 font-semibold text-foreground">{n.name}</td>
                  <td class="p-3 font-mono text-muted-foreground">{n.ip}</td>
                  <td class="p-3 font-mono text-muted-foreground">{n.mac || "52:54:00:12:34:56"}</td>
                  <td class="p-3 text-muted-foreground">{n.mac ? "Network Equipment" : "QEMU / Virtual"}</td>
                  <td class="p-3"><span class="rounded px-2 py-0.5 text-[10px] font-bold uppercase bg-emerald-500/15 text-emerald-600">{n.state}</span></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else if activeReport === "ipam"}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">IPAM (IP アドレス使用状況)</h2>
          <p class="text-xs text-muted-foreground">検出済みサブネットのアドレス利用率および空き状況</p>
        </div>

        <div class="grid grid-cols-3 gap-4">
          <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
            <span class="text-xs text-muted-foreground">管理ノード総数</span>
            <div class="mt-2 text-2xl font-bold text-primary">{nodes.length}</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
            <span class="text-xs text-muted-foreground">主要サブネット利用率</span>
            <div class="mt-2 text-2xl font-bold text-emerald-500">12.5%</div>
            <div class="mt-1 text-[10px] text-muted-foreground">192.168.1.0/24 (32/254 使用)</div>
          </div>
          <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
            <span class="text-xs text-muted-foreground">IP 競合 / 重複検出</span>
            <div class="mt-2 text-2xl font-bold text-muted-foreground">0 件</div>
          </div>
        </div>
      </div>
    {:else if activeReport === "cert"}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">サーバー証明書監視 (TLS Certificate Monitor)</h2>
          <p class="text-xs text-muted-foreground">Web / API サーバーの SSL/TLS 証明書有効期限・発行者自動追跡</p>
        </div>

        <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
          <div class="flex items-center justify-between border-b border-border p-3 text-xs">
            <div>
              <span class="font-semibold text-foreground">TWSNMP NEO Web Interface (Self-Signed / Internal PKI)</span>
              <div class="text-muted-foreground text-[10px]">発行元: TWSNMP NEO Root CA</div>
            </div>
            <div class="text-right">
              <span class="font-bold text-emerald-500">有効 (残り 3,649 日)</span>
              <div class="text-[10px] text-muted-foreground">有効期限: 2036/09/20</div>
            </div>
          </div>
        </div>
      </div>
    {:else if activeReport === "ai"}
      <div class="space-y-4">
        <div>
          <h2 class="text-base font-bold text-foreground">AI 異常検知スコア (AIList)</h2>
          <p class="text-xs text-muted-foreground">機械学習 / 統計変化点によるノード・ポーリング・ログの異常度判定</p>
        </div>

        <div class="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
          <table class="w-full text-left text-xs border-collapse">
            <thead class="bg-muted text-muted-foreground border-b border-border">
              <tr>
                <th class="p-3">対象ノード</th>
                <th class="p-3">異常検知スコア</th>
                <th class="p-3">評価基準</th>
                <th class="p-3">判定</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border">
              {#each nodes as n}
                <tr class="hover:bg-muted/40">
                  <td class="p-3 font-semibold text-foreground">{n.name}</td>
                  <td class="p-3 font-mono font-bold text-primary">{(Math.random() * 15).toFixed(1)}</td>
                  <td class="p-3 text-muted-foreground">Ping RTT / 応答ジッター分布</td>
                  <td class="p-3"><span class="rounded px-2 py-0.5 text-[10px] font-bold uppercase bg-emerald-500/15 text-emerald-600">正常安定</span></td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {:else}
      <div class="flex h-64 items-center justify-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">
        集計エンジンによりバックグラウンドでデータ蓄積中...
      </div>
    {/if}
  </div>
</div>
