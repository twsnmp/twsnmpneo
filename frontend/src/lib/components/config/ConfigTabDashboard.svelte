<script lang="ts">
  import { onMount } from "svelte";
  import {
    fetchNodes,
    fetchPollings,
    type NodeEnt,
    type PollingEnt,
    type PublicDashboardItemEnt,
    type PublicItemAlertConf,
  } from "../../api";
  import {
    LayoutDashboard,
    Key,
    Copy,
    Check,
    ExternalLink,
    Code,
    Plus,
    Trash2,
    ArrowUp,
    ArrowDown,
    Activity,
    Network,
    ShieldAlert,
    Volume2,
    VolumeX,
    Sparkles,
    Edit3,
    Bell,
    BellOff,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    enablePublicDashboard = $bindable(false),
    publicDashboardKey = $bindable(""),
    publicDashboardItems = $bindable<PublicDashboardItemEnt[]>([]),
    onError,
    onSuccess,
  }: {
    enablePublicDashboard: boolean;
    publicDashboardKey: string;
    publicDashboardItems: PublicDashboardItemEnt[];
    onError?: (msg: string) => void;
    onSuccess?: (msg: string) => void;
  } = $props();

  let nodes = $state<NodeEnt[]>([]);
  let pollings = $state<PollingEnt[]>([]);
  let copiedUrl = $state(false);
  let copiedIframe = $state(false);

  // Modal State for Add / Edit Item
  let showItemModal = $state(false);
  let editingItemIndex = $state<number | null>(null); // null = add new, number = edit
  let formType = $state<"map" | "polling">("map");
  let formNodeId = $state("");
  let formPollingId = $state("");
  let formTitle = $state("");
  let formAlertEnable = $state(true);
  let formAlertLevel = $state<"warn" | "high">("warn");
  let formAlertSound = $state(false);

  const filteredPollings = $derived.by(() => {
    if (!formNodeId) return pollings;
    return pollings.filter((p) => {
      const pNodeId = p.node_id || (p as any).NodeID;
      return pNodeId === formNodeId;
    });
  });

  const dashboardUrl = $derived.by(() => {
    if (typeof window === "undefined" || !publicDashboardKey) return "";
    return `${window.location.origin}/dashboard/${publicDashboardKey}`;
  });

  const iframeCode = $derived.by(() => {
    if (!dashboardUrl) return "";
    return `<iframe src="${dashboardUrl}?embed=true" width="100%" height="600" frameborder="0" allowfullscreen></iframe>`;
  });

  const generateRandomKey = () => {
    const arr = new Uint8Array(16);
    crypto.getRandomValues(arr);
    publicDashboardKey = Array.from(arr)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  };

  const copyUrl = async () => {
    if (!dashboardUrl) return;
    try {
      await navigator.clipboard.writeText(dashboardUrl);
      copiedUrl = true;
      setTimeout(() => (copiedUrl = false), 2000);
      onSuccess?.($_('config.dashboardUrlCopied') || "公開ダッシュボードのURLをコピーしました。");
    } catch {
      onError?.($_('config.dashboardCopyFailed') || "URLのコピーに失敗しました。");
    }
  };

  const copyIframe = async () => {
    if (!iframeCode) return;
    try {
      await navigator.clipboard.writeText(iframeCode);
      copiedIframe = true;
      setTimeout(() => (copiedIframe = false), 2000);
      onSuccess?.($_('config.dashboardIframeCopied') || "埋め込み用 iframe タグをコピーしました。");
    } catch {
      onError?.($_('config.dashboardTagCopyFailed') || "タグのコピーに失敗しました。");
    }
  };

  const openPreview = () => {
    if (!dashboardUrl) return;
    window.open(dashboardUrl, "_blank");
  };

  const openAddModal = () => {
    editingItemIndex = null;
    formType = "map";
    formNodeId = nodes[0]?.id || (nodes[0] as any)?.ID || "";
    formPollingId = "";
    formTitle = "";
    formAlertEnable = true;
    formAlertLevel = "warn";
    formAlertSound = false;
    showItemModal = true;
  };

  const openEditModal = (idx: number) => {
    const item = publicDashboardItems[idx];
    if (!item) return;
    editingItemIndex = idx;
    formType = item.type;
    formTitle = item.title || "";
    formPollingId = item.pollingId || (item as any).PollingID || "";
    if (item.type === "polling" && formPollingId) {
      const p = pollings.find((x) => (x.id || (x as any).ID) === formPollingId);
      if (p) {
        formNodeId = (p.node_id || (p as any).NodeID) || "";
      }
    } else {
      formNodeId = nodes[0]?.id || (nodes[0] as any)?.ID || "";
    }
    formAlertEnable = item.alert ? item.alert.enable : true;
    formAlertLevel = item.alert?.level || "warn";
    formAlertSound = item.alert?.sound || false;
    showItemModal = true;
  };

  const saveItemForm = () => {
    if (formType === "polling" && !formPollingId) {
      onError?.($_('config.dashboardSelectPollingError') || "ポーリングを選択してください。");
      return;
    }
    let title = formTitle.trim();
    if (!title) {
      if (formType === "map") {
        title = $_('config.dashboardDefaultMapTitle') || "ネットワークマップ";
      } else {
        const p = pollings.find((x) => (x.id || (x as any).ID) === formPollingId);
        title = p ? (p.name || (p as any).Name) : ($_('config.dashboardDefaultPollingTitle') || "ポーリング");
      }
    }

    const alertConfig: PublicItemAlertConf = {
      enable: formAlertEnable,
      level: formAlertLevel,
      sound: formAlertSound,
      soundTimeout: 30,
    };

    if (editingItemIndex !== null && editingItemIndex >= 0) {
      // Edit existing item
      const copy = [...publicDashboardItems];
      copy[editingItemIndex] = {
        ...copy[editingItemIndex],
        type: formType,
        pollingId: formType === "polling" ? formPollingId : undefined,
        title,
        alert: alertConfig,
      };
      publicDashboardItems = copy;
      onSuccess?.($_('config.dashboardItemUpdated') || "公開アイテムの設定を更新しました。");
    } else {
      // Add new item
      const newItem: PublicDashboardItemEnt = {
        id: "item-" + Date.now(),
        type: formType,
        pollingId: formType === "polling" ? formPollingId : undefined,
        title,
        alert: alertConfig,
      };
      publicDashboardItems = [...publicDashboardItems, newItem];
      onSuccess?.($_('config.dashboardItemAdded') || "公開アイテムを追加しました。");
    }
    showItemModal = false;
  };

  const removeItem = (index: number) => {
    publicDashboardItems = publicDashboardItems.filter((_, i) => i !== index);
  };

  const moveItem = (index: number, dir: -1 | 1) => {
    const target = index + dir;
    if (target < 0 || target >= publicDashboardItems.length) return;
    const copy = [...publicDashboardItems];
    const temp = copy[index];
    copy[index] = copy[target];
    copy[target] = temp;
    publicDashboardItems = copy;
  };

  const getItemTargetName = (item: PublicDashboardItemEnt) => {
    if (item.type === "map") return $_('config.dashboardEntireNetwork') || "ネットワーク全体";
    const pid = item.pollingId || (item as any).PollingID;
    const p = pollings.find((x) => (x.id || (x as any).ID) === pid);
    if (!p) return pid || "-";
    const pNodeId = p.node_id || (p as any).NodeID;
    const node = nodes.find((n) => (n.id || (n as any).ID) === pNodeId);
    const pName = p.name || (p as any).Name;
    const nName = node ? (node.name || (node as any).Name) : "";
    return nName ? `${nName} / ${pName}` : pName;
  };

  onMount(async () => {
    if (!publicDashboardKey) {
      generateRandomKey();
    }
    try {
      const [nList, pList] = await Promise.all([
        fetchNodes().catch(() => []),
        fetchPollings().catch(() => []),
      ]);
      nodes = nList || [];
      pollings = pList || [];
    } catch {}
  });
