<script lang="ts">
  import { onMount } from 'svelte';
  import { _ } from 'svelte-i18n';
  import { login, type UserEnt } from '../api';
  import { switchLocale, getSavedLocale, type SupportedLocale } from '../i18n';
  import logoUrl from '../../assets/logo.png';
  import {
    User,
    Lock,
    Eye,
    EyeOff,
    LogIn,
    Loader2,
    Sun,
    Moon,
    Languages,
    AlertCircle,
    Shield,
  } from '@lucide/svelte';

  interface Props {
    onLoginSuccess: (user: UserEnt) => void;
    isDark?: boolean;
    onToggleTheme?: () => void;
  }

  let { onLoginSuccess, isDark = true, onToggleTheme }: Props = $props();

  let user = $state('');
  let password = $state('');
  let showPassword = $state(false);
  let loading = $state(false);
  let errorMessage = $state('');
  let currentLocale = $state<SupportedLocale>(getSavedLocale());
  let animated = $state(false);

  onMount(() => {
    // Trigger welcome animation
    setTimeout(() => {
      animated = true;
    }, 50);
  });

  const toggleLocale = () => {
    const next: SupportedLocale = currentLocale === 'ja' ? 'en' : 'ja';
    currentLocale = next;
    switchLocale(next);
  };

  const handleLogin = async (e: Event) => {
    e.preventDefault();
    if (!user.trim() || !password) {
      errorMessage = $_('auth.emptyCredentials') || 'ユーザー名とパスワードを入力してください';
      return;
    }

    loading = true;
    errorMessage = '';

    try {
      const res = await login(user.trim(), password);
      onLoginSuccess(res.user);
    } catch (err: any) {
      errorMessage = err.message || $_('auth.loginFailed') || 'ログインに失敗しました';
    } finally {
      loading = false;
    }
  };
</script>

