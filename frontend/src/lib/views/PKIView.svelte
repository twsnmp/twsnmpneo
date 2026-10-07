<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import forge from "node-forge";
  import {
    createPKICertificate,
    downloadPKICertificate,
    fetchPKICertificates,
    fetchPKISettings,
    fetchPKIStatus,
    initializePKICA,
    issuePKICertificateFromCSR,
    resetPKICA,
    revokePKICertificate,
    updatePKISettings,
    type PKICertificate,
    type PKISettings,
    type PKIStatus,
  } from "../api";
  import { showConfirm } from "../stores/modalStore";

  let status = $state<PKIStatus | null>(null);
  let settings = $state<PKISettings | null>(null);
  let certificates = $state<PKICertificate[]>([]);
  let commonName = $state("");
  let dnsNames = $state("");
  let ipAddresses = $state("");
  let validDays = $state(365);
  let sans = $state("");
  let csrCommonName = $state("");
  let csrKeyType = $state("rsa-4096");
  let csrOrganization = $state("");
  let csrOrganizationalUnit = $state("");
  let csrCountry = $state("JP");
  let csrProvince = $state("");
  let csrLocality = $state("");
  let csrSANs = $state("");
  let csrPEM = $state("");
  let generatedCSR = $state("");
  let generatedKey = $state("");
  let generatedCSRBase = $state("");
  let activeSection = $state("setup");
  let issueMode = $state<"direct" | "csr">("direct");
  let loading = $state(false);
  let submitting = $state(false);
  let error = $state("");

  const refresh = async () => {
    loading = true;
    error = "";
    const wasReady = status?.ready ?? false;
    try {
      const [nextStatus, nextSettings, nextCertificates] = await Promise.all([
        fetchPKIStatus(),
        fetchPKISettings(),
        fetchPKICertificates(),
      ]);
      status = nextStatus;
      settings = nextSettings;
      sans = nextSettings.sans.join(", ");
      certificates = nextCertificates;
      if (nextStatus.ready && !wasReady) {
        activeSection = "main";
      } else if (!nextStatus.ready && wasReady) {
        activeSection = "setup";
      }
      if (!nextStatus.ready) validDays = Math.ceil(nextSettings.certValidityHours / 24);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      loading = false;
    }
  };

  const splitValues = (value: string) =>
    value.split(",").map((entry) => entry.trim()).filter(Boolean);

  const saveText = (filename: string, content: string) => {
    const url = URL.createObjectURL(new Blob([content], { type: "application/x-pem-file" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const initializeCA = async () => {
    submitting = true;
    error = "";
    try {
      if (!settings) throw new Error("PKI settings are not loaded");
      await initializePKICA({ ...settings, sans: splitValues(sans) });
      await refresh();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      submitting = false;
    }
  };

  const saveSettings = async () => {
    if (!settings) return;
    submitting = true;
    error = "";
    try {
      settings = await updatePKISettings({ ...settings, sans: splitValues(sans) });
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
      await refresh();
    } finally {
      submitting = false;
    }
  };

  const resetCA = async () => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'CAリセットの確認',
      message: $_("pki.confirmResetCA"),
      type: 'danger',
      confirmText: $_('common.delete') || 'リセット',
    });
    if (!ok) return;
    submitting = true;
    error = "";
    try {
      await resetPKICA();
      await refresh();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      submitting = false;
    }
  };

  const issueCertificate = async () => {
    submitting = true;
    error = "";
    try {
      await createPKICertificate({
        commonName: commonName.trim(),
        dnsNames: splitValues(dnsNames),
        ipAddresses: splitValues(ipAddresses),
        validDays,
      });
      commonName = "";
      dnsNames = "";
      ipAddresses = "";
      await refresh();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      submitting = false;
    }
  };

  const createCSR = async () => {
    submitting = true;
    error = "";
    try {
      const bits = Number(csrKeyType.split("-")[1]);
      const keys = await new Promise<forge.pki.rsa.KeyPair>((resolve, reject) => {
        forge.pki.rsa.generateKeyPair({ bits }, (cause, keyPair) => {
          if (cause) {
            reject(cause);
          } else if (keyPair) {
            resolve(keyPair);
          } else {
            reject(new Error("RSA key generation returned no key pair"));
          }
        });
      });
      const csr = forge.pki.createCertificationRequest();
      csr.publicKey = keys.publicKey;
      csr.setSubject([
        { name: "commonName", value: csrCommonName.trim() },
        ...(csrOrganization.trim() ? [{ name: "organizationName", value: csrOrganization.trim() }] : []),
        ...(csrOrganizationalUnit.trim() ? [{ name: "organizationalUnitName", value: csrOrganizationalUnit.trim() }] : []),
        ...(csrCountry.trim() ? [{ name: "countryName", value: csrCountry.trim() }] : []),
        ...(csrProvince.trim() ? [{ name: "stateOrProvinceName", value: csrProvince.trim() }] : []),
        ...(csrLocality.trim() ? [{ name: "localityName", value: csrLocality.trim() }] : []),
      ]);
      const altNames = splitValues(csrSANs).map((value) => {
        if (value.includes("@")) return { type: 1, value };
        if (value.includes(":")) {
          try {
            new URL(`http://[${value}]/`);
            return { type: 7, ip: value };
          } catch {
            throw new Error(`Invalid IP address in SAN: ${value}`);
          }
        }
        if (/^\d+(?:\.\d+){3}$/.test(value)) {
          if (value.split(".").some((part) => Number(part) > 255)) {
            throw new Error(`Invalid IP address in SAN: ${value}`);
          }
          return { type: 7, ip: value };
        }
        return { type: 2, value };
      });
      if (altNames.length > 100) throw new Error("At most 100 subject alternative names are allowed");
      if (altNames.length > 0) {
        csr.setAttributes([{
          name: "extensionRequest",
          extensions: [{ name: "subjectAltName", altNames }],
        }]);
      }
      csr.sign(keys.privateKey, forge.md.sha256.create());
      generatedCSRBase = csrCommonName.trim().replace(/[^a-zA-Z0-9._-]/g, "_");
      generatedCSR = forge.pki.certificationRequestToPem(csr);
      generatedKey = forge.pki.privateKeyToPem(keys.privateKey);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      submitting = false;
    }
  };

  const loadCSRFile = async (event: Event) => {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    try {
      csrPEM = await file.text();
      error = "";
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    }
  };

  const issueCSR = async () => {
    submitting = true;
    error = "";
    try {
      await issuePKICertificateFromCSR(csrPEM);
      csrPEM = "";
      await refresh();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      submitting = false;
    }
  };

  const revoke = async (cert: PKICertificate) => {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || '証明書失効の確認',
      message: $_("pki.confirmRevoke"),
      type: 'danger',
      confirmText: $_('common.delete') || '失効',
    });
    if (!ok) return;
    error = "";
    try {
      await revokePKICertificate(cert.serial);
      await refresh();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    }
  };

  const download = async (serial: string) => {
    error = "";
    try {
      await downloadPKICertificate(serial);
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    }
  };

  const renderDate = (seconds: number) =>
    new Date(seconds * 1000).toLocaleString();

  onMount(refresh);
