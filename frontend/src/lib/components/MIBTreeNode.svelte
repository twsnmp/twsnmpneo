<script lang="ts">
  import { locale } from "svelte-i18n";
  import { ChevronRight, ChevronDown } from "@lucide/svelte";
  import type { MIBTreeEnt, MIBInfoEnt } from "../api";
  import MIBTreeNode from "./MIBTreeNode.svelte";

  let {
    tree,
    expansionState,
    onselect,
    onHover,
    onLeave,
    onToggle,
  } = $props<{
    tree: MIBTreeEnt & { forceExpand?: boolean; count?: number };
    expansionState: Map<string, boolean>;
    onselect?: (name: string, oid: string) => void;
    onHover?: (info: { name: string; oid: string; type?: string; tooltip: string; x: number; y: number }) => void;
    onLeave?: () => void;
    onToggle?: (oid: string, expanded: boolean) => void;
  }>();

  const oid = $derived(tree.oid || "");
  const name = $derived(tree.name || "");
  const children = $derived(tree.children || []);
  const hasChildren = $derived(children.length > 0);
  const mibInfo = $derived((tree.mibInfo || (tree as any).MIBInfo) as MIBInfoEnt | undefined);
  const rawType = $derived(mibInfo?.type || (mibInfo as any)?.Type || "");
  const typeStr = $derived(rawType ? `:${rawType}` : "");
  const count = $derived(tree.count || 0);

  let isExpanded = $state(false);

  $effect(() => {
    if (tree.forceExpand) {
      isExpanded = true;
    } else if (oid && expansionState.has(oid)) {
      isExpanded = !!expansionState.get(oid);
    } else if (oid === ".1") {
      isExpanded = true;
    }
  });

  const toggleExpand = (e: MouseEvent) => {
    e.stopPropagation();
    isExpanded = !isExpanded;
    if (oid) {
      expansionState.set(oid, isExpanded);
      onToggle?.(oid, isExpanded);
    }
  };

  const handleSelect = (e: MouseEvent | KeyboardEvent) => {
    e.stopPropagation();
    if (onselect && name) {
      onselect(name, oid);
    }
  };

  const getTooltipContent = () => {
    if (!mibInfo) return "";
    const isJa = $locale === "ja" || (!$locale && typeof navigator !== "undefined" && navigator.language?.startsWith("ja"));
    if (isJa) {
      return mibInfo.descriptionJa || mibInfo.description || mibInfo.descriptionEn || "";
    }
    return mibInfo.descriptionEn || mibInfo.description || "";
  };

  const handleMouseEnter = (e: MouseEvent) => {
    const text = getTooltipContent();
    if (text && onHover) {
      onHover({
        name,
        oid,
        type: rawType,
        tooltip: text,
        x: e.clientX,
        y: e.clientY,
      });
    }
  };

  const handleMouseMove = (e: MouseEvent) => {
    const text = getTooltipContent();
    if (text && onHover) {
      onHover({
        name,
        oid,
        type: rawType,
        tooltip: text,
        x: e.clientX,
        y: e.clientY,
      });
    }
  };

  const handleMouseLeave = () => {
    if (onLeave) {
      onLeave();
    }
  };
</script>

<ul class="m-0 list-none pl-4 text-xs font-mono select-none">
  <li class="my-0.5">
    <div
      class="inline-flex items-center gap-1 py-0.5 px-1 rounded text-slate-700 dark:text-slate-200"
      onmouseenter={handleMouseEnter}
      onmousemove={handleMouseMove}
      onmouseleave={handleMouseLeave}
      role="none"
    >
      {#if hasChildren}
        <button
          type="button"
          class="inline-flex items-center justify-center w-4 h-4 rounded text-slate-400 hover:text-teal-600 hover:bg-slate-200 dark:hover:bg-slate-800 dark:hover:text-teal-400 transition-colors"
          onclick={toggleExpand}
          title={isExpanded ? "Collapse" : "Expand"}
          aria-label={isExpanded ? "Collapse" : "Expand"}
        >
          {#if isExpanded}
            <ChevronDown class="h-3.5 w-3.5" />
          {:else}
            <ChevronRight class="h-3.5 w-3.5" />
          {/if}
        </button>
      {:else}
        {#if rawType === "Notification"}
          <span class="w-4 text-center font-bold text-rose-500">*</span>
        {:else}
          <span class="w-4"></span>
        {/if}
      {/if}

      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <span
        class="inline-flex items-center gap-1 py-0.5 px-1.5 rounded cursor-pointer hover:bg-teal-500/15 dark:hover:bg-teal-400/20 text-slate-700 dark:text-slate-200 transition-colors"
        onclick={handleSelect}
        ondblclick={handleSelect}
        onkeydown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            handleSelect(e);
          }
        }}
        role="button"
        tabindex="0"
      >
        <span class="font-semibold text-teal-700 dark:text-teal-300">
          {name}
        </span>
        <span class="text-slate-500 dark:text-slate-400">
          ({oid}{typeStr})
        </span>
        {#if count > 0}
          <span class="text-teal-600 font-bold">: {count}</span>
        {/if}
      </span>
    </div>

    {#if hasChildren && isExpanded}
      {#each children as child (child.oid || child.name)}
        <MIBTreeNode
          tree={child}
          {expansionState}
          {onselect}
          {onHover}
          {onLeave}
          {onToggle}
        />
      {/each}
    {/if}
  </li>
</ul>
