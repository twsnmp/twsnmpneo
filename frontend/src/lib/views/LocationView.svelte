<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import "maplibre-gl/dist/maplibre-gl.css";
  import { Map as MapGl, NavigationControl, Marker } from "maplibre-gl";
  import {
    fetchNodes,
    saveNode,
    fetchLocConf,
    saveLocConf,
    type NodeEnt,
    type LocConfEnt,
  } from "../api";
  import {
    getIcon,
    getStateColor,
    isImageIcon,
    getIconImage,
  } from "../common";
  import { resolveMapLibreStyle } from "../map/locStyles";
  import { _ } from "svelte-i18n";
  import { showConfirm } from "../stores/modalStore";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import LocConfModal from "../components/LocConfModal.svelte";
  import {
    MapPin,
    Lock,
    Unlock,
    RefreshCw,
    Sliders,
    Save,
    Pencil,
    Trash2,
    X,
    CheckCircle2,
    Plus,
  } from "@lucide/svelte";

  let mapContainer: HTMLDivElement | null = $state(null);
  let mapInstance: MapGl | null = null;
  let markers = $state<Marker[]>([]);

  let nodes = $state<NodeEnt[]>([]);
  let locConf = $state<LocConfEnt | null>(null);
  let isLoading = $state(false);
  let isLocked = $state(false);

  // Selected node for toolbar actions & detail modal
  let selectedNode = $state<NodeEnt | null>(null);
  let showNodeDetail = $state(false);

  // Add node via dropdown placement
  let showAddNode = $state(false);
  let addNodeId = $state("");

  // Context menu (right-click on map) placement modal
  let showContextAddModal = $state(false);
  let contextLoc = $state("");
  let contextNodeId = $state("");

  // Loc Conf modal
  let showLocConfModal = $state(false);

  // Feedback toast
  let toastMsg = $state("");
  let isSavingDef = $state(false);

  const showToast = (msg: string) => {
    toastMsg = msg;
    setTimeout(() => {
      if (toastMsg === msg) toastMsg = "";
    }, 3500);
  };

  const unplacedNodes = $derived(
    nodes.filter((n) => !n.loc || n.loc.trim() === "" || n.loc.split(",").length < 2)
  );

  const getLngLat = (loc: string): [number, number] | null => {
    if (!loc) return null;
    const parts = loc.split(",");
    if (parts.length < 2) return null;
    const lng = parseFloat(parts[0].trim());
    const lat = parseFloat(parts[1].trim());
    if (isNaN(lng) || isNaN(lat)) return null;
    return [lng, lat];
  };

  const initMap = async () => {
    if (!mapContainer) return;

    try {
      locConf = await fetchLocConf();
    } catch (e) {
      console.warn("Failed to fetch loc conf:", e);
    }

    const defaultCenter: [number, number] = [139.75, 35.68];
    const center = locConf?.Center ? getLngLat(locConf.Center) || defaultCenter : defaultCenter;
    const zoom = locConf?.Zoom && locConf.Zoom > 0 ? locConf.Zoom : 10;
    const mapStyle = resolveMapLibreStyle(locConf?.Style);

    if (mapInstance) {
      mapInstance.remove();
      mapInstance = null;
    }

    mapInstance = new MapGl({
      container: mapContainer,
      style: mapStyle,
      center: center,
      zoom: zoom,
    });

    mapInstance.addControl(new NavigationControl({ visualizePitch: true }), "top-right");

    // Click on map
    mapInstance.on("click", (e) => {
      if (showAddNode && addNodeId) {
        placeNode(addNodeId, e.lngLat.lng, e.lngLat.lat);
      } else {
        // Deselect node if clicked on empty map space
        selectedNode = null;
      }
    });

    // Right-click contextmenu on map space
    mapInstance.on("contextmenu", (e) => {
      if (isLocked) return;
      contextLoc = `${e.lngLat.lng.toFixed(6)},${e.lngLat.lat.toFixed(6)}`;
      contextNodeId = unplacedNodes.length > 0 ? unplacedNodes[0].id : "";
      showContextAddModal = true;
    });

    await refreshNodes();
  };

  const clearMarkers = () => {
    for (const m of markers) {
      try {
        m.remove();
      } catch {}
    }
    markers = [];
  };

  const refreshNodes = async () => {
    isLoading = true;
    try {
      nodes = await fetchNodes();
      clearMarkers();

      if (!mapInstance) return;

      const iconPx = locConf?.IconSize || 24;
      const boxSize = iconPx + 8;

      for (const n of nodes) {
        const coords = getLngLat(n.loc || "");
        if (!coords) continue;

        const el = document.createElement("div");
        el.className = `twsnmp-map-marker cursor-pointer select-none flex flex-col items-center group ${selectedNode?.id === n.id ? "selected" : ""}`;
        el.dataset.nodeId = n.id;

        const stateColor = getStateColor(n.state);
        const isSelected = selectedNode?.id === n.id;

        let iconInner = "";
        if (isImageIcon(n.icon || "")) {
          const imgSrc = getIconImage(n.icon || "");
          iconInner = `<img src="${imgSrc}" alt="${n.name}" style="width: ${iconPx}px; height: ${iconPx}px; object-fit: contain;" />`;
        } else {
          const mdiClass = getIcon(n.icon || "");
          iconInner = `<span class="mdi ${mdiClass}" style="font-size: ${iconPx}px; line-height: 1;"></span>`;
        }

        el.innerHTML = `
          <div class="marker-icon-box flex items-center justify-center rounded-lg text-white shadow-lg transition-transform group-hover:scale-110"
               style="width: ${boxSize}px; height: ${boxSize}px; background-color: ${stateColor}; ${isSelected ? 'outline: 3px solid #3b82f6; outline-offset: 2px;' : 'box-shadow: 0 4px 6px -1px rgba(0,0,0,0.3);'}">
            ${iconInner}
          </div>
          <div class="mt-1 rounded bg-slate-900/85 px-1.5 py-0.5 text-[10px] font-bold text-white shadow backdrop-blur-sm whitespace-nowrap max-w-[120px] truncate">
            ${n.name || n.ip}
          </div>
        `;

        // Click to select
        el.addEventListener("click", (evt) => {
          evt.stopPropagation();
          selectedNode = selectedNode?.id === n.id ? null : n;
          updateMarkerStyles();
        });

        // Double click to open detail
        el.addEventListener("dblclick", (evt) => {
          evt.stopPropagation();
          selectedNode = n;
          showNodeDetail = true;
        });

        const marker = new Marker({
          element: el,
          draggable: !isLocked,
        })
          .setLngLat(coords)
          .addTo(mapInstance);

        // Marker dragend
        marker.on("dragend", async (e: any) => {
          if (isLocked) return;
          const targetCoords = e.target.getLngLat();
          const newLoc = `${targetCoords.lng.toFixed(6)},${targetCoords.lat.toFixed(6)}`;
          try {
            await saveNode({ ...n, loc: newLoc });
            n.loc = newLoc;
            n.Loc = newLoc;
          } catch (err) {
            console.error("Failed to save dragged node position:", err);
            await refreshNodes();
          }
        });

        markers.push(marker);
      }
    } catch (err) {
      console.error("Failed to load map nodes:", err);
    } finally {
      isLoading = false;
    }
  };

  const updateMarkerStyles = () => {
    for (const m of markers) {
      const el = m.getElement();
      const nodeId = el.dataset.nodeId;
      const box = el.querySelector(".marker-icon-box") as HTMLDivElement | null;
      if (!box) continue;
      if (nodeId && selectedNode && nodeId === selectedNode.id) {
        box.style.outline = "3px solid #3b82f6";
        box.style.outlineOffset = "2px";
      } else {
        box.style.outline = "";
        box.style.outlineOffset = "";
      }
    }
  };

  const placeNode = async (nodeId: string, lng: number, lat: number) => {
    const target = nodes.find((n) => n.id === nodeId);
    if (!target) return;

    const locStr = `${lng.toFixed(6)},${lat.toFixed(6)}`;
    const updated = { ...target, loc: locStr };
    try {
      await saveNode(updated);
      showAddNode = false;
      addNodeId = "";
      await refreshNodes();
      showToast(`${target.name} を地図上に配置しました`);
    } catch (err) {
      console.error("Failed to update node location:", err);
    }
  };

  const handleContextAdd = async () => {
    if (!contextNodeId || !contextLoc) return;
    const parts = contextLoc.split(",");
    if (parts.length < 2) return;
    const lng = parseFloat(parts[0]);
    const lat = parseFloat(parts[1]);
    await placeNode(contextNodeId, lng, lat);
    showContextAddModal = false;
  };

  const removeSelectedNodeLoc = async () => {
    if (!selectedNode || isLocked) return;
    const ok = await showConfirm({
      title: $_("location.clearLoc") || "地図から削除",
      message:
        $_("location.clearLocConfirm") ||
        "選択したノードを地図上から削除しますか？（ノード自体のデータは削除されず、位置情報のみクリアされます）",
      detail: `${selectedNode.name} (${selectedNode.ip})`,
      type: "danger",
      confirmText: $_("common.delete") || "削除",
    });
    if (!ok) return;

    try {
      await saveNode({ ...selectedNode, loc: "" });
      const clearedName = selectedNode.name;
      selectedNode = null;
      await refreshNodes();
      showToast(`${clearedName} を地図から削除しました`);
    } catch (err) {
      console.error("Failed to clear node coordinates:", err);
    }
  };

  const saveDefaultView = async () => {
    if (!mapInstance || isLocked) return;
    isSavingDef = true;
    try {
      const c = mapInstance.getCenter();
      const z = mapInstance.getZoom();
      const updatedConf: LocConfEnt = {
        Style: locConf?.Style || "",
        Center: `${c.lng.toFixed(6)},${c.lat.toFixed(6)}`,
        Zoom: Math.round(z * 10) / 10,
        IconSize: locConf?.IconSize || 24,
      };
      const ok = await saveLocConf(updatedConf);
      if (ok) {
        locConf = updatedConf;
        showToast($_("location.saveDefSuccess") || "Saved default view.");
      }
    } catch (err) {
      console.error("Failed to save default map view:", err);
    } finally {
      isSavingDef = false;
    }
  };

  const onLocConfSaved = async () => {
    // Reload locConf and reinitialize map or refresh markers
    try {
      const newConf = await fetchLocConf();
      const styleChanged = locConf?.Style !== newConf.Style;
      const sizeChanged = locConf?.IconSize !== newConf.IconSize;
      locConf = newConf;

      if (styleChanged) {
        if (mapInstance) {
          mapInstance.setStyle(resolveMapLibreStyle(newConf.Style));
        } else {
          await initMap();
        }
      }
      if (sizeChanged || styleChanged) {
        await refreshNodes();
      }
    } catch (err) {
      console.error("Failed to reload loc conf:", err);
    }
  };

  $effect(() => {
    // Synchronize marker draggability with lock state
    for (const m of markers) {
      m.setDraggable(!isLocked);
    }
  });

  onMount(() => {
    initMap();
  });

  onDestroy(() => {
    clearMarkers();
    if (mapInstance) {
      mapInstance.remove();
      mapInstance = null;
    }
  });