</script>

<div class="space-y-6 max-w-2xl text-xs">
  <!-- 1. Enable Toggle & Key Configuration -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <div class="flex items-center gap-2">
        <LayoutDashboard class="w-4 h-4 text-cyan-500" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
          {$_('config.dashboardBasicSettings') || '公開ダッシュボードの基本設定'}
        </h3>
      </div>
      <!-- Primary Enable/Disable Toggle -->
      <label class="relative inline-flex items-center cursor-pointer">
        <input type="checkbox" bind:checked={enablePublicDashboard} class="sr-only peer" />
        <div class="w-11 h-6 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-800 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-cyan-500"></div>
      </label>
    </div>

    <p class="text-[11px] text-slate-500 dark:text-slate-400 leading-relaxed">
      {$_('config.dashboardBasicDesc') || '公開ダッシュボードを有効にすると、生成されたランダムURLを通じてログインなしで安全にマップや選択したポーリング情報を閲覧・共有できます。IPアドレスや認証資格情報などの機密情報は自動的に保護されます。'}
    </p>

    <!-- Key & URL controls (Disabled if toggle is OFF) -->
    <div class="space-y-3 pt-1 transition-all {enablePublicDashboard ? '' : 'opacity-40 pointer-events-none select-none'}">
      <div>
        <label for="input-public-key" class="block font-semibold text-slate-700 dark:text-slate-300 mb-1 text-[11px]">
          {$_('config.dashboardAccessKey') || '公開アクセスキー (URLトークン)'}
        </label>
        <div class="flex items-center gap-2">
          <div class="relative flex-1">
            <Key class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              id="input-public-key"
              type="text"
              disabled={!enablePublicDashboard}
              bind:value={publicDashboardKey}
              placeholder={$_('config.dashboardKeyPlaceholder') || '32文字のランダム英数字'}
              class="w-full pl-9 pr-3 py-1.5 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 font-mono text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:border-cyan-500"
            />
          </div>
          <button
            type="button"
            disabled={!enablePublicDashboard}
            onclick={generateRandomKey}
            class="px-3 py-1.5 rounded-xl border border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-200 text-xs font-semibold transition-colors cursor-pointer"
          >
            {$_('config.dashboardRegenerateKey') || '再生成'}
          </button>
        </div>
      </div>

      {#if enablePublicDashboard && publicDashboardKey}
        <!-- URL Copy & Preview -->
        <div class="p-3.5 rounded-xl border border-cyan-500/20 bg-cyan-500/5 space-y-2.5">
          <div class="flex items-center justify-between">
            <span class="font-bold text-cyan-600 dark:text-cyan-400 text-[11px] flex items-center gap-1.5">
              <ExternalLink class="w-3.5 h-3.5" />
              {$_('config.dashboardPublicUrl') || '公開用URL'}
            </span>
            <div class="flex items-center gap-1.5">
              <button
                type="button"
                onclick={copyUrl}
                class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 font-semibold text-[10px] transition-colors cursor-pointer"
              >
                {#if copiedUrl}
                  <Check class="w-3 h-3 text-emerald-500" />
                  <span class="text-emerald-500">{$_('common.copied') || 'コピー完了'}</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>{$_('config.dashboardCopyUrl') || 'URLをコピー'}</span>
                {/if}
              </button>

              <button
                type="button"
                onclick={openPreview}
                class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 font-semibold text-[10px] transition-colors cursor-pointer"
              >
                <ExternalLink class="w-3 h-3" />
                <span>{$_('config.dashboardOpenPreview') || '開く'}</span>
              </button>
            </div>
          </div>
          <div class="p-2 rounded-lg bg-white/60 dark:bg-slate-950/80 border border-slate-200 dark:border-slate-800 font-mono text-[10px] text-slate-600 dark:text-slate-300 break-all select-all">
            {dashboardUrl}
          </div>

          <!-- Embed Iframe Code -->
          <div class="flex items-center justify-between pt-1">
            <span class="font-bold text-slate-700 dark:text-slate-300 text-[11px] flex items-center gap-1.5">
              <Code class="w-3.5 h-3.5 text-cyan-500" />
              {$_('config.dashboardEmbedIframe') || '外部ページへの埋め込み (iframe)'}
            </span>
            <button
              type="button"
              onclick={copyIframe}
              class="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 font-semibold text-[10px] transition-colors cursor-pointer"
            >
              {#if copiedIframe}
                <Check class="w-3 h-3 text-emerald-500" />
                <span class="text-emerald-500">{$_('common.copied') || 'コピー完了'}</span>
              {:else}
                <Copy class="w-3 h-3" />
                <span>{$_('config.dashboardCopyIframe') || 'iframeタグをコピー'}</span>
              {/if}
            </button>
          </div>
          <div class="p-2 rounded-lg bg-white/60 dark:bg-slate-950/80 border border-slate-200 dark:border-slate-800 font-mono text-[10px] text-slate-500 dark:text-slate-400 break-all select-all">
            {iframeCode}
          </div>
        </div>
      {/if}
    </div>
  </div>

  <!-- 2. Registered Public Items (Multiple Items Support with Individual Alerts) -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm space-y-4 transition-all {enablePublicDashboard ? '' : 'opacity-40 pointer-events-none select-none'}">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <div class="flex items-center gap-2">
        <Activity class="w-4 h-4 text-cyan-500" />
        <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
          {$_('config.dashboardItemsTitle') || '公開アイテムの複数登録 & 個別警報設定'}
        </h3>
      </div>
      <button
        type="button"
        disabled={!enablePublicDashboard}
        onclick={openAddModal}
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-500/40 bg-cyan-50 dark:bg-cyan-950/40 hover:bg-cyan-100 dark:hover:bg-cyan-900/60 text-cyan-700 dark:text-cyan-300 text-xs font-bold transition-colors cursor-pointer"
      >
        <Plus class="w-3.5 h-3.5" />
        <span>{$_('config.dashboardAddItem') || 'アイテムを追加'}</span>
      </button>
    </div>

    <p class="text-[11px] text-slate-500 dark:text-slate-400">
      {$_('config.dashboardItemsDesc') || 'ダッシュボード上に公開する「マップ」や「ポーリング」を選択して複数登録できます。各アイテムごとに警報画面（Drama Alert）の発火条件を個別に設定できます。'}
    </p>

    <!-- Items Table -->
    {#if publicDashboardItems.length > 0}
      <div class="overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-800">
        <table class="w-full text-left text-[11px] border-collapse">
          <thead class="bg-slate-50 dark:bg-slate-950/70 border-b border-slate-200 dark:border-slate-800 text-[10px] uppercase font-semibold text-slate-500 whitespace-nowrap">
            <tr>
              <th class="py-1.5 px-2.5 w-8 text-center">#</th>
              <th class="py-1.5 px-2.5 w-24">{$_('config.dashboardColType') || '種類'}</th>
              <th class="py-1.5 px-2.5">{$_('config.dashboardColTitle') || '表示タイトル'}</th>
              <th class="py-1.5 px-2.5">{$_('config.dashboardColTarget') || '監視対象'}</th>
              <th class="py-1.5 px-2.5">{$_('config.dashboardColAlert') || '警報設定'}</th>
              <th class="py-1.5 px-2.5 w-20 text-right">{$_('common.actions') || '操作'}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-sans">
            {#each publicDashboardItems as item, idx}
              <tr class="hover:bg-slate-50/50 dark:hover:bg-slate-800/30">
                <td class="py-1.5 px-2.5 text-slate-400 font-mono text-center">
                  {idx + 1}
                </td>
                <td class="py-1.5 px-2.5 whitespace-nowrap">
                  <span class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-[10px] font-semibold whitespace-nowrap shrink-0 {item.type === 'map' ? 'bg-blue-50 dark:bg-blue-950/40 text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800' : 'bg-emerald-50 dark:bg-emerald-950/40 text-emerald-600 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800'}">
                    {#if item.type === 'map'}
                      <Network class="w-3 h-3 shrink-0" />
                      <span class="whitespace-nowrap">{$_('config.dashboardTypeMap') || 'マップ'}</span>
                    {:else}
                      <Activity class="w-3 h-3 shrink-0" />
                      <span class="whitespace-nowrap">{$_('config.dashboardTypePolling') || 'ポーリング'}</span>
                    {/if}
                  </span>
                </td>
                <td class="py-1.5 px-2.5 font-medium text-slate-800 dark:text-slate-200 whitespace-nowrap">
                  {item.title}
                </td>
                <td class="py-1.5 px-2.5 text-slate-500 dark:text-slate-400 text-[10.5px] truncate max-w-[140px]">
                  {getItemTargetName(item)}
                </td>
                <td class="py-1.5 px-2.5 whitespace-nowrap">
                  {#if item.alert && item.alert.enable}
                    <span class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-bold bg-rose-50 dark:bg-rose-950/40 text-rose-600 dark:text-rose-400 border border-rose-200 dark:border-rose-800 shrink-0">
                      <ShieldAlert class="w-3 h-3 shrink-0" />
                      <span class="whitespace-nowrap">{item.alert.level === 'high' ? ($_('config.dashboardAlertLevelHighOnly') || '重度のみ') : ($_('config.dashboardAlertLevelWarnPlus') || '注意以上')}</span>
                    </span>
                  {:else}
                    <span class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-medium bg-slate-100 dark:bg-slate-800 text-slate-500 dark:text-slate-400 shrink-0">
                      <BellOff class="w-3 h-3 shrink-0" />
                      <span class="whitespace-nowrap">{$_('config.dashboardAlertOff') || '警報OFF'}</span>
                    </span>
                  {/if}
                </td>
                <td class="py-1.5 px-2.5 text-right whitespace-nowrap">
                  <div class="inline-flex items-center gap-0.5 justify-end">
                    <button
                      type="button"
                      title={$_('config.dashboardEditItem') || 'アイテム設定を編集'}
                      onclick={() => openEditModal(idx)}
                      class="p-1 rounded text-slate-500 hover:text-cyan-600 hover:bg-cyan-50 dark:hover:bg-cyan-950/30 transition-colors cursor-pointer"
                    >
                      <Edit3 class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      disabled={idx === 0}
                      onclick={() => moveItem(idx, -1)}
                      class="p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 disabled:opacity-20 cursor-pointer"
                    >
                      <ArrowUp class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      disabled={idx === publicDashboardItems.length - 1}
                      onclick={() => moveItem(idx, 1)}
                      class="p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 disabled:opacity-20 cursor-pointer"
                    >
                      <ArrowDown class="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      title={$_('config.dashboardDeleteItem') || 'アイテムを削除'}
                      onclick={() => removeItem(idx)}
                      class="p-1 rounded text-rose-400 hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-950/30 transition-colors cursor-pointer"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <div class="p-4 rounded-xl border border-dashed border-slate-200 dark:border-slate-800 text-center text-slate-400 text-xs">
        {$_('config.dashboardNoItems') || '登録されたアイテムはありません。「アイテムを追加」から追加してください。'}
      </div>
    {/if}
  </div>
</div>

<!-- Add / Edit Item Modal with Individual Alert Settings -->
{#if showItemModal}
  <div class="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
    <div class="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl p-5 max-w-md w-full shadow-2xl space-y-4">
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
        <h4 class="font-bold text-sm text-slate-900 dark:text-slate-100 flex items-center gap-2">
          {#if editingItemIndex !== null}
            <Edit3 class="w-4 h-4 text-cyan-500" />
            {$_('config.dashboardEditItemModalTitle') || '公開アイテムの編集'}
          {:else}
            <Plus class="w-4 h-4 text-cyan-500" />
            {$_('config.dashboardAddItemModalTitle') || '公開アイテムの追加'}
          {/if}
        </h4>
        <button
          type="button"
          onclick={() => (showItemModal = false)}
          class="p-1 rounded text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 cursor-pointer"
        >
          ✕
        </button>
      </div>

      <div class="space-y-3.5 text-xs">
        <!-- 1. Item Type Selector -->
        <div>
          <span class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">
            {$_('config.dashboardFormType') || 'アイテムの種類'}
          </span>
          <div class="grid grid-cols-2 gap-2">
            <button
              type="button"
              onclick={() => (formType = "map")}
              class="p-2.5 rounded-xl border text-center font-bold flex items-center justify-center gap-1.5 transition-all {formType === 'map' ? 'border-cyan-500 bg-cyan-50 dark:bg-cyan-950/40 text-cyan-600 dark:text-cyan-300' : 'border-slate-200 dark:border-slate-800 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800/40'}"
            >
              <Network class="w-4 h-4" />
              <span>{$_('config.dashboardFormTypeMap') || 'マップ (Map)'}</span>
            </button>
            <button
              type="button"
              onclick={() => (formType = "polling")}
              class="p-2.5 rounded-xl border text-center font-bold flex items-center justify-center gap-1.5 transition-all {formType === 'polling' ? 'border-cyan-500 bg-cyan-50 dark:bg-cyan-950/40 text-cyan-600 dark:text-cyan-300' : 'border-slate-200 dark:border-slate-800 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800/40'}"
            >
              <Activity class="w-4 h-4" />
              <span>{$_('config.dashboardFormTypePolling') || 'ポーリング (Polling)'}</span>
            </button>
          </div>
        </div>

        {#if formType === "polling"}
          <div>
            <label for="select-new-node" class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">
              {$_('config.dashboardFormSelectNode') || '対象ノードの選択'}
            </label>
            <select
              id="select-new-node"
              bind:value={formNodeId}
              class="w-full px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 focus:outline-none focus:border-cyan-500"
            >
              <option value="">{$_('config.dashboardFormAllNodes') || 'すべてのノード'}</option>
              {#each nodes as n}
                <option value={n.id || (n as any).ID}>{(n.name || (n as any).Name)} ({(n.ip || (n as any).IP)})</option>
              {/each}
            </select>
          </div>

          <div>
            <label for="select-new-polling" class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">
              {$_('config.dashboardFormSelectPolling') || '公開するポーリングの選択'}
            </label>
            <select
              id="select-new-polling"
              bind:value={formPollingId}
              class="w-full px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 focus:outline-none focus:border-cyan-500"
            >
              <option value="">{$_('config.dashboardFormSelectPlaceholder') || '選択してください'}</option>
              {#each filteredPollings as p}
                <option value={p.id || (p as any).ID}>{(p.name || (p as any).Name)} [{(p.type || (p as any).Type)}]</option>
              {/each}
            </select>
          </div>
        {/if}

        <div>
          <label for="input-new-title" class="block font-semibold text-slate-700 dark:text-slate-300 mb-1">
            {$_('config.dashboardFormTitle') || '表示タイトル (省略可)'}
          </label>
          <input
            id="input-new-title"
            type="text"
            bind:value={formTitle}
            placeholder={formType === 'map' ? ($_('config.dashboardFormTitleMapPlaceholder') || 'ネットワークマップ') : ($_('config.dashboardFormTitlePollingPlaceholder') || '例: サーバ室温度')}
            class="w-full px-3 py-2 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 focus:outline-none focus:border-cyan-500"
          />
        </div>

        <!-- Individual Drama Alert Settings for this item -->
        <div class="p-3.5 rounded-xl border border-rose-500/30 bg-rose-50/50 dark:bg-rose-950/20 space-y-3">
          <div class="flex items-center justify-between">
            <span class="font-bold text-rose-600 dark:text-rose-400 flex items-center gap-1.5 text-xs">
              <ShieldAlert class="w-3.5 h-3.5" />
              {$_('config.dashboardFormAlertTitle') || 'この画面の警報 (Drama Alert) 設定'}
            </span>
            <label class="relative inline-flex items-center cursor-pointer">
              <input type="checkbox" bind:checked={formAlertEnable} class="sr-only peer" />
              <div class="w-8 h-4.5 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-800 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-3.5 after:w-3.5 after:transition-all peer-checked:bg-rose-500"></div>
            </label>
          </div>

          {#if formAlertEnable}
            <div class="space-y-2.5 pt-1">
              <div>
                <label for="select-item-alert-level" class="block font-semibold text-slate-700 dark:text-slate-300 mb-1 text-[11px]">
                  {$_('config.dashboardFormAlertCondition') || '警報発火条件'}
                </label>
                <select
                  id="select-item-alert-level"
                  bind:value={formAlertLevel}
                  class="w-full px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 text-xs text-slate-800 dark:text-slate-200 focus:outline-none focus:border-rose-500"
                >
                  {#if formType === "map"}
                    <option value="warn">{$_('config.dashboardFormAlertMapWarn') || '注意 (Warn/Low) 以上の障害ノードがある場合'}</option>
                    <option value="high">{$_('config.dashboardFormAlertMapHigh') || '重度障害 (High) のノードがある場合のみ'}</option>
                  {:else}
                    <option value="warn">{$_('config.dashboardFormAlertPollingWarn') || '障害発生検知時 (Warning / Low / High)'}</option>
                    <option value="high">{$_('config.dashboardFormAlertPollingHigh') || '重度障害 (High) 検知時のみ'}</option>
                  {/if}
                </select>
              </div>

              <div class="flex items-center justify-between pt-1 border-t border-rose-200 dark:border-rose-900/40">
                <span class="text-[11px] text-slate-600 dark:text-slate-400 flex items-center gap-1">
                  <Volume2 class="w-3 h-3" />
                  {$_('config.dashboardFormAlertBeep') || '電子警告音 (Beep) を鳴らす'}
                </span>
                <input
                  type="checkbox"
                  bind:checked={formAlertSound}
                  class="rounded text-rose-600 focus:ring-rose-500 cursor-pointer"
                />
              </div>
            </div>
          {/if}
        </div>
      </div>

      <div class="flex items-center justify-end gap-2 pt-2 border-t border-slate-200 dark:border-slate-800">
        <button
          type="button"
          onclick={() => (showItemModal = false)}
          class="px-4 py-2 rounded-xl border border-slate-300 dark:border-slate-700 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 font-semibold cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={saveItemForm}
          class="px-4 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white font-bold shadow-md shadow-cyan-600/30 cursor-pointer"
        >
          {editingItemIndex !== null ? ($_('common.save')) : ($_('common.add'))}
        </button>
      </div>
    </div>
  </div>
{/if}
