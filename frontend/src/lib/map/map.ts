import P5 from "p5";
import { get } from "svelte/store";
import { locale } from "svelte-i18n";
import { getIconCode, getStateColor, isImageIcon, getIconImage, setCustomIcons } from "../common";
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
  fetchBackImage,
  fetchCustomIcons,
  updateNodePositions,
  type NodeEnt,
  type LineEnt,
  type NetworkEnt,
  type DrawItemEnt,
} from "../api";
import { gauge, line, bar, kpi, classicGauge } from "./chart/drawitem";
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
let lastBackImagePath = "";

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
  lastBackImagePath = "";
  _backImage = undefined;
};

export const reloadBackImage = () => {
  lastBackImagePath = "";
  _backImage = undefined;
  mapRedraw = true;
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
  try {
    mapConf = await fetchMapConf();
  } catch {
    if (!mapConf) mapConf = { IconSize: 3, MapSize: 0 };
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
      mapSizeX = typeof window !== "undefined" && window.screen.width > 4000 ? 5000 : 2500;
      mapSizeY = 5000;
  }
  if (_mapP5 && (_mapP5.width !== mapSizeX || _mapP5.height !== mapSizeY)) {
    _mapP5.resizeCanvas(mapSizeX, mapSizeY);
  }
  const z = mapConf?.IconSize ?? (mapConf?.icon_size ?? 24);
  if (z <= 5) {
    iconSize = 8 + z * 8;
    fontSize = 6 + z * 2;
  } else {
    iconSize = z;
    fontSize = (mapConf as any)?.FontSize || (mapConf as any)?.font_size || Math.max(10, Math.min(16, Math.round(z * 0.45)));
  }

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

    try {
      const iconListRes = await fetchCustomIcons().catch(() => []);
      if (iconListRes && iconListRes.length > 0) {
        setCustomIcons(iconListRes);
      }
    } catch {
      // ignore
    }

    for (const k in nodes) {
      const imgKey = nodes[k].image || (nodes[k] as any).Image;
      if (imgKey && !imageMap.has(imgKey) && _mapP5) {
        const imgData = getIconImage(imgKey);
        if (imgData) {
          const img = _mapP5.loadImage(imgData, (loaded) => {
            imageMap.set(imgKey, loaded);
            mapRedraw = true;
          });
          imageMap.set(imgKey, img);
        }
      }
    }

    try {
      backImage = await fetchBackImage();
      // If backImage has no path but mapConf has BackImage metadata, fallback
      if (!backImage?.Path && (mapConf as any)?.BackImage?.Path) {
        backImage = (mapConf as any).BackImage;
      }
      if (backImage?.Path) {
        if ((backImage.Path !== lastBackImagePath || !_backImage) && _mapP5) {
          lastBackImagePath = backImage.Path;
          let imgUrl = backImage.Path;
          if (!imgUrl.startsWith("http") && !imgUrl.startsWith("/") && !imgUrl.startsWith("data:")) {
            imgUrl = `/api/map/image/${imgUrl}`;
          } else if (imgUrl === "/backimage") {
            imgUrl = `/api/map/image/backimage_map.png`;
          }
          imgUrl = `${imgUrl}?t=${Date.now()}`;
          _mapP5.loadImage(
            imgUrl,
            (img) => {
              _backImage = img;
              mapRedraw = true;
            },
            () => {
              if (_mapP5) {
                _mapP5.loadImage(
                  `/api/map/backimage/raw?t=${Date.now()}`,
                  (fallbackImg) => {
                    _backImage = fallbackImg;
                    mapRedraw = true;
                  },
                  () => {
                    _mapP5?.loadImage(`/backimage?t=${Date.now()}`, (fcImg) => {
                      _backImage = fcImg;
                      mapRedraw = true;
                    });
                  }
                );
              }
            }
          );
        } else {
          // Path unchanged, but position/size might have changed
          mapRedraw = true;
        }
      } else {
        lastBackImagePath = "";
        _backImage = undefined;
        mapRedraw = true;
      }
    } catch {
      // ignore
    }
  } catch (e) {
    console.error("Failed to fetch map elements:", e);
  }

  if (!portImage && _mapP5) {
    portImage = _mapP5.loadImage(portImgUrl, () => {
      mapRedraw = true;
    });
  }

  _setMapState();
  mapRedraw = true;

  const backColor = dark ? "rgb(23,23,23)" : "rgb(252,252,252)";

  for (const k in items) {
    const it = items[k];
    switch (it.type) {
      case 2:
      case 4: {
        const displayText = it.formatted_text || (it as any).FormattedText || it.text || (it as any).Text || (it.type === 4 ? "No Value" : "Empty");
        const textLen = Math.max(displayText.length, 4);
        const sz = it.size || 14;
        it.w = (it.w && it.w > 0) ? it.w : sz * textLen;
        it.h = (it.h && it.h > 0) ? it.h : sz;
        (it as any).W = it.w;
        (it as any).H = it.h;
        if (imageMap.has(k)) {
          imageMap.delete(k);
        }
        break;
      }
      case 3: {
        if (it.path && _mapP5) {
          const img = _mapP5.loadImage(it.path, (loaded) => {
            imageMap.set(k, loaded);
            mapRedraw = true;
          });
          if (!imageMap.has(k)) imageMap.set(k, img);
        }
        break;
      }
      case 5: {
        const sz = it.size || 16;
        it.w = sz * 10;
        it.h = sz * 10;
        (it as any).W = it.w;
        (it as any).H = it.h;
        const dataUrl = classicGauge(it.text || "", it.color || "#00d2ff", it.value || 0, sz, dark);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 6: {
        const gh = it.h || 120;
        it.h = gh;
        it.w = gh;
        (it as any).W = it.w;
        (it as any).H = it.h;
        const dataUrl = gauge(it.text || "", it.value || 0, backColor);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 7: {
        const bh = it.h || 80;
        it.h = bh;
        it.w = bh * 4;
        (it as any).W = it.w;
        (it as any).H = it.h;
        const dataUrl = bar(it.text || "", it.color || "white", it.value || 0, backColor);
        const img = _mapP5.loadImage(dataUrl, (loaded) => {
          imageMap.set(k, loaded);
          mapRedraw = true;
        });
        if (!imageMap.has(k)) imageMap.set(k, img);
        break;
      }
      case 8: {
        const lh = it.h || 80;
        it.h = lh;
        it.w = lh * 4;
        (it as any).W = it.w;
        (it as any).H = it.h;
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
        (it as any).W = it.w;
        (it as any).H = it.h;
        let title = it.text || "";
        if (title.includes("\t")) title = title.split("\t")[0];
        let text = (it as any).formatted_text || (it as any).FormattedText || "";
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

export const getShowNodeInfo = (): boolean => showNodeInfo;

export const setEditDrawItems = (e: boolean) => {
  editDrawItems = e;
  if (!editDrawItems) {
    selectedDrawItems.length = 0;
  }
  mapRedraw = true;
};

export const getEditDrawItems = (): boolean => editDrawItems;

const getLinePos = (id: string, polling: string) => {
  if (id.startsWith("NET:")) {
    const a = id.split(":");
    if (a.length !== 2) return undefined;
    const net = networks[a[1]];
    if (!net) return undefined;
    const ports = net.ports || (net as any).Ports || [];
    let pi = -1;
    if (polling) {
      for (let i = 0; i < ports.length; i++) {
        if ((ports[i].id || (ports[i] as any).ID) === polling) {
          pi = i;
          break;
        }
      }
    }
    // Fallback if port not specified or not found: use first port or center of network
    if (pi < 0) {
      if (ports.length > 0) {
        pi = 0;
      } else {
        const nw = net.w || (net as any).W || 320;
        const nh = net.h || (net as any).H || 140;
        return {
          X: (net.x ?? (net as any).X ?? 0) + nw / 2,
          Y: (net.y ?? (net as any).Y ?? 0) + nh / 2,
        };
      }
    }
    // Port center: port is 40x40 at (X * 45 + 10, Y * 55 + fontSize + 15)
    const px = ((ports[pi].x ?? (ports[pi] as any).X) || 0) * 45 + 10 + 20;
    const py = ((ports[pi].y ?? (ports[pi] as any).Y) || 0) * 55 + fontSize + 15 + 20;
    return {
      X: (net.x ?? (net as any).X ?? 0) + px,
      Y: (net.y ?? (net as any).Y ?? 0) + py,
    };
  }
  if (!nodes[id]) return undefined;
  return {
    X: nodes[id].x ?? (nodes[id] as any).X ?? 0,
    Y: (nodes[id].y ?? (nodes[id] as any).Y ?? 0) + 6,
  };
};

export const grid = async (g: number, test: boolean) => {
  const list: { ID: string; X: number; Y: number }[] = [];
  const mx = Math.ceil(mapSizeX / g);
  const my = Math.ceil(mapSizeY / g);
  const m = new Array(mx);
  for (let x = 0; x < m.length; x++) {
    m[x] = new Array(my);
    for (let y = 0; y < m[x].length; y++) {
      m[x][y] = false;
    }
  }
  for (const id in nodes) {
    const curX = nodes[id].x ?? (nodes[id] as any).X ?? 0;
    const curY = nodes[id].y ?? (nodes[id] as any).Y ?? 0;
    let x = Math.max(Math.min(Math.ceil((curX * 1.0) / g), mx - 1), 0);
    let y = Math.max(Math.min(Math.ceil((curY * 1.0) / g), my - 1), 0);
    while (m[x] && m[x][y]) {
      x++;
      if (x >= mx) {
        y++;
        x = 0;
        if (y >= my) {
          y = 0;
          break;
        }
      }
    }
    if (m[x]) {
      m[x][y] = true;
    }
    nodes[id].x = x * g;
    nodes[id].y = y * g;
    (nodes[id] as any).X = x * g;
    (nodes[id] as any).Y = y * g;
    list.push({
      ID: id,
      X: x * g,
      Y: y * g,
    });
  }
  if (!test && list.length > 0) {
    await updateNodePositions(list).catch(console.error);
  }
  mapRedraw = true;
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
      if (mx >= ix - 5 && mx <= ix + iw + 5 && my >= iy - 5 && my <= iy + ih + 5) {
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

    // 4. Hit test networks
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
    p5.background(dark ? p5.color(11, 19, 41) : p5.color(252));

    p5.push();
    if (scale !== 1.0) {
      p5.scale(scale);
    }

    // Draw background image if configured
    if (_backImage) {
      if (backImage.Width > 0 && backImage.Height > 0) {
        p5.image(_backImage, backImage.X || 0, backImage.Y || 0, backImage.Width, backImage.Height);
      } else {
        p5.image(_backImage, backImage.X || 0, backImage.Y || 0);
      }
    }

    // 1. Draw draw items (background layers)
    drawItems(p5, dark);
    // 2. Draw networks
    drawNetworks(p5, dark);
    // 3. Draw lines (rendered in front of networks into ports)
    drawLines(p5, dark);
    // 4. Draw nodes (rendered in front)
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
      const ports = net.ports || (net as any).Ports || [];
      let calcW = 200;
      let calcH = 80;
      if (ports.length > 0) {
        let xMax = 5;
        let yMax = 0;
        for (const pt of ports) {
          const px = (pt.x ?? pt.X) || 0;
          const py = (pt.y ?? pt.Y) || 0;
          if (xMax < px) xMax = px;
          if (yMax < py) yMax = py;
        }
        calcW = (xMax + 1) * 45 + 20;
        calcH = (yMax + 1) * 55 + fontSize + 25;
      }
      const nw = Math.max(net.w || (net as any).W || 0, calcW);
      const nh = Math.max(net.h || (net as any).H || 0, calcH);

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
      const isJa = (get(locale) || "ja").startsWith("ja");
      p5.text(net.name || (net as any).Name || (isJa ? "ネットワーク" : "Network"), 10, fontSize + 8);

      const netError = net.error || (net as any).Error || "";
      if (ports.length < 1) {
        p5.fill(netError ? "#ef4444" : "#10b981");
        p5.text(netError ? netError : (isJa ? "ネットワーク (ポートなし)" : "Network (No Ports)"), 15, fontSize * 2 + 15);
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

  const drawLines = (p5: P5, dark: boolean) => {
    for (const l of lines) {
      const nid1 = l.node_id1 || (l as any).NodeID1 || "";
      const pid1 = l.polling_id1 || (l as any).PollingID1 || (l as any).port || "";
      const nid2 = l.node_id2 || (l as any).NodeID2 || "";
      const pid2 = l.polling_id2 || (l as any).PollingID2 || "";
      const p1 = getLinePos(nid1, pid1);
      const p2 = getLinePos(nid2, pid2);
      if (!p1 || !p2) continue;

      const lw = l.width || (l as any).Width || 2;
      const stColor = getStateColor(l.state || "normal");
      const stColor1 = getStateColor(l.state1 || l.state || "normal");
      const stColor2 = getStateColor(l.state2 || l.state || "normal");
      const xm = (p1.X + p2.X) / 2;
      const ym = (p1.Y + p2.Y) / 2;

      p5.push();
      // Draw two line segments (half-and-half state color matching twsnmpfk)
      p5.strokeWeight(lw);
      p5.stroke(stColor1);
      p5.line(p1.X, p1.Y, xm, ym);
      p5.stroke(stColor2);
      p5.line(xm, ym, p2.X, p2.Y);

      // Packet animation dot
      const t = (p5.frameCount % 40) / 40;
      const dotX = p1.X + (p2.X - p1.X) * t;
      const dotY = p1.Y + (p2.Y - p1.Y) * t;
      p5.noStroke();
      p5.fill(stColor);
      p5.circle(dotX, dotY, lw + 4);

      // Port terminal jack indicator on network so connection is clearly visible
      if (nid1.startsWith("NET:")) {
        p5.noStroke();
        p5.fill(stColor1);
        p5.circle(p1.X, p1.Y, lw + 6);
        p5.fill("#38bdf8");
        p5.circle(p1.X, p1.Y, Math.max(3, lw));
      }
      if (nid2.startsWith("NET:")) {
        p5.noStroke();
        p5.fill(stColor2);
        p5.circle(p2.X, p2.Y, lw + 6);
        p5.fill("#38bdf8");
        p5.circle(p2.X, p2.Y, Math.max(3, lw));
      }

      // Line Info label if present
      const infoText = l.info || (l as any).Info;
      if (infoText) {
        p5.textSize(fontSize - 2);
        p5.fill(dark ? "#94a3b8" : "#475569");
        p5.text(infoText, xm + 6, ym - 6);
      }

      p5.pop();
    }
  };

  const drawItems = (p5: P5, dark: boolean) => {
    for (const k in items) {
      const it = items[k];
      if (it.cond && it.cond > 0 && mapState < it.cond && !editDrawItems) {
        continue;
      }
      const iid = it.id || (it as any).ID || "";
      const ix = it.x ?? (it as any).X ?? 0;
      const iy = it.y ?? (it as any).Y ?? 0;
      const iw = it.w || (it as any).W || 120;
      const ih = it.h || (it as any).H || 40;

      p5.push();
      p5.translate(ix, iy);

      if (editDrawItems && selectedDrawItems.includes(iid)) {
        p5.stroke("#06b6d4");
        p5.strokeWeight(2);
        p5.fill(dark ? "rgba(6, 182, 212, 0.1)" : "rgba(6, 182, 212, 0.15)");
        p5.rect(-4, -4, iw + 8, ih + 8, 6);
      }

      if (imageMap.has(k)) {
        const img = imageMap.get(k);
        p5.image(img, 0, 0, iw, ih);
      } else {
        switch (it.type) {
          case 0: // Rect
            p5.fill(it.color || "#06b6d4");
            p5.stroke(dark ? "rgba(255,255,255,0.15)" : "rgba(0,0,0,0.12)");
            p5.strokeWeight(1);
            p5.rect(0, 0, iw, ih, 6);
            break;
          case 1: // Ellipse
            p5.fill(it.color || "#06b6d4");
            p5.stroke(dark ? "rgba(255,255,255,0.15)" : "rgba(0,0,0,0.12)");
            p5.strokeWeight(1);
            p5.ellipse(iw / 2, ih / 2, iw, ih);
            break;
          case 2: // Label
          case 4: // Polling Text
            p5.textSize(it.size || 14);
            p5.fill(it.color || (dark ? "#f3f4f6" : "#1f2937"));
            p5.noStroke();
            p5.textAlign(p5.LEFT, p5.TOP);
            const textToDraw = it.formatted_text || (it as any).FormattedText || it.text || (it as any).Text || (it.type === 4 ? "No Value" : "Empty");
            p5.text(textToDraw, 0, 0);
            break;
          case 3: // Image fallback
            p5.fill("#888");
            p5.rect(0, 0, iw, ih, 4);
            break;
          case 9: // GroupFrame
            p5.fill("rgba(23,23,23,0.02)");
            p5.strokeWeight(2);
            p5.stroke(it.color || "#06b6d4");
            p5.rect(0, 0, iw, ih, 8);
            if (it.text) {
              p5.textSize(it.size || 12);
              p5.fill(dark ? "#eee" : "#333");
              p5.noStroke();
              p5.textAlign(p5.RIGHT, p5.BOTTOM);
              p5.text(it.text, iw - 8, ih - 8);
            }
            break;
          case 10: // GroupFill
            p5.fill(it.color || "#06b6d4");
            p5.stroke(dark ? "rgba(255,255,255,0.08)" : "rgba(0,0,0,0.06)");
            p5.strokeWeight(1);
            p5.rect(0, 0, iw, ih, 8);
            if (it.text) {
              p5.textSize(it.size || 12);
              p5.fill(dark ? "#eee" : "#333");
              p5.noStroke();
              p5.textAlign(p5.RIGHT, p5.BOTTOM);
              p5.text(it.text, iw - 8, ih - 8);
            }
            break;
          default:
            p5.stroke(it.color || "#6b7280");
            p5.fill("rgba(100,100,100,0.1)");
            p5.rect(0, 0, iw, ih, 4);
            if (it.text) {
              p5.fill(dark ? "#f3f4f6" : "#1f2937");
              p5.text(it.text, 5, 15);
            }
            break;
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

      if (nimage) {
        let img = imageMap.get(nimage);
        if (!img && _mapP5) {
          const imgData = getIconImage(nimage);
          if (imgData) {
            img = _mapP5.loadImage(imgData, (loaded) => {
              imageMap.set(nimage, loaded);
              mapRedraw = true;
            });
            imageMap.set(nimage, img);
          }
        }
        if (img && img.width > 0) {
          const iw = isSelected ? iconSize + 8 : iconSize;
          const ih = (img.height * iw) / img.width;
          const w = iw + 16;
          const h = ih + 16 + fontSize + (showNodeInfo ? fontSize : 0);
          if (isSelected) {
            p5.stroke("#06b6d4");
            p5.strokeWeight(2);
            p5.fill(dark ? "rgba(15, 23, 42, 0.85)" : "rgba(243,244,246,0.85)");
            p5.rect(-w / 2, -ih / 2 - 8, w, h, 6);
          }
          p5.tint(stColor);
          p5.image(img, -iw / 2, -ih / 2, iw, ih);
          p5.noTint();

          // Node label
          p5.textFont("Roboto, sans-serif");
          p5.textAlign(p5.CENTER, p5.CENTER);
          p5.textSize(fontSize);
          p5.fill(dark ? 250 : 23);
          p5.text(nname, 0, ih / 2 + 8 + fontSize / 2);

          if (showNodeInfo && nip) {
            p5.textSize(Math.max(fontSize - 2, 8));
            p5.text(nip, 0, ih / 2 + 8 + fontSize + fontSize / 2);
          }
        } else {
          // If image is still loading or unavailable, draw standard MDI icon
          if (isSelected) {
            p5.stroke("#06b6d4");
            p5.strokeWeight(2);
            p5.fill(dark ? "rgba(15, 23, 42, 0.85)" : "rgba(243,244,246,0.85)");
            const selW = iconSize + 20;
            const selH = iconSize + 16 + fontSize + (showNodeInfo ? fontSize : 0);
            p5.rect(-selW / 2, -iconSize / 2 - 8, selW, selH, 6);
          }
          p5.textFont("Material Design Icons");
          p5.textAlign(p5.CENTER, p5.CENTER);
          p5.textSize(iconSize);
          p5.fill(stColor);
          p5.text(icon, 0, 0);

          p5.textFont("Roboto, sans-serif");
          p5.textAlign(p5.CENTER, p5.CENTER);
          p5.textSize(fontSize);
          p5.fill(dark ? 250 : 23);
          p5.text(nname, 0, iconSize / 2 + 8 + fontSize / 2);

          if (showNodeInfo && nip) {
            p5.textSize(Math.max(fontSize - 2, 8));
            p5.text(nip, 0, iconSize / 2 + 8 + fontSize + fontSize / 2);
          }
        }
      } else {
        if (isSelected) {
          p5.stroke("#06b6d4");
          p5.strokeWeight(2);
          p5.fill(dark ? "rgba(15, 23, 42, 0.85)" : "rgba(243,244,246,0.85)");
          const selW = iconSize + 20;
          const selH = iconSize + 16 + fontSize + (showNodeInfo ? fontSize : 0);
          p5.rect(-selW / 2, -iconSize / 2 - 8, selW, selH, 6);
        }
        p5.textFont("Material Design Icons");
        p5.textAlign(p5.CENTER, p5.CENTER);
        p5.textSize(iconSize);
        p5.fill(stColor);
        p5.text(icon, 0, 0);

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

    if (editDrawItems) {
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
    }

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
    if (editDrawItems) {
      for (const k in items) {
        const ix = items[k].x ?? (items[k] as any).X ?? 0;
        const iy = items[k].y ?? (items[k] as any).Y ?? 0;
        if (ix > sx && ix < lx && iy > sy && iy < ly) {
          selectedDrawItems.push(items[k].id || (items[k] as any).ID);
        }
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
    if (!editDrawItems) {
      selectedDrawItems.length = 0;
      return false;
    }
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

  let selectedNetwork2 = "";
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
        if (second) {
          selectedNetwork2 = nid;
        } else {
          selectedNetwork = nid;
        }
        return true;
      }
    }
    if (!second) {
      selectedNetwork = "";
    }
    return false;
  };

  const checkLine = (e?: MouseEvent) => {
    const isShift = (p5.keyIsDown && p5.keyIsDown(p5.SHIFT)) || (e && e.shiftKey);
    if (!isShift) {
      return false;
    }
    if (selectedNetwork !== "") {
      if (setSelectNode(true)) {
        return true;
      }
      if (setSelectNetwork(true)) {
        return true;
      }
    } else if (selectedNodes.length === 1) {
      if (setSelectNode(true)) {
        return true;
      }
      if (setSelectNetwork(false)) {
        return true;
      }
    }
    return false;
  };

  const editLine = () => {
    const targets: string[] = [...selectedNodes];
    if (selectedNetwork !== "") {
      targets.push("NET:" + selectedNetwork);
    }
    if (selectedNetwork2 !== "") {
      targets.push("NET:" + selectedNetwork2);
    }
    if (targets.length !== 2) {
      return;
    }
    if (mapCallBack) {
      mapCallBack({
        Cmd: "editLine",
        type: "editLine",
        Param: targets,
      });
    }
    selectedNodes.length = 0;
    selectedNetwork = "";
    selectedNetwork2 = "";
    mapRedraw = true;
  };

  const canvasMousePressed = (e?: MouseEvent) => {
    if (readOnly) return true;
    if (p5.mouseButton === p5.RIGHT || (e && (e.button === 2 || e.which === 3))) {
      return false;
    }
    clickInCanvas = true;
    mapRedraw = true;

    if (checkLine(e)) {
      editLine();
      dragMode = 0;
      return false;
    }

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
      if (!hitSelected && editDrawItems) {
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
        (editDrawItems && selectedDrawItems.length > 0) ||
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
      if (selectedNodes.length > 0 || (editDrawItems && selectedDrawItems.length > 0)) {
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
    } else if (editDrawItems && selectedDrawItems.length === 1 && mapCallBack) {
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
      } else if (editDrawItems && selectedDrawItems.length > 0 && mapCallBack) {
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
