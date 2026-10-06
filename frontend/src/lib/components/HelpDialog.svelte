<script lang="ts">
  import { _ } from "svelte-i18n";
  import logoUrl from "../../assets/logo.png";
  import {
    X,
    ExternalLink,
    HelpCircle,
    BookOpen,
    MessageCircle,
    MessageSquareQuote,
    Users,
    Bug,
    Tag,
    Network,
    MapPin,
    Layers,
    BarChart3,
    ShieldCheck,
    Calendar,
    Activity,
    Radio,
    Info,
    CheckCircle2,
  } from "@lucide/svelte";

  let {
    show = $bindable(false),
    currentPage = "map",
  } = $props<{
    show: boolean;
    currentPage?: string;
  }>();

  let selectedScreen = $state<string>("map");

  $effect(() => {
    if (show) {
      selectedScreen = currentPage || "map";
    }
  });

  const screens = [
    { id: "map", icon: Network, label: "nav.map" },
    { id: "location", icon: MapPin, label: "nav.location" },
    { id: "list", icon: Layers, label: "nav.list" },
    { id: "reports", icon: BarChart3, label: "nav.reports" },
    { id: "pki", icon: ShieldCheck, label: "pki.title" },
    { id: "logs", icon: Calendar, label: "nav.logs" },
    { id: "otel", icon: Activity, label: "nav.otel" },
    { id: "mqtt", icon: Radio, label: "nav.mqtt" },
    { id: "system", icon: Info, label: "nav.system" },
  ];

  const officialLinks = [
    {
      id: "manual",
      titleKey: "help.links.manual",
      descKey: "help.links.manualDesc",
      url: "https://twsmpneo.github.io",
      icon: BookOpen,
      badgeColor: "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20",
      hoverColor: "hover:border-blue-400 dark:hover:border-blue-500",
    },
    {
      id: "note",
      titleKey: "help.links.note",
      descKey: "help.links.noteDesc",
      url: "https://note.com/twsnmp",
      icon: MessageCircle,
      badgeColor: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20",
      hoverColor: "hover:border-emerald-400 dark:hover:border-emerald-500",
    },
    {
      id: "noteQa",
      titleKey: "help.links.noteQa",
      descKey: "help.links.noteQaDesc",
      url: "https://note.com/qa/twsnmp",
      icon: MessageSquareQuote,
      badgeColor: "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20",
      hoverColor: "hover:border-amber-400 dark:hover:border-amber-500",
    },
    {
      id: "noteBoard",
      titleKey: "help.links.noteBoard",
      descKey: "help.links.noteBoardDesc",
      url: "https://note.com/twsnmp/membership/boards",
      icon: Users,
      badgeColor: "bg-teal-500/10 text-teal-600 dark:text-teal-400 border-teal-500/20",
      hoverColor: "hover:border-teal-400 dark:hover:border-teal-500",
    },
    {
      id: "issues",
      titleKey: "help.links.issues",
      descKey: "help.links.issuesDesc",
      url: "https://github.com/twsnmp/twsnmpneo/issues",
      icon: Bug,
      badgeColor: "bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20",
      hoverColor: "hover:border-rose-400 dark:hover:border-rose-500",
    },
    {
      id: "releases",
      titleKey: "help.links.releases",
      descKey: "help.links.releasesDesc",
      url: "https://github.com/twsnmp/twsnmpneo/releases",
      icon: Tag,
      badgeColor: "bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20",
      hoverColor: "hover:border-purple-400 dark:hover:border-purple-500",
    },
  ];

  const handleKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape" && show) {
      show = false;
    }
  };
</script>

<svelte:window onkeydown={handleKeydown} />

