<script lang="ts">
  import { iconList, addrModeList, snmpModeList } from "../common";
  import { saveNode, type NodeEnt } from "../api";
  import { X, Save, HelpCircle, Cpu } from "@lucide/svelte";

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
  let snmpMode = $state("v2c");
  let community = $state("public");
  let user = $state("");
  let password = $state("");
  let addrMode = $state("ip");

  $effect(() => {
    if (node) {
      name = node.name || "";
      ip = node.ip || "";
      mac = node.mac || "";
      descr = node.descr || "";
      icon = node.icon || "desktop";
      snmpMode = (node as any).snmp_mode || "v2c";
      community = (node as any).community || "public";
      user = (node as any).user || "";
      password = (node as any).password || "";
      addrMode = (node as any).addr_mode || "ip";
    }
  });

  const handleSave = async () => {
    if (!name || !ip) return;
    const n: NodeEnt = {
      ...(node || { id: "", state: "normal", x: 320, y: 200 }),
      name,
      ip,
      mac,
      descr,
      icon,
      x: typeof node?.x === "number" && node.x > 0 ? node.x : 320,
      y: typeof node?.y === "number" && node.y > 0 ? node.y : 200,
    };
    (n as any).snmp_mode = snmpMode;
    (n as any).community = community;
    (n as any).user = user;
    (n as any).password = password;
    (n as any).addr_mode = addrMode;

    try {
      const saved = await saveNode(n);
      onSave(saved);
      show = false;
    } catch (e) {
      console.error("Save node error:", e);
    }
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
    <div class="w-full max-w-xl rounded-xl border border-border bg-card p-6 shadow-2xl">
      <div class="flex items-center justify-between border-b border-border pb-3">
        <div class="flex items-center gap-2 text-lg font-semibold">
          <Cpu class="h-5 w-5 text-primary" />
          <span>{node?.id ? "ノードの編集" : "ノードの追加"}</span>
        </div>
        <button onclick={() => (show = false)} class="rounded-lg p-1 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <div class="mt-4 grid grid-cols-2 gap-4 text-sm">
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">
            ノード名
            <input type="text" bind:value={name} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
          </label>
        </div>
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">
            IP アドレス
            <input type="text" bind:value={ip} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
          </label>
        </div>
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">
            MAC アドレス
            <input type="text" bind:value={mac} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
          </label>
        </div>
        <div class="col-span-2 sm:col-span-1">
          <label class="block text-xs font-medium text-muted-foreground">
            アイコン
            <select bind:value={icon} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none">
              {#each iconList as ic}
                <option value={ic.value}>{ic.name}</option>
              {/each}
            </select>
          </label>
        </div>

        <div class="col-span-2">
          <label class="block text-xs font-medium text-muted-foreground">
            説明・メモ
            <input type="text" bind:value={descr} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
          </label>
        </div>

        <!-- SNMP Settings -->
        <div class="col-span-2 mt-2 rounded-lg border border-border/60 bg-muted/30 p-3">
          <span class="text-xs font-semibold text-primary">SNMP / 認証設定</span>
          <div class="mt-2 grid grid-cols-2 gap-3">
            <div>
              <label class="block text-xs text-muted-foreground">
                SNMP バージョン
                <select bind:value={snmpMode} class="mt-1 w-full rounded-md border border-border bg-background px-2 py-1 text-xs">
                  {#each snmpModeList as sm}
                    <option value={sm.value}>{sm.name}</option>
                  {/each}
                </select>
              </label>
            </div>
            <div>
              <label class="block text-xs text-muted-foreground">
                コミュニティ名
                <input type="text" bind:value={community} class="mt-1 w-full rounded-md border border-border bg-background px-2 py-1 text-xs" />
              </label>
            </div>
            {#if snmpMode.startsWith("v3")}
              <div>
                <label class="block text-xs text-muted-foreground">
                  v3 ユーザー名
                  <input type="text" bind:value={user} class="mt-1 w-full rounded-md border border-border bg-background px-2 py-1 text-xs" />
                </label>
              </div>
              <div>
                <label class="block text-xs text-muted-foreground">
                  v3 パスワード
                  <input type="password" bind:value={password} class="mt-1 w-full rounded-md border border-border bg-background px-2 py-1 text-xs" />
                </label>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <div class="mt-6 flex justify-end gap-3 border-t border-border pt-4">
        <button onclick={() => (show = false)} class="rounded-lg border border-border px-4 py-1.5 text-sm hover:bg-muted">
          キャンセル
        </button>
        <button onclick={handleSave} class="flex items-center gap-1.5 rounded-lg bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90">
          <Save class="h-4 w-4" />
          保存
        </button>
      </div>
    </div>
  </div>
{/if}
