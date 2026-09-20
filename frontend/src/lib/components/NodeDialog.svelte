<script lang="ts">
  import { iconList, addrModeList, snmpModeList } from "../common";
  import { saveNode, type NodeEnt } from "../api";
  import { X, Save, Cpu, Shield, Key, Network, Globe } from "@lucide/svelte";

  let { show = $bindable(false), node = $bindable<NodeEnt | null>(null), onSave = () => {} } = $props<{
    show: boolean;
    node: NodeEnt | null;
    onSave?: (saved: NodeEnt) => void;
  }>();

  let name = $state("");
  let ip = $state("");
  let mac = $state("");
  let descr = $state("");
  let icon = $state("desktop");
  let addrMode = $state("ip");
  let autoAck = $state(false);
  let url = $state("");

  // SNMP
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpPort = $state(161);
  let user = $state("");
  let password = $state("");

  // SSH
  let sshUser = $state("");
  let publicKey = $state("");

  let saveError = $state("");

  $effect(() => {
    if (show) {
      saveError = "";
      if (node) {
        name = node.name || (node as any).Name || "";
        ip = node.ip || (node as any).IP || "";
        mac = node.mac || (node as any).MAC || "";
        descr = node.descr || (node as any).Descr || "";
        icon = node.icon || (node as any).Icon || "desktop";
        addrMode = (node as any).addr_mode || (node as any).AddrMode || "ip";
        autoAck = (node as any).auto_ack ?? (node as any).AutoAck ?? false;
        url = (node as any).url || (node as any).URL || "";

        snmpMode = (node as any).snmp_mode || (node as any).SnmpMode || "v2c";
        community = (node as any).community || (node as any).Community || "public";
        snmpPort = Number((node as any).snmp_port || (node as any).SnmpPort || 161);
        user = (node as any).user || (node as any).User || "";
        password = (node as any).password || (node as any).Password || "";

        sshUser = (node as any).ssh_user || (node as any).SSHUser || "";
        publicKey = (node as any).public_key || (node as any).PublicKey || "";
      } else {
        name = "新規ノード";
        ip = "192.168.1.10";
        mac = "";
        descr = "";
        icon = "desktop";
        addrMode = "ip";
        autoAck = false;
        url = "";
        snmpMode = "v2c";
        community = "public";
        snmpPort = 161;
        user = "";
        password = "";
        sshUser = "";
        publicKey = "";
      }
    }
  });

  const handleSave = async () => {
    if (!name || !ip) {
      saveError = "ノード名とIPアドレスは必須入力です。";
      return;
    }
    const n: any = {
      ...(node || { id: "", state: "normal" }),
      name,
      ip,
      mac,
      descr,
      icon,
      addr_mode: addrMode,
      auto_ack: autoAck,
      url,
      snmp_mode: snmpMode,
      community,
      snmp_port: Number(snmpPort) || 161,
      user,
      password,
      ssh_user: sshUser,
      public_key: publicKey,
      x: typeof node?.x === "number" && node.x > 0 ? node.x : 320,
      y: typeof node?.y === "number" && node.y > 0 ? node.y : 200,
    };

    try {
      const saved = await saveNode(n);
      onSave(saved);
      show = false;
    } catch (e: any) {
      saveError = "保存エラー: " + (e.message || e);
    }
  };
