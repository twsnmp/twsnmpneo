<script lang="ts">
  import { onMount } from "svelte";
  import { _ } from "svelte-i18n";
  import {
    ShieldCheck,
    AlertCircle,
    XCircle,
    Download,
    Trash2,
    RefreshCw,
    Key,
    FileText,
    Sliders,
    Upload,
    Check,
    AlertTriangle,
    Search,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
  } from "@lucide/svelte";
  import {
    fetchHasCA,
    fetchCreateCADefault,
    createCA,
    destroyCA,
    fetchPKICerts,
    createCSR,
    createCRT,
    revokeCert,
    exportCert,
    fetchPKIControl,
    updatePKIControl,
    type CreateCAReq,
    type CSRReqEnt,
    type PKIControlEnt,
    type PKICertEnt,
  } from "../api";
  import { showConfirm } from "../stores/modalStore";

  // Tab & state management
  let hasCA = $state<boolean>(false);
  let activeTab = $state<string>("certs");
  let loading = $state<boolean>(true);
  let submitting = $state<boolean>(false);
  let error = $state<string>("");
  let successMsg = $state<string>("");

  // Certificates & Search / Sort / Pagination
  let certs = $state<PKICertEnt[]>([]);
  let searchQuery = $state<string>("");
  let sortKey = $state<string>("Created");
  let sortDesc = $state<boolean>(true);
  let currentPage = $state<number>(1);
  let pageSize = $state<number>(20);

  // CA Setup form
  let createCAReq = $state<CreateCAReq>({
    RootCAKeyType: "ecdsa-256",
    Name: "TWSNMP Root CA",
    SANs: "",
    AcmePort: 8083,
    HttpBaseURL: "",
    AcmeBaseURL: "",
    HttpPort: 8082,
    RootCATerm: 5,
    CrlInterval: 24,
    CertTerm: 720,
  });

  // CSR Generation form
  let csrReq = $state<CSRReqEnt>({
    KeyType: "rsa-2048",
    CommonName: "",
    OrganizationalUnit: "",
    Organization: "",
    Locality: "",
    Province: "",
    Country: "JP",
    Sans: "",
  });

  // CRT Issuance form
  let crtFile = $state<File | null>(null);

  // PKI Server Control
  let pkiControl = $state<PKIControlEnt>({
    AcmeBaseURL: "",
    EnableAcme: false,
    EnableHttp: true,
    AcmeStatus: "",
    HttpStatus: "",
    CrlInterval: 24,
    CertTerm: 720,
  });

  const keyTypes = [
    { value: "rsa-2048", label: "RSA 2048 bits" },
    { value: "rsa-4096", label: "RSA 4096 bits" },
    { value: "rsa-8192", label: "RSA 8192 bits" },
    { value: "ecdsa-224", label: "ECDSA P-224" },
    { value: "ecdsa-256", label: "ECDSA P-256" },
    { value: "ecdsa-384", label: "ECDSA P-384" },
    { value: "ecdsa-521", label: "ECDSA P-521" },
  ];

  const formatNanoTime = (nano: number) => {
    if (!nano || nano <= 0) return "-";
    const ms = Math.floor(nano / 1000000);
    const d = new Date(ms);
    if (isNaN(d.getTime())) return "-";
    const pad = (n: number) => (n < 10 ? "0" + n : String(n));
    return `${d.getFullYear()}/${pad(d.getMonth() + 1)}/${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
  };

  const getTimestampStr = () => {
    const d = new Date();
    const pad = (n: number) => (n < 10 ? "0" + n : String(n));
    return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}${pad(d.getHours())}${pad(d.getMinutes())}`;
  };

  const downloadBlob = (blob: Blob, filename: string) => {
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  const showNotification = (msg: string) => {
    successMsg = msg;
    setTimeout(() => {
      if (successMsg === msg) successMsg = "";
    }, 4000);
  };

  const refresh = async () => {
    loading = true;
    error = "";
    const wasCA = hasCA;
    try {
      hasCA = await fetchHasCA();
      if (hasCA) {
        const [nextCerts, nextCtrl] = await Promise.all([
          fetchPKICerts(),
          fetchPKIControl().catch(() => pkiControl),
        ]);
        certs = nextCerts;
        pkiControl = nextCtrl;
        if (!wasCA || activeTab === "setup") {
          activeTab = "certs";
        }
      } else {
        createCAReq = await fetchCreateCADefault();
        if (activeTab !== "csr") {
          activeTab = "setup";
        }
      }
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      loading = false;
    }
  };

  const handleCreateCA = async () => {
    submitting = true;
    error = "";
    try {
      await createCA(createCAReq);
      showNotification($_("pki.createCASuccess"));
      await refresh();
      activeTab = "certs";
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      submitting = false;
    }
  };

  const handleDestroyCA = async () => {
    const ok = await showConfirm({
      title: $_("pki.destroyCA"),
      message: $_("pki.confirmResetCA"),
      type: "danger",
      confirmText: $_("common.delete"),
    });
    if (!ok) return;

    submitting = true;
    error = "";
    try {
      await destroyCA();
      showNotification($_("pki.destroyCASuccess"));
      await refresh();
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      submitting = false;
    }
  };

  const handleCreateCSR = async () => {
    submitting = true;
    error = "";
    try {
      const blob = await createCSR(csrReq);
      downloadBlob(blob, `csr_${getTimestampStr()}.zip`);
      showNotification($_("pki.csrCreatedSuccess"));
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      submitting = false;
    }
  };

  const handleCreateCRT = async () => {
    if (!crtFile) return;
    submitting = true;
    error = "";
    try {
      const blob = await createCRT(crtFile);
      downloadBlob(blob, `crt_${getTimestampStr()}.pem`);
      crtFile = null;
      showNotification($_("pki.crtIssuedSuccess"));
      await refresh();
      activeTab = "certs";
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      submitting = false;
    }
  };

  const handleExportCert = async (id: string) => {
    error = "";
    try {
      const blob = await exportCert(id);
      downloadBlob(blob, `crt_${getTimestampStr()}.pem`);
    } catch (err: any) {
      error = err.message || String(err);
    }
  };

  const handleRevokeCert = async (cert: PKICertEnt) => {
    const ok = await showConfirm({
      title: $_("pki.revoke"),
      message: $_("pki.confirmRevoke"),
      type: "danger",
      confirmText: $_("pki.revoke"),
    });
    if (!ok) return;

    error = "";
    try {
      await revokeCert(cert.ID);
      showNotification($_("pki.certRevokedSuccess"));
      await refresh();
    } catch (err: any) {
      error = err.message || String(err);
    }
  };

  const handleUpdatePKIControl = async () => {
    submitting = true;
    error = "";
    try {
      await updatePKIControl(pkiControl);
      showNotification($_("pki.controlUpdatedSuccess"));
      await refresh();
    } catch (err: any) {
      error = err.message || String(err);
    } finally {
      submitting = false;
    }
  };

  // Sorting
  const toggleSort = (key: string) => {
    if (sortKey === key) {
      sortDesc = !sortDesc;
    } else {
      sortKey = key;
      sortDesc = false;
    }
    currentPage = 1;
  };

  // Filtered & Sorted Certs
  const filteredCerts = $derived(
    certs
      .filter((c) => {
        if (!searchQuery.trim()) return true;
        const q = searchQuery.toLowerCase();
        return (
          c.ID.toLowerCase().includes(q) ||
          c.Subject.toLowerCase().includes(q) ||
          c.Node.toLowerCase().includes(q) ||
          c.Status.toLowerCase().includes(q) ||
          c.Type.toLowerCase().includes(q)
        );
      })
      .sort((a, b) => {
        let valA: any = (a as any)[sortKey];
        let valB: any = (b as any)[sortKey];
        if (typeof valA === "string") valA = valA.toLowerCase();
        if (typeof valB === "string") valB = valB.toLowerCase();
        if (valA < valB) return sortDesc ? 1 : -1;
        if (valA > valB) return sortDesc ? -1 : 1;
        return 0;
      })
  );

  // Pagination
  const totalPages = $derived(Math.max(1, Math.ceil(filteredCerts.length / pageSize)));
  const paginatedCerts = $derived(
    filteredCerts.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  );

  const exportCSV = () => {
    if (!filteredCerts.length) return;
    const headers = ["Status", "ID", "Subject", "Node", "Created", "Expire", "Revoked", "Type"];
    const rows = filteredCerts.map((c) => [
      c.Status,
      c.ID,
      `"${c.Subject.replace(/"/g, '""')}"`,
      `"${c.Node.replace(/"/g, '""')}"`,
      formatNanoTime(c.Created),
      formatNanoTime(c.Expire),
      formatNanoTime(c.Revoked),
      c.Type,
    ]);
    const csvContent = [headers.join(","), ...rows.map((r) => r.join(","))].join("\r\n");
    const blob = new Blob(["\uFEFF" + csvContent], { type: "text/csv;charset=utf-8;" });
    downloadBlob(blob, `TWSNMP_PKICert_List_${getTimestampStr()}.csv`);
  };

  onMount(() => {
    void refresh();
  });
</script>

<div class="flex h-full w-full overflow-hidden bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-100">
  <!-- Left Sidebar Menu -->
  <aside class="w-64 shrink-0 flex flex-col justify-between border-r border-slate-200 dark:border-slate-800 bg-white/70 dark:bg-slate-950/70 p-4 backdrop-blur-md">
    <div class="space-y-4">
      <!-- Sidebar Header -->
      <div class="flex items-center gap-3 px-1 py-1">
        <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-cyan-600 text-white shadow-md shadow-cyan-600/30">
          <Key class="h-5 w-5" />
        </div>
        <div>
          <h2 class="text-sm font-bold text-slate-900 dark:text-white">{$_("pki.title")}</h2>
          <p class="text-[11px] text-slate-500 dark:text-slate-400">{$_("pki.subtitle")}</p>
        </div>
      </div>

      <!-- Navigation Tabs -->
      <nav class="space-y-1 pt-2">
        {#if hasCA}
          <button
            type="button"
            onclick={() => (activeTab = "certs")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'certs' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <ShieldCheck class="h-4 w-4 shrink-0 {activeTab === 'certs' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.certManagement")}</span>
          </button>

          <button
            type="button"
            onclick={() => (activeTab = "csr")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'csr' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <FileText class="h-4 w-4 shrink-0 {activeTab === 'csr' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.createCSR")}</span>
          </button>

          <button
            type="button"
            onclick={() => (activeTab = "issue")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'issue' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Upload class="h-4 w-4 shrink-0 {activeTab === 'issue' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.issueFromCSR")}</span>
          </button>

          <button
            type="button"
            onclick={() => (activeTab = "control")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'control' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Sliders class="h-4 w-4 shrink-0 {activeTab === 'control' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.serverControl")}</span>
          </button>
        {:else}
          <button
            type="button"
            onclick={() => (activeTab = "setup")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'setup' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Key class="h-4 w-4 shrink-0 {activeTab === 'setup' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.createCA")}</span>
          </button>

          <button
            type="button"
            onclick={() => (activeTab = "csr")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all cursor-pointer {activeTab === 'csr' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <FileText class="h-4 w-4 shrink-0 {activeTab === 'csr' ? 'text-white' : 'text-cyan-600 dark:text-cyan-400'}" />
            <span class="truncate">{$_("pki.createCSR")}</span>
          </button>
        {/if}
      </nav>
    </div>

    <!-- Sidebar Status Pill -->
    <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-100/90 dark:bg-slate-900/80 p-3 text-[11px] text-slate-500 dark:text-slate-400 space-y-1.5">
      <div class="flex items-center justify-between">
        <span class="font-semibold text-slate-700 dark:text-slate-200">{$_("pki.caStatus")}</span>
        {#if hasCA}
          <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
            {$_("pki.running")}
          </span>
        {:else}
          <span class="inline-flex items-center gap-1 rounded-full bg-amber-100 dark:bg-amber-950/80 border border-amber-300 dark:border-amber-800/60 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-400">
            {$_("pki.notConfigured")}
          </span>
        {/if}
      </div>
      <div class="text-[10px] font-mono text-slate-500 dark:text-slate-400">
        {#if hasCA}
          {$_("pki.issuedCertCount", { values: { count: certs.length } })}
        {:else}
          {$_("pki.pleaseCreateCA")}
        {/if}
      </div>
    </div>
  </aside>

  <!-- Main Content Canvas -->
  <div class="flex-1 overflow-y-auto p-6 space-y-6">
    <!-- Alerts -->
    {#if error}
      <div role="alert" class="flex items-center justify-between rounded-xl border border-red-300 bg-red-50 p-4 text-xs text-red-800 dark:border-red-900 dark:bg-red-950/50 dark:text-red-200 shadow-sm">
        <div class="flex items-center gap-2">
          <AlertTriangle class="h-4 w-4 shrink-0 text-red-600 dark:text-red-400" />
          <span>{error}</span>
        </div>
        <button type="button" onclick={() => (error = "")} class="text-red-500 hover:text-red-700 text-sm font-bold">&times;</button>
      </div>
    {/if}

    {#if successMsg}
      <div role="alert" class="flex items-center justify-between rounded-xl border border-emerald-300 bg-emerald-50 p-4 text-xs text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/50 dark:text-emerald-200 shadow-sm">
        <div class="flex items-center gap-2">
          <Check class="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>{successMsg}</span>
        </div>
        <button type="button" onclick={() => (successMsg = "")} class="text-emerald-500 hover:text-emerald-700 text-sm font-bold">&times;</button>
      </div>
    {/if}

    <!-- TAB: Certificates List -->
    {#if activeTab === "certs" && hasCA}
      <!-- Top Action Bar -->
      <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-4 shadow-sm dark:shadow-lg">
        <!-- Single Search Input on Left -->
        <div class="flex items-center gap-3">
          <div class="relative w-72">
            <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              placeholder={$_("pki.searchPlaceholder")}
              bind:value={searchQuery}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 py-1.5 pl-9 pr-3 text-xs text-slate-800 dark:text-slate-100 placeholder-slate-400 dark:placeholder-slate-500 focus:border-cyan-500 focus:outline-none font-sans"
            />
          </div>
        </div>

        <!-- Right Side Action Buttons -->
        <div class="flex flex-wrap items-center gap-2.5">
          <!-- CA Certificates & CRL Downloads -->
          <a
            href="/ca.pem"
            download="root-ca.pem"
            class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors shadow-xs"
            title={$_("pki.downloadRootCATitle")}
          >
            <Download class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <span>Root CA</span>
          </a>
          <a
            href="/scepca.pem"
            download="scep-ca.pem"
            class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors shadow-xs"
            title={$_("pki.downloadScepCATitle")}
          >
            <Download class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <span>SCEP CA</span>
          </a>
          <a
            href="/crl"
            download="ca.crl"
            class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors shadow-xs"
            title={$_("pki.downloadCRLTitle")}
          >
            <Download class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
            <span>CRL</span>
          </a>

          <!-- Export CSV -->
          <button
            type="button"
            onclick={exportCSV}
            class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 px-4 py-1.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer"
          >
            <Download class="h-3.5 w-3.5" />
            <span>{$_("common.export")}</span>
          </button>

          <!-- Destroy CA Button -->
          <button
            type="button"
            onclick={handleDestroyCA}
            disabled={submitting}
            class="flex items-center gap-1.5 rounded-xl border border-rose-300 dark:border-rose-800/60 bg-rose-50 dark:bg-rose-950/40 hover:bg-rose-100 dark:hover:bg-rose-900/60 px-3.5 py-1.5 text-xs font-semibold text-rose-700 dark:text-rose-300 transition-colors cursor-pointer shadow-xs disabled:opacity-50"
            title={$_("pki.destroyCATitle")}
          >
            <Trash2 class="h-3.5 w-3.5 text-rose-500 dark:text-rose-400" />
            <span>{$_("pki.destroyCA")}</span>
          </button>

          <!-- Refresh Button -->
          <button
            type="button"
            onclick={refresh}
            disabled={loading}
            class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-slate-100 hover:bg-slate-200 dark:bg-slate-800 dark:hover:bg-slate-700 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 transition-colors cursor-pointer shadow-xs"
          >
            <RefreshCw class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400 {loading ? 'animate-spin' : ''}" />
            <span>{$_("common.refresh")}</span>
          </button>
        </div>
      </div>

      <!-- Certificates Table -->
      <div class="overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 shadow-sm dark:shadow-lg">
        <div class="overflow-x-auto">
          <table class="w-full text-left text-xs">
            <thead class="border-b border-slate-200 bg-slate-50/80 text-[11px] font-semibold uppercase tracking-wider text-slate-500 dark:border-slate-800 dark:bg-slate-950/60 dark:text-slate-400">
              <tr>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Status")}>
                  <div class="flex items-center gap-1">
                    <span>{$_("pki.status")}</span>
                    {#if sortKey === "Status"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("ID")}>
                  <div class="flex items-center gap-1">
                    <span>ID</span>
                    {#if sortKey === "ID"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Subject")}>
                  <div class="flex items-center gap-1">
                    <span>Subject</span>
                    {#if sortKey === "Subject"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Node")}>
                  <div class="flex items-center gap-1">
                    <span>{$_("pki.relatedNode")}</span>
                    {#if sortKey === "Node"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Created")}>
                  <div class="flex items-center gap-1">
                    <span>{$_("pki.created")}</span>
                    {#if sortKey === "Created"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Expire")}>
                  <div class="flex items-center gap-1">
                    <span>{$_("pki.expire")}</span>
                    {#if sortKey === "Expire"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 cursor-pointer select-none" onclick={() => toggleSort("Revoked")}>
                  <div class="flex items-center gap-1">
                    <span>{$_("pki.revoked")}</span>
                    {#if sortKey === "Revoked"}
                      {#if sortDesc}<ArrowDown class="h-3 w-3 text-cyan-600" />{:else}<ArrowUp class="h-3 w-3 text-cyan-600" />{/if}
                    {:else}
                      <ArrowUpDown class="h-3 w-3 opacity-30" />
                    {/if}
                  </div>
                </th>
                <th class="px-4 py-3 text-right">{$_("common.action")}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-slate-100 dark:divide-slate-800">
              {#if paginatedCerts.length === 0}
                <tr>
                  <td colspan="8" class="p-8 text-center text-xs text-slate-400">
                    {$_("common.noData")}
                  </td>
                </tr>
              {:else}
                {#each paginatedCerts as cert (cert.ID)}
                  <tr class="hover:bg-slate-50/60 dark:hover:bg-slate-800/40 transition-colors">
                    <td class="whitespace-nowrap px-4 py-3">
                      {#if cert.Status === "valid"}
                        <span class="inline-flex items-center gap-1 rounded-full bg-emerald-50 dark:bg-emerald-950/60 border border-emerald-200 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-300">
                          <ShieldCheck class="h-3 w-3" />
                          {$_("pki.valid")}
                        </span>
                      {:else if cert.Status === "expired"}
                        <span class="inline-flex items-center gap-1 rounded-full bg-amber-50 dark:bg-amber-950/60 border border-amber-200 dark:border-amber-800/60 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-300">
                          <AlertCircle class="h-3 w-3" />
                          {$_("pki.expired")}
                        </span>
                      {:else}
                        <span class="inline-flex items-center gap-1 rounded-full bg-rose-50 dark:bg-rose-950/60 border border-rose-200 dark:border-rose-800/60 px-2 py-0.5 text-[10px] font-semibold text-rose-700 dark:text-rose-300">
                          <XCircle class="h-3 w-3" />
                          {$_("pki.revoked")}
                        </span>
                      {/if}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 font-mono text-[11px] text-slate-600 dark:text-slate-300">{cert.ID}</td>
                    <td class="max-w-xs truncate px-4 py-3 font-medium text-slate-800 dark:text-slate-200" title={cert.Subject}>
                      {cert.Subject}
                      {#if cert.Type === "system"}
                        <span class="ml-1.5 rounded bg-cyan-100 px-1.5 py-0.5 text-[9px] font-bold text-cyan-800 dark:bg-cyan-900/60 dark:text-cyan-200">SYSTEM</span>
                      {:else if cert.Type === "scep"}
                        <span class="ml-1.5 rounded bg-indigo-100 px-1.5 py-0.5 text-[9px] font-bold text-indigo-800 dark:bg-indigo-900/60 dark:text-indigo-200">SCEP</span>
                      {:else if cert.Type === "acme"}
                        <span class="ml-1.5 rounded bg-purple-100 px-1.5 py-0.5 text-[9px] font-bold text-purple-800 dark:bg-purple-900/60 dark:text-purple-200">ACME</span>
                      {/if}
                    </td>
                    <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-400">{cert.Node || "-"}</td>
                    <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-400">{formatNanoTime(cert.Created)}</td>
                    <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-400">{formatNanoTime(cert.Expire)}</td>
                    <td class="whitespace-nowrap px-4 py-3 text-slate-600 dark:text-slate-400">{formatNanoTime(cert.Revoked)}</td>
                    <td class="whitespace-nowrap px-4 py-3 text-right">
                      <div class="flex items-center justify-end gap-1.5">
                        <button
                          type="button"
                          title={$_("pki.downloadCertTitle")}
                          onclick={() => handleExportCert(cert.ID)}
                          class="rounded-lg p-1.5 text-slate-500 hover:bg-slate-100 hover:text-cyan-600 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-cyan-400 transition-colors cursor-pointer"
                        >
                          <Download class="h-3.5 w-3.5" />
                        </button>
                        {#if cert.Type !== "system" && cert.Status === "valid"}
                          <button
                            type="button"
                            title={$_("pki.revokeCertTitle")}
                            onclick={() => handleRevokeCert(cert)}
                            class="rounded-lg p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-600 dark:text-slate-400 dark:hover:bg-rose-950/40 dark:hover:text-rose-400 transition-colors cursor-pointer"
                          >
                            <Trash2 class="h-3.5 w-3.5" />
                          </button>
                        {/if}
                      </div>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Pagination Bar -->
        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 px-4 py-3 dark:border-slate-800 text-xs text-slate-500 dark:text-slate-400 bg-slate-50/50 dark:bg-slate-950/30">
          <div class="flex items-center gap-2">
            <span>{$_("common.itemsPerPage")}</span>
            <select
              bind:value={pageSize}
              onchange={() => (currentPage = 1)}
              class="rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 px-2 py-1 text-xs text-slate-800 dark:text-slate-200 focus:outline-none"
            >
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
            <span class="ml-2">
              {filteredCerts.length === 0 ? 0 : (currentPage - 1) * pageSize + 1} - {Math.min(currentPage * pageSize, filteredCerts.length)} / {$_("common.totalCount", { values: { count: filteredCerts.length } })}
            </span>
          </div>

          <div class="flex items-center gap-1">
            <button
              type="button"
              disabled={currentPage <= 1}
              onclick={() => (currentPage = 1)}
              class="rounded-lg border border-slate-200 dark:border-slate-800 p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
            >
              <ChevronsLeft class="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              disabled={currentPage <= 1}
              onclick={() => (currentPage -= 1)}
              class="rounded-lg border border-slate-200 dark:border-slate-800 p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
            >
              <ChevronLeft class="h-3.5 w-3.5" />
            </button>
            <span class="px-2 font-medium text-slate-700 dark:text-slate-300">
              {currentPage} / {totalPages}
            </span>
            <button
              type="button"
              disabled={currentPage >= totalPages}
              onclick={() => (currentPage += 1)}
              class="rounded-lg border border-slate-200 dark:border-slate-800 p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
            >
              <ChevronRight class="h-3.5 w-3.5" />
            </button>
            <button
              type="button"
              disabled={currentPage >= totalPages}
              onclick={() => (currentPage = totalPages)}
              class="rounded-lg border border-slate-200 dark:border-slate-800 p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 cursor-pointer"
            >
              <ChevronsRight class="h-3.5 w-3.5" />
            </button>
          </div>
        </div>
      </div>
    {/if}

    <!-- TAB: CA Setup Form (When CA does not exist) -->
    {#if activeTab === "setup" && !hasCA}
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-6 shadow-sm dark:shadow-lg space-y-6">
        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-600 text-white shadow-md shadow-cyan-600/30">
            <Key class="h-5 w-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-white">{$_("pki.createCA")}</h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">{$_("pki.createCADescription")}</p>
          </div>
        </div>

        <form onsubmit={(e) => { e.preventDefault(); void handleCreateCA(); }} class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label for="pki-ca-keytype" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.caKeyType")}</label>
              <select id="pki-ca-keytype" bind:value={createCAReq.RootCAKeyType} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100">
                {#each keyTypes as kt}
                  <option value={kt.value}>{kt.label}</option>
                {/each}
              </select>
            </div>

            <div>
              <label for="pki-ca-name" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.commonName")}</label>
              <input id="pki-ca-name" required bind:value={createCAReq.Name} placeholder="TWSNMP Root CA" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div>
            <label for="pki-ca-sans" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.sans")}</label>
            <input id="pki-ca-sans" bind:value={createCAReq.SANs} placeholder="localhost, 127.0.0.1, twsnmp.local" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
          </div>

          <div class="rounded-xl border border-slate-100 bg-slate-50/70 p-4 dark:border-slate-800/80 dark:bg-slate-950/40 space-y-3">
            <h4 class="text-xs font-bold text-slate-700 dark:text-slate-300">{$_("pki.protocolServerSettings")}</h4>
            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label for="pki-ca-acmeurl" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.acmeBaseURL")}</label>
                <input id="pki-ca-acmeurl" bind:value={createCAReq.AcmeBaseURL} placeholder="https://twsnmp.local:8083" class="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" />
              </div>
              <div>
                <label for="pki-ca-acmeport" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.acmePort")}</label>
                <input id="pki-ca-acmeport" type="number" min="1" max="65535" bind:value={createCAReq.AcmePort} class="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" />
              </div>
              <div>
                <label for="pki-ca-httpurl" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.httpBaseURL")}</label>
                <input id="pki-ca-httpurl" bind:value={createCAReq.HttpBaseURL} placeholder="http://twsnmp.local:8082" class="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" />
              </div>
              <div>
                <label for="pki-ca-httpport" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.httpPort")}</label>
                <input id="pki-ca-httpport" type="number" min="1" max="65535" bind:value={createCAReq.HttpPort} class="mt-1 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100" />
              </div>
            </div>
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <div>
              <label for="pki-ca-rootterm" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.caValidYears")}</label>
              <input id="pki-ca-rootterm" type="number" min="1" max="100" bind:value={createCAReq.RootCATerm} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="pki-ca-crlint" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.crlIntervalHours")}</label>
              <input id="pki-ca-crlint" type="number" min="1" max="8760" bind:value={createCAReq.CrlInterval} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="pki-ca-certterm" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.certValidityHours")}</label>
              <input id="pki-ca-certterm" type="number" min="1" max="87600" bind:value={createCAReq.CertTerm} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div class="flex justify-end pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              type="submit"
              disabled={submitting}
              class="flex items-center gap-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-6 py-2.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer disabled:opacity-50"
            >
              <Key class="h-4 w-4" />
              <span>{submitting ? $_("common.saving") : $_("pki.buildCA")}</span>
            </button>
          </div>
        </form>
      </div>
    {/if}

    <!-- TAB: Create CSR -->
    {#if activeTab === "csr"}
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-6 shadow-sm dark:shadow-lg space-y-6">
        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-600 text-white shadow-md shadow-cyan-600/30">
            <FileText class="h-5 w-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-white">{$_("pki.createCSR")}</h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">{$_("pki.csrDescription")}</p>
          </div>
        </div>

        <form onsubmit={(e) => { e.preventDefault(); void handleCreateCSR(); }} class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label for="csr-keytype" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.csrKeyType")}</label>
              <select id="csr-keytype" bind:value={csrReq.KeyType} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100">
                {#each keyTypes as kt}
                  <option value={kt.value}>{kt.label}</option>
                {/each}
              </select>
            </div>
            <div>
              <label for="csr-cn" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.commonNameWithCN")}</label>
              <input id="csr-cn" required bind:value={csrReq.CommonName} placeholder="node1.local" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div>
            <label for="csr-sans-in" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.sans")}</label>
            <input id="csr-sans-in" bind:value={csrReq.Sans} placeholder="node1.local, 192.168.1.100" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
          </div>

          <div class="grid gap-4 sm:grid-cols-3">
            <div>
              <label for="csr-country-in" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.country")}</label>
              <input id="csr-country-in" bind:value={csrReq.Country} maxlength="2" placeholder="JP" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="csr-province-in" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.province")}</label>
              <input id="csr-province-in" bind:value={csrReq.Province} placeholder="Tokyo" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="csr-locality-in" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.locality")}</label>
              <input id="csr-locality-in" bind:value={csrReq.Locality} placeholder="Chiyoda" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label for="csr-org-in" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.organization")}</label>
              <input id="csr-org-in" bind:value={csrReq.Organization} placeholder="Example Inc." class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="csr-ou-in" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.organizationalUnit")}</label>
              <input id="csr-ou-in" bind:value={csrReq.OrganizationalUnit} placeholder="IT Dept" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div class="flex justify-end pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              type="submit"
              disabled={submitting}
              class="flex items-center gap-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-6 py-2.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer disabled:opacity-50"
            >
              <Download class="h-4 w-4" />
              <span>{submitting ? $_("common.saving") : $_("pki.downloadCSRAndKey")}</span>
            </button>
          </div>
        </form>
      </div>
    {/if}

    <!-- TAB: Issue Certificate from CSR -->
    {#if activeTab === "issue" && hasCA}
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-6 shadow-sm dark:shadow-lg space-y-6">
        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-md shadow-emerald-600/30">
            <Upload class="h-5 w-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-white">{$_("pki.issueFromCSR")}</h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">{$_("pki.issueDescription")}</p>
          </div>
        </div>

        <form onsubmit={(e) => { e.preventDefault(); void handleCreateCRT(); }} class="space-y-4">
          <div class="rounded-xl border border-dashed border-slate-300 dark:border-slate-700 p-8 text-center bg-slate-50/50 dark:bg-slate-950/40">
            <Upload class="mx-auto h-8 w-8 text-slate-400 mb-2" />
            <label for="issue-csr-file" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1">
              {$_("pki.selectCSRFile")}
            </label>
            <input
              id="issue-csr-file"
              type="file"
              accept=".csr,.pem,application/pkcs10"
              required
              onchange={(e) => {
                const target = e.currentTarget as HTMLInputElement;
                crtFile = target.files?.[0] || null;
              }}
              class="mt-2 block w-full max-w-sm mx-auto text-xs text-slate-500 file:mr-3 file:rounded-xl file:border-0 file:bg-cyan-50 file:px-4 file:py-2 file:text-xs file:font-semibold file:text-cyan-700 hover:file:bg-cyan-100 dark:file:bg-cyan-950/60 dark:file:text-cyan-300 cursor-pointer"
            />
          </div>

          <div class="flex justify-end pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              type="submit"
              disabled={submitting || !crtFile}
              class="flex items-center gap-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 px-6 py-2.5 text-xs font-bold text-white shadow-md shadow-emerald-600/30 transition-all cursor-pointer disabled:opacity-50"
            >
              <Check class="h-4 w-4" />
              <span>{submitting ? $_("common.saving") : $_("pki.signCSR")}</span>
            </button>
          </div>
        </form>
      </div>
    {/if}

    <!-- TAB: Server Control -->
    {#if activeTab === "control" && hasCA}
      <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 p-6 shadow-sm dark:shadow-lg space-y-6">
        <div class="flex items-center gap-3 border-b border-slate-100 dark:border-slate-800 pb-4">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-600 text-white shadow-md shadow-cyan-600/30">
            <Sliders class="h-5 w-5" />
          </div>
          <div>
            <h3 class="text-base font-bold text-slate-900 dark:text-white">{$_("pki.serverControl")}</h3>
            <p class="text-xs text-slate-500 dark:text-slate-400">{$_("pki.serverControlDescription")}</p>
          </div>
        </div>

        <!-- Server Status Banners -->
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="rounded-xl border p-4 text-xs {pkiControl.AcmeStatus?.includes('error') ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300' : 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-950/40 dark:text-slate-300'}">
            <div class="font-bold mb-1">{$_("pki.acmeServerStatus")}</div>
            <div class="font-mono text-[11px]">{pkiControl.AcmeStatus || $_("pki.notRunning")}</div>
          </div>
          <div class="rounded-xl border p-4 text-xs {pkiControl.HttpStatus?.includes('error') ? 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300' : 'border-slate-200 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-950/40 dark:text-slate-300'}">
            <div class="font-bold mb-1">{$_("pki.httpServerStatus")}</div>
            <div class="font-mono text-[11px]">{pkiControl.HttpStatus || $_("pki.notRunning")}</div>
          </div>
        </div>

        <form onsubmit={(e) => { e.preventDefault(); void handleUpdatePKIControl(); }} class="space-y-4">
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="flex items-center gap-2.5 rounded-xl border border-slate-200 dark:border-slate-800 p-3.5 text-xs font-semibold text-slate-700 dark:text-slate-300 cursor-pointer bg-slate-50/50 dark:bg-slate-950/30">
              <input type="checkbox" bind:checked={pkiControl.EnableAcme} class="h-4 w-4 rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
              <span>{$_("pki.enableACME")}</span>
            </label>
            <label class="flex items-center gap-2.5 rounded-xl border border-slate-200 dark:border-slate-800 p-3.5 text-xs font-semibold text-slate-700 dark:text-slate-300 cursor-pointer bg-slate-50/50 dark:bg-slate-950/30">
              <input type="checkbox" bind:checked={pkiControl.EnableHttp} class="h-4 w-4 rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
              <span>{$_("pki.enableHTTP")}</span>
            </label>
          </div>

          <div>
            <label for="ctrl-acme-url" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">{$_("pki.acmeBaseURL")}</label>
            <input id="ctrl-acme-url" bind:value={pkiControl.AcmeBaseURL} placeholder="https://twsnmp.local:8083" class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label for="ctrl-crl-int" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.crlIntervalHours")}</label>
              <input id="ctrl-crl-int" type="number" min="1" max="8760" bind:value={pkiControl.CrlInterval} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
            <div>
              <label for="ctrl-cert-term" class="block text-xs text-slate-600 dark:text-slate-400">{$_("pki.certValidityHours")}</label>
              <input id="ctrl-cert-term" type="number" min="1" max="87600" bind:value={pkiControl.CertTerm} class="mt-1 w-full rounded-xl border border-slate-300 bg-slate-50 px-3 py-2 text-xs text-slate-800 shadow-sm focus:border-cyan-500 focus:outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100" />
            </div>
          </div>

          <div class="flex justify-end pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              type="submit"
              disabled={submitting}
              class="flex items-center gap-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-6 py-2.5 text-xs font-bold text-white shadow-md shadow-cyan-600/30 transition-all cursor-pointer disabled:opacity-50"
            >
              <Sliders class="h-4 w-4" />
              <span>{submitting ? $_("common.saving") : $_("pki.updateServerControl")}</span>
            </button>
          </div>
        </form>
      </div>
    {/if}
  </div>
</div>
