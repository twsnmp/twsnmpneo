<script lang="ts">
  import { X, Upload, FileUp, CheckCircle2, AlertTriangle, Database } from "@lucide/svelte";
  import { importMapData } from "../api";
  import { _ } from "svelte-i18n";

  let {
    show = $bindable(false),
    onImported,
  }: {
    show: boolean;
    onImported?: () => void;
  } = $props();

  let fileInput: HTMLInputElement | null = $state(null);
  let fileName = $state("");
  let detectedFormat = $state("");
  let parsedData = $state<{
    nodes?: any[];
    lines?: any[];
    networks?: any[];
    drawItems?: any[];
  } | null>(null);

  let importing = $state(false);
  let errorMsg = $state("");
  let successMsg = $state("");

  const convertV4Icon = (icon: string) => {
    if (!icon) return "desktop";
    switch (icon) {
      case "PC1_PC":
        return "desktop";
      case "MS_CO":
        return "mdi-microsoft-windows";
      case "NOTE_PC":
        return "laptop";
    }
    if (icon.includes("_PC")) return "desktop";
    if (icon.includes("_RT")) return "router";
    if (icon.includes("_SW")) return "switch";
    if (icon.includes("_SV")) return "server";
    return "desktop";
  };

  const convertV4SnmpMode = (m: string) => {
    switch (m) {
      case "1":
        return "v2c";
      case "3":
      case "5":
        return "v3auth";
      case "4":
      case "6":
        return "v3authpriv";
    }
    return "v2c";
  };

  const convertV4AddrMode = (m: string) => {
    switch (m) {
      case "1":
        return "mac";
      case "2":
        return "host";
    }
    return "ip";
  };

  const parseSpmFile = (text: string) => {
    const rawLines = text.split(/\r?\n/);
    let defComOrUser = "public";
    let defPassword = "";
    let defSnmpMode = "v2c";
    let lastNode: any = null;
    const nodesMap = new Map<string, any>();
    const lineEntries: { srcNode: string; dstNode: string; width: number }[] = [];

    for (const rawLine of rawLines) {
      const line = rawLine.trim();
      if (!line) continue;
      const f = line.split(/\s+/);
      if (f.length < 1) continue;

      switch (f[0]) {
        case "DEFCOMORUSER":
          if (f[1]) defComOrUser = f[1];
          break;
        case "DEFPASSWD":
          if (f[1]) defPassword = f[1];
          break;
        case "DEFSNMPMODE":
          if (f[1]) defSnmpMode = convertV4SnmpMode(f[1]);
          break;
        case "NODE":
          if (f.length > 4) {
            const x = parseInt(f[2], 10) || 0;
            const y = parseInt(f[3], 10) || 0;
            const randomPart = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID().slice(0, 8) : Date.now().toString(36);
            lastNode = {
              id: `v4_${Date.now()}_${randomPart}`,
              name: f[1],
              x,
              y,
              icon: convertV4Icon(f[4]),
              state: "normal",
              ip: "",
              mac: "",
              url: "",
              descr: "",
              vendor: "",
              snmp_mode: defSnmpMode,
              community: defComOrUser,
              user: defComOrUser,
              password: defPassword,
              addr_mode: "ip",
              auto_ack: false,
            };
          }
          break;
        case "LINE":
          if (f.length > 6) {
            const width = parseInt(f[5], 10) || 1;
            lineEntries.push({
              srcNode: f[1],
              dstNode: f[3],
              width,
            });
          }
          break;
        case "IPADDR":
          if (lastNode && f[1]) lastNode.ip = f[1];
          break;
        case "MACADDR":
          if (lastNode && f[1]) lastNode.mac = f[1];
          break;
        case "URL":
          if (lastNode && f[1]) lastNode.url = f[1];
          break;
        case "SNMP_MODE":
          if (lastNode && f[1]) lastNode.snmp_mode = convertV4SnmpMode(f[1]);
          break;
        case "COM_OR_USER":
          if (lastNode && f[1]) {
            lastNode.user = f[1];
            if (!lastNode.community) lastNode.community = f[1];
          }
          break;
        case "RCOMMUNITY":
        case "WCOMMUNITY":
          if (lastNode && f[1] && !lastNode.community) {
            lastNode.community = f[1];
          }
          break;
        case "PASSWD":
          if (lastNode && f[1]) lastNode.password = f[1];
          break;
        case "ADDRMODE":
          if (lastNode && f[1]) lastNode.addr_mode = convertV4AddrMode(f[1]);
          break;
        case "VENDOR":
          if (lastNode && f.length > 1) lastNode.vendor = f.slice(1).join(" ");
          break;
        case "AUTOACK":
          if (lastNode) lastNode.auto_ack = true;
          break;
        case "SYSCONTACT":
        case "SYSLOCATION":
        case "SYSOID":
          if (lastNode && f[1]) {
            lastNode.descr = lastNode.descr ? `${lastNode.descr}, ${f[1]}` : f[1];
          }
          break;
        case "}":
          if (lastNode) {
            nodesMap.set(lastNode.name, lastNode);
            lastNode = null;
          }
          break;
      }
    }

    const finalNodes = Array.from(nodesMap.values());
    const finalLines: any[] = [];
    for (const le of lineEntries) {
      const sn = nodesMap.get(le.srcNode);
      const dn = nodesMap.get(le.dstNode);
      if (sn && dn) {
        const randomPart = typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID().slice(0, 8) : Date.now().toString(36);
        finalLines.push({
          id: `line_${Date.now()}_${randomPart}`,
          node_id1: sn.id,
          node_id2: dn.id,
          NodeID1: sn.id,
          NodeID2: dn.id,
          width: le.width,
          Width: le.width,
          state: "normal",
        });
      }
    }

    return {
      nodes: finalNodes,
      lines: finalLines,
      networks: [],
      drawItems: [],
    };
  };

  const processFile = (file: File) => {
    errorMsg = "";
    successMsg = "";
    parsedData = null;
    detectedFormat = "";
    fileName = file.name;

    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const buffer = ev.target?.result as ArrayBuffer;
        if (!buffer || buffer.byteLength === 0) {
          errorMsg = $_("map.importModal.parseError") + ": Empty file";
          return;
        }

        // 1. Try decoding as UTF-8
        let utf8Text = "";
        try {
          utf8Text = new TextDecoder("utf-8").decode(buffer);
        } catch {
          utf8Text = "";
        }

        // Check if JSON
        const trimmed = utf8Text.trim();
        if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
          try {
            const json = JSON.parse(utf8Text);
            const nodes = json.nodes || json.Nodes || (Array.isArray(json) ? json : []);
            const lines = json.lines || json.Lines || [];
            const networks = json.networks || json.Networks || [];
            const drawItems = json.drawItems || json.DrawItems || json.items || json.Items || [];

            parsedData = {
              nodes: Array.isArray(nodes) ? nodes : Object.values(nodes),
              lines: Array.isArray(lines) ? lines : Object.values(lines),
              networks: Array.isArray(networks) ? networks : Object.values(networks),
              drawItems: Array.isArray(drawItems) ? drawItems : Object.values(drawItems),
            };
            detectedFormat = "JSON (TWSNMP NEO / FC / FK)";
            return;
          } catch {
            // Not JSON, continue to SPM parser
          }
        }

        // 2. Decode as Shift-JIS or fallback to UTF-8 for SPM map files
        let spmText = "";
        try {
          spmText = new TextDecoder("shift-jis").decode(buffer);
        } catch {
          spmText = utf8Text;
        }

        if (spmText.includes("NODE ") || spmText.includes("DEFCOMORUSER") || fileName.toLowerCase().endsWith(".spm")) {
          const result = parseSpmFile(spmText);
          if (result.nodes.length === 0 && result.lines.length === 0) {
            errorMsg = $_("map.importModal.parseError");
            return;
          }
          parsedData = result;
          detectedFormat = "TWSNMP v4 (.spm)";
          return;
        }

        // If neither recognized
        errorMsg = $_("map.importModal.unsupportedFormat");
      } catch (err: any) {
        errorMsg = $_("map.importModal.parseError") + ": " + (err.message || err);
      }
    };
    reader.readAsArrayBuffer(file);
  };

  const handleFileChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    processFile(target.files[0]);
  };

  const handleImport = async () => {
    if (!parsedData) return;
    importing = true;
    errorMsg = "";
    try {
      const res = await importMapData(parsedData);
      successMsg = $_("map.importModal.successMsg", {
        values: {
          nodes: res.nodes ?? parsedData.nodes?.length ?? 0,
          lines: res.lines ?? parsedData.lines?.length ?? 0,
        },
      });
      onImported?.();
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
      }
      setTimeout(() => {
        show = false;
        parsedData = null;
        fileName = "";
        successMsg = "";
      }, 1500);
    } catch (err: any) {
      errorMsg = $_("map.importModal.importError") + ": " + (err.message || err);
    } finally {
      importing = false;
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => {
      if (e.key === "Escape") show = false;
    }}
  >
    <div
      class="w-full max-w-md rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b1329] p-5 shadow-2xl transition-colors text-slate-800 dark:text-slate-200"
    >
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-100 dark:border-slate-800/80 pb-3">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/10 border border-cyan-500/20 text-cyan-600 dark:text-cyan-400">
            <Database class="h-4 w-4" />
          </div>
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
            {$_("map.importModal.title")}
          </h3>
        </div>
        <button
          type="button"
          aria-label={$_("common.close")}
          onclick={() => (show = false)}
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 hover:text-slate-600 dark:hover:text-slate-200"
        >
          <X class="h-4 w-4" />
        </button>
      </div>

      <!-- Body -->
      <div class="space-y-4 py-4 text-xs">
        {#if errorMsg}
          <div class="flex items-center gap-2 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 p-3 text-rose-800 dark:text-rose-300">
            <AlertTriangle class="h-4 w-4 shrink-0 text-rose-500" />
            <span>{errorMsg}</span>
          </div>
        {/if}
        {#if successMsg}
          <div class="flex items-center gap-2 rounded-xl border border-emerald-300 dark:border-emerald-800/60 bg-emerald-50 dark:bg-emerald-950/40 p-3 text-emerald-800 dark:text-emerald-300">
            <CheckCircle2 class="h-4 w-4 shrink-0 text-emerald-500" />
            <span>{successMsg}</span>
          </div>
        {/if}

        <div>
          <span class="block font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_("map.importModal.selectFile")}
          </span>
          <input
            type="file"
            accept=".json,.spm,.dat,.txt,*"
            bind:this={fileInput}
            onchange={handleFileChange}
            class="hidden"
          />
          <button
            type="button"
            onclick={() => fileInput?.click()}
            ondragover={(e) => { e.preventDefault(); }}
            ondrop={(e) => {
              e.preventDefault();
              if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
                processFile(e.dataTransfer.files[0]);
              }
            }}
            class="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-slate-300 dark:border-slate-700 hover:border-cyan-500 dark:hover:border-cyan-400 bg-slate-50 dark:bg-slate-900/60 p-5 text-slate-600 dark:text-slate-300 transition-colors cursor-pointer"
          >
            <FileUp class="h-5 w-5 text-cyan-500" />
            <span>{fileName || $_("map.importModal.dropOrClick")}</span>
          </button>
        </div>

        {#if parsedData}
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/80 p-3 space-y-2">
            <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-1.5">
              <span class="font-semibold text-slate-800 dark:text-slate-200">
                {$_("map.importModal.previewTitle")}
              </span>
              {#if detectedFormat}
                <span class="rounded-full px-2 py-0.5 text-[10px] font-medium bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
                  {detectedFormat}
                </span>
              {/if}
            </div>
            <div class="grid grid-cols-2 gap-2 text-[11px] font-mono text-slate-600 dark:text-slate-400">
              <div>{$_("map.importModal.nodes")}: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{parsedData.nodes?.length ?? 0}</span></div>
              <div>{$_("map.importModal.lines")}: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{parsedData.lines?.length ?? 0}</span></div>
              <div>{$_("map.importModal.networks")}: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{parsedData.networks?.length ?? 0}</span></div>
              <div>{$_("map.importModal.drawItems")}: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{parsedData.drawItems?.length ?? 0}</span></div>
            </div>
          </div>
        {/if}
      </div>

      <!-- Actions -->
      <div class="flex items-center justify-end gap-2 border-t border-slate-100 dark:border-slate-800/80 pt-3.5">
        <button
          type="button"
          onclick={() => (show = false)}
          class="rounded-xl px-3.5 py-2 text-xs font-medium text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-700 transition-colors"
        >
          {$_("common.cancel")}
        </button>
        <button
          type="button"
          disabled={!parsedData || importing}
          onclick={handleImport}
          class="flex items-center gap-1.5 rounded-xl border border-cyan-500 bg-gradient-to-r from-cyan-600 to-cyan-500 px-4 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 hover:from-cyan-500 hover:to-cyan-400 transition-all cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Upload class="h-3.5 w-3.5" />
          <span>{importing ? $_("common.loading") : $_("map.importModal.execute")}</span>
        </button>
      </div>
    </div>
  </div>
{/if}
