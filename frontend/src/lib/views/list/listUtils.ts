import type { NodeEnt, NetworkEnt, LineEnt, DrawItemEnt } from "../../api";
import {
  Type,
  Square,
  Gauge,
  BarChart3,
  TrendingUp,
  CreditCard,
  Boxes,
} from "@lucide/svelte";

export const getNodeName = (id?: string, nodes: NodeEnt[] = []): string => {
  if (!id) return "-";
  const found = nodes.find((n) => (n.id || n.ID) === id);
  return found ? found.name || found.Name || id : id;
};

export const getTargetLabel = (
  id?: string,
  nodes: NodeEnt[] = [],
  networks: NetworkEnt[] = []
): string => {
  if (!id) return "-";
  if (id.startsWith("NET:")) {
    const netId = id.replace("NET:", "");
    const net = networks.find((n) => (n.id || (n as any).ID) === netId);
    return net ? `[Net] ${net.name || (net as any).Name}` : `[Net] ${netId}`;
  }
  const node = nodes.find((n) => (n.id || (n as any).ID) === id);
  return node ? node.name || (node as any).Name || id : id;
};

export const isLineOrphaned = (
  l: LineEnt,
  nodes: NodeEnt[] = [],
  networks: NetworkEnt[] = []
): boolean => {
  const id1 = l.node_id1 || (l as any).NodeID1 || "";
  const id2 = l.node_id2 || (l as any).NodeID2 || "";
  if (!id1 || !id2) return true;

  const exists1 = id1.startsWith("NET:")
    ? networks.some((n) => (n.id || (n as any).ID) === id1.replace("NET:", ""))
    : nodes.some((n) => (n.id || (n as any).ID) === id1);

  const exists2 = id2.startsWith("NET:")
    ? networks.some((n) => (n.id || (n as any).ID) === id2.replace("NET:", ""))
    : nodes.some((n) => (n.id || (n as any).ID) === id2);

  return !exists1 || !exists2;
};

export const isDrawItemOffscreen = (item: DrawItemEnt): boolean => {
  const x = item.x ?? (item as any).X ?? 0;
  const y = item.y ?? (item as any).Y ?? 0;
  return x < -500 || y < -500 || x > 15000 || y > 15000;
};

export const getStatusBadge = (state: string): string => {
  switch (state?.toLowerCase()) {
    case "normal":
      return "bg-emerald-100 text-emerald-800 border-emerald-300 dark:bg-emerald-500/10 dark:text-emerald-400 dark:border-emerald-500/30";
    case "warn":
    case "low":
      return "bg-amber-100 text-amber-800 border-amber-300 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/30";
    case "high":
    case "error":
      return "bg-rose-100 text-rose-800 border-rose-300 dark:bg-rose-500/10 dark:text-rose-400 dark:border-rose-500/30";
    default:
      return "bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-800 dark:text-slate-400 dark:border-slate-700";
  }
};

export const getDrawItemTypeName = (type: number, $_: (key: string) => string): string => {
  return $_(`drawItem.types.${type}`) || `Item (${type})`;
};

export const getDrawItemIcon = (type: number): any => {
  switch (type) {
    case 2:
      return Type;
    case 4:
      return Square;
    case 6:
      return Gauge;
    case 7:
      return BarChart3;
    case 8:
      return TrendingUp;
    case 11:
      return CreditCard;
    default:
      return Boxes;
  }
};

export const compareSortValues = (a: unknown, b: unknown): number => {
  if (Array.isArray(a) && Array.isArray(b)) {
    for (let index = 0; index < Math.max(a.length, b.length); index += 1) {
      const comparison = compareSortValues(a[index] ?? 0, b[index] ?? 0);
      if (comparison !== 0) return comparison;
    }
    return 0;
  }
  if (typeof a === "number" && typeof b === "number") return a - b;
  return String(a ?? "").localeCompare(String(b ?? ""), undefined, {
    numeric: true,
    sensitivity: "base",
  });
};
