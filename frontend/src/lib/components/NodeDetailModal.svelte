<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { initVPanel, setVPanel } from "../map/vpanel";
  import { getStateColor, getStateName } from "../common";
  import type { NodeEnt, PollingEnt, EventLogEnt } from "../api";
  import { X, Box, ListTree, Activity, FileText, CheckCircle2, RotateCw, ZoomIn, ZoomOut, Cpu } from "@lucide/svelte";

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
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-[88vh] w-full max-w-5xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Cpu class="h-5 w-5" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold text-slate-100">{node.name}</h2>
              <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold border" style="background-color: {getStateColor(node.state)}20; border-color: {getStateColor(node.state)}50; color: {getStateColor(node.state)}">
                <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(node.state)}"></span>
                {getStateName(node.state)}
              </span>
            </div>
            <p class="text-[11px] font-mono text-cyan-400">{node.ip} {node.mac ? `(${node.mac})` : ""}</p>
          </div>
        </div>

        <!-- Tab Buttons (twnoaa style) -->
        <div class="flex rounded-xl border border-slate-800 bg-slate-950 p-1 text-xs">
          <button
            type="button"
            onclick={() => (activeTab = "vpanel")}
            class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 font-semibold transition-all cursor-pointer {activeTab === 'vpanel' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
          >
            <Box class="h-3.5 w-3.5" />
            3D パネル
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "ports")}
            class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 font-semibold transition-all cursor-pointer {activeTab === 'ports' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
          >
            <ListTree class="h-3.5 w-3.5" />
            ポート一覧
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "polling")}
            class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 font-semibold transition-all cursor-pointer {activeTab === 'polling' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
          >
            <Activity class="h-3.5 w-3.5" />
            ポーリング
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "logs")}
            class="flex items-center gap-1.5 rounded-lg px-3 py-1.5 font-semibold transition-all cursor-pointer {activeTab === 'logs' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-400 hover:text-slate-200'}"
          >
            <FileText class="h-3.5 w-3.5" />
            個別ログ
          </button>
        </div>

        <button
          type="button"
          aria-label="閉じる"
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Tab Content -->
      <div class="relative flex-1 overflow-hidden p-6 bg-slate-900/40 text-xs">
        {#if activeTab === "vpanel"}
          <div class="relative flex h-full flex-col items-center justify-center rounded-2xl border border-slate-800 bg-[#080d1e] overflow-hidden shadow-inner">
            <!-- 3D Controls -->
            <div class="absolute top-3 right-3 z-10 flex items-center gap-2 rounded-xl border border-slate-800 bg-slate-900/90 p-1.5 backdrop-blur-md shadow-lg">
              <button
                type="button"
                onclick={toggleRotate}
                class="flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-semibold transition-all cursor-pointer {rotate ? 'bg-cyan-600 text-white shadow-sm shadow-cyan-600/30' : 'bg-slate-950 text-slate-400 hover:text-slate-200'}"
              >
                <RotateCw class="h-3.5 w-3.5" />
                自動回転
              </button>
              <button
                type="button"
                aria-label="拡大"
                onclick={() => handleZoom(true)}
                class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 cursor-pointer"
              >
                <ZoomIn class="h-4 w-4" />
              </button>
              <button
                type="button"
                aria-label="縮小"
                onclick={() => handleZoom(false)}
                class="rounded-lg p-1 text-slate-400 hover:bg-slate-800 hover:text-slate-200 cursor-pointer"
              >
                <ZoomOut class="h-4 w-4" />
              </button>
            </div>

            <!-- Canvas Container for p5.js WebGL -->
            <div id="vpanel-canvas-container" class="h-full w-full"></div>
          </div>
        {:else if activeTab === "ports"}
          <div class="h-full overflow-y-auto rounded-2xl border border-slate-800 bg-slate-900/80 shadow-lg">
            <table class="w-full text-left text-xs border-collapse font-mono">
              <thead class="sticky top-0 bg-slate-950 text-slate-400 uppercase text-[10px] font-semibold border-b border-slate-800">
                <tr>
                  <th class="p-3">ポート</th>
                  <th class="p-3">ステータス</th>
                  <th class="p-3">リンク速度</th>
                  <th class="p-3">In (受信)</th>
                  <th class="p-3">Out (送信)</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60">
                {#each ports as pt, i}
                  <tr class="hover:bg-slate-800/40 transition-colors">
                    <td class="p-3 font-semibold text-slate-100">Port {i + 1}</td>
                    <td class="p-3">
                      <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 font-semibold text-[11px] {pt.State === 'up' ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-slate-800 text-slate-500 border border-slate-700'}">
                        <span class="h-1.5 w-1.5 rounded-full {pt.State === 'up' ? 'bg-emerald-400' : 'bg-slate-500'}"></span>
                        {pt.State.toUpperCase()}
                      </span>
                    </td>
                    <td class="p-3 text-cyan-400">{pt.Speed > 0 ? "1 Gbps" : "-"}</td>
                    <td class="p-3 text-slate-300">{pt.State === "up" ? `${(Math.random() * 20).toFixed(2)} MB` : "0"}</td>
                    <td class="p-3 text-slate-300">{pt.State === "up" ? `${(Math.random() * 15).toFixed(2)} MB` : "0"}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {:else if activeTab === "polling"}
          <div class="h-full overflow-y-auto">
            {#if pollings.filter((p) => p.node_id === node?.id).length === 0}
              <div class="flex h-full items-center justify-center text-xs text-slate-500">
                このノードに紐づくポーリングはありません
              </div>
            {:else}
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3.5">
                {#each pollings.filter((p) => p.node_id === node?.id) as p}
                  <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-4 shadow-lg space-y-2">
                    <div class="flex items-center justify-between">
                      <span class="font-bold text-sm text-slate-100">{p.name}</span>
                      <span class="rounded-lg px-2.5 py-0.5 text-[10px] font-semibold uppercase bg-cyan-500/10 text-cyan-300 border border-cyan-500/30">{p.type}</span>
                    </div>
                    <div class="flex items-center justify-between text-xs text-slate-400 pt-1 border-t border-slate-800/80 font-mono">
                      <span>状態: <span class="font-semibold text-emerald-400">{getStateName(p.state)}</span></span>
                      <span>応答値: <span class="text-cyan-400">{p.last_val ?? "-"}</span></span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {:else if activeTab === "logs"}
          <div class="h-full overflow-y-auto font-mono text-xs space-y-2">
            {#each logs.filter((l) => l.node_id === node?.id || l.node_name === node?.name) as l}
              <div class="rounded-xl border border-slate-800 bg-slate-900/80 p-3 shadow-md space-y-1">
                <div class="flex items-center justify-between text-[11px] text-slate-400">
                  <span class="text-cyan-400">{new Date(l.time * 1000).toLocaleString()}</span>
                  <span class="font-semibold uppercase text-slate-300 bg-slate-950 px-2 py-0.5 rounded border border-slate-800">{l.type}</span>
                </div>
                <div class="text-slate-100 font-sans text-xs">{l.event}</div>
              </div>
            {/each}
            {#if logs.filter((l) => l.node_id === node?.id || l.node_name === node?.name).length === 0}
              <div class="flex h-full items-center justify-center text-slate-500 font-sans text-xs">
                該当するイベントログはありません
              </div>
            {/if}
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
