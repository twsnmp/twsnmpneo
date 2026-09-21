import P5 from "p5";
import { getIconCode, getStateColor } from "../common";
import {
  fetchNodes,
  saveNode,
  deleteNode,
  fetchLines,
  saveLine,
  deleteLine,
  fetchDrawItems,
  saveDrawItem,
  deleteDrawItem,
  fetchNetworks,
  saveNetwork,
  deleteNetwork,
  fetchMapConf,
  saveMapConf,
  type NodeEnt,
  type LineEnt,
  type NetworkEnt,
  type DrawItemEnt,
} from "../api";
import { gauge, line, bar, kpi } from "./chart/drawitem";
import portImgUrl from "../../assets/images/port.png";

let mapSizeX = 2500;
let mapSizeY = 5000;
let mapRedraw = true;
let readOnly = false;

let mapCallBack: any = undefined;

let nodes: Record<string, NodeEnt> = {};
let lines: LineEnt[] = [];
let items: Record<string, DrawItemEnt> = {};
let networks: Record<string, any> = {};
let backImage: any = {
  X: 0,
  Y: 0,
  Width: 0,
  Height: 0,
  Path: "",
};
let _backImage: any = undefined;

let fontSize = 12;
let iconSize = 32;

const selectedNodes: string[] = [];
const selectedDrawItems: string[] = [];
let selectedNetwork = "";

const imageMap = new Map<string, any>();
let mapState = 0;
let editDrawItems = false;
let showNodeInfo = false;

let _mapP5: P5 | undefined = undefined;
let scale = 1.0;
let mapConf: any = undefined;
let portImage: any = undefined;

const isDark = (): boolean => {
  return document.documentElement.classList.contains("dark");
};

export const initMAP = async (div: HTMLElement, cb: any) => {
  try {
    mapConf = await fetchMapConf();
  } catch {
    mapConf = { MapSize: 0, IconSize: 3 };
  }
  switch (mapConf?.MapSize) {
    case 1:
      mapSizeX = 2894;
      mapSizeY = 4093;
      break;
    case 2:
      mapSizeX = 4093;
      mapSizeY = 2894;
      break;
    default:
      mapSizeX = window.screen.width > 4000 ? 5000 : 2500;
      mapSizeY = 5000;
  }

  mapCallBack = cb;
  readOnly = false;
  mapRedraw = false;
  if (_mapP5 != undefined) {
    return;
  }
  div.oncontextmenu = (e) => {
    e.preventDefault();
  };
  if (typeof document !== "undefined" && (document as any).fonts) {
    (document as any).fonts.ready.then(() => {
      mapRedraw = true;
    });
  }
  _mapP5 = new P5(mapMain, div);
};

export const resetMap = () => {
  if (_mapP5) {
    _mapP5.remove();
    _mapP5 = undefined;
  }
};

export const getMapSize = () => ({
  width: mapSizeX,
  height: mapSizeY,
});

export const getNodeBounds = () => {
  const halfW = Math.max(Math.round((iconSize + 24) / 2), 36);
  const topH = Math.max(Math.round(iconSize / 2 + 14), 32);
  const bottomH = Math.max(Math.round(iconSize / 2 + fontSize * 2 + 24), 56);
  return { halfW, topH, bottomH };
};

export const checkNodePos = (n: any) => {
  const { halfW, topH, bottomH } = getNodeBounds();
  const curX = typeof n.x === "number" ? n.x : (typeof n.X === "number" ? n.X : halfW);
  const curY = typeof n.y === "number" ? n.y : (typeof n.Y === "number" ? n.Y : topH);

  let nx = curX;
  let ny = curY;

  if (nx < halfW) nx = halfW;
  if (nx > mapSizeX - halfW) nx = mapSizeX - halfW;
  if (ny < topH) ny = topH;
  if (ny > mapSizeY - bottomH) ny = mapSizeY - bottomH;

  n.x = Math.round(nx);
  n.X = n.x;
  n.y = Math.round(ny);
  n.Y = n.y;
};

export const checkItemPos = (i: any) => {
  const iw = i.w || (i as any).W || 120;
  const ih = i.h || (i as any).H || 40;
  const curX = typeof i.x === "number" ? i.x : (typeof i.X === "number" ? i.X : 8);
  const curY = typeof i.y === "number" ? i.y : (typeof i.Y === "number" ? i.Y : 8);

  let ix = curX;
  let iy = curY;

  if (ix < 8) ix = 8;
  if (iy < 8) iy = 8;
  if (ix > mapSizeX - iw - 8) ix = mapSizeX - iw - 8;
  if (iy > mapSizeY - ih - 8) iy = mapSizeY - ih - 8;

  i.x = Math.round(ix);
  i.X = i.x;
  i.y = Math.round(iy);
  i.Y = i.y;
};

