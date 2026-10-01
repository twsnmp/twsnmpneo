<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import "maplibre-gl/dist/maplibre-gl.css";
  import { Map as MapGl, NavigationControl, Marker, Popup } from "maplibre-gl";
  import {
    fetchNodes,
    saveNode,
    fetchLocConf,
    type NodeEnt,
    type LocConfEnt,
  } from "../api";
  import { getStateColor, getStateName } from "../common";
  import { _ } from "svelte-i18n";
  import NodeDetailModal from "../components/NodeDetailModal.svelte";
  import {
    MapPin,
    Lock,
    Unlock,
    RefreshCw,
    Plus,
    Search,
    ExternalLink,
    AlertCircle,
    Layers,
  } from "@lucide/svelte";

  let mapContainer: HTMLDivElement | null = $state(null);
  let mapInstance: MapGl | null = null;
  let markers = $state<Marker[]>([]);

  let nodes = $state<NodeEnt[]>([]);
  let locConf = $state<LocConfEnt | null>(null);
  let isLoading = $state(false);
  let isLocked = $state(false);

  // Selected node for detail modal
  let selectedNode = $state<NodeEnt | null>(null);
  let showNodeDetail = $state(false);

  // Add node to map placement state
  let showAddNode = $state(false);
  let addNodeId = $state("");

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
    if (!mapContainer || mapInstance) return;

    try {
      locConf = await fetchLocConf();
    } catch {
      // Use defaults
    }

    const defaultCenter: [number, number] = [139.6917, 35.6895]; // Tokyo default
    const center = locConf?.Center ? getLngLat(locConf.Center) || defaultCenter : defaultCenter;
    const zoom = locConf?.Zoom || 10;

    const styleUrl =
      locConf?.Url ||
      "https://tile.openstreetmap.org/{z}/{x}/{y}.png";

    // Use OpenStreetMap raster style compatible with MapLibre GL
    const mapStyle = {
      version: 8 as const,
      sources: {
        "osm-tiles": {
          type: "raster" as const,
          tiles: [styleUrl],
          tileSize: 256,
          attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
        },
      },
      layers: [
        {
          id: "osm-layer",
          type: "raster" as const,
          source: "osm-tiles",
          minzoom: 0,
          maxzoom: 19,
        },
      ],
    };

    mapInstance = new MapGl({
      container: mapContainer,
      style: mapStyle,
      center: center,
      zoom: zoom,
    });

    mapInstance.addControl(new NavigationControl(), "top-right");

    mapInstance.on("click", (e) => {
      if (showAddNode && addNodeId) {
        placeNode(addNodeId, e.lngLat.lng, e.lngLat.lat);
      }
    });

    await refreshNodes();
  };

  const clearMarkers = () => {
    for (const m of markers) {
      m.remove();
    }
    markers = [];
  };

  const refreshNodes = async () => {
    isLoading = true;
    try {
      nodes = await fetchNodes();
      clearMarkers();

      if (!mapInstance) return;

      for (const n of nodes) {
        const coords = getLngLat(n.loc || "");
        if (!coords) continue;

        // Custom HTML Marker element
        const el = document.createElement("div");
        el.className = "twsnmp-map-marker cursor-pointer group flex flex-col items-center";
        
        const stateColor = getStateColor(n.state);
        el.innerHTML = `
          <div style="background-color: ${stateColor};" class="flex h-7 w-7 items-center justify-center rounded-full text-white shadow-lg ring-2 ring-white/80 transition-transform group-hover:scale-125">
            <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7z"/>
            </svg>
          </div>
          <div class="mt-1 rounded bg-slate-900/85 px-1.5 py-0.5 text-[10px] font-bold text-white shadow backdrop-blur-sm whitespace-nowrap">
            ${n.name || n.ip}
          </div>
        `;

        el.addEventListener("click", (evt) => {
          evt.stopPropagation();
          selectedNode = n;
          showNodeDetail = true;
        });

        const marker = new Marker({ element: el })
          .setLngLat(coords)
          .addTo(mapInstance);

        markers.push(marker);
      }
    } catch (err) {
      console.error("Failed to load map nodes:", err);
    } finally {
      isLoading = false;
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
    } catch (err) {
      console.error("Failed to update node location:", err);
    }
  };

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
  <div class="z-10 flex flex-wrap items-center justify-between border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 px-4 py-2.5 backdrop-blur-md">
    <div class="flex items-center gap-3">
      <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
        <MapPin class="h-4 w-4" />
      </div>
      <div>
        <h1 class="text-sm font-bold text-slate-900 dark:text-slate-100">
          {$_("location.title") || "Location View (GIS Map)"}
        </h1>
        <p class="text-[11px] text-slate-500 dark:text-slate-400">
          {$_("location.subtitle") || "Geographic distribution of network nodes powered by OpenStreetMap"}
        </p>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex items-center gap-2">
      <!-- Add Node to Location -->
      {#if unplacedNodes.length > 0}
        <div class="relative flex items-center gap-1.5">
          <select
            bind:value={addNodeId}
            onchange={() => (showAddNode = !!addNodeId)}
            class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2.5 py-1.5 text-xs text-slate-800 dark:text-slate-200"
          >
            <option value="">{$_("location.selectNodeToPlace") || "Place Node on Map..."}</option>
            {#each unplacedNodes as un}
              <option value={un.id}>{un.name} ({un.ip})</option>
            {/each}
          </select>
          {#if showAddNode}
            <span class="rounded bg-amber-500/10 px-2 py-1 text-[11px] font-semibold text-amber-600 dark:text-amber-400 border border-amber-500/30 animate-pulse">
              {$_("location.clickMapToPlace") || "Click map to place"}
            </span>
          {/if}
        </div>
      {/if}

      <!-- Lock Toggle -->
      <button
        type="button"
        onclick={() => (isLocked = !isLocked)}
        class="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors cursor-pointer"
      >
        {#if isLocked}
          <Lock class="h-3.5 w-3.5 text-amber-500" />
          <span>{$_("location.locked") || "Locked"}</span>
        {:else}
          <Unlock class="h-3.5 w-3.5 text-slate-400" />
          <span>{$_("location.unlocked") || "Unlocked"}</span>
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
  />
{/if}
