<script lang="ts">
  import { untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import { savePolling, type PollingEnt, type NodeEnt } from "../api";
  import { X, Save, CheckSquare } from "@lucide/svelte";

  let {
    show = $bindable(false),
    polling = $bindable<PollingEnt | null>(null),
    nodes = [],
    onSave = () => {}
  } = $props<{
    show: boolean;
    polling: PollingEnt | null;
    nodes?: NodeEnt[];
    onSave?: (saved: PollingEnt) => void;
  }>();

  let id = $state("");
  let name = $state("");
  let nodeId = $state("");
  let type = $state("ping");
  let target = $state("");
  let state = $state("normal");
  let saveError = $state("");
  let isSubmitting = $state(false);

  const pollingTypes = [
    { value: "ping", label: "PING (ICMP Echo)" },
    { value: "snmp", label: "SNMP (sysUpTime / OID)" },
    { value: "http", label: "HTTP / HTTPS (Web)" },
    { value: "tcp", label: "TCP Port (Socket)" },
    { value: "dns", label: "DNS Lookup" },
    { value: "ntp", label: "NTP Time Sync" },
  ];

  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        if (polling) {
          id = polling.id || polling.ID || "";
          name = polling.name || polling.Name || "";
          nodeId = polling.node_id || polling.NodeID || (nodes.length > 0 ? (nodes[0].id || nodes[0].ID || "") : "");
          type = polling.type || polling.Type || "ping";
          target = polling.target || (polling as any).Target || "";
          state = polling.state || polling.State || "normal";
        } else {
          id = "";
          name = $_('polling.defaultName');
          nodeId = nodes.length > 0 ? (nodes[0].id || nodes[0].ID || "") : "";
          type = "ping";
          target = "";
          state = "normal";
        }
      });
    }
  });

  // When node changes and target is empty, fill target with node's IP
  const handleNodeChange = (selectedId: string) => {
    nodeId = selectedId;
    const n = nodes.find((item) => (item.id || item.ID) === selectedId);
    if (n && (!target || target === "")) {
      target = n.ip || n.IP || "";
    }
  };

  const handleSave = async (e: Event) => {
    e.preventDefault();
    if (!name.trim()) {
      saveError = $_('polling.errNameRequired');
      return;
    }
    if (!nodeId) {
      saveError = $_('polling.errNodeRequired');
      return;
    }

    isSubmitting = true;
    saveError = "";
    try {
      const saved = await savePolling({
        id,
        name,
        node_id: nodeId,
        type,
        target,
        state,
      });
      onSave(saved);
      show = false;
    } catch (err: any) {
      saveError = err?.message || $_('polling.errSaveFailed');
    } finally {
      isSubmitting = false;
    }
  };
</script>

{#if show}
  <div
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div
      class="w-full max-w-lg rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-2xl overflow-hidden flex flex-col max-h-[90vh] text-slate-800 dark:text-slate-100 font-sans"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 px-5 py-3.5 bg-slate-50 dark:bg-slate-950/80">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-500 dark:text-cyan-400">
            <CheckSquare class="h-4 w-4" />
          </div>
          <div>
            <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
              {id ? $_('polling.editTitle') : $_('polling.createTitle')}
            </h3>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">{$_('polling.subtitle')}</p>
          </div>
        </div>
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-700 dark:hover:text-slate-200 transition-colors"
          title={$_('common.close')}
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <!-- Form Body -->
      <form onsubmit={handleSave} class="flex-1 overflow-y-auto p-5 space-y-4 text-xs">
        {#if saveError}
          <div class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-rose-600 dark:text-rose-300 font-medium">
            {saveError}
          </div>
        {/if}

        <!-- Name -->
        <div class="space-y-1.5">
          <label for="poll-name" class="block font-semibold text-slate-700 dark:text-slate-300">
            {$_('polling.name')} <span class="text-rose-500 dark:text-rose-400">*</span>
          </label>
          <input
            id="poll-name"
            type="text"
            bind:value={name}
            placeholder={$_('polling.namePlaceholder')}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none"
            required
          />
        </div>

        <!-- Target Node -->
        <div class="space-y-1.5">
          <label for="poll-node" class="block font-semibold text-slate-700 dark:text-slate-300">
            {$_('polling.targetNode')} <span class="text-rose-500 dark:text-rose-400">*</span>
          </label>
          <select
            id="poll-node"
            value={nodeId}
            onchange={(e) => handleNodeChange((e.target as HTMLSelectElement).value)}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
          >
            {#if nodes.length === 0}
              <option value="">{$_('polling.noNodes')}</option>
            {/if}
            {#each nodes as n}
              <option value={n.id || n.ID}>
                {n.name || n.Name} ({n.ip || n.IP})
              </option>
            {/each}
          </select>
        </div>

        <!-- Polling Type -->
        <div class="space-y-1.5">
          <label for="poll-type" class="block font-semibold text-slate-700 dark:text-slate-300">
            {$_('polling.type')}
          </label>
          <div class="grid grid-cols-2 gap-2">
            {#each pollingTypes as pt}
              <button
                type="button"
                onclick={() => (type = pt.value)}
                class="flex items-center gap-2 rounded-xl border px-3 py-2 text-left font-medium transition-all {type === pt.value ? 'border-cyan-500 bg-cyan-500/15 text-cyan-600 dark:text-cyan-300 font-bold' : 'border-slate-200 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-950/60 text-slate-600 dark:text-slate-400 hover:border-slate-300 dark:hover:border-slate-700 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                <span class="h-2 w-2 rounded-full {type === pt.value ? 'bg-cyan-500 dark:bg-cyan-400' : 'bg-slate-400 dark:bg-slate-600'}"></span>
                <span class="truncate">{pt.label}</span>
              </button>
            {/each}
          </div>
        </div>

        <!-- Target Destination (IP / Host / URL / Port / OID) -->
        <div class="space-y-1.5">
          <label for="poll-target" class="block font-semibold text-slate-700 dark:text-slate-300">
            {$_('polling.target')}
          </label>
          <input
            id="poll-target"
            type="text"
            bind:value={target}
            placeholder={$_('polling.targetPlaceholder')}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-mono"
          />
          <p class="text-[11px] text-slate-500 dark:text-slate-400">
            {$_('polling.targetHelp')}
          </p>
        </div>

        <!-- Initial Status (if editing) -->
        <div class="space-y-1.5">
          <label for="poll-state" class="block font-semibold text-slate-700 dark:text-slate-300">
            {$_('polling.status')}
          </label>
          <select
            id="poll-state"
            bind:value={state}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
          >
            <option value="normal">{$_('status.normal')}</option>
            <option value="warn">{$_('status.warn')}</option>
            <option value="low">{$_('status.low')}</option>
            <option value="high">{$_('status.high')}</option>
            <option value="error">{$_('common.error')}</option>
          </select>
        </div>

        <!-- Footer Actions -->
        <div class="flex items-center justify-end gap-2.5 pt-4 border-t border-slate-200 dark:border-slate-800">
          <button
            type="button"
            onclick={() => (show = false)}
            class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-4 py-2 font-semibold text-slate-700 dark:text-slate-300 transition-colors"
          >
            {$_('common.cancel')}
          </button>
          <button
            type="submit"
            disabled={isSubmitting}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-cyan-500 hover:from-cyan-500 hover:to-cyan-400 px-5 py-2 font-bold text-white shadow-md shadow-cyan-600/30 transition-all disabled:opacity-50"
          >
            <Save class="h-3.5 w-3.5" />
            <span>{isSubmitting ? $_('common.saving') : $_('common.save')}</span>
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
