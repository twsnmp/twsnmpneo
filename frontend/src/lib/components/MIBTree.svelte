<script lang="ts">
  import { _ } from "svelte-i18n";
  import { Search, FolderTree } from "@lucide/svelte";
  import type { MIBTreeEnt } from "../api";
  import MIBTreeNode from "./MIBTreeNode.svelte";

  const STORAGE_KEY = "twsnmp_mibtree_expansion";

  function loadExpansionState(): Map<string, boolean> {
    const map = new Map<string, boolean>();
    map.set(".1", true); // Default root expanded
    try {
      if (typeof localStorage !== "undefined") {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (raw) {
          const obj = JSON.parse(raw);
          if (typeof obj === "object" && obj !== null) {
            for (const [k, v] of Object.entries(obj)) {
              map.set(k, Boolean(v));
            }
          }
        }
      }
    } catch {}
    return map;
  }

  const globalExpansionState = loadExpansionState();

  function saveExpansionState() {
    try {
      if (typeof localStorage !== "undefined") {
        const obj: Record<string, boolean> = {};
        for (const [k, v] of globalExpansionState.entries()) {
          if (v) obj[k] = true;
        }
        localStorage.setItem(STORAGE_KEY, JSON.stringify(obj));
      }
    } catch {}
  }

  let {
    treeData = [],
    onselect = undefined,
    heightClass = "h-[65vh]",
    filterPlaceholder = undefined,
  } = $props<{
    treeData: MIBTreeEnt[] | MIBTreeEnt;
    onselect?: (name: string, oid: string) => void;
    heightClass?: string;
    filterPlaceholder?: string;
  }>();

  let filterText = $state("");
  let expansionState = globalExpansionState;

  // Hover Tooltip state
  let hoveredInfo = $state<{
    name: string;
    oid: string;
    type?: string;
    tooltip: string;
    x: number;
    y: number;
  } | null>(null);

  // Filter tree recursively
  const filterMIBTree = (node: MIBTreeEnt, needle: string): (MIBTreeEnt & { forceExpand?: boolean }) | null => {
    if (!node) return null;
    const n = needle.toLowerCase().trim();
    if (!n) return node;

    const nameMatch = (node.name || "").toLowerCase().includes(n);
    const oidMatch = (node.oid || "").includes(n);
    const match = nameMatch || oidMatch;

    let filteredChildren: (MIBTreeEnt & { forceExpand?: boolean })[] = [];
    if (node.children && node.children.length > 0) {
      filteredChildren = node.children
        .map((c) => filterMIBTree(c, n))
        .filter((c): c is MIBTreeEnt & { forceExpand?: boolean } => c !== null);
    }

    if (match || filteredChildren.length > 0) {
      return {
        ...node,
        children: filteredChildren,
        forceExpand: filteredChildren.length > 0,
      };
    }
    return null;
  };

  // Build root structure
  const rootNode = $derived.by(() => {
    let root: MIBTreeEnt;
    if (Array.isArray(treeData)) {
      // If treeData is an array of top-level children, wrap in .iso (.1)
      root = {
        oid: ".1",
        name: ".iso",
        children: treeData,
      };
    } else {
      root = treeData;
    }

    if (filterText.trim()) {
      return filterMIBTree(root, filterText.trim());
    }
    return root;
  });

  const handleHover = (info: { name: string; oid: string; type?: string; tooltip: string; x: number; y: number }) => {
    hoveredInfo = info;
  };

  const handleLeave = () => {
    hoveredInfo = null;
  };
</script>

<div class="flex flex-col gap-3 w-full h-full">
  <div class="relative shrink-0">
    <Search class="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-slate-400" />
    <input
      type="text"
      bind:value={filterText}
      placeholder={filterPlaceholder || $_("mib.treeFilter")}
      class="w-full rounded-lg border border-slate-300 bg-white pl-8 pr-3 py-1.5 text-xs text-slate-800 focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
    />
  </div>

  <div class="min-h-0 flex-1 overflow-auto rounded-xl border border-slate-200 bg-slate-50/50 p-2 dark:border-slate-800 dark:bg-slate-950/40 {heightClass}">
    {#if rootNode}
      <MIBTreeNode
        tree={rootNode}
        {expansionState}
        {onselect}
        onHover={handleHover}
        onLeave={handleLeave}
        onToggle={saveExpansionState}
      />
    {:else}
      <div class="flex h-32 items-center justify-center text-xs text-slate-500">
        {$_("mib.noTreeMatches")}
      </div>
    {/if}
  </div>

  {#if hoveredInfo && hoveredInfo.tooltip}
    <div
      class="pointer-events-none fixed z-[9999] max-w-lg rounded-xl border border-slate-700 bg-slate-900/95 p-3 text-xs text-slate-100 shadow-2xl backdrop-blur-md"
      style="left: {Math.min(hoveredInfo.x + 16, (typeof window !== 'undefined' ? window.innerWidth - 420 : 500))}px; top: {Math.min(hoveredInfo.y + 16, (typeof window !== 'undefined' ? window.innerHeight - 260 : 500))}px;"
    >
      <div class="flex items-center gap-1.5 font-mono font-bold text-teal-400 pb-1 mb-1 border-b border-slate-700/80">
        <FolderTree class="h-3.5 w-3.5 text-teal-400" />
        <span>{hoveredInfo.name}</span>
        <span class="text-slate-400 font-normal">({hoveredInfo.oid}{hoveredInfo.type ? `:${hoveredInfo.type}` : ""})</span>
      </div>
      <pre class="font-sans whitespace-pre-wrap leading-relaxed text-slate-300 max-h-56 overflow-y-auto text-[11px] select-text">{hoveredInfo.tooltip}</pre>
    </div>
  {/if}
</div>
