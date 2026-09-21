<script lang="ts">
  import { deleteLine, type LineEnt, type NodeEnt, type NetworkEnt, type PollingEnt } from "../api";
  import { getStateColor } from "../common";
  import { X, Network, Edit3, Trash2, Link, Server, Laptop, Activity } from "@lucide/svelte";

  let {
    show = $bindable(false),
    network = $bindable<NetworkEnt | null>(null),
    lines = [],
    nodes = [],
    networks = [],
    pollings = [],
    onEditLine = () => {},
    onDeleteLine = () => {},
  } = $props<{
    show: boolean;
    network: NetworkEnt | null;
    lines?: LineEnt[];
    nodes?: NodeEnt[];
    networks?: NetworkEnt[];
    pollings?: PollingEnt[];
    onEditLine?: (line: LineEnt) => void;
    onDeleteLine?: (id: string) => void;
  }>();

  let netId = $derived(network ? (network.id || (network as any).ID || "") : "");
  let netPrefix = $derived("NET:" + netId);

  // Filter lines connected to this network
  let hubLines = $derived.by(() => {
    if (!netId) return [];
    return lines.filter((l) => {
      const n1 = l.node_id1 || (l as any).NodeID1 || "";
      const n2 = l.node_id2 || (l as any).NodeID2 || "";
      return n1 === netPrefix || n2 === netPrefix;
    });
  });

  const getTargetInfo = (l: LineEnt) => {
    const n1 = l.node_id1 || (l as any).NodeID1 || "";
    const n2 = l.node_id2 || (l as any).NodeID2 || "";
    const isFirst = n1 === netPrefix;
    const localPortId = isFirst ? (l.polling_id1 || (l as any).PollingID1 || "") : (l.polling_id2 || (l as any).PollingID2 || "");
    const remoteId = isFirst ? n2 : n1;
    const remotePollingId = isFirst ? (l.polling_id2 || (l as any).PollingID2 || "") : (l.polling_id1 || (l as any).PollingID1 || "");

    // Local port name
    let localPortName = "自動 / 未指定";
    if (network && network.ports) {
      const p = network.ports.find((port) => (port.id || (port as any).ID) === localPortId);
      if (p) localPortName = p.name || (p as any).Name || localPortId;
    }

    // Remote target
    let remoteName = remoteId;
    let remoteType = "node";
    if (remoteId.startsWith("NET:")) {
      remoteType = "net";
      const rn = networks.find((net) => (net.id || (net as any).ID) === remoteId.replace("NET:", ""));
      if (rn) remoteName = rn.name || (rn as any).Name || remoteId;
    } else {
      const rn = nodes.find((node) => (node.id || (node as any).ID) === remoteId);
      if (rn) remoteName = `${rn.name || (rn as any).Name || remoteId} (${rn.ip || (rn as any).IP || ""})`;
    }

    // Remote port/polling
    let remotePollingName = "-";
    if (remotePollingId) {
      const poll = pollings.find((p) => (p.id || (p as any).ID) === remotePollingId);
      if (poll) remotePollingName = poll.name || (poll as any).Name || remotePollingId;
      else remotePollingName = remotePollingId;
    }

    return {
      localPortName,
      remoteName,
      remoteType,
      remotePollingName,
    };
  };

  const handleEdit = (line: LineEnt) => {
    show = false;
    onEditLine(line);
  };

  const handleDelete = async (line: LineEnt) => {
    const lineId = line.id || (line as any).ID;
    if (!lineId) return;
    if (confirm("このライン（結線）を切断・削除しますか？")) {
      try {
        await deleteLine(lineId);
        onDeleteLine(lineId);
      } catch (e: any) {
        alert("削除エラー: " + (e.message || e));
      }
    }
  };
</script>

{#if show && network}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-auto max-h-[85vh] w-full max-w-3xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Network class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">
              {network.name || (network as any).Name || "ネットワーク"} - 接続ライン編集
            </h2>
            <p class="text-[11px] text-slate-400">ネットワーク各ポートに接続されているラインの一覧と管理</p>
          </div>
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

      <!-- Table Content -->
      <div class="flex-1 overflow-y-auto p-6 bg-slate-900/30 text-xs">
        {#if hubLines.length === 0}
          <div class="flex flex-col items-center justify-center py-12 text-slate-500">
            <Link class="h-10 w-10 mb-3 opacity-30" />
            <p class="text-sm font-semibold">接続されているラインはありません</p>
            <p class="text-xs text-slate-500 mt-1">Shiftキーを押しながらノードとネットワークを選択して接続するか、接続先探索を実行してください</p>
          </div>
        {:else}
          <div class="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/60 shadow-lg">
            <table class="w-full text-left text-xs text-slate-300">
              <thead class="bg-slate-950/80 uppercase font-mono text-[11px] text-slate-400 border-b border-slate-800">
                <tr>
                  <th class="py-2.5 px-3">自ポート</th>
                  <th class="py-2.5 px-3">接続先機器</th>
                  <th class="py-2.5 px-3">相手側ポート / ポーリング</th>
                  <th class="py-2.5 px-3">状態</th>
                  <th class="py-2.5 px-3 text-right">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800/60">
                {#each hubLines as l}
                  {@const info = getTargetInfo(l)}
                  <tr class="hover:bg-slate-800/40 transition-colors">
                    <td class="py-2 px-3 font-semibold text-cyan-300">
                      {info.localPortName}
                    </td>
                    <td class="py-2 px-3 font-medium text-slate-100">
                      <div class="flex items-center gap-2">
                        {#if info.remoteType === 'net'}
                          <Server class="w-3.5 h-3.5 text-cyan-400 shrink-0" />
                        {:else}
                          <Laptop class="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                        {/if}
                        <span class="truncate">{info.remoteName}</span>
                      </div>
                    </td>
                    <td class="py-2 px-3 text-slate-400 truncate">
                      {info.remotePollingName}
                    </td>
                    <td class="py-2 px-3">
                      <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase border border-slate-700/60 bg-slate-800/60">
                        <span class="h-1.5 w-1.5 rounded-full" style="background-color: {getStateColor(l.state || (l as any).State || 'normal')}"></span>
                        {l.state || (l as any).State || 'normal'}
                      </span>
                    </td>
                    <td class="py-2 px-3 text-right">
                      <div class="flex items-center justify-end gap-1.5">
                        <button
                          type="button"
                          onclick={() => handleEdit(l)}
                          class="flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-800 px-2.5 py-1 text-[11px] font-semibold text-slate-200 hover:bg-slate-700 hover:border-slate-600 transition-all cursor-pointer"
                        >
                          <Edit3 class="h-3 w-3 text-cyan-400" />
                          編集
                        </button>
                        <button
                          type="button"
                          onclick={() => handleDelete(l)}
                          class="flex items-center gap-1 rounded-lg border border-rose-800/40 bg-rose-950/30 px-2.5 py-1 text-[11px] font-semibold text-rose-400 hover:bg-rose-900/40 transition-all cursor-pointer"
                        >
                          <Trash2 class="h-3 w-3" />
                          切断
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-end border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-5 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-xl text-xs font-semibold text-slate-200 transition-colors cursor-pointer"
        >
          閉じる
        </button>
      </div>
    </div>
  </div>
{/if}
