<script lang="ts">
  import {
    testNotifyMail,
    testNotifyWebhook,
    startNotifyOAuth2,
    deleteNotifyOAuth2Token,
    fetchNotifyOAuth2Status,
    saveNotifyConf,
    type NotifyConfEnt,
  } from "../../api";
  import {
    Bell,
    Mail,
    Network,
    Database,
    Send,
    Key,
    CheckCircle2,
    X,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    notifyProvider = $bindable("smtp"),
    notifyLevel = $bindable("warn"),
    notifyInterval = $bindable(60),
    notifySubject = $bindable("TWSNMP"),
    mailServer = $bindable(""),
    mailFrom = $bindable(""),
    mailTo = $bindable(""),
    mailUser = $bindable(""),
    mailPassword = $bindable(""),
    insecureSkipVerify = $bindable(false),
    notifyReport = $bindable(false),
    notifyLLMSummary = $bindable(false),
    notifyNotifyRepair = $bindable(false),
    notifyCheckDependency = $bindable(false),
    notifyExecCmd = $bindable(""),
    notifyWebHookNotify = $bindable(""),
    notifyWebHookReport = $bindable(""),
    notifyClientID = $bindable(""),
    notifyClientSecret = $bindable(""),
    notifyMSTenant = $bindable(""),
    notifyOAuth2HasToken = $bindable(false),
  }: {
    notifyProvider: string;
    notifyLevel: string;
    notifyInterval: number;
    notifySubject: string;
    mailServer: string;
    mailFrom: string;
    mailTo: string;
    mailUser: string;
    mailPassword: string;
    insecureSkipVerify: boolean;
    notifyReport: boolean;
    notifyLLMSummary: boolean;
    notifyNotifyRepair: boolean;
    notifyCheckDependency: boolean;
    notifyExecCmd: string;
    notifyWebHookNotify: string;
    notifyWebHookReport: string;
    notifyClientID: string;
    notifyClientSecret: string;
    notifyMSTenant: string;
    notifyOAuth2HasToken: boolean;
  } = $props();

  let notifyTestingMail = $state(false);
  let notifyTestingWebhook = $state(false);
  let notifyTestMsg = $state("");
  let notifyTestError = $state("");

  const getCurrentNotifyConf = (): NotifyConfEnt => ({
    Provider: notifyProvider,
    Level: notifyLevel,
    Interval: Number(notifyInterval),
    Subject: notifySubject,
    MailServer: mailServer,
    MailFrom: mailFrom,
    MailTo: mailTo,
    User: mailUser,
    Password: mailPassword,
    InsecureSkipVerify: Boolean(insecureSkipVerify),
    Report: Boolean(notifyReport),
    LLMSummary: Boolean(notifyLLMSummary),
    NotifyRepair: Boolean(notifyNotifyRepair),
    CheckDependency: Boolean(notifyCheckDependency),
    ExecCmd: notifyExecCmd,
    WebHookNotify: notifyWebHookNotify,
    WebHookReport: notifyWebHookReport,
    ClientID: notifyClientID,
    ClientSecret: notifyClientSecret,
    MSTenant: notifyMSTenant,
  });

  const handleTestMail = async () => {
    notifyTestingMail = true;
    notifyTestMsg = "";
    notifyTestError = "";
    try {
      await testNotifyMail(getCurrentNotifyConf());
      notifyTestMsg = $_('config.notifyTestSuccess');
    } catch (e: any) {
      notifyTestError = `${$_('config.notifyTestFailed')}: ${e.message || e}`;
    } finally {
      notifyTestingMail = false;
    }
  };

  const handleTestWebhook = async () => {
    notifyTestingWebhook = true;
    notifyTestMsg = "";
    notifyTestError = "";
    try {
      await testNotifyWebhook(getCurrentNotifyConf());
      notifyTestMsg = $_('config.notifyTestSuccess');
    } catch (e: any) {
      notifyTestError = `${$_('config.notifyTestFailed')}: ${e.message || e}`;
    } finally {
      notifyTestingWebhook = false;
    }
  };

  const handleStartOAuth2 = async () => {
    notifyTestMsg = "";
    notifyTestError = "";
    const popup = window.open("about:blank", "_blank");
    if (!popup) {
      notifyTestError = $_('config.notifyOAuth2PopupBlocked');
      return;
    }
    try {
      await saveNotifyConf(getCurrentNotifyConf());
      const res = await startNotifyOAuth2();
      if (res?.url) {
        const onOAuth2Message = (event: MessageEvent) => {
          if (
            event.origin !== window.location.origin ||
            event.source !== popup ||
            event.data?.type !== "twsnmpneo-notify-oauth2-complete"
          ) {
            return;
          }
          window.removeEventListener("message", onOAuth2Message);
          window.clearInterval(closedCheck);
          popup.close();
          fetchNotifyOAuth2Status()
            .then((status) => {
              notifyOAuth2HasToken = status.hasToken;
              notifyTestMsg = $_('config.notifyOAuth2Success');
            })
            .catch((err) => {
              notifyTestError = $_('config.notifyOAuth2StatusError', { values: { error: String(err.message || err) } });
            });
        };
        window.addEventListener("message", onOAuth2Message);
        const closedCheck = window.setInterval(() => {
          if (popup.closed) {
            window.removeEventListener("message", onOAuth2Message);
            window.clearInterval(closedCheck);
          }
        }, 1000);
        popup.location.href = res.url;
      } else {
        throw new Error("OAuth2 authorization URL was not returned");
      }
    } catch (e: any) {
      popup.close();
      notifyTestError = $_('config.notifyOAuth2StartError', { values: { error: String(e.message || e) } });
    }
  };

  const handleDeleteOAuth2Token = async () => {
    try {
      await deleteNotifyOAuth2Token();
      notifyOAuth2HasToken = false;
      notifyTestMsg = $_('config.notifyOAuth2TokenDeleted');
    } catch (e: any) {
      notifyTestError = $_('config.notifyOAuth2TokenDeleteError', { values: { error: String(e.message || e) } });
    }
  };
