<script lang="ts">
  import { untrack } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    fetchGNMICapabilities,
    runGNMIGet,
    type GNMICapabilitiesEnt,
    type GNMIValueEnt,
    type NodeEnt,
    type PollingEnt,
  } from "../api";
  import {
    X,
    Radio,
    Play,
    Copy,
    Check,
    RefreshCw,
    Plus,
    FileSpreadsheet,
    Layers,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    node = null,
    onAddPolling = undefined,
  } = $props<{
    show: boolean;
    node: NodeEnt | null;
    onAddPolling?: (p: Partial<PollingEnt>) => void;
  }>();

  let path = $state("/interfaces");
  let target = $state("");
  let encoding = $state("json_ietf");
  let history = $state<string[]>(["/interfaces", "/system", "/components"]);

  let isLoading = $state(false);
  let errorMessage = $state("");
  let capabilities = $state<GNMICapabilitiesEnt | null>(null);
  let results = $state<GNMIValueEnt[]>([]);
  let selectedIndices = $state<number[]>([]);
  let copied = $state(false);
  let showCapDetails = $state(false);

  const nodeId = $derived(node?.id || (node as any)?.ID || "");
  const targetName = $derived(node?.name || (node as any)?.Name || "");

  $effect(() => {
    if (show && node) {
      untrack(() => {
        errorMessage = "";
        results = [];
        selectedIndices = [];
        const port = node?.gnmi_port || (node as any)?.GNMIPort || "57400";
        target = `${node?.ip || (node as any)?.IP || ""}:${port}`;
        encoding = node?.gnmi_encoding || (node as any)?.GNMIEncoding || "json_ietf";
      });
      void loadCapabilities();
    }
  });

  const loadCapabilities = async () => {
    if (!nodeId) return;
    isLoading = true;
    errorMessage = "";
    try {
      capabilities = await fetchGNMICapabilities(nodeId);
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isLoading = false;
    }
  };

  const runGet = async () => {
    if (!nodeId || !path.trim()) return;
    isLoading = true;
    errorMessage = "";
    selectedIndices = [];
    try {
      const q = path.trim();
      if (!history.includes(q)) {
        history = [q, ...history.slice(0, 9)];
      }
      results = await runGNMIGet(nodeId, q, encoding);
    } catch (e) {
      errorMessage = e instanceof Error ? e.message : String(e);
    } finally {
      isLoading = false;
    }
  };

  const toggleSelect = (idx: number) => {
    if (selectedIndices.includes(idx)) {
      selectedIndices = selectedIndices.filter((i) => i !== idx);
    } else {
      selectedIndices = [...selectedIndices, idx];
    }
  };

  const copySelected = async () => {
    const list = selectedIndices.length > 0 ? selectedIndices.map((i) => results[i]) : results;
    if (list.length === 0) return;
    const lines = ["Path\tIndex\tValue"];
    list.forEach((r) => {
      lines.push(`${r.Path}\t${r.Index || ""}\t${r.Value}`);
    });
    await navigator.clipboard.writeText(lines.join("\n"));
    copied = true;
    setTimeout(() => (copied = false), 2000);
  };

  const exportCSV = () => {
    if (results.length === 0) return;
    const lines = ["Path,Index,Value"];
    results.forEach((r) => {
      const v = `"${r.Value.replace(/"/g, '""')}"`;
      lines.push(`"${r.Path}","${r.Index || ""}",${v}`);
    });
    const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `gnmi_${targetName}_${Date.now()}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const handleCreatePolling = () => {
    if (selectedIndices.length !== 1 || !onAddPolling) return;
    const r = results[selectedIndices[0]];
    if (!r) return;
    onAddPolling({
      node_id: nodeId,
      name: `gNMI ${r.Path}`,
      type: "gnmi",
      params: r.Path,
      state: "unknown",
    });
    show = false;
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-[60] flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="gNMIツール"
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (show = false)}
  >
    <div class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-3">
          <Radio class="h-5 w-5 text-cyan-500" />
          <div>
            <h2 class="text-sm font-bold">gNMIツール — {targetName}</h2>
            <p class="text-[11px] font-mono text-slate-500 dark:text-slate-400">{target || "No Target"}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <!-- Controls Toolbar -->
      <div class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/40">
        <div class="flex flex-wrap items-center gap-2">
          <input
            type="text"
            bind:value={path}
            placeholder="gNMI パス (例: /interfaces, /system)"
            class="min-w-[220px] flex-1 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-mono text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
          />

          <select
            onchange={(e) => { const v = (e.target as HTMLSelectElement).value; if (v) path = v; }}
            class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950"
          >
            <option value="">履歴</option>
            {#each history as h}
              <option value={h}>{h}</option>
            {/each}
          </select>

          <select bind:value={encoding} class="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs dark:border-slate-700 dark:bg-slate-950">
            <option value="json_ietf">JSON_IETF</option>
            <option value="json">JSON</option>
            <option value="proto">PROTO</option>
          </select>

          <button
            type="button"
            disabled={isLoading || !path.trim()}
            onclick={runGet}
            class="flex items-center gap-1.5 rounded-lg bg-cyan-600 px-4 py-1.5 text-xs font-semibold text-white shadow hover:bg-cyan-500 disabled:opacity-50"
          >
            {#if isLoading}
              <RefreshCw class="h-3.5 w-3.5 animate-spin" />
            {:else}
              <Play class="h-3.5 w-3.5 fill-current" />
            {/if}
            Get
          </button>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 text-xs">
          <div class="flex items-center gap-3">
            {#if capabilities}
              <button
                type="button"
                onclick={() => (showCapDetails = !showCapDetails)}
                class="flex items-center gap-1 rounded bg-slate-200 px-2 py-0.5 text-[11px] text-slate-700 hover:bg-slate-300 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
              >
                <Layers class="h-3 w-3 text-cyan-500" />
                Capabilities: {capabilities.version || "v0"} ({capabilities.models?.length || 0} models)
              </button>
            {/if}
            <span class="text-slate-400">取得件数: {results.length}</span>
          </div>

          <div class="flex items-center gap-2">
            {#if selectedIndices.length === 1 && onAddPolling}
              <button
                type="button"
                onclick={handleCreatePolling}
                class="flex items-center gap-1 rounded-md bg-blue-600 px-2.5 py-1 text-xs font-medium text-white hover:bg-blue-500"
              >
                <Plus class="h-3 w-3" />ポーリング作成
              </button>
            {/if}
            <button
              type="button"
              disabled={results.length === 0}
              onclick={copySelected}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              {#if copied}<Check class="h-3 w-3 text-emerald-500" />{:else}<Copy class="h-3 w-3" />{/if}
              {selectedIndices.length > 0 ? `選択(${selectedIndices.length})コピー` : "全てコピー"}
            </button>
            <button
              type="button"
              disabled={results.length === 0}
              onclick={exportCSV}
              class="flex items-center gap-1 rounded-md border border-slate-300 bg-white px-2.5 py-1 text-xs font-medium text-slate-700 hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-200"
            >
              <FileSpreadsheet class="h-3 w-3 text-emerald-500" />CSV
            </button>
          </div>
        </div>

        {#if showCapDetails && capabilities}
          <div class="rounded-lg border border-slate-200 bg-white p-3 text-xs dark:border-slate-800 dark:bg-slate-950">
            <p class="font-semibold text-slate-700 dark:text-slate-300">エンコーディング: <span class="font-mono text-cyan-600 dark:text-cyan-400">{capabilities.encodings}</span></p>
            <p class="mt-1 font-semibold text-slate-700 dark:text-slate-300">サポートモデル:</p>
            <div class="mt-1 max-h-32 overflow-auto font-mono text-[11px] text-slate-500 dark:text-slate-400">
              {#each capabilities.models as m}
                <div>{m.name} ({m.version || "—"}) - {m.organization || ""}</div>
              {:else}
                <div>モデル情報なし</div>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <!-- Result Table Area -->
      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        {#if errorMessage}
          <div class="mb-3 rounded-lg border border-rose-300 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
            {errorMessage}
          </div>
        {/if}

        <div class="overflow-auto rounded-xl border border-slate-200 dark:border-slate-800">
          <table class="w-full text-left text-xs">
            <thead class="sticky top-0 bg-slate-100 dark:bg-slate-900">
              <tr>
                <th class="p-2 w-8 text-center">#</th>
                <th class="p-2 font-mono">Path</th>
                <th class="p-2 font-mono w-20">Index</th>
                <th class="p-2">Value</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-200 dark:divide-slate-800">
              {#each results as r, idx}
                {@const isSelected = selectedIndices.includes(idx)}
                <tr
                  onclick={() => toggleSelect(idx)}
                  class={`cursor-pointer transition-colors ${isSelected ? "bg-cyan-50 dark:bg-cyan-950/40" : "hover:bg-slate-50 dark:hover:bg-slate-900/60"}`}
                >
                  <td class="p-2 text-center text-slate-400 font-mono text-[10px]">{idx + 1}</td>
                  <td class="p-2 font-mono text-slate-700 dark:text-slate-300 break-all">{r.Path}</td>
                  <td class="p-2 font-mono text-slate-500">{r.Index || "—"}</td>
                  <td class="p-2 font-mono break-all font-medium">{r.Value}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="4" class="p-6 text-center text-slate-500">
                    {#if isLoading}読み込み中...{:else}結果はありません (Pathを入力して「Get」を押してください){/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
{/if}