<div class="relative flex min-h-screen w-screen flex-col items-center justify-center overflow-hidden bg-slate-100 text-slate-800 dark:bg-[#070d1e] dark:text-slate-100 font-sans transition-colors duration-300 select-none">
  <!-- Dynamic Background Glow Effects -->
  <div class="pointer-events-none absolute -top-40 -left-40 h-96 w-96 rounded-full bg-cyan-500/15 dark:bg-cyan-500/10 blur-3xl"></div>
  <div class="pointer-events-none absolute -bottom-40 -right-40 h-96 w-96 rounded-full bg-blue-600/15 dark:bg-blue-600/10 blur-3xl"></div>
  <div class="pointer-events-none absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 h-[500px] w-[500px] rounded-full bg-indigo-500/10 dark:bg-cyan-600/5 blur-[100px]"></div>

  <!-- Top Action Controls (Language & Theme) -->
  <div class="absolute top-4 right-4 z-20 flex items-center gap-2">
    <!-- Language Switcher -->
    <button
      onclick={toggleLocale}
      type="button"
      title={$_('nav.language')}
      class="flex h-9 items-center gap-1.5 px-3 rounded-xl border border-slate-200 bg-white/80 backdrop-blur-md text-slate-700 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-200 dark:hover:border-slate-700 dark:hover:text-white transition-all text-xs font-semibold shadow-sm"
    >
      <Languages class="h-3.5 w-3.5 text-cyan-600 dark:text-cyan-400" />
      <span>{currentLocale.toUpperCase()}</span>
    </button>

    <!-- Theme Toggle -->
    {#if onToggleTheme}
      <button
        onclick={onToggleTheme}
        type="button"
        title={$_('nav.theme')}
        class="flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white/80 backdrop-blur-md text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-slate-800 dark:bg-slate-900/80 dark:text-slate-300 dark:hover:border-slate-700 dark:hover:text-white transition-all shadow-sm"
      >
        {#if isDark}
          <Sun class="h-4 w-4 text-amber-400" />
        {:else}
          <Moon class="h-4 w-4 text-cyan-600" />
        {/if}
      </button>
    {/if}
  </div>

  <!-- Main Login Container with Welcome Animation -->
  <div class="relative z-10 flex w-full max-w-md flex-col items-center px-4">
    <!-- Cat Logo Welcome Section with Spring & Glow Animation -->
    <div class="relative flex flex-col items-center mb-6">
      <!-- Glow Ring around Cat -->
      <div
        class="absolute -inset-2 rounded-3xl bg-gradient-to-tr from-cyan-500 to-blue-600 opacity-30 blur-lg transition-all duration-1000 {animated ? 'scale-100 opacity-40 animate-pulse' : 'scale-75 opacity-0'}"
      ></div>

      <!-- Animated Cat Logo Card -->
      <div
        class="relative flex h-28 w-28 items-center justify-center rounded-2xl overflow-hidden border-2 border-white/60 dark:border-cyan-500/40 bg-gradient-to-b from-slate-900 to-slate-950 shadow-2xl shadow-cyan-500/30 transition-all duration-700 ease-out {animated ? 'scale-100 translate-y-0 opacity-100 animate-cat-bounce' : 'scale-50 translate-y-8 opacity-0'}"
      >
        <img
          src={logoUrl}
          alt="TWSNMP NEO Cat Logo"
          class="h-full w-full object-contain p-2 filter drop-shadow-[0_4px_12px_rgba(6,182,212,0.4)]"
        />
      </div>

      <!-- Welcome Title & Subtitle -->
      <div class="mt-4 text-center transition-all duration-700 delay-200 ease-out {animated ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-4'}">
        <h1 class="text-2xl font-black tracking-tight text-slate-900 dark:text-white flex items-center justify-center gap-2">
          <span>{$_('app.title')}</span>
          <span class="text-xs px-2 py-0.5 rounded-full font-semibold uppercase bg-cyan-100 text-cyan-800 dark:bg-cyan-950/80 dark:text-cyan-300 border border-cyan-300 dark:border-cyan-700/60">NEO</span>
        </h1>
        <p class="mt-1 text-xs font-medium text-slate-500 dark:text-slate-400">
          {$_('auth.welcomeSubtitle') || 'ネットワーク監視・運用管理システム'}
        </p>
      </div>
    </div>

    <!-- Login Card Form -->
    <div
      class="w-full rounded-2xl border border-slate-200/80 bg-white/90 p-6 shadow-xl backdrop-blur-xl dark:border-slate-800/80 dark:bg-slate-900/90 dark:shadow-2xl transition-all duration-700 delay-300 ease-out {animated ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-6'}"
    >
      <form onsubmit={handleLogin} class="space-y-4">
        <!-- Error Alert -->
        {#if errorMessage}
          <div
            class="flex items-center gap-2.5 rounded-xl border border-rose-200 bg-rose-50/90 p-3 text-xs font-medium text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300 animate-shake"
            role="alert"
          >
            <AlertCircle class="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400" />
            <span>{errorMessage}</span>
          </div>
        {/if}

        <!-- Username Field -->
        <div class="space-y-1.5">
          <label for="login-username" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
            {$_('auth.username') || 'ユーザー名'}
          </label>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
              <User class="h-4 w-4 text-slate-400 dark:text-slate-500" />
            </div>
            <input
              id="login-username"
              type="text"
              bind:value={user}
              placeholder={$_('auth.usernamePlaceholder') || 'twsnmp'}
              autocomplete="username"
              required
              disabled={loading}
              class="w-full rounded-xl border border-slate-300 bg-slate-50/70 pl-9 pr-3.5 py-2.5 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-cyan-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400 dark:focus:bg-slate-800 transition-all font-mono"
            />
          </div>
        </div>

        <!-- Password Field -->
        <div class="space-y-1.5">
          <div class="flex items-center justify-between">
            <label for="login-password" class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
              {$_('auth.password') || 'パスワード'}
            </label>
          </div>
          <div class="relative">
            <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
              <Lock class="h-4 w-4 text-slate-400 dark:text-slate-500" />
            </div>
            {#if showPassword}
              <input
                id="login-password"
                type="text"
                bind:value={password}
                placeholder="••••••••"
                autocomplete="current-password"
                required
                disabled={loading}
                class="w-full rounded-xl border border-slate-300 bg-slate-50/70 pl-9 pr-10 py-2.5 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-cyan-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400 dark:focus:bg-slate-800 transition-all font-mono"
              />
            {:else}
              <input
                id="login-password"
                type="password"
                bind:value={password}
                placeholder="••••••••"
                autocomplete="current-password"
                required
                disabled={loading}
                class="w-full rounded-xl border border-slate-300 bg-slate-50/70 pl-9 pr-10 py-2.5 text-xs text-slate-900 placeholder:text-slate-400 focus:border-cyan-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-cyan-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:placeholder:text-slate-500 dark:focus:border-cyan-400 dark:focus:bg-slate-800 transition-all font-mono"
              />
            {/if}
            <button
              type="button"
              onclick={() => (showPassword = !showPassword)}
              class="absolute inset-y-0 right-0 flex items-center pr-3 text-slate-400 hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300"
              tabindex={-1}
              title={showPassword ? $_('auth.hidePassword') || 'パスワードを隠す' : $_('auth.showPassword') || 'パスワードを表示'}
            >
              {#if showPassword}
                <EyeOff class="h-4 w-4" />
              {:else}
                <Eye class="h-4 w-4" />
              {/if}
            </button>
          </div>
        </div>

        <!-- Submit Button -->
        <button
          type="submit"
          disabled={loading}
          class="relative mt-2 flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-cyan-600 to-blue-600 px-4 py-2.5 text-xs font-bold text-white shadow-lg shadow-cyan-600/30 hover:from-cyan-500 hover:to-blue-500 active:scale-[0.99] disabled:opacity-50 disabled:cursor-not-allowed transition-all"
        >
          {#if loading}
            <Loader2 class="h-4 w-4 animate-spin" />
            <span>{$_('auth.loggingIn') || 'ログイン中...'}</span>
          {:else}
            <LogIn class="h-4 w-4" />
            <span>{$_('auth.loginButton') || 'ログイン'}</span>
          {/if}
        </button>
      </form>

      <!-- Default User Hint -->
      <div class="mt-4 pt-3.5 border-t border-slate-200 dark:border-slate-800/80 text-center">
        <p class="text-[11px] text-slate-400 dark:text-slate-500 flex items-center justify-center gap-1">
          <Shield class="h-3 w-3 text-cyan-600 dark:text-cyan-400" />
          <span>{$_('auth.defaultHint') || '初期ユーザー: twsnmp / twsnmp'}</span>
        </p>
      </div>
    </div>
  </div>
</div>

<style>
  @keyframes catBounce {
    0% {
      transform: scale(0.6) translateY(24px);
      opacity: 0;
    }
    60% {
      transform: scale(1.08) translateY(-6px);
      opacity: 1;
    }
    80% {
      transform: scale(0.96) translateY(2px);
    }
    100% {
      transform: scale(1) translateY(0);
      opacity: 1;
    }
  }

  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    20%, 60% { transform: translateX(-4px); }
    40%, 80% { transform: translateX(4px); }
  }

  .animate-cat-bounce {
    animation: catBounce 0.8s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
  }

  .animate-shake {
    animation: shake 0.35s ease-in-out;
  }
</style>
