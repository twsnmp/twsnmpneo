<script lang="ts">
  import { untrack } from "svelte";
  import { saveDrawItem, fetchDrawItem, type DrawItemEnt, type NodeEnt, type PollingEnt } from "../api";
  import { checkItemPos } from "../map/map";
  import { gauge, bar, line, kpi, classicGauge } from "../map/chart/drawitem";
  import { _ } from "svelte-i18n";
  import {
    X,
    Save,
    Palette,
    Eye,
    Image as ImageIcon,
    Upload,
    Check,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    item = $bindable<DrawItemEnt | null>(null),
    nodes = [],
    pollings = [],
    onSave = () => {},
  } = $props<{
    show: boolean;
    item: DrawItemEnt | null;
    nodes?: NodeEnt[];
    pollings?: PollingEnt[];
    onSave?: (saved: DrawItemEnt) => void;
  }>();

  interface InternalDrawItem {
    ID: string;
    Type: number;
    X: number;
    Y: number;
    W: number;
    H: number;
    Text: string;
    Color: string;
    Size: number;
    PollingID: string;
    VarName: string;
    Format: string;
    Scale: number;
    Cond: number;
    Path: string;
    Value: number;
    Values: number[];
  }

  let drawItem = $state<InternalDrawItem>({
    ID: "",
    Type: 11,
    X: 200,
    Y: 200,
    W: 220,
    H: 84,
    Text: "",
    Color: "#00d2ff",
    Size: 14,
    PollingID: "",
    VarName: "",
    Format: "",
    Scale: 1.0,
    Cond: 0,
    Path: "",
    Value: 0,
    Values: [20, 45, 30, 60, 50, 75, 50],
  });

  let nodeID = $state("");
  let alpha = $state(255);
  let previewDark = $state(true);
  let imagePreview = $state("");
  let saveError = $state("");
  let fileInputRef = $state<HTMLInputElement | null>(null);

  // Recommendations for common variables (from TWSNMP FK)
  interface VarRecommendation {
    format: string;
    scale: number;
    desc: string;
  }

  const getRecommendation = (varName: string, rawVal?: any): VarRecommendation | null => {
    const v = (varName || "").toLowerCase();
    if (v === "rtt") {
      if (typeof rawVal === "number" && rawVal > 1000) {
        return { format: "%.2f ms", scale: 0.000001, desc: "ms (ナノ秒変換)" };
      }
      return { format: "%.2f ms", scale: 1.0, desc: "ms" };
    }
    if (v.includes("bps") || v === "traffic" || v === "throughput") {
      return { format: "BPS", scale: 1.0, desc: "自動単位(bps/Mbps/Gbps)" };
    }
    if (v.includes("pps")) {
      return { format: "PPS", scale: 1.0, desc: "自動パケット数(PPS)" };
    }
    if (
      v.includes("cpu") ||
      v.includes("mem") ||
      v.includes("disk") ||
      v.includes("usage") ||
      v.includes("util")
    ) {
      if (typeof rawVal === "number" && rawVal <= 1.0 && rawVal > 0) {
        return { format: "%.1f%%", scale: 100.0, desc: "パーセント (0-1比率)" };
      }
      return { format: "%.1f%%", scale: 1.0, desc: "パーセント (%)" };
    }
    if (v.includes("temp")) {
      return { format: "%.1f ℃", scale: 1.0, desc: "温度 (℃)" };
    }
    if (v.includes("humid")) {
      return { format: "%.1f%%", scale: 1.0, desc: "湿度 (%)" };
    }
    if (v.includes("volt")) {
      return { format: "%.2f V", scale: 1.0, desc: "電圧 (V)" };
    }
    if (v === "loss") {
      return { format: "%.1f%%", scale: 1.0, desc: "パケット損失率 (%)" };
    }
    if (v === "load") {
      return { format: "LOAD=%.2f", scale: 1.0, desc: "システム負荷" };
    }
    if (v === "count") {
      return { format: "COUNT=%.0f", scale: 1.0, desc: "回数" };
    }
    return null;
  };

  const condList = $derived([
    { name: $_("DrawItem.showItemsAllways") || "常に表示", value: 0 },
    { name: $_("DrawItem.showItemsLow") || "マップ状態が軽度以上の時", value: 1 },
    { name: $_("DrawItem.showItemsHigh") || "マップ状態が重度の時", value: 2 },
  ]);

  const isExisting = $derived(Boolean(drawItem.ID));

  const nodeList = $derived(
    nodes.map((n: NodeEnt) => ({
      name: n.name || (n as any).Name || (n.id || (n as any).ID),
      value: n.id || (n as any).ID || "",
    }))
  );

  const pollingList = $derived(
    pollings
      .filter((p: any) => !nodeID || (p.node_id || p.NodeID) === nodeID)
      .map((p: any) => {
        const id = p.id || (p as any).ID || "";
        const name = p.name || (p as any).Name || id;
        return {
          id,
          name,
          value: id,
        };
      })
  );

  const selectedPolling = $derived(
    pollings.find((p: any) => (p.id || (p as any).ID) === drawItem.PollingID)
  );

  const availableVars = $derived.by(() => {
    if (!selectedPolling) return [];
    const res = (selectedPolling as any).result || (selectedPolling as any).Result;
    if (res && typeof res === "object") {
      return Object.entries(res).map(([k, v]) => ({
        key: k,
        val: v,
        rec: getRecommendation(k, v),
      }));
    }
    return [];
  });

  const defaultTitle = $derived.by(() => {
    if (selectedPolling) {
      const pName = selectedPolling.name || (selectedPolling as any).Name || "";
      return drawItem.VarName ? `${pName} (${drawItem.VarName})` : pName;
    }
    return Number(drawItem.Type) === 4 ? "No Value" : "METRIC";
  });

  // Size presets definition
  interface SizePreset {
    label: string;
    w?: number;
    h?: number;
    size?: number;
  }

  const currentPresets = $derived.by((): SizePreset[] => {
    switch (Number(drawItem.Type)) {
      case 11: // KPI Card
        return [
          { label: `${$_("DrawItem.PresetCompact") || "コンパクト"} (180×70)`, w: 180, h: 70 },
          { label: `${$_("DrawItem.PresetNormal") || "標準"} (220×84)`, w: 220, h: 84 },
          { label: `${$_("DrawItem.PresetLarge") || "大"} (280×100)`, w: 280, h: 100 },
          { label: `${$_("DrawItem.PresetWide") || "ワイド"} (340×90)`, w: 340, h: 90 },
        ];
      case 6: // New Gauge
        return [
          { label: `${$_("DrawItem.PresetSmall") || "小"} (64px)`, h: 64, w: 64 },
          { label: `${$_("DrawItem.PresetNormal") || "標準"} (120px)`, h: 120, w: 120 },
          { label: `${$_("DrawItem.PresetLarge") || "大"} (180px)`, h: 180, w: 180 },
          { label: `${$_("DrawItem.PresetExtraLarge") || "特大"} (240px)`, h: 240, w: 240 },
        ];
      case 7: // Bar
      case 8: // Line
        return [
          { label: `${$_("DrawItem.PresetSmall") || "小"} (200×50)`, h: 50, w: 200 },
          { label: `${$_("DrawItem.PresetNormal") || "標準"} (320×80)`, h: 80, w: 320 },
          { label: `${$_("DrawItem.PresetLarge") || "大"} (440×110)`, h: 110, w: 440 },
        ];
      case 5: // Polling Gauge (Classic)
        return [
          { label: "12 (120px)", size: 12 },
          { label: "16 (160px)", size: 16 },
          { label: "20 (200px)", size: 20 },
          { label: "24 (240px)", size: 24 },
        ];
      case 2: // Label
      case 4: // Polling Text
        return [
          { label: "12px", size: 12 },
          { label: "16px", size: 16 },
          { label: "20px", size: 20 },
          { label: "24px", size: 24 },
          { label: "32px", size: 32 },
        ];
      default: // Rect, Ellipse, Image, GroupFrame, GroupFill
        return [
          { label: `${$_("DrawItem.PresetSmall") || "小"} (160×100)`, w: 160, h: 100 },
          { label: `${$_("DrawItem.PresetNormal") || "中"} (300×180)`, w: 300, h: 180 },
          { label: `${$_("DrawItem.PresetLarge") || "大"} (500×300)`, w: 500, h: 300 },
        ];
    }
  });

  const isPresetActive = (preset: SizePreset): boolean => {
    if (preset.size !== undefined) {
      return Number(drawItem.Size) === preset.size;
    }
    if (preset.w !== undefined && preset.h !== undefined) {
      return Number(drawItem.W) === preset.w && Number(drawItem.H) === preset.h;
    }
    if (preset.h !== undefined) {
      return Number(drawItem.H) === preset.h;
    }
    return false;
  };

  const applyPreset = (preset: SizePreset) => {
    if (preset.w !== undefined) drawItem.W = preset.w;
    if (preset.h !== undefined) drawItem.H = preset.h;
    if (preset.size !== undefined) drawItem.Size = preset.size;
  };

  const selectVariable = (key: string, val: any) => {
    drawItem.VarName = key;
    const rec = getRecommendation(key, val);
    if (rec) {
      drawItem.Format = rec.format;
      drawItem.Scale = rec.scale;
    }
  };

  // Preview computations
  const previewValue = $derived.by(() => {
    if (selectedPolling && drawItem.VarName) {
      const res = (selectedPolling as any).result || (selectedPolling as any).Result;
      if (res && res[drawItem.VarName] !== undefined) {
        const v = Number(res[drawItem.VarName]);
        if (!isNaN(v)) return v * (drawItem.Scale || 1.0);
      }
    }
    const t = Number(drawItem.Type);
    if (t === 11) return 12.4;
    if (t === 6 || t === 5 || t === 7) return 68.5;
    return 25.0;
  });

  const previewFormattedText = $derived.by(() => {
    const val = previewValue;
    if (drawItem.Format) {
      if (drawItem.Format === "BPS") {
        if (val < 1000) return `${val.toFixed(1)} bps`;
        if (val < 1000000) return `${(val / 1000).toFixed(1)} Kbps`;
        if (val < 1000000000) return `${(val / 1000000).toFixed(1)} Mbps`;
        return `${(val / 1000000000).toFixed(1)} Gbps`;
      }
      if (drawItem.Format === "PPS") {
        return `${Math.round(val).toLocaleString()} PPS`;
      }
      try {
        if (drawItem.Format.includes("%")) {
          const precisionMatch = drawItem.Format.match(/%\.([0-9]+)f/);
          const precision = precisionMatch ? parseInt(precisionMatch[1], 10) : 1;
          return drawItem.Format.replace(/%[0-9.]*f/, val.toFixed(precision)).replace(/%s/, String(val));
        }
      } catch {}
    }
    const t = Number(drawItem.Type);
    if (t === 6 || t === 7 || t === 5) {
      return `${val.toFixed(1)}%`;
    }
    return `${val.toFixed(1)}`;
  });

  const previewDataUrl = $derived.by(() => {
    const color = drawItem.Color || "#00d2ff";
    const title = drawItem.Text || defaultTitle;
    const bg = previewDark ? "#171923" : "#f8fafc";
    const sampleHistory = [18, 24, 32, 28, 45, 40, 55, 62, 58, previewValue];

    try {
      switch (Number(drawItem.Type)) {
        case 5: // Classic Polling Gauge
          return classicGauge(title, color, previewValue, drawItem.Size || 16, previewDark);
        case 6: // New Gauge
          return gauge(title, previewValue, bg);
        case 7: // Bar
          return bar(title, color, previewValue, bg);
        case 8: // Line
          return line(title, color, sampleHistory, bg);
        case 11: // KPI Card
          return kpi(
            title,
            previewFormattedText,
            previewValue,
            color,
            sampleHistory,
            previewDark,
            drawItem.W || 220,
            drawItem.H || 84
          );
        default:
          return "";
      }
    } catch {
      return "";
    }
  });

  // Handle open / initialization
  $effect(() => {
    if (show) {
      untrack(() => {
        saveError = "";
        const raw = item;
        const id = raw?.id || (raw as any)?.ID || "";
        const rawType = typeof raw?.type === "number" ? raw.type : (typeof (raw as any)?.Type === "number" ? (raw as any).Type : 11);
        let rawColor = raw?.color || (raw as any)?.Color || "#00d2ff";

        if (rawColor.length === 9) {
          alpha = parseInt(rawColor.substring(7), 16) || 255;
          rawColor = rawColor.substring(0, 7);
        } else {
          alpha = 255;
        }

        let rawText = raw?.text ?? (raw as any)?.Text ?? "";
        if (rawText && rawText.includes("\t")) {
          rawText = rawText.split("\t")[0];
        }

        const pollingID = raw?.polling_id || (raw as any)?.PollingID || "";
        nodeID = "";
        if (pollingID) {
          const matched = pollings.find((p: PollingEnt | any) => (p.id || (p as any).ID) === pollingID);
          if (matched) {
            nodeID = matched.node_id || (matched as any).NodeID || "";
          }
        }

        drawItem = {
          ID: id,
          Type: rawType,
          X: typeof raw?.x === "number" ? raw.x : (typeof (raw as any)?.X === "number" ? (raw as any).X : 200),
          Y: typeof raw?.y === "number" ? raw.y : (typeof (raw as any)?.Y === "number" ? (raw as any).Y : 200),
          W: Number(raw?.w ?? (raw as any)?.W) || (rawType === 11 ? 220 : 120),
          H: Number(raw?.h ?? (raw as any)?.H) || (rawType === 11 ? 84 : 40),
          Text: rawText,
          Color: rawColor,
          Size: Number(raw?.size ?? (raw as any)?.Size) || 14,
          PollingID: pollingID,
          VarName: raw?.var_name || (raw as any)?.VarName || "",
          Format: raw?.format || (raw as any)?.Format || "",
          Scale: Number(raw?.scale ?? (raw as any)?.Scale) || 1.0,
          Cond: Number(raw?.cond ?? (raw as any)?.Cond) || 0,
          Path: raw?.path || (raw as any)?.Path || "",
          Value: Number(raw?.value ?? (raw as any)?.Value) || 0,
          Values: raw?.values || (raw as any)?.Values || [20, 45, 30, 60, 50, 75, 50],
        };

        imagePreview = drawItem.Path || "";

        if (id) {
          fetchDrawItem(id).then((fresh) => {
            if (fresh && fresh.id === id) {
              if (fresh.text !== undefined) drawItem.Text = fresh.text;
              if (fresh.w !== undefined) drawItem.W = fresh.w;
              if (fresh.h !== undefined) drawItem.H = fresh.h;
              if (fresh.color !== undefined) {
                let freshColor = fresh.color;
                if (freshColor.length === 9) {
                  alpha = parseInt(freshColor.substring(7), 16) || 255;
                  freshColor = freshColor.substring(0, 7);
                }
                drawItem.Color = freshColor;
              }
              if (fresh.var_name !== undefined) drawItem.VarName = fresh.var_name;
              if (fresh.format !== undefined) drawItem.Format = fresh.format;
              if (fresh.scale !== undefined) drawItem.Scale = fresh.scale;
            }
          }).catch(() => {});
        }
      });
    }
  });

  const handleSelectImageFile = (e: Event) => {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    const reader = new FileReader();
    reader.onload = (re) => {
      const res = re.target?.result as string;
      if (res) {
        drawItem.Path = res;
        imagePreview = res;
      }
    };
    reader.readAsDataURL(file);
  };

  const handleSave = async () => {
    const a = Math.max(0, Math.min(255, Math.round(alpha)))
      .toString(16)
      .padStart(2, "0");
    const fullColor = (drawItem.Color.startsWith("#") ? drawItem.Color.substring(0, 7) : "#00d2ff") + a;

    const typeNum = Number(drawItem.Type);
    let finalTitle = drawItem.Text;
    if (!finalTitle && (typeNum === 4 || typeNum === 11)) {
      finalTitle = defaultTitle;
    }
    let w = Number(drawItem.W) || 0;
    let h = Number(drawItem.H) || 0;
    const size = Number(drawItem.Size) || 14;

    if (typeNum === 4) {
      if (!w || w <= 0) {
        w = size * Math.max(8, (finalTitle || "No Value").length);
      }
      if (!h || h <= 0) {
        h = size;
      }
    } else if (typeNum === 11) {
      if (!w || w <= 0) w = 220;
      if (!h || h <= 0) h = 84;
    }

    const payload: DrawItemEnt = {
      ...(item || {}),
      id: drawItem.ID,
      ID: drawItem.ID,
      type: typeNum,
      Type: typeNum,
      x: Number(drawItem.X) || 0,
      X: Number(drawItem.X) || 0,
      y: Number(drawItem.Y) || 0,
      Y: Number(drawItem.Y) || 0,
      w,
      W: w,
      h,
      H: h,
      text: finalTitle,
      Text: finalTitle,
      color: fullColor,
      Color: fullColor,
      size,
      Size: size,
      polling_id: drawItem.PollingID,
      PollingID: drawItem.PollingID,
      var_name: drawItem.VarName,
      VarName: drawItem.VarName,
      format: drawItem.Format,
      Format: drawItem.Format,
      scale: Number(drawItem.Scale) || 1.0,
      Scale: Number(drawItem.Scale) || 1.0,
      cond: Number(drawItem.Cond) || 0,
      Cond: Number(drawItem.Cond) || 0,
      path: drawItem.Path,
      Path: drawItem.Path,
      value: Number(drawItem.Value) || 0,
      Value: Number(drawItem.Value) || 0,
      values: drawItem.Values,
      Values: drawItem.Values,
    };
    checkItemPos(payload);

    try {
      const saved = await saveDrawItem(payload);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = `${$_("drawItem.saveError")}: ${e.message || e}`;
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") show = false;
    }}
  >
    <div
      class="flex h-auto max-h-[92vh] w-full max-w-5xl flex-col rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200"
    >
      <!-- Modal Header -->
      <div
        class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0"
      >
        <div class="flex items-center gap-3">
          <div
            class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-600 dark:text-cyan-400"
          >
            <Palette class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              {$_("DrawItem.EditDrawItem") || "描画アイテムの編集"}
            </h2>
            <p class="text-[11px] text-slate-500 dark:text-slate-400">
              {$_("drawItem.subtitle") || "マップ上の装飾ラベル・ゲージ・グラフの表示設定"}
            </p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_("common.close") || "閉じる"}
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body (2-Pane Grid: 7 cols Form, 5 cols Preview) -->
      <div class="flex-1 overflow-y-auto p-5 sm:p-6 custom-scrollbar">
        {#if saveError}
          <div
            class="mb-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-2.5 text-xs text-rose-600 dark:text-rose-400 flex items-center gap-2"
          >
            <span class="font-bold">Error:</span>
            <span>{saveError}</span>
          </div>
        {/if}

        <div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
          <!-- Left Pane: Settings Form (7 Cols) -->
          <div class="lg:col-span-7 space-y-4">
            <!-- Type Selection (Categorized & Compact) -->
            <div class="space-y-1.5">
              <label
                for="drawitem-type-select"
                class="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center justify-between"
              >
                <span>
                  {$_("DrawItem.Type") || "種類"}
                  {#if isExisting}
                    <span class="text-xs text-slate-400 ml-1 font-normal">
                      ({$_("DrawItem.NonEditable") || "変更不可"})
                    </span>
                  {/if}
                </span>
              </label>
              <select
                id="drawitem-type-select"
                class="w-full bg-slate-50 dark:bg-slate-800/80 border border-slate-300 dark:border-slate-700 text-slate-900 dark:text-slate-100 text-sm rounded-lg focus:ring-2 focus:ring-cyan-500 focus:border-cyan-500 p-2.5 disabled:opacity-60 transition-colors cursor-pointer"
                value={drawItem.Type}
                onchange={(e) => {
                  const val = Number((e.target as HTMLSelectElement).value);
                  drawItem.Type = val;
                  if (val === 4) {
                    if (!drawItem.Size) drawItem.Size = 14;
                  } else if (val === 11) {
                    if (!drawItem.W) drawItem.W = 220;
                    if (!drawItem.H) drawItem.H = 84;
                  }
                }}
                disabled={isExisting}
              >
                <optgroup label="📊 {$_('DrawItem.CategoryPolling') || 'ポーリング測定値'}">
                  <option value={11}>💎 {$_("DrawItem.KPI") || "ポーリング結果(KPIカード)"}</option>
                  <option value={6}>🎯 {$_("DrawItem.NewGauge") || "ポーリング結果(新ゲージ)"}</option>
                  <option value={7}>📊 {$_("DrawItem.Bar") || "ポーリング結果(バー)"}</option>
                  <option value={8}>📈 {$_("DrawItem.Line") || "ポーリング結果(ライン)"}</option>
                  <option value={4}>📝 {$_("DrawItem.PollingText") || "ポーリング結果(テキスト)"}</option>
                  <option value={5}>⏱️ {$_("DrawItem.PollingGauge") || "ポーリング結果(ゲージ)"}</option>
                </optgroup>
                <optgroup label="📐 {$_('DrawItem.CategoryShape') || '基本図形・装飾'}">
                  <option value={2}>🏷️ {$_("DrawItem.Label") || "ラベル"}</option>
                  <option value={3}>🖼️ {$_("DrawItem.Image") || "イメージ"}</option>
                  <option value={0}>⬜ {$_("DrawItem.Rect") || "矩形"}</option>
                  <option value={1}>⚪ {$_("DrawItem.Ellipse") || "楕円"}</option>
                </optgroup>
                <optgroup label="🔲 {$_('DrawItem.CategoryGroup') || 'グループ化'}">
                  <option value={9}>▢ {$_("DrawItem.GroupFrame") || "グループ(枠)"}</option>
                  <option value={10}>⬛ {$_("DrawItem.GroupFill") || "グループ(背景)"}</option>
                </optgroup>
              </select>
            </div>

            <!-- Size Presets & Dimension Inputs -->
            <div
              class="p-3.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-700/80 space-y-3"
            >
              <div class="flex items-center justify-between">
                <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  {$_("DrawItem.Preset") || "推奨サイズ"}
                </span>
              </div>

              <!-- Preset Quick Buttons -->
              <div class="flex flex-wrap gap-1.5">
                {#each currentPresets as p}
                  {@const active = isPresetActive(p)}
                  <button
                    type="button"
                    class="px-2.5 py-1 text-xs rounded-lg transition-all flex items-center gap-1.5 cursor-pointer {active
                      ? 'bg-cyan-600 text-white font-bold border border-cyan-500 shadow-sm ring-2 ring-cyan-300 dark:ring-cyan-800'
                      : 'bg-white dark:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-300 dark:border-slate-600 hover:border-cyan-400 hover:text-cyan-600 dark:hover:text-cyan-300 font-medium'}"
                    onclick={() => applyPreset(p)}
                  >
                    {#if active}
                      <Check class="h-3.5 w-3.5" />
                    {/if}
                    <span>{p.label}</span>
                  </button>
                {/each}
              </div>

              <!-- Manual Fine-tuning Inputs -->
              <div class="grid grid-cols-2 gap-3 pt-1">
                {#if Number(drawItem.Type) === 11 || Number(drawItem.Type) < 2 || Number(drawItem.Type) === 3 || Number(drawItem.Type) === 9 || Number(drawItem.Type) === 10}
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Width") || "幅"} (px)</span>
                    <input
                      class="h-8 w-full text-right bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      type="number"
                      min={0}
                      max={2000}
                      bind:value={drawItem.W}
                    />
                  </label>
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Height") || "高さ"} (px)</span>
                    <input
                      class="h-8 w-full text-right bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      type="number"
                      min={0}
                      max={2000}
                      bind:value={drawItem.H}
                    />
                  </label>
                {:else if Number(drawItem.Type) === 6 || Number(drawItem.Type) === 7 || Number(drawItem.Type) === 8}
                  <label class="space-y-1 text-xs col-span-2">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Height") || "サイズ/高さ"} (px)</span>
                    <input
                      class="h-8 w-full text-right bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      type="number"
                      min={10}
                      max={1000}
                      bind:value={drawItem.H}
                    />
                  </label>
                {:else}
                  <label class="space-y-1 text-xs col-span-2">
                    <span class="text-slate-600 dark:text-slate-400">
                      {#if Number(drawItem.Type) === 5}
                        {$_("DrawItem.GaugeSize") || "サイズ (直径 = 設定値 × 10 px)"}
                      {:else}
                        {$_("DrawItem.FontSize") || "文字サイズ"} (px)
                      {/if}
                    </span>
                    <input
                      class="h-8 w-full text-right bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      type="number"
                      min={8}
                      max={128}
                      bind:value={drawItem.Size}
                    />
                  </label>
                {/if}
              </div>
            </div>

            <!-- Polling Selection & Variable Assistant -->
            {#if (Number(drawItem.Type) >= 4 && Number(drawItem.Type) < 9) || Number(drawItem.Type) === 11}
              <div
                class="p-3.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-700/80 space-y-3"
              >
                <div class="grid grid-cols-2 gap-3">
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Node") || "ノード"}</span>
                    <select
                      class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500 cursor-pointer"
                      bind:value={nodeID}
                    >
                      <option value="">{$_("DrawItem.SelectNode") || "ノードを選択"}</option>
                      {#each nodeList as n}
                        <option value={n.value}>{n.name}</option>
                      {/each}
                    </select>
                  </label>
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Polling") || "ポーリング"}</span>
                    <select
                      class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500 cursor-pointer"
                      bind:value={drawItem.PollingID}
                    >
                      <option value="">{$_("DrawItem.SelectPolling") || "ポーリングを選択"}</option>
                      {#each pollingList as p}
                        <option value={p.value}>{p.name}</option>
                      {/each}
                    </select>
                  </label>
                </div>

                <!-- Available Variable Chips (Click to auto-apply format & scale) -->
                <div class="space-y-1.5">
                  <div class="flex items-center justify-between">
                    <span class="text-xs font-semibold text-slate-700 dark:text-slate-300">
                      {$_("DrawItem.AvailableVars") || "利用可能な変数（クリックで書式・倍率も自動設定）"}
                    </span>
                  </div>
                  {#if availableVars.length > 0}
                    <div class="flex flex-wrap gap-1.5 pt-1">
                      {#each availableVars as it}
                        {@const isSelected = drawItem.VarName === it.key}
                        <button
                          type="button"
                          class="px-2.5 py-1 text-xs rounded-full border transition-all flex items-center gap-1.5 cursor-pointer {isSelected
                            ? 'bg-cyan-600 text-white border-cyan-600 shadow-sm'
                            : 'bg-white dark:bg-slate-700 text-slate-800 dark:text-slate-200 border-slate-300 dark:border-slate-600 hover:border-cyan-400'}"
                          onclick={() => selectVariable(it.key, it.val)}
                          title={it.rec
                            ? `${$_("DrawItem.Recommended") || "推奨"}: ${it.rec.format} (${$_("DrawItem.Scale") || "倍率"}: ${it.rec.scale})`
                            : $_("DrawItem.ClickToSelect") || "クリックして選択"}
                        >
                          <span class="font-bold">{it.key}</span>
                          {#if it.val !== undefined}
                            <span class="opacity-75 text-[10px]">({it.val})</span>
                          {/if}
                          {#if it.rec}
                            <span
                              class="text-[9px] bg-cyan-100 dark:bg-cyan-900/60 text-cyan-800 dark:text-cyan-300 px-1 rounded font-medium"
                            >
                              {it.rec.desc}
                            </span>
                          {/if}
                        </button>
                      {/each}
                    </div>
                  {:else if drawItem.PollingID}
                    <p class="text-xs text-slate-400 italic">
                      {$_("DrawItem.WaitingPollingData") || "※ ポーリングの最新結果データを待機中、または変数が自動設定されます"}
                    </p>
                  {:else}
                    <p class="text-xs text-slate-400 italic">
                      {$_("DrawItem.SelectNodeAndPolling") || "※ ノードとポーリングを選択すると、利用可能な測定変数がここに表示されます"}
                    </p>
                  {/if}
                </div>

                <!-- Variable Name, Format, Label, Scale -->
                <div class="grid grid-cols-2 gap-3 pt-1">
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.ValName") || "変数名"}</span>
                    <input
                      class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      bind:value={drawItem.VarName}
                      placeholder={$_("DrawItem.ValNamePH") || "空欄時は自動設定"}
                    />
                  </label>
                  {#if Number(drawItem.Type) === 4 || Number(drawItem.Type) === 11}
                    <label class="space-y-1 text-xs">
                      <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.TextFormat") || "表示フォーマット"}</span>
                      <input
                        class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                        bind:value={drawItem.Format}
                        placeholder={$_("DrawItem.TextFormatPH") || "例: %.2f ms / BPS"}
                      />
                    </label>
                  {/if}
                </div>

                <div class="grid grid-cols-2 gap-3">
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.GaugeLabel") || "表示ラベル/タイトル"}</span>
                    <input
                      class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      bind:value={drawItem.Text}
                      placeholder={defaultTitle}
                    />
                  </label>
                  <label class="space-y-1 text-xs">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Zoom") || "倍率/スケール"}</span>
                    <input
                      class="h-8 w-full text-right bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      type="number"
                      min={0.000000001}
                      max={1000}
                      step={0.1}
                      bind:value={drawItem.Scale}
                    />
                  </label>
                </div>
              </div>
            {/if}

            <!-- Colors & Image Selection for Shapes / Custom Styles -->
            {#if Number(drawItem.Type) < 4 || Number(drawItem.Type) >= 9}
              <div
                class="p-3.5 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-700/80 space-y-3"
              >
                {#if Number(drawItem.Type) === 3}
                  <!-- Image Selection -->
                  <div class="flex items-center gap-3">
                    <input
                      type="file"
                      accept="image/*"
                      class="hidden"
                      bind:this={fileInputRef}
                      onchange={handleSelectImageFile}
                    />
                    <button
                      type="button"
                      class="h-8 px-3 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-sm flex items-center gap-1.5 cursor-pointer transition-colors"
                      onclick={() => fileInputRef?.click()}
                    >
                      <Upload class="h-3.5 w-3.5" />
                      {$_("DrawItem.Select") || "画像ファイルを選択"}
                    </button>
                    <span class="text-xs text-slate-500 dark:text-slate-400 truncate max-w-xs">
                      {drawItem.Path ? (drawItem.Path.startsWith("data:") ? "Data URL (画像ロード済)" : drawItem.Path) : ($_("DrawItem.NotSelected") || "未選択")}
                    </span>
                  </div>
                {:else}
                  <div class="grid grid-cols-2 gap-3 items-center">
                    <label class="space-y-1 text-xs">
                      <div class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Color") || "色・不透明度"}</div>
                      <div class="flex items-center space-x-2">
                        <input
                          type="color"
                          class="h-8 w-11 rounded cursor-pointer border border-slate-300 dark:border-slate-600 bg-transparent p-0"
                          bind:value={drawItem.Color}
                        />
                        <input
                          type="range"
                          min={0}
                          max={255}
                          step={1}
                          bind:value={alpha}
                          class="w-24 accent-cyan-500"
                        />
                        <span class="text-xs text-slate-500">{Math.round((alpha / 255) * 100)}%</span>
                      </div>
                    </label>
                    <label class="space-y-1 text-xs">
                      <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.showCond") || "表示条件"}</span>
                      <select
                        class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500 cursor-pointer"
                        bind:value={drawItem.Cond}
                      >
                        {#each condList as c}
                          <option value={c.value}>{c.name}</option>
                        {/each}
                      </select>
                    </label>
                  </div>
                {/if}

                {#if Number(drawItem.Type) === 2 || Number(drawItem.Type) === 9 || Number(drawItem.Type) === 10}
                  <label class="space-y-1 text-xs block">
                    <span class="text-slate-600 dark:text-slate-400">{$_("DrawItem.Text") || "表示文字列"}</span>
                    <input
                      class="h-8 w-full bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg px-2.5 text-xs text-slate-900 dark:text-slate-100 focus:ring-1 focus:ring-cyan-500"
                      bind:value={drawItem.Text}
                      placeholder={$_("DrawItem.TextToDisplay") || "表示するテキスト"}
                    />
                  </label>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Right Pane: Real-time Live Preview (5 Cols) -->
          <div class="lg:col-span-5 flex flex-col space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs font-semibold text-slate-700 dark:text-slate-300 flex items-center gap-1.5">
                <Eye class="h-4 w-4 text-emerald-500" />
                {$_("DrawItem.Preview") || "リアルタイムプレビュー"}
              </span>
              <!-- Theme Toggle for Preview -->
              <div class="inline-flex rounded-lg shadow-sm overflow-hidden border border-slate-300 dark:border-slate-700 text-xs">
                <button
                  type="button"
                  class="px-2.5 py-0.5 font-medium transition-colors cursor-pointer {previewDark
                    ? 'bg-slate-800 text-cyan-400 font-bold'
                    : 'bg-white text-slate-600 hover:bg-slate-50'}"
                  onclick={() => (previewDark = true)}
                >
                  {$_("DrawItem.PreviewDark") || "Dark"}
                </button>
                <button
                  type="button"
                  class="px-2.5 py-0.5 font-medium transition-colors cursor-pointer {!previewDark
                    ? 'bg-cyan-50 dark:bg-slate-700 text-cyan-600 font-bold'
                    : 'bg-slate-800 text-slate-400 hover:bg-slate-700'}"
                  onclick={() => (previewDark = false)}
                >
                  {$_("DrawItem.PreviewLight") || "Light"}
                </button>
              </div>
            </div>

            <!-- Preview Stage Container -->
            <div
              class="flex-1 min-h-[280px] rounded-xl border p-4 flex flex-col items-center justify-center relative overflow-hidden transition-colors {previewDark
                ? 'bg-[#12141c] border-slate-700'
                : 'bg-[#f1f5f9] border-slate-300'}"
            >
              <!-- Grid Background Pattern -->
              <div
                class="absolute inset-0 pointer-events-none opacity-20"
                style="background-image: radial-gradient({previewDark ? '#ffffff' : '#000000'} 1px, transparent 1px); background-size: 16px 16px;"
              ></div>

              <!-- Content Preview -->
              <div class="relative z-10 flex items-center justify-center max-w-full max-h-full overflow-hidden p-2">
                {#if previewDataUrl}
                  <img
                    src={previewDataUrl}
                    alt="Preview"
                    class="max-w-full max-h-[220px] object-contain drop-shadow-md"
                  />
                {:else if Number(drawItem.Type) === 0}
                  <!-- Rect Preview -->
                  <div
                    class="border transition-all"
                    style="width: {Math.min(260, Math.max(40, drawItem.W || 160))}px; height: {Math.min(180, Math.max(30, drawItem.H || 100))}px; background-color: {drawItem.Color}; opacity: {alpha / 255}; border-radius: 6px; border-color: {previewDark ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.2)'};"
                  ></div>
                {:else if Number(drawItem.Type) === 1}
                  <!-- Ellipse Preview -->
                  <div
                    class="border transition-all"
                    style="width: {Math.min(260, Math.max(40, drawItem.W || 160))}px; height: {Math.min(180, Math.max(30, drawItem.H || 100))}px; background-color: {drawItem.Color}; opacity: {alpha / 255}; border-radius: 50%; border-color: {previewDark ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.2)'};"
                  ></div>
                {:else if Number(drawItem.Type) === 2}
                  <!-- Label Preview -->
                  <span
                    style="font-size: {Math.min(36, Math.max(10, drawItem.Size || 16))}px; color: {drawItem.Color || (previewDark ? '#f3f4f6' : '#1e293b')}; opacity: {alpha / 255}; font-weight: 600;"
                  >
                    {drawItem.Text || "Sample Label"}
                  </span>
                {:else if Number(drawItem.Type) === 3}
                  <!-- Image Preview -->
                  {#if imagePreview}
                    <img
                      src={imagePreview}
                      alt="Preview"
                      class="max-w-full max-h-[180px] object-contain rounded"
                    />
                  {:else}
                    <div class="text-xs text-slate-400 flex flex-col items-center gap-1.5">
                      <ImageIcon class="h-8 w-8 text-slate-500" />
                      <span>{$_("DrawItem.NoImageSelected") || "画像が未選択です"}</span>
                    </div>
                  {/if}
                {:else if Number(drawItem.Type) === 4}
                  <!-- Polling Text Preview -->
                  <div class="px-3 py-1.5 rounded-lg bg-black/40 backdrop-blur-sm border border-white/10 flex items-center gap-2">
                    <div class="w-2 h-2 rounded-full bg-emerald-400"></div>
                    <span
                      style="font-size: {Math.min(28, Math.max(10, drawItem.Size || 14))}px; color: {drawItem.Color || '#00d2ff'}; font-family: monospace;"
                    >
                      {previewFormattedText || "12.4 Mbps"}
                    </span>
                  </div>
                {:else if Number(drawItem.Type) === 9 || Number(drawItem.Type) === 10}
                  <!-- Group Frame / Fill Preview -->
                  <div
                    class="relative p-2"
                    style="width: {Math.min(260, Math.max(60, drawItem.W || 200))}px; height: {Math.min(180, Math.max(40, drawItem.H || 120))}px; border-radius: 8px; border: {Number(drawItem.Type) === 9 ? `2px solid ${drawItem.Color}` : 'none'}; background-color: {Number(drawItem.Type) === 10 ? drawItem.Color : 'rgba(23,23,23,0.05)'}; opacity: {Number(drawItem.Type) === 10 ? alpha / 255 : 1};"
                  >
                    <span
                      class="absolute bottom-1 right-2 text-xs font-bold"
                      style="color: {previewDark ? '#eee' : '#333'}; font-size: {drawItem.Size || 11}px;"
                    >
                      {drawItem.Text || "Group Title"}
                    </span>
                  </div>
                {/if}
              </div>

              <!-- Size / Dimension Badge -->
              <div
                class="absolute bottom-2 left-3 text-[11px] text-slate-400 bg-black/40 backdrop-blur-sm px-2 py-0.5 rounded-md"
              >
                {#if Number(drawItem.Type) === 2 || Number(drawItem.Type) === 4}
                  {$_("DrawItem.PreviewSize") || "サイズ"}: {drawItem.Size || 16} px
                {:else if Number(drawItem.Type) === 5}
                  {$_("DrawItem.PreviewSize") || "サイズ"}: {(drawItem.Size || 16) * 10} × {(drawItem.Size || 16) * 10} px ({$_("DrawItem.SettingValue") || "設定値"}: {drawItem.Size || 16})
                {:else if Number(drawItem.Type) === 6}
                  {$_("DrawItem.PreviewSize") || "サイズ"}: {drawItem.H || 120} × {drawItem.H || 120} px
                {:else if Number(drawItem.Type) === 7 || Number(drawItem.Type) === 8}
                  {$_("DrawItem.PreviewSize") || "サイズ"}: {(drawItem.H || 80) * 4} × {drawItem.H || 80} px
                {:else}
                  {$_("DrawItem.PreviewSize") || "サイズ"}: {drawItem.W || 220} × {drawItem.H || 84} px
                {/if}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Action Buttons -->
      <div
        class="flex items-center justify-end gap-3 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5 shrink-0"
      >
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-xl px-4 py-2 text-xs font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-200/60 dark:hover:bg-slate-800 transition-colors cursor-pointer"
        >
          {$_("DrawItem.Cancel") || "キャンセル"}
        </button>
        <button
          type="button"
          onclick={handleSave}
          class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-500 to-blue-600 px-5 py-2 text-xs font-semibold text-white shadow-lg shadow-cyan-500/20 hover:from-cyan-400 hover:to-blue-500 transition-all cursor-pointer"
        >
          <Save class="h-3.5 w-3.5" />
          {$_("DrawItem.Save") || "保存"}
        </button>
      </div>
    </div>
  </div>
{/if}
