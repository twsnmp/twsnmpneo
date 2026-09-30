<script lang="ts">
  import { _ } from "svelte-i18n";
  import { X, Network, Link, Router } from "@lucide/svelte";
  import type { LineEnt, NetworkEnt, NodeEnt, PortEnt } from "../api";

  let {
    show = $bindable(false),
    network = null,
    lines = [],
    nodes = [],
  } = $props<{
    show: boolean;
    network: NetworkEnt | null;
    lines?: LineEnt[];
    nodes?: NodeEnt[];
  }>();

  const networkId = $derived(network?.id || network?.ID || "");
  const networkPrefix = $derived(`NET:${networkId}`);
  const connectedLines = $derived(
    lines.filter((line: LineEnt) => (line.node_id1 || line.NodeID1) === networkPrefix || (line.node_id2 || line.NodeID2) === networkPrefix)
  );

  const getRemoteName = (line: LineEnt) => {
    const id1 = line.node_id1 || line.NodeID1 || "";
    const remoteId = id1 === networkPrefix ? (line.node_id2 || line.NodeID2 || "") : id1;
    if (remoteId.startsWith("NET:")) return remoteId.slice(4);
    const node = nodes.find((entry: NodeEnt) => (entry.id || entry.ID) === remoteId);
    return node ? `${node.name || node.Name} (${node.ip || node.IP})` : remoteId;
  };

  const ports = $derived(network?.ports || (network as any)?.Ports || []);

  const getPortName = (line: LineEnt) => {
    if (!network) return "";
    const localIsFirst = (line.node_id1 || line.NodeID1) === networkPrefix;
    const portId = localIsFirst ? (line.polling_id1 || line.PollingID1) : (line.polling_id2 || line.PollingID2);
    const port = ports.find((entry: PortEnt) => (entry.id || entry.ID) === portId);
    return port?.name || port?.Name || portId || "—";
  };
</script>

{#if show && network}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_("map.context.report")}
    tabindex="-1"
    onkeydown={(event) => event.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-4 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <Network class="h-5 w-5 text-cyan-500" />
          <div>
            <h2 class="text-sm font-bold">{$_("map.context.report")}: {network.name || network.Name}</h2>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">{network.ip || network.IP}</p>
          </div>
        </div>
        <button type="button" aria-label={$_("common.close")} onclick={() => (show = false)} class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800">
          <X class="h-4 w-4" />
        </button>
      </header>

      <div class="min-h-0 flex-1 space-y-5 overflow-y-auto p-5">
        <section class="rounded-xl border border-slate-200 dark:border-slate-800">
          <h3 class="flex items-center gap-2 border-b border-slate-200 px-4 py-3 text-xs font-semibold dark:border-slate-800">
            <Router class="h-4 w-4 text-cyan-500" />{$_("map.tools.overview")}
          </h3>
          <dl class="grid grid-cols-1 divide-y divide-slate-200 text-xs dark:divide-slate-800 md:grid-cols-2 md:divide-y-0">
            <div class="flex justify-between gap-4 border-b border-slate-200 px-4 py-3 dark:border-slate-800"><dt class="text-slate-500">{$_("map.tools.name")}</dt><dd class="font-medium">{network.name || network.Name}</dd></div>
            <div class="flex justify-between gap-4 border-b border-slate-200 px-4 py-3 dark:border-slate-800"><dt class="text-slate-500">IP</dt><dd class="font-mono">{network.ip || network.IP}</dd></div>
            <div class="flex justify-between gap-4 border-b border-slate-200 px-4 py-3 dark:border-slate-800"><dt class="text-slate-500">{$_("map.tools.portCount")}</dt><dd>{ports.length}</dd></div>
            <div class="flex justify-between gap-4 border-b border-slate-200 px-4 py-3 dark:border-slate-800"><dt class="text-slate-500">{$_("map.tools.connectedLines")}</dt><dd>{connectedLines.length}</dd></div>
            <div class="flex justify-between gap-4 px-4 py-3 md:col-span-2"><dt class="text-slate-500">{$_("map.tools.description")}</dt><dd class="text-right">{network.descr || network.Descr || "—"}</dd></div>
          </dl>
          {#if network.error || network.Error}
            <p class="border-t border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">{network.error || network.Error}</p>
          {/if}
        </section>

        <section class="rounded-xl border border-slate-200 dark:border-slate-800">
          <h3 class="flex items-center gap-2 border-b border-slate-200 px-4 py-3 text-xs font-semibold dark:border-slate-800">
            <Network class="h-4 w-4 text-emerald-500" />{$_("map.tools.ports")}
          </h3>
          <div class="max-h-64 overflow-auto">
            <table class="w-full text-left text-xs">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900"><tr><th class="p-2">{$_("map.tools.port")}</th><th class="p-2">{$_("map.tools.state")}</th><th class="p-2">{$_("map.tools.index")}</th></tr></thead>
              <tbody>
                {#each ports as port}
                  <tr class="border-t border-slate-200 dark:border-slate-800"><td class="p-2">{port.name || port.Name}</td><td class="p-2">{port.state || port.State}</td><td class="p-2 font-mono">{port.index || port.Index || "—"}</td></tr>
                {:else}
                  <tr><td colspan="3" class="p-4 text-center text-slate-500">{$_("map.tools.noPorts")}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>

        <section class="rounded-xl border border-slate-200 dark:border-slate-800">
          <h3 class="flex items-center gap-2 border-b border-slate-200 px-4 py-3 text-xs font-semibold dark:border-slate-800">
            <Link class="h-4 w-4 text-indigo-500" />{$_("map.tools.connections")}
          </h3>
          <div class="max-h-64 overflow-auto">
            <table class="w-full text-left text-xs">
              <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900"><tr><th class="p-2">{$_("map.tools.port")}</th><th class="p-2">{$_("map.tools.remoteNode")}</th><th class="p-2">{$_("map.tools.state")}</th></tr></thead>
              <tbody>
                {#each connectedLines as line}
                  <tr class="border-t border-slate-200 dark:border-slate-800"><td class="p-2">{getPortName(line)}</td><td class="p-2">{getRemoteName(line)}</td><td class="p-2">{line.state || line.State}</td></tr>
                {:else}
                  <tr><td colspan="3" class="p-4 text-center text-slate-500">{$_("map.tools.noConnections")}</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </div>
  </div>
{/if}
