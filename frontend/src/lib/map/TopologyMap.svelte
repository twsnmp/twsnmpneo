<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import p5 from 'p5';
  import type { NodeEnt, LineEnt } from '$lib/api';
  import { ZoomIn, ZoomOut, RefreshCw, Layers } from '@lucide/svelte';

  let { nodes = [], lines = [], onSelectNode }: {
    nodes: NodeEnt[];
    lines: LineEnt[];
    onSelectNode?: (node: NodeEnt | null) => void;
  } = $props();

  let container: HTMLDivElement;
  let p5Instance: p5 | null = null;

  // Viewport transforms
  let offsetX = 0;
  let offsetY = 0;
  let scale = 1.0;
  let isDraggingBg = false;
  let dragStartX = 0;
  let dragStartY = 0;

  // Node dragging
  let draggedNode: NodeEnt | null = null;
  let selectedNodeId: string | null = null;

  const STATE_COLORS: Record<string, [number, number, number]> = {
    normal: [16, 185, 129],  // emerald-500
    warn: [245, 158, 11],    // amber-500
    low: [59, 130, 246],     // blue-500
    high: [249, 115, 22],    // orange-500
    error: [239, 68, 68],    // red-500
    unknown: [100, 116, 139] // slate-500
  };

  function getNodeColor(state: string): [number, number, number] {
    return STATE_COLORS[state.toLowerCase()] || STATE_COLORS.unknown;
  }

  function resetView() {
    offsetX = 0;
    offsetY = 0;
    scale = 1.0;
  }

  function zoom(delta: number) {
    scale = Math.max(0.2, Math.min(3.0, scale + delta));
  }

  onMount(() => {
    const sketch = (p: p5) => {
      p.setup = () => {
        const w = container.clientWidth || 800;
        const h = container.clientHeight || 600;
        p.createCanvas(w, h);
        p.textAlign(p.CENTER, p.CENTER);
        p.textFont('system-ui, -apple-system, BlinkMacSystemFont');
      };

      p.windowResized = () => {
        if (!container) return;
        p.resizeCanvas(container.clientWidth, container.clientHeight);
      };

      p.draw = () => {
        p.background(15, 23, 42); // slate-900

        // Grid dots
        p.push();
        p.translate(offsetX, offsetY);
        p.scale(scale);

        // Draw background grid
        p.stroke(30, 41, 59, 150);
        p.strokeWeight(1 / scale);
        const gridSize = 40;
        const startX = -offsetX / scale - 100;
        const endX = (p.width - offsetX) / scale + 100;
        const startY = -offsetY / scale - 100;
        const endY = (p.height - offsetY) / scale + 100;

        for (let x = Math.floor(startX / gridSize) * gridSize; x < endX; x += gridSize) {
          p.line(x, startY, x, endY);
        }
        for (let y = Math.floor(startY / gridSize) * gridSize; y < endY; y += gridSize) {
          p.line(startX, y, endX, y);
        }

        // Draw Lines
        const nodeMap = new Map<string, NodeEnt>();
        for (const n of nodes) {
          nodeMap.set(n.id, n);
        }

        for (const l of lines) {
          const n1 = nodeMap.get(l.node_id1);
          const n2 = nodeMap.get(l.node_id2);
          if (n1 && n2) {
            const x1 = n1.x ?? 100;
            const y1 = n1.y ?? 100;
            const x2 = n2.x ?? 200;
            const y2 = n2.y ?? 200;

            const [r, g, b] = getNodeColor(l.state || 'normal');
            p.stroke(r, g, b, 180);
            p.strokeWeight((l.width || 3) / scale);
            p.line(x1, y1, x2, y2);
          }
        }

        // Draw Nodes
        for (const n of nodes) {
          const nx = n.x ?? 100;
          const ny = n.y ?? 100;
          const [r, g, b] = getNodeColor(n.state);
          const isSelected = n.id === selectedNodeId;

          p.push();
          p.translate(nx, ny);

          // Selection halo
          if (isSelected) {
            p.noFill();
            p.stroke(96, 165, 250, 200); // blue-400
            p.strokeWeight(3 / scale);
            p.ellipse(0, 0, 56, 56);
          }

          // Node body glow
          p.noStroke();
          p.fill(r, g, b, 40);
          p.ellipse(0, 0, 48, 48);

          // Node core circle
          p.fill(30, 41, 59); // slate-800
          p.stroke(r, g, b);
          p.strokeWeight(2.5 / scale);
          p.ellipse(0, 0, 36, 36);

          // State indicator dot
          p.fill(r, g, b);
          p.noStroke();
          p.ellipse(0, 0, 14, 14);

          // Node Label
          p.fill(241, 245, 249); // slate-100
          p.textSize(12 / scale);
          p.text(n.name, 0, 30);

          // IP Sub-label
          p.fill(148, 163, 184); // slate-400
          p.textSize(10 / scale);
          p.text(n.ip, 0, 44);

          p.pop();
        }

        p.pop();
      };

      p.mousePressed = () => {
        // Transform screen coords to world coords
        const worldX = (p.mouseX - offsetX) / scale;
        const worldY = (p.mouseY - offsetY) / scale;

        // Check if a node was clicked (within radius 24)
        for (let i = nodes.length - 1; i >= 0; i--) {
          const n = nodes[i];
          const nx = n.x ?? 100;
          const ny = n.y ?? 100;
          const d = p.dist(worldX, worldY, nx, ny);
          if (d <= 28) {
            draggedNode = n;
            selectedNodeId = n.id;
            if (onSelectNode) onSelectNode(n);
            return;
          }
        }

        // Clicked background: start pan
        selectedNodeId = null;
        if (onSelectNode) onSelectNode(null);
        isDraggingBg = true;
        dragStartX = p.mouseX - offsetX;
        dragStartY = p.mouseY - offsetY;
      };

      p.mouseDragged = () => {
        if (draggedNode) {
          draggedNode.x = (p.mouseX - offsetX) / scale;
          draggedNode.y = (p.mouseY - offsetY) / scale;
        } else if (isDraggingBg) {
          offsetX = p.mouseX - dragStartX;
          offsetY = p.mouseY - dragStartY;
        }
      };

      p.mouseReleased = () => {
        draggedNode = null;
        isDraggingBg = false;
      };

      p.mouseWheel = (event: WheelEvent) => {
        const zoomFactor = event.deltaY < 0 ? 1.08 : 0.92;
        const newScale = Math.max(0.2, Math.min(3.0, scale * zoomFactor));

        // Zoom towards mouse position
        offsetX = p.mouseX - (p.mouseX - offsetX) * (newScale / scale);
        offsetY = p.mouseY - (p.mouseY - offsetY) * (newScale / scale);
        scale = newScale;
        return false;
      };
    };

    p5Instance = new p5(sketch, container);
  });

  onDestroy(() => {
    p5Instance?.remove();
  });
