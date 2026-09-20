<script lang="ts">
  import type { NodeEnt, PollingEnt } from '$lib/api';
  import { saveNode, deleteNode } from '$lib/api';
  import { Server, Plus, Trash2, Edit2, CheckCircle, AlertTriangle, AlertCircle, HelpCircle } from '@lucide/svelte';

  let { nodes = [], pollings = [], onRefresh }: {
    nodes: NodeEnt[];
    pollings: PollingEnt[];
    onRefresh?: () => void;
  } = $props();

  let showModal = $state(false);
  let editId = $state<string | null>(null);
  let nodeName = $state('');
  let nodeIP = $state('');
  let nodeDescr = $state('');

  function openCreate() {
    editId = null;
    nodeName = '';
    nodeIP = '';
    nodeDescr = '';
    showModal = true;
  }

  function openEdit(n: NodeEnt) {
    editId = n.id;
    nodeName = n.name;
    nodeIP = n.ip;
    nodeDescr = n.descr || '';
    showModal = true;
  }

  async function handleSave() {
    if (!nodeName || !nodeIP) return;
    const id = editId || `node-${Date.now()}`;
    const n: NodeEnt = {
      id,
      name: nodeName,
      ip: nodeIP,
      descr: nodeDescr,
      state: 'normal',
      x: 200 + Math.floor(Math.random() * 300),
      y: 150 + Math.floor(Math.random() * 200),
    };
    await saveNode(n);
    showModal = false;
    if (onRefresh) onRefresh();
  }

  async function handleDelete(id: string) {
    if (!confirm('Are you sure you want to delete this node?')) return;
    await deleteNode(id);
    if (onRefresh) onRefresh();
  }

  function getPollingsCount(nodeId: string): number {
    return pollings.filter((p) => p.node_id === nodeId).length;
  }
</script>

<div class="bg-slate-800/80 border border-slate-700/70 rounded-xl p-5 shadow-sm space-y-4">
  <div class="flex items-center justify-between">
    <div class="flex items-center space-x-2">
      <Server class="w-5 h-5 text-blue-400" />
      <h3 class="text-base font-semibold text-slate-100">Managed Network Nodes</h3>
      <span class="text-xs bg-slate-700 text-slate-300 px-2 py-0.5 rounded-full font-mono">
        {nodes.length}
      </span>
    </div>

    <button
      onclick={openCreate}
      class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition-colors shadow-sm"
    >
      <Plus class="w-4 h-4" />
      Add Node
    </button>
  </div>

  <div class="overflow-x-auto rounded-lg border border-slate-700/60">
    <table class="w-full text-left text-sm text-slate-300">
      <thead class="bg-slate-900/90 text-xs uppercase text-slate-400">
        <tr>
          <th class="px-4 py-3">Status</th>
          <th class="px-4 py-3">Node Name</th>
          <th class="px-4 py-3">IP Address</th>
          <th class="px-4 py-3">Pollings</th>
          <th class="px-4 py-3">Description</th>
          <th class="px-4 py-3 text-right">Actions</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-700/50 bg-slate-800/40">
        {#if nodes.length === 0}
          <tr>
            <td colspan="6" class="px-4 py-8 text-center text-slate-500">
              No nodes configured. Click "Add Node" to register devices.
            </td>
          </tr>
        {:else}
          {#each nodes as node}
            <tr class="hover:bg-slate-700/30 transition-colors">
              <td class="px-4 py-2.5 whitespace-nowrap">
                {#if node.state === 'normal'}
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-emerald-400">
                    <CheckCircle class="w-3.5 h-3.5" /> Normal
                  </span>
                {:else if node.state === 'warn'}
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-amber-400">
                    <AlertTriangle class="w-3.5 h-3.5" /> Warning
                  </span>
                {:else if node.state === 'error'}
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-red-400">
                    <AlertCircle class="w-3.5 h-3.5" /> Error
                  </span>
                {:else}
                  <span class="inline-flex items-center gap-1 text-xs font-medium text-slate-400">
                    <HelpCircle class="w-3.5 h-3.5" /> Unknown
                  </span>
                {/if}
              </td>
              <td class="px-4 py-2.5 font-medium text-slate-100 whitespace-nowrap">
                {node.name}
              </td>
              <td class="px-4 py-2.5 font-mono text-xs text-slate-300 whitespace-nowrap">
                {node.ip}
              </td>
              <td class="px-4 py-2.5 text-xs text-slate-400 whitespace-nowrap">
                <span class="px-2 py-0.5 bg-slate-700/80 rounded-md font-mono">
                  {getPollingsCount(node.id)} polls
                </span>
              </td>
              <td class="px-4 py-2.5 text-xs text-slate-400 truncate max-w-xs">
                {node.descr || '-'}
              </td>
              <td class="px-4 py-2.5 text-right whitespace-nowrap space-x-2">
                <button
                  onclick={() => openEdit(node)}
                  class="p-1 text-slate-400 hover:text-blue-400 transition-colors"
                  title="Edit Node"
                >
                  <Edit2 class="w-4 h-4" />
                </button>
                <button
                  onclick={() => handleDelete(node.id)}
                  class="p-1 text-slate-400 hover:text-red-400 transition-colors"
                  title="Delete Node"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>
</div>

<!-- Modal Dialog -->
{#if showModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="bg-slate-800 border border-slate-700 rounded-xl shadow-2xl max-w-md w-full p-6 space-y-4">
      <h3 class="text-lg font-bold text-white">
        {editId ? 'Edit Node' : 'Register New Node'}
      </h3>

      <div class="space-y-3 text-sm">
        <div>
          <label for="node-name-input" class="block text-slate-300 text-xs font-semibold mb-1">Node Name</label>
          <input
            id="node-name-input"
            type="text"
            bind:value={nodeName}
            placeholder="e.g. Core Switch 01"
            class="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label for="node-ip-input" class="block text-slate-300 text-xs font-semibold mb-1">IP Address / Hostname</label>
          <input
            id="node-ip-input"
            type="text"
            bind:value={nodeIP}
            placeholder="e.g. 192.168.1.1"
            class="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label for="node-descr-input" class="block text-slate-300 text-xs font-semibold mb-1">Description</label>
          <input
            id="node-descr-input"
            type="text"
            bind:value={nodeDescr}
            placeholder="Optional notes or location"
            class="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-2 border-t border-slate-700">
        <button
          onclick={() => showModal = false}
          class="px-4 py-1.5 bg-slate-700 hover:bg-slate-600 text-sm font-medium rounded-lg text-white transition-colors"
        >
          Cancel
        </button>
        <button
          onclick={handleSave}
          class="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-sm font-medium rounded-lg text-white transition-colors"
        >
          Save Node
        </button>
      </div>
    </div>
  </div>
{/if}
