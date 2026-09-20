<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { initVPanel, setVPanel } from "../map/vpanel";
  import { getStateColor, getStateName } from "../common";
  import type { NodeEnt, PollingEnt, EventLogEnt } from "../api";
  import { X, Box, ListTree, Activity, FileText, CheckCircle2, RotateCw, ZoomIn, ZoomOut } from "@lucide/svelte";

  let { show = $bindable(false), node = null, pollings = [], logs = [] } = $props<{
    show: boolean;
    node: NodeEnt | null;
    pollings?: PollingEnt[];
    logs?: EventLogEnt[];
  }>();

  let activeTab = $state<"vpanel" | "ports" | "polling" | "logs">("vpanel");
  let rotate = $state(false);
  let vpanelZoom = $state(1.0);
  let power = $state(true);
  let ports = $state<any[]>([]);

  $effect(() => {
    if (show && node) {
      // Generate sample/detected ports for demonstration
      ports = [];
      for (let i = 1; i <= 24; i++) {
        ports.push({
          State: i <= 8 ? "up" : "down",
          Speed: i <= 8 ? 1000 * 1000 * 1000 : 0,
        });
      }

      setTimeout(() => {
        initVPanel("vpanel-canvas-container");
        setVPanel(ports, power, rotate, vpanelZoom, 12);
      }, 100);
    }
  });

  const toggleRotate = () => {
    rotate = !rotate;
    setVPanel(ports, power, rotate, vpanelZoom, 12);
  };

  const handleZoom = (inZoom: boolean) => {
    vpanelZoom = inZoom ? Math.min(vpanelZoom + 0.2, 3.0) : Math.max(vpanelZoom - 0.2, 0.4);
    setVPanel(ports, power, rotate, vpanelZoom, 12);
  };
</script>