</script>

<div class="relative w-full h-full min-h-[500px] flex-1 overflow-hidden rounded-xl bg-slate-900 border border-slate-800">
  <div bind:this={container} class="w-full h-full cursor-grab active:cursor-grabbing"></div>

  <!-- Floating Map Controls -->
  <div class="absolute bottom-4 right-4 flex items-center space-x-1.5 bg-slate-800/90 backdrop-blur border border-slate-700 p-1.5 rounded-lg shadow-lg">
    <button
      onclick={() => zoom(0.15)}
      class="p-2 text-slate-300 hover:text-white hover:bg-slate-700/80 rounded transition-colors"
      title="Zoom In"
    >
      <ZoomIn class="w-4 h-4" />
    </button>
    <button
      onclick={() => zoom(-0.15)}
      class="p-2 text-slate-300 hover:text-white hover:bg-slate-700/80 rounded transition-colors"
      title="Zoom Out"
    >
      <ZoomOut class="w-4 h-4" />
    </button>
    <div class="h-4 w-px bg-slate-700"></div>
    <button
      onclick={resetView}
      class="p-2 text-slate-300 hover:text-white hover:bg-slate-700/80 rounded transition-colors"
      title="Reset View"
    >
      <RefreshCw class="w-4 h-4" />
    </button>
  </div>

  <!-- Legend Overlay -->
  <div class="absolute top-4 left-4 bg-slate-800/85 backdrop-blur border border-slate-700/80 px-3 py-2 rounded-lg text-xs space-y-1.5 shadow-md">
    <div class="font-medium text-slate-300 flex items-center gap-1.5 mb-1">
      <Layers class="w-3.5 h-3.5 text-blue-400" />
      Status Legend
    </div>
    <div class="flex items-center space-x-2">
      <span class="w-2.5 h-2.5 rounded-full bg-emerald-500"></span>
      <span class="text-slate-300">Normal</span>
    </div>
    <div class="flex items-center space-x-2">
      <span class="w-2.5 h-2.5 rounded-full bg-amber-500"></span>
      <span class="text-slate-300">Warning</span>
    </div>
    <div class="flex items-center space-x-2">
      <span class="w-2.5 h-2.5 rounded-full bg-red-500"></span>
      <span class="text-slate-300">Error / Down</span>
    </div>
  </div>
</div>