</script>

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-auto max-h-[90vh] w-full max-w-2xl flex-col rounded-2xl border border-slate-800 bg-[#0b1329] shadow-2xl overflow-hidden text-slate-200">
      <!-- Modal Header (twnoaa style) -->
      <div class="flex items-center justify-between border-b border-slate-800/80 bg-slate-900/60 px-6 py-4 shrink-0">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Cpu class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-100">{node?.id ? "ノードの編集 (Edit Node)" : "ノードの追加 (Add Node)"}</h2>
            <p class="text-[11px] text-slate-400">ネットワーク監視対象機器の基本情報および接続パラメータ設定</p>
          </div>
        </div>
        <button
          type="button"
          aria-label="閉じる"
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors cursor-pointer"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="flex-1 overflow-y-auto p-6 space-y-5 bg-slate-900/40 text-xs">
        {#if saveError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{saveError}</span>
          </div>
        {/if}

        <!-- Section 1: Basic Information -->
        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-2.5">
            <Network class="w-4 h-4 text-cyan-400" />
            基本情報
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label for="node-name" class="block text-xs font-semibold text-slate-400 mb-1.5">
                ノード名称 <span class="text-rose-400">*</span>
              </label>
              <input
                id="node-name"
                type="text"
                bind:value={name}
                placeholder="例: Web-Server-01"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-100 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-ip" class="block text-xs font-semibold text-slate-400 mb-1.5">
                IP アドレス / ホスト名 <span class="text-rose-400">*</span>
              </label>
              <input
                id="node-ip"
                type="text"
                bind:value={ip}
                placeholder="192.168.1.10"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-mac" class="block text-xs font-semibold text-slate-400 mb-1.5">MAC アドレス</label>
              <input
                id="node-mac"
                type="text"
                bind:value={mac}
                placeholder="00:11:22:33:44:55"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="node-icon" class="block text-xs font-semibold text-slate-400 mb-1.5">ノードアイコン</label>
              <select
                id="node-icon"
                bind:value={icon}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                {#each iconList as ic}
                  <option value={ic.value}>{ic.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="node-addr-mode" class="block text-xs font-semibold text-slate-400 mb-1.5">アドレス解決モード</label>
              <select
                id="node-addr-mode"
                bind:value={addrMode}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                {#each addrModeList as am}
                  <option value={am.value}>{am.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="node-url" class="block text-xs font-semibold text-slate-400 mb-1.5">Web管理コンソール URL</label>
              <input
                id="node-url"
                type="text"
                bind:value={url}
                placeholder="https://192.168.1.10:8443/"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2">
              <label for="node-descr" class="block text-xs font-semibold text-slate-400 mb-1.5">説明・設置場所・メモ</label>
              <input
                id="node-descr"
                type="text"
                bind:value={descr}
                placeholder="例: 本社 3F サーバールーム Rack-A"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div class="md:col-span-2 pt-1">
              <label class="flex items-center gap-2.5 cursor-pointer">
                <input
                  type="checkbox"
                  bind:checked={autoAck}
                  class="h-4 w-4 rounded border-slate-700 bg-slate-900 text-cyan-500 focus:ring-cyan-500/20"
                />
                <span class="text-xs text-slate-300 font-medium">障害復旧時に自動で確認状態（Auto Ack）にする</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Section 2: SNMP & Authentication -->
        <div class="rounded-2xl border border-slate-800 bg-slate-900/80 p-5 shadow-lg space-y-4">
          <h3 class="text-xs font-bold text-slate-100 flex items-center gap-2 border-b border-slate-800 pb-2.5">
            <Shield class="w-4 h-4 text-cyan-400" />
            SNMP & 認証設定
          </h3>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label for="snmp-ver" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMP モード</label>
              <select
                id="snmp-ver"
                bind:value={snmpMode}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              >
                {#each snmpModeList as sm}
                  <option value={sm.value}>{sm.name}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="snmp-community" class="block text-xs font-semibold text-slate-400 mb-1.5">コミュニティ名 (v1/v2c)</label>
              <input
                id="snmp-community"
                type="text"
                bind:value={community}
                placeholder="public"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>

            <div>
              <label for="snmp-port" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMP ポート</label>
              <input
                id="snmp-port"
                type="number"
                min={1}
                max={65535}
                bind:value={snmpPort}
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-400 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
          </div>

          {#if snmpMode.startsWith("v3")}
            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-800/80">
              <div>
                <label for="snmp-user" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMPv3 ユーザー名</label>
                <input
                  id="snmp-user"
                  type="text"
                  bind:value={user}
                  placeholder="v3user"
                  class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
              <div>
                <label for="snmp-pass" class="block text-xs font-semibold text-slate-400 mb-1.5">SNMPv3 パスワード / 認証鍵</label>
                <input
                  id="snmp-pass"
                  type="password"
                  bind:value={password}
                  placeholder="••••••••"
                  class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
                />
              </div>
            </div>
          {/if}

          <!-- SSH Section -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-800/80">
            <div>
              <label for="ssh-user" class="block text-xs font-semibold text-slate-400 mb-1.5">SSH ログインユーザー (任意)</label>
              <input
                id="ssh-user"
                type="text"
                bind:value={sshUser}
                placeholder="admin"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
            <div>
              <label for="ssh-key" class="block text-xs font-semibold text-slate-400 mb-1.5">SSH 公開鍵 / 鍵識別名</label>
              <input
                id="ssh-key"
                type="text"
                bind:value={publicKey}
                placeholder="id_ed25519"
                class="w-full rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-200 focus:border-cyan-500 focus:outline-none transition-colors"
              />
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer (twnoaa style) -->
      <div class="flex items-center justify-end gap-3 border-t border-slate-800/80 bg-slate-900/60 px-6 py-3.5 shrink-0">
        <button
          type="button"
          onclick={() => (show = false)}
          class="px-4 py-2 bg-slate-900 hover:bg-slate-800 border border-slate-800 rounded-xl text-xs font-medium text-slate-300 transition-colors cursor-pointer"
        >
          キャンセル
        </button>
        <button
          type="button"
          onclick={handleSave}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
        >
          <Save class="w-4 h-4" />
          保存
        </button>
      </div>
    </div>
  </div>
{/if}