</script>

<div class="flex h-full flex-col overflow-hidden">
  <header class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white/80 px-4 py-3 dark:border-slate-800 dark:bg-slate-950/80 sm:px-6">
    <div>
      <h2 class="text-xl font-semibold text-slate-900 dark:text-white">{$_("pki.title")}</h2>
      <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{$_("pki.description")}</p>
    </div>
    <button
      type="button"
      onclick={refresh}
      disabled={loading}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm hover:bg-slate-100 disabled:opacity-50 dark:border-slate-700 dark:hover:bg-slate-800"
    >{loading ? $_("common.loading") : $_("common.refresh")}</button>
  </header>

  {#if error}
    <div role="alert" class="mx-4 mt-3 shrink-0 rounded-lg border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-800 dark:border-red-900 dark:bg-red-950/40 dark:text-red-200 sm:mx-6">
      {error}
    </div>
  {/if}

  <div class="flex min-h-0 flex-1 flex-col md:flex-row">
    <aside class="flex shrink-0 flex-row gap-1 overflow-x-auto border-b border-slate-200 bg-white/80 p-2 dark:border-slate-800 dark:bg-slate-950/80 md:w-60 md:flex-col md:gap-1.5 md:border-b-0 md:border-r md:p-3">
      {#if status?.ready}
        {#each [
          { id: "main", label: $_("pki.certManagement") },
          { id: "servers", label: $_("pki.serverControl") },
          { id: "csr", label: $_("pki.csrCreate") },
        ] as item}
          <button
            type="button"
            aria-current={activeSection === item.id ? "page" : undefined}
            onclick={() => (activeSection = item.id)}
            class="shrink-0 rounded-xl px-3.5 py-2.5 text-left text-xs font-semibold transition-all md:w-full {activeSection === item.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-200'}"
          >{item.label}</button>
        {/each}
      {:else}
        {#each [
          { id: "setup", label: $_("pki.createCA") },
          { id: "csr", label: $_("pki.csrCreate") },
        ] as item}
          <button
            type="button"
            aria-current={activeSection === item.id ? "page" : undefined}
            onclick={() => (activeSection = item.id)}
            class="shrink-0 rounded-xl px-3.5 py-2.5 text-left text-xs font-semibold transition-all md:w-full {activeSection === item.id ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-900 dark:hover:text-slate-200'}"
          >{item.label}</button>
        {/each}
      {/if}
    </aside>

    <main class="min-h-0 flex-1 overflow-y-auto p-4 sm:p-6">
      <div class="mx-auto flex max-w-6xl flex-col gap-5">

    {#if status?.ready}
      {#if activeSection === "main"}
      <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h3 class="font-semibold text-slate-900 dark:text-white">{$_("pki.rootCA")}</h3>
            <p class="mt-1 text-sm text-slate-600 dark:text-slate-300">{status.commonName}</p>
            <p class="mt-1 text-xs text-slate-500">{$_("pki.expires")}: {status.expiresAt ? renderDate(status.expiresAt) : "-"}</p>
          </div>
          <div class="flex gap-2">
            <a
              href="/api/pki/ca.pem"
              download="root-ca.pem"
              class="rounded-lg bg-cyan-700 px-3 py-2 text-sm font-medium text-white hover:bg-cyan-600"
            >{$_("pki.downloadCA")}</a>
            <button
              type="button"
              onclick={resetCA}
              disabled={submitting}
              class="rounded-lg border border-red-300 px-3 py-2 text-sm text-red-700 hover:bg-red-50 disabled:opacity-50 dark:border-red-900 dark:text-red-300 dark:hover:bg-red-950/40"
            >{$_("pki.resetCA")}</button>
          </div>
        </div>
      </section>

      <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h3 class="mb-4 font-semibold text-slate-900 dark:text-white">{$_("pki.issueTitle")}</h3>
        <div class="mb-4 flex flex-wrap gap-2 border-b border-slate-200 pb-3 dark:border-slate-800">
          <button
            type="button"
            aria-pressed={issueMode === "direct"}
            onclick={() => (issueMode = "direct")}
            class="rounded-lg px-3 py-2 text-sm font-medium {issueMode === 'direct' ? 'bg-cyan-700 text-white' : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
          >{$_("pki.issueDirect")}</button>
          <button
            type="button"
            aria-pressed={issueMode === "csr"}
            onclick={() => (issueMode = "csr")}
            class="rounded-lg px-3 py-2 text-sm font-medium {issueMode === 'csr' ? 'bg-cyan-700 text-white' : 'text-slate-600 hover:bg-slate-100 dark:text-slate-300 dark:hover:bg-slate-800'}"
          >{$_("pki.issueFromCSR")}</button>
        </div>
        {#if issueMode === "direct"}
        <form class="grid gap-3 sm:grid-cols-2" onsubmit={(event) => { event.preventDefault(); void issueCertificate(); }}>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.commonName")}</span>
            <input required bind:value={commonName} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.validDays")}</span>
            <input required type="number" min="1" max="36500" bind:value={validDays} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.dnsNames")}</span>
            <input bind:value={dnsNames} placeholder="server.example.net, api.example.net" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.ipAddresses")}</span>
            <input bind:value={ipAddresses} placeholder="192.0.2.10, 2001:db8::1" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <div class="sm:col-span-2">
            <button type="submit" disabled={submitting} class="rounded-lg bg-cyan-700 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-600 disabled:opacity-50">
              {submitting ? $_("common.saving") : $_("pki.issue")}
            </button>
          </div>
        </form>
        {:else}
          <form class="flex flex-col gap-3" onsubmit={(event) => { event.preventDefault(); void issueCSR(); }}>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.uploadCSR")}</span>
              <input type="file" accept=".csr,.pem,application/pkcs10" onchange={loadCSRFile} class="text-sm" />
            </label>
            <textarea bind:value={csrPEM} rows="6" placeholder="-----BEGIN CERTIFICATE REQUEST-----" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 font-mono text-xs dark:border-slate-700"></textarea>
            <button type="submit" disabled={submitting || !csrPEM.trim()} class="self-start rounded-lg bg-cyan-700 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-600 disabled:opacity-50">
              {submitting ? $_("common.saving") : $_("pki.signCSR")}
            </button>
          </form>
        {/if}
      </section>

      <section class="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div class="border-b border-slate-200 px-4 py-3 dark:border-slate-800">
          <h3 class="font-semibold text-slate-900 dark:text-white">{$_("pki.certificates")} ({certificates.length})</h3>
        </div>
        {#if certificates.length === 0}
          <p class="p-5 text-sm text-slate-500">{$_("common.noData")}</p>
        {:else}
          <div class="overflow-x-auto">
            <table class="w-full min-w-[700px] text-left text-sm">
              <thead class="bg-slate-50 text-xs uppercase text-slate-500 dark:bg-slate-950 dark:text-slate-400">
                <tr>
                  <th class="px-4 py-3">{$_("pki.subject")}</th>
                  <th class="px-4 py-3">{$_("pki.serial")}</th>
                  <th class="px-4 py-3">{$_("pki.expires")}</th>
                  <th class="px-4 py-3">{$_("pki.status")}</th>
                  <th class="px-4 py-3">{$_("common.action")}</th>
                </tr>
              </thead>
              <tbody>
                {#each certificates as cert (cert.serial)}
                  <tr class="border-t border-slate-200 dark:border-slate-800">
                    <td class="max-w-sm truncate px-2 py-1" title={cert.subject}>{cert.subject}</td>
                    <td class="px-2 py-1 font-mono text-xs">{cert.serial}</td>
                    <td class="whitespace-nowrap px-2 py-1">{renderDate(cert.expiresAt)}</td>
                    <td class="px-2 py-1">
                      {cert.revokedAt
                        ? $_("pki.revoked")
                        : cert.expiresAt < Date.now() / 1000
                          ? $_("pki.expired")
                          : $_("pki.valid")}
                    </td>
                    <td class="whitespace-nowrap px-2 py-1">
                      <button type="button" onclick={() => download(cert.serial)} class="mr-3 text-cyan-700 hover:underline dark:text-cyan-400">{$_("common.export")}</button>
                      {#if !cert.revokedAt}
                        <button type="button" onclick={() => revoke(cert)} class="text-red-700 hover:underline dark:text-red-400">{$_("pki.revoke")}</button>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>
      <a href="/api/pki/crl" download="ca.crl" class="self-start text-sm text-cyan-700 hover:underline dark:text-cyan-400">{$_("pki.downloadCRL")}</a>
      {/if}
      {#if activeSection === "servers" && settings}
        <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 class="mb-4 font-semibold text-slate-900 dark:text-white">{$_("pki.serverControl")}</h3>
          <div class="grid gap-3 sm:grid-cols-2">
            <label class="flex items-center gap-2 text-sm">
              <input type="checkbox" bind:checked={settings.enableHTTP} />
              <span>{$_("pki.enableHTTP")}</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input type="checkbox" bind:checked={settings.enableACME} />
              <span>{$_("pki.enableACME")}</span>
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.sans")}</span>
              <input bind:value={sans} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.httpBaseURL")}</span>
              <input bind:value={settings.httpBaseURL} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.acmeBaseURL")}</span>
              <input bind:value={settings.acmeBaseURL} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.crlIntervalHours")}</span>
              <input type="number" min="1" max="8760" bind:value={settings.crlIntervalHours} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.certValidityHours")}</span>
              <input type="number" min="1" max="87600" bind:value={settings.certValidityHours} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.httpPort")}</span>
              <input type="number" min="1" max="65535" bind:value={settings.httpPort} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <label class="flex flex-col gap-1 text-sm">
              <span>{$_("pki.acmePort")}</span>
              <input type="number" min="1" max="65535" bind:value={settings.acmePort} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
            </label>
            <div class="sm:col-span-2">
              <button type="button" onclick={saveSettings} disabled={submitting} class="rounded-lg bg-cyan-700 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-600 disabled:opacity-50">
                {submitting ? $_("common.saving") : $_("pki.saveAndApply")}
              </button>
            </div>
          </div>
        </section>
      {/if}
    {:else if !loading && settings && activeSection === "setup"}
      <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h3 class="mb-4 font-semibold text-slate-900 dark:text-white">{$_("pki.createCA")}</h3>
        <form class="grid gap-3 sm:grid-cols-2" onsubmit={(event) => { event.preventDefault(); void initializeCA(); }}>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.commonName")}</span>
            <input required bind:value={settings!.commonName} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.organization")}</span>
            <input bind:value={settings!.organization} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.sans")}</span>
            <input required bind:value={sans} placeholder="host.example.net, 192.0.2.10" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.caKeyType")}</span>
            <select bind:value={settings!.keyType} class="rounded-lg border border-slate-300 bg-white px-3 py-2 dark:border-slate-700 dark:bg-slate-950">
              <option value="ecdsa-256">ECDSA P-256</option>
              <option value="rsa-2048">RSA 2048</option>
              <option value="rsa-4096">RSA 4096</option>
            </select>
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.caValidYears")}</span>
            <input required type="number" min="1" max="100" bind:value={settings!.validYears} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.httpBaseURL")}</span>
            <input required bind:value={settings!.httpBaseURL} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.acmeBaseURL")}</span>
            <input required bind:value={settings!.acmeBaseURL} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.crlIntervalHours")}</span>
            <input required type="number" min="1" max="8760" bind:value={settings!.crlIntervalHours} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.certValidityHours")}</span>
            <input required type="number" min="1" max="87600" bind:value={settings!.certValidityHours} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.httpPort")}</span>
            <input required type="number" min="1" max="65535" bind:value={settings!.httpPort} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            <span>{$_("pki.acmePort")}</span>
            <input required type="number" min="1" max="65535" bind:value={settings!.acmePort} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
          </label>
          <div class="sm:col-span-2">
            <button type="submit" disabled={submitting} class="rounded-lg bg-cyan-700 px-4 py-2 text-sm font-medium text-white hover:bg-cyan-600 disabled:opacity-50">
              {submitting ? $_("common.saving") : $_("pki.createCA")}
            </button>
          </div>
        </form>
      </section>
    {/if}

    {#if activeSection === "csr"}
    <section class="rounded-xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <h3 class="mb-4 font-semibold text-slate-900 dark:text-white">{$_("pki.createCSR")}</h3>
      <p class="mb-4 text-sm text-slate-500 dark:text-slate-400">{$_("pki.csrKeyPrivacy")}</p>
      <form class="grid gap-3 sm:grid-cols-2" onsubmit={(event) => { event.preventDefault(); void createCSR(); }}>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.commonName")}</span>
          <input required bind:value={csrCommonName} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.csrKeyType")}</span>
          <select bind:value={csrKeyType} class="rounded-lg border border-slate-300 bg-white px-3 py-2 dark:border-slate-700 dark:bg-slate-950">
            <option value="rsa-2048">RSA 2048</option>
            <option value="rsa-4096">RSA 4096</option>
            <option value="rsa-8192">RSA 8192</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.organization")}</span>
          <input bind:value={csrOrganization} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.organizationalUnit")}</span>
          <input bind:value={csrOrganizationalUnit} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.country")}</span>
          <input bind:value={csrCountry} maxlength="2" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.province")}</span>
          <input bind:value={csrProvince} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.locality")}</span>
          <input bind:value={csrLocality} class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          <span>{$_("pki.sans")}</span>
          <input bind:value={csrSANs} placeholder="host.example.net, 192.0.2.10, mail@example.net" class="rounded-lg border border-slate-300 bg-transparent px-3 py-2 dark:border-slate-700" />
        </label>
        <div class="sm:col-span-2">
          <button type="submit" disabled={submitting} class="rounded-lg bg-emerald-700 px-4 py-2 text-sm font-medium text-white hover:bg-emerald-600 disabled:opacity-50">
            {submitting ? $_("common.saving") : $_("pki.downloadCSRAndKey")}
          </button>
        </div>
      </form>
      {#if generatedCSR && generatedKey}
        <div class="mt-4 flex flex-wrap gap-3 border-t border-slate-200 pt-4 text-sm dark:border-slate-800">
          <button type="button" onclick={() => saveText(`${generatedCSRBase}.csr`, generatedCSR)} class="text-cyan-700 hover:underline dark:text-cyan-400">
            {$_("pki.downloadCSR")}
          </button>
          <button type="button" onclick={() => saveText(`${generatedCSRBase}.key`, generatedKey)} class="text-cyan-700 hover:underline dark:text-cyan-400">
            {$_("pki.downloadPrivateKey")}
          </button>
        </div>
      {/if}
    </section>

    {/if}
      </div>
    </main>
  </div>
</div>