</script>

<div class="relative flex h-full w-full flex-col overflow-hidden bg-slate-100 dark:bg-slate-950">
  <!-- Top Action Bar -->
  <div class="z-10 flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 dark:border-slate-800 bg-white/95 dark:bg-slate-900/95 px-4 py-2.5 backdrop-blur-md">
    <!-- Left Title -->
    <div class="flex items-center gap-3">
      <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
        <MapPin class="h-4 w-4" />
      </div>
      <div>
        <h1 class="text-sm font-bold text-slate-900 dark:text-slate-100">
          {$_("location.title") || "地図 (GIS)"}
        </h1>
        <p class="text-[11px] text-slate-500 dark:text-slate-400">
          {$_("location.subtitle") || "OpenStreetMapを用いたネットワークノードの地理的配置と稼働状況の可視化"}
        </p>
      </div>

      <!-- Selected Node Badge -->
      {#if selectedNode}
        <div class="hidden sm:flex items-center gap-1.5 rounded-lg border border-blue-500/30 bg-blue-500/10 px-2.5 py-1 text-xs text-blue-700 dark:text-blue-300">
          <span class="font-semibold">{selectedNode.name}</span>
          <span class="text-[10px] text-slate-500 dark:text-slate-400">({selectedNode.ip})</span>
        </div>
      {/if}
    </div>

    <!-- Right Actions -->
    <div class="flex flex-wrap items-center gap-2">
      <!-- Selected Node Actions -->
      {#if selectedNode}
        <div class="flex items-center gap-1.5 border-r border-slate-200 dark:border-slate-700 pr-2 mr-1">
          <button
            type="button"
            onclick={() => (showNodeDetail = true)}
            class="inline-flex items-center gap-1 rounded-lg bg-blue-50 dark:bg-blue-950/40 border border-blue-200 dark:border-blue-800 px-2.5 py-1.5 text-xs font-semibold text-blue-700 dark:text-blue-300 hover:bg-blue-100 dark:hover:bg-blue-900/60 transition-colors cursor-pointer"
            title={$_("location.edit") || "編集"}
          >
            <Pencil class="h-3.5 w-3.5" />
            <span>{$_("location.edit") || "編集"}</span>
          </button>
          {#if !isLocked}
            <button
              type="button"
              onclick={removeSelectedNodeLoc}
              class="inline-flex items-center gap-1 rounded-lg bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-800 px-2.5 py-1.5 text-xs font-semibold text-rose-700 dark:text-rose-300 hover:bg-rose-100 dark:hover:bg-rose-900/60 transition-colors cursor-pointer"
              title={$_("location.clearLoc") || "地図から削除"}
            >
              <Trash2 class="h-3.5 w-3.5" />
              <span>{$_("location.clearLoc") || "地図から削除"}</span>
            </button>
          {/if}
        </div>
      {/if}

      <!-- Add Node to Location Dropdown -->
      {#if unplacedNodes.length > 0 && !isLocked}
        <div class="relative flex items-center gap-1.5">
          <select
            bind:value={addNodeId}
            onchange={() => (showAddNode = !!addNodeId)}
            class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2.5 py-1.5 text-xs text-slate-800 dark:text-slate-200 cursor-pointer"
          >
            <option value="">{$_("location.selectNodeToPlace") || "ノードを地図上に配置..."}</option>
            {#each unplacedNodes as un}
              <option value={un.id}>{un.name} ({un.ip})</option>
            {/each}
          </select>
          {#if showAddNode}
            <span class="rounded bg-amber-500/10 px-2 py-1 text-[11px] font-semibold text-amber-600 dark:text-amber-400 border border-amber-500/30 animate-pulse whitespace-nowrap">
              {$_("location.clickMapToPlace") || "地図をクリックして配置"}
            </span>
          {/if}
        </div>
      {/if}

      <!-- Save Default View Button -->
      {#if !isLocked}
        <button
          type="button"
          disabled={isSavingDef}
          onclick={saveDefaultView}
          class="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors cursor-pointer disabled:opacity-50"
          title={$_("location.saveDef") || "初期表示保存"}
        >
          <Save class="h-3.5 w-3.5 text-indigo-500" />
          <span>{$_("location.saveDef") || "初期表示保存"}</span>
        </button>
      {/if}

      <!-- Settings Button -->
      <button
        type="button"
        onclick={() => (showLocConfModal = true)}
        class="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors cursor-pointer"
        title={$_("location.settings") || "地図設定"}
      >
        <Sliders class="h-3.5 w-3.5 text-slate-500 dark:text-slate-400" />
        <span>{$_("location.settings") || "地図設定"}</span>
      </button>

      <!-- Lock Toggle Button -->
      <button
        type="button"
        onclick={() => (isLocked = !isLocked)}
        class="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors cursor-pointer"
      >
        {#if isLocked}
          <Lock class="h-3.5 w-3.5 text-amber-500" />
          <span>{$_("location.locked") || "位置ロック中"}</span>
        {:else}
          <Unlock class="h-3.5 w-3.5 text-slate-400" />
          <span>{$_("location.unlocked") || "位置編集可能"}</span>
        {/if}
      </button>

      <!-- Reload Button -->
      <button
        type="button"
        disabled={isLoading}
        onclick={refreshNodes}
        class="inline-flex items-center gap-1.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 text-xs font-semibold shadow-sm transition-all disabled:opacity-50 cursor-pointer"
      >
        <RefreshCw class="h-3.5 w-3.5 {isLoading ? 'animate-spin' : ''}" />
        <span>{$_("common.reload")}</span>
      </button>
    </div>
  </div>

  <!-- Toast Notification -->
  {#if toastMsg}
    <div class="absolute top-14 left-1/2 -translate-x-1/2 z-30 flex items-center gap-2 rounded-xl bg-slate-900/90 text-white px-4 py-2 text-xs font-semibold shadow-xl border border-slate-700 backdrop-blur-md animate-in fade-in slide-in-from-top-2 duration-150">
      <CheckCircle2 class="h-4 w-4 text-emerald-400" />
      <span>{toastMsg}</span>
    </div>
  {/if}

  <!-- Map Container -->
  <div bind:this={mapContainer} class="relative flex-1 w-full h-full min-h-0">
    {#if isLoading && markers.length === 0}
      <div class="absolute inset-0 z-20 flex items-center justify-center bg-slate-900/20 backdrop-blur-[2px]">
        <div class="flex items-center gap-2 rounded-xl bg-white dark:bg-slate-900 px-4 py-2.5 shadow-xl border border-slate-200 dark:border-slate-800">
          <RefreshCw class="h-4 w-4 animate-spin text-blue-600 dark:text-cyan-400" />
          <span class="text-xs font-semibold text-slate-700 dark:text-slate-200">
            {$_("common.loading")}
          </span>
        </div>
      </div>
    {/if}
  </div>
</div>

<!-- Node Detail Modal -->
{#if selectedNode}
  <NodeDetailModal
    bind:show={showNodeDetail}
    node={selectedNode}
    onNodeUpdated={refreshNodes}
  />
{/if}

<!-- Map Configuration Modal -->
<LocConfModal
  bind:show={showLocConfModal}
  onSaved={onLocConfSaved}
/>

<!-- Context Menu Add Node Modal -->
{#if showContextAddModal}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="flex flex-col w-full max-w-sm rounded-xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-2xl p-4 space-y-4 animate-in fade-in zoom-in-95 duration-150">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2">
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
          <Plus class="h-4 w-4 text-blue-600" />
          {$_("location.addNode") || "ノードの追加"}
        </h3>
        <button
          type="button"
          onclick={() => (showContextAddModal = false)}
          class="text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <div class="space-y-2 text-xs">
        <div class="text-slate-500 dark:text-slate-400">
          座標: <span class="font-mono text-slate-700 dark:text-slate-300">{contextLoc}</span>
        </div>

        {#if unplacedNodes.length === 0}
          <div class="rounded-lg bg-amber-50 dark:bg-amber-950/30 border border-amber-500/20 p-2.5 text-amber-600 dark:text-amber-400">
            未配置のノードがありません。
          </div>
        {:else}
          <label for="context-node-select" class="block font-semibold text-slate-800 dark:text-slate-200">
            {$_("location.selectNodeToAdd") || "配置するノードを選択"}
          </label>
          <select
            id="context-node-select"
            bind:value={contextNodeId}
            class="w-full rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-2 text-xs text-slate-900 dark:text-slate-100"
          >
            {#each unplacedNodes as n}
              <option value={n.id}>{n.name} ({n.ip})</option>
            {/each}
          </select>
        {/if}
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showContextAddModal = false)}
          class="rounded-lg border border-slate-300 dark:border-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        {#if unplacedNodes.length > 0}
          <button
            type="button"
            onclick={handleContextAdd}
            class="rounded-lg bg-blue-600 hover:bg-blue-700 text-white px-4 py-1.5 text-xs font-semibold shadow-sm transition-all"
          >
            {$_("common.add") || "追加"}
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  :global(.twsnmp-map-marker) {
    transition: transform 0.15s ease-out;
  }
  :global(.twsnmp-map-marker.selected .marker-icon-box) {
    animation: marker-pulse 2s infinite;
  }
  @keyframes marker-pulse {
    0%, 100% {
      box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7);
    }
    50% {
      box-shadow: 0 0 0 6px rgba(59, 130, 246, 0);
    }
  }
</style>