export const checkNetworkPos = (net: any) => {
  const nw = net.w || (net as any).W || 320;
  const nh = net.h || (net as any).H || 140;
  const curX = typeof net.x === "number" ? net.x : (typeof net.X === "number" ? net.X : 8);
  const curY = typeof net.y === "number" ? net.y : (typeof net.Y === "number" ? net.Y : 8);

  let nx = curX;
  let ny = curY;

  if (nx < 8) nx = 8;
  if (ny < 8) ny = 8;
  if (nx > mapSizeX - nw - 8) nx = mapSizeX - nw - 8;
  if (ny > mapSizeY - nh - 8) ny = mapSizeY - nh - 8;

  net.x = Math.round(nx);
  net.X = net.x;
  net.y = Math.round(ny);
  net.Y = net.y;
};

export const updateMAP = async () => {
  if (!_mapP5) return;
  const dark = isDark();
  if (!mapConf) {
    try {
      mapConf = await fetchMapConf();
    } catch {
      mapConf = { IconSize: 3 };
    }
  }
  const z = mapConf.IconSize || 3;
  iconSize = 8 + z * 8;
  fontSize = 6 + z * 2;

  try {
    const nodeList = await fetchNodes();
    nodes = {};
    nodeList.forEach((n) => {
      const id = n.id || (n as any).ID || '';
      if (id) {
        checkNodePos(n);
        nodes[id] = n;
      }
    });

    lines = await fetchLines();

    const itemList = await fetchDrawItems();
    items = {};
    itemList.forEach((item) => {
      const id = item.id || (item as any).ID || '';
      if (id) {
        checkItemPos(item);
        items[id] = item;
      }
    });

    const netList = await fetchNetworks();
    networks = {};
    netList.forEach((net) => {
      const id = net.id || (net as any).ID || '';
      if (id) {
        checkNetworkPos(net);
        networks[id] = net;
      }
    });
  } catch (e) {
    console.error("Failed to fetch map elements:", e);
  }

  if (!portImage && _mapP5) {
    portImage = _mapP5.loadImage(portImgUrl, () => {
      mapRedraw = true;
    });
  }

  _setMapState();

  const backColor = dark ? "rgb(23,23,23)" : "rgb(252,252,252)";

  for (const k in items) {
    const it = items[k];
    switch (it.type) {
      case 6: {
        const dataUrl = gauge(it.text || "", it.value || 0, backColor);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 7: {
        const dataUrl = bar(it.text || "", it.color || "white", it.value || 0, backColor);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 8: {
        const dataUrl = line(it.text || "", it.color || "white", it.values || [], backColor);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 11: {
        const kpiW = (it.w || 0) > 0 ? it.w! : 220;
        const kpiH = (it.h || 0) > 0 ? it.h! : 84;
        it.w = kpiW;
        it.h = kpiH;
        let title = it.text || "";
        if (title.includes("\t")) title = title.split("\t")[0];
        let text = (it as any).formatted_text || "";
        if (!text && it.value !== undefined) text = it.value.toFixed(1);
        const dataUrl = kpi(title, text, it.value || 0, it.color || "#00d2ff", it.values || [], dark, it.w, it.h);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
    }
  }

  mapRedraw = true;
};

export const zoom = (zoomin: boolean) => {
  scale += zoomin ? 0.05 : -0.05;
  if (scale > 3.0) scale = 3.0;
  else if (scale < 0.05) scale = 0.05;
  mapRedraw = true;
};

const _setMapState = () => {
  mapState = 0;
  for (const id in nodes) {
    switch (nodes[id].state) {
      case "high":
      case "error":
        mapState = 2;
        return;
      case "low":
      case "warn":
        mapState = 1;
        break;
    }
  }
};

export const setMapReadOnly = (ro: boolean) => {
  readOnly = ro;
};

export const setShowNodeInfo = (s: boolean) => {
  showNodeInfo = s;
  mapRedraw = true;
};

export const setEditDrawItems = (e: boolean) => {
  editDrawItems = e;
  mapRedraw = true;
};

const getLinePos = (id: string, polling: string) => {
  if (id.startsWith("NET:")) {
    const a = id.split(":");
    if (a.length !== 2) return undefined;
    const net = networks[a[1]];
    if (!net) return undefined;
    const ports = net.ports || net.Ports || [];
    let pi = -1;
    for (let i = 0; i < ports.length; i++) {
      if ((ports[i].id || ports[i].ID) === polling) {
        pi = i;
        break;
      }
    }
    if (pi < 0) return undefined;
    const px = (ports[pi].x ?? ports[pi].X ?? 0) * 45 + 10 + 20;
    const py = (ports[pi].y ?? ports[pi].Y ?? 0) * 55 + fontSize + 20 + 10;
    return {
      X: (net.x ?? net.X ?? 0) + px,
      Y: (net.y ?? net.Y ?? 0) + py,
    };
  }
  if (!nodes[id]) return undefined;
  return {
    X: nodes[id].x ?? (nodes[id] as any).X ?? 0,
    Y: (nodes[id].y ?? (nodes[id] as any).Y ?? 0) + 6,
  };
};

export const horizontal = async (selected: string[]) => {
  if (!selected || selected.length < 2) return;
  selected.sort((a, b) => {
    const ax = nodes[a]?.x ?? (nodes[a] as any)?.X ?? 0;
    const bx = nodes[b]?.x ?? (nodes[b] as any)?.X ?? 0;
    return ax - bx;
  });
  const id0 = selected[0];
  const x0 = nodes[id0]?.x ?? (nodes[id0] as any)?.X ?? 0;
  const x1 = nodes[selected[1]]?.x ?? (nodes[selected[1]] as any)?.X ?? 0;
  let dx = x1 - x0;
  if (dx < 60) dx = 60;
  let prevX = x0;
  const y0 = nodes[id0]?.y ?? (nodes[id0] as any)?.Y ?? 0;

  for (let i = 1; i < selected.length; i++) {
    const id = selected[i];
    if (nodes[id]) {
      nodes[id].y = y0;
      nodes[id].Y = y0;
      prevX = prevX + dx;
      nodes[id].x = prevX;
      nodes[id].X = prevX;
      checkNodePos(nodes[id]);
      await saveNode(nodes[id]).catch(console.error);
    }
  }
  mapRedraw = true;
};

export const vertical = async (selected: string[]) => {
  if (!selected || selected.length < 2) return;
  selected.sort((a, b) => {
    const ay = nodes[a]?.y ?? (nodes[a] as any)?.Y ?? 0;
    const by = nodes[b]?.y ?? (nodes[b] as any)?.Y ?? 0;
    return ay - by;
  });
  const id0 = selected[0];
  const y0 = nodes[id0]?.y ?? (nodes[id0] as any)?.Y ?? 0;
  const y1 = nodes[selected[1]]?.y ?? (nodes[selected[1]] as any)?.Y ?? 0;
  let dy = y1 - y0;
  if (dy < 60) dy = 60;
  let prevY = y0;
  const x0 = nodes[id0]?.x ?? (nodes[id0] as any)?.X ?? 0;

  for (let i = 1; i < selected.length; i++) {
    const id = selected[i];
    if (nodes[id]) {
      nodes[id].x = x0;
      nodes[id].X = x0;
      prevY = prevY + dy;
      nodes[id].y = prevY;
      nodes[id].Y = prevY;
      checkNodePos(nodes[id]);
      await saveNode(nodes[id]).catch(console.error);
    }
  }
  mapRedraw = true;
};

export const circle = async (selected: string[]) => {
  if (!selected || selected.length < 2) return;
  selected.sort((a, b) => {
    const ax = nodes[a]?.x ?? (nodes[a] as any)?.X ?? 0;
    const bx = nodes[b]?.x ?? (nodes[b] as any)?.X ?? 0;
    return ax - bx;
  });
  const c = 80 * selected.length;
  const r = Math.min(Math.trunc(c / Math.PI / 2), mapSizeX / 2 - 80);
  const cx = (nodes[selected[0]]?.x ?? (nodes[selected[0]] as any)?.X ?? 0) + r;
  let cy = nodes[selected[0]]?.y ?? (nodes[selected[0]] as any)?.Y ?? 0;
  if (cy - r < 0) cy = 40 + r;

  for (let i = 0; i < selected.length; i++) {
    const id = selected[i];
    if (nodes[id]) {
      const d = 180 - i * (360 / selected.length);
      const a = (d * Math.PI) / 180;
      const nx = Math.trunc(r * Math.cos(a) + cx);
      const ny = Math.trunc(r * Math.sin(a) + cy);
      nodes[id].x = nx;
      nodes[id].X = nx;
      nodes[id].y = ny;
      nodes[id].Y = ny;
      checkNodePos(nodes[id]);
      await saveNode(nodes[id]).catch(console.error);
    }
  }
  mapRedraw = true;
};

const mapMain = (p5: P5) => {
  let startMouseX = 0;
  let startMouseY = 0;
  let lastMouseX = 0;
  let lastMouseY = 0;
  let dragMode = 0; // 0: None, 1: Select box, 2: Move elements, 3: Selection finished
  let oldDark = isDark();
  let draggedNetwork = "";
  const draggedNodes: string[] = [];
  const draggedItems: string[] = [];
  let clickInCanvas = false;

  p5.setup = () => {
    const c = p5.createCanvas(mapSizeX, mapSizeY);
    c.mousePressed(canvasMousePressed);
    c.elt.addEventListener("contextmenu", handleCanvasContextMenu);
    const parentEl = (p5 as any)._userNode as HTMLElement | undefined;
    if (parentEl) {
      parentEl.addEventListener("contextmenu", handleCanvasContextMenu);
    }
    p5.frameRate(30);
    p5.strokeWeight(1);
    p5.textFont("Roboto, sans-serif");
    updateMAP();
  };

  const handleCanvasContextMenu = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (readOnly) return false;

    const canvasEl = ((p5 as any).canvas as HTMLCanvasElement) || ((p5 as any)._userNode?.querySelector("canvas") as HTMLCanvasElement);
    const rect = canvasEl ? canvasEl.getBoundingClientRect() : { left: 0, top: 0 };
    const clickX = e.clientX - rect.left;
    const clickY = e.clientY - rect.top;
    const mx = Math.max(0, Math.min(mapSizeX, Math.round(clickX / scale)));
    const my = Math.max(0, Math.min(mapSizeY, Math.round(clickY / scale)));

    // 1. Multiple nodes selected: format menu
    if (selectedNodes.length > 1) {
      if (mapCallBack) {
        mapCallBack({
          type: "formatNodes",
          Cmd: "formatNodes",
          Nodes: [...selectedNodes],
          x: e.clientX,
          y: e.clientY,
          mapX: mx,
          mapY: my,
        });
      }
      return false;
    }

    // 2. Hit test single node
    let hitNodeId = "";
    for (const k in nodes) {
      const n = nodes[k];
      const nid = n.id || (n as any).ID;
      const nx = n.x ?? (n as any).X ?? 0;
      const ny = n.y ?? (n as any).Y ?? 0;
      const r = Math.max(iconSize / 2 + 10, 24);
      if (mx >= nx - r && mx <= nx + r && my >= ny - r && my <= ny + r) {
        hitNodeId = nid;
        break;
      }
    }

    if (hitNodeId) {
      selectedNodes.length = 0;
      selectedNodes.push(hitNodeId);
      selectedDrawItems.length = 0;
      selectedNetwork = "";
      mapRedraw = true;
      if (mapCallBack) {
        mapCallBack({
          type: "contextmenu",
          Cmd: "contextMenu",
          Node: hitNodeId,
          nodeId: hitNodeId,
          DrawItem: "",
          itemId: "",
          Network: "",
          networkId: "",
          x: e.clientX,
          y: e.clientY,
          mapX: mx,
          mapY: my,
        });
      }
      return false;
    }

    // 3. Hit test draw items
    let hitItemId = "";
    for (const k in items) {
      const it = items[k];
      const iid = it.id || (it as any).ID;
      const ix = it.x ?? (it as any).X ?? 0;
      const iy = it.y ?? (it as any).Y ?? 0;
      const iw = it.w || (it as any).W || 120;
      const ih = it.h || (it as any).H || 40;
      if (mx >= ix && mx <= ix + iw && my >= iy && my <= iy + ih) {
        hitItemId = iid;
        break;
      }
    }

    if (hitItemId) {
      selectedDrawItems.length = 0;
      selectedDrawItems.push(hitItemId);
      selectedNodes.length = 0;
      selectedNetwork = "";
      mapRedraw = true;
      if (mapCallBack) {
        mapCallBack({
          type: "contextmenu",
          Cmd: "contextMenu",
          Node: "",
          nodeId: "",
          DrawItem: hitItemId,
          itemId: hitItemId,
          Network: "",
          networkId: "",
          x: e.clientX,
          y: e.clientY,
          mapX: mx,
          mapY: my,
        });
      }
      return false;
    }

    // 4. Hit test SW-HUB networks
    let hitNetId = "";
    for (const k in networks) {
      const net = networks[k];
      const nid = net.id || (net as any).ID;
      const nx = net.x ?? (net as any).X ?? 0;
      const ny = net.y ?? (net as any).Y ?? 0;
      const nw = net.w || (net as any).W || 320;
      const nh = net.h || (net as any).H || 140;
      if (mx >= nx && mx <= nx + nw && my >= ny && my <= ny + nh) {
        hitNetId = nid;
        break;
      }
    }

    if (hitNetId) {
      selectedNetwork = hitNetId;
      selectedNodes.length = 0;
      selectedDrawItems.length = 0;
      mapRedraw = true;
      if (mapCallBack) {
        mapCallBack({
          type: "contextmenu",
          Cmd: "contextMenu",
          Node: "",
          nodeId: "",
          DrawItem: "",
          itemId: "",
          Network: hitNetId,
          networkId: hitNetId,
          x: e.clientX,
          y: e.clientY,
          mapX: mx,
          mapY: my,
        });
      }
      return false;
    }

    // 5. Empty map canvas
    selectedNodes.length = 0;
    selectedDrawItems.length = 0;
    selectedNetwork = "";
    mapRedraw = true;
    if (mapCallBack) {
      mapCallBack({
        type: "contextmenu",
        Cmd: "contextMenu",
        Node: "",
        nodeId: "",
        DrawItem: "",
        itemId: "",
        Network: "",
        networkId: "",
        x: e.clientX,
        y: e.clientY,
        mapX: mx,
        mapY: my,
      });
    }
    return false;
  };

  p5.draw = () => {
    const dark = isDark();
    if (dark !== oldDark) {
      mapRedraw = true;
      oldDark = dark;
    }
    if (!mapRedraw) return;
    mapRedraw = false;

    p5.clear();
    p5.background(dark ? p5.color(11, 19, 41) : 252);

    p5.push();
    if (scale !== 1.0) {
      p5.scale(scale);
    }

    // Draw lines
    drawLines(p5);
    // Draw SW-HUB networks
    drawNetworks(p5, dark);
    // Draw draw items
    drawItems(p5, dark);
    // Draw nodes
    drawNodes(p5, dark);

    // Draw selection box in dragMode 1
    if (dragMode === 1) {
      const curX = lastMouseX;
      const curY = lastMouseY;
      const x = Math.min(startMouseX, curX);
      const y = Math.min(startMouseY, curY);
      const w = Math.abs(curX - startMouseX);
      const h = Math.abs(curY - startMouseY);

      p5.push();
      p5.fill("rgba(6, 182, 212, 0.15)");
      p5.stroke("#06b6d4");
      p5.strokeWeight(1.5);
      p5.rect(x, y, w, h, 4);
      p5.pop();
    }

    p5.pop();
  };

  const drawNetworks = (p5: P5, dark: boolean) => {
    for (const k in networks) {
      const net = networks[k];
      p5.push();
      const nx = net.x ?? (net as any).X ?? 0;
      const ny = net.y ?? (net as any).Y ?? 0;
      p5.translate(nx, ny);

      const netId = net.id || (net as any).ID;
      const nw = net.w || (net as any).W || 320;
      const nh = net.h || (net as any).H || 140;

      if (selectedNetwork === netId) {
        p5.stroke("#06b6d4");
        p5.strokeWeight(2);
      } else if (net.error || (net as any).Error) {
        p5.stroke("#ef4444");
        p5.strokeWeight(1.5);
      } else {
        p5.stroke("#334155");
        p5.strokeWeight(1);
      }
      p5.fill(dark ? "rgba(15, 23, 42, 0.9)" : "rgba(243,244,246,0.9)");
      p5.rect(0, 0, nw, nh, 8);

      p5.stroke("#9ca3af");
      p5.strokeWeight(1);
      p5.textFont("Roboto, sans-serif");
      p5.textSize(fontSize);
      p5.fill(dark ? "#f1f5f9" : "#1e293b");
      p5.text(net.name || (net as any).Name || "SW-HUB", 10, fontSize + 8);

      const ports = net.ports || (net as any).Ports || [];
      const netError = net.error || (net as any).Error || "";
      if (ports.length < 1) {
        p5.fill(netError ? "#ef4444" : "#10b981");
        p5.text(netError ? netError : "SW-HUB (No ports)", 15, fontSize * 2 + 15);
      } else {
        p5.textSize(8);
        for (const pt of ports) {
          const px = ((pt.x ?? pt.X) || 0) * 45 + 10;
          const py = ((pt.y ?? pt.Y) || 0) * 55 + fontSize + 15;
          if (portImage && portImage.width > 0) {
            p5.image(portImage, px, py, 40, 40);
          } else {
            // RJ45 port socket graphic
            p5.stroke(dark ? "#475569" : "#94a3b8");
            p5.strokeWeight(1);
            p5.fill(dark ? "#1e293b" : "#e2e8f0");
            p5.rect(px, py, 38, 38, 4);
            p5.fill(dark ? "#0f172a" : "#64748b");
            p5.rect(px + 6, py + 8, 26, 22, 2);
          }
          const st = (pt.state || pt.State || "none").toLowerCase();
          p5.noStroke();
          p5.fill(st === "up" ? "#10b981" : "#64748b");
          p5.circle(px + 8, py + 8, 7);
          p5.fill(dark ? "#f1f5f9" : "#1e293b");
          p5.text(pt.name || pt.Name || "", px + 4, py + 40 + 8);
        }
      }
      p5.pop();
    }
  };

  const drawLines = (p5: P5) => {
    for (const l of lines) {
      const p1 = getLinePos(l.node_id1, (l as any).polling_id1 || "");
      const p2 = getLinePos(l.node_id2, (l as any).polling_id2 || "");
      if (!p1 || !p2) continue;

      p5.push();
      const stColor = getStateColor(l.state || "normal");
      p5.stroke(stColor);
      p5.strokeWeight(l.width || 2);
      p5.line(p1.X, p1.Y, p2.X, p2.Y);

      // Packet animation dots
      const t = (p5.frameCount % 40) / 40;
      const dotX = p1.X + (p2.X - p1.X) * t;
      const dotY = p1.Y + (p2.Y - p1.Y) * t;
      p5.noStroke();
      p5.fill(stColor);
      p5.circle(dotX, dotY, (l.width || 2) + 4);
      p5.pop();
    }
  };

  const drawItems = (p5: P5, dark: boolean) => {
    for (const k in items) {
      const it = items[k];
      const iid = it.id || (it as any).ID || "";
      const ix = it.x ?? (it as any).X ?? 0;
      const iy = it.y ?? (it as any).Y ?? 0;
      const iw = it.w || (it as any).W || 120;
      const ih = it.h || (it as any).H || 40;

      p5.push();
      p5.translate(ix, iy);

      if (selectedDrawItems.includes(iid)) {
        p5.stroke("#06b6d4");
        p5.strokeWeight(2);
        p5.fill(dark ? "rgba(6, 182, 212, 0.1)" : "rgba(6, 182, 212, 0.15)");
        p5.rect(-4, -4, iw + 8, ih + 8, 6);
      }

      if (imageMap.has(k)) {
        const img = imageMap.get(k);
        p5.image(img, 0, 0, iw, ih);
      } else {
        // Simple shape/text fallback
        if (it.type === 2) {
          p5.fill(it.color || (dark ? "#f3f4f6" : "#1f2937"));
          p5.textSize(it.size || 14);
          p5.text(it.text || "", 0, it.size || 14);
        } else {
          p5.stroke(it.color || "#6b7280");
          p5.fill("rgba(100,100,100,0.1)");
          p5.rect(0, 0, iw, ih, 4);
          if (it.text) {
            p5.fill(dark ? "#f3f4f6" : "#1f2937");
            p5.text(it.text, 5, 15);
          }
        }
      }
      p5.pop();
    }
  };

  const drawNodes = (p5: P5, dark: boolean) => {
    for (const k in nodes) {
      const n = nodes[k];
      const nid = n.id || (n as any).ID || "";
      const nx = n.x ?? (n as any).X ?? 0;
      const ny = n.y ?? (n as any).Y ?? 0;
      const nname = n.name || (n as any).Name || "Node";
      const nip = n.ip || (n as any).IP || "";
      const nstate = n.state || (n as any).State || "normal";
      const nicon = n.icon || (n as any).Icon || "desktop";
      const nimage = n.image || (n as any).Image || "";
      const icon = getIconCode(nicon);

      p5.push();
      p5.translate(nx, ny);

      const isSelected = selectedNodes.includes(nid);
      const stColor = getStateColor(nstate);

      if (isSelected) {
        p5.stroke("#06b6d4");
        p5.strokeWeight(2);
        p5.fill(dark ? "rgba(15, 23, 42, 0.85)" : "rgba(243,244,246,0.85)");
        const selW = iconSize + 20;
        const selH = iconSize + 16 + fontSize + (showNodeInfo ? fontSize : 0);
        p5.rect(-selW / 2, -iconSize / 2 - 8, selW, selH, 6);
      }

      if (nimage && imageMap.has(nimage)) {
        const img = imageMap.get(nimage);
        p5.tint(stColor);
        p5.image(img, -iconSize / 2, -iconSize / 2, iconSize, iconSize);
        p5.noTint();
      } else {
        p5.textFont("Material Design Icons");
        p5.textAlign(p5.CENTER, p5.CENTER);
        p5.textSize(iconSize);
        p5.fill(stColor);
        p5.text(icon, 0, 0);
      }

      // Node label
      p5.textFont("Roboto, sans-serif");
      p5.textAlign(p5.CENTER, p5.CENTER);
      p5.textSize(fontSize);
      p5.fill(dark ? 250 : 23);
      p5.text(nname, 0, iconSize / 2 + 8 + fontSize / 2);

      if (showNodeInfo && nip) {
        p5.textSize(Math.max(fontSize - 2, 8));
        p5.text(nip, 0, iconSize / 2 + 8 + fontSize + fontSize / 2);
      }

      p5.pop();
    }
  };

  const dragMoveNodes = () => {
    const curX = p5.mouseX / scale;
    const curY = p5.mouseY / scale;
    const dx = Math.trunc(curX - lastMouseX);
    const dy = Math.trunc(curY - lastMouseY);
    if (dx === 0 && dy === 0) return;

    selectedNodes.forEach((id: string) => {
      if (nodes[id]) {
        nodes[id].x = (nodes[id].x ?? (nodes[id] as any).X ?? 0) + dx;
        nodes[id].y = (nodes[id].y ?? (nodes[id] as any).Y ?? 0) + dy;
        checkNodePos(nodes[id]);
        if (!draggedNodes.includes(id)) {
          draggedNodes.push(id);
        }
      }
    });

    selectedDrawItems.forEach((id: string) => {
      if (items[id]) {
        items[id].x = (items[id].x ?? (items[id] as any).X ?? 0) + dx;
        items[id].y = (items[id].y ?? (items[id] as any).Y ?? 0) + dy;
        checkItemPos(items[id]);
        if (!draggedItems.includes(id)) {
          draggedItems.push(id);
        }
      }
    });

    if (selectedNetwork !== "" && networks[selectedNetwork]) {
      networks[selectedNetwork].x = (networks[selectedNetwork].x ?? (networks[selectedNetwork] as any).X ?? 0) + dx;
      networks[selectedNetwork].y = (networks[selectedNetwork].y ?? (networks[selectedNetwork] as any).Y ?? 0) + dy;
      checkNetworkPos(networks[selectedNetwork]);
      draggedNetwork = selectedNetwork;
    }
    mapRedraw = true;
  };

  const dragSelectNodes = () => {
    selectedNodes.length = 0;
    const curX = p5.mouseX / scale;
    const curY = p5.mouseY / scale;
    const sx = Math.min(startMouseX, curX);
    const sy = Math.min(startMouseY, curY);
    const lx = Math.max(startMouseX, curX);
    const ly = Math.max(startMouseY, curY);

    for (const k in nodes) {
      const nx = nodes[k].x ?? (nodes[k] as any).X ?? 0;
      const ny = nodes[k].y ?? (nodes[k] as any).Y ?? 0;
      if (nx > sx && nx < lx && ny > sy && ny < ly) {
        selectedNodes.push(nodes[k].id || (nodes[k] as any).ID);
      }
    }
    selectedDrawItems.length = 0;
    for (const k in items) {
      const ix = items[k].x ?? (items[k] as any).X ?? 0;
      const iy = items[k].y ?? (items[k] as any).Y ?? 0;
      if (ix > sx && ix < lx && iy > sy && iy < ly) {
        selectedDrawItems.push(items[k].id || (items[k] as any).ID);
      }
    }
    mapRedraw = true;
  };

  const setSelectNode = (bMulti: boolean) => {
    const l = selectedNodes.length;
    const x = p5.mouseX / scale;
    const y = p5.mouseY / scale;
    for (const k in nodes) {
      const n = nodes[k];
      const nid = n.id || (n as any).ID;
      const nx = n.x ?? (n as any).X ?? 0;
      const ny = n.y ?? (n as any).Y ?? 0;
      const r = Math.max(iconSize / 2 + 10, 24);
      if (x >= nx - r && x <= nx + r && y >= ny - r && y <= ny + r) {
        if (selectedNodes.includes(nid)) {
          if (bMulti) {
            const idx = selectedNodes.indexOf(nid);
            selectedNodes.splice(idx, 1);
          }
          return true;
        }
        if (!bMulti) {
          selectedNodes.length = 0;
        }
        selectedNodes.push(nid);
        return true;
      }
    }
    if (!bMulti) {
      selectedNodes.length = 0;
    }
    return l !== selectedNodes.length;
  };

  const setSelectItem = () => {
    const x = p5.mouseX / scale;
    const y = p5.mouseY / scale;
    for (const k in items) {
      const it = items[k];
      const iid = it.id || (it as any).ID;
      const ix = it.x ?? (it as any).X ?? 0;
      const iy = it.y ?? (it as any).Y ?? 0;
      const iw = it.w || (it as any).W || 120;
      const ih = it.h || (it as any).H || 40;
      if (x >= ix - 5 && x <= ix + iw + 5 && y >= iy - 5 && y <= iy + ih + 5) {
        if (selectedDrawItems.includes(iid)) {
          return true;
        }
        selectedDrawItems.length = 0;
        selectedDrawItems.push(iid);
        return true;
      }
    }
    selectedDrawItems.length = 0;
    return false;
  };

  const setSelectNetwork = (second: boolean = false) => {
    const x = p5.mouseX / scale;
    const y = p5.mouseY / scale;
    for (const k in networks) {
      const net = networks[k];
      const nid = net.id || (net as any).ID;
      const nx = net.x ?? (net as any).X ?? 0;
      const ny = net.y ?? (net as any).Y ?? 0;
      const nw = net.w || (net as any).W || 320;
      const nh = net.h || (net as any).H || 140;
      if (x >= nx && x <= nx + nw && y >= ny && y <= ny + nh) {
        selectedNetwork = nid;
        return true;
      }
    }
    selectedNetwork = "";
    return false;
  };

  const canvasMousePressed = (e?: MouseEvent) => {
    if (readOnly) return true;
    if (p5.mouseButton === p5.RIGHT || (e && (e.button === 2 || e.which === 3))) {
      return false;
    }
    clickInCanvas = true;
    mapRedraw = true;

    const mx = p5.mouseX / scale;
    const my = p5.mouseY / scale;

    const isMulti = p5.keyIsDown && (p5.keyIsDown(p5.ALT) || (e && e.altKey) || p5.keyIsDown(p5.SHIFT) || (e && e.shiftKey));

    if (isMulti) {
      setSelectNode(true);
    } else if (dragMode === 3) {
      let hitSelected = false;
      for (const id of selectedNodes) {
        if (nodes[id]) {
          const nx = nodes[id].x ?? (nodes[id] as any).X ?? 0;
          const ny = nodes[id].y ?? (nodes[id] as any).Y ?? 0;
          const r = Math.max(iconSize / 2 + 10, 24);
          if (mx >= nx - r && mx <= nx + r && my >= ny - r && my <= ny + r) {
            hitSelected = true;
            break;
          }
        }
      }
      if (!hitSelected) {
        for (const id of selectedDrawItems) {
          if (items[id]) {
            const ix = items[id].x ?? (items[id] as any).X ?? 0;
            const iy = items[id].y ?? (items[id] as any).Y ?? 0;
            const iw = items[id].w || (items[id] as any).W || 120;
            const ih = items[id].h || (items[id] as any).H || 40;
            if (mx >= ix - 5 && mx <= ix + iw + 5 && my >= iy - 5 && my <= iy + ih + 5) {
              hitSelected = true;
              break;
            }
          }
        }
      }

      if (!hitSelected) {
        selectedNodes.length = 0;
        selectedDrawItems.length = 0;
        selectedNetwork = "";
        setSelectNode(false);
        setSelectItem();
        setSelectNetwork(false);
      }
    } else {
      setSelectNode(false);
      setSelectItem();
      setSelectNetwork(false);
    }

    lastMouseX = mx;
    lastMouseY = my;
    startMouseX = mx;
    startMouseY = my;
    dragMode = 0;
    return false;
  };

  p5.mouseDragged = () => {
    if (readOnly || !clickInCanvas) return true;
    if (p5.mouseButton === p5.RIGHT) return true;

    // Space key or middle click panning
    if (p5.mouseButton === p5.CENTER || (p5.keyIsDown && p5.keyIsDown(32))) {
      const parentEl = (p5 as any)._userNode as HTMLElement | undefined;
      if (parentEl) {
        parentEl.scrollLeft -= (p5.mouseX / scale - lastMouseX) * scale;
        parentEl.scrollTop -= (p5.mouseY / scale - lastMouseY) * scale;
      }
      lastMouseX = p5.mouseX / scale;
      lastMouseY = p5.mouseY / scale;
      return false;
    }

    if (dragMode === 0) {
      if (
        selectedNodes.length > 0 ||
        selectedDrawItems.length > 0 ||
        selectedNetwork !== ""
      ) {
        dragMode = 2; // Move mode
      } else {
        dragMode = 1; // Select box mode
      }
    }

    if (dragMode === 1) {
      lastMouseX = p5.mouseX / scale;
      lastMouseY = p5.mouseY / scale;
      dragSelectNodes();
    } else if (dragMode === 2) {
      dragMoveNodes();
      lastMouseX = p5.mouseX / scale;
      lastMouseY = p5.mouseY / scale;
    }
    return false;
  };

  p5.mouseReleased = (e?: MouseEvent) => {
    if (readOnly) return true;
    if (p5.mouseButton === p5.RIGHT || (e && (e.button === 2 || e.which === 3))) {
      return false;
    }
    mapRedraw = true;

    if (!clickInCanvas) {
      selectedNodes.length = 0;
      selectedDrawItems.length = 0;
      selectedNetwork = "";
      return true;
    }

    clickInCanvas = false;

    if (dragMode === 0) {
      return false;
    }

    if (dragMode === 1) {
      if (selectedNodes.length > 0 || selectedDrawItems.length > 0) {
        dragMode = 3;
      } else {
        dragMode = 0;
      }
      return false;
    }

    if (dragMode === 2) {
      if (draggedNodes.length > 0) {
        draggedNodes.forEach((id) => {
          if (nodes[id]) {
            saveNode(nodes[id]).catch(console.error);
          }
        });
        draggedNodes.length = 0;
      }
      if (draggedItems.length > 0) {
        draggedItems.forEach((id) => {
          if (items[id]) {
            saveDrawItem(items[id]).catch(console.error);
          }
        });
        draggedItems.length = 0;
      }
      if (draggedNetwork !== "" && networks[draggedNetwork]) {
        saveNetwork(networks[draggedNetwork]).catch(console.error);
        draggedNetwork = "";
      }
      dragMode = 0;
    }
    return false;
  };

  p5.doubleClicked = () => {
    if (selectedNodes.length === 1 && mapCallBack) {
      mapCallBack({
        type: "dblclick",
        Cmd: "nodeDoubleClicked",
        nodeId: selectedNodes[0],
        Param: selectedNodes[0],
      });
    } else if (selectedNetwork && mapCallBack) {
      mapCallBack({
        type: "dblclick",
        Cmd: "networkDoubleClicked",
        networkId: selectedNetwork,
        Param: selectedNetwork,
      });
    } else if (selectedDrawItems.length === 1 && mapCallBack) {
      mapCallBack({
        type: "dblclick",
        Cmd: "itemDoubleClicked",
        itemId: selectedDrawItems[0],
        Param: selectedDrawItems[0],
      });
    }
    return true;
  };

  p5.keyReleased = () => {
    if (readOnly) return true;
    if (p5.keyCode === p5.DELETE || p5.keyCode === p5.BACKSPACE) {
      if (selectedNodes.length > 0 && mapCallBack) {
        mapCallBack({
          Cmd: "deleteNodes",
          Param: [...selectedNodes],
        });
        selectedNodes.length = 0;
      } else if (selectedDrawItems.length > 0 && mapCallBack) {
        mapCallBack({
          Cmd: "deleteDrawItems",
          Param: [...selectedDrawItems],
        });
        selectedDrawItems.length = 0;
      } else if (selectedNetwork !== "" && mapCallBack) {
        mapCallBack({
          Cmd: "deleteNetwork",
          Param: selectedNetwork,
        });
        selectedNetwork = "";
      }
      return false;
    }
    if (p5.keyCode === p5.ENTER) {
      p5.doubleClicked();
      return false;
    }
    return true;
  };

  p5.mouseWheel = (event: WheelEvent) => {
    if (event.ctrlKey || event.metaKey) {
      if (event.deltaY < 0) zoom(true);
      else zoom(false);
      return false;
    }
    return true;
  };
};
