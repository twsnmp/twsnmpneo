<script lang="ts">
  import { onMount } from "svelte";
  import { fetchMapConf, saveMapConf } from "../api";
  import { X, Save, Sliders, Bell, Brain, Image, Database, HardDrive } from "@lucide/svelte";

  let { show = $bindable(false) } = $props<{ show: boolean }>();

  let activeTab = $state<"map" | "notify" | "ai" | "datastore">("map");
  let mapName = $state("TWSNMP NEO");
  let iconSize = $state(3);
  let pollInt = $state(60);
  let timeout = $state(1);
  let retry = $state(1);

  // AI config
  let aiProvider = $state("gemini");
  let aiApiKey = $state("");
  let aiModel = $state("gemini-1.5-flash");

  onMount(async () => {
    try {
      const conf = await fetchMapConf();
      if (conf) {
        mapName = conf.map_name || "TWSNMP NEO";
        iconSize = conf.icon_size || 3;
        pollInt = conf.poll_int || 60;
      }
    } catch (e) {
      console.error(e);
    }
  });

  const handleSave = async () => {
    try {
      await saveMapConf({
        map_name: mapName,
        icon_size: iconSize,
        poll_int: pollInt,
        timeout,
        retry,
        ai_provider: aiProvider,
        ai_api_key: aiApiKey,
        ai_model: aiModel,
      });
      show = false;
    } catch (e) {
      console.error(e);
    }
  };
</script>

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
    <div class="flex h-[80vh] w-full max-w-3xl flex-col rounded-xl border border-border bg-card shadow-2xl overflow-hidden">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-border px-6 py-3 bg-muted/40">
        <div class="flex items-center gap-2 text-base font-bold text-foreground">
          <Sliders class="h-5 w-5 text-primary" />
          <span>システム環境設定 (Config)</span>
        </div>
        <button onclick={() => (show = false)} class="rounded p-1 text-muted-foreground hover:bg-muted">
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Main body -->
      <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar -->
        <div class="w-48 border-r border-border bg-muted/20 p-3 space-y-1">
          <button
            onclick={() => (activeTab = "map")}
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTab === 'map' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Sliders class="h-4 w-4" />
            マップ設定
          </button>
          <button
            onclick={() => (activeTab = "notify")}
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTab === 'notify' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Bell class="h-4 w-4" />
            通知設定
          </button>
          <button
            onclick={() => (activeTab = "ai")}
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTab === 'ai' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Brain class="h-4 w-4" />
            AI / LLM 設定
          </button>
          <button
            onclick={() => (activeTab = "datastore")}
            class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-xs font-medium transition-colors {activeTab === 'datastore' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'}"
          >
            <Database class="h-4 w-4" />
            データストア
          </button>
        </div>

        <!-- Content Area -->
        <div class="flex-1 overflow-y-auto p-6 text-xs">
          {#if activeTab === "map"}
            <div class="space-y-4 max-w-lg">
              <label class="block font-medium text-muted-foreground">
                マップ名称
                <input type="text" bind:value={mapName} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
              </label>
              <label class="block font-medium text-muted-foreground">
                デフォルトアイコンサイズ (1〜5)
                <input type="number" min={1} max={5} bind:value={iconSize} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
              </label>
              <label class="block font-medium text-muted-foreground">
                通常ポーリング間隔 (秒)
                <input type="number" min={5} bind:value={pollInt} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
              </label>
            </div>
          {:else if activeTab === "notify"}
            <div class="space-y-4 max-w-lg">
              <span class="text-muted-foreground">障害検知時の通知先設定 (Slack, LINE, Teams, Webhook, SMTP)</span>
              <label class="block font-medium text-muted-foreground">
                Webhook URL
                <input type="text" placeholder="https://hooks.slack.com/services/..." class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none" />
              </label>
            </div>
          {:else if activeTab === "ai"}
            <div class="space-y-4 max-w-lg">
              <label class="block font-medium text-muted-foreground">
                AI プロバイダー
                <select bind:value={aiProvider} class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none">
                  <option value="gemini">Google Gemini</option>
                  <option value="openai">OpenAI (GPT-4o)</option>
                  <option value="claude">Anthropic Claude</option>
                  <option value="ollama">Ollama (Local LLM)</option>
                </select>
              </label>
              <label class="block font-medium text-muted-foreground">
                API キー
                <input type="password" bind:value={aiApiKey} placeholder="AI サービスの API キーを入力" class="mt-1 w-full rounded-md border border-border bg-background px-3 py-1.5 focus:border-primary focus:outline-none font-mono" />
              </label>
            </div>
          {:else if activeTab === "datastore"}
            <div class="space-y-4 max-w-lg">
              <span class="text-muted-foreground">bbolt ドキュメントストアおよび Apache Parquet ログストレージ</span>
              <div class="rounded-lg border border-border p-3 space-y-2">
                <div class="flex justify-between">
                  <span class="font-medium text-foreground">bbolt 構成 DB:</span>
                  <span class="font-mono text-emerald-500">正常稼働 (test.db)</span>
                </div>
                <div class="flex justify-between">
                  <span class="font-medium text-foreground">Parquet ログストア:</span>
                  <span class="font-mono text-emerald-500">正常稼働 (./data/logs)</span>
                </div>
              </div>
            </div>
          {/if}
        </div>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-end gap-3 border-t border-border px-6 py-3 bg-muted/20">
        <button onclick={() => (show = false)} class="rounded-lg border border-border px-4 py-1.5 text-xs hover:bg-muted font-medium">
          キャンセル
        </button>
        <button onclick={handleSave} class="flex items-center gap-1.5 rounded-lg bg-primary px-4 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90">
          <Save class="h-4 w-4" />
          保存
        </button>
      </div>
    </div>
  </div>
{/if}
