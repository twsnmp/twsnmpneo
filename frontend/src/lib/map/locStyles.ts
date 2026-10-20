/**
 * Map style presets and utilities for GIS / Location view.
 * Fully compatible with twsnmpfk style definitions.
 */

export interface LocStylePreset {
  key: string;
  nameJa: string;
  nameEn: string;
  style: string;
}

export const defaultOsmStyle = {
  version: 8 as const,
  sources: {
    "osm-tiles": {
      type: "raster" as const,
      tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
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

export const locStylePresets: LocStylePreset[] = [
  {
    key: "LocStyleNone",
    nameJa: "なし (デフォルト / OpenStreetMap)",
    nameEn: "None (Default / OpenStreetMap)",
    style: "",
  },
  {
    key: "LocStyleGSIPale",
    nameJa: "国土地理院 淡色地図 (推奨)",
    nameEn: "GSI Pale (Recommended)",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          "gsi-pale": {
            type: "raster",
            tiles: [
              "https://cyberjapandata.gsi.go.jp/xyz/pale/{z}/{x}/{y}.png",
            ],
            tileSize: 256,
            attribution: "国土地理院 (地理院タイル 淡色地図)",
          },
        },
        layers: [
          {
            id: "gsi-pale",
            type: "raster",
            source: "gsi-pale",
            minzoom: 2,
            maxzoom: 18,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleCartoDark",
    nameJa: "CARTO Dark Matter (ダーク推奨)",
    nameEn: "CARTO Dark Matter (Dark)",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          "carto-dark": {
            type: "raster",
            tiles: [
              "https://a.basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}.png",
              "https://b.basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}.png",
              "https://c.basemaps.cartocdn.com/rastertiles/dark_all/{z}/{x}/{y}.png",
            ],
            tileSize: 256,
            attribution: "© OpenStreetMap contributors, © CARTO",
          },
        },
        layers: [
          {
            id: "carto-dark",
            type: "raster",
            source: "carto-dark",
            minzoom: 0,
            maxzoom: 19,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleOSMBright",
    nameJa: "OSM Bright (OpenStreetMap Japan)",
    nameEn: "OSM Bright (OpenStreetMap Japan)",
    style: "https://tile.openstreetmap.jp/styles/osm-bright-ja/style.json",
  },
  {
    key: "LocStyleMieruneMono",
    nameJa: "MIERUNE Mono (モノクロ)",
    nameEn: "MIERUNE Mono (Monochrome)",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          MIERUNEMAP: {
            type: "raster",
            tiles: [
              "https://tile.mierune.co.jp/mierune_mono/{z}/{x}/{y}.png",
            ],
            tileSize: 256,
            attribution:
              "Maptiles by MIERUNE, under CC BY. Data by OpenStreetMap contributors, under ODbL.",
          },
        },
        layers: [
          {
            id: "MIERUNEMAP",
            type: "raster",
            source: "MIERUNEMAP",
            minzoom: 0,
            maxzoom: 18,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleGSIStd",
    nameJa: "国土地理院 標準地図",
    nameEn: "GSI Standard",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          "gsi-std": {
            type: "raster",
            tiles: [
              "https://cyberjapandata.gsi.go.jp/xyz/std/{z}/{x}/{y}.png",
            ],
            tileSize: 256,
            attribution: "国土地理院 (地理院タイル 標準地図)",
          },
        },
        layers: [
          {
            id: "gsi-std",
            type: "raster",
            source: "gsi-std",
            minzoom: 2,
            maxzoom: 18,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleGSIPhoto",
    nameJa: "国土地理院 写真 (航空写真)",
    nameEn: "GSI Photo (Aerial photo)",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          "gsi-photo": {
            type: "raster",
            tiles: [
              "https://cyberjapandata.gsi.go.jp/xyz/seamlessphoto/{z}/{x}/{y}.jpg",
            ],
            tileSize: 256,
            attribution: "国土地理院 (地理院タイル 写真)",
          },
        },
        layers: [
          {
            id: "gsi-photo",
            type: "raster",
            source: "gsi-photo",
            minzoom: 2,
            maxzoom: 18,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleCartoLight",
    nameJa: "CARTO Positron (ライト)",
    nameEn: "CARTO Positron (Light)",
    style: JSON.stringify(
      {
        version: 8,
        sources: {
          "carto-light": {
            type: "raster",
            tiles: [
              "https://a.basemaps.cartocdn.com/rastertiles/light_all/{z}/{x}/{y}.png",
              "https://b.basemaps.cartocdn.com/rastertiles/light_all/{z}/{x}/{y}.png",
              "https://c.basemaps.cartocdn.com/rastertiles/light_all/{z}/{x}/{y}.png",
            ],
            tileSize: 256,
            attribution: "© OpenStreetMap contributors, © CARTO",
          },
        },
        layers: [
          {
            id: "carto-light",
            type: "raster",
            source: "carto-light",
            minzoom: 0,
            maxzoom: 19,
          },
        ],
      },
      null,
      2
    ),
  },
  {
    key: "LocStyleOSMStd",
    nameJa: "OpenStreetMap 標準 (Raster)",
    nameEn: "OpenStreetMap Standard",
    style: JSON.stringify(defaultOsmStyle, null, 2),
  },
];

export const iconSizes = [
  { value: 16, labelJa: "16 px (極小)", labelEn: "16 px (XS)" },
  { value: 24, labelJa: "24 px (標準)", labelEn: "24 px (Standard)" },
  { value: 32, labelJa: "32 px (中)", labelEn: "32 px (Medium)" },
  { value: 48, labelJa: "48 px (大)", labelEn: "48 px (Large)" },
  { value: 64, labelJa: "64 px (特大)", labelEn: "64 px (XL)" },
];

/**
 * Parses style string to a MapLibre-compatible style spec or URL.
 */
export function resolveMapLibreStyle(styleStr?: string): any {
  if (!styleStr || !styleStr.trim()) {
    return defaultOsmStyle;
  }
  const trimmed = styleStr.trim();
  if (trimmed.startsWith("{")) {
    try {
      return JSON.parse(trimmed);
    } catch (e) {
      console.warn("Failed to parse map style JSON, fallback to OSM:", e);
      return defaultOsmStyle;
    }
  }
  return trimmed;
}
