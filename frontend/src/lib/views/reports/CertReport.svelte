<script lang="ts">
  import { _ } from "svelte-i18n";
  import { ShieldCheck, Plus, RefreshCw, Trash2, X } from "@lucide/svelte";
  import {
    saveCertMonitor,
    checkCertMonitors,
    deleteCertMonitor,
    type CertMonitorEnt,
  } from "../../api";

  let {
    certMonitors = [],
    searchQuery = "",
    onReload,
  }: {
    certMonitors?: CertMonitorEnt[];
    searchQuery?: string;
    onReload?: () => Promise<void> | void;
  } = $props();

  let showAddCertModal = $state(false);
  let newCertTarget = $state("");
  let newCertPort = $state(443);
  let isCheckingCerts = $state(false);
  let isSavingCert = $state(false);
  let certErrorMessage = $state("");

  const rawCertItems = $derived.by(() => {
    if (certMonitors.length > 0) {
      return certMonitors.map((c) => {
        const nowSec = Math.floor(Date.now() / 1000);
        const days = c.notAfter > 0 ? Math.ceil((c.notAfter - nowSec) / 86400) : 0;
        let status: "valid" | "warning" | "error" = "valid";
        if (c.state === "error" || (c.error && c.error.length > 0) || days <= 0) {
          status = "error";
        } else if (days <= 30 || c.state === "warn") {
          status = "warning";
        }
        const validUntil =
          c.notAfter > 0 ? new Date(c.notAfter * 1000).toISOString().split("T")[0] : "-";
        return {
          id: c.id,
          host: c.target,
          port: c.port,
          issuer: c.issuer || "-",
          subject: c.subject || "-",
          key: (c as any).key_type || (c as any).keyType || "TLS",
          validUntil,
          days,
          status,
          error: c.error,
        };
      });
    }
    return [
      {
        id: "demo-1",
        host: "TWSNMP NEO Internal API",
        port: 8080,
        issuer: "TWSNMP NEO Root CA",
        subject: "CN=localhost",
        key: "RSA 2048-bit",
        validUntil: "2036-09-20",
        days: 3649,
        status: "valid" as const,
        error: "",
      },
      {
        id: "demo-2",
        host: "Core Switch Management",
        port: 443,
        issuer: "Let's Encrypt Authority X3",
        subject: "CN=sw01.internal.lan",
        key: "ECDSA P-256",
        validUntil: "2026-12-15",
        days: 85,
        status: "valid" as const,
        error: "",
      },
      {
        id: "demo-3",
        host: "Edge Gateway Router",
        port: 8443,
        issuer: "Self-Signed Certificate",
        subject: "CN=gateway.corp",
        key: "RSA 4096-bit",
        validUntil: "2026-10-05",
        days: 14,
        status: "warning" as const,
        error: "",
      },
    ];
  });

  const certItems = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return rawCertItems;
    return rawCertItems.filter(
      (c) =>
        c.host.toLowerCase().includes(q) ||
        String(c.port).includes(q) ||
        c.issuer.toLowerCase().includes(q) ||
        c.subject.toLowerCase().includes(q) ||
        c.key.toLowerCase().includes(q) ||
        c.status.toLowerCase().includes(q)
    );
  });

  const handleAddCert = async () => {
    if (!newCertTarget.trim()) return;
    isSavingCert = true;
    certErrorMessage = "";
    try {
      await saveCertMonitor({
        target: newCertTarget.trim(),
        port: Number(newCertPort) || 443,
      });
      showAddCertModal = false;
      newCertTarget = "";
      newCertPort = 443;
      await checkCertMonitors();
      await onReload?.();
    } catch (e: any) {
      certErrorMessage = e.message || "Failed to add certificate monitor target";
    } finally {
      isSavingCert = false;
    }
  };

  const handleCheckAllCerts = async () => {
    isCheckingCerts = true;
    try {
      await checkCertMonitors();
      await onReload?.();
    } catch (e: any) {
      alert("Certificate check failed: " + e.message);
    } finally {
      isCheckingCerts = false;
    }
  };

  const handleDeleteCert = async (id: string, target: string, port: number) => {
    if (!confirm($_("report.confirmDeleteCert", { values: { target, port } }))) {
      return;
    }
    try {
      await deleteCertMonitor(id);
      await onReload?.();
    } catch (e: any) {
      alert("Failed to delete certificate target: " + e.message);
    }
  };

  export function exportCSV(): void {
    const csv =
      "Target,Port,Issuer,Subject,Key,ValidUntil,RemainingDays,Status\n" +
      certItems
        .map(
          (c) =>
            `"${c.host}",${c.port},"${c.issuer}","${c.subject}","${c.key}","${c.validUntil}",${c.days},"${c.status}"`
        )
        .join("\n");
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `twsnmp_report_cert_${Date.now()}.csv`;
    link.click();
  }
