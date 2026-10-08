<script lang="ts">
  import { onMount } from "svelte";
  import {
    fetchMapConf,
    saveMapConf,
    fetchNotifyConf,
    saveNotifyConf,
    uploadGeoIP,
    deleteGeoIP,
    fetchBackImage,
    saveBackImage,
    deleteBackImage,
    uploadBackImage,
    testNotifyMail,
    testNotifyWebhook,
    startNotifyOAuth2,
    deleteNotifyOAuth2Token,
    fetchNotifyOAuth2Status,
    fetchMIBModules,
    uploadMIBModule,
    deleteMIBModule,
    reloadMIBModules,
    fetchMIBTree,
    fetchCustomIcons,
    saveCustomIcon,
    saveCustomIcons,
    deleteCustomIcon,
    fetchUsers,
    createUser,
    updateUser,
    deleteUser,
    type UserEnt,
    type IconEnt,
    type MIBModuleEnt,
    type MIBTreeEnt,
    type NotifyConfEnt,
    fetchLocalModels,
    fetchAIHardwareStatus,
    type ModelInfo,
    type AIHardwareStatus,
  } from "../api";
  import { setCustomIcons } from "../common";
  import { showConfirm } from "../stores/modalStore";
  import { allMdiIcons } from "../mdiIcons";
  import ImportMapModal from "./ImportMapModal.svelte";
  import ModelManagerDialog from "./ModelManagerDialog.svelte";
  import ListPagination from "../views/list/ListPagination.svelte";
  import MIBTree from "./MIBTree.svelte";
  import {
    X,
    Save,
    Sliders,
    Bell,
    Brain,
    Database,
    CheckCircle2,
    CheckCircle,
    Server,
    Shield,
    Mail,
    Radio,
    Sparkles,
    Cpu,
    Network,
    Globe,
    Upload,
    Trash2,
    Image,
    FileUp,
    Link,
    Unlink,
    Maximize2,
    Send,
    Key,
    FolderTree,
    RefreshCw,
    AlertTriangle,
    Search,
    ArrowUp,
    ArrowDown,
    ArrowUpDown,
    Plus,
    HelpCircle,
    Edit2,
    Edit3,
    Download,
    Shapes,
    Users,
    UserPlus,
    UserCheck,
    KeyRound,
    UserCircle2,
    ShieldAlert,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let { show = $bindable(false), onSaved, currentUser = null }: { show: boolean; onSaved?: () => void; currentUser?: UserEnt | null } = $props();

  let activeTab = $state<"map" | "polling" | "receivers" | "notify" | "ai" | "database" | "mib" | "icons" | "users">("map");

  let saveMsg = $state("");
  let saveError = $state("");

  // User Management State
  let usersList = $state<UserEnt[]>([]);
  let usersLoading = $state(false);
  let userSearch = $state("");
  let showUserModal = $state(false);
  let isEditingUser = $state(false);
  let dialogUsername = $state("");
  let dialogDisplayName = $state("");
  let dialogPassword = $state("");
  let dialogPasswordConfirm = $state("");
  let dialogRole = $state("user");
  let userDialogError = $state("");
  let userDialogLoading = $state(false);
  let userOpMessage = $state("");

  const formatUserError = (msg: string): string => {
    if (!msg) return $_('users.saveFailed') || "ユーザーの保存に失敗しました";
    if (msg.includes("admin privilege required to create users")) {
      return $_('users.errAdminRequiredCreate') || "ユーザーの追加には管理者権限が必要です";
    }
    if (msg.includes("admin privilege required to delete users")) {
      return $_('users.errAdminRequiredDelete') || "ユーザーの削除には管理者権限が必要です";
    }
    if (msg.includes("cannot modify other users")) {
      return $_('users.errForbiddenModifyOther') || "他のユーザーを変更する権限がありません";
    }
    if (msg.includes("only administrator can change roles")) {
      return $_('users.errForbiddenChangeRole') || "ロールの変更には管理者権限が必要です";
    }
    if (msg.includes("cannot demote last administrator")) {
      return $_('users.errCannotDemoteLastAdmin') || "最後の管理者を格下げすることはできません";
    }
    if (msg.includes("cannot delete the only remaining user")) {
      return $_('users.errCannotDeleteLastUser') || "唯一のユーザーを削除することはできません";
    }
    if (msg.includes("cannot delete the last administrator")) {
      return $_('users.errCannotDeleteLastAdmin') || "最後の管理者を削除することはできません";
    }
    if (msg.includes("user already exists")) {
      return $_('users.errUserAlreadyExists') || "このユーザー名は既に存在します";
    }
    if (msg.includes("user not found")) {
      return $_('users.errUserNotFound') || "ユーザーが見つかりません";
    }
    if (msg.includes("username and password are required")) {
      return $_('users.errUsernameAndPasswordRequired') || "ユーザー名とパスワードは必須です";
    }
    if (msg.includes("permission denied: read-only")) {
      return $_('users.errReadonlyForbidden') || "閲覧専用アカウントのため変更操作は許可されていません";
    }
    return msg;
  };

  const loadUsers = async () => {
    usersLoading = true;
    userOpMessage = "";
    try {
      usersList = await fetchUsers();
    } catch (e: any) {
      saveError = formatUserError(e.message || "");
    } finally {
      usersLoading = false;
    }
  };

  const openAddUser = () => {
    if (currentUser && currentUser.role !== 'admin') {
      userOpMessage = "";
      saveError = $_('users.errAdminRequiredCreate') || "ユーザーの追加には管理者権限が必要です";
      return;
    }
    isEditingUser = false;
    dialogUsername = "";
    dialogDisplayName = "";
    dialogPassword = "";
    dialogPasswordConfirm = "";
    dialogRole = "user";
    userDialogError = "";
    showUserModal = true;
  };

  const openEditUser = (u: UserEnt) => {
    isEditingUser = true;
    dialogUsername = u.user;
    dialogDisplayName = u.name;
    dialogPassword = "";
    dialogPasswordConfirm = "";
    dialogRole = u.role;
    userDialogError = "";
    showUserModal = true;
  };

  const handleSaveUser = async () => {
    userDialogError = "";
    if (!dialogUsername.trim()) {
      userDialogError = $_('users.usernameRequired') || "ユーザー名を入力してください";
      return;
    }
    if (!isEditingUser && !dialogPassword) {
      userDialogError = $_('users.passwordRequired') || "パスワードを入力してください";
      return;
    }
    if (dialogPassword && dialogPassword !== dialogPasswordConfirm) {
      userDialogError = $_('users.passwordMismatch') || "パスワードが一致しません";
      return;
    }

    userDialogLoading = true;
    try {
      if (isEditingUser) {
        await updateUser(dialogUsername, {
          name: dialogDisplayName,
          password: dialogPassword || undefined,
          role: dialogRole,
        });
        userOpMessage = $_('users.userUpdated') || "ユーザーを更新しました";
      } else {
        await createUser({
          user: dialogUsername.trim(),
          name: dialogDisplayName.trim() || dialogUsername.trim(),
          password: dialogPassword,
          role: dialogRole,
        });
        userOpMessage = $_('users.userCreated') || "ユーザーを作成しました";
      }
      showUserModal = false;
      await loadUsers();
    } catch (e: any) {
      userDialogError = formatUserError(e.message || "");
    } finally {
      userDialogLoading = false;
    }
  };

  const handleDeleteUser = async (u: UserEnt) => {
    const confirmMsg = `${u.user} (${u.name})\n` + ($_('users.confirmDelete', { values: { user: u.user } }) || "このユーザーを削除してもよろしいですか？");
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'ユーザー削除の確認',
      message: confirmMsg,
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;

    usersLoading = true;
    try {
      await deleteUser(u.user);
      userOpMessage = $_('users.userDeleted') || "ユーザーを削除しました";
      await loadUsers();
    } catch (e: any) {
      saveError = formatUserError(e.message || "");
    } finally {
      usersLoading = false;
    }
  };


  // Background Image configuration
  let backImageX = $state(0);
  let backImageY = $state(0);
  let backImageW = $state(800);
  let backImageH = $state(600);
  let backImagePath = $state("");
  let backImageUploading = $state(false);
  let backImageFileInput: HTMLInputElement | null = $state(null);
  let imgNaturalW = $state(0);
  let imgNaturalH = $state(0);
  let lockRatio = $state(true);
  let showImportModal = $state(false);

  // Map configuration
  let mapName = $state("TWSNMP NEO");
  let mapSize = $state(0);
  let curMapW = $derived(
    mapSize === 1 ? 2894 : mapSize === 2 ? 4093 : (typeof window !== "undefined" && window.screen?.width > 4000 ? 5000 : 2500)
  );
  let curMapH = $derived(mapSize === 1 ? 4093 : mapSize === 2 ? 2894 : 5000);
  let previewScale = $derived(
    Math.min(
      340 / (curMapW || 2500),
      150 / (curMapH || 5000)
    )
  );
  let previewCanvasW = $derived(Math.max(40, Math.round((curMapW || 2500) * previewScale)));
  let previewCanvasH = $derived(Math.max(40, Math.round((curMapH || 5000) * previewScale)));
  let iconSize = $state(3);
  let pollInt = $state(60);
  let timeout = $state(1);
  let retry = $state(1);
  let logDays = $state(14);
  let reportDays = $state(30);
  let reportLimit = $state(10000);
  let scoreThreshold = $state(35.0);
  let fumbleThreshold = $state(10);
  let snmpMode = $state("v2c");
  let community = $state("public");
  let snmpUser = $state("");
  let snmpPassword = $state("");

  // Receivers & Daemons
  let enableSyslogd = $state(true);
  let enableTrapd = $state(true);
  let enableNetflowd = $state(false);
  let enableSFlowd = $state(false);
  let enableArpWatch = $state(false);
  let enableSshd = $state(false);
  let enableTcpd = $state(false);
  let enableOTel = $state(false);
  let enableMqtt = $state(false);
  let arpWatchRange = $state("");
  let arpTimeout = $state(60);

  // Notifications
  let notifyProvider = $state("smtp");
  let notifyLevel = $state("warn");
  let notifyInterval = $state(60);
  let notifySubject = $state("TWSNMP");
  let mailServer = $state("");
  let mailFrom = $state("");
  let mailTo = $state("");
  let mailUser = $state("");
  let mailPassword = $state("");
  let insecureSkipVerify = $state(false);
  let notifyReport = $state(false);
  let notifyLLMSummary = $state(false);
  let notifyNotifyRepair = $state(false);
  let notifyCheckDependency = $state(false);
  let notifyExecCmd = $state("");
  let notifyWebHookNotify = $state("");
  let notifyWebHookReport = $state("");
  let notifyClientID = $state("");
  let notifyClientSecret = $state("");
  let notifyMSTenant = $state("");
  let notifyOAuth2HasToken = $state(false);
  let notifyTestingMail = $state(false);
  let notifyTestingWebhook = $state(false);
  let notifyTestMsg = $state("");
  let notifyTestError = $state("");

  // AI / LLM configuration
  let llmProvider = $state("tensai");
  let llmBaseUrl = $state("http://localhost:11434");
  let llmModel = $state("qwen2.5-0.5b");
  let llmApiKey = $state("");
  let enableMCP = $state(true);
  let mcpMode = $state("noauth");
  let mcpFrom = $state("");

  let showModelManager = $state(false);
  let localAIModels = $state<ModelInfo[]>([]);
  let aiHardware = $state<AIHardwareStatus | null>(null);

  async function loadLocalAIInfo() {
    try {
      localAIModels = (await fetchLocalModels()) || [];
      aiHardware = await fetchAIHardwareStatus();
      if (llmProvider === "tensai") {
        if (localAIModels.length > 0 && (!llmModel || !localAIModels.some(m => m.name === llmModel))) {
          llmModel = localAIModels[0].name;
        }
      }
    } catch {
      // Ignore
    }
  }

  const tensaiPresetModels = [
    { id: "qwen2.5-0.5b", name: "Qwen 2.5 0.5B Instruct (Fast / ~500MB)" },
    { id: "qwen2.5-1.5b", name: "Qwen 2.5 1.5B Instruct (Balanced / ~980MB)" },
    { id: "qwen2.5-coder-0.5b", name: "Qwen 2.5 Coder 0.5B (~500MB)" },
    { id: "qwen2.5-coder-1.5b", name: "Qwen 2.5 Coder 1.5B (~980MB)" },
    { id: "smollm2-360m", name: "SmolLM2 360M Instruct (Ultra-light / ~380MB)" },
    { id: "smollm2-1.7b", name: "SmolLM2 1.7B Instruct (~1.1GB)" },
    { id: "llama-3.2-1b", name: "Llama 3.2 1B Instruct (~750MB)" },
    { id: "deepseek-r1-1.5b", name: "DeepSeek R1 Distill Qwen 1.5B (~1.1GB)" },
    { id: "tinyllama", name: "TinyLlama 1.1B Chat (~670MB)" },
  ];

  // Data Store
  let logFormat = $state("parquet");

  // MIB Management state
  let mibModules = $state<MIBModuleEnt[]>([]);
  let mibLoading = $state(false);
  let mibFilter = $state("");
  let mibMsg = $state("");
  let mibError = $state("");
  let mibFileInput = $state<HTMLInputElement | null>(null);
  let showMIBTreeModal = $state(false);
  let mibTreeData = $state<MIBTreeEnt[]>([]);
  let mibSortColumn = $state<"index" | "type" | "name" | "file" | "status" | null>(null);
  let mibSortDirection = $state<"asc" | "desc">("asc");
  let mibPageSize = $state(25);
  let mibCurrentPage = $state(1);

  function handleMibSort(col: "index" | "type" | "name" | "file" | "status") {
    if (mibSortColumn === col) {
      mibSortDirection = mibSortDirection === "asc" ? "desc" : "asc";
    } else {
      mibSortColumn = col;
      mibSortDirection = "asc";
    }
  }

  async function loadMIBModules() {
    mibLoading = true;
    mibError = "";
    try {
      mibModules = await fetchMIBModules();
    } catch (e: any) {
      mibError = e?.message || String(e);
    } finally {
      mibLoading = false;
    }
  }

  async function handleReloadMIB() {
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await reloadMIBModules();
      mibMsg = $_('config.mibReloadSuccess');
      setTimeout(() => (mibMsg = ""), 3000);
    } catch (e: any) {
      mibError = e?.message || String(e);
    } finally {
      mibLoading = false;
    }
  }

  async function handleUploadMIB(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await uploadMIBModule(file);
      mibMsg = $_('config.mibUploadSuccess', { values: { file: file.name } });
      setTimeout(() => (mibMsg = ""), 4000);
    } catch (e: any) {
      mibError = e?.message || String(e);
    } finally {
      mibLoading = false;
      if (mibFileInput) mibFileInput.value = "";
    }
  }

  async function handleDeleteMIB(filePath: string) {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'MIB削除の確認',
      message: $_("config.mibDeleteConfirm"),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    mibLoading = true;
    mibMsg = "";
    mibError = "";
    try {
      mibModules = await deleteMIBModule(filePath);
      mibMsg = $_('config.mibDeleteSuccess');
      setTimeout(() => (mibMsg = ""), 3000);
    } catch (e: any) {
      mibError = e?.message || String(e);
    } finally {
      mibLoading = false;
    }
  }

  async function handleOpenMIBTree() {
    showMIBTreeModal = true;
    if (mibTreeData.length === 0) {
      try {
        mibTreeData = await fetchMIBTree();
      } catch (e: any) {
        mibError = e?.message || String(e);
      }
    }
  }

  const filteredMibModules = $derived.by(() => {
    const q = mibFilter.trim().toLowerCase();
    if (!q) return mibModules;
    return mibModules.filter((m) => {
      const name = (m.name || m.Name || "").toLowerCase();
      const file = (m.file || m.File || "").toLowerCase();
      const err = (m.error || m.Error || "").toLowerCase();
      const type = (m.type || m.Type || "").toLowerCase();
      return name.includes(q) || file.includes(q) || err.includes(q) || type.includes(q);
    });
  });

  const sortedMibModules = $derived.by(() => {
    const indexed = filteredMibModules.map((m, originalIdx) => ({
      mod: m,
      originalIdx,
    }));

    if (!mibSortColumn || mibSortColumn === "index") {
      if (mibSortColumn === "index" && mibSortDirection === "desc") {
        return [...indexed].reverse().map((i) => i.mod);
      }
      return indexed.map((i) => i.mod);
    }

    const direction = mibSortDirection === "asc" ? 1 : -1;
    return [...indexed].sort((a, b) => {
      let valA = "";
      let valB = "";
      if (mibSortColumn === "type") {
        valA = a.mod.type || a.mod.Type || "";
        valB = b.mod.type || b.mod.Type || "";
      } else if (mibSortColumn === "name") {
        valA = a.mod.name || a.mod.Name || "";
        valB = b.mod.name || b.mod.Name || "";
      } else if (mibSortColumn === "file") {
        valA = a.mod.file || a.mod.File || "";
        valB = b.mod.file || b.mod.File || "";
      } else if (mibSortColumn === "status") {
        valA = a.mod.error || a.mod.Error || "OK";
        valB = b.mod.error || b.mod.Error || "OK";
      }
      const cmp = valA.localeCompare(valB, undefined, { numeric: true, sensitivity: "base" });
      if (cmp !== 0) return direction * cmp;
      return a.originalIdx - b.originalIdx;
    }).map((i) => i.mod);
  });

  const paginatedMibModules = $derived.by(() => {
    if (mibPageSize === -1) return sortedMibModules;
    const totalPages = Math.max(1, Math.ceil(sortedMibModules.length / mibPageSize));
    const page = Math.min(Math.max(1, mibCurrentPage), totalPages);
    const start = (page - 1) * mibPageSize;
    return sortedMibModules.slice(start, start + mibPageSize);
  });

  // Custom Icon Management state
  let customIcons = $state<IconEnt[]>([]);
  let iconLoading = $state(false);
  let iconFilter = $state("");
  let iconSortColumn = $state<"name" | "type" | "code" | null>(null);
  let iconSortDirection = $state<"asc" | "desc">("asc");
  let iconPageSize = $state(10);
  let iconCurrentPage = $state(1);

  // Icon Dialog state
  let showIconDialog = $state(false);
  let editingIcon = $state<IconEnt | null>(null);
  let dialogIconType = $state<"mdi" | "image">("mdi");
  let dialogIconImage = $state("");
  let dialogMdiFilter = $state("");
  let dialogSelectedMdi = $state("access-point");
  let dialogIconName = $state("");
  let dialogIconCode = $state<string | number>(983043);
  let dialogIconError = $state("");
  let dialogParsedCode = $derived(parseIconCode(dialogIconCode));

  let iconFileInput = $state<HTMLInputElement | null>(null);
  let imageFileInput = $state<HTMLInputElement | null>(null);

  const filteredMdiList = $derived.by(() => {
    const q = dialogMdiFilter.trim().toLowerCase();
    if (!q) {
      return allMdiIcons.slice(0, 300);
    }
    return allMdiIcons.filter(ic => ic.name.toLowerCase().includes(q)).slice(0, 500);
  });

  function parseIconCode(input: string | number): number {
    if (typeof input === "number") return input;
    const s = String(input).trim();
    if (!s) return 0;
    if (s.startsWith("0x") || s.startsWith("0X")) {
      return parseInt(s, 16) || 0;
    }
    if (/^[0-9a-fA-F]{4,6}$/.test(s) && (s.toUpperCase().startsWith("F") || s.toUpperCase().startsWith("E"))) {
      return parseInt(s, 16) || 0;
    }
    return parseInt(s, 10) || 0;
  }

  function formatCodePreview(codeNum: number): string {
    if (!codeNum || codeNum <= 0) return "-";
    try {
      return String.fromCodePoint(codeNum);
    } catch {
      return "?";
    }
  }

  async function loadIcons() {
    iconLoading = true;
    try {
      const list = await fetchCustomIcons();
      customIcons = list || [];
      setCustomIcons(customIcons);
    } catch (e: any) {
      console.error("Failed to load custom icons:", e);
    } finally {
      iconLoading = false;
    }
  }

  function handleSelectMdi(mdiName: string) {
    const found = allMdiIcons.find((m) => m.name === mdiName);
    if (found) {
      dialogSelectedMdi = found.name;
      dialogIconCode = found.code;
      if (!editingIcon) {
        dialogIconName = found.name;
      }
    }
  }

  function openAddIconDialog() {
    editingIcon = null;
    dialogIconType = "mdi";
    dialogIconImage = "";
    dialogMdiFilter = "";
    dialogSelectedMdi = "access-point";
    dialogIconCode = 983043;
    dialogIconName = "access-point";
    dialogIconError = "";
    showIconDialog = true;
  }

  function openEditIconDialog(item: IconEnt) {
    editingIcon = item;
    const nameVal = item.name || item.Name || "";
    const codeVal = Number(item.code ?? item.Code ?? 0);
    const typeVal = (item.type || item.Type || (item.image || item.Image ? "image" : "mdi")) as "mdi" | "image";
    const imageVal = item.image || item.Image || "";
    dialogIconType = typeVal;
    dialogIconImage = imageVal;
    dialogIconName = nameVal;
    dialogIconCode = codeVal;
    if (typeVal === "mdi") {
      const found = allMdiIcons.find((m) => m.code === codeVal || m.name === nameVal);
      dialogSelectedMdi = found ? found.name : "";
      dialogMdiFilter = found ? found.name : "";
    } else {
      dialogSelectedMdi = "";
      dialogMdiFilter = "";
    }
    dialogIconError = "";
    showIconDialog = true;
  }

  function handleImageFileSelected(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    if (file.size > 2 * 1024 * 1024) {
      dialogIconError = $_('config.iconImageSizeError');
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      dialogIconImage = reader.result as string;
      if (!dialogIconName) {
        const base = file.name.replace(/\.[^/.]+$/, "");
        dialogIconName = base;
      }
      dialogIconError = "";
    };
    reader.onerror = () => {
      dialogIconError = $_('config.iconImageReadError');
    };
    reader.readAsDataURL(file);
  }

  async function handleSaveIcon() {
    const name = dialogIconName.trim();
    if (!name) {
      dialogIconError = $_('config.iconName') + " is required";
      return;
    }
    if (dialogIconType === "image") {
      if (!dialogIconImage) {
        dialogIconError = $_('config.iconSelectImageFile');
        return;
      }
      try {
        await saveCustomIcon({
          id: editingIcon?.id || editingIcon?.ID,
          name,
          code: 0,
          type: "image",
          image: dialogIconImage,
        });
        showIconDialog = false;
        await loadIcons();
        saveMsg = $_('config.iconSaveSuccess');
        setTimeout(() => (saveMsg = ""), 3000);
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } catch (e: any) {
        dialogIconError = e.message || String(e);
      }
    } else {
      const code = parseIconCode(dialogIconCode);
      if (!code || code <= 0) {
        dialogIconError = $_('config.iconCode') + " is required";
        return;
      }
      try {
        await saveCustomIcon({
          id: editingIcon?.id || editingIcon?.ID,
          name,
          code,
          type: "mdi",
          image: "",
        });
        showIconDialog = false;
        await loadIcons();
        saveMsg = $_('config.iconSaveSuccess');
        setTimeout(() => (saveMsg = ""), 3000);
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } catch (e: any) {
        dialogIconError = e.message || String(e);
      }
    }
  }

  async function handleDeleteIcon(item: IconEnt) {
    const name = item.name || item.Name || "";
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'アイコン削除の確認',
      message: $_('config.iconDeleteConfirm', { values: { name } }),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    try {
      await deleteCustomIcon(name);
      await loadIcons();
      saveMsg = $_('config.iconDeleteSuccess');
      setTimeout(() => (saveMsg = ""), 3000);
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
      }
    } catch (e: any) {
      saveError = e.message || String(e);
    }
  }

  async function handleImportIcons(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    try {
      const text = await file.text();
      let list: Partial<IconEnt>[] = [];
      if (file.name.endsWith(".json")) {
        const parsed = JSON.parse(text);
        if (Array.isArray(parsed)) {
          list = parsed.map((item: any) => {
            const isImg = item.type === "image" || item.Type === "image" || Boolean(item.image || item.Image);
            return {
              name: item.name || item.Name || "",
              code: isImg ? 0 : parseIconCode(item.code ?? item.Code),
              type: isImg ? "image" : "mdi",
              image: item.image || item.Image || "",
            };
          }).filter((x) => x.name && (x.type === "image" ? Boolean(x.image) : Boolean(x.code)));
        }
      } else {
        const lines = text.split(/\r?\n/);
        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("名前") || trimmed.toLowerCase().startsWith("name")) continue;
          const parts = trimmed.split(/,|\t/).map((p) => p.replace(/^["']|["']$/g, '').trim());
          if (parts.length >= 2) {
            const name = parts[0];
            const code = parseIconCode(parts[1]);
            if (name && code) {
              list.push({ name, code, type: "mdi" });
            }
          }
        }
      }
      if (list.length > 0) {
        await saveCustomIcons(list);
        await loadIcons();
        saveMsg = $_('config.iconImportSuccess', { values: { count: list.length } });
        setTimeout(() => (saveMsg = ""), 3000);
        if (typeof window !== "undefined") {
          window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
        }
      } else {
        saveError = $_('config.iconImportFailed');
      }
    } catch (err: any) {
      saveError = `${$_('config.iconImportFailed')}: ${err.message || err}`;
    } finally {
      if (iconFileInput) iconFileInput.value = "";
    }
  }

  function handleExportIcons() {
    const data = JSON.stringify(customIcons, null, 2);
    const blob = new Blob([data], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `twsnmp_icons_${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  function handleIconSort(col: "name" | "type" | "code") {
    if (iconSortColumn === col) {
      iconSortDirection = iconSortDirection === "asc" ? "desc" : "asc";
    } else {
      iconSortColumn = col;
      iconSortDirection = "asc";
    }
  }

  const filteredCustomIcons = $derived.by(() => {
    const q = iconFilter.trim().toLowerCase();
    if (!q) return customIcons;
    return customIcons.filter((ic) => {
      const name = (ic.name || ic.Name || "").toLowerCase();
      const type = (ic.type || ic.Type || (ic.image || ic.Image ? "image" : "mdi")).toLowerCase();
      const code = String(ic.code ?? ic.Code ?? "");
      const hexCode = "0x" + (Number(ic.code ?? ic.Code ?? 0)).toString(16).toLowerCase();
      return name.includes(q) || type.includes(q) || code.includes(q) || hexCode.includes(q);
    });
  });

  const sortedCustomIcons = $derived.by(() => {
    const list = [...filteredCustomIcons];
    if (!iconSortColumn) return list;
    const dir = iconSortDirection === "asc" ? 1 : -1;
    return list.sort((a, b) => {
      if (iconSortColumn === "name") {
        const na = a.name || a.Name || "";
        const nb = b.name || b.Name || "";
        return dir * na.localeCompare(nb, undefined, { numeric: true });
      } else if (iconSortColumn === "type") {
        const ta = a.type || a.Type || (a.image || a.Image ? "image" : "mdi");
        const tb = b.type || b.Type || (b.image || b.Image ? "image" : "mdi");
        return dir * ta.localeCompare(tb);
      } else if (iconSortColumn === "code") {
        const ca = Number(a.code ?? a.Code ?? 0);
        const cb = Number(b.code ?? b.Code ?? 0);
        return dir * (ca - cb);
      }
      return 0;
    });
  });

  const paginatedCustomIcons = $derived.by(() => {
    if (iconPageSize === -1) return sortedCustomIcons;
    const totalPages = Math.max(1, Math.ceil(sortedCustomIcons.length / iconPageSize));
    const page = Math.min(Math.max(1, iconCurrentPage), totalPages);
    const start = (page - 1) * iconPageSize;
    return sortedCustomIcons.slice(start, start + iconPageSize);
  });

  async function loadConfig() {
    try {
      const conf = await fetchMapConf();
      if (conf) {
        mapName = conf.MapName ?? conf.map_name ?? "TWSNMP NEO";
        mapSize = conf.MapSize ?? conf.map_size ?? 0;
        iconSize = conf.IconSize ?? conf.icon_size ?? 3;
        pollInt = conf.PollInt ?? conf.poll_int ?? 60;
        timeout = conf.Timeout ?? conf.timeout ?? 1;
        retry = conf.Retry ?? conf.retry ?? 1;
        logDays = conf.LogDays ?? (conf as any).log_days ?? 14;
        reportDays = conf.ReportDays ?? (conf as any).report_days ?? 30;
        reportLimit = conf.ReportLimit ?? (conf as any).report_limit ?? 10000;
        scoreThreshold = conf.ScoreThreshold ?? (conf as any).score_threshold ?? 35.0;
        fumbleThreshold = conf.FumbleThreshold ?? (conf as any).fumble_threshold ?? 10;
        snmpMode = conf.SnmpMode ?? (conf as any).snmp_mode ?? "v2c";
        community = conf.Community ?? conf.community ?? "public";
        snmpUser = conf.SnmpUser ?? conf.snmp_user ?? "";
        snmpPassword = conf.SnmpPassword ?? conf.snmp_password ?? "";

        enableSyslogd = conf.EnableSyslogd ?? conf.enable_syslogd ?? true;
        enableTrapd = conf.EnableTrapd ?? conf.enable_trapd ?? true;
        enableNetflowd = conf.EnableNetflowd ?? conf.enable_netflowd ?? false;
        enableSFlowd = conf.EnableSFlowd ?? conf.enable_sflowd ?? false;
        enableArpWatch = conf.EnableArpWatch ?? conf.enable_arp_watch ?? false;
        enableSshd = conf.EnableSshd ?? conf.enable_sshd ?? false;
        enableTcpd = conf.EnableTcpd ?? conf.enable_tcpd ?? false;
        enableOTel = conf.EnableOTel ?? conf.enable_otel ?? false;
        enableMqtt = conf.EnableMqtt ?? conf.enable_mqtt ?? false;
        arpWatchRange = conf.ArpWatchRange ?? conf.arp_watch_range ?? "";
        arpTimeout = conf.ArpTimeout ?? conf.arp_timeout ?? 60;

        llmProvider = conf.LLMProvider ?? conf.llm_provider ?? "tensai";
        llmBaseUrl = conf.LLMBaseURL ?? conf.llm_base_url ?? "http://localhost:11434";
        llmModel = conf.LLMModel ?? conf.llm_model ?? "gemini-1.5-flash";
        llmApiKey = conf.LLMAPIKey ?? conf.llm_api_key ?? "";
        enableMCP = conf.EnableMCP ?? (conf as any).enable_mcp ?? true;
        mcpMode = conf.MCPMode ?? (conf as any).mcp_mode ?? "noauth";
        mcpFrom = conf.MCPFrom ?? (conf as any).mcp_from ?? "";
        logFormat = conf.LogFormat ?? conf.log_format ?? "parquet";
        geoIPInfo = conf.GeoIPInfo ?? conf.geo_ip_info ?? "";
        await loadLocalAIInfo();
      }

      const nConf = await fetchNotifyConf().catch(() => null);
      if (nConf) {
        notifyProvider = nConf.Provider || "smtp";
        notifyLevel = nConf.Level || "warn";
        notifyInterval = nConf.Interval ?? 60;
        notifySubject = nConf.Subject || "TWSNMP";
        mailServer = nConf.MailServer || "";
        mailFrom = nConf.MailFrom || "";
        mailTo = nConf.MailTo || "";
        mailUser = nConf.User || "";
        mailPassword = nConf.Password || "";
        insecureSkipVerify = nConf.InsecureSkipVerify || false;
        notifyReport = nConf.Report || false;
        notifyLLMSummary = nConf.LLMSummary || false;
        notifyNotifyRepair = nConf.NotifyRepair || false;
        notifyCheckDependency = nConf.CheckDependency || false;
        notifyExecCmd = nConf.ExecCmd || "";
        notifyWebHookNotify = nConf.WebHookNotify || "";
        notifyWebHookReport = nConf.WebHookReport || "";
        notifyClientID = nConf.ClientID || "";
        notifyClientSecret = nConf.ClientSecret || "";
        notifyMSTenant = nConf.MSTenant || "";
      }

      const oauth2Status = await fetchNotifyOAuth2Status().catch(() => null);
      if (oauth2Status) {
        notifyOAuth2HasToken = Boolean(oauth2Status.hasToken);
      }

      const bi = await fetchBackImage().catch(() => null);
      if (bi) {
        backImageX = bi.X ?? 0;
        backImageY = bi.Y ?? 0;
        backImageW = (bi.Width && bi.Width > 0) ? bi.Width : 0;
        backImageH = (bi.Height && bi.Height > 0) ? bi.Height : 0;
        backImagePath = bi.Path ?? "";
        if (backImagePath) {
          updateImageNaturalSize(backImagePath, false);
        }
      }
    } catch (e) {
      console.error("Failed to load configuration:", e);
    }
  }

  // BackImage helpers and handlers
  function updateImageNaturalSize(src: string, forceResetSize: boolean = false) {
    if (!src) {
      imgNaturalW = 0;
      imgNaturalH = 0;
      return;
    }
    const img = new window.Image();
    img.onload = () => {
      imgNaturalW = img.naturalWidth;
      imgNaturalH = img.naturalHeight;
      if (forceResetSize || !backImageW || !backImageH || backImageW <= 0 || backImageH <= 0) {
        backImageW = img.naturalWidth;
        backImageH = img.naturalHeight;
      }
    };
    img.src = src;
  }

  function handleWidthChange(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value, 10) || 0;
    backImageW = val;
    if (lockRatio && imgNaturalW > 0 && imgNaturalH > 0 && val > 0) {
      backImageH = Math.round((val * imgNaturalH) / imgNaturalW);
    }
  }

  function handleHeightChange(e: Event) {
    const val = parseInt((e.target as HTMLInputElement).value, 10) || 0;
    backImageH = val;
    if (lockRatio && imgNaturalW > 0 && imgNaturalH > 0 && val > 0) {
      backImageW = Math.round((val * imgNaturalW) / imgNaturalH);
    }
  }

  function resetToOriginalSize() {
    if (imgNaturalW > 0 && imgNaturalH > 0) {
      backImageW = imgNaturalW;
      backImageH = imgNaturalH;
    }
  }

  function setScaleMultiplier(mult: number) {
    if (imgNaturalW > 0 && imgNaturalH > 0) {
      backImageW = Math.round(imgNaturalW * mult);
      backImageH = Math.round(imgNaturalH * mult);
    }
  }

  function fitToMapWidth() {
    if (curMapW > 0) {
      backImageW = curMapW;
      if (imgNaturalW > 0 && imgNaturalH > 0) {
        backImageH = Math.round((curMapW * imgNaturalH) / imgNaturalW);
      }
    }
  }

  function centerOnMap() {
    backImageX = Math.max(0, Math.round((curMapW - backImageW) / 2));
    backImageY = Math.max(0, Math.round((curMapH - backImageH) / 2));
  }

  function alignTopLeft() {
    backImageX = 0;
    backImageY = 0;
  }

  async function handleUploadBackImageFile(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    backImageUploading = true;
    saveError = "";
    try {
      const file = target.files[0];
      const localUrl = URL.createObjectURL(file);
      updateImageNaturalSize(localUrl, true);
      const res = await uploadBackImage(file);
      if (res?.path) {
        backImagePath = res.path;
      }
    } catch (err: any) {
      saveError = `${$_('config.backImageUploadFailed')}: ${err.message || err}`;
    } finally {
      backImageUploading = false;
    }
  }

  async function handleClearBackImage() {
    backImagePath = "";
    imgNaturalW = 0;
    imgNaturalH = 0;
    backImageX = 0;
    backImageY = 0;
    backImageW = 0;
    backImageH = 0;
    await deleteBackImage().catch(console.error);
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
    }
  }

  // GeoIP database handlers
  let geoIPInfo = $state("");
  let geoIPLoading = $state(false);
  let geoIPFileInput: HTMLInputElement | undefined = $state();

  async function handleUploadGeoIP(e: Event) {
    const target = e.target as HTMLInputElement;
    if (!target.files || target.files.length === 0) return;
    const file = target.files[0];
    geoIPLoading = true;
    saveMsg = "";
    saveError = "";
    try {
      const res = await uploadGeoIP(file);
      saveMsg = `${$_('config.geoipUpdated')} (Ver: ${res.version || "OK"})`;
      await loadConfig();
    } catch (err: any) {
      saveError = `${$_('config.geoipUpdateFailed')}: ${err.message || err}`;
    } finally {
      geoIPLoading = false;
      if (geoIPFileInput) geoIPFileInput.value = "";
    }
  }

  async function handleDeleteGeoIP() {
    const ok = await showConfirm({
      title: $_('common.confirmDelete') || 'GeoIP削除の確認',
      message: $_('config.geoipConfirmDelete'),
      type: 'danger',
      confirmText: $_('common.delete') || '削除',
    });
    if (!ok) return;
    geoIPLoading = true;
    saveMsg = "";
    saveError = "";
    try {
      await deleteGeoIP();
      saveMsg = $_('config.geoipDeleted');
      geoIPInfo = "";
      await loadConfig();
    } catch (err: any) {
      saveError = `${$_('config.geoipDeleteFailed')}: ${err.message || err}`;
    } finally {
      geoIPLoading = false;
    }
  }

  $effect(() => {
    if (show) {
      loadConfig();
      if (activeTab === "mib") {
        loadMIBModules();
      } else if (activeTab === "icons") {
        loadIcons();
      }
      saveMsg = "";
      saveError = "";
    }
  });

  const handleSave = async () => {
    saveMsg = "";
    saveError = "";
    try {
      // 1. Save Map & Core Conf
      await saveMapConf({
        MapName: mapName,
        MapSize: Number(mapSize),
        IconSize: Number(iconSize),
        PollInt: Number(pollInt),
        Timeout: Number(timeout),
        Retry: Number(retry),
        LogDays: Number(logDays),
        ReportDays: Number(reportDays),
        ReportLimit: Number(reportLimit),
        ScoreThreshold: Number(scoreThreshold),
        FumbleThreshold: Number(fumbleThreshold),
        SnmpMode: snmpMode,
        Community: community,
        SnmpUser: snmpUser,
        SnmpPassword: snmpPassword,
        EnableSyslogd: Boolean(enableSyslogd),
        EnableTrapd: Boolean(enableTrapd),
        EnableNetflowd: Boolean(enableNetflowd),
        EnableSFlowd: Boolean(enableSFlowd),
        EnableArpWatch: Boolean(enableArpWatch),
        EnableSshd: Boolean(enableSshd),
        EnableTcpd: Boolean(enableTcpd),
        EnableOTel: Boolean(enableOTel),
        EnableMqtt: Boolean(enableMqtt),
        ArpWatchRange: arpWatchRange,
        ArpTimeout: Number(arpTimeout),
        LLMProvider: llmProvider,
        LLMBaseURL: llmBaseUrl,
        LLMModel: llmModel,
        LLMAPIKey: llmApiKey,
        EnableMCP: Boolean(enableMCP),
        MCPMode: mcpMode,
        MCPFrom: mcpFrom,
        LogFormat: logFormat,
      });

      // 2. Save Notify Conf
      await saveNotifyConf({
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

      // 3. Save BackImage Conf
      await saveBackImage({
        X: Number(backImageX) || 0,
        Y: Number(backImageY) || 0,
        Width: Number(backImageW) || 800,
        Height: Number(backImageH) || 600,
        Path: backImagePath,
      });

      saveMsg = $_('config.saveSuccess');
      if (typeof window !== "undefined") {
        window.dispatchEvent(new CustomEvent("twsnmp:reload-map"));
      }
      onSaved?.();
      setTimeout(() => {
        if (saveMsg) show = false;
      }, 1200);
    } catch (e: any) {
      saveError = $_('config.saveError') + ": " + (e.message || e);
    }
  };

  const currentNotifyConf = (): NotifyConfEnt => ({
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
      await testNotifyMail(currentNotifyConf());
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
      await testNotifyWebhook(currentNotifyConf());
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
      // First save current configuration so OAuth2 uses updated client id/secret
      await saveNotifyConf(currentNotifyConf());
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

{#if show}
  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-4 backdrop-blur-md"
    role="dialog"
    aria-modal="true"
    tabindex="-1"
    onkeydown={(e) => { if (e.key === "Escape") show = false; }}
  >
    <div class="flex h-[88vh] w-full max-w-7xl flex-col rounded-2xl border border-slate-800 bg-white dark:bg-[#0b1329] shadow-2xl overflow-hidden text-slate-800 dark:text-slate-200">
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-4">
        <div class="flex items-center gap-3">
          <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/10 border border-cyan-500/30 text-cyan-400">
            <Sliders class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100">{$_('config.title')}</h2>
            <p class="text-[11px] text-slate-400">{$_('app.subtitle')}</p>
          </div>
        </div>
        <button
          type="button"
          aria-label={$_('common.close')}
          onclick={() => (show = false)}
          class="rounded-xl p-1.5 text-slate-400 hover:bg-slate-800 hover:text-slate-100 transition-colors"
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Main Layout -->
      <div class="flex flex-1 overflow-hidden">
        <!-- Sidebar Navigation -->
        <div class="w-52 border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950/60 p-3 space-y-1.5 shrink-0">
          <button
            type="button"
            onclick={() => (activeTab = "map")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'map' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Sliders class="h-4 w-4" />
            {$_('config.tabMap')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "polling")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'polling' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Network class="h-4 w-4" />
            {$_('config.tabPolling')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "receivers")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'receivers' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Radio class="h-4 w-4" />
            {$_('config.tabReceivers')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "notify")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'notify' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Bell class="h-4 w-4" />
            {$_('config.tabNotify')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "ai")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'ai' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Brain class="h-4 w-4" />
            {$_('config.tabAi')}
          </button>
          <button
            type="button"
            onclick={() => (activeTab = "database")}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'database' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Database class="h-4 w-4" />
            {$_('config.tabDatabase')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "mib"; loadMIBModules(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'mib' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <FolderTree class="h-4 w-4" />
            {$_('config.tabMib')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "icons"; loadIcons(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'icons' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Sparkles class="h-4 w-4" />
            {$_('config.tabIcons')}
          </button>
          <button
            type="button"
            onclick={() => { activeTab = "users"; loadUsers(); }}
            class="flex w-full items-center gap-2.5 rounded-xl px-3.5 py-2.5 text-xs font-semibold transition-all {activeTab === 'users' ? 'bg-gradient-to-r from-cyan-600 to-cyan-500 text-white shadow-md shadow-cyan-600/30' : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-900 hover:text-slate-900 dark:hover:text-slate-200'}"
          >
            <Users class="h-4 w-4" />
            {$_('config.tabUsers') || 'ユーザー管理'}
          </button>
        </div>

        <!-- Form Panels -->
        <div class="flex-1 overflow-y-auto p-6 text-xs bg-slate-50/50 dark:bg-slate-900/40">
          {#if saveMsg}
            <div class="mb-5 flex items-center gap-2 rounded-xl border border-emerald-800/40 bg-emerald-950/40 p-3.5 text-xs font-medium text-emerald-300 shadow-sm">
              <CheckCircle2 class="h-4 w-4 text-emerald-400 shrink-0" />
              <span>{saveMsg}</span>
            </div>
          {/if}
          {#if saveError}
            <div class="mb-5 flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3.5 text-xs font-medium text-rose-300 shadow-sm">
              <X class="h-4 w-4 text-rose-400 shrink-0" />
              <span>{saveError}</span>
            </div>
          {/if}

          <!-- Map settings -->
          {#if activeTab === "map"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <Sliders class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('config.tabMapTitle')}
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="map-name" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mapName')}</label>
                    <input
                      id="map-name"
                      type="text"
                      bind:value={mapName}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="map-size" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mapSize')}</label>
                    <select
                      id="map-size"
                      bind:value={mapSize}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value={0}>{$_('config.mapSizeAuto')}</option>
                      <option value={1}>A4P (2894x4093)</option>
                      <option value={2}>A4L (4093x2894)</option>
                    </select>
                  </div>
                </div>

                <div class="grid grid-cols-1 gap-4 pt-1">
                  <div>
                    <label for="icon-size" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('config.iconSize')}: <span class="font-mono text-cyan-600 dark:text-cyan-400">{iconSize}</span> ({$_('config.iconSizeDesc')})
                    </label>
                    <input
                      id="icon-size"
                      type="range"
                      min={1}
                      max={5}
                      bind:value={iconSize}
                      class="w-full accent-cyan-500 cursor-pointer"
                    />
                  </div>
                </div>
              </div>
            </div>
          {/if}

          <!-- Polling settings -->
          {#if activeTab === "polling"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <Network class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('config.pollingSnmpTitle')}
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div>
                    <label for="poll-int" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.pollInt')}</label>
                    <input
                      id="poll-int"
                      type="number"
                      min={5}
                      bind:value={pollInt}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="poll-timeout" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.timeout')}</label>
                    <input
                      id="poll-timeout"
                      type="number"
                      min={1}
                      bind:value={timeout}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="poll-retry" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.retry')}</label>
                    <input
                      id="poll-retry"
                      type="number"
                      min={0}
                      bind:value={retry}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
                  <div>
                    <label for="snmp-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.snmpMode')}</label>
                    <select
                      id="snmp-mode"
                      bind:value={snmpMode}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="v2c">SNMP v2c ({$_('config.recommended')})</option>
                      <option value="v3">SNMP v3</option>
                      <option value="v1">SNMP v1</option>
                    </select>
                  </div>
                  <div>
                    <label for="snmp-comm" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.community')}</label>
                    <input
                      id="snmp-comm"
                      type="text"
                      bind:value={community}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>

                {#if snmpMode === "v3"}
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2 border-t border-slate-200 dark:border-slate-800/60">
                    <div>
                      <label for="snmp-user" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpUser')}</label>
                      <input
                        id="snmp-user"
                        type="text"
                        bind:value={snmpUser}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="snmp-pwd" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('node.snmpPassword')}</label>
                      <input
                        id="snmp-pwd"
                        type="password"
                        bind:value={snmpPassword}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                  </div>
                {/if}
              </div>
            </div>
          {/if}

          <!-- GeoIP Database Section (TWSNMP FC / FK Compatible) -->
          {#if activeTab === "database"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                  <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                    <Globe class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                    {$_('config.geoipTitle')}
                  </h3>
                  {#if geoIPInfo}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse"></span>
                      {$_('config.geoipActive')} (Ver: {geoIPInfo})
                    </span>
                  {:else}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 px-2.5 py-0.5 text-[10px] font-medium text-slate-600 dark:text-slate-400">
                      {$_('config.geoipInactive')}
                    </span>
                  {/if}
                </div>

                <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
                  {$_('config.geoipDesc')}
                </p>

                <!-- Fixed: Light/Dark adaptive card for GeoIP upload -->
                <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950/60 p-4 space-y-3">
                  <div class="flex flex-wrap items-center justify-between gap-3">
                    <div class="space-y-0.5">
                      <span class="block text-xs font-semibold text-slate-800 dark:text-slate-200">{$_('config.geoipFile')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.geoipFileDesc')}</span>
                    </div>

                    <div class="flex items-center gap-2">
                      <input
                        type="file"
                        accept=".mmdb"
                        bind:this={geoIPFileInput}
                        onchange={handleUploadGeoIP}
                        class="hidden"
                        id="geoip-file-input"
                      />
                      <label
                        for="geoip-file-input"
                        class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold text-cyan-600 dark:text-cyan-300 transition-colors cursor-pointer {geoIPLoading ? 'opacity-50 pointer-events-none' : ''}"
                      >
                        <Upload class="w-3.5 h-3.5" />
                        <span>{geoIPLoading ? $_('common.loading') : $_('config.geoipApply')}</span>
                      </label>

                      {#if geoIPInfo}
                        <button
                          type="button"
                          onclick={handleDeleteGeoIP}
                          disabled={geoIPLoading}
                          class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-600 dark:text-rose-300 transition-colors cursor-pointer disabled:opacity-50"
                        >
                          <Trash2 class="w-3.5 h-3.5" />
                          <span>{$_('common.delete')}</span>
                        </button>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>

              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <Database class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('config.logRetentionTitle')}
                </h3>
                <div class="max-w-sm">
                  <label for="log-days" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.logDays')}</label>
                  <input
                    id="log-days"
                    type="number"
                    min={1}
                    max={365}
                    bind:value={logDays}
                    class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                  />
                </div>
              </div>
            </div>
          {/if}

          {#if activeTab === "map"}
            <div class="mt-6 space-y-6 max-w-2xl">
              <!-- Background Image Section -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                  <div class="flex items-center gap-2">
                    <Image class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100">
                      {$_('config.backImageTitle')}
                    </h3>
                    {#if imgNaturalW > 0}
                      <span class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                        ({$_('config.originalSize')}: {imgNaturalW} × {imgNaturalH} px)
                      </span>
                    {/if}
                  </div>
                  {#if backImagePath}
                    <span class="inline-flex items-center gap-1.5 rounded-full border border-cyan-500/30 bg-cyan-500/10 px-2.5 py-0.5 text-[10px] font-semibold text-cyan-600 dark:text-cyan-400">
                      {$_('config.backImageConfigured')}
                    </span>
                  {/if}
                </div>

                <!-- Dimension and Position Controls -->
                <div class="space-y-3">
                  <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div>
                      <label for="bg-x" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">X (px)</label>
                      <input
                        id="bg-x"
                        type="number"
                        bind:value={backImageX}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="bg-y" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">Y (px)</label>
                      <input
                        id="bg-y"
                        type="number"
                        bind:value={backImageY}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                    <div>
                      <div class="flex items-center justify-between mb-1">
                        <label for="bg-w" class="text-xs font-semibold text-slate-600 dark:text-slate-400">{$_('drawItem.width')}</label>
                        <button
                          type="button"
                          onclick={() => (lockRatio = !lockRatio)}
                          title={lockRatio ? $_('config.unlockRatio') : $_('config.lockRatio')}
                          class="text-[10px] p-0.5 rounded transition-colors {lockRatio ? 'text-cyan-600 dark:text-cyan-400 hover:text-cyan-500' : 'text-slate-400 hover:text-slate-600'}"
                        >
                          {#if lockRatio}
                            <Link class="w-3.5 h-3.5" />
                          {:else}
                            <Unlink class="w-3.5 h-3.5" />
                          {/if}
                        </button>
                      </div>
                      <input
                        id="bg-w"
                        type="number"
                        value={backImageW}
                        oninput={handleWidthChange}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="bg-h" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('drawItem.height')}</label>
                      <input
                        id="bg-h"
                        type="number"
                        value={backImageH}
                        oninput={handleHeightChange}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 py-1.5 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                  </div>

                  <!-- Quick Presets -->
                  {#if backImagePath}
                    <div class="flex flex-wrap items-center gap-1.5 pt-1 text-[11px]">
                      <span class="text-slate-500 font-medium">{$_('config.presets')}:</span>
                      <button
                        type="button"
                        onclick={resetToOriginalSize}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        100% ({$_('config.originalSize')})
                      </button>
                      <button
                        type="button"
                        onclick={() => setScaleMultiplier(0.5)}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        50%
                      </button>
                      <button
                        type="button"
                        onclick={() => setScaleMultiplier(2.0)}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        200%
                      </button>
                      <button
                        type="button"
                        onclick={fitToMapWidth}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        {$_('config.fitMapWidth')}
                      </button>
                      <span class="text-slate-300 dark:text-slate-700">|</span>
                      <button
                        type="button"
                        onclick={alignTopLeft}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        (0, 0)
                      </button>
                      <button
                        type="button"
                        onclick={centerOnMap}
                        class="px-2 py-0.5 rounded-lg border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300 transition-colors"
                      >
                        {$_('config.centerMap')}
                      </button>
                    </div>

                    <!-- Mini Canvas Placement Visualizer -->
                    <div class="rounded-xl border border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-950 p-3 space-y-2">
                      <div class="flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400">
                        <span class="font-semibold flex items-center gap-1.5">
                          <Maximize2 class="w-3.5 h-3.5 text-cyan-500" />
                          {$_('config.placementPreview')}
                        </span>
                        <span class="font-mono text-[10px]">
                          {$_('config.mapCanvas')} {curMapW} × {curMapH} px
                        </span>
                      </div>

                      {#if backImageW <= 0 || backImageH <= 0}
                        <div class="text-[11px] text-amber-500 dark:text-amber-400 bg-amber-500/10 border border-amber-500/20 rounded-lg p-2">
                          {$_('config.backImageZeroWarning')}
                        </div>
                      {/if}

                      <!-- Simulated miniature map canvas container -->
                      <div class="relative w-full h-44 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-900/60 overflow-hidden flex items-center justify-center p-3">
                        <!-- Canvas Boundary Visual -->
                        <div
                          class="relative bg-white dark:bg-slate-950 border-2 border-slate-400 dark:border-slate-600 rounded shadow-md overflow-hidden transition-all"
                          style="width: {previewCanvasW}px; height: {previewCanvasH}px;"
                        >
                          <!-- Placed Image Visual Box -->
                          <div
                            class="absolute border-2 border-cyan-500 bg-cyan-500/20 overflow-hidden transition-all flex items-center justify-center"
                            style="
                              left: {((Number(backImageX) || 0) / curMapW) * 100}%;
                              top: {((Number(backImageY) || 0) / curMapH) * 100}%;
                              width: {((Number(backImageW) || 0) / curMapW) * 100}%;
                              height: {((Number(backImageH) || 0) / curMapH) * 100}%;
                            "
                          >
                            <img src={backImagePath} alt="BackImage" class="w-full h-full object-fill opacity-75 pointer-events-none" />
                          </div>
                        </div>
                      </div>
                    </div>
                  {/if}
                </div>

                <div class="flex items-center gap-3 pt-2">
                  <input
                    type="file"
                    accept="image/*"
                    bind:this={backImageFileInput}
                    onchange={handleUploadBackImageFile}
                    class="hidden"
                    id="backimage-file-input"
                  />
                  <label
                    for="backimage-file-input"
                    class="flex items-center gap-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 px-3.5 py-2 text-xs font-semibold text-cyan-600 dark:text-cyan-300 transition-colors cursor-pointer {backImageUploading ? 'opacity-50 pointer-events-none' : ''}"
                  >
                    <Upload class="w-3.5 h-3.5" />
                    <span>{backImageUploading ? $_('common.loading') : $_('config.backImageSelect')}</span>
                  </label>

                  {#if backImagePath}
                    <button
                      type="button"
                      onclick={handleClearBackImage}
                      class="flex items-center gap-1.5 rounded-xl border border-rose-500/30 bg-rose-500/10 hover:bg-rose-500/20 px-3 py-2 text-xs font-semibold text-rose-600 dark:text-rose-300 transition-colors cursor-pointer"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                      <span>{$_('config.backImageClear')}</span>
                    </button>
                  {/if}
                </div>
              </div>

              <!-- Map Import Section -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
                <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                  <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                    <FileUp class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                    {$_('config.importMapTitle')}
                  </h3>
                </div>
                <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
                  {$_('config.importMapDesc')}
                </p>
                <div class="pt-1">
                  <button
                    type="button"
                    onclick={() => (showImportModal = true)}
                    class="flex items-center gap-2 rounded-xl border border-cyan-500/40 bg-cyan-50 dark:bg-cyan-950/40 px-4 py-2 text-xs font-bold text-cyan-700 dark:text-cyan-300 hover:bg-cyan-100 dark:hover:bg-cyan-900/50 transition-colors cursor-pointer"
                  >
                    <FileUp class="w-4 h-4" />
                    <span>{$_('config.importMapButton')}</span>
                  </button>
                </div>
              </div>
            </div>
          {/if}

          <!-- Receivers & Daemons -->
          {#if activeTab === "receivers"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <Radio class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('config.receiverTitle')}
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableSyslogd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.syslogd')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.syslogDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableTrapd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.trapd')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.trapDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableNetflowd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.netflowd')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.netflowDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableSFlowd} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.sflowd')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.sflowDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableArpWatch} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.arpwatch')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.arpWatchDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableOTel} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.otel')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.otelDesc')}</span>
                    </div>
                  </label>

                  <label class="flex items-center gap-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 hover:border-slate-300 dark:hover:border-slate-700 cursor-pointer transition-colors">
                    <input type="checkbox" bind:checked={enableMqtt} class="h-4 w-4 rounded border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-900 text-cyan-500 focus:ring-cyan-500/20" />
                    <div>
                      <span class="block font-semibold text-slate-800 dark:text-slate-200 text-xs">{$_('config.mqtt')}</span>
                      <span class="block text-[11px] text-slate-500 dark:text-slate-400">{$_('config.mqttDesc')}</span>
                    </div>
                  </label>
                </div>

                {#if enableArpWatch}
                  <div class="mt-3 p-3 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 space-y-3">
                    <label for="arp-range" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1">{$_('config.arpRangeLabel')}</label>
                    <input
                      id="arp-range"
                      type="text"
                      bind:value={arpWatchRange}
                      placeholder={$_('config.arpRangePlaceholder')}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-white dark:bg-slate-900 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {/if}
              </div>
            </div>

          <!-- TAB 3: Notifications -->
          {/if}

          {#if activeTab === "notify"}
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
                    class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors disabled:opacity-50"
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
                            class="text-[10px] text-rose-500 hover:text-rose-600 underline"
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
                        class="px-3.5 py-1.5 rounded-xl border border-cyan-500/40 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors"
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
                    class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-cyan-500/30 bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-600 dark:text-cyan-300 text-xs font-semibold transition-colors disabled:opacity-50"
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


          <!-- TAB 4: AI & LLM Settings -->
          {/if}

          {#if activeTab === "ai"}
            <div class="space-y-6 max-w-2xl">
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <Brain class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                  {$_('config.aiTitle')}
                </h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div>
                    <label for="llm-provider" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiProvider')}</label>
                    <select
                      id="llm-provider"
                      bind:value={llmProvider}
                      onchange={() => {
                        if (llmProvider === "tensai" && (!llmModel || llmModel.includes("gemini") || llmModel.includes("gpt") || llmModel.includes("claude"))) {
                          llmModel = "qwen2.5-0.5b";
                        } else if (llmProvider === "gemini" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
                          llmModel = "gemini-1.5-flash";
                        } else if (llmProvider === "openai" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
                          llmModel = "gpt-4o-mini";
                        } else if (llmProvider === "claude" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
                          llmModel = "claude-3-5-sonnet-20241022";
                        } else if (llmProvider === "ollama" && (!llmModel || llmModel === "qwen2.5-0.5b")) {
                          llmModel = "llama3";
                        }
                      }}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value="none">{$_('config.aiProviderNone')}</option>
                      <option value="tensai">{$_('config.aiTensai')}</option>
                      <option value="gemini">{$_('config.aiGeminiRecommended')}</option>
                      <option value="openai">{$_('config.aiOpenAI')}</option>
                      <option value="claude">{$_('config.aiClaude')}</option>
                      <option value="ollama">{$_('config.aiOllama')}</option>
                    </select>
                  </div>

                  {#if llmProvider === "tensai"}
                    <div class="space-y-1.5">
                      <div class="flex items-center justify-between">
                        <label for="llm-model-tensai" class="block text-xs font-semibold text-slate-600 dark:text-slate-400">
                          {$_('config.aiTensaiModel')}
                        </label>
                        {#if aiHardware}
                          <div class="flex items-center gap-1.5 text-[11px]">
                            <span class="text-slate-400">{$_('config.aiHardware')}:</span>
                            {#if aiHardware.acceleration === "GPU"}
                              <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">GPU</span>
                            {:else if aiHardware.acceleration && aiHardware.acceleration.includes("SIMD")}
                              <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">SIMD</span>
                            {:else}
                              <span class="px-1.5 py-0.5 text-[10px] font-bold rounded bg-slate-500/10 text-slate-600 dark:text-slate-400 border border-slate-500/20">CPU</span>
                            {/if}
                          </div>
                        {/if}
                      </div>

                      <div class="flex gap-2 items-center">
                        <div class="flex-1">
                          {#if localAIModels.length > 0}
                            <select
                              id="llm-model-tensai"
                              bind:value={llmModel}
                              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                            >
                              {#each localAIModels as m}
                                <option value={m.name}>{m.name} ({m.size_human})</option>
                              {/each}
                            </select>
                          {:else}
                            <button
                              type="button"
                              onclick={() => (showModelManager = true)}
                              class="w-full text-left px-3.5 py-2 rounded-xl border border-amber-300 dark:border-amber-800/80 bg-amber-50 dark:bg-amber-950/30 text-xs font-medium text-amber-800 dark:text-amber-300 hover:bg-amber-100 transition-colors"
                            >
                              {$_('config.aiNoLocalModel')}
                            </button>
                          {/if}
                        </div>
                        <button
                          type="button"
                          onclick={() => (showModelManager = true)}
                          class="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-semibold shadow-sm transition-all shrink-0 cursor-pointer"
                        >
                          <Cpu class="w-3.5 h-3.5" />
                          <span>{$_('config.aiManageModels')}</span>
                        </button>
                      </div>
                    </div>
                  {:else if llmProvider && llmProvider !== "none"}
                    <div>
                      <label for="llm-model" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiModel')}</label>
                      <input
                        id="llm-model"
                        type="text"
                        bind:value={llmModel}
                        class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                      />
                    </div>
                  {/if}
                </div>

                {#if llmProvider === "tensai"}
                  <div class="p-3.5 rounded-xl bg-cyan-50 dark:bg-cyan-950/30 border border-cyan-200 dark:border-cyan-900/50 text-xs text-cyan-800 dark:text-cyan-300 flex items-start gap-2.5">
                    <Sparkles class="w-4 h-4 text-cyan-500 shrink-0 mt-0.5" />
                    <div class="space-y-1">
                      <p class="font-semibold">{$_('config.aiTensai')}</p>
                      <p class="text-[11px] leading-relaxed text-cyan-700 dark:text-cyan-400">
                        {$_('config.aiTensaiDesc')}
                      </p>
                      <p class="text-[11px] leading-relaxed text-cyan-600/90 dark:text-cyan-400/90">
                        {$_('config.aiTensaiModelHelp')}
                      </p>
                    </div>
                  </div>
                {:else if llmProvider === "ollama"}
                  <div>
                    <label for="llm-url" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiOllamaUrl')}</label>
                    <input
                      id="llm-url"
                      type="text"
                      bind:value={llmBaseUrl}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {:else if llmProvider && llmProvider !== "none"}
                  <div>
                    <label for="llm-key" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.aiApiKey')}</label>
                    <input
                      id="llm-key"
                      type="password"
                      bind:value={llmApiKey}
                      placeholder="{llmProvider.toUpperCase()} {$_('config.aiApiKeyPlaceholder')}"
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                {/if}
              </div>

              <!-- MCP Server Settings -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                  <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                    <Cpu class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                    {$_('config.mcpTitle')}
                  </h3>
                  <label class="relative inline-flex items-center cursor-pointer">
                    <input type="checkbox" bind:checked={enableMCP} class="sr-only peer" />
                    <div class="w-9 h-5 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-slate-600 peer-checked:bg-cyan-600"></div>
                    <span class="ml-2 text-xs font-medium text-slate-700 dark:text-slate-300">{$_('config.mcpEnable')}</span>
                  </label>
                </div>

                {#if enableMCP}
                  <div class="space-y-4">
                    <!-- Endpoint Display -->
                    <div>
                      <span class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpEndpoint')}</span>
                      <div class="flex items-center gap-2">
                        <input
                          type="text"
                          readonly
                          value="/api/mcp"
                          class="w-full rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-950/60 px-3.5 py-2 text-xs font-mono text-cyan-600 dark:text-cyan-400 focus:outline-none select-all"
                        />
                      </div>
                      <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-1">
                        {$_('config.mcpEndpointDesc')}
                      </p>
                    </div>

                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <!-- Auth Mode -->
                      <div>
                        <label for="mcp-mode" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpMode')}</label>
                        <select
                          id="mcp-mode"
                          bind:value={mcpMode}
                          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                        >
                          <option value="noauth">{$_('config.mcpModeNoAuth')}</option>
                          <option value="auth">{$_('config.mcpModeAuth')}</option>
                        </select>
                      </div>

                      <!-- Client IP Filter (mcpFrom) -->
                      <div>
                        <label for="mcp-from" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">{$_('config.mcpFrom')}</label>
                        <input
                          id="mcp-from"
                          type="text"
                          bind:value={mcpFrom}
                          placeholder={$_('config.mcpFromPlaceholder')}
                          class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                        />
                      </div>
                    </div>
                    <p class="text-[11px] text-slate-500 dark:text-slate-400">
                      {$_('config.mcpFromHelp')}
                    </p>
                  </div>
                {/if}
              </div>
            </div>

          <!-- TAB 5: Datastore & System -->
          {/if}

          {#if activeTab === "database"}
            <div class="mt-6 space-y-6 max-w-2xl">
              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <!-- BBolt Status Card -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
                  <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                    <div class="flex items-center gap-2">
                      <Database class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                      <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.bboltTitle')}</h4>
                    </div>
                    <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
                      {$_('config.statusRunning')}
                    </span>
                  </div>
                  <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
                    {$_('config.bboltDesc')}
                  </p>
                  <div class="pt-2 text-[11px] text-slate-700 dark:text-slate-300 font-mono space-y-1">
                    <div class="flex justify-between">
                      <span class="text-slate-500 dark:text-slate-400">{$_('config.dbPath')}</span>
                      <span class="text-slate-900 dark:text-slate-200">./data/twsnmpneo.db</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-slate-500 dark:text-slate-400">{$_('config.integrityCheck')}</span>
                      <span class="text-emerald-600 dark:text-emerald-400 font-semibold">{$_('config.statusNormalOk')}</span>
                    </div>
                  </div>
                </div>

                <!-- Parquet Status Card -->
                <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/90 p-5 shadow-sm dark:shadow-lg space-y-3">
                  <div class="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-2.5">
                    <div class="flex items-center gap-2">
                      <Server class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                      <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.parquetTitle')}</h4>
                    </div>
                    <span class="inline-flex items-center gap-1 rounded-full bg-emerald-100 dark:bg-emerald-950/80 border border-emerald-300 dark:border-emerald-800/60 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:text-emerald-400">
                      {$_('config.statusRunning')}
                    </span>
                  </div>
                  <p class="text-[11px] text-slate-600 dark:text-slate-400 leading-relaxed">
                    {$_('config.parquetDesc')}
                  </p>
                  <div class="pt-2 text-[11px] text-slate-700 dark:text-slate-300 font-mono space-y-1">
                    <div class="flex justify-between">
                      <span class="text-slate-500 dark:text-slate-400">{$_('config.saveDir')}</span>
                      <span class="text-slate-900 dark:text-slate-200">./data/logs</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-slate-500 dark:text-slate-400">{$_('config.compression')}</span>
                      <span class="text-cyan-600 dark:text-cyan-400">Snappy / Parquet v2</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Format selector -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-3">
                <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100">{$_('config.logFormatTitle')}</h4>
                <p class="text-[11px] text-slate-600 dark:text-slate-400">
                  {$_('config.logFormatDesc')}
                </p>
                <div class="flex items-center gap-4 pt-1">
                  <label class="flex items-center gap-2 cursor-pointer">
                    <input type="radio" bind:group={logFormat} value="parquet" class="text-cyan-500 focus:ring-cyan-500/20" />
                    <span class="text-xs text-slate-800 dark:text-slate-200 font-medium">{$_('config.parquetOption')}</span>
                  </label>
                </div>
              </div>

              <!-- Report Retention & Anomaly Score Thresholds -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="border-b border-slate-200 dark:border-slate-800 pb-2.5">
                  <h4 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                    <Sliders class="w-4 h-4 text-cyan-500" />
                    {$_('config.reportSettingTitle')}
                  </h4>
                  <p class="text-[11px] text-slate-600 dark:text-slate-400 mt-1">
                    {$_('config.reportSettingDesc')}
                  </p>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div>
                    <label for="report-days" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('config.reportDaysLabel')}
                    </label>
                    <input
                      id="report-days"
                      type="number"
                      min="1"
                      max="365"
                      bind:value={reportDays}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="report-limit" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('config.reportLimitLabel')}
                    </label>
                    <input
                      id="report-limit"
                      type="number"
                      min="100"
                      max="1000000"
                      step="500"
                      bind:value={reportLimit}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="score-threshold" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('config.scoreThresholdLabel')}
                    </label>
                    <input
                      id="score-threshold"
                      type="number"
                      min="1"
                      max="100"
                      step="0.5"
                      bind:value={scoreThreshold}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div>
                    <label for="fumble-threshold" class="block text-xs font-semibold text-slate-600 dark:text-slate-400 mb-1.5">
                      {$_('config.fumbleThresholdLabel')}
                    </label>
                    <input
                      id="fumble-threshold"
                      type="number"
                      min="1"
                      max="1000"
                      bind:value={fumbleThreshold}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-mono text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                </div>
              </div>
            </div>
          {/if}

          <!-- MIB Management -->
          {#if activeTab === "mib"}
            <div class="space-y-6">
              <!-- Header info card -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                      <FolderTree class="h-4 w-4 text-teal-500" />
                      {$_('config.mibTitle')}
                    </h3>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                      {$_('config.mibDesc')}
                    </p>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      onclick={handleOpenMIBTree}
                      class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm"
                    >
                      <FolderTree class="h-3.5 w-3.5 text-teal-500" />
                      {$_('config.mibTree')}
                    </button>
                    <button
                      type="button"
                      disabled={mibLoading}
                      onclick={handleReloadMIB}
                      class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm disabled:opacity-50"
                    >
                      <RefreshCw class={`h-3.5 w-3.5 text-cyan-500 ${mibLoading ? 'animate-spin' : ''}`} />
                      {$_('config.mibReload')}
                    </button>
                    <label class="flex items-center gap-1.5 rounded-xl bg-teal-600 hover:bg-teal-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm cursor-pointer transition-colors">
                      <Upload class="h-3.5 w-3.5" />
                      {$_('config.mibUpload')}
                      <input
                        type="file"
                        bind:this={mibFileInput}
                        accept=".txt,.mib,.asn1,.my"
                        onchange={handleUploadMIB}
                        class="hidden"
                      />
                    </label>
                  </div>
                </div>

                {#if mibMsg}
                  <div class="rounded-xl border border-emerald-300 bg-emerald-50 dark:border-emerald-900/60 dark:bg-emerald-950/40 p-3 text-xs text-emerald-700 dark:text-emerald-300 flex items-center gap-2">
                    <CheckCircle2 class="h-4 w-4 shrink-0 text-emerald-500" />
                    <span>{mibMsg}</span>
                  </div>
                {/if}
                {#if mibError}
                  <div class="rounded-xl border border-rose-300 bg-rose-50 dark:border-rose-900/60 dark:bg-rose-950/40 p-3 text-xs text-rose-700 dark:text-rose-300 flex items-center gap-2">
                    <X class="h-4 w-4 shrink-0 text-rose-500" />
                    <span>{mibError}</span>
                  </div>
                {/if}

                <!-- Search and Stats Toolbar -->
                <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-100 dark:border-slate-800/80">
                  <div class="relative flex-1 min-w-[220px]">
                    <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
                    <input
                      type="text"
                      bind:value={mibFilter}
                      oninput={() => (mibCurrentPage = 1)}
                      placeholder={$_('config.mibSearch')}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                    {$_('config.mibTotalModules')}: <span class="font-bold text-slate-800 dark:text-slate-200">{filteredMibModules.length}</span> / {mibModules.length}
                  </div>
                </div>
              </div>

              <!-- MIB Modules Table -->
              <div class="overflow-x-auto rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm dark:shadow-lg">
                <table class="w-full min-w-[900px] whitespace-nowrap text-left text-xs">
                  <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="p-3 w-12 text-center" aria-sort={mibSortColumn === 'index' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
                        <button
                          type="button"
                          class="inline-flex items-center justify-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
                          onclick={() => handleMibSort('index')}
                        >
                          <span>#</span>
                          {#if mibSortColumn === 'index'}
                            {#if mibSortDirection === 'asc'}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {:else}
                            <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                          {/if}
                        </button>
                      </th>
                      <th class="p-3 w-28" aria-sort={mibSortColumn === 'type' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
                          onclick={() => handleMibSort('type')}
                        >
                          <span>{$_('config.mibType')}</span>
                          {#if mibSortColumn === 'type'}
                            {#if mibSortDirection === 'asc'}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {:else}
                            <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                          {/if}
                        </button>
                      </th>
                      <th class="p-3 w-56" aria-sort={mibSortColumn === 'name' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
                          onclick={() => handleMibSort('name')}
                        >
                          <span>{$_('config.mibName')}</span>
                          {#if mibSortColumn === 'name'}
                            {#if mibSortDirection === 'asc'}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {:else}
                            <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                          {/if}
                        </button>
                      </th>
                      <th class="p-3" aria-sort={mibSortColumn === 'file' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
                          onclick={() => handleMibSort('file')}
                        >
                          <span>{$_('config.mibFile')}</span>
                          {#if mibSortColumn === 'file'}
                            {#if mibSortDirection === 'asc'}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {:else}
                            <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                          {/if}
                        </button>
                      </th>
                      <th class="p-3 w-32" aria-sort={mibSortColumn === 'status' ? (mibSortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}>
                        <button
                          type="button"
                          class="inline-flex items-center gap-1 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200"
                          onclick={() => handleMibSort('status')}
                        >
                          <span>{$_('config.mibStatus')}</span>
                          {#if mibSortColumn === 'status'}
                            {#if mibSortDirection === 'asc'}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {:else}
                            <ArrowUpDown class="h-2.5 w-2.5 text-slate-400 dark:text-slate-600" />
                          {/if}
                        </button>
                      </th>
                      <th class="p-3 w-16 text-center">{$_('config.mibAction')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono">
                    {#each paginatedMibModules as mod, idx}
                      {@const isExt = mod.type === "ext" || mod.Type === "ext"}
                      {@const hasErr = Boolean(mod.error || mod.Error)}
                      {@const fileName = mod.file || mod.File}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="py-1 px-2 text-center text-slate-400 text-[11px]">
                          {(mibPageSize === -1 ? 0 : (mibCurrentPage - 1) * mibPageSize) + idx + 1}
                        </td>
                        <td class="py-1 px-2">
                          {#if isExt}
                            <span class="inline-flex items-center gap-1 rounded-md bg-indigo-50 dark:bg-indigo-950/60 px-2 py-0.5 text-[10px] font-semibold text-indigo-700 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-800/60">
                              {$_('config.mibExt')}
                            </span>
                          {:else}
                            <span class="inline-flex items-center gap-1 rounded-md bg-teal-50 dark:bg-teal-950/60 px-2 py-0.5 text-[10px] font-semibold text-teal-700 dark:text-teal-300 border border-teal-200 dark:border-teal-800/60">
                              {$_('config.mibInt')}
                            </span>
                          {/if}
                        </td>
                        <td class="py-1 px-2 font-semibold text-slate-900 dark:text-slate-100">
                          {mod.name || mod.Name || "Unknown"}
                        </td>
                        <td class="py-1 px-2 text-slate-600 dark:text-slate-400 text-[11px]">
                          {mod.file || mod.File}
                        </td>
                        <td class="py-1 px-2">
                          {#if hasErr}
                            <span class="inline-flex items-center gap-1 text-[11px] text-rose-600 dark:text-rose-400" title={mod.error || mod.Error}>
                              <AlertTriangle class="h-3.5 w-3.5 shrink-0" />
                              <span class="truncate max-w-[120px]">{mod.error || mod.Error}</span>
                            </span>
                          {:else}
                            <span class="inline-flex items-center gap-1 text-[11px] text-emerald-600 dark:text-emerald-400">
                              <CheckCircle class="h-3.5 w-3.5 shrink-0" />
                              OK
                            </span>
                          {/if}
                        </td>
                        <td class="py-0 px-1 text-center">
                          {#if isExt && fileName}
                            <button
                              type="button"
                              onclick={() => handleDeleteMIB(fileName)}
                              class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                              title={$_('common.delete')}
                            >
                              <Trash2 class="h-4 w-4" />
                            </button>
                          {/if}
                        </td>
                      </tr>
                    {:else}
                      <tr>
                        <td colspan="6" class="py-8 px-2 text-center text-slate-500">
                          {#if mibLoading}
                            {$_("mib.loading")}
                          {:else}
                            {$_("mib.noModules")}
                          {/if}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
                <ListPagination
                  bind:pageSize={mibPageSize}
                  bind:currentPage={mibCurrentPage}
                  totalCount={sortedMibModules.length}
                />
              </div>
            </div>
          {/if}

          <!-- Icon Management settings -->
          {#if activeTab === "icons"}
            <div class="space-y-6">
              <!-- Header info card -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                      <Sparkles class="h-4 w-4 text-cyan-500" />
                      {$_('config.iconTitle')}
                    </h3>
                    <p class="mt-1 text-xs text-slate-500 dark:text-slate-400">
                      {$_('config.iconDesc')}
                    </p>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      type="button"
                      onclick={openAddIconDialog}
                      class="flex items-center gap-1.5 rounded-xl bg-cyan-600 hover:bg-cyan-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition-colors cursor-pointer"
                    >
                      <Plus class="h-3.5 w-3.5" />
                      {$_('config.iconAdd')}
                    </button>
                    <label class="flex items-center gap-1.5 rounded-xl bg-teal-600 hover:bg-teal-500 px-3 py-1.5 text-xs font-semibold text-white shadow-sm cursor-pointer transition-colors">
                      <Upload class="h-3.5 w-3.5" />
                      {$_('config.iconImport')}
                      <input
                        type="file"
                        bind:this={iconFileInput}
                        accept=".json,.csv,.txt"
                        onchange={handleImportIcons}
                        class="hidden"
                      />
                    </label>
                    <button
                      type="button"
                      onclick={handleExportIcons}
                      disabled={customIcons.length === 0}
                      class="flex items-center gap-1.5 rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors shadow-sm disabled:opacity-50 cursor-pointer"
                    >
                      <Download class="h-3.5 w-3.5 text-blue-500" />
                      {$_('config.iconExport')}
                    </button>
                  </div>
                </div>

                <!-- Search and Stats Toolbar -->
                <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-100 dark:border-slate-800/80">
                  <div class="relative flex-1 min-w-[220px]">
                    <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
                    <input
                      type="text"
                      bind:value={iconFilter}
                      oninput={() => (iconCurrentPage = 1)}
                      placeholder={$_('config.iconSearch')}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div class="flex items-center gap-3">
                    <select
                      bind:value={iconPageSize}
                      class="rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 px-2.5 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    >
                      <option value={10}>10 件表示</option>
                      <option value={25}>25 件表示</option>
                      <option value={50}>50 件表示</option>
                      <option value={100}>100 件表示</option>
                      <option value={-1}>すべて表示</option>
                    </select>
                    <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                      {$_('config.iconTotalCount')}: <span class="font-bold text-slate-800 dark:text-slate-200">{filteredCustomIcons.length}</span> / {customIcons.length}
                    </div>
                  </div>
                </div>
              </div>

              <!-- Icon Table -->
              <div class="overflow-x-auto rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm dark:shadow-lg">
                <table class="w-full min-w-[600px] whitespace-nowrap text-left text-xs">
                  <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="p-3 w-16 text-center">{$_('config.iconColIcon')}</th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("name")}>
                        <div class="flex items-center gap-1.5">
                          <span>{$_('config.iconColName')}</span>
                          {#if iconSortColumn === "name"}
                            {#if iconSortDirection === "asc"}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("type")}>
                        <div class="flex items-center gap-1.5">
                          <span>{$_('config.iconColType')}</span>
                          {#if iconSortColumn === "type"}
                            {#if iconSortDirection === "asc"}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 cursor-pointer select-none hover:text-slate-800 dark:hover:text-slate-200" onclick={() => handleIconSort("code")}>
                        <div class="flex items-center gap-1.5">
                          <span>{$_('config.iconColCode')}</span>
                          {#if iconSortColumn === "code"}
                            {#if iconSortDirection === "asc"}
                              <ArrowUp class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {:else}
                              <ArrowDown class="h-2.5 w-2.5 text-cyan-600 dark:text-cyan-400" />
                            {/if}
                          {/if}
                        </div>
                      </th>
                      <th class="p-3 text-center w-28">{$_('config.iconColAction')}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
                    {#each paginatedCustomIcons as ic (ic.id || ic.ID || ic.name || ic.Name)}
                      {@const codeVal = Number(ic.code ?? ic.Code ?? 0)}
                      {@const nameVal = ic.name || ic.Name || ""}
                      {@const typeVal = ic.type || ic.Type || (ic.image || ic.Image ? "image" : "mdi")}
                      {@const isImg = typeVal === "image" || Boolean(ic.image || ic.Image)}
                      {@const imgVal = ic.image || ic.Image || ""}
                      <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                        <td class="px-2 py-1 text-center">
                          {#if isImg}
                            <div class="w-8 h-8 mx-auto rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 p-0.5 flex items-center justify-center overflow-hidden">
                              <img src={imgVal} class="max-w-full max-h-full object-contain" alt={nameVal} />
                            </div>
                          {:else}
                            <span class="text-2xl text-cyan-500 dark:text-cyan-400 inline-block align-middle" style="font-family: 'Material Design Icons'">
                              {formatCodePreview(codeVal)}
                            </span>
                          {/if}
                        </td>
                        <td class="px-2 py-1 font-semibold text-slate-800 dark:text-slate-100 font-sans">
                          {nameVal}
                        </td>
                        <td class="px-2 py-1 font-sans">
                          {#if isImg}
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20">
                              {$_('config.iconTypeImage')}
                            </span>
                          {:else}
                            <span class="inline-flex items-center px-2 py-0.5 rounded text-[10px] font-semibold bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
                              {$_('config.iconTypeMdi')}
                            </span>
                          {/if}
                        </td>
                        <td class="px-2 py-1 text-slate-600 dark:text-slate-300">
                          {#if isImg}
                            <span class="text-[11px] text-slate-400 font-sans">{$_('config.iconTypeImage')}</span>
                          {:else}
                            <span>{codeVal}</span>
                            <span class="ml-2 text-[11px] text-slate-400">(0x{codeVal.toString(16).toUpperCase()})</span>
                          {/if}
                        </td>
                        <td class="px-1 py-0 text-center">
                          <div class="flex items-center justify-center font-sans">
                            <button
                              type="button"
                              onclick={() => openEditIconDialog(ic)}
                              class="rounded-lg p-1.5 text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-950/40 hover:text-amber-700 dark:hover:text-amber-300 transition-colors cursor-pointer"
                              title={$_('config.iconEdit')}
                            >
                              <Edit3 class="h-4 w-4" />
                            </button>
                            <button
                              type="button"
                              onclick={() => handleDeleteIcon(ic)}
                              class="rounded-lg p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-950/40 hover:text-rose-700 dark:hover:text-rose-300 transition-colors cursor-pointer"
                              title={$_('config.iconDelete')}
                            >
                              <Trash2 class="h-4 w-4" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    {:else}
                      <tr>
                        <td colspan="5" class="py-10 px-4 text-center text-slate-400 font-sans">
                          {#if iconLoading}
                            {$_('common.loading')}
                          {:else}
                            {$_('config.iconNoData')}
                          {/if}
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
                <ListPagination
                  bind:pageSize={iconPageSize}
                  bind:currentPage={iconCurrentPage}
                  totalCount={sortedCustomIcons.length}
                />
              </div>
            </div>
          {/if}

          <!-- Users Management tab -->
          {#if activeTab === "users"}
            <div class="space-y-6 max-w-4xl">
              <!-- Users Header Card -->
              <div class="rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 p-5 shadow-sm dark:shadow-lg space-y-4">
                <div class="flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 dark:border-slate-800 pb-4">
                  <div class="space-y-1">
                    <h3 class="text-sm font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                      <Users class="w-4 h-4 text-cyan-500 dark:text-cyan-400" />
                      {$_('users.title') || 'ユーザー管理'}
                    </h3>
                    <p class="text-[11px] text-slate-500 dark:text-slate-400">
                      {$_('users.subtitle') || 'TWSNMP NEO へのアクセス権限を持つユーザーアカウントの管理'}
                    </p>
                  </div>
                  {#if !currentUser || currentUser.role === 'admin'}
                    <button
                      type="button"
                      onclick={openAddUser}
                      class="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-cyan-600 to-blue-600 px-3.5 py-2 text-xs font-bold text-white shadow-md shadow-cyan-600/30 hover:from-cyan-500 hover:to-blue-500 transition-all cursor-pointer"
                    >
                      <UserPlus class="w-3.5 h-3.5" />
                      <span>{$_('users.addUser') || 'ユーザー追加'}</span>
                    </button>
                  {/if}
                </div>

                {#if currentUser && currentUser.role !== 'admin'}
                  <div class="flex items-center gap-2 rounded-xl border border-amber-800/40 bg-amber-950/40 p-3 text-xs font-medium text-amber-300">
                    <ShieldAlert class="h-4 w-4 text-amber-400 shrink-0" />
                    <span>{$_('users.adminOnlyNotice') || 'ユーザーの追加・削除および権限変更は管理者のみ実行可能です。'}</span>
                  </div>
                {/if}

                {#if userOpMessage}
                  <div class="flex items-center gap-2 rounded-xl border border-emerald-800/40 bg-emerald-950/40 p-3 text-xs font-medium text-emerald-300">
                    <CheckCircle2 class="h-4 w-4 text-emerald-400 shrink-0" />
                    <span>{userOpMessage}</span>
                  </div>
                {/if}

                {#if saveError}
                  <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3 text-xs font-medium text-rose-300">
                    <AlertTriangle class="h-4 w-4 text-rose-400 shrink-0" />
                    <span>{saveError}</span>
                  </div>
                {/if}

                <!-- Search toolbar -->
                <div class="flex items-center justify-between gap-3 pt-1">
                  <div class="relative flex-1 max-w-sm">
                    <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
                    <input
                      type="text"
                      bind:value={userSearch}
                      placeholder={$_('users.searchPlaceholder') || 'ユーザー名または表示名で検索...'}
                      class="w-full rounded-xl border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 pl-8 pr-3 py-1.5 text-xs text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none"
                    />
                  </div>
                  <div class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
                    {$_('users.totalUsers') || '登録数'}: <span class="font-bold text-slate-800 dark:text-slate-200">{usersList.length}</span>
                  </div>
                </div>
              </div>

              <!-- Users Table -->
              <div class="overflow-x-auto rounded-2xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900/80 shadow-sm dark:shadow-lg">
                <table class="w-full min-w-[550px] whitespace-nowrap text-left text-xs">
                  <thead class="bg-slate-100 dark:bg-slate-950 text-slate-600 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
                    <tr>
                      <th class="p-3">{$_('users.colUsername') || 'ユーザー名'}</th>
                      <th class="p-3">{$_('users.colDisplayName') || '表示名'}</th>
                      <th class="p-3">{$_('users.colRole') || '権限'}</th>
                      <th class="p-3">{$_('users.colCreatedAt') || '作成日時'}</th>
                      <th class="p-3 text-center w-28">{$_('common.actions') || '操作'}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-100 dark:divide-slate-800/60 font-mono text-slate-700 dark:text-slate-300">
                    {#if usersLoading}
                      <tr>
                        <td colspan="5" class="p-8 text-center text-slate-400">
                          {$_('common.loading')}
                        </td>
                      </tr>
                    {:else}
                      {@const filteredUsers = usersList.filter(u => {
                        const q = userSearch.toLowerCase();
                        return !q || u.user.toLowerCase().includes(q) || u.name.toLowerCase().includes(q);
                      })}
                      {#if filteredUsers.length === 0}
                        <tr>
                          <td colspan="5" class="p-8 text-center text-slate-400">
                            {$_('users.noUsersFound') || '該当するユーザーはいません'}
                          </td>
                        </tr>
                      {:else}
                        {#each filteredUsers as u (u.user)}
                          <tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
                            <td class="p-3 font-semibold text-slate-900 dark:text-slate-100 flex items-center gap-2">
                              <UserCircle2 class="h-4 w-4 text-cyan-600 dark:text-cyan-400 shrink-0" />
                              <span>{u.user}</span>
                            </td>
                            <td class="p-3">{u.name || '-'}</td>
                            <td class="p-3">
                              {#if u.role === 'admin'}
                                <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase bg-emerald-50 text-emerald-700 border border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-800/60">
                                  <span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
                                  Administrator
                                </span>
                              {:else if u.role === 'readonly'}
                                <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase bg-slate-100 text-slate-700 border border-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700">
                                  <span class="h-1.5 w-1.5 rounded-full bg-slate-500"></span>
                                  Read-Only
                                </span>
                              {:else}
                                <span class="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase bg-blue-50 text-blue-700 border border-blue-200 dark:bg-blue-950/40 dark:text-blue-300 dark:border-blue-800/60">
                                  <span class="h-1.5 w-1.5 rounded-full bg-blue-500"></span>
                                  Operator
                                </span>
                              {/if}
                            </td>
                            <td class="p-3 text-[11px] text-slate-500 dark:text-slate-400">
                              {u.created_at ? new Date(u.created_at * 1000).toLocaleString() : '-'}
                            </td>
                            <td class="p-3 text-center">
                              <div class="flex items-center justify-center gap-1.5">
                                {#if !currentUser || currentUser.role === 'admin' || currentUser.user === u.user}
                                  <button
                                    type="button"
                                    onclick={() => openEditUser(u)}
                                    title={$_('users.editUser') || '編集 / パスワード変更'}
                                    class="p-1.5 rounded-lg border border-slate-200 hover:border-slate-300 bg-slate-50 hover:bg-slate-100 text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-colors cursor-pointer"
                                  >
                                    <KeyRound class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
                                  </button>
                                {/if}
                                {#if !currentUser || currentUser.role === 'admin'}
                                  <button
                                    type="button"
                                    onclick={() => handleDeleteUser(u)}
                                    disabled={usersList.length <= 1}
                                    title={$_('common.delete')}
                                    class="p-1.5 rounded-lg border border-rose-200 hover:border-rose-300 bg-rose-50 hover:bg-rose-100 text-rose-600 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-400 dark:hover:bg-rose-900/60 transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
                                  >
                                    <Trash2 class="h-3.5 w-3.5" />
                                  </button>
                                {/if}
                              </div>
                            </td>
                          </tr>
                        {/each}
                      {/if}
                    {/if}
                  </tbody>
                </table>
              </div>
            </div>
          {/if}
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-slate-200 dark:border-slate-800/80 bg-slate-50 dark:bg-slate-900/60 px-6 py-3.5">
        {#if activeTab === 'mib' || activeTab === 'icons' || activeTab === 'users'}
          <div></div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              onclick={() => (show = false)}
              class="px-5 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
            >
              {$_('common.close')}
            </button>
          </div>
        {:else}
          <div class="text-[11px] text-slate-500 dark:text-slate-400">
            {$_('config.saveHint')}
          </div>
          <div class="flex items-center gap-3">
            <button
              type="button"
              onclick={() => (show = false)}
              class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
            >
              {$_('common.cancel')}
            </button>
            <button
              type="button"
              onclick={handleSave}
              class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-2 transition-all cursor-pointer"
            >
              <Save class="w-4 h-4" />
              {$_('common.save')}
            </button>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}


<ImportMapModal bind:show={showImportModal} onImported={() => { onSaved?.(); loadConfig(); }} />

<!-- Sub-Modal: MIB Tree Viewer -->
{#if showMIBTreeModal}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={$_('config.mibTree')}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showMIBTreeModal = false)}
  >
    <div class="flex max-h-[85vh] w-full max-w-2xl flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3 dark:border-slate-800">
        <div class="flex items-center gap-2">
          <FolderTree class="h-4 w-4 text-teal-500" />
          <h3 class="text-xs font-bold">{$_('mib.treeStructure')}</h3>
        </div>
        <button
          type="button"
          onclick={() => (showMIBTreeModal = false)}
          class="rounded-lg p-1 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
        >
          <X class="h-4 w-4" />
        </button>
      </header>
      <div class="min-h-0 flex-1 overflow-hidden p-4">
        <MIBTree
          treeData={mibTreeData}
          heightClass="h-[60vh]"
        />
      </div>
    </div>
  </div>
{/if}

<!-- Sub-Modal: Add / Edit Custom Icon -->
{#if showIconDialog}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={editingIcon ? $_('config.iconEditTitle') : $_('config.iconAddTitle')}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showIconDialog = false)}
  >
    <div class="flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-slate-700 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-6 py-4 dark:border-slate-800">
        <div class="flex items-center gap-2.5">
          <div class="flex h-8 w-8 items-center justify-center rounded-xl bg-cyan-500/20 text-cyan-400">
            <Sparkles class="h-4 w-4" />
          </div>
          <h3 class="text-sm font-bold">{editingIcon ? $_('config.iconEditTitle') : $_('config.iconAddTitle')}</h3>
        </div>
        <button
          type="button"
          onclick={() => (showIconDialog = false)}
          class="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <div class="p-6 space-y-4 overflow-y-auto text-xs">
        {#if dialogIconError}
          <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3 text-xs font-medium text-rose-300">
            <X class="h-4 w-4 text-rose-400 shrink-0" />
            <span>{dialogIconError}</span>
          </div>
        {/if}

        <!-- Icon Type Selector Tab -->
        {#if !editingIcon}
          <div>
            <span class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconType')}
            </span>
            <div class="flex rounded-xl bg-slate-100 dark:bg-slate-950 p-1 border border-slate-200 dark:border-slate-800">
              <button
                type="button"
                onclick={() => { dialogIconType = "mdi"; }}
                class="flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all cursor-pointer {dialogIconType === 'mdi' ? 'bg-cyan-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                {$_('config.iconTypeMdi')}
              </button>
              <button
                type="button"
                onclick={() => { dialogIconType = "image"; }}
                class="flex-1 py-1.5 text-xs font-semibold rounded-lg transition-all cursor-pointer {dialogIconType === 'image' ? 'bg-purple-600 text-white shadow-xs' : 'text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200'}"
              >
                {$_('config.iconTypeImage')}
              </button>
            </div>
          </div>
        {/if}

        {#if dialogIconType === "mdi"}
          {#if !editingIcon}
            <!-- Icon Filter Keyword -->
            <div>
              <label for="dialog-icon-filter" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                {$_('config.iconFilterKeyword')}
              </label>
              <div class="relative">
                <Search class="absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
                <input
                  id="dialog-icon-filter"
                  type="text"
                  bind:value={dialogMdiFilter}
                  placeholder={$_('config.iconFilterPlaceholder')}
                  class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 pl-8 pr-3 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
                />
              </div>
            </div>

            <!-- MDI Icon Selection Dropdown -->
            <div>
              <div class="flex items-center justify-between mb-1.5">
                <label for="dialog-icon-select" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                  {$_('config.iconSelect')} <span class="text-rose-500">*</span>
                </label>
                <span class="text-[11px] text-slate-400 font-mono">
                  {$_('config.iconMatchedCount', { values: { count: filteredMdiList.length } })}
                </span>
              </div>
              <select
                id="dialog-icon-select"
                value={dialogSelectedMdi}
                onchange={(e) => handleSelectMdi((e.target as HTMLSelectElement).value)}
                class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none font-mono"
              >
                {#if filteredMdiList.length === 0}
                  <option value="" disabled>{$_('config.iconNoMatch')}</option>
                {:else}
                  {#each filteredMdiList as mdi}
                    <option value={mdi.name}>
                      {mdi.name} (0x{mdi.code.toString(16).toUpperCase()})
                    </option>
                  {/each}
                {/if}
              </select>
            </div>
          {/if}

          <!-- Custom Icon Name -->
          <div>
            <label for="dialog-icon-name" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconName')} <span class="text-rose-500">*</span>
            </label>
            <input
              id="dialog-icon-name"
              type="text"
              bind:value={dialogIconName}
              placeholder={$_('config.iconNamePlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <!-- Live Glyph Preview -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 flex items-center gap-4">
            <div class="w-16 h-16 rounded-xl bg-cyan-500/10 border border-cyan-500/30 flex items-center justify-center text-4xl text-cyan-500 dark:text-cyan-400 shrink-0 shadow-inner">
              <span style="font-family: 'Material Design Icons'">{formatCodePreview(dialogParsedCode)}</span>
            </div>
            <div class="space-y-1 min-w-0">
              <div class="text-xs font-bold text-slate-800 dark:text-slate-200">
                {dialogSelectedMdi || dialogIconName || $_('config.iconPreview')}
              </div>
              <div class="text-[11px] font-mono text-slate-500">10進数: <span class="text-cyan-600 dark:text-cyan-400 font-bold">{dialogParsedCode || 0}</span></div>
              <div class="text-[11px] font-mono text-slate-500">16進数: <span class="text-emerald-600 dark:text-emerald-400 font-bold">0x{(dialogParsedCode || 0).toString(16).toUpperCase()}</span></div>
            </div>
          </div>
        {:else}
          <!-- Image Icon Mode -->
          <div>
            <span class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconSelectImageFile')} <span class="text-rose-500">*</span>
            </span>
            <div class="flex items-center gap-3">
              <input
                type="file"
                bind:this={imageFileInput}
                accept="image/*"
                onchange={handleImageFileSelected}
                class="hidden"
              />
              <button
                type="button"
                onclick={() => imageFileInput?.click()}
                class="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-xl text-xs font-semibold transition-colors cursor-pointer"
              >
                <FileUp class="h-4 w-4" />
                {$_('config.iconSelectImageFile')}
              </button>
              <span class="text-[11px] text-slate-500 dark:text-slate-400">
                PNG, JPG, SVG, WebP, GIF (Max 2MB)
              </span>
            </div>
          </div>

          <!-- Custom Icon Name -->
          <div>
            <label for="dialog-icon-name-img" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
              {$_('config.iconName')} <span class="text-rose-500">*</span>
            </label>
            <input
              id="dialog-icon-name-img"
              type="text"
              bind:value={dialogIconName}
              placeholder={$_('config.iconNamePlaceholder')}
              class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
            />
          </div>

          <!-- Live Image Preview -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/60 p-4 flex items-center gap-4">
            <div class="w-16 h-16 rounded-xl bg-purple-500/10 border border-purple-500/30 flex items-center justify-center shrink-0 shadow-inner overflow-hidden p-1">
              {#if dialogIconImage}
                <img src={dialogIconImage} class="max-w-full max-h-full object-contain" alt="Preview" />
              {:else}
                <Image class="h-8 w-8 text-slate-400" />
              {/if}
            </div>
            <div class="space-y-1 min-w-0">
              <div class="text-xs font-bold text-slate-800 dark:text-slate-200 truncate">
                {dialogIconName || $_('config.iconPreview')}
              </div>
              <div class="text-[11px] text-slate-500">
                {dialogIconImage ? $_('config.iconTypeImage') : $_('config.iconDropImageHint')}
              </div>
            </div>
          </div>
        {/if}
      </div>

      <footer class="flex items-center justify-end gap-3 border-t border-slate-200 px-6 py-3.5 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60">
        <button
          type="button"
          onclick={() => (showIconDialog = false)}
          class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-800 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={handleSaveIcon}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-emerald-600 hover:from-cyan-500 hover:to-emerald-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-1.5 transition-all cursor-pointer"
        >
          <Save class="w-4 h-4" />
          {$_('common.save')}
        </button>
      </footer>
    </div>
  </div>
{/if}

<!-- Sub-Modal: Add / Edit User -->
{#if showUserModal}
  <div
    class="fixed inset-0 z-[70] flex items-center justify-center bg-black/80 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label={isEditingUser ? $_('users.editUser') || 'ユーザー編集' : $_('users.addUser') || 'ユーザー追加'}
    tabindex="-1"
    onkeydown={(e) => e.key === "Escape" && (showUserModal = false)}
  >
    <div class="flex max-h-[90vh] w-full max-w-md flex-col overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800 bg-white text-slate-800 shadow-2xl dark:bg-[#0b1329] dark:text-slate-100">
      <header class="flex shrink-0 items-center justify-between border-b border-slate-200 px-5 py-3.5 dark:border-slate-800">
        <div class="flex items-center gap-2">
          {#if isEditingUser}
            <KeyRound class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <h3 class="text-sm font-bold">{$_('users.editUser') || 'ユーザー編集 / パスワード変更'}</h3>
          {:else}
            <UserPlus class="h-4 w-4 text-cyan-600 dark:text-cyan-400" />
            <h3 class="text-sm font-bold">{$_('users.addUser') || 'ユーザー追加'}</h3>
          {/if}
        </div>
        <button
          type="button"
          onclick={() => (showUserModal = false)}
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
        >
          <X class="h-4 w-4" />
        </button>
      </header>

      <div class="flex-1 overflow-y-auto p-5 space-y-4 text-xs">
        {#if userDialogError}
          <div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300 font-medium">
            {userDialogError}
          </div>
        {/if}

        <!-- Username -->
        <div>
          <label for="dialog-user-name" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_('users.colUsername') || 'ユーザー名'} <span class="text-rose-500">*</span>
          </label>
          <input
            id="dialog-user-name"
            type="text"
            bind:value={dialogUsername}
            disabled={isEditingUser}
            placeholder="operator1"
            autocomplete="off"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none font-mono disabled:opacity-60"
          />
        </div>

        <!-- Display Name -->
        <div>
          <label for="dialog-user-display" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_('users.colDisplayName') || '表示名'}
          </label>
          <input
            id="dialog-user-display"
            type="text"
            bind:value={dialogDisplayName}
            placeholder="Network Operator"
            autocomplete="off"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none"
          />
        </div>

        <!-- Role -->
        <div>
          <label for="dialog-user-role" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_('users.colRole') || '権限ロール'}
          </label>
          <select
            id="dialog-user-role"
            bind:value={dialogRole}
            disabled={currentUser && currentUser.role !== 'admin'}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none disabled:opacity-60 disabled:cursor-not-allowed"
          >
            <option value="user">{$_('users.roleUser') || 'Operator (一般運用者)'}</option>
            <option value="admin">{$_('users.roleAdmin') || 'Administrator (管理者)'}</option>
            <option value="readonly">{$_('users.roleReadonly') || 'Read-Only (閲覧のみ)'}</option>
          </select>
        </div>

        <!-- Password -->
        <div>
          <label for="dialog-user-password" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_('users.password') || 'パスワード'}
            {#if !isEditingUser}<span class="text-rose-500">*</span>{/if}
          </label>
          <input
            id="dialog-user-password"
            type="password"
            bind:value={dialogPassword}
            autocomplete="new-password"
            placeholder={isEditingUser ? $_('users.passwordKeepCurrentPlaceholder') || '変更する場合のみ入力' : '••••••••'}
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none font-mono"
          />
        </div>

        <!-- Password Confirm -->
        <div>
          <label for="dialog-user-password-confirm" class="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
            {$_('users.confirmPassword') || 'パスワード (確認)'}
          </label>
          <input
            id="dialog-user-password-confirm"
            type="password"
            bind:value={dialogPasswordConfirm}
            autocomplete="new-password"
            placeholder="••••••••"
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs text-slate-900 dark:text-slate-100 focus:border-cyan-500 focus:outline-none font-mono"
          />
        </div>
      </div>

      <footer class="flex items-center justify-end gap-3 border-t border-slate-200 px-5 py-3.5 dark:border-slate-800 bg-slate-50 dark:bg-slate-900/60">
        <button
          type="button"
          onclick={() => (showUserModal = false)}
          disabled={userDialogLoading}
          class="px-4 py-2 bg-white dark:bg-slate-900 hover:bg-slate-100 dark:hover:bg-slate-800 border border-slate-300 dark:border-slate-800 rounded-xl text-xs font-medium text-slate-700 dark:text-slate-300 transition-colors cursor-pointer"
        >
          {$_('common.cancel')}
        </button>
        <button
          type="button"
          onclick={handleSaveUser}
          disabled={userDialogLoading}
          class="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white rounded-xl text-xs font-bold shadow-lg shadow-cyan-600/30 flex items-center gap-1.5 transition-all cursor-pointer disabled:opacity-50"
        >
          <Save class="w-4 h-4" />
          <span>{$_('common.save')}</span>
        </button>
      </footer>
    </div>
  </div>
{/if}

<ModelManagerDialog
  bind:show={showModelManager}
  onmodelsChanged={loadLocalAIInfo}
  onclose={loadLocalAIInfo}
/>