{#if show}
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-200">
    <!-- Backdrop click to close -->
    <button
      type="button"
      class="absolute inset-0 h-full w-full cursor-default bg-transparent border-none"
      onclick={() => (show = false)}
      aria-label={$_('common.close')}
    ></button>

    <!-- Modal Content: max-w-4xl to ensure all 9 tabs stay on a single row without wrapping -->
    <div class="relative flex flex-col max-h-[90vh] w-full max-w-4xl rounded-2xl border border-slate-200 bg-white shadow-2xl transition-all dark:border-slate-800 dark:bg-slate-900 overflow-hidden z-10">
      <!-- Header -->
      <div class="flex items-center justify-between border-b border-slate-200 px-6 py-4 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-600/10 text-cyan-600 dark:bg-cyan-500/20 dark:text-cyan-400 border border-cyan-500/20">
            <HelpCircle class="h-5 w-5" />
          </div>
          <div>
            <h2 class="text-base font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              {$_('help.title')}
              <span class="rounded-full bg-cyan-500/10 px-2 py-0.5 text-[11px] font-semibold text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
                TWSNMP NEO
              </span>
            </h2>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              {$_('help.subtitle')}
            </p>
          </div>
        </div>

        <button
          onclick={() => (show = false)}
          class="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors"
          aria-label={$_('common.close')}
        >
          <X class="h-5 w-5" />
        </button>
      </div>

      <!-- Body (Scrollable) -->
      <div class="flex-1 overflow-y-auto p-6 space-y-6">
        <!-- Detective Cat Mascot Banner -->
        <div class="relative overflow-hidden rounded-2xl bg-gradient-to-br from-cyan-900/10 via-slate-100 to-indigo-900/10 dark:from-cyan-950/40 dark:via-slate-900 dark:to-indigo-950/40 border border-cyan-500/20 p-4 sm:p-5 shadow-sm">
          <div class="flex flex-col sm:flex-row items-center sm:items-start gap-4">
            <!-- Cat Avatar with glow -->
            <div class="relative shrink-0 group">
              <div class="absolute -inset-1 rounded-2xl bg-gradient-to-r from-cyan-500 to-indigo-500 opacity-30 blur-sm group-hover:opacity-60 transition duration-300"></div>
              <div class="relative h-20 w-20 sm:h-24 sm:w-24 rounded-2xl overflow-hidden border-2 border-cyan-400/40 bg-slate-950 shadow-md">
                <img src={logoUrl} alt="TWSNMP Detective Cat" class="h-full w-full object-cover" />
              </div>
            </div>

            <!-- Speech Bubble Message -->
            <div class="flex-1 text-center sm:text-left space-y-2">
              <div class="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full bg-cyan-500/10 text-cyan-700 dark:text-cyan-300 text-xs font-semibold border border-cyan-500/20">
                <span>🐾 Detective Cat Guide</span>
              </div>
              <p class="text-sm font-medium text-slate-800 dark:text-slate-200 leading-relaxed">
                {$_('help.catGreeting')}
              </p>
              <div class="text-xs text-slate-600 dark:text-slate-400">
                {$_('help.contextHelp')} - <span class="font-semibold text-cyan-600 dark:text-cyan-400">{$_(`help.pages.${selectedScreen}.title`)}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Contextual Screen Guide Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
              {$_('help.contextHelp')}
            </h3>
            <span class="text-[11px] text-slate-400 dark:text-slate-500">
              {$_('help.selectScreen')}
            </span>
          </div>

          <!-- Screen Selector Pills: Single-row flex layout with no wrap -->
          <div class="flex items-center gap-1 p-1 bg-slate-100 dark:bg-slate-950/60 rounded-xl border border-slate-200/80 dark:border-slate-800 overflow-x-auto">
            {#each screens as scr}
              <button
                type="button"
                onclick={() => (selectedScreen = scr.id)}
                class="flex-1 min-w-0 flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-all {selectedScreen === scr.id ? 'bg-cyan-600 text-white shadow-sm font-semibold' : 'text-slate-600 hover:text-slate-900 hover:bg-slate-200/70 dark:text-slate-400 dark:hover:text-slate-200 dark:hover:bg-slate-800/60'}"
              >
                <scr.icon class="h-3.5 w-3.5 shrink-0" />
                <span class="truncate">{$_(scr.label)}</span>
              </button>
            {/each}
          </div>

          <!-- Active Screen Content Card -->
          <div class="rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-950/40 p-4 space-y-3">
            <div class="flex items-center gap-2">
              <h4 class="text-sm font-bold text-slate-900 dark:text-slate-100">
                {$_(`help.pages.${selectedScreen}.title`)}
              </h4>
            </div>
            <p class="text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
              {$_(`help.pages.${selectedScreen}.summary`)}
            </p>

            <div class="pt-1 space-y-2 border-t border-slate-200/60 dark:border-slate-800/80">
              {#each ($_(`help.pages.${selectedScreen}.tips`) as string[]) || [] as tip}
                <div class="flex items-start gap-2 text-xs text-slate-700 dark:text-slate-300">
                  <CheckCircle2 class="h-4 w-4 shrink-0 text-cyan-600 dark:text-cyan-400 mt-0.5" />
                  <span class="leading-tight">{tip}</span>
                </div>
              {/each}
            </div>
          </div>
        </div>

        <!-- Official Links & Support Section: 3-column grid on desktop -->
        <div class="space-y-3">
          <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
            {$_('help.resourcesTitle')}
          </h3>

          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {#each officialLinks as item}
              <a
                href={item.url}
                target="_blank"
                rel="noopener noreferrer"
                class="group flex items-start gap-3 rounded-xl border border-slate-200 bg-white p-3.5 shadow-xs transition-all hover:shadow-md dark:border-slate-800 dark:bg-slate-950 {item.hoverColor}"
              >
                <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border {item.badgeColor}">
                  <item.icon class="h-4.5 w-4.5" />
                </div>
                <div class="flex-1 min-w-0">
                  <div class="flex items-center justify-between gap-1">
                    <span class="text-xs font-bold text-slate-900 dark:text-slate-100 group-hover:text-cyan-600 dark:group-hover:text-cyan-400 transition-colors truncate">
                      {$_(item.titleKey)}
                    </span>
                    <ExternalLink class="h-3.5 w-3.5 text-slate-400 group-hover:text-slate-600 dark:group-hover:text-slate-300 shrink-0" />
                  </div>
                  <p class="mt-0.5 text-[11px] text-slate-500 dark:text-slate-400 leading-snug line-clamp-2">
                    {$_(item.descKey)}
                  </p>
                </div>
              </a>
            {/each}
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="flex items-center justify-between border-t border-slate-200 px-6 py-3.5 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
        <span class="text-[11px] text-slate-500 dark:text-slate-400 font-mono">
          TWSNMP NEO Community & Documentation
        </span>

        <button
          onclick={() => (show = false)}
          class="px-4 py-1.5 rounded-lg bg-slate-200 hover:bg-slate-300 dark:bg-slate-800 dark:hover:bg-slate-700 text-xs font-semibold text-slate-800 dark:text-slate-200 transition-colors shadow-xs"
        >
          {$_('common.close')}
        </button>
      </div>
    </div>
  </div>
{/if}
