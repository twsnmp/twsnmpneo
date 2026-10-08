<script lang="ts">
  import {
    fetchUsers,
    createUser,
    updateUser,
    deleteUser,
    type UserEnt,
  } from "../../api";
  import { showConfirm } from "../../stores/modalStore";
  import {
    Users,
    UserPlus,
    UserCircle2,
    ShieldAlert,
    KeyRound,
    Trash2,
    Search,
    AlertTriangle,
    CheckCircle2,
    X,
    Save,
  } from "@lucide/svelte";
  import { _ } from "svelte-i18n";

  let {
    currentUser = null,
    onError,
  }: {
    currentUser?: UserEnt | null;
    onError?: (msg: string) => void;
  } = $props();

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
  let userError = $state("");

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

  export async function loadUsers() {
    usersLoading = true;
    userOpMessage = "";
    userError = "";
    try {
      usersList = await fetchUsers();
    } catch (e: any) {
      userError = formatUserError(e.message || "");
      onError?.(userError);
    } finally {
      usersLoading = false;
    }
  }

  const openAddUser = () => {
    if (currentUser && currentUser.role !== 'admin') {
      userOpMessage = "";
      userError = $_('users.errAdminRequiredCreate') || "ユーザーの追加には管理者権限が必要です";
      onError?.(userError);
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
      userError = formatUserError(e.message || "");
      onError?.(userError);
    } finally {
      usersLoading = false;
    }
  };

  $effect(() => {
    loadUsers();
  });
</script>

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

    {#if userError}
      <div class="flex items-center gap-2 rounded-xl border border-rose-800/40 bg-rose-950/40 p-3 text-xs font-medium text-rose-300">
        <AlertTriangle class="h-4 w-4 text-rose-400 shrink-0" />
        <span>{userError}</span>
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
          class="rounded-lg p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors cursor-pointer"
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
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none disabled:opacity-60 disabled:cursor-not-allowed"
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
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none font-mono"
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
            class="w-full rounded-xl border border-slate-300 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-800 dark:text-slate-200 focus:border-cyan-500 focus:outline-none font-mono"
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
