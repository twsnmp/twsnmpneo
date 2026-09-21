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
      if (id) nodes[id] = n;
    });

    lines = await fetchLines();

    const itemList = await fetchDrawItems();
    items = {};
    itemList.forEach((item) => {
      const id = item.id || (item as any).ID || '';
      if (id) items[id] = item;
    });

    const netList = await fetchNetworks();
    networks = {};
    netList.forEach((net) => {
      const id = net.id || (net as any).ID || '';
      if (id) networks[id] = net;
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

const mapMain = (p5: P5) => {
  let moveX = 0;
  let moveY = 0;
  let dragX = 0;
  let dragY = 0;
  let dragging = false;
  let draggingElements = false;
  let lastClickTime = 0;
  let clickInCanvas = false;

  p5.setup = () => {
    const parentEl = (p5 as any)._userNode as HTMLElement | undefined;
    const w = parentEl && parentEl.clientWidth > 0 ? parentEl.clientWidth : p5.windowWidth;
    const h = parentEl && parentEl.clientHeight > 0 ? parentEl.clientHeight : Math.floor(p5.windowHeight * 0.6);
    const c = p5.createCanvas(w, h);
    c.mousePressed(canvasMousePressed);
    p5.frameRate(30);
    p5.strokeWeight(1);
    p5.textFont("Roboto, sans-serif");
    updateMAP();
  };

  p5.windowResized = () => {
    const parentEl = (p5 as any)._userNode as HTMLElement | undefined;
    const w = parentEl && parentEl.clientWidth > 0 ? parentEl.clientWidth : p5.windowWidth;
    const h = parentEl && parentEl.clientHeight > 0 ? parentEl.clientHeight : Math.floor(p5.windowHeight * 0.6);
    p5.resizeCanvas(w, h);
    mapRedraw = true;
  };

  p5.draw = () => {
    if (!mapRedraw) return;
    const dark = isDark();
    p5.clear();
    p5.background(dark ? p5.color(11, 19, 41) : 252);

    p5.push();
    p5.translate(moveX, moveY);
    p5.scale(scale);

    // Draw lines
    drawLines(p5);
    // Draw SW-HUB networks
    drawNetworks(p5, dark);
    // Draw draw items
    drawItems(p5, dark);
    // Draw nodes
    drawNodes(p5, dark);

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
      if (selectedNetwork === netId) {
        p5.stroke("#06b6d4");
        p5.strokeWeight(2);
      } else if (net.error || (net as any).Error) {
        p5.stroke("#ef4444");
      } else {
        p5.stroke("#334155");
      }
      p5.fill(dark ? "rgba(15, 23, 42, 0.9)" : "rgba(243,244,246,0.9)");
      const nw = net.w || (net as any).W || 320;
      const nh = net.h || (net as any).H || 140;
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
            // High quality RJ45 port socket graphic
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
      p5.push();
      p5.translate(it.x, it.y);

      if (imageMap.has(k)) {
        const img = imageMap.get(k);
        p5.image(img, 0, 0, it.w || 100, it.h || 100);
      } else {
        // Simple shape/text fallback
        if (it.type === 2) {
          p5.fill(it.color || (dark ? "#f3f4f6" : "#1f2937"));
          p5.textSize(it.size || 14);
          p5.text(it.text || "", 0, it.size || 14);
        } else {
          p5.stroke(it.color || "#6b7280");
          p5.fill("rgba(100,100,100,0.1)");
          p5.rect(0, 0, it.w || 80, it.h || 40, 4);
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
        p5.stroke(stColor);
        p5.strokeWeight(1.5);
        p5.fill(dark ? "rgba(15, 23, 42, 0.8)" : "rgba(243,244,246,0.8)");
        const selW = iconSize + 16;
        const selH = iconSize + 16 + fontSize + (showNodeInfo ? fontSize : 0);
        p5.rect(-selW / 2, -iconSize / 2 - 8, selW, selH, 6);
      }

      if (nimage && imageMap.has(nimage)) {
        const img = imageMap.get(nimage);
        p5.tint(stColor);
        p5.image(img, -iconSize / 2, -iconSize / 2, iconSize, iconSize);
        p5.noTint();
      } else {
        // Icon glyph matching twsnmpfk: full size MDI icon directly in status color
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

  const canvasMousePressed = () => {
    if (readOnly) return;
    clickInCanvas = true;
    const mx = (p5.mouseX - moveX) / scale;
    const my = (p5.mouseY - moveY) / scale;

    if (p5.mouseButton === p5.LEFT) {
      const now = Date.now();
      const isDbl = now - lastClickTime < 300;
      lastClickTime = now;

      // Check node click
      let clickedNode = "";
      for (const k in nodes) {
        const n = nodes[k];
        const nx = n.x ?? (n as any).X ?? 0;
        const ny = n.y ?? (n as any).Y ?? 0;
        const dist = Math.hypot(mx - nx, my - ny);
        if (dist <= iconSize) {
          clickedNode = n.id || (n as any).ID;
          break;
        }
      }

      if (clickedNode) {
        if (isDbl) {
          if (mapCallBack) {
            mapCallBack({
              type: "dblclick",
              Cmd: "nodeDoubleClicked",
              nodeId: clickedNode,
              Param: clickedNode,
            });
          }
          return;
        }
        if (!selectedNodes.includes(clickedNode)) {
          selectedNodes.length = 0;
          selectedNodes.push(clickedNode);
        }
        selectedNetwork = "";
        selectedDrawItems.length = 0;
        draggingElements = true;
        dragX = p5.mouseX;
        dragY = p5.mouseY;
        mapRedraw = true;
        return;
      }

      // Check network click
      let clickedNet = "";
      for (const k in networks) {
        const net = networks[k];
        const nw = net.w || (net as any).W || 320;
        const nh = net.h || (net as any).H || 140;
        const nx = net.x ?? (net as any).X ?? 0;
        const ny = net.y ?? (net as any).Y ?? 0;
        if (mx >= nx && mx <= nx + nw && my >= ny && my <= ny + nh) {
          clickedNet = net.id || (net as any).ID;
          break;
        }
      }

      if (clickedNet) {
        if (isDbl) {
          if (mapCallBack) {
            mapCallBack({
              type: "dblclick",
              Cmd: "networkDoubleClicked",
              networkId: clickedNet,
              Param: clickedNet,
            });
          }
          return;
        }
        selectedNetwork = clickedNet;
        selectedNodes.length = 0;
        selectedDrawItems.length = 0;
        draggingElements = true;
        dragX = p5.mouseX;
        dragY = p5.mouseY;
        mapRedraw = true;
        return;
      }

      // Canvas drag
      dragging = true;
      dragX = p5.mouseX;
      dragY = p5.mouseY;
      selectedNodes.length = 0;
      selectedNetwork = "";
      selectedDrawItems.length = 0;
      mapRedraw = true;
    } else if (p5.mouseButton === p5.RIGHT) {
      // Context menu
      let targetNode = "";
      for (const k in nodes) {
        const n = nodes[k];
        const nx = n.x ?? (n as any).X ?? 0;
        const ny = n.y ?? (n as any).Y ?? 0;
        const dist = Math.hypot(mx - nx, my - ny);
        if (dist <= iconSize) {
          targetNode = n.id || (n as any).ID;
          break;
        }
      }
      let targetNet = "";
      for (const k in networks) {
        const net = networks[k];
        const nw = net.w || (net as any).W || 320;
        const nh = net.h || (net as any).H || 140;
        const nx = net.x ?? (net as any).X ?? 0;
        const ny = net.y ?? (net as any).Y ?? 0;
        if (mx >= nx && mx <= nx + nw && my >= ny && my <= ny + nh) {
          targetNet = net.id || (net as any).ID;
          break;
        }
      }

      if (mapCallBack) {
        mapCallBack({
          type: "contextmenu",
          Cmd: "contextMenu",
          nodeId: targetNode,
          networkId: targetNet,
          Node: targetNode,
          Network: targetNet,
          x: p5.mouseX,
          y: p5.mouseY,
          mapX: mx,
          mapY: my,
        });
      }
    }
  };

  p5.mouseDragged = () => {
    if (readOnly || !clickInCanvas) return;
    if (dragging) {
      moveX += p5.mouseX - dragX;
      moveY += p5.mouseY - dragY;
      dragX = p5.mouseX;
      dragY = p5.mouseY;
      mapRedraw = true;
    } else if (draggingElements) {
      const dx = (p5.mouseX - dragX) / scale;
      const dy = (p5.mouseY - dragY) / scale;
      dragX = p5.mouseX;
      dragY = p5.mouseY;

      if (selectedNodes.length > 0) {
        for (const id of selectedNodes) {
          if (nodes[id]) {
            nodes[id].x = (nodes[id].x || 0) + dx;
            nodes[id].y = (nodes[id].y || 0) + dy;
          }
        }
      }
      if (selectedNetwork && networks[selectedNetwork]) {
        networks[selectedNetwork].x = (networks[selectedNetwork].x || 0) + dx;
        networks[selectedNetwork].y = (networks[selectedNetwork].y || 0) + dy;
      }
      mapRedraw = true;
    }
  };

  p5.mouseReleased = () => {
    if (draggingElements) {
      // Save position to backend
      for (const id of selectedNodes) {
        if (nodes[id]) saveNode(nodes[id]).catch(console.error);
      }
      if (selectedNetwork && networks[selectedNetwork]) {
        saveNetwork(networks[selectedNetwork]).catch(console.error);
      }
    }
    dragging = false;
    draggingElements = false;
    clickInCanvas = false;
  };

  p5.mouseWheel = (event: WheelEvent) => {
    if (event.deltaY < 0) zoom(true);
    else zoom(false);
    return false;
  };
};