{#if show && node}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-md">
    <div class="flex h-[85vh] w-full max-w-5xl flex-col rounded-xl border border-border bg-card shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-border px-6 py-3 bg-muted/30">
        <div class="flex items-center gap-3">
          <div class="h-3.5 w-3.5 rounded-full" style="background-color: {getStateColor(node.state)}"></div>
          <div>
            <h2 class="text-base font-bold text-foreground">{node.name}</h2>
            <p class="text-xs text-muted-foreground">{node.ip} {node.mac ? `(${node.mac})` : ""}</p>
          </div>
        </div>

        <!-- Tab Buttons -->
        <div class="flex rounded-lg border border-border bg-background p-1 text-xs">
          <button
            onclick={() => (activeTab = "vpanel")}
            class="flex items-center gap-1.5 rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'vpanel' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Box class="h-3.5 w-3.5" />
            3D バーチャルパネル
          </button>
          <button
            onclick={() => (activeTab = "ports")}
            class="flex items-center gap-1.5 rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'ports' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <ListTree class="h-3.5 w-3.5" />
            ポート一覧
          </button>
          <button
            onclick={() => (activeTab = "polling")}
            class="flex items-center gap-1.5 rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'polling' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Activity class="h-3.5 w-3.5" />
            ポーリング
          </button>
          <button
            onclick={() => (activeTab = "logs")}
            class="flex items-center gap-1.5 rounded-md px-3 py-1 font-medium transition-colors {activeTab === 'logs' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <FileText class="h-3.5 w-3.5" />
            個別ログ
          </button>
        </div>

        <button onclick={() => (show = false)} class="rounded-lg p-1.5 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Tab Content -->
      <div class="relative flex-1 overflow-hidden p-6 bg-background">
        {#if activeTab === "vpanel"}
          <div class="relative flex h-full flex-col items-center justify-center rounded-lg border border-border bg-zinc-950/80 overflow-hidden">
            <!-- 3D Controls -->
            <div class="absolute top-3 right-3 z-10 flex items-center gap-2 rounded-lg border border-border/80 bg-card/80 p-1.5 backdrop-blur-sm shadow-md">
              <button
                onclick={toggleRotate}
                class="flex items-center gap-1 rounded px-2.5 py-1 text-xs font-medium transition-colors {rotate ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
              >
                <RotateCw class="h-3.5 w-3.5" />
                自動回転
              </button>
              <button onclick={() => handleZoom(true)} class="rounded p-1 text-muted-foreground hover:bg-muted">
                <ZoomIn class="h-4 w-4" />
              </button>
              <button onclick={() => handleZoom(false)} class="rounded p-1 text-muted-foreground hover:bg-muted">
                <ZoomOut class="h-4 w-4" />
              </button>
            </div>

            <!-- Canvas Container for p5.js WebGL -->
            <div id="vpanel-canvas-container" class="h-full w-full"></div>
          </div>
        {:else if activeTab === "ports"}
          <div class="h-full overflow-y-auto">
            <table class="w-full text-left text-xs border-collapse">
              <thead class="sticky top-0 bg-muted text-muted-foreground border-b border-border">
                <tr>
                  <th class="p-3">ポート</th>
                  <th class="p-3">ステータス</th>
                  <th class="p-3">リンク速度</th>
                  <th class="p-3">In (受信)</th>
                  <th class="p-3">Out (送信)</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border">
                {#each ports as pt, i}
                  <tr class="hover:bg-muted/30">
                    <td class="p-3 font-semibold text-foreground">Port {i + 1}</td>
                    <td class="p-3">
                      <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-medium {pt.State === 'up' ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400' : 'bg-zinc-500/15 text-zinc-500'}">
                        <span class="h-1.5 w-1.5 rounded-full {pt.State === 'up' ? 'bg-emerald-500' : 'bg-zinc-500'}"></span>
                        {pt.State.toUpperCase()}
                      </span>
                    </td>
                    <td class="p-3 text-muted-foreground">{pt.Speed > 0 ? "1 Gbps" : "-"}</td>
                    <td class="p-3 text-muted-foreground">{pt.State === "up" ? `${(Math.random() * 20).toFixed(2)} MB` : "0"}</td>
                    <td class="p-3 text-muted-foreground">{pt.State === "up" ? `${(Math.random() * 15).toFixed(2)} MB` : "0"}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else if activeTab === "polling"}
          <div class="h-full overflow-y-auto">
            {#if pollings.filter((p) => p.node_id === node?.id).length === 0}
              <div class="flex h-full items-center justify-center text-xs text-muted-foreground">
                このノードに紐づくポーリングはありません
              </div>
            {:else}
              <div class="grid grid-cols-2 gap-3">
                {#each pollings.filter((p) => p.node_id === node?.id) as p}
                  <div class="rounded-lg border border-border p-3">
                    <div class="flex items-center justify-between">
                      <span class="font-semibold text-sm">{p.name}</span>
                      <span class="rounded px-2 py-0.5 text-[10px] font-medium uppercase bg-muted text-muted-foreground">{p.type}</span>
                    </div>
                    <div class="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                      <span>状態: {getStateName(p.state)}</span>
                      <span>応答値: {p.last_val ?? "-"}</span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else if activeTab === "logs"}
          <div class="h-full overflow-y-auto text-xs font-mono space-y-2">
            {#each logs.filter((l) => l.node_id === node?.id || l.node_name === node?.name) as l}
              <div class="rounded border border-border bg-muted/20 p-2">
                <div class="flex items-center justify-between text-muted-foreground text-[10px]">
                  <span>{new Date(l.time * 1000).toLocaleString()}</span>
                  <span class="font-semibold text-primary">{l.type}</span>
                </div>
                <div class="mt-1 text-foreground">{l.event}</div>
              </div>
            {/each}
            {#if logs.filter((l) => l.node_id === node?.id || l.node_name === node?.name).length === 0}
              <div class="flex h-full items-center justify-center text-muted-foreground font-sans text-xs">
                該当するイベントログはありません
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