</script>

<div class="space-y-6">
  <div class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <ShieldCheck class="w-5 h-5 text-cyan-400" />
        {$_("report.certTitle")}
      </h2>
      <p class="text-xs text-slate-400 mt-1">{$_("report.certSubtitle")}</p>
    </div>
    <div class="flex items-center gap-2">
      <button
        type="button"
        onclick={() => (showAddCertModal = true)}
        class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-3.5 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
      >
        <Plus class="w-4 h-4" />
        <span>{$_("report.btnAddCert")}</span>
      </button>
      <button
        type="button"
        onclick={handleCheckAllCerts}
        disabled={isCheckingCerts}
        class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-100 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer"
      >
        <RefreshCw class="w-3.5 h-3.5 text-cyan-400 {isCheckingCerts ? 'animate-spin' : ''}" />
        <span>{isCheckingCerts ? $_("report.checkingCerts") : $_("report.btnCheckCerts")}</span>
      </button>
    </div>
  </div>

  <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-400">{$_("report.monitoredCerts")}</span>
      <div class="text-2xl font-bold font-mono text-cyan-400">{certItems.length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
      <div class="text-[10px] text-slate-400">{$_("report.monitoredCertsSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-400">{$_("report.validCerts")}</span>
      <div class="text-2xl font-bold font-mono text-emerald-400">{certItems.filter((c) => c.status === 'valid').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
      <div class="text-[10px] text-slate-400">{$_("report.validCertsSub")}</div>
    </div>
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg space-y-2">
      <span class="text-xs font-semibold text-slate-400">{$_("report.expiringCerts")}</span>
      <div class="text-2xl font-bold font-mono text-amber-400">{certItems.filter((c) => c.status === 'warning' || c.status === 'error').length} <span class="text-xs font-normal text-slate-400">{$_("report.unitCerts")}</span></div>
      <div class="text-[10px] text-amber-400/80">{$_("report.expiringCertsSub")}</div>
    </div>
  </div>

  <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg overflow-hidden">
    <table class="w-full text-left text-xs border-collapse font-mono">
      <thead class="sticky top-0 bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 uppercase text-[10px] font-semibold tracking-wider border-b border-slate-800">
        <tr>
          <th class="py-1 px-2.5">{$_("report.colMonitoredService")}</th>
          <th class="py-1 px-2.5">{$_("report.colIssuer")}</th>
          <th class="py-1 px-2.5">{$_("report.colSubject")}</th>
          <th class="py-1 px-2.5">{$_("report.colKeyStrength")}</th>
          <th class="py-1 px-2.5">{$_("report.colValidUntil")}</th>
          <th class="py-1 px-2.5">{$_("report.colRemainingDays")}</th>
          <th class="py-1 px-2.5">{$_("report.colStatus")}</th>
          <th class="py-1 px-2 text-center w-12">{$_("report.colAction")}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-200/80 dark:divide-slate-800/40 text-slate-700 dark:text-slate-300">
        {#if certItems.length === 0}
          <tr>
            <td colspan="8" class="p-8 text-center text-slate-500 font-sans">
              監視対象の証明書が登録されていません。「監視対象追加」から TLS サーバーを登録してください。
            </td>
          </tr>
        {:else}
          {#each certItems as c}
            <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
              <td class="py-1 px-2.5 font-bold font-sans text-slate-900 dark:text-slate-100 text-[11px]">{c.host}:{c.port}</td>
              <td class="py-1 px-2.5 text-slate-700 dark:text-slate-400 font-sans text-[11px] truncate max-w-xs">{c.issuer}</td>
              <td class="py-1 px-2.5 text-cyan-600 dark:text-cyan-400 text-[11px] truncate max-w-xs">{c.subject}</td>
              <td class="py-1 px-2.5 text-slate-800 dark:text-slate-200 text-[11px]">{c.key}</td>
              <td class="py-1 px-2.5 text-slate-700 dark:text-slate-300 text-[11px]">{c.validUntil}</td>
              <td class="py-1 px-2.5 font-bold text-[11px] {c.days <= 0 ? 'text-rose-500' : c.days < 30 ? 'text-amber-500' : 'text-emerald-500'}">
                {c.days <= 0 ? '期限切れ' : $_("report.remainingDaysUnit", { values: { days: c.days } })}
              </td>
              <td class="py-1 px-2.5">
                <span class="rounded px-1.5 py-0.5 text-[9px] font-bold uppercase border leading-none {c.status === 'valid' ? 'bg-emerald-100 dark:bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-300 dark:border-emerald-500/30' : c.status === 'warning' ? 'bg-amber-100 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-300 dark:border-amber-500/30' : 'bg-rose-100 dark:bg-rose-500/10 text-rose-700 dark:text-rose-400 border-rose-300 dark:border-rose-500/30'}">
                  {c.status}
                </span>
              </td>
              <td class="py-1 px-2 text-center">
                <button
                  type="button"
                  onclick={() => handleDeleteCert(c.id, c.host, c.port)}
                  class="inline-flex items-center justify-center rounded border border-rose-500/30 bg-rose-500/10 p-1 text-rose-400 hover:bg-rose-500/20 hover:text-rose-200 transition-all cursor-pointer"
                  title="削除"
                >
                  <Trash2 class="h-3 w-3" />
                </button>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <!-- Add Certificate Modal -->
  {#if showAddCertModal}
    <div class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/60 backdrop-blur-xs p-4">
      <div class="w-full max-w-md rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 p-6 shadow-2xl space-y-4">
        <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
          <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
            <ShieldCheck class="w-4 h-4 text-cyan-400" />
            {$_("report.modalAddCertTitle")}
          </h3>
          <button
            type="button"
            onclick={() => { showAddCertModal = false; certErrorMessage = ""; }}
            class="rounded-lg p-1 text-slate-400 hover:text-slate-100 hover:bg-slate-800 transition-colors"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        {#if certErrorMessage}
          <div class="rounded-xl border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-400 font-sans">
            {certErrorMessage}
          </div>
        {/if}

        <div class="space-y-3 font-sans text-xs">
          <div>
            <label class="block text-slate-400 font-semibold mb-1" for="newCertTargetInput">
              {$_("report.targetHostLabel")}
            </label>
            <input
              id="newCertTargetInput"
              type="text"
              placeholder="example.com または 192.168.1.1"
              bind:value={newCertTarget}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <div>
            <label class="block text-slate-400 font-semibold mb-1" for="newCertPortInput">
              {$_("report.targetPortLabel")}
            </label>
            <input
              id="newCertPortInput"
              type="number"
              bind:value={newCertPort}
              min="1"
              max="65535"
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 px-3 py-2 text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>
        </div>

        <div class="flex items-center justify-end gap-2 pt-3 border-t border-slate-200 dark:border-slate-800">
          <button
            type="button"
            onclick={() => { showAddCertModal = false; certErrorMessage = ""; }}
            class="rounded-xl px-4 py-2 text-xs font-semibold text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            {$_("report.btnCancel")}
          </button>
          <button
            type="button"
            onclick={handleAddCert}
            disabled={isSavingCert || !newCertTarget.trim()}
            class="rounded-xl bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 px-4 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
          >
            {isSavingCert ? "保存・検査中..." : $_("report.btnSave")}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