</script>

<div class="space-y-6 max-w-2xl">
  <!-- Message & Error Alerts for Notify Actions -->
  {#if notifyTestMsg}
    <div class="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-3 text-xs text-emerald-600 dark:text-emerald-400 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <CheckCircle2 class="w-4 h-4 shrink-0" />
        <span>{notifyTestMsg}</span>
      </div>
      <button type="button" onclick={() => (notifyTestMsg = "")} class="text-emerald-500 hover:text-emerald-600">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}
  {#if notifyTestError}
    <div class="rounded-xl border border-rose-500/20 bg-rose-500/10 p-3 text-xs text-rose-600 dark:text-rose-400 flex items-center justify-between">
      <span>{notifyTestError}</span>
      <button type="button" onclick={() => (notifyTestError = "")} class="text-rose-500 hover:text-rose-600">
        <X class="w-3.5 h-3.5" />
      </button>
    </div>
  {/if}

  <!-- 1. Alert Notification Policy -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Bell class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.notifyPolicyTitle')}
    </h3>
    <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
      <div>
        <label for="notify-level" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyLevel')}</label>
        <select
          id="notify-level"
          bind:value={notifyLevel}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        >
          <option value="none">{$_('config.notifyLevelNone')}</option>
          <option value="warn">{$_('config.notifyLevelWarn')}</option>
          <option value="low">{$_('config.notifyLevelLow')}</option>
          <option value="high">{$_('config.notifyLevelHigh')}</option>
        </select>
      </div>
      <div>
        <label for="notify-interval" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyIntervalLabel')}</label>
        <input
          id="notify-interval"
          type="number"
          min={1}
          bind:value={notifyInterval}
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="notify-subject" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifySubject')}</label>
        <input
          id="notify-subject"
          type="text"
          bind:value={notifySubject}
          placeholder="TWSNMP"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-3 pt-1">
      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={notifyNotifyRepair} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.notifyRepair')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.notifyRepairDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={notifyCheckDependency} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.notifyCheckDependency')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.notifyCheckDependencyDesc')}</span>
        </div>
      </label>
    </div>

    <div>
      <label for="notify-exec-cmd" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyExecCmd')}</label>
      <input
        id="notify-exec-cmd"
        type="text"
        bind:value={notifyExecCmd}
        placeholder="/path/to/script.sh $level"
        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
      />
      <p class="text-[11px] text-slate-400 mt-1">{$_('config.notifyExecCmdDesc')}</p>
    </div>
  </div>

  <!-- 2. Email Delivery & OAuth2 -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Mail class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        {$_('config.smtpTitle')}
      </h3>
      <button
        type="button"
        disabled={notifyTestingMail}
        onclick={handleTestMail}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors disabled:opacity-50 cursor-pointer"
      >
        <Send class="w-3 h-3" />
        <span>{notifyTestingMail ? $_('config.notifyTesting') : $_('config.notifyTestMail')}</span>
      </button>
    </div>

    <!-- Provider Selector -->
    <div>
      <label for="notify-provider" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyProvider')}</label>
      <select
        id="notify-provider"
        bind:value={notifyProvider}
        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
      >
        <option value="smtp">{$_('config.notifyProviderSmtp')}</option>
        <option value="google">{$_('config.notifyProviderGoogle')}</option>
        <option value="microsoft">{$_('config.notifyProviderMS')}</option>
        <option value="mscustom">{$_('config.notifyProviderMSCustom')}</option>
      </select>
    </div>

    <!-- OAuth2 Credentials (Google / Microsoft) -->
    {#if notifyProvider !== "smtp"}
      <div class="p-4 rounded-xl border border-cyan-500/20 bg-cyan-500/5 space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Key class="w-4 h-4 text-cyan-500" />
            <span class="text-xs font-bold text-slate-800 dark:text-slate-200">{$_('config.notifyOAuth2Auth')}</span>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-[11px] px-2.5 py-0.5 rounded-full font-medium {notifyOAuth2HasToken ? 'bg-emerald-500/20 text-emerald-500' : 'bg-amber-500/20 text-amber-500'}">
              {notifyOAuth2HasToken ? $_('config.notifyOAuth2Authorized') : $_('config.notifyOAuth2NotAuthorized')}
            </span>
            {#if notifyOAuth2HasToken}
              <button
                type="button"
                onclick={handleDeleteOAuth2Token}
                class="text-[10px] text-rose-500 hover:text-rose-600 underline cursor-pointer"
              >
                {$_('config.notifyOAuth2DeleteToken')}
              </button>
            {/if}
          </div>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div>
            <label for="notify-client-id" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('config.notifyClientID')}</label>
            <input
              id="notify-client-id"
              type="text"
              bind:value={notifyClientID}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3 py-1.5 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
            />
          </div>
          <div>
            <label for="notify-client-secret" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('config.notifyClientSecret')}</label>
            <input
              id="notify-client-secret"
              type="password"
              bind:value={notifyClientSecret}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3 py-1.5 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
            />
          </div>
        </div>

        {#if notifyProvider === "microsoft" || notifyProvider === "mscustom"}
          <div>
            <label for="notify-tenant" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('config.notifyMSTenant')}</label>
            <input
              id="notify-tenant"
              type="text"
              bind:value={notifyMSTenant}
              placeholder={$_('config.notifyMSTenantPlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3 py-1.5 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
            />
          </div>
        {/if}

        <div class="pt-1">
          <button
            type="button"
            onclick={handleStartOAuth2}
            class="px-3.5 py-1.5 rounded-xl border border-cyan-500/40 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors cursor-pointer"
          >
            {$_('config.notifyOAuth2AuthButton')}
          </button>
        </div>
      </div>
    {/if}

    <!-- SMTP Server details (SMTP or MSCustom) -->
    {#if notifyProvider === "smtp" || notifyProvider === "mscustom"}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div class="md:col-span-2">
          <label for="mail-server" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mailServerLabel')}</label>
          <input
            id="mail-server"
            type="text"
            bind:value={mailServer}
            placeholder="smtp.example.com:587"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
          />
          <p class="mt-1 text-[11px] text-slate-400">{$_('config.notifyPublicTestDestinations')}</p>
        </div>
        <div>
          <label for="mail-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mailUserLabel')}</label>
          <input
            id="mail-user"
            type="text"
            bind:value={mailUser}
            placeholder="user@example.com"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
          />
        </div>
        <div>
          <label for="mail-pwd" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mailPasswordLabel')}</label>
          <input
            id="mail-pwd"
            type="password"
            bind:value={mailPassword}
            placeholder="••••••••"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
          />
        </div>
      </div>

      <label class="flex items-center gap-3 cursor-pointer">
        <input type="checkbox" bind:checked={insecureSkipVerify} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <span class="text-xs text-slate-600 dark:text-slate-300">{$_('config.skipTlsVerify')}</span>
      </label>
    {/if}

    <!-- Mail From / To -->
    <div class="grid grid-cols-1 md:grid-cols-2 gap-3 pt-1">
      <div>
        <label for="mail-from" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mailFromLabel')}</label>
        <input
          id="mail-from"
          type="email"
          bind:value={mailFrom}
          placeholder="twsnmp@example.com"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="mail-to" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mailToLabel')}</label>
        <input
          id="mail-to"
          type="text"
          bind:value={mailTo}
          placeholder="admin@example.com, alerts@example.com"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>
  </div>

  <!-- 3. Daily Report Settings -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
      <Database class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
      {$_('config.notifyReport')}
    </h3>
    <div class="space-y-3">
      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
        <input type="checkbox" bind:checked={notifyReport} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.notifyReport')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.notifyReportDesc')}</span>
        </div>
      </label>

      <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors {notifyReport ? '' : 'opacity-50 pointer-events-none'}">
        <input type="checkbox" bind:checked={notifyLLMSummary} disabled={!notifyReport} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
        <div>
          <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.notifyLLMSummary')}</span>
          <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.notifyLLMSummaryDesc')}</span>
        </div>
      </label>
    </div>
  </div>

  <!-- 4. Webhook Notifications -->
  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
    <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
      <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
        Webhook
      </h3>
      <button
        type="button"
        disabled={notifyTestingWebhook}
        onclick={handleTestWebhook}
        class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors disabled:opacity-50 cursor-pointer"
      >
        <Send class="w-3 h-3" />
        <span>{notifyTestingWebhook ? $_('config.notifyTesting') : $_('config.notifyTestWebhook')}</span>
      </button>
    </div>

    <p class="text-[11px] text-slate-400">{$_('config.notifyPublicTestDestinations')}</p>
    <div class="space-y-3">
      <div>
        <label for="notify-webhook" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyWebhookNotify')}</label>
        <input
          id="notify-webhook"
          type="text"
          bind:value={notifyWebHookNotify}
          placeholder="https://example.com/api/notify-hook"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
      <div>
        <label for="notify-webhook-report" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.notifyWebhookReport')}</label>
        <input
          id="notify-webhook-report"
          type="text"
          bind:value={notifyWebHookReport}
          placeholder="https://example.com/api/report-hook"
          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
        />
      </div>
    </div>
  </div>
</div>
